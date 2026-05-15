use holons::composite;
use holons::gen::relay::v1::{relay_service_client::RelayServiceClient, HopReceipt, TickRequest};
use holons::observability;
use std::collections::HashMap;
use std::env;
use std::path::PathBuf;
use std::sync::Once;
use std::time::{Duration, Instant};
use tonic::{Request, Response, Status};

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
                    "/gen/tonic/observability_cascade/v1/observability_cascade.v1.rs"
                ));
            }
        }
    }
}

use gen::rust::observability_cascade::v1 as cascadepb;

const GO_SLUG: &str = "observability-cascade-go-node";
const RUST_SLUG: &str = "observability-cascade-rust-node";
const RUN_TICKS: i32 = 3;
const DESCRIPTOR_SET: &[u8] = include_bytes!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/gen/tonic/observability_cascade/v1/observability_cascade_descriptor.bin"
));

#[derive(Default)]
struct ObservabilityCascadeRpc;

#[derive(Clone)]
struct LanguageMember {
    lang: &'static str,
    slug: &'static str,
    binary: String,
}

struct NamedPattern {
    name: String,
    members: Vec<LanguageMember>,
}

struct TickResult {
    pass: bool,
    log: composite::CheckOutcome,
    event: composite::CheckOutcome,
    hops: composite::CheckOutcome,
}

#[tokio::main]
async fn main() {
    let args: Vec<String> = env::args().skip(1).collect();
    if args
        .first()
        .map(|arg| canonical_command(arg) == "serve")
        .unwrap_or(false)
    {
        if let Err(error) = serve_composite(&args[1..]).await {
            eprintln!("serve: {error}");
            std::process::exit(1);
        }
        return;
    }

    let failed = if args.iter().any(|arg| arg == "--multi-pattern") {
        run_multi_pattern_report(true).await.total_fail
    } else {
        let live = args.iter().any(|arg| arg == "--live-stream");
        let name = if live { "live-stream" } else { "default" };
        run_report(name, own_language_members(), live, true)
            .await
            .fail
    };
    if failed > 0 {
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
        Ok(Response::new(
            run_report("default", own_language_members(), false, false).await,
        ))
    }

    async fn run_live_stream(
        &self,
        _request: Request<cascadepb::RunRequest>,
    ) -> Result<Response<cascadepb::CascadeReport>, Status> {
        Ok(Response::new(
            run_report("live-stream", own_language_members(), true, false).await,
        ))
    }

    async fn run_multi_pattern(
        &self,
        _request: Request<cascadepb::RunRequest>,
    ) -> Result<Response<cascadepb::MultiPatternReport>, Status> {
        Ok(Response::new(run_multi_pattern_report(false).await))
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
            descriptor_set: Some(DESCRIPTOR_SET.to_vec()),
            ..holons::serve::RunOptions::default()
        },
    )
    .await
}

async fn run_multi_pattern_report(emit: bool) -> cascadepb::MultiPatternReport {
    let total_start = Instant::now();
    let patterns = rust_patterns();
    let mut out = cascadepb::MultiPatternReport::default();
    if emit {
        println!("=== observability-cascade-rust --multi-pattern ===\n");
    }
    for (idx, pattern) in patterns.iter().enumerate() {
        if emit {
            println!("Pattern {}/{}: {}", idx + 1, patterns.len(), pattern.name);
        }
        let report = run_report(&pattern.name, pattern.members.clone(), true, emit).await;
        out.total_pass += report.pass;
        out.total_fail += report.fail;
        out.patterns.push(report);
        if emit {
            let report = out.patterns.last().unwrap();
            println!(
                "Pattern {}: {}/{} {} (elapsed={})\n",
                report.name,
                report.pass,
                report.ticks,
                pass_text(report.fail == 0),
                elapsed_text(report.elapsed_us)
            );
        }
    }
    out.total_elapsed_us = total_start.elapsed().as_micros() as i64;
    if emit {
        println!(
            "Summary: {} PASS / {} FAIL across {} ticks (total elapsed={})",
            out.total_pass,
            out.total_fail,
            out.total_pass + out.total_fail,
            elapsed_text(out.total_elapsed_us)
        );
    }
    out
}

async fn run_report(
    name: &str,
    members: Vec<LanguageMember>,
    live: bool,
    emit: bool,
) -> cascadepb::CascadeReport {
    ensure_cascade_observability();
    let report_start = Instant::now();
    let mut report = cascadepb::CascadeReport {
        name: name.to_string(),
        ..cascadepb::CascadeReport::default()
    };
    let poll = if live {
        Duration::from_millis(50)
    } else {
        Duration::from_millis(100)
    };
    let timeout = if live {
        Duration::from_secs(1)
    } else {
        Duration::from_secs(3)
    };
    if emit {
        println!("=== observability-cascade-rust {}===\n", mode_suffix(name));
    }

    for (phase_idx, transport_name) in composite::TRANSPORT_COVERAGE_SEQUENCE.iter().enumerate() {
        let phase_start = Instant::now();
        let from = if phase_idx == 0 {
            *transport_name
        } else {
            composite::TRANSPORT_COVERAGE_SEQUENCE[phase_idx - 1]
        };
        let mut phase = cascadepb::PhaseResult {
            name: format!("{:02}-{}→{}", phase_idx + 1, from, transport_name),
            ..cascadepb::PhaseResult::default()
        };
        if emit {
            println!(
                "Phase {}/{}: {}",
                phase_idx + 1,
                composite::TRANSPORT_COVERAGE_SEQUENCE.len(),
                phase.name
            );
        }

        let mut cascade = match composite::build_cascade(composite::CascadeOptions {
            transport: (*transport_name).to_string(),
            members: child_specs(&members),
            extra_env: HashMap::from([
                ("OP_OBS".to_string(), "logs,events,metrics,prom".to_string()),
                ("OP_PROM_ADDR".to_string(), "127.0.0.1:0".to_string()),
            ]),
        })
        .await
        {
            Ok(cascade) => cascade,
            Err(error) => {
                phase.fail += RUN_TICKS;
                for tick in 1..=RUN_TICKS {
                    phase.failures.push(format!(
                        "tick={tick} log=spawn event=spawn hops={}",
                        compact_evidence(&error)
                    ));
                }
                phase.elapsed_us = phase_start.elapsed().as_micros() as i64;
                add_phase(&mut report, phase.clone());
                if emit {
                    print_phase_summary(&phase);
                }
                continue;
            }
        };

        let mut previous = HashMap::<String, i64>::new();
        for tick in 1..=RUN_TICKS {
            let sender = format!("{name}-phase-{:02}-tick-{tick}", phase_idx + 1);
            let result = run_tick(
                &cascade,
                &sender,
                transport_name,
                &members,
                &mut previous,
                timeout,
                poll,
                live,
            )
            .await;
            if result.pass {
                phase.pass += 1;
            } else {
                phase.fail += 1;
                phase.failures.push(evidence_line(&result, tick));
            }
            if emit {
                println!("  Tick {tick}/{RUN_TICKS}: {}", pass_text(result.pass));
                if !result.pass {
                    eprintln!("    {}", evidence_line(&result, tick));
                }
            }
        }
        let _ = cascade.stop().await;
        phase.elapsed_us = phase_start.elapsed().as_micros() as i64;
        add_phase(&mut report, phase.clone());
        if emit {
            print_phase_summary(&phase);
        }
    }

    report.elapsed_us = report_start.elapsed().as_micros() as i64;
    if emit {
        println!(
            "\nSummary: {} ticks, {} PASS, {} FAIL (total elapsed={})",
            report.ticks,
            report.pass,
            report.fail,
            elapsed_text(report.elapsed_us)
        );
    }
    report
}

async fn run_tick(
    cascade: &composite::Cascade,
    sender: &str,
    note: &str,
    members: &[LanguageMember],
    previous: &mut HashMap<String, i64>,
    timeout: Duration,
    poll: Duration,
    live: bool,
) -> TickResult {
    let mut client = RelayServiceClient::new(cascade.top.conn.clone());
    let response = match client
        .tick(TickRequest {
            sender: sender.to_string(),
            note: note.to_string(),
        })
        .await
    {
        Ok(response) => response.into_inner(),
        Err(error) => {
            let out = composite::CheckOutcome {
                pass: false,
                evidence: compact_evidence(&error.to_string()),
            };
            return TickResult {
                pass: false,
                log: out.clone(),
                event: out.clone(),
                hops: out,
            };
        }
    };

    let hops = check_hops(&response.hops, members, previous);
    if !hops.pass {
        return TickResult {
            pass: false,
            hops,
            log: composite::CheckOutcome {
                pass: false,
                evidence: "skipped".to_string(),
            },
            event: composite::CheckOutcome {
                pass: false,
                evidence: "skipped".to_string(),
            },
        };
    }
    let expected = hop_chain(&response.hops);
    let leaf_uid = response
        .hops
        .first()
        .map(|hop| hop.uid.clone())
        .unwrap_or_default();
    let log = composite::check_relayed_log(composite::LogCheckOptions {
        sender: sender.to_string(),
        leaf_uid: leaf_uid.clone(),
        expected_chain: expected.clone(),
        timeout,
        poll_interval: poll,
        live,
        ..composite::LogCheckOptions::default()
    })
    .await;
    let event = composite::check_relayed_event(composite::EventCheckOptions {
        event_type: observability::EventType::InstanceReady,
        leaf_uid,
        expected_chain: expected,
        timeout,
        poll_interval: poll,
        live,
        ..composite::EventCheckOptions::default()
    })
    .await;
    TickResult {
        pass: hops.pass && log.pass && event.pass,
        hops,
        log,
        event,
    }
}

fn check_hops(
    hops: &[HopReceipt],
    members: &[LanguageMember],
    previous: &mut HashMap<String, i64>,
) -> composite::CheckOutcome {
    if hops.len() != members.len() {
        return composite::CheckOutcome {
            pass: false,
            evidence: format!("hops length {} want {}", hops.len(), members.len()),
        };
    }
    for (idx, hop) in hops.iter().enumerate() {
        let want = &members[members.len() - 1 - idx];
        if hop.slug != want.slug {
            return composite::CheckOutcome {
                pass: false,
                evidence: format!("hop {idx} slug={} want {}", hop.slug, want.slug),
            };
        }
        if hop.uid.trim().is_empty() {
            return composite::CheckOutcome {
                pass: false,
                evidence: format!("hop {idx} uid empty"),
            };
        }
        let old = *previous.get(&hop.uid).unwrap_or(&0);
        if hop.received <= old {
            return composite::CheckOutcome {
                pass: false,
                evidence: format!("hop {idx} received={} previous={old}", hop.received),
            };
        }
        previous.insert(hop.uid.clone(), hop.received);
    }
    composite::CheckOutcome {
        pass: true,
        evidence: String::new(),
    }
}

fn hop_chain(hops: &[HopReceipt]) -> Vec<composite::ChainHop> {
    hops.iter()
        .map(|hop| composite::ChainHop {
            slug: hop.slug.clone(),
            instance_uid: hop.uid.clone(),
        })
        .collect()
}

fn own_language_members() -> Vec<LanguageMember> {
    let binary = member_binary("rust-node");
    vec![
        LanguageMember {
            lang: "rust",
            slug: RUST_SLUG,
            binary: binary.clone(),
        },
        LanguageMember {
            lang: "rust",
            slug: RUST_SLUG,
            binary: binary.clone(),
        },
        LanguageMember {
            lang: "rust",
            slug: RUST_SLUG,
            binary,
        },
    ]
}

fn rust_patterns() -> Vec<NamedPattern> {
    let rust = LanguageMember {
        lang: "rust",
        slug: RUST_SLUG,
        binary: member_binary("rust-node"),
    };
    let go = LanguageMember {
        lang: "go",
        slug: GO_SLUG,
        binary: member_binary("go-node"),
    };
    let mut out = Vec::new();
    for a in [rust.clone(), go.clone()] {
        for b in [rust.clone(), go.clone()] {
            for c in [rust.clone(), go.clone()] {
                out.push(NamedPattern {
                    name: format!("{}-{}-{}", a.lang, b.lang, c.lang),
                    members: vec![a.clone(), b.clone(), c.clone()],
                });
            }
        }
    }
    out
}

fn child_specs(members: &[LanguageMember]) -> Vec<composite::ChildSpec> {
    members
        .iter()
        .map(|member| composite::ChildSpec {
            slug: member.slug.to_string(),
            binary: member.binary.clone(),
        })
        .collect()
}

fn member_binary(id: &str) -> String {
    composite::member(id)
        .map(path_to_string)
        .unwrap_or_else(|_| String::new())
}

fn path_to_string(path: PathBuf) -> String {
    path.to_string_lossy().into_owned()
}

fn add_phase(report: &mut cascadepb::CascadeReport, phase: cascadepb::PhaseResult) {
    report.pass += phase.pass;
    report.fail += phase.fail;
    report.ticks += phase.pass + phase.fail;
    report.phases.push(phase);
}

fn ensure_cascade_observability() {
    let obs = observability::current();
    if obs.enabled(observability::Family::Logs) && obs.enabled(observability::Family::Events) {
        return;
    }
    env::set_var("OP_OBS", "logs,events,metrics,prom");
    env::set_var("OP_PROM_ADDR", "127.0.0.1:0");
    let _ = observability::from_env(observability::Config {
        slug: "observability-cascade-rust".to_string(),
        ..observability::Config::default()
    });
}

fn evidence_line(result: &TickResult, tick: i32) -> String {
    format!(
        "tick={tick} log={} event={} hops={}",
        evidence_text(&result.log),
        evidence_text(&result.event),
        evidence_text(&result.hops)
    )
}

fn evidence_text(out: &composite::CheckOutcome) -> String {
    if out.pass {
        "ok".to_string()
    } else {
        compact_evidence(&out.evidence)
    }
}

fn compact_evidence(value: &str) -> String {
    let compact = value.split_whitespace().collect::<Vec<_>>().join(" ");
    if compact.is_empty() {
        "<empty>".to_string()
    } else if compact.len() <= 240 {
        compact
    } else {
        format!("{}...", &compact[..240])
    }
}

fn pass_text(pass: bool) -> &'static str {
    if pass {
        "PASS"
    } else {
        "FAIL"
    }
}

fn print_phase_summary(phase: &cascadepb::PhaseResult) {
    println!(
        "Phase {}: {}/{} {} (elapsed={})",
        phase.name,
        phase.pass,
        phase.pass + phase.fail,
        pass_text(phase.fail == 0),
        elapsed_text(phase.elapsed_us)
    );
}

fn elapsed_text(elapsed_us: i64) -> String {
    let duration = Duration::from_micros(elapsed_us.max(0) as u64);
    if duration < Duration::from_secs(1) {
        format!("{}ms", duration.as_millis())
    } else if duration < Duration::from_secs(60) {
        format!("{:.2}s", duration.as_secs_f64())
    } else {
        format!("{:.1}m", duration.as_secs_f64() / 60.0)
    }
}

fn mode_suffix(name: &str) -> &'static str {
    if name == "default" {
        ""
    } else {
        "--live-stream "
    }
}

fn canonical_command(raw: &str) -> String {
    raw.trim().to_ascii_lowercase().replace(['-', '_', ' '], "")
}

fn register_static_describe() {
    static INIT: Once = Once::new();
    INIT.call_once(|| {
        holons::describe::use_static_response(gen::describe_generated::static_describe_response());
    });
}
