//! Standard gRPC server runner for Rust holons.

use crate::describe;
use crate::observability;
use crate::transport::{self, Listener, StdioTransport, DEFAULT_URI};
use std::convert::Infallible;
use std::env;
use std::error::Error;
use std::fs;
use std::path::Path;
#[cfg(unix)]
use std::path::PathBuf;
use std::pin::Pin;
use std::process::Command;
use std::task::{Context, Poll};
use tokio::io::{AsyncRead, AsyncWrite};
use tokio_stream::wrappers::TcpListenerStream;
#[cfg(unix)]
use tokio_stream::wrappers::UnixListenerStream;
use tokio_stream::Stream;
use tonic::body::BoxBody;
use tonic::codegen::http::{Request, Response};
use tonic::server::NamedService;
use tonic::transport::server::Connected;
use tonic::transport::Server;
use tower_service::Service;

pub type Result<T> = std::result::Result<T, Box<dyn Error + Send + Sync>>;

#[derive(Debug, Clone, Default)]
pub struct RunOptions {
    pub accept_http1: bool,
    pub reflect: bool,
    pub proto_dir: Option<String>,
    pub manifest_path: Option<String>,
    pub describe_response: Option<crate::gen::holons::v1::DescribeResponse>,
    pub descriptor_set: Option<Vec<u8>>,
    pub env: Option<std::collections::HashMap<String, String>>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ParsedFlags {
    pub listen_uri: String,
    pub reflect: bool,
}

macro_rules! serve_router {
    ($router:expr, $listen_uri:expr, $options:expr, $obs:expr) => {{
        let router = $router;
        let options = $options;
        let listen_uri = $listen_uri;
        let obs = $obs;

        match transport::listen(listen_uri).await? {
            Listener::Tcp(listener) => {
                let actual_uri = bound_tcp_uri(listen_uri, &listener)?;
                start_observability_runtime(obs.as_ref(), &actual_uri, "tcp");
                announce_bound_uri(&actual_uri, options);
                router
                    .serve_with_incoming_shutdown(
                        TcpListenerStream::new(listener),
                        shutdown_signal(),
                    )
                    .await?;
            }
            #[cfg(unix)]
            Listener::Unix(listener) => {
                let cleanup = unix_socket_path(listen_uri)?;
                start_observability_runtime(obs.as_ref(), listen_uri, "unix");
                announce_bound_uri(listen_uri, options);
                let result = router
                    .serve_with_incoming_shutdown(
                        UnixListenerStream::new(listener),
                        shutdown_signal(),
                    )
                    .await;
                cleanup_unix_socket(cleanup.as_deref());
                result?;
            }
            Listener::Stdio => {
                start_observability_runtime(obs.as_ref(), "stdio://", "stdio");
                announce_bound_uri("stdio://", options);
                router
                    .serve_with_incoming_shutdown(
                        StdioIncoming::new(transport::listen_stdio()?),
                        shutdown_signal(),
                    )
                    .await?;
            }
            Listener::Ws(_) => {
                return Err(boxed_err(
                    "serve::run() does not support ws:// or wss://; use a custom server loop",
                ));
            }
        }

        Ok(())
    }};
}

/// Extract --listen or --port from command-line args.
pub fn parse_flags(args: &[String]) -> String {
    parse_options(args).listen_uri
}

/// Extract serve-related options from command-line args.
pub fn parse_options(args: &[String]) -> ParsedFlags {
    let mut listen_uri = DEFAULT_URI.to_string();
    let mut reflect = false;
    let mut i = 0;
    while i < args.len() {
        if args[i] == "--listen" && i + 1 < args.len() {
            listen_uri = args[i + 1].clone();
            i += 1;
        } else if args[i] == "--port" && i + 1 < args.len() {
            listen_uri = format!("tcp://:{}", args[i + 1]);
            i += 1;
        } else if args[i] == "--reflect" {
            reflect = true;
        }
        i += 1;
    }
    ParsedFlags {
        listen_uri,
        reflect,
    }
}

/// Run a single gRPC service on the requested transport URI.
pub async fn run_single<Svc>(listen_uri: &str, service: Svc) -> Result<()>
where
    Svc: Service<Request<BoxBody>, Response = Response<BoxBody>, Error = Infallible>
        + NamedService
        + Clone
        + Send
        + 'static,
    Svc::Future: Send + 'static,
{
    run_single_with_options(listen_uri, service, RunOptions::default()).await
}

/// Run a single gRPC service with server options.
pub async fn run_single_with_options<Svc>(
    listen_uri: &str,
    service: Svc,
    options: RunOptions,
) -> Result<()>
where
    Svc: Service<Request<BoxBody>, Response = Response<BoxBody>, Error = Infallible>
        + NamedService
        + Clone
        + Send
        + 'static,
    Svc::Future: Send + 'static,
{
    let mut builder = Server::builder().accept_http1(options.accept_http1);
    let obs = observability_from_options(&options)?;
    let obs_service = obs.as_ref().map(|obs| observability::service(obs.clone()));
    let meta_service = registered_holon_meta_service(&options)?;
    if options.reflect {
        let descriptor_set = auto_reflection_descriptor_set(&options)?
            .ok_or_else(|| boxed_err("reflection requested but no proto files were found"))?;
        let reflection = tonic_reflection::server::Builder::configure()
            .register_encoded_file_descriptor_set(&descriptor_set)
            .build_v1()
            .map_err(|err| boxed_err(format!("failed to build reflection service: {err}")))?;
        let router = builder
            .add_service(meta_service)
            .add_optional_service(obs_service)
            .add_service(reflection)
            .add_service(service);
        serve_router!(router, listen_uri, options, obs)
    } else {
        let router = builder
            .add_service(meta_service)
            .add_optional_service(obs_service)
            .add_service(service);
        serve_router!(router, listen_uri, options, obs)
    }
}

/// Run a gRPC service with one optional companion service, typically reflection.
pub async fn run<Extra, Svc>(
    listen_uri: &str,
    extra_service: Option<Extra>,
    service: Svc,
) -> Result<()>
where
    Extra: Service<Request<BoxBody>, Response = Response<BoxBody>, Error = Infallible>
        + NamedService
        + Clone
        + Send
        + 'static,
    Extra::Future: Send + 'static,
    Svc: Service<Request<BoxBody>, Response = Response<BoxBody>, Error = Infallible>
        + NamedService
        + Clone
        + Send
        + 'static,
    Svc::Future: Send + 'static,
{
    run_with_options(listen_uri, extra_service, service, RunOptions::default()).await
}

/// Run a gRPC service with one optional companion service and server options.
pub async fn run_with_options<Extra, Svc>(
    listen_uri: &str,
    extra_service: Option<Extra>,
    service: Svc,
    options: RunOptions,
) -> Result<()>
where
    Extra: Service<Request<BoxBody>, Response = Response<BoxBody>, Error = Infallible>
        + NamedService
        + Clone
        + Send
        + 'static,
    Extra::Future: Send + 'static,
    Svc: Service<Request<BoxBody>, Response = Response<BoxBody>, Error = Infallible>
        + NamedService
        + Clone
        + Send
        + 'static,
    Svc::Future: Send + 'static,
{
    let mut builder = Server::builder().accept_http1(options.accept_http1);
    let obs = observability_from_options(&options)?;
    let obs_service = obs.as_ref().map(|obs| observability::service(obs.clone()));
    let meta_service = registered_holon_meta_service(&options)?;
    if options.reflect {
        let descriptor_set = auto_reflection_descriptor_set(&options)?
            .ok_or_else(|| boxed_err("reflection requested but no proto files were found"))?;
        let reflection = tonic_reflection::server::Builder::configure()
            .register_encoded_file_descriptor_set(&descriptor_set)
            .build_v1()
            .map_err(|err| boxed_err(format!("failed to build reflection service: {err}")))?;
        let router = builder
            .add_service(meta_service)
            .add_optional_service(extra_service)
            .add_optional_service(obs_service)
            .add_service(reflection)
            .add_service(service);
        serve_router!(router, listen_uri, options, obs)
    } else {
        let router = builder
            .add_service(meta_service)
            .add_optional_service(extra_service)
            .add_optional_service(obs_service)
            .add_service(service);
        serve_router!(router, listen_uri, options, obs)
    }
}

fn registered_holon_meta_service(
    options: &RunOptions,
) -> Result<crate::gen::holons::v1::holon_meta_server::HolonMetaServer<crate::describe::MetaService>>
{
    if let Some(response) = options.describe_response.clone() {
        return Ok(describe::service_from_response(response));
    }

    match describe::service() {
        Ok(service) => Ok(service),
        Err(err) => {
            eprintln!("HolonMeta registration failed: {err}");
            Err(boxed_err(format!("register HolonMeta: {err}")))
        }
    }
}

fn observability_from_options(
    options: &RunOptions,
) -> Result<Option<std::sync::Arc<observability::Observability>>> {
    let env = options
        .env
        .clone()
        .unwrap_or_else(|| std::env::vars().collect());
    observability::check_env_from(&env).map_err(|err| boxed_err(err.to_string()))?;
    if env.get("OP_OBS").map(|s| s.trim()).unwrap_or("").is_empty() {
        return Ok(None);
    }
    let obs = observability::from_env_map(observability::Config::default(), &env);
    Ok((!obs.families.is_empty()).then_some(obs))
}

fn start_observability_runtime(
    obs: Option<&std::sync::Arc<observability::Observability>>,
    public_uri: &str,
    transport: &str,
) {
    let Some(obs) = obs else { return };
    if obs.cfg.run_dir.is_empty() {
        return;
    }
    observability::enable_disk_writers(&obs.cfg.run_dir);
    if obs.enabled(observability::Family::Events) {
        let mut payload = std::collections::BTreeMap::new();
        payload.insert("listener".to_string(), public_uri.to_string());
        obs.emit(observability::EventType::InstanceReady, payload);
    }
    let log_path = if obs.enabled(observability::Family::Logs) {
        Path::new(&obs.cfg.run_dir)
            .join("stdout.log")
            .to_string_lossy()
            .into_owned()
    } else {
        String::new()
    };
    let _ = observability::write_meta_json(
        &obs.cfg.run_dir,
        &observability::MetaJson {
            slug: obs.cfg.slug.clone(),
            uid: obs.cfg.instance_uid.clone(),
            pid: std::process::id() as i32,
            started_at: std::time::SystemTime::now(),
            mode: "persistent".to_string(),
            transport: transport.to_string(),
            address: public_uri.to_string(),
            metrics_addr: String::new(),
            log_path,
            log_bytes_rotated: 0,
            organism_uid: obs.cfg.organism_uid.clone(),
            organism_slug: obs.cfg.organism_slug.clone(),
            is_default: false,
        },
    );
}

fn announce_bound_uri(uri: &str, options: RunOptions) {
    let mode = format!(
        "http1 {}, reflection {}",
        if options.accept_http1 { "ON" } else { "OFF" },
        if options.reflect { "ON" } else { "OFF" }
    );
    eprintln!("gRPC server listening on {uri} ({mode})");
}

fn resolved_proto_dir(options: &RunOptions) -> Result<PathBuf> {
    let root = env::current_dir()?;
    if let Some(proto_dir) = options.proto_dir.as_ref() {
        let path = PathBuf::from(proto_dir);
        return Ok(if path.is_absolute() {
            path
        } else {
            root.join(path)
        });
    }
    Ok(root.join("protos"))
}

fn auto_reflection_descriptor_set(options: &RunOptions) -> Result<Option<Vec<u8>>> {
    if let Some(descriptor_set) = options.descriptor_set.clone() {
        return Ok(Some(descriptor_set));
    }

    let proto_dir = resolved_proto_dir(options)?;
    if !proto_dir.is_dir() {
        return Ok(None);
    }

    let proto_files = collect_relative_proto_files(&proto_dir)?;
    if proto_files.is_empty() {
        return Ok(None);
    }

    let descriptor_path = env::temp_dir().join(format!(
        "holons-reflection-{}-{}.bin",
        std::process::id(),
        std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .map_err(|err| boxed_err(format!("invalid system time: {err}")))?
            .as_nanos()
    ));

    let status = Command::new("protoc")
        .arg(format!("--proto_path={}", proto_dir.display()))
        .arg("--include_imports")
        .arg(format!(
            "--descriptor_set_out={}",
            descriptor_path.display()
        ))
        .args(&proto_files)
        .status()?;

    if !status.success() {
        let _ = fs::remove_file(&descriptor_path);
        return Err(boxed_err(format!(
            "protoc failed while building reflection descriptors from {}",
            proto_dir.display()
        )));
    }

    let bytes = fs::read(&descriptor_path)?;
    let _ = fs::remove_file(&descriptor_path);
    Ok(Some(bytes))
}

fn collect_relative_proto_files(root: &Path) -> Result<Vec<String>> {
    let mut files = Vec::new();
    collect_relative_proto_files_into(root, root, &mut files)?;
    files.sort();
    Ok(files)
}

fn collect_relative_proto_files_into(root: &Path, dir: &Path, out: &mut Vec<String>) -> Result<()> {
    for entry in fs::read_dir(dir)? {
        let entry = entry?;
        let path = entry.path();
        if path.is_dir() {
            collect_relative_proto_files_into(root, &path, out)?;
            continue;
        }
        if path.extension().and_then(|ext| ext.to_str()) == Some("proto") {
            let relative = path
                .strip_prefix(root)
                .map_err(|err| boxed_err(format!("strip proto path prefix: {err}")))?;
            out.push(relative.to_string_lossy().replace('\\', "/"));
        }
    }
    Ok(())
}

fn bound_tcp_uri(listen_uri: &str, listener: &tokio::net::TcpListener) -> Result<String> {
    let parsed = transport::parse_uri(listen_uri).map_err(boxed_err)?;
    let bound = listener.local_addr()?;
    let host = match parsed.host.as_deref() {
        Some("") | Some("0.0.0.0") | Some("::") | None => "127.0.0.1".to_string(),
        Some(host) => host.to_string(),
    };
    Ok(format!("tcp://{host}:{}", bound.port()))
}

#[cfg(unix)]
fn unix_socket_path(listen_uri: &str) -> Result<Option<PathBuf>> {
    let parsed = transport::parse_uri(listen_uri).map_err(boxed_err)?;
    Ok(parsed.path.map(PathBuf::from))
}

#[cfg(unix)]
fn cleanup_unix_socket(path: Option<&std::path::Path>) {
    if let Some(path) = path {
        let _ = std::fs::remove_file(path);
    }
}

fn boxed_err(message: impl Into<String>) -> Box<dyn Error + Send + Sync> {
    Box::new(std::io::Error::new(
        std::io::ErrorKind::Other,
        message.into(),
    ))
}

/// Hold a single stdio connection open for the lifetime of the server.
///
/// `serve_with_incoming_shutdown` treats the end of the incoming stream as a
/// signal that the server can drain and exit. For stdio we only ever have one
/// transport, so returning `None` immediately after yielding it makes the
/// server quit before the client sends its first RPC.
struct StdioIncoming<R = tokio::io::Stdin, W = tokio::io::Stdout> {
    transport: Option<StdioTransport<R, W>>,
}

impl<R, W> StdioIncoming<R, W> {
    fn new(transport: StdioTransport<R, W>) -> Self {
        Self {
            transport: Some(transport),
        }
    }
}

impl<R, W> Stream for StdioIncoming<R, W>
where
    R: Unpin,
    W: Unpin,
{
    type Item = std::io::Result<StdioTransport<R, W>>;

    fn poll_next(self: Pin<&mut Self>, _cx: &mut Context<'_>) -> Poll<Option<Self::Item>> {
        let this = self.get_mut();
        if let Some(transport) = this.transport.take() {
            return Poll::Ready(Some(Ok(transport)));
        }
        Poll::Pending
    }
}

async fn shutdown_signal() {
    #[cfg(unix)]
    {
        let mut terminate =
            tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())
                .expect("SIGTERM handler");
        tokio::select! {
            _ = tokio::signal::ctrl_c() => {}
            _ = terminate.recv() => {}
        }
    }

    #[cfg(not(unix))]
    {
        let _ = tokio::signal::ctrl_c().await;
    }
}

impl<R, W> Connected for StdioTransport<R, W>
where
    R: AsyncRead + Unpin + Send + 'static,
    W: AsyncWrite + Unpin + Send + 'static,
{
    type ConnectInfo = ();

    fn connect_info(&self) -> Self::ConnectInfo {}
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::gen::holons::v1::{
        holon_manifest::{Artifacts, Build, Identity},
        holon_meta_client::HolonMetaClient,
        holon_observability_client::HolonObservabilityClient,
        DescribeRequest, DescribeResponse, EventsRequest, HolonManifest, LogsRequest,
        MetricsRequest,
    };
    use crate::test_support::{acquire_process_guard, ProcessStateGuard};
    use std::fs;
    use std::future;
    use std::net::TcpListener;
    use tempfile::TempDir;
    use tokio::time::{timeout, Duration};
    use tokio_stream::StreamExt;
    use tonic::body::empty_body;

    #[test]
    fn test_parse_listen() {
        let args: Vec<String> = vec!["--listen".into(), "tcp://:8080".into()];
        assert_eq!(parse_flags(&args), "tcp://:8080");
    }

    #[test]
    fn test_parse_port() {
        let args: Vec<String> = vec!["--port".into(), "3000".into()];
        assert_eq!(parse_flags(&args), "tcp://:3000");
    }

    #[test]
    fn test_parse_default() {
        let args: Vec<String> = vec![];
        assert_eq!(parse_flags(&args), DEFAULT_URI);
    }

    #[test]
    fn test_parse_options_reflect() {
        let args: Vec<String> = vec!["--listen".into(), "tcp://:8080".into(), "--reflect".into()];
        let parsed = parse_options(&args);
        assert_eq!(parsed.listen_uri, "tcp://:8080");
        assert!(parsed.reflect);
    }

    #[tokio::test]
    async fn test_bound_tcp_uri_normalizes_wildcard_host() {
        let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let actual = bound_tcp_uri("tcp://:0", &listener).unwrap();
        assert!(actual.starts_with("tcp://127.0.0.1:"));
    }

    #[cfg(unix)]
    #[test]
    fn test_unix_socket_path() {
        let path = unix_socket_path("unix:///tmp/holons.sock").unwrap();
        assert_eq!(path.unwrap(), std::path::PathBuf::from("/tmp/holons.sock"));
    }

    #[tokio::test]
    async fn test_stdio_incoming_yields_once_then_waits() {
        let mut incoming =
            StdioIncoming::new(StdioTransport::new(tokio::io::empty(), tokio::io::sink()));

        assert!(incoming.next().await.unwrap().is_ok());
        assert!(
            timeout(Duration::from_millis(50), incoming.next())
                .await
                .is_err(),
            "stdio incoming should stay pending after the first connection"
        );
    }

    #[tokio::test]
    async fn test_run_single_with_options_uses_registered_static_holon_meta() {
        let _lock = acquire_process_guard().await;
        let _state = ProcessStateGuard::capture();
        let holon = write_echo_holon();
        let response = crate::describe::build_response(holon.path().join("protos")).unwrap();
        crate::describe::clear_static_response();
        crate::describe::use_static_response(response);

        let empty = TempDir::new().unwrap();
        env::set_current_dir(empty.path()).unwrap();

        let port = free_port();
        let listen_uri = format!("tcp://127.0.0.1:{port}");

        let server = tokio::spawn(async move {
            run_single_with_options(&listen_uri, UnimplementedService, RunOptions::default())
                .await
                .unwrap();
        });

        let endpoint = format!("http://127.0.0.1:{port}");
        let mut client = wait_for_holon_meta_client(&endpoint).await;
        let response = client
            .describe(DescribeRequest {})
            .await
            .unwrap()
            .into_inner();
        let identity = response
            .manifest
            .as_ref()
            .unwrap()
            .identity
            .as_ref()
            .unwrap();

        assert_eq!(identity.given_name, "Echo");
        assert_eq!(identity.family_name, "Server");
        assert_eq!(identity.motto, "Reply precisely.");
        assert_eq!(response.services.len(), 1);
        assert_eq!(response.services[0].name, "echo.v1.Echo");

        crate::describe::clear_static_response();
        server.abort();
        let _ = server.await;
    }

    #[tokio::test]
    async fn test_run_single_with_options_fails_without_registered_holon_meta() {
        let _lock = acquire_process_guard().await;
        let _state = ProcessStateGuard::capture();
        crate::describe::clear_static_response();
        let empty = TempDir::new().unwrap();
        env::set_current_dir(empty.path()).unwrap();

        let port = free_port();
        let listen_uri = format!("tcp://127.0.0.1:{port}");

        let error =
            run_single_with_options(&listen_uri, UnimplementedService, RunOptions::default())
                .await
                .unwrap_err();
        assert_eq!(
            error.to_string(),
            format!(
                "register HolonMeta: {}",
                crate::describe::ERR_NO_INCODE_DESCRIPTION
            )
        );
    }

    #[tokio::test]
    async fn test_run_single_with_options_registers_embedded_holon_meta() {
        let _lock = acquire_process_guard().await;
        let _state = ProcessStateGuard::capture();
        crate::describe::clear_static_response();
        let empty = TempDir::new().unwrap();
        env::set_current_dir(empty.path()).unwrap();

        let port = free_port();
        let listen_uri = format!("tcp://127.0.0.1:{port}");

        let server = tokio::spawn(async move {
            run_single_with_options(
                &listen_uri,
                UnimplementedService,
                RunOptions {
                    describe_response: Some(DescribeResponse {
                        manifest: Some(embedded_manifest(
                            "Embedded",
                            "Holon",
                            "Runs without local proto files.",
                            "1.0.0",
                        )),
                        services: Vec::new(),
                    }),
                    ..RunOptions::default()
                },
            )
            .await
            .unwrap();
        });

        let endpoint = format!("http://127.0.0.1:{port}");
        let mut client = wait_for_holon_meta_client(&endpoint).await;
        let response = client
            .describe(DescribeRequest {})
            .await
            .unwrap()
            .into_inner();
        let identity = response
            .manifest
            .as_ref()
            .unwrap()
            .identity
            .as_ref()
            .unwrap();

        assert_eq!(identity.given_name, "Embedded");
        assert_eq!(identity.family_name, "Holon");
        assert_eq!(identity.motto, "Runs without local proto files.");
        assert_eq!(identity.version, "1.0.0");
        assert!(response.services.is_empty());

        crate::describe::clear_static_response();
        server.abort();
        let _ = server.await;
    }

    #[tokio::test]
    async fn test_run_single_with_options_registers_observability_service() {
        let _lock = acquire_process_guard().await;
        let _state = ProcessStateGuard::capture();
        crate::describe::clear_static_response();
        observability::reset();
        let empty = TempDir::new().unwrap();
        env::set_current_dir(empty.path()).unwrap();

        let registry = TempDir::new().unwrap();
        let mut obs_env = std::collections::HashMap::new();
        obs_env.insert("OP_OBS".to_string(), "logs,metrics,events".to_string());
        obs_env.insert(
            "OP_RUN_DIR".to_string(),
            registry.path().to_string_lossy().into_owned(),
        );
        obs_env.insert("OP_INSTANCE_UID".to_string(), "rust-obs-1".to_string());

        let port = free_port();
        let listen_uri = format!("tcp://127.0.0.1:{port}");

        let server = tokio::spawn(async move {
            run_single_with_options(
                &listen_uri,
                UnimplementedService,
                RunOptions {
                    env: Some(obs_env),
                    describe_response: Some(DescribeResponse {
                        manifest: Some(embedded_manifest(
                            "Observable",
                            "Holon",
                            "Registers HolonObservability.",
                            "1.0.0",
                        )),
                        services: Vec::new(),
                    }),
                    ..RunOptions::default()
                },
            )
            .await
            .unwrap();
        });

        let endpoint = format!("http://127.0.0.1:{port}");
        let mut client = wait_for_holon_observability_client(&endpoint).await;
        let obs = observability::current();
        obs.logger("serve-test")
            .info("serve-log", &[("sdk", "rust")]);
        obs.counter(
            "serve_requests_total",
            "",
            std::collections::BTreeMap::new(),
        )
        .unwrap()
        .inc();
        obs.emit(
            observability::EventType::InstanceReady,
            std::collections::BTreeMap::new(),
        );

        let logs = client
            .logs(LogsRequest {
                min_level: observability::Level::Info as i32,
                session_ids: Vec::new(),
                rpc_methods: Vec::new(),
                since: None,
                follow: false,
            })
            .await
            .unwrap()
            .into_inner()
            .collect::<Vec<_>>()
            .await;
        assert!(logs
            .iter()
            .any(|entry| entry.as_ref().unwrap().message == "serve-log"));

        let metrics = client
            .metrics(MetricsRequest {
                name_prefixes: Vec::new(),
                include_session_rollup: false,
            })
            .await
            .unwrap()
            .into_inner();
        assert!(metrics
            .samples
            .iter()
            .any(|sample| sample.name == "serve_requests_total"));

        let events = client
            .events(EventsRequest {
                types: Vec::new(),
                since: None,
                follow: false,
            })
            .await
            .unwrap()
            .into_inner()
            .collect::<Vec<_>>()
            .await;
        assert!(events.iter().any(|event| {
            event.as_ref().unwrap().r#type == observability::EventType::InstanceReady as i32
        }));

        let meta_path = registry
            .path()
            .join(&obs.cfg.slug)
            .join("rust-obs-1")
            .join("meta.json");
        assert!(meta_path.is_file());

        observability::reset();
        crate::describe::clear_static_response();
        server.abort();
        let _ = server.await;
    }

    #[tokio::test]
    async fn test_run_single_with_options_rejects_ws_and_wss() {
        let _lock = acquire_process_guard().await;
        let _state = ProcessStateGuard::capture();

        for listen_uri in ["ws://127.0.0.1:8080/grpc", "wss://example.com:443/grpc"] {
            let error = run_single_with_options(
                listen_uri,
                UnimplementedService,
                RunOptions {
                    describe_response: Some(DescribeResponse {
                        manifest: Some(embedded_manifest(
                            "Unsupported",
                            "Transport",
                            "Checks transport audit failures.",
                            "1.0.0",
                        )),
                        services: Vec::new(),
                    }),
                    ..RunOptions::default()
                },
            )
            .await
            .unwrap_err();
            assert_eq!(
                error.to_string(),
                "serve::run() does not support ws:// or wss://; use a custom server loop"
            );
        }
    }

    #[derive(Clone, Default)]
    struct UnimplementedService;

    impl NamedService for UnimplementedService {
        const NAME: &'static str = "test.v1.Noop";
    }

    impl Service<Request<BoxBody>> for UnimplementedService {
        type Response = Response<BoxBody>;
        type Error = Infallible;
        type Future = future::Ready<std::result::Result<Self::Response, Self::Error>>;

        fn poll_ready(
            &mut self,
            _cx: &mut Context<'_>,
        ) -> Poll<std::result::Result<(), Self::Error>> {
            Poll::Ready(Ok(()))
        }

        fn call(&mut self, _req: Request<BoxBody>) -> Self::Future {
            let mut response = Response::new(empty_body());
            let headers = response.headers_mut();
            headers.insert(
                tonic::Status::GRPC_STATUS,
                (tonic::Code::Unimplemented as i32).into(),
            );
            headers.insert(
                tonic::codegen::http::header::CONTENT_TYPE,
                tonic::codegen::http::HeaderValue::from_static("application/grpc"),
            );
            future::ready(Ok(response))
        }
    }

    async fn wait_for_holon_meta_client(
        endpoint: &str,
    ) -> HolonMetaClient<tonic::transport::Channel> {
        let deadline = tokio::time::Instant::now() + Duration::from_secs(2);
        loop {
            match HolonMetaClient::connect(endpoint.to_string()).await {
                Ok(client) => return client,
                Err(err) if tokio::time::Instant::now() < deadline => {
                    let _ = err;
                    tokio::time::sleep(Duration::from_millis(25)).await;
                }
                Err(err) => panic!("timed out waiting for HolonMeta client: {err}"),
            }
        }
    }

    async fn wait_for_holon_observability_client(
        endpoint: &str,
    ) -> HolonObservabilityClient<tonic::transport::Channel> {
        let deadline = tokio::time::Instant::now() + Duration::from_secs(2);
        loop {
            match HolonObservabilityClient::connect(endpoint.to_string()).await {
                Ok(client) => return client,
                Err(err) if tokio::time::Instant::now() < deadline => {
                    let _ = err;
                    tokio::time::sleep(Duration::from_millis(25)).await;
                }
                Err(err) => panic!("timed out waiting for HolonObservability client: {err}"),
            }
        }
    }

    fn free_port() -> u16 {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        listener.local_addr().unwrap().port()
    }

    fn embedded_manifest(
        given_name: &str,
        family_name: &str,
        motto: &str,
        version: &str,
    ) -> HolonManifest {
        HolonManifest {
            identity: Some(Identity {
                schema: "holon/v1".to_string(),
                uuid: "embedded-holon-0000".to_string(),
                given_name: given_name.to_string(),
                family_name: family_name.to_string(),
                motto: motto.to_string(),
                composer: "serve-test".to_string(),
                status: "draft".to_string(),
                born: "2026-03-20".to_string(),
                version: version.to_string(),
                aliases: Vec::new(),
            }),
            description: String::new(),
            lang: "rust".to_string(),
            skills: Vec::new(),
            contract: None,
            kind: "native".to_string(),
            platforms: Vec::new(),
            transport: String::new(),
            build: Some(Build {
                runner: String::new(),
                main: String::new(),
                defaults: None,
                members: Vec::new(),
                targets: std::collections::HashMap::new(),
                templates: Vec::new(),
                before_commands: Vec::new(),
                after_commands: Vec::new(),
            }),
            requires: None,
            artifacts: Some(Artifacts {
                binary: String::new(),
                primary: String::new(),
                by_target: std::collections::HashMap::new(),
            }),
            sequences: Vec::new(),
            guide: String::new(),
            session_visibility: 0,
            session_visibility_overrides: Vec::new(),
        }
    }

    fn write_echo_holon() -> TempDir {
        let dir = TempDir::new().unwrap();
        let proto_dir = dir.path().join("protos/echo/v1");
        fs::create_dir_all(&proto_dir).unwrap();
        fs::write(
            dir.path().join("holon.proto"),
            r#"syntax = "proto3";

package holons.test.v1;

option (holons.v1.manifest) = {
  identity: {
    uuid: "echo-server-0000"
    given_name: "Echo"
    family_name: "Server"
    motto: "Reply precisely."
    composer: "serve-test"
    status: "draft"
    born: "2026-03-17"
  }
  lang: "rust"
};
"#,
        )
        .unwrap();
        fs::write(
            proto_dir.join("echo.proto"),
            r#"syntax = "proto3";
package echo.v1;

// Echo echoes request payloads for documentation tests.
service Echo {
  // Ping echoes the inbound message.
  rpc Ping(PingRequest) returns (PingResponse);
}

message PingRequest {
  string message = 1;
}

message PingResponse {
  string message = 1;
}
"#,
        )
        .unwrap();
        dir
    }
}
