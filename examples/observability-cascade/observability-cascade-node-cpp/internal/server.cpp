#include "internal/server.hpp"

#include <filesystem>

namespace cascade::node::cppholon::internal {
namespace {

std::string ResponderSlug(const holons::observability::Observability &obs) {
  if (!obs.cfg.slug.empty()) {
    return obs.cfg.slug;
  }
  return std::filesystem::path("observability-cascade-node-cpp").filename().string();
}

} // namespace

grpc::Status Server::Tick(grpc::ServerContext *,
                          const ::relay::v1::TickRequest *request,
                          ::relay::v1::TickResponse *response) {
  auto &obs = holons::observability::current();
  const auto slug = ResponderSlug(obs);
  const auto uid = obs.cfg.instance_uid;
  obs.logger("tick").info("tick received",
                          {{"sender", request == nullptr ? "" : request->sender()},
                           {"note", request == nullptr ? "" : request->note()},
                           {"responder_slug", slug},
                           {"responder_uid", uid}});
  auto counter = obs.counter("cascade_ticks_total",
                             "Ticks received by this cascade node.",
                             {{"responder_uid", uid}});
  if (counter) {
    counter->inc();
  }
  response->set_responder_slug(slug);
  response->set_responder_instance_uid(uid);
  return grpc::Status::OK;
}

RunningServer StartServer(const std::vector<std::string> &listeners,
                          bool announce,
                          bool reflect,
                          std::vector<holons::serve::MemberRef> members) {
  RunningServer running;
  running.service = std::make_shared<Server>();

  holons::serve::options options;
  options.enable_reflection = reflect;
  options.auto_register_holon_meta = false;
  options.announce = announce;
  options.slug = "observability-cascade-node-cpp";
  options.member_endpoints = std::move(members);

  const std::vector<std::string> resolved =
      listeners.empty() ? std::vector<std::string>{"tcp://:9090"} : listeners;

  running.handle = holons::serve::start(
      resolved,
      [service = running.service](grpc::ServerBuilder &builder) {
        builder.RegisterService(service.get());
      },
      options);
  return running;
}

void Serve(const std::vector<std::string> &listeners,
           bool reflect,
           std::vector<holons::serve::MemberRef> members) {
  holons::serve::options options;
  options.enable_reflection = reflect;
  options.auto_register_holon_meta = false;
  options.announce = true;
  options.slug = "observability-cascade-node-cpp";
  options.member_endpoints = std::move(members);

  auto service = std::make_shared<Server>();
  holons::serve::serve(
      listeners.empty() ? std::vector<std::string>{"tcp://:9090"} : listeners,
      [service](grpc::ServerBuilder &builder) { builder.RegisterService(service.get()); },
      options);
}

} // namespace cascade::node::cppholon::internal
