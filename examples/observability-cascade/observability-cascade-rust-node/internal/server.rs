use crate::gen::rust::relay::v1 as pb;
use std::collections::BTreeMap;
use std::path::Path;
use std::sync::Once;
use tonic::{Request, Response, Status};

const DESCRIPTOR_SET: &[u8] = include_bytes!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/gen/rust/relay/v1/relay_descriptor.bin"
));

#[derive(Default)]
pub(crate) struct RelayServer;

#[tonic::async_trait]
impl pb::relay_service_server::RelayService for RelayServer {
    async fn tick(
        &self,
        request: Request<pb::TickRequest>,
    ) -> Result<Response<pb::TickResponse>, Status> {
        let request = request.into_inner();
        let obs = holons::observability::current();
        let slug = responder_slug(&obs);
        let uid = obs.cfg.instance_uid.clone();
        obs.logger("tick").info(
            "tick received",
            &[
                ("sender", &request.sender),
                ("note", &request.note),
                ("responder_slug", &slug),
                ("responder_uid", &uid),
            ],
        );
        let mut labels = BTreeMap::new();
        labels.insert("responder_uid".to_string(), uid.clone());
        if let Some(counter) = obs.counter(
            "cascade_ticks_total",
            "Ticks received by this cascade node.",
            labels,
        ) {
            counter.inc();
        }
        Ok(Response::new(pb::TickResponse {
            responder_slug: slug,
            responder_instance_uid: uid,
        }))
    }
}

pub(crate) async fn listen_and_serve(
    listen_uri: &str,
    reflect: bool,
    members: Vec<holons::serve::MemberRef>,
) -> holons::serve::Result<()> {
    register_static_describe();

    holons::serve::run_single_with_options(
        listen_uri,
        pb::relay_service_server::RelayServiceServer::new(RelayServer),
        holons::serve::RunOptions {
            reflect,
            descriptor_set: Some(DESCRIPTOR_SET.to_vec()),
            member_endpoints: members,
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

fn responder_slug(obs: &holons::observability::Observability) -> String {
    let configured = obs.cfg.slug.trim();
    if !configured.is_empty() {
        return configured.to_string();
    }
    std::env::args()
        .next()
        .and_then(|arg| {
            Path::new(&arg)
                .file_name()
                .map(|name| name.to_string_lossy().to_string())
        })
        .unwrap_or_else(|| "observability-cascade-rust-node".to_string())
}
