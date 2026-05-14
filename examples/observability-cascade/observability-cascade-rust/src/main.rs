use anyhow::{anyhow, Context, Result};
use observability_cascade_node_rust::gen::rust::relay::v1::{
    relay_service_client::RelayServiceClient, TickRequest,
};
use holons::gen::holons::v1::{
    holon_meta_client::HolonMetaClient, holon_observability_client::HolonObservabilityClient,
    ChainHop, DescribeRequest, EventInfo, EventType, EventsRequest, LogEntry, LogLevel,
    LogsRequest,
};
use hyper_util::rt::TokioIo;
use serde::Deserialize;
use std::collections::HashMap;
use std::env;
use std::fs;
use std::future::Future;
use std::io::{BufRead, BufReader};
use std::path::{Path, PathBuf};
use std::pin::Pin;
use std::process::{Child, Command, Stdio};
use std::sync::{Arc, Mutex, Once};
use std::task::{Context as TaskContext, Poll};
use std::time::{Duration, Instant};
use tokio::net::UnixStream;
use tokio::task::JoinHandle;
use tonic::{Request, Response, Status};
use tonic::codegen::http::Uri;
use tonic::transport::{Channel, Endpoint};
use tower_service::Service;

mod gen {
    pub mod describe_generated {
        include!(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/gen/describe_generated.rs"
        ));
    }

    pub mod rust {
        pub mod observability_cascade {
            pub mod v1 {
                include!(concat!(
                    env!("CARGO_MANIFEST_DIR"),
                    "/gen/rust/observability_cascade/v1/observability_cascade.v1.rs"
                ));
            }
        }
    }
}

use gen::rust::observability_cascade::v1 as cascadepb;

const GO_SLUG: &str = "observability-cascade-node-go";
const RUST_SLUG: &str = "observability-cascade-node-rust";
const RUN_TICKS: usize = 3;
const RUN_PHASES: usize = 4;
const ROLE_ORDER: [&str; 4] = ["D", "C", "B", "A"];
const OBSERVABILITY_CASCADE_DESCRIPTOR_SET: &[u8] = include_bytes!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/gen/rust/observability_cascade/v1/observability_cascade_descriptor.bin"
));

macro_rules! outputln {
    ($emit:expr) => {
        if $emit {
            println!();
        }
    };
    ($emit:expr, $($arg:tt)*) => {
        if $emit {
            println!($($arg)*);
        }
    };
}

#[derive(Clone)]
struct RoleSpec {
    slug: String,
    binary_path: PathBuf,
}

struct RoleRuntime {
    role: String,
    uid: String,
    slug: String,
    binary_path: PathBuf,
    listen_uris: Vec<String>,
    relay_address: String,
    member_address: String,
    member_slug: String,
    client_target: String,
    metrics_addr: String,
    child: Option<Child>,
    channel: Option<Channel>,
    stdout: Arc<Mutex<String>>,
    stderr: Arc<Mutex<String>>,
}

struct Cascade {
    phase: usize,
    transport: String,
    roles: HashMap<String, RoleRuntime>,
}

struct CheckResult {
    pass: bool,
    evidence: String,
}

struct TickOutcome {
    log: CheckResult,
    event: CheckResult,
    metric: CheckResult,
    metric_value: f64,
}

struct CascadePattern {
    name: &'static str,
    roles: HashMap<String, RoleSpec>,
}

struct LiveStreams {
    logs: Arc<Mutex<Vec<LogEntry>>>,
    events: Arc<Mutex<Vec<EventInfo>>>,
    errors: Arc<Mutex<Vec<String>>>,
    tasks: Vec<JoinHandle<()>>,
}

struct PhaseReport {
    name: String,
    pass: i32,
    fail: i32,
    failures: Vec<String>,
}

struct CascadeReportData {
    ticks: i32,
    pass: i32,
    fail: i32,
    phases: Vec<PhaseReport>,
}

struct MultiPatternReportData {
    patterns: Vec<CascadeReportData>,
    total_pass: i32,
    total_fail: i32,
}

#[derive(Default)]
struct ObservabilityCascadeRpc;

#[derive(Clone)]
struct UnixConnector {
    path: Arc<PathBuf>,
}

#[derive(Deserialize)]
struct MetaJson {
    uid: String,
    metrics_addr: String,
}

#[tokio::main]
async fn main() {
    let args: Vec<String> = env::args().skip(1).collect();
    let result = if args
        .first()
        .map(|arg| canonical_command(arg) == "serve")
        .unwrap_or(false)
    {
        serve_composite(&args[1..]).await.map_err(|err| anyhow!(err))
    } else if args.iter().any(|arg| arg == "--multi-pattern") {
        run_multi_pattern(true).await.map(|_| ())
    } else if args.iter().any(|arg| arg == "--live-stream") {
        run_live_stream(true).await.map(|_| ())
    } else {
        run_default(true).await.map(|_| ())
    };
    if let Err(err) = result {
        eprintln!("\nFAIL: {err:#}");
        std::process::exit(1);
    }
}

#[tonic::async_trait]
impl cascadepb::observability_cascade_service_server::ObservabilityCascadeService
    for ObservabilityCascadeRpc
{
    async fn run_default(
        &self,
        _request: Request<cascadepb::RunRequest>,
    ) -> Result<Response<cascadepb::CascadeReport>, Status> {
        run_default(false)
            .await
            .map(to_cascade_report)
            .map(Response::new)
            .map_err(to_status)
    }

    async fn run_live_stream(
        &self,
        _request: Request<cascadepb::RunRequest>,
    ) -> Result<Response<cascadepb::CascadeReport>, Status> {
        run_live_stream(false)
            .await
            .map(to_cascade_report)
            .map(Response::new)
            .map_err(to_status)
    }

    async fn run_multi_pattern(
        &self,
        _request: Request<cascadepb::RunRequest>,
    ) -> Result<Response<cascadepb::MultiPatternReport>, Status> {
        run_multi_pattern(false)
            .await
            .map(to_multi_pattern_report)
            .map(Response::new)
            .map_err(to_status)
    }
}

async fn serve_composite(args: &[String]) -> holons::serve::Result<()> {
    register_static_describe();
    let parsed = holons::serve::parse_options(args);
    holons::serve::run_single_with_options(
        &parsed.listen_uri,
        cascadepb::observability_cascade_service_server::ObservabilityCascadeServiceServer::new(
            ObservabilityCascadeRpc,
        ),
        holons::serve::RunOptions {
            reflect: parsed.reflect,
            descriptor_set: Some(OBSERVABILITY_CASCADE_DESCRIPTOR_SET.to_vec()),
            ..holons::serve::RunOptions::default()
        },
    )
    .await
}

fn register_static_describe() {
    static INIT: Once = Once::new();
    INIT.call_once(|| {
        holons::describe::use_static_response(
            gen::describe_generated::static_describe_response(),
        );
    });
}

fn to_status(error: anyhow::Error) -> Status {
    Status::internal(format!("{error:#}"))
}

async fn run_default(emit: bool) -> Result<CascadeReportData> {
    let binary_path = find_cascade_node_binary()?;
    let run_root = run_root();
    let transports = ["tcp", "unix", "tcp", "unix"];

    outputln!(emit, "=== observability-cascade-rust ===");
    outputln!(emit);

    let mut total_pass = 0;
    let mut total_fail = 0;
    let mut previous = "";
    for (phase_idx, transport) in transports.iter().enumerate() {
        let phase_no = phase_idx + 1;
        if previous.is_empty() {
            outputln!(emit, "Phase {phase_no}/{RUN_PHASES}: transport={transport}");
        } else if phase_no == RUN_PHASES && *transport == transports[0] {
            outputln!(
                emit,
                "Phase {phase_no}/{RUN_PHASES}: transport={transport} (cycle wrap)"
            );
        } else {
            outputln!(
                emit,
                "Phase {phase_no}/{RUN_PHASES}: transport={transport} (switching from {previous})"
            );
        }

        let spawn_start = Instant::now();
        let mut cascade =
            match spawn_cascade(phase_no, transport, binary_path.clone(), &run_root).await {
                Ok(cascade) => cascade,
                Err(err) => {
                    total_fail += RUN_TICKS;
                    outputln!(emit, "  spawn FAIL: {err}\n");
                    previous = transport;
                    continue;
                }
            };
        outputln!(emit, "  spawned 4 nodes in {}", elapsed(spawn_start));

        let mut previous_metric = 0.0;
        for tick in 1..=RUN_TICKS {
            let tick_start = Instant::now();
            let result = cascade.run_tick(tick, previous_metric).await;
            if result.metric.pass {
                previous_metric = result.metric_value;
            }
            let overall = result.log.pass && result.event.pass && result.metric.pass;
            if overall {
                total_pass += 1;
            } else {
                total_fail += 1;
            }
            outputln!(
                emit,
                "  Tick {tick}/{RUN_TICKS}: log {}, event {}, metric {} (overall {} in {})",
                pass_text(result.log.pass),
                pass_text(result.event.pass),
                pass_text(result.metric.pass),
                pass_text(overall),
                elapsed(tick_start),
            );
            if emit && !overall {
                print_failure_evidence("log", &result.log);
                print_failure_evidence("event", &result.event);
                print_failure_evidence("metric", &result.metric);
            }
        }
        cascade.stop().await;
        outputln!(emit);
        previous = transport;
    }

    outputln!(
        emit,
        "Summary: {} ticks, {} PASS, {} FAIL",
        total_pass + total_fail,
        total_pass,
        total_fail
    );
    if total_fail > 0 {
        return Err(anyhow!("{total_fail} tick(s) failed"));
    }
    Ok(CascadeReportData {
        ticks: (total_pass + total_fail) as i32,
        pass: total_pass as i32,
        fail: total_fail as i32,
        phases: vec![PhaseReport {
            name: "default".to_string(),
            pass: total_pass as i32,
            fail: total_fail as i32,
            failures: Vec::new(),
        }],
    })
}

async fn run_live_stream(emit: bool) -> Result<CascadeReportData> {
    let binary_path = find_cascade_node_binary()?;
    let run_root = run_root();
    let transports = ["tcp", "unix", "tcp", "unix"];

    outputln!(emit, "=== observability-cascade-rust --live-stream ===");
    outputln!(emit);
    outputln!(emit, "Setup: opening long-lived Follow:true streams on A");
    outputln!(emit, "       (initial transport: tcp, port 9090)");
    outputln!(emit);

    let mut total_pass = 0;
    let mut total_fail = 0;
    let mut cascade: Option<Cascade> = None;
    let mut streams: Option<LiveStreams> = None;
    for (phase_idx, transport) in transports.iter().enumerate() {
        let phase_no = phase_idx + 1;
        if phase_no == 1 {
            outputln!(
                emit,
                "Phase {phase_no}/{RUN_PHASES}: initial chain ({transport})"
            );
        } else {
            outputln!(emit, "Phase {phase_no}/{RUN_PHASES}: respawn on {transport}");
            let kill_start = Instant::now();
            if let Some(mut s) = streams.take() {
                s.stop().await;
            }
            if let Some(mut c) = cascade.take() {
                c.stop().await;
            }
            outputln!(emit, "  killed 4 nodes in {}", elapsed(kill_start));
        }

        let spawn_start = Instant::now();
        let phase_cascade =
            match spawn_cascade(phase_no, transport, binary_path.clone(), &run_root).await {
                Ok(cascade) => cascade,
                Err(err) => {
                    total_fail += RUN_TICKS;
                    outputln!(emit, "  spawn FAIL: {err}\n");
                    streams = None;
                    continue;
                }
            };
        outputln!(emit, "  spawned 4 nodes in {}", elapsed(spawn_start));
        if phase_no > 1 {
            outputln!(emit, "  re-opening Follow:true streams on new A");
        }
        let stream_open_error =
            match start_live_streams(phase_cascade.roles["A"].channel.as_ref().unwrap().clone())
                .await
            {
                Ok(s) => {
                    streams = Some(s);
                    None
                }
                Err(err) => {
                    streams = None;
                    outputln!(emit, "  stream re-open failed: {err}");
                    Some(err.to_string())
                }
            };

        let mut previous_metric = 0.0;
        for tick in 1..=RUN_TICKS {
            let tick_start = Instant::now();
            let result = phase_cascade
                .run_live_tick(
                    streams.as_ref(),
                    stream_open_error.as_deref(),
                    tick,
                    previous_metric,
                )
                .await;
            if result.metric.pass {
                previous_metric = result.metric_value;
            }
            let overall = result.log.pass && result.event.pass && result.metric.pass;
            if overall {
                total_pass += 1;
            } else {
                total_fail += 1;
            }
            outputln!(
                emit,
                "  Tick {tick}/{RUN_TICKS}: log {}, event {}, metric {} (overall {} in {})",
                pass_text(result.log.pass),
                pass_text(result.event.pass),
                pass_text(result.metric.pass),
                pass_text(overall),
                elapsed(tick_start),
            );
            if emit && !overall {
                print_failure_evidence("log", &result.log);
                print_failure_evidence("event", &result.event);
                print_failure_evidence("metric", &result.metric);
            }
        }
        outputln!(emit);
        cascade = Some(phase_cascade);
    }

    if let Some(mut s) = streams {
        s.stop().await;
    }
    if let Some(mut c) = cascade {
        c.stop().await;
    }

    outputln!(
        emit,
        "Summary: {total_pass} PASS / {total_fail} FAIL across {} ticks",
        total_pass + total_fail
    );
    if total_fail > 0 {
        return Err(anyhow!("{total_fail} tick(s) failed"));
    }
    Ok(CascadeReportData {
        ticks: (total_pass + total_fail) as i32,
        pass: total_pass as i32,
        fail: total_fail as i32,
        phases: vec![PhaseReport {
            name: "live-stream".to_string(),
            pass: total_pass as i32,
            fail: total_fail as i32,
            failures: Vec::new(),
        }],
    })
}

async fn run_multi_pattern(emit: bool) -> Result<MultiPatternReportData> {
    let rust_binary = find_holon_binary(RUST_SLUG)?;
    let go_binary = find_holon_binary(GO_SLUG)?;
    let patterns = vec![
        CascadePattern {
            name: "rust-rust-rust-rust",
            roles: pattern_roles([
                ("A", RUST_SLUG, rust_binary.clone()),
                ("B", RUST_SLUG, rust_binary.clone()),
                ("C", RUST_SLUG, rust_binary.clone()),
                ("D", RUST_SLUG, rust_binary.clone()),
            ]),
        },
        CascadePattern {
            name: "rust-rust-go-rust",
            roles: pattern_roles([
                ("A", RUST_SLUG, rust_binary.clone()),
                ("B", RUST_SLUG, rust_binary.clone()),
                ("C", GO_SLUG, go_binary.clone()),
                ("D", RUST_SLUG, rust_binary.clone()),
            ]),
        },
        CascadePattern {
            name: "rust-rust-go-go",
            roles: pattern_roles([
                ("A", RUST_SLUG, rust_binary.clone()),
                ("B", RUST_SLUG, rust_binary.clone()),
                ("C", GO_SLUG, go_binary.clone()),
                ("D", GO_SLUG, go_binary.clone()),
            ]),
        },
    ];
    let run_root = run_root();
    let transports = ["tcp", "unix", "tcp", "unix"];

    outputln!(emit, "=== observability-cascade-rust (multi-pattern) ===");
    outputln!(emit);

    let mut total_pass = 0;
    let mut total_fail = 0;
    for (pattern_idx, pattern) in patterns.iter().enumerate() {
        outputln!(
            emit,
            "Pattern {}/{}: {}",
            pattern_idx + 1,
            patterns.len(),
            pattern.name
        );
        let mut pattern_pass = 0;
        for (phase_idx, transport) in transports.iter().enumerate() {
            let phase_no = phase_idx + 1;
            let spawn_start = Instant::now();
            let mut cascade =
                match spawn_pattern_cascade(phase_no, transport, pattern.roles.clone(), &run_root)
                    .await
                {
                    Ok(cascade) => cascade,
                    Err(err) => {
                        total_fail += RUN_TICKS;
                        outputln!(
                            emit,
                            "  Phase {phase_no}/{RUN_PHASES} ({transport}): spawn FAIL ({err})"
                        );
                        continue;
                    }
                };

            let mut stream_open_error: Option<String> = None;
            let mut streams = match start_live_streams(
                cascade.roles["A"].channel.as_ref().unwrap().clone(),
            )
            .await
            {
                Ok(streams) => {
                    let ready =
                        wait_for_every(Duration::from_secs(5), Duration::from_millis(50), || {
                            let cascade = &cascade;
                            let streams = &streams;
                            async move { cascade.check_live_event(streams).await }
                        })
                        .await;
                    if !ready.pass {
                        stream_open_error =
                            Some(format!("live relay readiness: {}", ready.evidence));
                    }
                    Some(streams)
                }
                Err(err) => {
                    stream_open_error = Some(err.to_string());
                    None
                }
            };

            let mut previous_metric = 0.0;
            let mut results = Vec::new();
            let mut evidence = Vec::new();
            for tick in 1..=RUN_TICKS {
                let sender = format!("{}-phase-{phase_no}-tick-{tick}", pattern.name);
                let result = cascade
                    .run_live_tick_with_sender(
                        streams.as_ref(),
                        stream_open_error.as_deref(),
                        &sender,
                        previous_metric,
                    )
                    .await;
                if result.metric.pass {
                    previous_metric = result.metric_value;
                }
                let overall = result.log.pass && result.event.pass && result.metric.pass;
                if overall {
                    pattern_pass += 1;
                    total_pass += 1;
                    results.push(format!("Tick {tick} PASS"));
                } else {
                    total_fail += 1;
                    results.push(format!("Tick {tick} FAIL ({})", failure_summary(&result)));
                    evidence.push(format!(
                        "      Tick {tick} evidence: {}",
                        compact_evidence(&result)
                    ));
                }
            }
            outputln!(
                emit,
                "  Phase {phase_no}/{RUN_PHASES} ({transport}): {} (spawned in {})",
                results.join(", "),
                elapsed(spawn_start)
            );
            if emit {
                for line in evidence {
                    println!("{line}");
                }
            }
            if let Some(mut streams) = streams.take() {
                streams.stop().await;
            }
            cascade.stop().await;
        }
        outputln!(emit, "  Subtotal: {pattern_pass}/12 PASS");
        outputln!(emit);
    }

    outputln!(
        emit,
        "Summary: {total_pass} PASS / {total_fail} FAIL across {} ticks",
        total_pass + total_fail
    );
    if total_fail > 0 {
        return Err(anyhow!("{total_fail} tick(s) failed"));
    }
    Ok(MultiPatternReportData {
        patterns: vec![CascadeReportData {
            ticks: (total_pass + total_fail) as i32,
            pass: total_pass as i32,
            fail: total_fail as i32,
            phases: vec![PhaseReport {
                name: "multi-pattern".to_string(),
                pass: total_pass as i32,
                fail: total_fail as i32,
                failures: Vec::new(),
            }],
        }],
        total_pass: total_pass as i32,
        total_fail: total_fail as i32,
    })
}

impl Cascade {
    async fn run_tick(&self, tick: usize, previous_metric: f64) -> TickOutcome {
        let sender = format!("phase-{}-tick-{tick}", self.phase);
        self.run_tick_with_sender(&sender, previous_metric).await
    }

    async fn run_tick_with_sender(&self, sender: &str, previous_metric: f64) -> TickOutcome {
        if let Err(err) = self.tick_leaf(sender).await {
            let result = CheckResult {
                pass: false,
                evidence: err.to_string(),
            };
            return TickOutcome {
                log: result.clone(),
                event: result.clone(),
                metric: result,
                metric_value: previous_metric,
            };
        }

        let log = wait_for(Duration::from_secs(3), || async {
            self.check_log(sender).await
        })
        .await;
        let event = wait_for(Duration::from_secs(3), || async {
            self.check_event().await
        })
        .await;
        let (metric, metric_value) = self
            .wait_metric(
                previous_metric,
                Duration::from_secs(3),
                Duration::from_millis(100),
            )
            .await;
        TickOutcome {
            log,
            event,
            metric,
            metric_value,
        }
    }

    async fn run_live_tick(
        &self,
        streams: Option<&LiveStreams>,
        stream_open_error: Option<&str>,
        tick: usize,
        previous_metric: f64,
    ) -> TickOutcome {
        let sender = format!("phase-{}-tick-{tick}", self.phase);
        self.run_live_tick_with_sender(streams, stream_open_error, &sender, previous_metric)
            .await
    }

    async fn run_live_tick_with_sender(
        &self,
        streams: Option<&LiveStreams>,
        stream_open_error: Option<&str>,
        sender: &str,
        previous_metric: f64,
    ) -> TickOutcome {
        if let Err(err) = self.tick_leaf(sender).await {
            let result = CheckResult {
                pass: false,
                evidence: err.to_string(),
            };
            return TickOutcome {
                log: result.clone(),
                event: result.clone(),
                metric: result,
                metric_value: previous_metric,
            };
        }

        let mut log = CheckResult {
            pass: false,
            evidence: format!(
                "stream re-open failed: {}",
                stream_open_error.unwrap_or("<nil>")
            ),
        };
        let mut event = log.clone();
        if stream_open_error.is_none() {
            log = wait_for_every(
                Duration::from_secs(1),
                Duration::from_millis(50),
                || async { self.check_live_log(streams, sender).await },
            )
            .await;
            event = wait_for_every(
                Duration::from_secs(1),
                Duration::from_millis(50),
                || async { self.check_live_event(streams.unwrap()).await },
            )
            .await;
        }
        let (metric, metric_value) = self
            .wait_metric(
                previous_metric,
                Duration::from_secs(1),
                Duration::from_millis(50),
            )
            .await;
        TickOutcome {
            log,
            event,
            metric,
            metric_value,
        }
    }

    async fn tick_leaf(&self, sender: &str) -> Result<()> {
        let channel = self.roles["D"]
            .channel
            .as_ref()
            .ok_or_else(|| anyhow!("D channel is not open"))?
            .clone();
        let mut client = RelayServiceClient::new(channel);
        client
            .tick(TickRequest {
                sender: sender.to_string(),
                note: self.transport.clone(),
            })
            .await
            .map(|_| ())
            .context("Tick RPC failed")
    }

    async fn check_log(&self, sender: &str) -> CheckResult {
        let entries = match read_logs(self.roles["A"].channel.as_ref().unwrap().clone()).await {
            Ok(entries) => entries,
            Err(err) => return CheckResult::fail(err.to_string()),
        };
        for entry in &entries {
            if entry.message != "tick received" {
                continue;
            }
            if entry.fields.get("sender").map(String::as_str) != Some(sender)
                || entry.fields.get("responder_uid").map(String::as_str)
                    != Some(self.roles["D"].uid.as_str())
            {
                continue;
            }
            if let Err(err) = self.check_chain(&entry.chain) {
                return CheckResult::fail(format!(
                    "matching log has bad chain: {err} entry={entry:?}"
                ));
            }
            return CheckResult::pass(format!("{entry:?}"));
        }
        CheckResult::fail(format!(
            "no relayed D tick log for sender={sender} in {} A log entries",
            entries.len()
        ))
    }

    async fn check_event(&self) -> CheckResult {
        let events = match read_events(self.roles["A"].channel.as_ref().unwrap().clone()).await {
            Ok(events) => events,
            Err(err) => return CheckResult::fail(err.to_string()),
        };
        for event in &events {
            if event.r#type != EventType::InstanceReady as i32
                || event.instance_uid != self.roles["D"].uid
            {
                continue;
            }
            if let Err(err) = self.check_chain(&event.chain) {
                return CheckResult::fail(format!(
                    "matching event has bad chain: {err} event={event:?}"
                ));
            }
            return CheckResult::pass(format!("{event:?}"));
        }
        CheckResult::fail(format!(
            "no relayed D INSTANCE_READY event in {} A events",
            events.len()
        ))
    }

    async fn check_live_log(&self, streams: Option<&LiveStreams>, sender: &str) -> CheckResult {
        let Some(streams) = streams else {
            return CheckResult::fail("live streams are not open");
        };
        let entries = streams.log_entries();
        for entry in &entries {
            if entry.message != "tick received" {
                continue;
            }
            if entry.fields.get("sender").map(String::as_str) != Some(sender)
                || entry.fields.get("responder_uid").map(String::as_str)
                    != Some(self.roles["D"].uid.as_str())
            {
                continue;
            }
            if let Err(err) = self.check_chain(&entry.chain) {
                return CheckResult::fail(format!(
                    "matching live log has bad chain: {err} entry={entry:?}"
                ));
            }
            return CheckResult::pass(format!("{entry:?}"));
        }
        CheckResult::fail(format!(
            "no live log found for sender={sender} current_d_uid={} within 1s (buffer={}, stream_errors={:?})",
            self.roles["D"].uid,
            entries.len(),
            streams.stream_errors()
        ))
    }

    async fn check_live_event(&self, streams: &LiveStreams) -> CheckResult {
        let events = streams.event_entries();
        for event in &events {
            if event.r#type != EventType::InstanceReady as i32
                || event.instance_uid != self.roles["D"].uid
            {
                continue;
            }
            if let Err(err) = self.check_chain(&event.chain) {
                return CheckResult::fail(format!(
                    "matching live event has bad chain: {err} event={event:?}"
                ));
            }
            return CheckResult::pass(format!("{event:?}"));
        }
        CheckResult::fail(format!(
            "no live INSTANCE_READY event found for current_d_uid={} within 1s (buffer={}, stream_errors={:?})",
            self.roles["D"].uid,
            events.len(),
            streams.stream_errors()
        ))
    }

    async fn check_metric(&self, previous: f64) -> (CheckResult, f64) {
        let body = match fetch_metrics(&self.roles["D"].metrics_addr).await {
            Ok(body) => body,
            Err(err) => return (CheckResult::fail(err.to_string()), previous),
        };
        let Some(value) = parse_cascade_ticks(&body, &self.roles["D"].uid) else {
            return (CheckResult::fail(body), previous);
        };
        if value <= previous {
            return (
                CheckResult::fail(format!(
                    "cascade_ticks_total={value} did not increase beyond {previous}\n{body}"
                )),
                value,
            );
        }
        (
            CheckResult::pass(format!("cascade_ticks_total={value}")),
            value,
        )
    }

    async fn wait_metric(
        &self,
        previous: f64,
        timeout: Duration,
        interval: Duration,
    ) -> (CheckResult, f64) {
        let deadline = Instant::now() + timeout;
        loop {
            let (result, value) = self.check_metric(previous).await;
            if result.pass || Instant::now() >= deadline {
                return (result, value);
            }
            tokio::time::sleep(interval).await;
        }
    }

    fn check_chain(&self, chain: &[ChainHop]) -> Result<()> {
        let want_roles = ["D", "C", "B"];
        if chain.len() < want_roles.len() {
            return Err(anyhow!(
                "chain length {} < {}",
                chain.len(),
                want_roles.len()
            ));
        }
        for (idx, role) in want_roles.iter().enumerate() {
            let want = &self.roles[*role];
            let got = &chain[idx];
            if got.slug != want.slug || got.instance_uid != want.uid {
                return Err(anyhow!(
                    "hop {idx} = {}/{}, want {}/{}",
                    got.slug,
                    got.instance_uid,
                    want.slug,
                    want.uid
                ));
            }
        }
        Ok(())
    }

    async fn stop(&mut self) {
        for role in ROLE_ORDER.iter().rev() {
            if let Some(runtime) = self.roles.get_mut(*role) {
                runtime.channel = None;
                if let Some(child) = runtime.child.as_mut() {
                    let _ = signal_term(child);
                }
            }
        }
        let deadline = Instant::now() + Duration::from_secs(3);
        for role in ROLE_ORDER {
            let Some(runtime) = self.roles.get_mut(role) else {
                continue;
            };
            let Some(mut child) = runtime.child.take() else {
                continue;
            };
            loop {
                match child.try_wait() {
                    Ok(Some(_)) => break,
                    Ok(None) => {
                        if Instant::now() >= deadline {
                            let _ = child.kill();
                            let _ = child.wait();
                            break;
                        }
                        tokio::time::sleep(Duration::from_millis(50)).await;
                    }
                    Err(_) => break,
                }
            }
        }
    }
}

impl Clone for CheckResult {
    fn clone(&self) -> Self {
        Self {
            pass: self.pass,
            evidence: self.evidence.clone(),
        }
    }
}

impl CheckResult {
    fn pass(evidence: impl Into<String>) -> Self {
        Self {
            pass: true,
            evidence: evidence.into(),
        }
    }

    fn fail(evidence: impl Into<String>) -> Self {
        Self {
            pass: false,
            evidence: evidence.into(),
        }
    }
}

async fn spawn_cascade(
    phase: usize,
    transport: &str,
    binary_path: PathBuf,
    run_root: &Path,
) -> Result<Cascade> {
    let specs = pattern_roles([
        ("A", RUST_SLUG, binary_path.clone()),
        ("B", RUST_SLUG, binary_path.clone()),
        ("C", RUST_SLUG, binary_path.clone()),
        ("D", RUST_SLUG, binary_path),
    ]);
    spawn_pattern_cascade(phase, transport, specs, run_root).await
}

async fn spawn_pattern_cascade(
    phase: usize,
    transport: &str,
    specs: HashMap<String, RoleSpec>,
    run_root: &Path,
) -> Result<Cascade> {
    let mut roles = HashMap::new();
    for role in ROLE_ORDER {
        let spec = specs
            .get(role)
            .ok_or_else(|| anyhow!("missing role spec for {role}"))?;
        let runtime = new_role_runtime(phase, transport, role, spec.clone());
        let run_dir = run_root.join(&runtime.slug).join(&runtime.uid);
        let _ = fs::remove_dir_all(run_dir);
        if transport == "unix" {
            for uri in &runtime.listen_uris {
                if let Some(path) = uri.strip_prefix("unix://") {
                    let _ = fs::remove_file(path);
                }
            }
        }
        roles.insert(role.to_string(), runtime);
    }

    for (idx, role) in ROLE_ORDER.iter().enumerate() {
        if idx > 0 {
            let child_role = ROLE_ORDER[idx - 1];
            let child = roles.get(child_role).unwrap();
            let member_address = child.relay_address.clone();
            let member_slug = child.slug.clone();
            let runtime = roles.get_mut(*role).unwrap();
            runtime.member_address = member_address;
            runtime.member_slug = member_slug;
        }

        let organism_uid = roles["A"].uid.clone();
        let organism_slug = roles["A"].slug.clone();
        let runtime = roles.get_mut(*role).unwrap();
        start_role(runtime, &organism_uid, &organism_slug, run_root).await?;
    }
    tokio::time::sleep(Duration::from_millis(150)).await;
    Ok(Cascade {
        phase,
        transport: transport.to_string(),
        roles,
    })
}

fn new_role_runtime(phase: usize, transport: &str, role: &str, spec: RoleSpec) -> RoleRuntime {
    let lower = role.to_ascii_lowercase();
    let uid = format!("relay-p{phase:02}-{lower}");
    let (listen_uris, client_target, relay_address) = match transport {
        "tcp" => {
            let port = match role {
                "A" => 9090,
                "B" => 9091,
                "C" => 9092,
                "D" => 9093,
                _ => unreachable!(),
            };
            let uri = format!("tcp://127.0.0.1:{port}");
            (vec![uri.clone()], format!("127.0.0.1:{port}"), uri)
        }
        "unix" => {
            let path = format!("/tmp/observability-cascade-{lower}.sock");
            let uri = format!("unix://{path}");
            (vec![uri.clone()], uri.clone(), uri)
        }
        other => panic!("unknown transport {other}"),
    };
    RoleRuntime {
        role: role.to_string(),
        uid,
        slug: spec.slug,
        binary_path: spec.binary_path,
        listen_uris,
        relay_address,
        member_address: String::new(),
        member_slug: String::new(),
        client_target,
        metrics_addr: String::new(),
        child: None,
        channel: None,
        stdout: Arc::new(Mutex::new(String::new())),
        stderr: Arc::new(Mutex::new(String::new())),
    }
}

async fn start_role(
    runtime: &mut RoleRuntime,
    organism_uid: &str,
    organism_slug: &str,
    run_root: &Path,
) -> Result<()> {
    let mut args = vec!["serve".to_string()];
    for uri in &runtime.listen_uris {
        args.push("--listen".to_string());
        args.push(uri.clone());
    }
    if !runtime.member_address.is_empty() {
        args.push("--member".to_string());
        args.push(format!(
            "{}={}",
            runtime.member_slug, runtime.member_address
        ));
    }
    let mut cmd = Command::new(&runtime.binary_path);
    cmd.args(&args)
        .env("OP_OBS", "logs,events,metrics,prom")
        .env("OP_RUN_DIR", run_root)
        .env("OP_INSTANCE_UID", &runtime.uid)
        .env("OP_ORGANISM_UID", organism_uid)
        .env("OP_ORGANISM_SLUG", organism_slug)
        .env("OP_PROM_ADDR", "127.0.0.1:0")
        .stdout(Stdio::piped())
        .stderr(Stdio::piped());

    let mut child = cmd
        .spawn()
        .with_context(|| format!("start {}", runtime.role))?;
    capture_pipe(child.stdout.take(), runtime.stdout.clone());
    capture_pipe(child.stderr.take(), runtime.stderr.clone());
    runtime.child = Some(child);

    let meta = wait_meta(
        run_root,
        &runtime.slug,
        &runtime.uid,
        Duration::from_secs(10),
    )
    .await
    .with_context(|| {
        format!(
            "wait {} meta; stderr={}",
            runtime.role,
            runtime.stderr_text()
        )
    })?;
    runtime.metrics_addr = meta.metrics_addr;
    let channel = dial_ready(&runtime.client_target, Duration::from_secs(10))
        .await
        .with_context(|| format!("dial {}; stderr={}", runtime.role, runtime.stderr_text()))?;
    runtime.channel = Some(channel);
    Ok(())
}

async fn wait_meta(run_root: &Path, slug: &str, uid: &str, timeout: Duration) -> Result<MetaJson> {
    let deadline = Instant::now() + timeout;
    let path = run_root.join(slug).join(uid).join("meta.json");
    let mut last_error: Option<anyhow::Error> = None;
    loop {
        match fs::read_to_string(&path)
            .with_context(|| format!("read {}", path.display()))
            .and_then(|body| serde_json::from_str::<MetaJson>(&body).context("parse meta.json"))
        {
            Ok(meta) if meta.uid == uid && !meta.metrics_addr.is_empty() => return Ok(meta),
            Ok(_) => {}
            Err(err) => last_error = Some(err),
        }
        if Instant::now() >= deadline {
            return Err(last_error.unwrap_or_else(|| anyhow!("meta not ready for {uid}")));
        }
        tokio::time::sleep(Duration::from_millis(50)).await;
    }
}

async fn dial_ready(target: &str, timeout: Duration) -> Result<Channel> {
    let deadline = Instant::now() + timeout;
    let mut last_error: anyhow::Error;
    loop {
        let attempt = match dial(target).await {
            Ok(channel) => match describe_ready(channel.clone(), Duration::from_secs(1)).await {
                Ok(()) => Ok(channel),
                Err(err) => Err(err),
            },
            Err(err) => Err(err),
        };
        match attempt {
            Ok(channel) => return Ok(channel),
            Err(err) => last_error = err,
        }
        if Instant::now() >= deadline {
            return Err(last_error);
        }
        tokio::time::sleep(Duration::from_millis(50)).await;
    }
}

async fn dial(target: &str) -> Result<Channel> {
    if let Some(path) = target.strip_prefix("unix://") {
        let endpoint = Endpoint::try_from("http://[::]:50051")?;
        let connector = UnixConnector {
            path: Arc::new(PathBuf::from(path)),
        };
        return endpoint
            .connect_with_connector(connector)
            .await
            .context("dial unix");
    }
    let addr = target.strip_prefix("tcp://").unwrap_or(target);
    Endpoint::from_shared(format!("http://{addr}"))?
        .connect()
        .await
        .context("dial tcp")
}

async fn describe_ready(channel: Channel, timeout: Duration) -> Result<()> {
    let deadline = Instant::now() + timeout;
    let mut last_error: anyhow::Error;
    loop {
        let mut client = HolonMetaClient::new(channel.clone());
        match client.describe(DescribeRequest {}).await {
            Ok(_) => return Ok(()),
            Err(err) => last_error = anyhow!(err),
        }
        if Instant::now() >= deadline {
            return Err(last_error);
        }
        tokio::time::sleep(Duration::from_millis(50)).await;
    }
}

impl Service<Uri> for UnixConnector {
    type Response = TokioIo<UnixStream>;
    type Error = std::io::Error;
    type Future = Pin<Box<dyn Future<Output = std::io::Result<Self::Response>> + Send>>;

    fn poll_ready(&mut self, _cx: &mut TaskContext<'_>) -> Poll<std::io::Result<()>> {
        Poll::Ready(Ok(()))
    }

    fn call(&mut self, _req: Uri) -> Self::Future {
        let path = self.path.clone();
        Box::pin(async move { UnixStream::connect(&*path).await.map(TokioIo::new) })
    }
}

async fn read_logs(channel: Channel) -> Result<Vec<LogEntry>> {
    let mut client = HolonObservabilityClient::new(channel);
    let mut stream = client
        .logs(LogsRequest {
            min_level: LogLevel::Info as i32,
            follow: false,
            ..LogsRequest::default()
        })
        .await?
        .into_inner();
    let mut entries = Vec::new();
    while let Some(entry) = stream.message().await? {
        entries.push(entry);
    }
    Ok(entries)
}

async fn read_events(channel: Channel) -> Result<Vec<EventInfo>> {
    let mut client = HolonObservabilityClient::new(channel);
    let mut stream = client
        .events(EventsRequest {
            follow: false,
            ..EventsRequest::default()
        })
        .await?
        .into_inner();
    let mut entries = Vec::new();
    while let Some(entry) = stream.message().await? {
        entries.push(entry);
    }
    Ok(entries)
}

async fn start_live_streams(channel: Channel) -> Result<LiveStreams> {
    let mut client = HolonObservabilityClient::new(channel);
    let log_stream = client
        .logs(LogsRequest {
            min_level: LogLevel::Info as i32,
            follow: true,
            ..LogsRequest::default()
        })
        .await?
        .into_inner();
    let event_stream = client
        .events(EventsRequest {
            follow: true,
            ..EventsRequest::default()
        })
        .await?
        .into_inner();

    let streams = LiveStreams {
        logs: Arc::new(Mutex::new(Vec::new())),
        events: Arc::new(Mutex::new(Vec::new())),
        errors: Arc::new(Mutex::new(Vec::new())),
        tasks: Vec::new(),
    };
    let mut streams = streams;
    streams.tasks.push(read_log_stream(
        log_stream,
        streams.logs.clone(),
        streams.errors.clone(),
    ));
    streams.tasks.push(read_event_stream(
        event_stream,
        streams.events.clone(),
        streams.errors.clone(),
    ));
    Ok(streams)
}

fn read_log_stream(
    mut stream: tonic::Streaming<LogEntry>,
    logs: Arc<Mutex<Vec<LogEntry>>>,
    errors: Arc<Mutex<Vec<String>>>,
) -> JoinHandle<()> {
    tokio::spawn(async move {
        loop {
            match stream.message().await {
                Ok(Some(entry)) => logs.lock().unwrap().push(entry),
                Ok(None) => return,
                Err(err) => {
                    errors
                        .lock()
                        .unwrap()
                        .push(format!("logs stream ended: {err}"));
                    return;
                }
            }
        }
    })
}

fn read_event_stream(
    mut stream: tonic::Streaming<EventInfo>,
    events: Arc<Mutex<Vec<EventInfo>>>,
    errors: Arc<Mutex<Vec<String>>>,
) -> JoinHandle<()> {
    tokio::spawn(async move {
        loop {
            match stream.message().await {
                Ok(Some(entry)) => events.lock().unwrap().push(entry),
                Ok(None) => return,
                Err(err) => {
                    errors
                        .lock()
                        .unwrap()
                        .push(format!("events stream ended: {err}"));
                    return;
                }
            }
        }
    })
}

impl LiveStreams {
    async fn stop(&mut self) {
        for task in &self.tasks {
            task.abort();
        }
        for task in self.tasks.drain(..) {
            let _ = task.await;
        }
    }

    fn log_entries(&self) -> Vec<LogEntry> {
        self.logs.lock().unwrap().clone()
    }

    fn event_entries(&self) -> Vec<EventInfo> {
        self.events.lock().unwrap().clone()
    }

    fn stream_errors(&self) -> Vec<String> {
        self.errors.lock().unwrap().clone()
    }
}

async fn wait_for<F, Fut>(timeout: Duration, f: F) -> CheckResult
where
    F: FnMut() -> Fut,
    Fut: Future<Output = CheckResult>,
{
    wait_for_every(timeout, Duration::from_millis(100), f).await
}

async fn wait_for_every<F, Fut>(timeout: Duration, interval: Duration, mut f: F) -> CheckResult
where
    F: FnMut() -> Fut,
    Fut: Future<Output = CheckResult>,
{
    let deadline = Instant::now() + timeout;
    loop {
        let last = f().await;
        if last.pass || Instant::now() >= deadline {
            return last;
        }
        tokio::time::sleep(interval).await;
    }
}

async fn fetch_metrics(addr: &str) -> Result<String> {
    let response = reqwest::Client::new()
        .get(addr)
        .timeout(Duration::from_secs(2))
        .send()
        .await?;
    let status = response.status();
    let body = response.text().await?;
    if !status.is_success() {
        return Err(anyhow!("metrics HTTP {status}: {body}"));
    }
    Ok(body)
}

fn parse_cascade_ticks(body: &str, uid: &str) -> Option<f64> {
    let needle = format!("responder_uid=\"{uid}\"");
    for line in body.lines() {
        if !line.starts_with("cascade_ticks_total{") || !line.contains(&needle) {
            continue;
        }
        if let Some(value) = line.split_whitespace().last() {
            if let Ok(parsed) = value.parse::<f64>() {
                return Some(parsed);
            }
        }
    }
    None
}

fn to_cascade_report(report: CascadeReportData) -> cascadepb::CascadeReport {
    cascadepb::CascadeReport {
        ticks: report.ticks,
        pass: report.pass,
        fail: report.fail,
        phases: report
            .phases
            .into_iter()
            .map(|phase| cascadepb::PhaseResult {
                name: phase.name,
                pass: phase.pass,
                fail: phase.fail,
                failures: phase.failures,
            })
            .collect(),
    }
}

fn to_multi_pattern_report(report: MultiPatternReportData) -> cascadepb::MultiPatternReport {
    cascadepb::MultiPatternReport {
        patterns: report.patterns.into_iter().map(to_cascade_report).collect(),
        total_pass: report.total_pass,
        total_fail: report.total_fail,
    }
}

fn canonical_command(raw: &str) -> String {
    raw.trim().to_ascii_lowercase().replace(['-', '_', ' '], "")
}

fn pattern_roles(items: [(&'static str, &'static str, PathBuf); 4]) -> HashMap<String, RoleSpec> {
    items
        .into_iter()
        .map(|(role, slug, binary_path)| {
            (
                role.to_string(),
                RoleSpec {
                    slug: slug.to_string(),
                    binary_path,
                },
            )
        })
        .collect()
}

fn find_cascade_node_binary() -> Result<PathBuf> {
    find_holon_binary(RUST_SLUG)
}

fn find_holon_binary(slug: &str) -> Result<PathBuf> {
    let lang = slug
        .trim_start_matches("observability-cascade-node-")
        .to_ascii_uppercase();
    let env_name = format!("OBSERVABILITY_CASCADE_NODE_{lang}_BIN");
    if let Ok(value) = env::var(&env_name) {
        if !value.trim().is_empty() {
            return Ok(PathBuf::from(value));
        }
    }

    let manifest_dir = Path::new(env!("CARGO_MANIFEST_DIR"));
    let mut roots = Vec::new();
    if slug == RUST_SLUG {
        roots.push(
            manifest_dir
                .join("holons/observability-cascade-node/.op/build/observability-cascade-node.holon/bin"),
        );
        roots.push(
            manifest_dir
                .join("holons/observability-cascade-node/.op/build/observability-cascade-node-rust.holon/bin"),
        );
    }
    if slug == GO_SLUG {
        roots.push(
            manifest_dir
                .join("../observability-cascade-go/holons/observability-cascade-node/.op/build/observability-cascade-node.holon/bin"),
        );
        roots.push(
            manifest_dir
                .join("../observability-cascade-go/holons/observability-cascade-node/.op/build/observability-cascade-node-go.holon/bin"),
        );
    }

    let home = env::var("HOME").context("HOME is not set")?;
    roots.push(Path::new(&home)
        .join(".op")
        .join("bin")
        .join(format!("{slug}.holon"))
        .join("bin"));

    let mut found: Option<PathBuf> = None;
    for root in &roots {
        if !root.exists() {
            continue;
        }
        visit_files(root, &mut |path| {
            if path.file_name().and_then(|name| name.to_str()) != Some(slug) {
                return;
            }
            let Ok(meta) = fs::metadata(path) else {
                return;
            };
            #[cfg(unix)]
            {
                use std::os::unix::fs::PermissionsExt;
                if meta.permissions().mode() & 0o111 == 0 {
                    return;
                }
            }
            if path.to_string_lossy().contains(env::consts::OS) || found.is_none() {
                found = Some(path.to_path_buf());
            }
        })?;
        if found.is_some() {
            break;
        }
    }
    found.ok_or_else(|| {
        anyhow!(
            "{slug} binary not found under {}; run op build {slug} --install",
            roots
                .iter()
                .map(|path| path.display().to_string())
                .collect::<Vec<_>>()
                .join(", ")
        )
    })
}

fn visit_files(root: &Path, visit: &mut impl FnMut(&Path)) -> Result<()> {
    for entry in fs::read_dir(root).with_context(|| format!("read {}", root.display()))? {
        let entry = entry?;
        let path = entry.path();
        if path.is_dir() {
            visit_files(&path, visit)?;
        } else {
            visit(&path);
        }
    }
    Ok(())
}

fn run_root() -> PathBuf {
    Path::new(&env::var("HOME").expect("HOME is required"))
        .join(".op")
        .join("run")
}

fn capture_pipe<T>(pipe: Option<T>, target: Arc<Mutex<String>>)
where
    T: std::io::Read + Send + 'static,
{
    let Some(pipe) = pipe else {
        return;
    };
    std::thread::spawn(move || {
        let reader = BufReader::new(pipe);
        for line in reader.lines() {
            match line {
                Ok(line) => {
                    let mut text = target.lock().unwrap();
                    text.push_str(&line);
                    text.push('\n');
                }
                Err(_) => break,
            }
        }
    });
}

impl RoleRuntime {
    fn stderr_text(&self) -> String {
        self.stderr.lock().unwrap().clone()
    }
}

fn signal_term(child: &mut Child) -> Result<()> {
    let Some(pid) = child.id().try_into().ok() else {
        return Ok(());
    };
    unsafe {
        libc::kill(pid, libc::SIGTERM);
    }
    Ok(())
}

fn elapsed(start: Instant) -> String {
    format!("{}ms", start.elapsed().as_millis())
}

fn pass_text(pass: bool) -> &'static str {
    if pass {
        "PASS"
    } else {
        "FAIL"
    }
}

fn print_failure_evidence(family: &str, result: &CheckResult) {
    if result.pass {
        return;
    }
    let evidence = if result.evidence.trim().is_empty() {
        "<empty>"
    } else {
        result.evidence.trim()
    };
    println!("    {family} evidence: {evidence}");
}

fn failure_summary(result: &TickOutcome) -> String {
    let mut families = Vec::new();
    if !result.log.pass {
        families.push("log family");
    }
    if !result.event.pass {
        families.push("event family");
    }
    if !result.metric.pass {
        families.push("metric family");
    }
    if families.is_empty() {
        "unknown".to_string()
    } else {
        families.join(", ")
    }
}

fn compact_evidence(result: &TickOutcome) -> String {
    let mut parts = Vec::new();
    if !result.log.pass {
        parts.push(format!("log={}", truncate_evidence(&result.log.evidence)));
    }
    if !result.event.pass {
        parts.push(format!(
            "event={}",
            truncate_evidence(&result.event.evidence)
        ));
    }
    if !result.metric.pass {
        parts.push(format!(
            "metric={}",
            truncate_evidence(&result.metric.evidence)
        ));
    }
    parts.join("; ")
}

fn truncate_evidence(value: &str) -> String {
    let compact = value.split_whitespace().collect::<Vec<_>>().join(" ");
    if compact.is_empty() {
        "<empty>".to_string()
    } else if compact.len() <= 240 {
        compact
    } else {
        format!("{}...", &compact[..240])
    }
}
