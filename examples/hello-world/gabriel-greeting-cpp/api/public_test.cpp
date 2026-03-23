#include "api/public.hpp"

#include <iostream>

namespace {

bool Expect(bool condition, const char *message) {
  if (condition) {
    return true;
  }
  std::cerr << message << '\n';
  return false;
}

} // namespace

int main() {
  const auto languages = gabriel::greeting::cppholon::api::ListLanguages();
  if (!Expect(languages.languages_size() == 56, "expected 56 languages")) {
    return 1;
  }
  if (!Expect(languages.languages(0).code() == "en", "expected first language to be English")) {
    return 1;
  }
  if (!Expect(languages.languages(1).native() == "Français", "expected French native label")) {
    return 1;
  }

  greeting::v1::SayHelloRequest french;
  french.set_lang_code("fr");
  auto french_response = gabriel::greeting::cppholon::api::SayHello(french);
  if (!Expect(french_response.greeting() == "Bonjour Marie", "expected localized default French greeting")) {
    return 1;
  }

  greeting::v1::SayHelloRequest unknown;
  unknown.set_name("Bob");
  unknown.set_lang_code("xx");
  auto english_fallback = gabriel::greeting::cppholon::api::SayHello(unknown);
  if (!Expect(english_fallback.greeting() == "Hello Bob", "expected English fallback greeting")) {
    return 1;
  }
  if (!Expect(english_fallback.lang_code() == "en", "expected English fallback lang code")) {
    return 1;
  }

  return 0;
}
