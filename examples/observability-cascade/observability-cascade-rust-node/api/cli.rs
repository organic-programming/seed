use std::io::Write;

pub const VERSION: &str = "observability-cascade-rust-node 0.1.0";

pub fn run_cli(args: &[String], stdout: &mut dyn Write, stderr: &mut dyn Write) -> i32 {
    if args.is_empty() {
        let _ = print_usage(stderr);
        return 1;
    }

    match canonical_command(&args[0]).as_str() {
        "serve" => {
            let (children, remaining) = holons::serve::parse_child_flags(&args[1..]);
            let parsed = holons::serve::parse_options(&remaining);
            let transport = parse_transport(&remaining);
            let runtime = match tokio::runtime::Builder::new_multi_thread()
                .enable_all()
                .build()
            {
                Ok(runtime) => runtime,
                Err(error) => {
                    let _ = writeln!(stderr, "serve: {error}");
                    return 1;
                }
            };

            runtime.block_on(async {
                let _ = holons::observability::from_env(holons::observability::Config {
                    slug: manifest_slug(),
                    ..holons::observability::Config::default()
                });
                let mut downstream = None;
                if let Some(first) = children.first() {
                    match holons::composite::spawn_member(holons::composite::SpawnOptions {
                        slug: first.slug.clone(),
                        binary_path: first.binary.clone(),
                        transport,
                        downstream_chain: children.iter().skip(1).cloned().collect(),
                        ..holons::composite::SpawnOptions::default()
                    })
                    .await
                    {
                        Ok(member) => downstream = Some(member),
                        Err(error) => {
                            let _ = writeln!(stderr, "serve: {error}");
                            return 1;
                        }
                    }
                }
                let downstream_conn = downstream.as_ref().map(|member| member.conn.clone());
                let result = crate::server::listen_and_serve(
                    &parsed.listen_uri,
                    parsed.reflect,
                    downstream_conn,
                )
                .await;
                if let Some(mut member) = downstream {
                    let _ = member.stop().await;
                }
                match result {
                    Ok(()) => 0,
                    Err(error) => {
                        let _ = writeln!(stderr, "serve: {error}");
                        1
                    }
                }
            })
        }
        "version" => {
            let _ = writeln!(stdout, "{VERSION}");
            0
        }
        "help" => {
            let _ = print_usage(stdout);
            0
        }
        _ => {
            let _ = writeln!(stderr, "unknown command {:?}", args[0]);
            let _ = print_usage(stderr);
            1
        }
    }
}

fn parse_transport(args: &[String]) -> String {
    let mut index = 0;
    while index < args.len() {
        let arg = &args[index];
        if arg == "--transport" && index + 1 < args.len() {
            return args[index + 1].clone();
        }
        if let Some(raw) = arg.strip_prefix("--transport=") {
            return raw.to_string();
        }
        index += 1;
    }
    "stdio".to_string()
}

fn manifest_slug() -> String {
    let response = crate::gen::describe_generated::static_describe_response();
    let Some(manifest) = response.manifest else {
        return String::new();
    };
    if let Some(artifacts) = manifest.artifacts {
        let binary = artifacts.binary.trim();
        if !binary.is_empty() {
            return binary.to_string();
        }
    }
    let Some(identity) = manifest.identity else {
        return String::new();
    };
    format!(
        "{}-{}",
        identity.given_name.trim(),
        identity.family_name.trim().trim_end_matches('?')
    )
    .trim()
    .to_ascii_lowercase()
    .replace(' ', "-")
    .trim_matches('-')
    .to_string()
}

fn canonical_command(raw: &str) -> String {
    raw.trim().to_ascii_lowercase().replace(['-', '_', ' '], "")
}

fn print_usage(out: &mut dyn Write) -> std::io::Result<()> {
    writeln!(
        out,
        "usage: observability-cascade-rust-node <command> [args] [flags]"
    )?;
    writeln!(out)?;
    writeln!(out, "commands:")?;
    writeln!(
        out,
        "  serve [--listen <uri>] [--transport <name>] [--child <slug>=<binary>]  Start the gRPC server"
    )?;
    writeln!(
        out,
        "  version                                             Print version and exit"
    )?;
    writeln!(
        out,
        "  help                                                Print this help"
    )
}
