#pragma once

#include "observability.hpp"

#include "relay/v1/relay.grpc.pb.h"

#include <atomic>
#include <filesystem>
#include <memory>

namespace holons::relay {

class RelayService final : public ::relay::v1::RelayService::Service {
public:
  explicit RelayService(std::shared_ptr<grpc::Channel> downstream = {})
      : downstream_(std::move(downstream)) {}

  grpc::Status Tick(grpc::ServerContext *context,
                    const ::relay::v1::TickRequest *request,
                    ::relay::v1::TickResponse *response) override {
    auto received = received_.fetch_add(1, std::memory_order_relaxed) + 1;
    auto &obs = observability::current();
    auto slug = obs.cfg.slug.empty()
                    ? std::filesystem::path("observability-cascade-cpp-node").filename().string()
                    : obs.cfg.slug;
    auto uid = obs.cfg.instance_uid;
    obs.logger("tick").info("tick received",
                            {{"sender", request == nullptr ? "" : request->sender()},
                             {"note", request == nullptr ? "" : request->note()},
                             {"responder_slug", slug},
                             {"responder_uid", uid}});
    auto counter = obs.counter("cascade_ticks_total",
                               "Ticks received by this cascade node.",
                               {{"responder_uid", uid}});
    if (counter) counter->inc();

    if (downstream_) {
      auto stub = ::relay::v1::RelayService::NewStub(downstream_);
      grpc::ClientContext downstream_context;
      (void)context;
      ::relay::v1::TickResponse downstream_response;
      auto status = stub->Tick(&downstream_context,
                               request == nullptr ? ::relay::v1::TickRequest{} : *request,
                               &downstream_response);
      if (!status.ok()) return status;
      for (const auto &hop : downstream_response.hops()) {
        *response->add_hops() = hop;
      }
    }
    response->set_responder_slug(slug);
    response->set_responder_instance_uid(uid);
    auto *hop = response->add_hops();
    hop->set_slug(slug);
    hop->set_uid(uid);
    hop->set_received(received);
    return grpc::Status::OK;
  }

private:
  std::shared_ptr<grpc::Channel> downstream_;
  std::atomic<std::int64_t> received_{0};
};

} // namespace holons::relay
