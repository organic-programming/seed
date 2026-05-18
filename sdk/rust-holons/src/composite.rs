//! Helpers for composite holons.

use crate::command_channel::{connect_stdio_transport, start_stdio_command_with_env, stop_child};
use crate::gen::holons::v1::{
    holon_meta_client::HolonMetaClient, holon_observability_client::HolonObservabilityClient,
    DescribeRequest, EventsRequest, LogsRequest,
};
use crate::{observability, transport};
use hyper_util::rt::TokioIo;
use std::collections::HashMap;
use std::env;
use std::fs;
use std::future::Future;
use std::io;
use std::path::{Path, PathBuf};
use std::pin::Pin;
use std::sync::{Mutex, OnceLock};
use std::task::{Context, Poll};
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};
use tokio::process::{Child, Command};
use tonic::codegen::http::Uri;
use tonic::transport::{Channel, Endpoint};
use tower_service::Service;

#[cfg(unix)]
use tokio::net::UnixStream;

/// Resolve a declared member's binary relative to the calling composite's
/// own executable.
pub fn member(id: &str) -> io::Result<PathBuf> {
    member_from_executable(&env::current_exe()?, id)
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ChildSpec {
    pub slug: String,
    pub binary: String,
}

#[derive(Debug, Clone, Copy, Default)]
pub struct DialOption {
    transitive_observability: Option<bool>,
}

pub fn with_transitive_observability(enabled: bool) -> DialOption {
    DialOption {
        transitive_observability: Some(enabled),
    }
}

#[allow(non_snake_case)]
pub fn WithTransitiveObservability(enabled: bool) -> DialOption {
    with_transitive_observability(enabled)
}

#[derive(Debug, Clone, Default)]
pub struct SpawnOptions {
    pub slug: String,
    pub binary_path: String,
    pub transport: String,
    pub instance_uid: String,
    pub downstream_chain: Vec<ChildSpec>,
    pub extra_env: HashMap<String, String>,
    pub dial_options: Vec<DialOption>,
}

pub struct SpawnedMember {
    pub slug: String,
    pub uid: String,
    pub listen_uri: String,
    pub conn: Channel,

    child: Option<Child>,
    relay: Option<observability::MemberRelay>,
}

impl SpawnedMember {
    pub async fn stop(&mut self) -> Result<(), String> {
        self.relay.take();
        self.conn = Endpoint::from_static("http://127.0.0.1:9").connect_lazy();
        if let Some(child) = self.child.as_mut() {
            stop_child(child)?;
        }
        self.child = None;
        Ok(())
    }
}

#[derive(Debug, Clone, Default)]
pub struct CascadeOptions {
    pub transport: String,
    pub members: Vec<ChildSpec>,
    pub extra_env: HashMap<String, String>,
}

pub struct Cascade {
    pub top: SpawnedMember,
}

impl Cascade {
    pub async fn stop(&mut self) -> Result<(), String> {
        self.top.stop().await
    }
}

pub const TRANSPORT_COVERAGE_SEQUENCE: [&str; 10] = [
    "stdio", "stdio", "tcp", "unix", "tcp", "tcp", "stdio", "unix", "unix", "stdio",
];

#[allow(non_upper_case_globals)]
pub const TransportCoverageSequence: [&str; 10] = TRANSPORT_COVERAGE_SEQUENCE;

pub async fn spawn_member(opts: SpawnOptions) -> Result<SpawnedMember, String> {
    let slug = opts_slug(&opts)?;
    let binary = opts.binary_path.trim();
    if binary.is_empty() {
        return Err(format!("spawn member {slug}: binary path is required"));
    }
    let uid = if opts.instance_uid.trim().is_empty() {
        new_instance_uid()
    } else {
        opts.instance_uid.trim().to_string()
    };
    let transport_name = if opts.transport.trim().is_empty() {
        "stdio".to_string()
    } else {
        opts.transport.trim().to_ascii_lowercase()
    };
    let (listen_uri, cleanup_path) = listen_uri_for_spawn(&transport_name, &uid)?;
    if let Some(path) = cleanup_path {
        let _ = fs::remove_file(path);
    }

    let mut args = vec![
        "serve".to_string(),
        "--listen".to_string(),
        listen_uri.clone(),
        "--transport".to_string(),
        transport_name.clone(),
    ];
    for child in &opts.downstream_chain {
        if child.slug.trim().is_empty() || child.binary.trim().is_empty() {
            return Err(format!(
                "spawn member {slug}: downstream child requires slug and binary"
            ));
        }
        args.push("--child".to_string());
        args.push(format!("{}={}", child.slug.trim(), child.binary.trim()));
    }

    let env = spawn_environment(&uid, &opts.extra_env);
    let binary_path = PathBuf::from(binary);
    let cwd = binary_path.parent();
    let child;
    let conn;
    let public_uri;
    if transport_name == "stdio" {
        let (transport, started) = start_stdio_command_with_env(&binary_path, &args, cwd, &env)?;
        child = started;
        conn = connect_stdio_transport(transport, 10_000).await?;
        describe_ready(conn.clone(), Duration::from_secs(10)).await?;
        public_uri = "stdio://".to_string();
    } else {
        let mut command = Command::new(&binary_path);
        #[cfg(unix)]
        command.process_group(0);
        command.args(&args);
        if let Some(cwd) = cwd {
            command.current_dir(cwd);
        }
        for (key, value) in &env {
            command.env(key, value);
        }
        child = command
            .spawn()
            .map_err(|err| format!("spawn member {slug}: {err}"))?;
        let meta = wait_spawn_meta(&spawn_run_root(&env), &slug, &uid, Duration::from_secs(10))
            .await
            .map_err(|err| format!("spawn member {slug}: {err}"))?;
        public_uri = meta.address;
        conn = dial_ready(&public_uri, Duration::from_secs(10))
            .await
            .map_err(|err| format!("spawn member {slug} dial {public_uri}: {err}"))?;
    }

    let mut member = SpawnedMember {
        slug: slug.clone(),
        uid: uid.clone(),
        listen_uri: public_uri,
        conn,
        child: Some(child),
        relay: None,
    };

    let transitive = apply_dial_options(&opts.dial_options).unwrap_or(true);
    if transitive {
        member.relay = Some(observability::MemberRelay::start(
            slug,
            uid,
            member.conn.clone(),
            observability::current(),
        ));
    }
    Ok(member)
}

#[allow(non_snake_case)]
pub async fn SpawnMember(opts: SpawnOptions) -> Result<SpawnedMember, String> {
    spawn_member(opts).await
}

pub async fn build_cascade(opts: CascadeOptions) -> Result<Cascade, String> {
    let Some(top) = opts.members.first() else {
        return Err("build cascade: at least one member is required".to_string());
    };
    let top = top.clone();
    let spawned = spawn_member(SpawnOptions {
        slug: top.slug,
        binary_path: top.binary,
        transport: opts.transport,
        downstream_chain: opts.members.iter().skip(1).cloned().collect(),
        extra_env: opts.extra_env,
        ..SpawnOptions::default()
    })
    .await?;
    Ok(Cascade { top: spawned })
}

#[allow(non_snake_case)]
pub async fn BuildCascade(opts: CascadeOptions) -> Result<Cascade, String> {
    build_cascade(opts).await
}

pub async fn dial(address: &str, opts: &[DialOption]) -> Result<Channel, String> {
    let conn = dial_ready(address, Duration::from_secs(10)).await?;
    let transitive = apply_dial_options(opts).unwrap_or(false);
    if transitive {
        let identity = resolve_relay_member_identity(conn.clone(), "").await?;
        let relay = observability::MemberRelay::start(
            identity.slug,
            identity.uid,
            conn.clone(),
            observability::current(),
        );
        dial_relays().lock().unwrap().push(relay);
    }
    Ok(conn)
}

#[allow(non_snake_case)]
pub async fn Dial(address: &str, opts: &[DialOption]) -> Result<Channel, String> {
    dial(address, opts).await
}

#[derive(Debug, Clone)]
pub struct CheckOutcome {
    pub pass: bool,
    pub evidence: String,
}

pub type ChainHop = observability::Hop;

#[derive(Debug, Clone, Default)]
pub struct LogCheckOptions {
    pub conn: Option<Channel>,
    pub sender: String,
    pub leaf_uid: String,
    pub expected_chain: Vec<ChainHop>,
    pub timeout: Duration,
    pub poll_interval: Duration,
    pub live: bool,
}

#[derive(Debug, Clone)]
pub struct EventCheckOptions {
    pub conn: Option<Channel>,
    pub event_name: String,
    pub leaf_uid: String,
    pub expected_chain: Vec<ChainHop>,
    pub timeout: Duration,
    pub poll_interval: Duration,
    pub live: bool,
}

impl Default for EventCheckOptions {
    fn default() -> Self {
        Self {
            conn: None,
            event_name: String::new(),
            leaf_uid: String::new(),
            expected_chain: Vec::new(),
            timeout: Duration::ZERO,
            poll_interval: Duration::ZERO,
            live: false,
        }
    }
}

pub async fn check_relayed_log(opts: LogCheckOptions) -> CheckOutcome {
    let timeout = nonzero_duration(opts.timeout, Duration::from_secs(3));
    let interval = nonzero_duration(opts.poll_interval, Duration::from_millis(100));
    let deadline = Instant::now() + timeout;
    let mut last = CheckOutcome {
        pass: false,
        evidence: String::new(),
    };
    loop {
        match read_log_entries(opts.conn.clone()).await {
            Ok(entries) => {
                last = match_relayed_log(&entries, &opts);
                if last.pass {
                    return last;
                }
            }
            Err(err) => {
                last.evidence = compact_check_evidence(&err);
            }
        }
        if Instant::now() >= deadline {
            return last;
        }
        tokio::time::sleep(interval).await;
    }
}

#[allow(non_snake_case)]
pub async fn CheckRelayedLog(opts: LogCheckOptions) -> CheckOutcome {
    check_relayed_log(opts).await
}

pub async fn check_relayed_event(opts: EventCheckOptions) -> CheckOutcome {
    let timeout = nonzero_duration(opts.timeout, Duration::from_secs(3));
    let interval = nonzero_duration(opts.poll_interval, Duration::from_millis(100));
    let deadline = Instant::now() + timeout;
    let mut last = CheckOutcome {
        pass: false,
        evidence: String::new(),
    };
    loop {
        match read_event_entries(opts.conn.clone()).await {
            Ok(events) => {
                last = match_relayed_event(&events, &opts);
                if last.pass {
                    return last;
                }
            }
            Err(err) => {
                last.evidence = compact_check_evidence(&err);
            }
        }
        if Instant::now() >= deadline {
            return last;
        }
        tokio::time::sleep(interval).await;
    }
}

#[allow(non_snake_case)]
pub async fn CheckRelayedEvent(opts: EventCheckOptions) -> CheckOutcome {
    check_relayed_event(opts).await
}

fn member_from_executable(self_path: &Path, id: &str) -> io::Result<PathBuf> {
    let id = id.trim();
    if id.is_empty() {
        return Err(io::Error::new(
            io::ErrorKind::InvalidInput,
            "member id is required",
        ));
    }
    let bin_dir = self_path.parent().ok_or_else(|| {
        io::Error::new(
            io::ErrorKind::InvalidInput,
            format!("executable has no parent: {}", self_path.display()),
        )
    })?;
    let member_dir = bin_dir.join("holons").join(id);
    for entry in fs::read_dir(&member_dir)? {
        let entry = entry?;
        let path = entry.path();
        if path.is_file() && is_executable(&path)? {
            return Ok(path);
        }
    }
    Err(io::Error::new(
        io::ErrorKind::NotFound,
        format!("no executable found in {}", member_dir.display()),
    ))
}

fn is_executable(path: &Path) -> io::Result<bool> {
    let meta = fs::metadata(path)?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        Ok(meta.permissions().mode() & 0o111 != 0)
    }
    #[cfg(windows)]
    {
        Ok(path
            .extension()
            .and_then(|ext| ext.to_str())
            .map(|ext| ext.eq_ignore_ascii_case("exe"))
            .unwrap_or(false))
    }
    #[cfg(not(any(unix, windows)))]
    {
        Ok(meta.is_file())
    }
}

fn opts_slug(opts: &SpawnOptions) -> Result<String, String> {
    if !opts.slug.trim().is_empty() {
        return Ok(opts.slug.trim().to_string());
    }
    Path::new(opts.binary_path.trim())
        .file_name()
        .map(|name| name.to_string_lossy().to_string())
        .filter(|name| !name.trim().is_empty())
        .ok_or_else(|| "spawn member: slug is required".to_string())
}

fn apply_dial_options(opts: &[DialOption]) -> Option<bool> {
    opts.iter()
        .filter_map(|opt| opt.transitive_observability)
        .last()
}

fn listen_uri_for_spawn(
    transport_name: &str,
    uid: &str,
) -> Result<(String, Option<PathBuf>), String> {
    match transport_name {
        "stdio" => Ok(("stdio://".to_string(), None)),
        "tcp" => Ok(("tcp://127.0.0.1:0".to_string(), None)),
        "unix" => {
            let path = env::temp_dir().join(format!("op-{}.sock", clean_socket_token(uid)));
            Ok((format!("unix://{}", path.display()), Some(path)))
        }
        other => Err(format!("unsupported transport {other:?}")),
    }
}

fn spawn_environment(uid: &str, extra: &HashMap<String, String>) -> Vec<(String, String)> {
    let mut env_map: HashMap<String, String> = env::vars().collect();
    env_map.insert("OP_INSTANCE_UID".to_string(), uid.to_string());
    env_map.insert("OP_RUN_DIR".to_string(), run_root_from_env(&env_map));
    env_map.insert(
        "HOLONS_PARENT_PID".to_string(),
        std::process::id().to_string(),
    );
    if let Some(families) = active_observability_families(&env_map) {
        env_map.insert("OP_OBS".to_string(), families);
    }
    for (key, value) in extra {
        env_map.insert(key.clone(), value.clone());
    }
    env_map.into_iter().collect()
}

fn active_observability_families(env_map: &HashMap<String, String>) -> Option<String> {
    let obs = observability::current();
    let mut families = Vec::new();
    for (family, name) in [
        (observability::Family::Logs, "logs"),
        (observability::Family::Metrics, "metrics"),
        (observability::Family::Events, "events"),
        (observability::Family::Prom, "prom"),
    ] {
        if obs.enabled(family) {
            families.push(name);
        }
    }
    if !families.is_empty() {
        return Some(families.join(","));
    }
    env_map
        .get("OP_OBS")
        .map(|value| value.trim().to_string())
        .filter(|value| !value.is_empty())
}

fn run_root_from_env(env_map: &HashMap<String, String>) -> String {
    if let Some(root) = env_map
        .get("OP_RUN_DIR")
        .filter(|value| !value.trim().is_empty())
    {
        return root.clone();
    }
    if let Some(root) = env_map
        .get("OPPATH")
        .filter(|value| !value.trim().is_empty())
    {
        return Path::new(root).join("run").to_string_lossy().into_owned();
    }
    if let Some(home) = env_map.get("HOME").filter(|value| !value.trim().is_empty()) {
        return Path::new(home)
            .join(".op")
            .join("run")
            .to_string_lossy()
            .into_owned();
    }
    env::temp_dir()
        .join(".op")
        .join("run")
        .to_string_lossy()
        .into_owned()
}

fn spawn_run_root(env_values: &[(String, String)]) -> String {
    let env_map: HashMap<String, String> = env_values.iter().cloned().collect();
    run_root_from_env(&env_map)
}

struct SpawnMeta {
    address: String,
}

async fn wait_spawn_meta(
    run_root: &str,
    slug: &str,
    uid: &str,
    timeout: Duration,
) -> Result<SpawnMeta, String> {
    let deadline = Instant::now() + timeout;
    let path = Path::new(run_root).join(slug).join(uid).join("meta.json");
    loop {
        let error = match fs::read_to_string(&path)
            .map_err(|err| err.to_string())
            .and_then(|body| {
                let value: serde_json::Value =
                    serde_json::from_str(&body).map_err(|err| err.to_string())?;
                let meta_uid = value
                    .get("uid")
                    .and_then(|v| v.as_str())
                    .unwrap_or_default();
                let address = value
                    .get("address")
                    .and_then(|v| v.as_str())
                    .unwrap_or_default();
                if meta_uid == uid && !address.is_empty() {
                    Ok(SpawnMeta {
                        address: address.to_string(),
                    })
                } else {
                    Err("meta not ready".to_string())
                }
            }) {
            Ok(meta) => return Ok(meta),
            Err(err) => err,
        };
        if Instant::now() >= deadline {
            return Err(format!(
                "meta not ready for {slug}/{uid} at {}: {}",
                path.display(),
                error
            ));
        }
        tokio::time::sleep(Duration::from_millis(50)).await;
    }
}

async fn dial_ready(address: &str, timeout: Duration) -> Result<Channel, String> {
    let deadline = Instant::now() + timeout;
    loop {
        let error = match dial_address(address).await {
            Ok(channel) => match describe_ready(channel.clone(), Duration::from_secs(1)).await {
                Ok(()) => return Ok(channel),
                Err(err) => err,
            },
            Err(err) => err,
        };
        if Instant::now() >= deadline {
            return Err(error);
        }
        tokio::time::sleep(Duration::from_millis(50)).await;
    }
}

async fn dial_address(address: &str) -> Result<Channel, String> {
    let trimmed = address.trim();
    if trimmed.is_empty() {
        return Err("dial address is required".to_string());
    }
    if trimmed.starts_with("stdio://") {
        return Err(
            "composite::dial does not support stdio addresses; use spawn_member".to_string(),
        );
    }
    let parsed = if trimmed.contains("://") {
        transport::parse_uri(trimmed)?
    } else if valid_host_port(trimmed) {
        transport::parse_uri(&format!("tcp://{trimmed}"))?
    } else {
        return Err(format!(
            "dial address must be tcp://host:port, unix:///path, or host:port: {address:?}"
        ));
    };
    match parsed.scheme.as_str() {
        "tcp" => {
            let host = match parsed.host.as_deref() {
                Some("") | Some("0.0.0.0") | Some("::") | None => "127.0.0.1",
                Some(host) => host,
            };
            let port = parsed
                .port
                .ok_or_else(|| format!("invalid tcp target {address:?}: missing port"))?;
            Endpoint::from_shared(format!("http://{host}:{port}"))
                .map_err(|err| err.to_string())?
                .connect()
                .await
                .map_err(|err| err.to_string())
        }
        #[cfg(unix)]
        "unix" => {
            let path = parsed
                .path
                .ok_or_else(|| format!("invalid unix target {address:?}: missing path"))?;
            Endpoint::from_static("http://[::]:50051")
                .connect_with_connector(UnixConnector {
                    path: PathBuf::from(path),
                })
                .await
                .map_err(|err| err.to_string())
        }
        other => Err(format!("unsupported dial address scheme {other:?}")),
    }
}

async fn describe_ready(channel: Channel, timeout: Duration) -> Result<(), String> {
    let deadline = Instant::now() + timeout;
    loop {
        let mut client = HolonMetaClient::new(channel.clone());
        let error = match client.describe(DescribeRequest {}).await {
            Ok(_) => return Ok(()),
            Err(err) => err.to_string(),
        };
        if Instant::now() >= deadline {
            return Err(error);
        }
        tokio::time::sleep(Duration::from_millis(50)).await;
    }
}

#[derive(Clone)]
struct RelayIdentity {
    slug: String,
    uid: String,
}

async fn resolve_relay_member_identity(
    channel: Channel,
    fallback_slug: &str,
) -> Result<RelayIdentity, String> {
    let mut client = HolonObservabilityClient::new(channel.clone());
    if let Ok(response) = client
        .events(EventsRequest {
            event_names: vec![observability::EVENT_INSTANCE_READY.to_string()],
            since: None,
            follow: false,
        })
        .await
    {
        let mut stream = response.into_inner();
        while let Some(event) = stream.message().await.map_err(|err| err.to_string())? {
            let uid = observability::string_attribute(
                &event.attributes,
                observability::ATTR_HOLONS_INSTANCE_UID,
            );
            if uid.trim().is_empty() || !event.chain.is_empty() {
                continue;
            }
            let wire_slug =
                observability::string_attribute(&event.attributes, observability::ATTR_HOLONS_SLUG);
            let slug = if wire_slug.trim().is_empty() {
                fallback_slug.to_string()
            } else {
                wire_slug.trim().to_string()
            };
            if !slug.is_empty() {
                return Ok(RelayIdentity { slug, uid });
            }
        }
    }

    let mut client = HolonObservabilityClient::new(channel);
    if let Ok(response) = client
        .logs(LogsRequest {
            min_severity_number: observability::Level::Info as i32,
            follow: false,
            ..LogsRequest::default()
        })
        .await
    {
        let mut stream = response.into_inner();
        while let Some(entry) = stream.message().await.map_err(|err| err.to_string())? {
            let uid = observability::string_attribute(
                &entry.attributes,
                observability::ATTR_HOLONS_INSTANCE_UID,
            );
            if uid.trim().is_empty() || !entry.chain.is_empty() {
                continue;
            }
            let wire_slug =
                observability::string_attribute(&entry.attributes, observability::ATTR_HOLONS_SLUG);
            let slug = if wire_slug.trim().is_empty() {
                fallback_slug.to_string()
            } else {
                wire_slug.trim().to_string()
            };
            if !slug.is_empty() {
                return Ok(RelayIdentity { slug, uid });
            }
        }
    }
    Err("resolve relay identity: peer did not expose a local log or event with slug and instance_uid".to_string())
}

async fn read_log_entries(conn: Option<Channel>) -> Result<Vec<observability::LogRecord>, String> {
    if let Some(channel) = conn {
        let mut client = HolonObservabilityClient::new(channel);
        let mut stream = client
            .logs(LogsRequest {
                min_severity_number: observability::Level::Info as i32,
                follow: false,
                ..LogsRequest::default()
            })
            .await
            .map_err(|err| err.to_string())?
            .into_inner();
        let mut out = Vec::new();
        while let Some(entry) = stream.message().await.map_err(|err| err.to_string())? {
            out.push(observability::from_proto_log_record(entry));
        }
        return Ok(out);
    }
    observability::current()
        .log_ring
        .as_ref()
        .map(|ring| ring.drain())
        .ok_or_else(|| "logs family is not enabled".to_string())
}

async fn read_event_entries(
    conn: Option<Channel>,
) -> Result<Vec<observability::LogRecord>, String> {
    if let Some(channel) = conn {
        let mut client = HolonObservabilityClient::new(channel);
        let mut stream = client
            .events(EventsRequest {
                follow: false,
                ..EventsRequest::default()
            })
            .await
            .map_err(|err| err.to_string())?
            .into_inner();
        let mut out = Vec::new();
        while let Some(event) = stream.message().await.map_err(|err| err.to_string())? {
            out.push(observability::from_proto_log_record(event));
        }
        return Ok(out);
    }
    observability::current()
        .event_bus
        .as_ref()
        .map(|bus| bus.drain())
        .ok_or_else(|| "events family is not enabled".to_string())
}

fn match_relayed_log(entries: &[observability::LogRecord], opts: &LogCheckOptions) -> CheckOutcome {
    for entry in entries {
        if entry.message != "tick received" {
            continue;
        }
        if entry.attr_string("sender").as_deref() != Some(opts.sender.as_str())
            || entry.attr_string("responder_uid").as_deref() != Some(opts.leaf_uid.as_str())
        {
            continue;
        }
        if let Some(evidence) = compare_chain(&entry.chain, &opts.expected_chain) {
            return CheckOutcome {
                pass: false,
                evidence: compact_check_evidence(&format!("matching log bad chain: {evidence}")),
            };
        }
        return CheckOutcome {
            pass: true,
            evidence: String::new(),
        };
    }
    CheckOutcome {
        pass: false,
        evidence: compact_check_evidence(&format!(
            "no relayed tick log sender={} leaf_uid={} entries={}",
            opts.sender,
            opts.leaf_uid,
            entries.len()
        )),
    }
}

fn match_relayed_event(
    events: &[observability::LogRecord],
    opts: &EventCheckOptions,
) -> CheckOutcome {
    let event_name = if opts.event_name.trim().is_empty() {
        observability::EVENT_INSTANCE_READY
    } else {
        opts.event_name.as_str()
    };
    for event in events {
        if event.event_name != event_name || event.instance_uid != opts.leaf_uid {
            continue;
        }
        if let Some(evidence) = compare_chain(&event.chain, &opts.expected_chain) {
            return CheckOutcome {
                pass: false,
                evidence: compact_check_evidence(&format!("matching event bad chain: {evidence}")),
            };
        }
        return CheckOutcome {
            pass: true,
            evidence: String::new(),
        };
    }
    CheckOutcome {
        pass: false,
        evidence: compact_check_evidence(&format!(
            "no relayed INSTANCE_READY event leaf_uid={} events={}",
            opts.leaf_uid,
            events.len()
        )),
    }
}

fn compare_chain(got: &[ChainHop], want: &[ChainHop]) -> Option<String> {
    if got.len() != want.len() {
        return Some(format!("chain length {} want {}", got.len(), want.len()));
    }
    for (idx, (got, want)) in got.iter().zip(want.iter()).enumerate() {
        if got.slug != want.slug || got.instance_uid != want.instance_uid {
            return Some(format!(
                "hop {idx}={}/{} want {}/{}",
                got.slug, got.instance_uid, want.slug, want.instance_uid
            ));
        }
    }
    None
}

fn compact_check_evidence(value: &str) -> String {
    let compact = value.split_whitespace().collect::<Vec<_>>().join(" ");
    if compact.len() <= 240 {
        compact
    } else {
        format!("{}...", &compact[..240])
    }
}

fn nonzero_duration(value: Duration, fallback: Duration) -> Duration {
    if value.is_zero() {
        fallback
    } else {
        value
    }
}

fn valid_host_port(value: &str) -> bool {
    value.rsplit_once(':').is_some_and(|(host, port)| {
        !host.trim().is_empty() && !port.trim().is_empty() && port.parse::<u16>().is_ok()
    })
}

fn new_instance_uid() -> String {
    let nanos = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_nanos();
    format!("{}-{nanos}", std::process::id())
}

fn clean_socket_token(value: &str) -> String {
    let mut out = String::new();
    for ch in value.trim().chars().take(24) {
        if ch.is_ascii_alphanumeric() || ch == '-' || ch == '_' {
            out.push(ch);
        } else {
            out.push('_');
        }
    }
    if out.is_empty() {
        new_instance_uid()
    } else {
        out
    }
}

fn dial_relays() -> &'static Mutex<Vec<observability::MemberRelay>> {
    static RELAYS: OnceLock<Mutex<Vec<observability::MemberRelay>>> = OnceLock::new();
    RELAYS.get_or_init(|| Mutex::new(Vec::new()))
}

#[cfg(unix)]
#[derive(Clone)]
struct UnixConnector {
    path: PathBuf,
}

#[cfg(unix)]
impl Service<Uri> for UnixConnector {
    type Response = TokioIo<UnixStream>;
    type Error = std::io::Error;
    type Future =
        Pin<Box<dyn Future<Output = std::io::Result<TokioIo<UnixStream>>> + Send + 'static>>;

    fn poll_ready(&mut self, _cx: &mut Context<'_>) -> Poll<std::io::Result<()>> {
        Poll::Ready(Ok(()))
    }

    fn call(&mut self, _uri: Uri) -> Self::Future {
        let path = self.path.clone();
        Box::pin(async move { UnixStream::connect(path).await.map(TokioIo::new) })
    }
}

#[cfg(test)]
mod tests {
    use super::member_from_executable;
    use std::fs;

    #[cfg(unix)]
    use std::os::unix::fs::PermissionsExt;

    #[test]
    fn resolves_embedded_member_binary() {
        let root = tempfile::tempdir().unwrap();
        let bin_dir = root.path().join("composite.holon/bin/darwin_arm64");
        let member_dir = bin_dir.join("holons/node-a");
        fs::create_dir_all(&member_dir).unwrap();
        let self_path = bin_dir.join("composite");
        fs::write(&self_path, b"composite").unwrap();
        fs::write(member_dir.join("README.txt"), b"not executable").unwrap();
        let member = member_dir.join("node-a-bin");
        fs::write(&member, b"member").unwrap();
        #[cfg(unix)]
        fs::set_permissions(&member, fs::Permissions::from_mode(0o755)).unwrap();

        let got = member_from_executable(&self_path, "node-a").unwrap();
        assert_eq!(got, member);
    }

    #[test]
    fn rejects_empty_member_id() {
        let err = member_from_executable(std::path::Path::new("/tmp/composite"), " ")
            .expect_err("empty id should fail");
        assert_eq!(err.kind(), std::io::ErrorKind::InvalidInput);
    }
}
