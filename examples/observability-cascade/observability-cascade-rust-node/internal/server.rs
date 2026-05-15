use std::sync::Once;
use tonic::transport::Channel;

const DESCRIPTOR_SET: &[u8] = include_bytes!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/gen/rust/relay/v1/relay_descriptor.bin"
));

pub(crate) async fn listen_and_serve(
    listen_uri: &str,
    reflect: bool,
    downstream: Option<Channel>,
) -> holons::serve::Result<()> {
    register_static_describe();

    holons::serve::run_single_with_options(
        listen_uri,
        holons::relay::service(downstream),
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
