use std::io::Write;

pub const VERSION: &str = "observability-cascade-node-rust 0.1.0";

pub fn run_cli(args: &[String], stdout: &mut dyn Write, stderr: &mut dyn Write) -> i32 {
    if args.is_empty() {
        let _ = print_usage(stderr);
        return 1;
    }

    match canonical_command(&args[0]).as_str() {
        "serve" => {
            let parsed = holons::serve::parse_options(&args[1..]);
            let members = match parse_member_refs(&args[1..]) {
                Ok(members) => members,
                Err(error) => {
                    let _ = writeln!(stderr, "serve: {error}");
                    return 1;
                }
            };
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

            match runtime.block_on(crate::server::listen_and_serve(
                &parsed.listen_uri,
                parsed.reflect,
                members,
            )) {
                Ok(()) => 0,
                Err(error) => {
                    let _ = writeln!(stderr, "serve: {error}");
                    1
                }
            }
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

fn parse_member_refs(args: &[String]) -> Result<Vec<holons::serve::MemberRef>, String> {
    let mut members = Vec::new();
    let mut index = 0;
    while index < args.len() {
        let arg = &args[index];
        if arg == "--member" {
            index += 1;
            if index >= args.len() {
                return Err("--member requires <slug>=<address>".to_string());
            }
            members.push(parse_member_ref(&args[index])?);
        } else if let Some(raw) = arg.strip_prefix("--member=") {
            members.push(parse_member_ref(raw)?);
        }
        index += 1;
    }
    Ok(members)
}

fn parse_member_ref(raw: &str) -> Result<holons::serve::MemberRef, String> {
    let Some((slug, address)) = raw.split_once('=') else {
        return Err("--member requires <slug>=<address>".to_string());
    };
    let slug = slug.trim();
    let address = address.trim();
    if slug.is_empty() || address.is_empty() {
        return Err("--member requires non-empty slug and address".to_string());
    }
    Ok(holons::serve::MemberRef {
        slug: slug.to_string(),
        uid: String::new(),
        address: address.to_string(),
    })
}

fn canonical_command(raw: &str) -> String {
    raw.trim().to_ascii_lowercase().replace(['-', '_', ' '], "")
}

fn print_usage(out: &mut dyn Write) -> std::io::Result<()> {
    writeln!(out, "usage: observability-cascade-node-rust <command> [args] [flags]")?;
    writeln!(out)?;
    writeln!(out, "commands:")?;
    writeln!(
        out,
        "  serve [--listen <uri>] [--member <slug>=<address>]  Start the gRPC server"
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
