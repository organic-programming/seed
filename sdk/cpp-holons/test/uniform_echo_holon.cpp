#include "holons/describe.hpp"
#include "holons/discover.hpp"
#include "holons/serve.hpp"

#include "echo/v1/echo.grpc.pb.h"

namespace {

holons::v1::DescribeResponse make_static_response() {
  holons::v1::DescribeResponse response;

  auto *manifest = response.mutable_manifest();
  auto *identity = manifest->mutable_identity();
  identity->set_schema("holon/v1");
  identity->set_uuid("uniform-echo-holon-uuid");
  identity->set_given_name("Echo");
  identity->set_family_name("Server");
  identity->set_motto("Replies with the payload.");
  identity->add_aliases("echo");
  identity->set_status("draft");
  identity->set_born("2026-03-31");

  manifest->set_lang("cpp");
  manifest->set_kind("native");
  manifest->set_transport("stdio");
  manifest->mutable_build()->set_runner("shell");
  manifest->mutable_artifacts()->set_binary("uniform_echo_holon");
  manifest->add_platforms(holons::discovery_detail::current_arch_dir());

  auto *service = response.add_services();
  service->set_name("echo.v1.Echo");
  service->set_description("Echo test service.");
  auto *method = service->add_methods();
  method->set_name("Ping");
  method->set_description("Echoes the inbound payload.");

  return response;
}

class EchoServiceImpl final : public echo::v1::Echo::Service {
public:
  grpc::Status Ping(grpc::ServerContext *,
                    const echo::v1::PingRequest *request,
                    echo::v1::PingResponse *response) override {
    response->set_message(request->message());
    response->set_sdk("uniform-echo-holon");
    return grpc::Status();
  }
};

} // namespace

int main(int argc, char **argv) {
  holons::describe::use_static_response(make_static_response());

  std::vector<std::string> args;
  args.reserve(static_cast<size_t>(std::max(argc - 1, 0)));
  for (int i = 1; i < argc; ++i) {
    args.emplace_back(argv[i]);
  }
  if (!args.empty() && args.front() == "serve") {
    args.erase(args.begin());
  }

  auto parsed = holons::serve::parse_options(args);
  holons::serve::options opts;
  EchoServiceImpl service;
  holons::serve::serve(
      parsed.listeners,
      [&service](grpc::ServerBuilder &builder) { builder.RegisterService(&service); },
      opts);
  return 0;
}
