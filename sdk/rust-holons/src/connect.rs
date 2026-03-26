//! Resolve holons by slug or direct target and return ready gRPC channels.

use crate::discover::{self, HolonEntry};
use hyper_util::rt::TokioIo;
use std::collections::HashMap;
use std::env;
use std::error::Error;
use std::ffi::OsStr;
use std::future::Future;
use std::path::{Path, PathBuf};
use std::pin::Pin;
use std::process::Stdio;
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::{Arc, Mutex, OnceLock};
use std::task::{Context, Poll};
use std::time::Duration;
use tokio::io::{AsyncBufReadExt, AsyncRead, AsyncWrite, ReadBuf};
use tokio::net::TcpStream;
use tokio::process::{Child, ChildStdin, ChildStdout, Command};
use tokio::time::{self, Instant};
use tonic::codegen::http::Uri;
use tonic::transport::{Channel, Endpoint};
use tower_service::Service;

pub type Result<T> = std::result::Result<T, Box<dyn Error>>;

const DEFAULT_TIMEOUT: Duration = Duration::from_secs(5);

#[derive(Debug, Clone)]
pub struct ConnectOptions {
    pub timeout: Duration,
    pub transport: String,
    pub start: bool,
    pub port_file: Option<String>,
}

impl Default for ConnectOptions {
    fn default() -> Self {
        Self {
            timeout: DEFAULT_TIMEOUT,
            transport: "stdio".to_string(),
            start: true,
            port_file: None,
        }
    }
}

struct ProcessOwner {
    pid: u32,
    child: tokio::sync::Mutex<Option<Child>>,
    leases: AtomicUsize,
    stopped: AtomicBool,
}

struct ProcessLease {
    owner: Arc<ProcessOwner>,
}

struct TcpConnector {
    address: String,
    _lease: Option<ProcessLease>,
}

struct StdioConnector {
    transport: Arc<Mutex<Option<ChildStdioTransport>>>,
    _lease: Option<ProcessLease>,
}

struct ChildStdioTransport {
    reader: ChildStdout,
    writer: ChildStdin,
}

fn started_registry() -> &'static Mutex<HashMap<u32, Arc<ProcessOwner>>> {
    static REGISTRY: OnceLock<Mutex<HashMap<u32, Arc<ProcessOwner>>>> = OnceLock::new();
    REGISTRY.get_or_init(|| Mutex::new(HashMap::new()))
}

pub async fn connect(target: &str) -> Result<Channel> {
    connect_with_mode(target, ConnectOptions::default(), true).await
}

pub async fn connect_with_opts(target: &str, opts: ConnectOptions) -> Result<Channel> {
    connect_with_mode(target, opts, false).await
}

pub async fn disconnect(channel: Channel) -> Result<()> {
    drop(channel);
    cleanup_orphaned_processes().await
}

async fn connect_with_mode(
    target: &str,
    mut opts: ConnectOptions,
    ephemeral: bool,
) -> Result<Channel> {
    let trimmed = target.trim();
    if trimmed.is_empty() {
        return Err(boxed_err("target is required"));
    }

    if opts.timeout.is_zero() {
        opts.timeout = DEFAULT_TIMEOUT;
    }

    if is_direct_target(trimmed) {
        return connect_direct(trimmed, opts.timeout, None).await;
    }

    let transport = if opts.transport.trim().is_empty() {
        ConnectOptions::default().transport
    } else {
        opts.transport.trim().to_string()
    }
    .to_lowercase();
    match transport.as_str() {
        "tcp" => {}
        "stdio" if ephemeral => {}
        "stdio" => return Err(boxed_err("stdio transport only supports connect()")),
        other => return Err(boxed_err(format!("unsupported transport {other:?}"))),
    }

    let entry = discover::find_by_slug(trimmed)?
        .ok_or_else(|| boxed_err(format!("holon {trimmed:?} not found")))?;

    if transport == "stdio" {
        if !opts.start {
            return Err(boxed_err(format!("holon {trimmed:?} is not running")));
        }

        let binary_path = resolve_binary_path(&entry)?;
        let (transport, child) = start_stdio_holon(&binary_path)?;
        let owner = remember_started(child)?;
        let endpoint = Endpoint::from_shared("http://127.0.0.1:50051".to_string())?
            .connect_timeout(opts.timeout)
            .timeout(opts.timeout);
        let channel = match endpoint
            .connect_with_connector(StdioConnector {
                transport: Arc::new(Mutex::new(Some(transport))),
                _lease: Some(owner.lease()),
            })
            .await
        {
            Ok(channel) => channel,
            Err(err) => {
                let owner = started_registry().lock().unwrap().remove(&owner.pid);
                if let Some(owner) = owner {
                    let _ = owner.stop().await;
                }
                return Err(err.into());
            }
        };
        return Ok(channel);
    }

    let port_file = opts
        .port_file
        .as_deref()
        .map(PathBuf::from)
        .unwrap_or_else(|| default_port_file_path(&entry.slug));

    if let Some(uri) = usable_port_file(&port_file, opts.timeout).await? {
        return connect_direct(&uri, opts.timeout, None).await;
    }
    if !opts.start {
        return Err(boxed_err(format!("holon {trimmed:?} is not running")));
    }

    let binary_path = resolve_binary_path(&entry)?;
    let (listen_uri, child) = start_tcp_holon(&binary_path, opts.timeout).await?;

    if ephemeral {
        let owner = remember_started(child)?;
        return match connect_direct(&listen_uri, opts.timeout, Some(owner.clone())).await {
            Ok(channel) => Ok(channel),
            Err(err) => {
                let owner = started_registry().lock().unwrap().remove(&owner.pid);
                if let Some(owner) = owner {
                    let _ = owner.stop().await;
                }
                Err(err)
            }
        };
    }

    let mut child = child;
    let channel = match connect_direct(&listen_uri, opts.timeout, None).await {
        Ok(channel) => channel,
        Err(err) => {
            let _ = stop_child(&mut child).await;
            return Err(err);
        }
    };
    if let Err(err) = write_port_file(&port_file, &listen_uri) {
        let _ = stop_child(&mut child).await;
        return Err(err);
    }
    tokio::spawn(async move {
        let _ = child.wait().await;
    });
    Ok(channel)
}

async fn connect_direct(
    target: &str,
    timeout: Duration,
    owner: Option<Arc<ProcessOwner>>,
) -> Result<Channel> {
    let normalized = normalize_direct_target(target)?;
    let endpoint = Endpoint::from_shared(normalized.endpoint_uri)?
        .connect_timeout(timeout)
        .timeout(timeout);

    let channel = endpoint
        .connect_with_connector(TcpConnector {
            address: normalized.address,
            _lease: owner.map(|owner| owner.lease()),
        })
        .await?;

    Ok(channel)
}

async fn cleanup_orphaned_processes() -> Result<()> {
    let deadline = Instant::now() + Duration::from_secs(2);

    loop {
        let owners = {
            let mut guard = started_registry().lock().unwrap();
            let orphaned: Vec<u32> = guard
                .iter()
                .filter_map(|(pid, owner)| (owner.lease_count() == 0).then_some(*pid))
                .collect();
            orphaned
                .into_iter()
                .filter_map(|pid| guard.remove(&pid))
                .collect::<Vec<_>>()
        };

        if !owners.is_empty() {
            let mut first_error: Option<Box<dyn Error>> = None;
            for owner in owners {
                if let Err(err) = owner.stop().await {
                    if first_error.is_none() {
                        first_error = Some(err);
                    }
                }
            }
            return match first_error {
                Some(err) => Err(err),
                None => Ok(()),
            };
        }

        if Instant::now() >= deadline {
            let owners = {
                let mut guard = started_registry().lock().unwrap();
                guard.drain().map(|(_, owner)| owner).collect::<Vec<_>>()
            };
            if owners.is_empty() {
                return Ok(());
            }

            let mut first_error: Option<Box<dyn Error>> = None;
            for owner in owners {
                if let Err(err) = owner.stop().await {
                    if first_error.is_none() {
                        first_error = Some(err);
                    }
                }
            }
            return match first_error {
                Some(err) => Err(err),
                None => Ok(()),
            };
        }

        time::sleep(Duration::from_millis(10)).await;
    }
}

impl ProcessOwner {
    fn new(child: Child) -> Result<Arc<Self>> {
        let pid = child
            .id()
            .ok_or_else(|| boxed_err("spawned process has no pid"))?;
        Ok(Arc::new(Self {
            pid,
            child: tokio::sync::Mutex::new(Some(child)),
            leases: AtomicUsize::new(0),
            stopped: AtomicBool::new(false),
        }))
    }

    fn lease(self: &Arc<Self>) -> ProcessLease {
        self.leases.fetch_add(1, Ordering::SeqCst);
        ProcessLease {
            owner: self.clone(),
        }
    }

    fn lease_count(&self) -> usize {
        self.leases.load(Ordering::SeqCst)
    }

    async fn stop(&self) -> Result<()> {
        if self.stopped.swap(true, Ordering::SeqCst) {
            return Ok(());
        }

        let mut guard = self.child.lock().await;
        let Some(child) = guard.as_mut() else {
            return Ok(());
        };

        if child.try_wait()?.is_some() {
            *guard = None;
            return Ok(());
        }

        send_sigterm(self.pid)?;
        let deadline = Instant::now() + Duration::from_secs(2);
        loop {
            if child.try_wait()?.is_some() {
                *guard = None;
                return Ok(());
            }
            if Instant::now() >= deadline {
                break;
            }
            time::sleep(Duration::from_millis(50)).await;
        }

        let _ = send_sigkill(self.pid);
        child.kill().await?;
        let _ = child.wait().await;
        *guard = None;
        Ok(())
    }
}

impl AsyncRead for ChildStdioTransport {
    fn poll_read(
        mut self: Pin<&mut Self>,
        cx: &mut Context<'_>,
        buf: &mut ReadBuf<'_>,
    ) -> Poll<std::io::Result<()>> {
        Pin::new(&mut self.reader).poll_read(cx, buf)
    }
}

impl Drop for ProcessLease {
    fn drop(&mut self) {
        self.owner.leases.fetch_sub(1, Ordering::SeqCst);
    }
}

impl AsyncWrite for ChildStdioTransport {
    fn poll_write(
        mut self: Pin<&mut Self>,
        cx: &mut Context<'_>,
        buf: &[u8],
    ) -> Poll<std::io::Result<usize>> {
        Pin::new(&mut self.writer).poll_write(cx, buf)
    }

    fn poll_flush(mut self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<std::io::Result<()>> {
        Pin::new(&mut self.writer).poll_flush(cx)
    }

    fn poll_shutdown(mut self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<std::io::Result<()>> {
        Pin::new(&mut self.writer).poll_shutdown(cx)
    }
}

impl Service<Uri> for TcpConnector {
    type Response = TokioIo<TcpStream>;
    type Error = std::io::Error;
    type Future = Pin<Box<dyn Future<Output = std::io::Result<TokioIo<TcpStream>>> + Send>>;

    fn poll_ready(&mut self, _cx: &mut Context<'_>) -> Poll<std::io::Result<()>> {
        Poll::Ready(Ok(()))
    }

    fn call(&mut self, _uri: Uri) -> Self::Future {
        let address = self.address.clone();
        Box::pin(async move { TcpStream::connect(address).await.map(TokioIo::new) })
    }
}

impl Service<Uri> for StdioConnector {
    type Response = TokioIo<ChildStdioTransport>;
    type Error = std::io::Error;
    type Future =
        Pin<Box<dyn Future<Output = std::io::Result<TokioIo<ChildStdioTransport>>> + Send>>;

    fn poll_ready(&mut self, _cx: &mut Context<'_>) -> Poll<std::io::Result<()>> {
        Poll::Ready(Ok(()))
    }

    fn call(&mut self, _uri: Uri) -> Self::Future {
        let transport = self.transport.lock().unwrap().take();
        Box::pin(async move {
            transport.map(TokioIo::new).ok_or_else(|| {
                std::io::Error::new(
                    std::io::ErrorKind::BrokenPipe,
                    "stdio transport already consumed",
                )
            })
        })
    }
}

fn remember_started(child: Child) -> Result<Arc<ProcessOwner>> {
    let owner = ProcessOwner::new(child)?;
    started_registry()
        .lock()
        .unwrap()
        .insert(owner.pid, owner.clone());
    Ok(owner)
}

async fn usable_port_file(path: &Path, timeout: Duration) -> Result<Option<String>> {
    let data = match std::fs::read_to_string(path) {
        Ok(data) => data,
        Err(err) if err.kind() == std::io::ErrorKind::NotFound => return Ok(None),
        Err(err) => return Err(err.into()),
    };

    let target = data.trim().to_string();
    if target.is_empty() {
        let _ = std::fs::remove_file(path);
        return Ok(None);
    }

    let check_timeout = timeout
        .min(Duration::from_secs(1))
        .max(Duration::from_millis(250));
    match connect_direct(&target, check_timeout, None).await {
        Ok(channel) => {
            drop(channel);
            Ok(Some(target))
        }
        Err(_) => {
            let _ = std::fs::remove_file(path);
            Ok(None)
        }
    }
}

async fn start_tcp_holon(binary_path: &Path, timeout: Duration) -> Result<(String, Child)> {
    let mut command = Command::new(binary_path);
    #[cfg(unix)]
    command.process_group(0);
    command
        .arg("serve")
        .arg("--listen")
        .arg("tcp://127.0.0.1:0")
        .stdin(Stdio::null())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped());

    let mut child = command.spawn()?;
    let stdout = child
        .stdout
        .take()
        .ok_or_else(|| boxed_err("failed to capture startup stdout"))?;
    let stderr = child
        .stderr
        .take()
        .ok_or_else(|| boxed_err("failed to capture startup stderr"))?;

    let (line_tx, mut line_rx) = tokio::sync::mpsc::unbounded_channel::<String>();
    spawn_line_reader(stdout, line_tx.clone());
    spawn_line_reader(stderr, line_tx.clone());
    drop(line_tx);

    let deadline = Instant::now() + timeout;
    loop {
        if let Some(status) = child.try_wait()? {
            return Err(boxed_err(format!(
                "holon exited before advertising an address: {status}"
            )));
        }

        if Instant::now() >= deadline {
            let _ = stop_child(&mut child).await;
            return Err(boxed_err("timed out waiting for holon startup"));
        }

        if let Ok(Some(line)) = time::timeout(Duration::from_millis(50), line_rx.recv()).await {
            if let Some(uri) = first_uri(&line) {
                return Ok((uri, child));
            }
        }
    }
}

fn start_stdio_holon(binary_path: &Path) -> Result<(ChildStdioTransport, Child)> {
    let mut command = Command::new(binary_path);
    #[cfg(unix)]
    command.process_group(0);
    command
        .arg("serve")
        .arg("--listen")
        .arg("stdio://")
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::null());

    let mut child = command.spawn()?;
    let stdin = child
        .stdin
        .take()
        .ok_or_else(|| boxed_err("failed to capture child stdin"))?;
    let stdout = child
        .stdout
        .take()
        .ok_or_else(|| boxed_err("failed to capture child stdout"))?;

    Ok((
        ChildStdioTransport {
            reader: stdout,
            writer: stdin,
        },
        child,
    ))
}

fn spawn_line_reader<R>(reader: R, tx: tokio::sync::mpsc::UnboundedSender<String>)
where
    R: AsyncRead + Unpin + Send + 'static,
{
    tokio::spawn(async move {
        let mut lines = tokio::io::BufReader::new(reader).lines();
        while let Ok(Some(line)) = lines.next_line().await {
            let _ = tx.send(line);
        }
    });
}

fn resolve_binary_path(entry: &HolonEntry) -> Result<PathBuf> {
    let manifest = entry
        .manifest
        .as_ref()
        .ok_or_else(|| boxed_err(format!("holon {:?} has no manifest", entry.slug)))?;

    let binary = manifest.artifacts.binary.trim();
    if binary.is_empty() {
        return Err(boxed_err(format!(
            "holon {:?} has no artifacts.binary",
            entry.slug
        )));
    }

    let binary_path = PathBuf::from(binary);
    if binary_path.is_absolute() && binary_path.is_file() {
        return Ok(binary_path);
    }

    let built = entry
        .dir
        .join(".op")
        .join("build")
        .join("bin")
        .join(file_name(binary)?);
    if built.is_file() {
        return Ok(built);
    }

    if let Some(path) = find_on_path(file_name(binary)?) {
        return Ok(path);
    }

    Err(boxed_err(format!(
        "built binary not found for holon {:?}",
        entry.slug
    )))
}

fn default_port_file_path(slug: &str) -> PathBuf {
    let root = env::current_dir().unwrap_or_else(|_| PathBuf::from("."));
    root.join(".op").join("run").join(format!("{slug}.port"))
}

fn write_port_file(path: &Path, uri: &str) -> Result<()> {
    if let Some(parent) = path.parent() {
        std::fs::create_dir_all(parent)?;
    }
    std::fs::write(path, format!("{}\n", uri.trim()))?;
    Ok(())
}

fn is_direct_target(target: &str) -> bool {
    target.contains("://") || target.contains(':')
}

fn first_uri(line: &str) -> Option<String> {
    line.split_whitespace().find_map(|field| {
        let trimmed = field.trim_matches(|ch: char| "\"'()[]{}., ".contains(ch));
        matches!(
            (),
            _ if trimmed.starts_with("tcp://")
                || trimmed.starts_with("unix://")
                || trimmed.starts_with("ws://")
                || trimmed.starts_with("wss://")
                || trimmed.starts_with("stdio://")
        )
        .then(|| trimmed.to_string())
    })
}

fn normalize_direct_target(target: &str) -> Result<DirectTarget> {
    let trimmed = target.trim();
    if trimmed.is_empty() {
        return Err(boxed_err("target is required"));
    }

    if trimmed.starts_with("http://") || trimmed.starts_with("https://") {
        let uri: Uri = trimmed.parse()?;
        let host = uri
            .host()
            .ok_or_else(|| boxed_err(format!("invalid target {trimmed:?}: missing host")))?;
        let host = normalize_host(host);
        let port = uri
            .port_u16()
            .unwrap_or(if uri.scheme_str() == Some("https") {
                443
            } else {
                80
            });
        return Ok(DirectTarget {
            address: format!("{host}:{port}"),
            endpoint_uri: format!("{}://{host}:{port}", uri.scheme_str().unwrap_or("http")),
        });
    }

    if trimmed.starts_with("tcp://") {
        let parsed = crate::transport::parse_uri(trimmed).map_err(boxed_err)?;
        let host = normalize_host(parsed.host.as_deref().unwrap_or("127.0.0.1"));
        let port = parsed
            .port
            .ok_or_else(|| boxed_err(format!("invalid tcp target {trimmed:?}: missing port")))?;
        return Ok(DirectTarget {
            address: format!("{host}:{port}"),
            endpoint_uri: format!("http://{host}:{port}"),
        });
    }

    if trimmed.starts_with("ws://")
        || trimmed.starts_with("wss://")
        || trimmed.starts_with("rest+sse://")
    {
        return Err(boxed_err(format!("unsupported direct target {trimmed:?}")));
    }

    if trimmed.contains(':') {
        let (host, port) = split_host_port(trimmed)?;
        let host = normalize_host(&host).to_string();
        return Ok(DirectTarget {
            address: format!("{host}:{port}"),
            endpoint_uri: format!("http://{host}:{port}"),
        });
    }

    Err(boxed_err(format!("unsupported direct target {trimmed:?}")))
}

fn normalize_host(host: &str) -> &str {
    match host {
        "" | "0.0.0.0" | "::" => "127.0.0.1",
        other => other,
    }
}

async fn stop_child(child: &mut Child) -> Result<()> {
    if let Some(status) = child.try_wait()? {
        let _ = status;
        return Ok(());
    }

    if let Some(pid) = child.id() {
        let _ = send_sigterm(pid);
        let deadline = Instant::now() + Duration::from_secs(2);
        loop {
            if child.try_wait()?.is_some() {
                return Ok(());
            }
            if Instant::now() >= deadline {
                break;
            }
            time::sleep(Duration::from_millis(50)).await;
        }
    }

    if let Some(pid) = child.id() {
        let _ = send_sigkill(pid);
    }
    child.kill().await?;
    let _ = child.wait().await;
    Ok(())
}

#[cfg(unix)]
fn send_sigterm(pid: u32) -> Result<()> {
    send_signal(pid, libc::SIGTERM)
}

#[cfg(not(unix))]
fn send_sigterm(pid: u32) -> Result<()> {
    let _ = pid;
    Ok(())
}

#[cfg(unix)]
fn send_sigkill(pid: u32) -> Result<()> {
    send_signal(pid, libc::SIGKILL)
}

#[cfg(not(unix))]
fn send_sigkill(pid: u32) -> Result<()> {
    let _ = pid;
    Ok(())
}

#[cfg(unix)]
fn send_signal(pid: u32, signal: i32) -> Result<()> {
    let status = unsafe { libc::kill(-(pid as i32), signal) };
    if status == 0 || std::io::Error::last_os_error().raw_os_error() == Some(libc::ESRCH) {
        return Ok(());
    }
    Err(std::io::Error::last_os_error().into())
}

fn file_name(path: &str) -> Result<&OsStr> {
    Path::new(path)
        .file_name()
        .ok_or_else(|| boxed_err(format!("invalid binary path {path:?}")))
}

fn split_host_port(value: &str) -> Result<(String, u16)> {
    let (host, port) = value
        .rsplit_once(':')
        .ok_or_else(|| boxed_err(format!("invalid host:port target {value:?}")))?;
    let port = port.parse::<u16>()?;
    let host = if host.is_empty() {
        "127.0.0.1".to_string()
    } else {
        host.to_string()
    };
    Ok((host, port))
}

fn find_on_path(binary_name: &OsStr) -> Option<PathBuf> {
    let path_var = env::var_os("PATH")?;
    env::split_paths(&path_var)
        .map(|dir| dir.join(binary_name))
        .find(|candidate| candidate.is_file())
}

fn boxed_err(message: impl Into<String>) -> Box<dyn Error> {
    std::io::Error::other(message.into()).into()
}

#[derive(Debug)]
struct DirectTarget {
    address: String,
    endpoint_uri: String,
}

#[cfg(all(test, unix))]
mod tests {
    use super::*;
    use crate::gen::holons::v1::{holon_meta_client::HolonMetaClient, DescribeRequest};
    use crate::test_support::{acquire_process_guard, ProcessStateGuard};
    use std::fs;
    use std::net::TcpListener;
    use std::os::unix::fs::PermissionsExt;
    use std::process::Command as ProcessCommand;
    use std::sync::OnceLock;
    use std::time::{SystemTime, UNIX_EPOCH};

    #[tokio::test]
    async fn test_connect_direct_dial_keeps_manual_server_running() {
        let _lock = acquire_process_guard().await;
        let fixed_port = free_port();
        let listen_uri = format!("tcp://127.0.0.1:{fixed_port}");
        let server_bin = compiled_echo_server_binary();
        let mut server = start_server_process(&server_bin, &listen_uri).await;

        let channel = connect(&format!("127.0.0.1:{fixed_port}")).await.unwrap();
        disconnect(channel).await.unwrap();

        wait_for_tcp_open(fixed_port).await.unwrap();
        stop_child(&mut server).await.unwrap();
    }

    #[tokio::test]
    async fn test_connect_slug_uses_stdio_by_default() {
        let _lock = acquire_process_guard().await;
        let _state = ProcessStateGuard::capture();
        let root = temp_dir("connect-rust-stdio");
        let slug = unique_slug("stdio");
        let marker = root.join("started.log");
        let listen_log = root.join("listen.log");
        let wrapper = write_stdio_wrapper_script(&root, &marker, &listen_log);

        write_holon(&root.join("holons/stdio"), &slug, &wrapper);
        env::set_current_dir(&root).unwrap();

        let channel = connect(&slug).await.unwrap();
        wait_for_file(&marker).await.unwrap();
        assert_eq!(wait_for_file(&listen_log).await.unwrap().trim(), "stdio://");
        assert!(!default_port_file_path(&slug).exists());

        disconnect(channel).await.unwrap();
    }

    #[tokio::test]
    async fn test_connect_slug_tcp_override_starts_binary_and_disconnect_stops_it() {
        let _lock = acquire_process_guard().await;
        let _state = ProcessStateGuard::capture();
        let root = temp_dir("connect-rust-slug");
        let slug = unique_slug("slug");
        let fixed_port = free_port();
        let marker = root.join("started.log");
        let wrapper = write_wrapper_script(&root, fixed_port, &marker);

        write_holon(&root.join("holons/slug"), &slug, &wrapper);
        env::set_current_dir(&root).unwrap();

        let channel = connect_with_mode(&slug, tcp_connect_options(), true)
            .await
            .unwrap();
        assert!(marker.is_file());
        wait_for_tcp_open(fixed_port).await.unwrap();

        disconnect(channel).await.unwrap();
        wait_for_tcp_closed(fixed_port).await.unwrap();
    }

    #[tokio::test]
    async fn test_connect_slug_tcp_override_reuses_live_port_file() {
        let _lock = acquire_process_guard().await;
        let _state = ProcessStateGuard::capture();
        let root = temp_dir("connect-rust-reuse");
        let slug = unique_slug("reuse");
        let fixed_port = free_port();
        let listen_uri = format!("tcp://127.0.0.1:{fixed_port}");
        let marker = root.join("started.log");
        let wrapper = write_wrapper_script(&root, fixed_port, &marker);

        write_holon(&root.join("holons/reuse"), &slug, &wrapper);
        env::set_current_dir(&root).unwrap();

        let server_bin = compiled_echo_server_binary();
        let mut server = start_server_process(&server_bin, &listen_uri).await;
        write_port_file(&default_port_file_path(&slug), &listen_uri).unwrap();

        let channel = connect_with_mode(&slug, tcp_connect_options(), true)
            .await
            .unwrap();
        disconnect(channel).await.unwrap();

        assert!(!marker.exists());
        wait_for_tcp_open(fixed_port).await.unwrap();
        stop_child(&mut server).await.unwrap();
    }

    #[tokio::test]
    async fn test_connect_slug_tcp_override_cleans_stale_port_file_and_starts_fresh() {
        let _lock = acquire_process_guard().await;
        let _state = ProcessStateGuard::capture();
        let root = temp_dir("connect-rust-stale");
        let slug = unique_slug("stale");
        let fixed_port = free_port();
        let marker = root.join("started.log");
        let wrapper = write_wrapper_script(&root, fixed_port, &marker);

        write_holon(&root.join("holons/stale"), &slug, &wrapper);
        env::set_current_dir(&root).unwrap();

        let stale_port = free_port();
        let stale_uri = format!("tcp://127.0.0.1:{stale_port}");
        let port_file = default_port_file_path(&slug);
        write_port_file(&port_file, &stale_uri).unwrap();

        let channel = connect_with_mode(&slug, tcp_connect_options(), true)
            .await
            .unwrap();
        assert!(marker.is_file());
        assert!(!port_file.exists());
        wait_for_tcp_open(fixed_port).await.unwrap();

        disconnect(channel).await.unwrap();
        wait_for_tcp_closed(fixed_port).await.unwrap();
    }

    #[tokio::test]
    async fn test_built_binary_describe_without_proto_files() {
        let _lock = acquire_process_guard().await;
        let _state = ProcessStateGuard::capture();
        let crate_dir = temp_dir("connect-rust-protoless-crate");
        write_protoless_holon_crate(&crate_dir);
        let cargo_manifest = crate_dir.join("Cargo.toml");

        let build = ProcessCommand::new("cargo")
            .arg("build")
            .arg("--manifest-path")
            .arg(&cargo_manifest)
            .output()
            .unwrap();
        if !build.status.success() {
            panic!(
                "cargo build failed:\nstdout:\n{}\nstderr:\n{}",
                String::from_utf8_lossy(&build.stdout),
                String::from_utf8_lossy(&build.stderr)
            );
        }

        let built_binary = crate_dir.join("target/debug/protoless-static-holon");
        assert!(built_binary.is_file(), "missing binary {}", built_binary.display());

        let runtime_dir = temp_dir("connect-rust-protoless");
        let runtime_binary = runtime_dir.join("protoless-static-holon");
        fs::copy(&built_binary, &runtime_binary).unwrap();
        let mut perms = fs::metadata(&runtime_binary).unwrap().permissions();
        perms.set_mode(0o755);
        fs::set_permissions(&runtime_binary, perms).unwrap();

        assert!(!runtime_dir.join("holon.proto").exists());
        assert!(!runtime_dir.join("api").exists());

        let mut command = Command::new(&runtime_binary);
        command.process_group(0);
        command
            .arg("serve")
            .arg("--listen")
            .arg("tcp://127.0.0.1:0")
            .current_dir(&runtime_dir)
            .stdin(Stdio::null())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped());

        let mut child = command.spawn().unwrap();
        let stdout = child.stdout.take().unwrap();
        let stderr = child.stderr.take().unwrap();
        let (line_tx, mut line_rx) = tokio::sync::mpsc::unbounded_channel::<String>();
        spawn_line_reader(stdout, line_tx.clone());
        spawn_line_reader(stderr, line_tx.clone());
        drop(line_tx);

        let deadline = Instant::now() + Duration::from_secs(15);
        let listen_uri = loop {
            if let Some(status) = child.try_wait().unwrap() {
                panic!("built binary exited before advertising an address: {status}");
            }
            if Instant::now() >= deadline {
                let _ = stop_child(&mut child).await;
                panic!("timed out waiting for built binary startup");
            }
            if let Ok(Some(line)) = time::timeout(Duration::from_millis(50), line_rx.recv()).await {
                if let Some(uri) = first_uri(&line) {
                    break uri;
                }
            }
        };

        let endpoint = format!("http://{}", listen_uri.trim_start_matches("tcp://"));
        let mut client = wait_for_holon_meta_client(&endpoint).await;
        let response = client
            .describe(DescribeRequest {})
            .await
            .unwrap()
            .into_inner();
        let identity = response.manifest.as_ref().unwrap().identity.as_ref().unwrap();

        assert_eq!(identity.given_name, "ProtoLess");
        assert_eq!(identity.family_name, "Holon");
        assert_eq!(response.services[0].name, "test.v1.Noop");

        stop_child(&mut child).await.unwrap();
    }

    #[test]
    fn test_normalize_direct_target_rejects_ws_wss_and_rest_sse() {
        for target in [
            "ws://127.0.0.1:8080/grpc",
            "wss://example.com:443/grpc",
            "rest+sse://127.0.0.1:8080",
        ] {
            let error = normalize_direct_target(target).unwrap_err();
            assert!(
                error.to_string().contains("unsupported direct target"),
                "unexpected error for {target:?}: {error}"
            );
        }
    }

    fn sdk_root() -> PathBuf {
        PathBuf::from(env!("CARGO_MANIFEST_DIR"))
    }

    fn write_protoless_holon_crate(root: &Path) {
        fs::create_dir_all(root.join("src")).unwrap();
        fs::write(
            root.join("Cargo.toml"),
            format!(
                "[package]\nname = \"protoless-static-holon\"\nversion = \"0.1.0\"\nedition = \"2021\"\n\n[dependencies]\nholons = {{ path = {:?} }}\ntokio = {{ version = \"1\", features = [\"macros\", \"rt-multi-thread\", \"signal\", \"net\"] }}\ntonic = {{ version = \"0.12\", features = [\"transport\"] }}\ntower-service = \"0.3\"\n",
                sdk_root()
            ),
        )
        .unwrap();
        fs::write(
            root.join("src/describe_generated.rs"),
            r#"use holons::gen::holons::v1::{
    holon_manifest::Identity,
    DescribeResponse, HolonManifest, MethodDoc, ServiceDoc,
};

pub fn static_describe_response() -> DescribeResponse {
    DescribeResponse {
        manifest: Some(HolonManifest {
            identity: Some(Identity {
                schema: "holon/v1".to_string(),
                uuid: "protoless-holon-0000".to_string(),
                given_name: "ProtoLess".to_string(),
                family_name: "Holon".to_string(),
                motto: "Serves Describe without adjacent proto files.".to_string(),
                composer: "connect-test".to_string(),
                status: "draft".to_string(),
                born: "2026-03-23".to_string(),
                version: "0.1.0".to_string(),
                aliases: vec![],
            }),
            description: String::new(),
            lang: "rust".to_string(),
            skills: vec![],
            contract: None,
            kind: "native".to_string(),
            platforms: vec![],
            transport: String::new(),
            build: None,
            requires: None,
            artifacts: None,
            sequences: vec![],
            guide: String::new(),
        }),
        services: vec![ServiceDoc {
            name: "test.v1.Noop".to_string(),
            description: "Proto-less static Describe test service.".to_string(),
            methods: vec![MethodDoc {
                name: "Ping".to_string(),
                description: "Returns unimplemented.".to_string(),
                input_type: String::new(),
                output_type: String::new(),
                input_fields: vec![],
                output_fields: vec![],
                client_streaming: false,
                server_streaming: false,
                example_input: String::new(),
            }],
        }],
    }
}
"#,
        )
        .unwrap();
        fs::write(
            root.join("src/main.rs"),
            r#"use std::convert::Infallible;
use std::task::{Context, Poll};

use holons::describe;
use tonic::body::{empty_body, BoxBody};
use tonic::codegen::http::{Request, Response};
use tonic::server::NamedService;
use tower_service::Service;

mod describe_generated;

#[derive(Clone, Default)]
struct UnimplementedService;

impl NamedService for UnimplementedService {
    const NAME: &'static str = "test.v1.Noop";
}

impl Service<Request<BoxBody>> for UnimplementedService {
    type Response = Response<BoxBody>;
    type Error = Infallible;
    type Future = std::future::Ready<Result<Self::Response, Self::Error>>;

    fn poll_ready(
        &mut self,
        _cx: &mut Context<'_>,
    ) -> Poll<Result<(), Self::Error>> {
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
        std::future::ready(Ok(response))
    }
}

#[tokio::main(flavor = "multi_thread")]
async fn main() -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let args: Vec<String> = std::env::args().skip(1).collect();
    let parsed = holons::serve::parse_options(&args);
    describe::use_static_response(describe_generated::static_describe_response());
    holons::serve::run_single_with_options(
        &parsed.listen_uri,
        UnimplementedService,
        holons::serve::RunOptions {
            reflect: parsed.reflect,
            ..holons::serve::RunOptions::default()
        },
    )
    .await?;
    Ok(())
}
"#,
        )
        .unwrap();
    }

    fn temp_dir(prefix: &str) -> PathBuf {
        let unique = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_nanos();
        let dir = env::temp_dir().join(format!("{prefix}-{unique}"));
        fs::create_dir_all(&dir).unwrap();
        dir
    }

    fn unique_slug(prefix: &str) -> String {
        let unique = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_nanos();
        format!("{prefix}-{unique}")
    }

    fn free_port() -> u16 {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        listener.local_addr().unwrap().port()
    }

    fn write_holon(dir: &Path, slug: &str, binary: &Path) {
        fs::create_dir_all(dir).unwrap();
        let (given_name, family_name) = slug_parts(slug);
        fs::write(
            dir.join("holon.proto"),
            format!(
                "syntax = \"proto3\";\n\npackage test.v1;\n\noption (holons.v1.manifest) = {{\n  identity: {{\n    uuid: \"{}\"\n    given_name: \"{}\"\n    family_name: \"{}\"\n    motto: \"Test\"\n    composer: \"test\"\n    clade: \"deterministic/pure\"\n    status: \"draft\"\n    born: \"2026-03-08\"\n  }}\n  lineage: {{\n    generated_by: \"test\"\n  }}\n  kind: \"native\"\n  build: {{\n    runner: \"shell\"\n  }}\n  artifacts: {{\n    binary: \"{}\"\n  }}\n}};\n",
                slug, given_name, family_name, binary.display()
            ),
        )
        .unwrap();
    }

    fn slug_parts(slug: &str) -> (&str, String) {
        if let Some((given, family)) = slug.split_once('-') {
            return (given, family.replace('-', " "));
        }
        (slug, String::new())
    }

    fn write_wrapper_script(root: &Path, fixed_port: u16, marker: &Path) -> PathBuf {
        let wrapper = root.join("echo-server-wrapper");
        let real_bin = compiled_echo_server_binary();
        let script = format!(
            "#!/usr/bin/env bash\nset -euo pipefail\nprintf 'started\\n' >> '{}'\nargs=()\nwhile (($#)); do\n  if [[ \"$1\" == \"--listen\" && $# -ge 2 && \"$2\" == \"tcp://127.0.0.1:0\" ]]; then\n    args+=(\"--listen\" \"tcp://127.0.0.1:{}\")\n    shift 2\n    continue\n  fi\n  args+=(\"$1\")\n  shift\ndone\nexec \"{}\" \"${{args[@]}}\"\n",
            marker.display(),
            fixed_port,
            real_bin.display()
        );
        fs::write(&wrapper, script).unwrap();
        let mut perms = fs::metadata(&wrapper).unwrap().permissions();
        perms.set_mode(0o755);
        fs::set_permissions(&wrapper, perms).unwrap();
        wrapper
    }

    fn write_stdio_wrapper_script(root: &Path, marker: &Path, listen_log: &Path) -> PathBuf {
        let wrapper = root.join("echo-server-stdio-wrapper");
        let real_bin = compiled_echo_server_binary();
        let script = format!(
            "#!/usr/bin/env bash\nset -euo pipefail\nprintf 'started\\n' >> '{}'\nlisten=''\nargs=()\nwhile (($#)); do\n  if [[ \"$1\" == \"--listen\" && $# -ge 2 ]]; then\n    listen=\"$2\"\n  fi\n  args+=(\"$1\")\n  shift\ndone\nprintf '%s\\n' \"$listen\" > '{}'\nexec \"{}\" \"${{args[@]}}\"\n",
            marker.display(),
            listen_log.display(),
            real_bin.display()
        );
        fs::write(&wrapper, script).unwrap();
        let mut perms = fs::metadata(&wrapper).unwrap().permissions();
        perms.set_mode(0o755);
        fs::set_permissions(&wrapper, perms).unwrap();
        wrapper
    }

    fn tcp_connect_options() -> ConnectOptions {
        ConnectOptions {
            transport: "tcp".to_string(),
            ..ConnectOptions::default()
        }
    }

    async fn start_server_process(binary: &Path, listen_uri: &str) -> Child {
        let mut command = Command::new(binary);
        #[cfg(unix)]
        command.process_group(0);
        command
            .arg("serve")
            .arg("--listen")
            .arg(listen_uri)
            .stdin(Stdio::null())
            .stdout(Stdio::null())
            .stderr(Stdio::null());
        let child = command.spawn().unwrap();
        let port = crate::transport::parse_uri(listen_uri)
            .unwrap()
            .port
            .unwrap();
        wait_for_tcp_open(port).await.unwrap();
        child
    }

    fn compiled_echo_server_binary() -> PathBuf {
        static BIN: OnceLock<PathBuf> = OnceLock::new();
        BIN.get_or_init(|| {
            let root = sdk_root();
            let build = ProcessCommand::new("cargo")
                .arg("build")
                .arg("--bin")
                .arg("echo-server")
                .current_dir(&root)
                .output()
                .unwrap();
            if !build.status.success() {
                panic!(
                    "cargo build --bin echo-server failed:\nstdout:\n{}\nstderr:\n{}",
                    String::from_utf8_lossy(&build.stdout),
                    String::from_utf8_lossy(&build.stderr)
                );
            }

            let binary = root.join("target/debug/echo-server");
            assert!(binary.is_file(), "missing built binary: {}", binary.display());
            binary
        })
        .clone()
    }

    async fn wait_for_tcp_open(port: u16) -> Result<()> {
        let deadline = Instant::now() + Duration::from_secs(10);
        loop {
            match TcpStream::connect(("127.0.0.1", port)).await {
                Ok(stream) => {
                    drop(stream);
                    return Ok(());
                }
                Err(err) if Instant::now() < deadline => {
                    let _ = err;
                    time::sleep(Duration::from_millis(50)).await;
                }
                Err(err) => return Err(err.into()),
            }
        }
    }

    async fn wait_for_file(path: &Path) -> Result<String> {
        let deadline = Instant::now() + Duration::from_secs(10);
        loop {
            match fs::read_to_string(path) {
                Ok(contents) => return Ok(contents),
                Err(err) if Instant::now() < deadline => {
                    let _ = err;
                    time::sleep(Duration::from_millis(25)).await;
                }
                Err(err) => return Err(err.into()),
            }
        }
    }

    async fn wait_for_tcp_closed(port: u16) -> Result<()> {
        let deadline = Instant::now() + Duration::from_secs(10);
        loop {
            match TcpStream::connect(("127.0.0.1", port)).await {
                Ok(stream) if Instant::now() < deadline => {
                    drop(stream);
                    time::sleep(Duration::from_millis(50)).await;
                }
                Ok(_) => return Err(boxed_err(format!("port {port} remained open"))),
                Err(_) => return Ok(()),
            }
        }
    }

    async fn wait_for_holon_meta_client(
        endpoint: &str,
    ) -> HolonMetaClient<tonic::transport::Channel> {
        let deadline = Instant::now() + Duration::from_secs(5);
        loop {
            match HolonMetaClient::connect(endpoint.to_string()).await {
                Ok(client) => return client,
                Err(err) if Instant::now() < deadline => {
                    let _ = err;
                    time::sleep(Duration::from_millis(25)).await;
                }
                Err(err) => panic!("timed out waiting for HolonMeta client: {err}"),
            }
        }
    }
}
