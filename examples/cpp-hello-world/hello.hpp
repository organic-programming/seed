#pragma once
#include <string>

namespace hello {

/// Pure deterministic HelloService.
inline std::string greet(const std::string &name) {
  return "Hello, " + (name.empty() ? std::string("World") : name) + "!";
}

} // namespace hello
