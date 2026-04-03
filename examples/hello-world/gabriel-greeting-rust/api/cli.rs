use crate::gen::rust::greeting::v1 as pb;
use serde_json::json;
use std::io::Write;

pub const VERSION: &str = "gabriel-greeting-rust 8.8.89";

pub fn run_cli(args: &[String], stdout: &mut dyn Write, stderr: &mut dyn Write) -> i32 {
    if args.is_empty() {
        let _ = print_usage(stderr);
        return 1;
    }

    match canonical_command(&args[0]).as_str() {
        "serve" => {
            let parsed = holons::serve::parse_options(&args[1..]);
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

            match runtime.block_on(crate::internal::listen_and_serve(
                &parsed.listen_uri,
                parsed.reflect,
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
        "listlanguages" => run_list_languages(&args[1..], stdout, stderr),
        "sayhello" => run_say_hello(&args[1..], stdout, stderr),
        _ => {
            let _ = writeln!(stderr, "unknown command {:?}", args[0]);
            let _ = print_usage(stderr);
            1
        }
    }
}

fn run_list_languages(args: &[String], stdout: &mut dyn Write, stderr: &mut dyn Write) -> i32 {
    let (options, positional) = match parse_command_options(args) {
        Ok(parsed) => parsed,
        Err(error) => {
            let _ = writeln!(stderr, "listLanguages: {error}");
            return 1;
        }
    };

    if !positional.is_empty() {
        let _ = writeln!(stderr, "listLanguages: accepts no positional arguments");
        return 1;
    }

    let response = crate::public::list_languages(pb::ListLanguagesRequest::default());
    match options.format {
        OutputFormat::Text => write_list_languages_text(stdout, &response),
        OutputFormat::Json => write_list_languages_json(stdout, &response),
    }
    .map_or_else(
        |error| {
            let _ = writeln!(stderr, "listLanguages: {error}");
            1
        },
        |_| 0,
    )
}

fn run_say_hello(args: &[String], stdout: &mut dyn Write, stderr: &mut dyn Write) -> i32 {
    let (options, positional) = match parse_command_options(args) {
        Ok(parsed) => parsed,
        Err(error) => {
            let _ = writeln!(stderr, "sayHello: {error}");
            return 1;
        }
    };

    if positional.len() > 2 {
        let _ = writeln!(stderr, "sayHello: accepts at most <name> [lang_code]");
        return 1;
    }

    let mut request = pb::SayHelloRequest {
        name: String::new(),
        lang_code: "en".to_string(),
    };

    if let Some(name) = positional.first() {
        request.name = name.clone();
    }
    if positional.len() >= 2 {
        if options.lang.is_some() {
            let _ = writeln!(
                stderr,
                "sayHello: use either a positional lang_code or --lang, not both"
            );
            return 1;
        }
        request.lang_code = positional[1].clone();
    }
    if let Some(lang) = options.lang {
        request.lang_code = lang;
    }

    let response = crate::public::say_hello(request);
    match options.format {
        OutputFormat::Text => write_say_hello_text(stdout, &response),
        OutputFormat::Json => write_say_hello_json(stdout, &response),
    }
    .map_or_else(
        |error| {
            let _ = writeln!(stderr, "sayHello: {error}");
            1
        },
        |_| 0,
    )
}

fn parse_command_options(args: &[String]) -> Result<(CommandOptions, Vec<String>), String> {
    let mut options = CommandOptions::default();
    let mut positional = Vec::new();
    let mut index = 0;

    while index < args.len() {
        let arg = &args[index];
        match arg.as_str() {
            "--json" => {
                options.format = OutputFormat::Json;
            }
            "--format" => {
                index += 1;
                if index >= args.len() {
                    return Err("--format requires a value".to_string());
                }
                options.format = OutputFormat::parse(&args[index])?;
            }
            "--lang" => {
                index += 1;
                if index >= args.len() {
                    return Err("--lang requires a value".to_string());
                }
                options.lang = Some(args[index].trim().to_string());
            }
            _ => {
                if let Some(value) = arg.strip_prefix("--format=") {
                    options.format = OutputFormat::parse(value)?;
                } else if let Some(value) = arg.strip_prefix("--lang=") {
                    options.lang = Some(value.trim().to_string());
                } else if arg.starts_with("--") {
                    return Err(format!("unknown flag {:?}", arg));
                } else {
                    positional.push(arg.clone());
                }
            }
        }
        index += 1;
    }

    Ok((options, positional))
}

fn write_list_languages_text(
    stdout: &mut dyn Write,
    response: &pb::ListLanguagesResponse,
) -> std::io::Result<()> {
    for language in &response.languages {
        writeln!(
            stdout,
            "{}\t{}\t{}",
            language.code, language.name, language.native
        )?;
    }
    Ok(())
}

fn write_list_languages_json(
    stdout: &mut dyn Write,
    response: &pb::ListLanguagesResponse,
) -> std::io::Result<()> {
    serde_json::to_writer_pretty(
        &mut *stdout,
        &json!({
            "languages": response.languages.iter().map(|language| json!({
                "code": language.code,
                "name": language.name,
                "native": language.native,
            })).collect::<Vec<_>>(),
        }),
    )?;
    writeln!(stdout)
}

fn write_say_hello_text(
    stdout: &mut dyn Write,
    response: &pb::SayHelloResponse,
) -> std::io::Result<()> {
    writeln!(stdout, "{}", response.greeting)
}

fn write_say_hello_json(
    stdout: &mut dyn Write,
    response: &pb::SayHelloResponse,
) -> std::io::Result<()> {
    serde_json::to_writer_pretty(
        &mut *stdout,
        &json!({
            "greeting": response.greeting,
            "language": response.language,
            "langCode": response.lang_code,
        }),
    )?;
    writeln!(stdout)
}

fn canonical_command(raw: &str) -> String {
    raw.trim().to_lowercase().replace(['-', '_', ' '], "")
}

fn print_usage(output: &mut dyn Write) -> std::io::Result<()> {
    writeln!(
        output,
        "usage: gabriel-greeting-rust <command> [args] [flags]"
    )?;
    writeln!(output)?;
    writeln!(output, "commands:")?;
    writeln!(
        output,
        "  serve [--listen <uri>] [--reflect]        Start the gRPC server"
    )?;
    writeln!(
        output,
        "  version                                  Print version and exit"
    )?;
    writeln!(
        output,
        "  help                                     Print usage"
    )?;
    writeln!(
        output,
        "  listLanguages [--format text|json]       List supported languages"
    )?;
    writeln!(
        output,
        "  sayHello [name] [lang_code] [--format text|json] [--lang <code>]"
    )?;
    writeln!(output)?;
    writeln!(output, "examples:")?;
    writeln!(output, "  gabriel-greeting-rust serve --listen stdio")?;
    writeln!(
        output,
        "  gabriel-greeting-rust listLanguages --format json"
    )?;
    writeln!(output, "  gabriel-greeting-rust sayHello Bob fr")?;
    writeln!(
        output,
        "  gabriel-greeting-rust sayHello Bob --lang fr --format json"
    )?;
    Ok(())
}

#[derive(Default)]
struct CommandOptions {
    format: OutputFormat,
    lang: Option<String>,
}

#[derive(Clone, Copy, Default)]
enum OutputFormat {
    #[default]
    Text,
    Json,
}

impl OutputFormat {
    fn parse(raw: &str) -> Result<Self, String> {
        match raw.trim().to_lowercase().as_str() {
            "" | "text" | "txt" => Ok(Self::Text),
            "json" => Ok(Self::Json),
            _ => Err(format!("unsupported format {:?}", raw)),
        }
    }
}
