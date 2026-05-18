use crate::gen::rust::greeting::v1 as pb;
use crate::internal::greetings::lookup;
use std::collections::BTreeMap;
use std::sync::Once;
use std::time::{Duration, Instant};
use tonic::{Request, Response, Status};

const DESCRIPTOR_SET: &[u8] = include_bytes!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/gen/rust/greeting/v1/greeting_descriptor.bin"
));

#[derive(Default)]
pub(crate) struct GreetingServer;

#[tonic::async_trait]
impl pb::greeting_service_server::GreetingService for GreetingServer {
    async fn list_languages(
        &self,
        request: Request<pb::ListLanguagesRequest>,
    ) -> Result<Response<pb::ListLanguagesResponse>, Status> {
        Ok(Response::new(crate::public::list_languages(
            request.into_inner(),
        )))
    }

    async fn say_hello(
        &self,
        request: Request<pb::SayHelloRequest>,
    ) -> Result<Response<pb::SayHelloResponse>, Status> {
        let start = Instant::now();
        let request = request.into_inner();
        let name = resolved_name(&request);
        let response = crate::public::say_hello(request);
        emit_greeting_observability(&name, &response, start.elapsed());
        Ok(Response::new(response))
    }
}

pub(crate) async fn listen_and_serve(listen_uri: &str, reflect: bool) -> holons::serve::Result<()> {
    register_static_describe();

    holons::serve::run_single_with_options(
        listen_uri,
        pb::greeting_service_server::GreetingServiceServer::new(GreetingServer),
        holons::serve::RunOptions {
            reflect,
            descriptor_set: Some(DESCRIPTOR_SET.to_vec()),
            ..holons::serve::RunOptions::default()
        },
    )
    .await
}

fn register_static_describe() {
    static INIT: Once = Once::new();
    INIT.call_once(|| {
        holons::describe::use_static_response(
            crate::gen::describe_generated::static_describe_response(),
        );
    });
}

fn resolved_name(request: &pb::SayHelloRequest) -> String {
    let name = request.name.trim();
    if name.is_empty() {
        lookup(&request.lang_code).default_name.to_string()
    } else {
        name.to_string()
    }
}

fn emit_greeting_observability(name: &str, response: &pb::SayHelloResponse, elapsed: Duration) {
    // Rust serve does not yet expose a handler-visible current transport.
    let transport = "unknown";
    let duration_ns = elapsed.as_nanos().to_string();
    let message = format!(
        "Greeted {} in {} ({})",
        name, response.language, response.lang_code
    );
    let obs = holons::observability::current();

    obs.logger("greeting").info(
        &message,
        &[
            ("lang_code", response.lang_code.as_str()),
            ("language", response.language.as_str()),
            ("name", name),
            ("greeting", response.greeting.as_str()),
            ("transport", transport),
            ("duration_ns", duration_ns.as_str()),
        ],
    );

    let labels = BTreeMap::from([
        ("lang_code".to_string(), response.lang_code.clone()),
        ("language".to_string(), response.language.clone()),
        ("transport".to_string(), transport.to_string()),
    ]);
    if let Some(counter) = obs.counter(
        "greeting_emitted_total",
        "Greetings emitted, partitioned by language and transport.",
        labels,
    ) {
        counter.inc();
    }
}
