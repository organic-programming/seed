#pragma once

#include <array>
#include <string_view>

namespace gabriel::greeting::cppholon::internal {

struct Greeting {
  std::string_view lang_code;
  std::string_view lang_english;
  std::string_view lang_native;
  std::string_view template_string;
  std::string_view default_name;
};

const std::array<Greeting, 56> &Greetings();
const Greeting &Lookup(std::string_view lang_code);

} // namespace gabriel::greeting::cppholon::internal
