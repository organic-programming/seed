#include "internal/server.hpp"

#include "api/public.hpp"
#include "gen/describe_generated.hpp"

#include <chrono>
#include <thread>

namespace gabriel::greeting::cppholon::internal {

namespace {

const bool kStaticDescribeRegistered = [] {
  holons::describe::use_static_response(gen::StaticDescribeResponse());
  return true;
}();

} // namespace

grpc::Status Server::ListLanguages(grpc::ServerContext *,
                                   const ::greeting::v1::ListLanguagesRequest *,
                                   ::greeting::v1::ListLanguagesResponse *response) {
  *response = api::ListLanguages();
  return grpc::Status::OK;
}

grpc::Status Server::SayHello(grpc::ServerContext *,
                              const ::greeting::v1::SayHelloRequest *request,
                              ::greeting::v1::SayHelloResponse *response) {
  *response = api::SayHello(*request);
  return grpc::Status::OK;
}

RunningServer StartServer(const std::vector<std::string> &listeners,
                          bool announce,
                          bool reflect) {
  (void)kStaticDescribeRegistered;
  RunningServer running;
  running.service = std::make_shared<Server>();

  holons::serve::options options;
  options.enable_reflection = reflect;
  options.auto_register_holon_meta = true;
  options.announce = announce;

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

void Serve(const std::vector<std::string> &listeners, bool reflect) {
  (void)kStaticDescribeRegistered;
  holons::serve::options options;
  options.enable_reflection = reflect;
  options.auto_register_holon_meta = true;
  options.announce = true;

  auto service = std::make_shared<Server>();
  holons::serve::serve(
      listeners.empty() ? std::vector<std::string>{"tcp://:9090"} : listeners,
      [service](grpc::ServerBuilder &builder) { builder.RegisterService(service.get()); },
      options);
}

} // namespace gabriel::greeting::cppholon::internal
