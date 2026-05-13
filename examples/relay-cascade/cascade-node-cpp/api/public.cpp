#include "api/public.hpp"

#include "holons/observability.hpp"

#include <filesystem>

namespace cascade::node::cppholon::api {
namespace {

std::string ResponderSlug(const holons::observability::Observability &obs) {
  if (!obs.cfg.slug.empty()) {
    return obs.cfg.slug;
  }
  return "cascade-node-cpp";
}

} // namespace

::relay::v1::TickResponse Tick(const ::relay::v1::TickRequest &) {
  auto &obs = holons::observability::current();
  ::relay::v1::TickResponse response;
  response.set_responder_slug(ResponderSlug(obs));
  response.set_responder_instance_uid(obs.cfg.instance_uid);
  return response;
}

} // namespace cascade::node::cppholon::api
