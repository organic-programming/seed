use crate::gen::rust::greeting::v1 as pb;
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

pub(crate) async fn listen_and_serve(listen_uri: &str) -> holons::serve::Result<()> {
    let reflection = tonic_reflection::server::Builder::configure()
        .register_encoded_file_descriptor_set(DESCRIPTOR_SET)
        .build_v1()?;

    holons::serve::run_with_options(
        listen_uri,
        Some(reflection),
        pb::greeting_service_server::GreetingServiceServer::new(GreetingServer),
        holons::serve::RunOptions::default(),
    )
    .await
}
