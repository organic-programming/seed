#pragma once

#include "holons/serve.hpp"
#include "greeting.grpc.pb.h"

#include <memory>
#include <string>
#include <vector>

namespace gabriel::greeting::cppholon::internal {

class Server final : public ::greeting::v1::GreetingService::Service {
public:
  grpc::Status ListLanguages(grpc::ServerContext *context,
                             const ::greeting::v1::ListLanguagesRequest *request,
                             ::greeting::v1::ListLanguagesResponse *response) override;
  grpc::Status SayHello(grpc::ServerContext *context,
                        const ::greeting::v1::SayHelloRequest *request,
                        ::greeting::v1::SayHelloResponse *response) override;
};

struct RunningServer {
  holons::serve::server_handle handle;
  std::shared_ptr<Server> service;
};

RunningServer StartServer(const std::vector<std::string> &listeners,
                          bool announce,
                          bool reflect = false);
void Serve(const std::vector<std::string> &listeners, bool reflect = false);

} // namespace gabriel::greeting::cppholon::internal
