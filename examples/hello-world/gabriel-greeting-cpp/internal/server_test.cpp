#include "internal/server.hpp"

#include "holons/observability.hpp"
#include "holons/serve.hpp"
#include "holons/v1/describe.grpc.pb.h"

#include <chrono>
#include <cstdlib>
#include <iostream>
#include <map>
#include <memory>
#include <optional>
#include <string>
#include <utility>

namespace {

bool Expect(bool condition, const char *message) {
  if (condition) {
    return true;
  }
  std::cerr << message << '\n';
  return false;
}

std::string ToTarget(const std::string &advertised_listener) {
  if (advertised_listener.rfind("tcp://", 0) == 0) {
    return advertised_listener.substr(6);
  }
  return advertised_listener;
}

class EnvGuard {
public:
  explicit EnvGuard(const char *key) : key_(key) {
    if (const char *value = std::getenv(key)) {
      previous_ = std::string(value);
    }
  }
  ~EnvGuard() {
    if (previous_) {
      setenv(key_.c_str(), previous_->c_str(), 1);
    } else {
      unsetenv(key_.c_str());
    }
  }

private:
  std::string key_;
  std::optional<std::string> previous_;
};

std::map<std::string, const holons::v1::AnyValue *>
AttrsByKey(const holons::v1::LogRecord &record) {
  std::map<std::string, const holons::v1::AnyValue *> out;
  for (const auto &attr : record.attributes()) {
    out.emplace(attr.key(), attr.has_value() ? &attr.value() : nullptr);
  }
  return out;
}

} // namespace

int main() {
  EnvGuard op_obs("OP_OBS");
  setenv("OP_OBS", "logs,metrics", 1);

  auto running = gabriel::greeting::cppholon::internal::StartServer(
      {"tcp://127.0.0.1:0"}, false);
  if (!Expect(!running.handle.listeners().empty(), "expected at least one listener")) {
    return 1;
  }

  const std::string target =
      ToTarget(running.handle.listeners().front().advertised);
  auto channel = grpc::CreateChannel(target, grpc::InsecureChannelCredentials());
  if (!Expect(channel->WaitForConnected(std::chrono::system_clock::now() +
                                        std::chrono::seconds(5)),
              "channel failed to connect")) {
    return 1;
  }

  auto stub = greeting::v1::GreetingService::NewStub(channel);
  auto meta_stub = holons::v1::HolonMeta::NewStub(channel);

  grpc::ClientContext list_context;
  greeting::v1::ListLanguagesRequest list_request;
  greeting::v1::ListLanguagesResponse list_response;
  auto list_status = stub->ListLanguages(&list_context, list_request, &list_response);
  if (!Expect(list_status.ok(), "ListLanguages RPC failed")) {
    return 1;
  }
  if (!Expect(list_response.languages_size() == 56, "ListLanguages should return 56 entries")) {
    return 1;
  }

  grpc::ClientContext describe_context;
  holons::v1::DescribeRequest describe_request;
  holons::v1::DescribeResponse describe_response;
  auto describe_status =
      meta_stub->Describe(&describe_context, describe_request, &describe_response);
  if (!Expect(describe_status.ok(), "Describe RPC failed")) {
    return 1;
  }
  if (!Expect(describe_response.manifest().identity().given_name() == "Gabriel",
              "Describe should serve the generated static manifest")) {
    return 1;
  }

  grpc::ClientContext hello_context;
  greeting::v1::SayHelloRequest hello_request;
  hello_request.set_lang_code("ja");
  greeting::v1::SayHelloResponse hello_response;
  auto hello_status = stub->SayHello(&hello_context, hello_request, &hello_response);
  if (!Expect(hello_status.ok(), "SayHello RPC failed")) {
    return 1;
  }
  if (!Expect(hello_response.greeting() == "こんにちは、マリアさん",
              "SayHello should use localized Japanese default")) {
    return 1;
  }

  grpc::ClientContext spanish_context;
  greeting::v1::SayHelloRequest spanish_request;
  spanish_request.set_name("Ana");
  spanish_request.set_lang_code("es");
  greeting::v1::SayHelloResponse spanish_response;
  auto spanish_status =
      stub->SayHello(&spanish_context, spanish_request, &spanish_response);
  if (!Expect(spanish_status.ok(), "Spanish SayHello RPC failed")) {
    return 1;
  }

  bool saw_typed_log = false;
  auto &obs = holons::observability::current();
  if (!Expect(static_cast<bool>(obs.log_ring), "logs ring should be enabled")) {
    return 1;
  }
  for (const auto &entry : obs.log_ring->drain()) {
    auto proto = holons::serve::detail::to_proto_log(entry);
    if (proto.body().value_case() != holons::v1::AnyValue::kStringValue ||
        proto.body().string_value() != "Greeted Ana in Spanish (es)") {
      continue;
    }
    saw_typed_log = true;
    const auto attrs = AttrsByKey(proto);
    if (!Expect(proto.severity_number() == holons::v1::SEVERITY_NUMBER_INFO,
                "greeting log should be INFO")) {
      return 1;
    }
    if (!Expect(attrs.at("holons.slug")->string_value() == "gabriel-greeting-cpp",
                "greeting log should include holons.slug")) {
      return 1;
    }
    if (!Expect(attrs.at("service.name")->string_value() == "gabriel-greeting-cpp",
                "greeting log should include service.name")) {
      return 1;
    }
    if (!Expect(!attrs.at("holons.instance_uid")->string_value().empty(),
                "greeting log should include holons.instance_uid")) {
      return 1;
    }
    if (!Expect(!attrs.at("service.instance.id")->string_value().empty(),
                "greeting log should include service.instance.id")) {
      return 1;
    }
    if (!Expect(!attrs.at("holons.session_id")->string_value().empty(),
                "greeting log should include holons.session_id")) {
      return 1;
    }
    if (!Expect(attrs.at("transport")->value_case() ==
                    holons::v1::AnyValue::kStringValue &&
                attrs.at("transport")->string_value() == "tcp",
                "transport should be typed string tcp")) {
      return 1;
    }
    if (!Expect(attrs.at("duration_ns")->value_case() ==
                    holons::v1::AnyValue::kIntValue &&
                attrs.at("duration_ns")->int_value() >= 0,
                "duration_ns should be typed int64")) {
      return 1;
    }
  }
  if (!Expect(saw_typed_log, "missing typed Ana greeting log")) {
    return 1;
  }

  running.handle.stop();
  holons::observability::reset();
  return 0;
}
