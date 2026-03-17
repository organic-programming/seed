#include "api/public.hpp"

#include "internal/greetings.hpp"

#include <cctype>
#include <string>
#include <string_view>

namespace gabriel::greeting::cppholon::api {
namespace {

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

std::string FormatGreeting(std::string_view pattern, std::string_view name) {
  std::string result(pattern);
  const size_t marker = result.find("%s");
  if (marker == std::string::npos) {
    return result;
  }
  result.replace(marker, 2, name);
  return result;
}

} // namespace

::greeting::v1::ListLanguagesResponse ListLanguages() {
  ::greeting::v1::ListLanguagesResponse response;
  for (const auto &entry : internal::Greetings()) {
    auto *language = response.add_languages();
    language->set_code(std::string(entry.lang_code));
    language->set_name(std::string(entry.lang_english));
    language->set_native(std::string(entry.lang_native));
  }
  return response;
}

::greeting::v1::SayHelloResponse SayHello(
    const ::greeting::v1::SayHelloRequest &request) {
  const auto &entry = internal::Lookup(request.lang_code());

  std::string name = Trim(request.name());
  if (name.empty()) {
    name = std::string(entry.default_name);
  }

  ::greeting::v1::SayHelloResponse response;
  response.set_greeting(FormatGreeting(entry.template_string, name));
  response.set_language(std::string(entry.lang_english));
  response.set_lang_code(std::string(entry.lang_code));
  return response;
}

} // namespace gabriel::greeting::cppholon::api
