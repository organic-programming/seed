#pragma once

#include "greeting.pb.h"

namespace gabriel::greeting::cppholon::api {

::greeting::v1::ListLanguagesResponse ListLanguages();
::greeting::v1::SayHelloResponse SayHello(
    const ::greeting::v1::SayHelloRequest &request);

} // namespace gabriel::greeting::cppholon::api
