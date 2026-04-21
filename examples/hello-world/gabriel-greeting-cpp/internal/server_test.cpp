#include "internal/server.hpp"

#include "holons/v1/describe.grpc.pb.h"

#include <chrono>
#include <iostream>
#include <memory>
#include <string>

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

} // namespace

int main() {
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

  running.handle.stop();
  return 0;
}
