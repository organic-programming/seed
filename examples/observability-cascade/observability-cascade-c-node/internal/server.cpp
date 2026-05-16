#include "internal/server.h"

#include "holons/composite.hpp"
#include "holons/describe.hpp"
#include "holons/observability.h"
#include "holons/relay.hpp"
#include "holons/serve.hpp"
#include "holons/v1/describe.pb.h"

#include <cstdlib>
#include <cstdio>
#include <cstring>
#include <exception>
#include <memory>
#include <string>
#include <vector>

extern "C" const unsigned char *holons_generated_describe_response_bytes(
    size_t *len);

namespace {

constexpr const char *kSlug = "observability-cascade-c-node";

std::vector<holons::composite::ChildSpec> child_specs(
    const holons_composite_child_spec_t *children,
    size_t child_count) {
  std::vector<holons::composite::ChildSpec> out;
  out.reserve(child_count);
  for (size_t i = 0; i < child_count; ++i) {
    out.push_back({children[i].slug != nullptr ? children[i].slug : "",
                   children[i].binary != nullptr ? children[i].binary : ""});
  }
  return out;
}

void register_static_describe() {
  size_t len = 0;
  const unsigned char *bytes = holons_generated_describe_response_bytes(&len);
  if (bytes == nullptr || len == 0) {
    throw std::runtime_error("generated DescribeResponse is not available");
  }
  holons::v1::DescribeResponse response;
  if (!response.ParseFromArray(bytes, static_cast<int>(len))) {
    throw std::runtime_error("generated DescribeResponse did not parse");
  }
  holons::describe::use_static_response(response);
}

void configure_c_observability() {
  holon_obs_config_t obs_config;
  std::memset(&obs_config, 0, sizeof(obs_config));
  obs_config.slug = kSlug;
  obs_config.instance_uid = std::getenv("OP_INSTANCE_UID");
  obs_config.organism_uid = std::getenv("OP_ORGANISM_UID");
  obs_config.organism_slug = std::getenv("OP_ORGANISM_SLUG");
  obs_config.run_dir = std::getenv("OP_RUN_DIR");
  obs_config.default_log_level = HOLON_LEVEL_INFO;
  holon_obs_configure(&obs_config);
}

void configure_cpp_observability() {
  auto &current = holons::observability::current();
  if (current.families != 0) {
    return;
  }
  holons::observability::from_env(holons::observability::Config{kSlug});
}

}  // namespace

extern "C" int cascade_node_c_serve(
    const char *const *listen_uris,
    size_t listen_uri_count,
    const char *transport,
    const holons_composite_child_spec_t *children,
    size_t child_count,
    FILE *stderr_stream) {
  try {
    register_static_describe();
    configure_c_observability();
    configure_cpp_observability();

    std::unique_ptr<holons::composite::Cascade> downstream;
    std::shared_ptr<grpc::Channel> downstream_channel;
    if (children != nullptr && child_count > 0) {
      downstream = std::make_unique<holons::composite::Cascade>(
          holons::composite::BuildCascade(
              holons::composite::CascadeOptions{
                  transport != nullptr ? transport : "stdio",
                  child_specs(children, child_count),
                  {{"OP_OBS", "logs,events,metrics,prom"},
                   {"OP_PROM_ADDR", "127.0.0.1:0"}},
              }));
      if (downstream->top) {
        downstream_channel = downstream->top->channel;
      }
    }

    auto relay_service =
        std::make_shared<holons::relay::RelayService>(downstream_channel);
    std::vector<std::string> listeners;
    if (listen_uris != nullptr) {
      for (size_t i = 0; i < listen_uri_count; ++i) {
        if (listen_uris[i] != nullptr && listen_uris[i][0] != '\0') {
          listeners.emplace_back(listen_uris[i]);
        }
      }
    }
    if (listeners.empty()) {
      listeners.emplace_back(HOLONS_DEFAULT_URI);
    }

    holons::serve::options options;
    options.enable_reflection = false;
    options.auto_register_holon_meta = true;
    options.announce = true;
    options.slug = kSlug;

    holons::serve::serve(
        listeners,
        [relay_service](grpc::ServerBuilder &builder) {
          builder.RegisterService(relay_service.get());
        },
        options,
        {relay_service, std::shared_ptr<void>(downstream.release(),
                                              [](void *ptr) {
                                                delete static_cast<
                                                    holons::composite::Cascade *>(
                                                    ptr);
                                              })});
    return 0;
  } catch (const std::exception &error) {
    if (stderr_stream != nullptr) {
      std::fprintf(stderr_stream, "serve: %s\n", error.what());
    }
    return 1;
  }
}
