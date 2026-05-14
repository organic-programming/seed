#pragma once

#include "holons/serve.hpp"
#include "relay/v1/relay.grpc.pb.h"

#include <memory>
#include <string>
#include <vector>

namespace cascade::node::cppholon::internal {

class Server final : public ::relay::v1::RelayService::Service {
public:
  grpc::Status Tick(grpc::ServerContext *context,
                    const ::relay::v1::TickRequest *request,
                    ::relay::v1::TickResponse *response) override;
};

struct RunningServer {
  holons::serve::server_handle handle;
  std::shared_ptr<Server> service;
};

RunningServer StartServer(const std::vector<std::string> &listeners,
                          bool announce,
                          bool reflect,
                          std::vector<holons::serve::MemberRef> members);
void Serve(const std::vector<std::string> &listeners,
           bool reflect,
           std::vector<holons::serve::MemberRef> members);

} // namespace cascade::node::cppholon::internal
