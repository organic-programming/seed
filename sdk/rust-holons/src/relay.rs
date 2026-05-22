use crate::gen::relay::v1 as pb;
use crate::observability::{self, Field};
use std::collections::BTreeMap;
use std::sync::atomic::{AtomicI64, Ordering};
use tonic::{Request, Response, Status};

#[derive(Default)]
pub struct RelayServer {
    downstream: Option<tonic::transport::Channel>,
    received: AtomicI64,
}

impl RelayServer {
    pub fn new(downstream: Option<tonic::transport::Channel>) -> Self {
        Self {
            downstream,
            received: AtomicI64::new(0),
        }
    }
}

pub fn service(
    downstream: Option<tonic::transport::Channel>,
) -> pb::relay_service_server::RelayServiceServer<RelayServer> {
    pb::relay_service_server::RelayServiceServer::new(RelayServer::new(downstream))
}

#[tonic::async_trait]
impl pb::relay_service_server::RelayService for RelayServer {
    async fn tick(
        &self,
        request: Request<pb::TickRequest>,
    ) -> Result<Response<pb::TickResponse>, Status> {
        let count = self.received.fetch_add(1, Ordering::SeqCst) + 1;
        let request = request.into_inner();
        let obs = observability::current();
        let slug = responder_slug(&obs);
        let uid = obs.cfg.instance_uid.clone();
        obs.logger("tick").info(
            "tick received",
            &[
                ("sender", Field::String(request.sender.clone())),
                ("note", Field::String(request.note.clone())),
                ("responder_slug", Field::String(slug.clone())),
                ("responder_uid", Field::String(uid.clone())),
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

        let mut hops = Vec::new();
        if let Some(channel) = self.downstream.clone() {
            let mut client = pb::relay_service_client::RelayServiceClient::new(channel);
            let response = client.tick(request).await?.into_inner();
            hops.extend(response.hops);
        }
        hops.push(pb::HopReceipt {
            slug: slug.clone(),
            uid: uid.clone(),
            received: count,
        });

        Ok(Response::new(pb::TickResponse {
            responder_slug: slug,
            responder_instance_uid: uid,
            hops,
        }))
    }
}

fn responder_slug(obs: &observability::Observability) -> String {
    let configured = obs.cfg.slug.trim();
    if !configured.is_empty() {
        return configured.to_string();
    }
    "unknown".to_string()
}
