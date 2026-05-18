#include "internal/server.hpp"

#include "api/public.hpp"
#include "gen/describe_generated.hpp"
#include "holons/observability.hpp"
#include "internal/greetings.hpp"

#include <chrono>
#include <cctype>
#include <map>
#include <string>
#include <string_view>

namespace gabriel::greeting::cppholon::internal {

namespace {

const bool kStaticDescribeRegistered = [] {
  holons::describe::use_static_response(gen::StaticDescribeResponse());
  return true;
}();

std::string Trim(std::string_view value) {
  size_t start = 0;
  while (start < value.size() &&
         std::isspace(static_cast<unsigned char>(value[start]))) {
    ++start;
  }

  size_t end = value.size();
  while (end > start &&
         std::isspace(static_cast<unsigned char>(value[end - 1]))) {
    --end;
  }

  return std::string(value.substr(start, end - start));
}

std::string ResolvedName(const ::greeting::v1::SayHelloRequest &request) {
  std::string name = Trim(request.name());
  if (!name.empty()) {
    return name;
  }
  return std::string(Lookup(request.lang_code()).default_name);
}

void EmitGreetingObservability(
    const std::string &name, const ::greeting::v1::SayHelloResponse &response,
    std::chrono::steady_clock::time_point start) {
  // C++ Serve does not yet expose a handler-visible current transport.
  const std::string transport = "unknown";
  const auto duration_ns = std::chrono::duration_cast<std::chrono::nanoseconds>(
                               std::chrono::steady_clock::now() - start)
                               .count();
  const std::string duration_text = std::to_string(duration_ns);
  const std::string message = "Greeted " + name + " in " + response.language() +
                              " (" + response.lang_code() + ")";
  auto &obs = holons::observability::current();

  obs.logger("greeting")
      .info(message, {{"lang_code", response.lang_code()},
                      {"language", response.language()},
                      {"name", name},
                      {"greeting", response.greeting()},
                      {"transport", transport},
                      {"duration_ns", duration_text}});

  auto counter = obs.counter(
      "greeting_emitted_total",
      "Greetings emitted, partitioned by language and transport.",
      {{"lang_code", response.lang_code()},
       {"language", response.language()},
       {"transport", transport}});
  if (counter) {
    counter->inc();
  }
}

} // namespace

grpc::Status Server::ListLanguages(grpc::ServerContext *,
                                   const ::greeting::v1::ListLanguagesRequest *,
                                   ::greeting::v1::ListLanguagesResponse *response) {
  *response = api::ListLanguages();
  return grpc::Status::OK;
}

grpc::Status Server::SayHello(grpc::ServerContext *context,
                              const ::greeting::v1::SayHelloRequest *request,
                              ::greeting::v1::SayHelloResponse *response) {
  const auto start = std::chrono::steady_clock::now();
  (void)context;
  const std::string name = ResolvedName(*request);
  *response = api::SayHello(*request);
  EmitGreetingObservability(name, *response, start);
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
