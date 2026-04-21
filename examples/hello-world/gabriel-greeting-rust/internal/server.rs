use crate::gen::rust::greeting::v1 as pb;
use std::sync::Once;
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
        Ok(Response::new(crate::public::say_hello(
            request.into_inner(),
        )))
    }
}

pub(crate) async fn listen_and_serve(
    listen_uri: &str,
    reflect: bool,
) -> holons::serve::Result<()> {
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
