mod greetings;
mod service;

pub mod proto {
    tonic::include_proto!("greeting.v1");
}

use std::env;
use holons::serve::{self, RunOptions};
use tonic_reflection::server::Builder as ReflectionBuilder;

use crate::proto::greeting_service_server::GreetingServiceServer;
use crate::service::GreetingDaemon;

const DESCRIPTOR_SET: &[u8] = include_bytes!("../greeting_descriptor.bin");

#[tokio::main]
async fn main() {
    if let Err(error) = run().await {
        eprintln!("{error}");
        std::process::exit(1);
    }
}

async fn run() -> serve::Result<()> {
    let mut args = env::args().skip(1);
    let Some(command) = args.next() else {
        usage();
    };

    match command.as_str() {
        "serve" => {
            let remaining: Vec<String> = args.collect();
            let listen_uri = serve::parse_flags(&remaining);
            let reflection_service = if remaining.iter().any(|arg| arg == "--no-reflect") {
                None
            } else {
                Some(
                    ReflectionBuilder::configure()
                        .register_encoded_file_descriptor_set(DESCRIPTOR_SET)
                        .build_v1()?,
                )
            };

            serve::run_with_options(
                &listen_uri,
                reflection_service,
                tonic_web::enable(GreetingServiceServer::new(GreetingDaemon)),
                RunOptions { accept_http1: true },
            )
            .await?;
        }
        "version" => {
            println!("gudule-daemon-greeting-rust v0.4.2");
        }
        _ => usage(),
    }

    Ok(())
}

fn usage() -> ! {
    eprintln!("usage: gudule-daemon-greeting-rust <serve|version> [flags]");
    eprintln!("  serve   Start the gRPC server (--listen tcp://:9091) [--no-reflect]");
    eprintln!("  version Print version and exit");
    std::process::exit(1);
}
