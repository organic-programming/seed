#pragma once

#include "relay/v1/relay.pb.h"

namespace cascade::node::cppholon::api {

::relay::v1::TickResponse Tick(const ::relay::v1::TickRequest &request);

} // namespace cascade::node::cppholon::api
