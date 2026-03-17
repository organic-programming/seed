#include "api/cli.hpp"

#include <iostream>
#include <sstream>
#include <string>

namespace {

bool Expect(bool condition, const char *message) {
  if (condition) {
    return true;
  }
  std::cerr << message << '\n';
  return false;
}

bool Contains(const std::string &text, const std::string &needle) {
  return text.find(needle) != std::string::npos;
}

size_t CountOccurrences(const std::string &text, const std::string &needle) {
  size_t count = 0;
  size_t offset = 0;
  while ((offset = text.find(needle, offset)) != std::string::npos) {
    ++count;
    offset += needle.size();
  }
  return count;
}

} // namespace

int main() {
  {
    std::ostringstream stdout_stream;
    std::ostringstream stderr_stream;
    const int exit_code = gabriel::greeting::cppholon::api::RunCLI(
        {"version"}, stdout_stream, stderr_stream);
    if (!Expect(exit_code == 0, "version should succeed")) {
      return 1;
    }
    if (!Expect(stdout_stream.str().find(gabriel::greeting::cppholon::api::kVersion) != std::string::npos,
                "version output mismatch")) {
      return 1;
    }
    if (!Expect(stderr_stream.str().empty(), "version should not write stderr")) {
      return 1;
    }
  }

  {
    std::ostringstream stdout_stream;
    std::ostringstream stderr_stream;
    const int exit_code = gabriel::greeting::cppholon::api::RunCLI(
        {"help"}, stdout_stream, stderr_stream);
    if (!Expect(exit_code == 0, "help should succeed")) {
      return 1;
    }
    if (!Expect(Contains(stdout_stream.str(), "usage: gabriel-greeting-cpp"),
                "help should print usage")) {
      return 1;
    }
    if (!Expect(Contains(stdout_stream.str(), "listLanguages"),
                "help should mention listLanguages")) {
      return 1;
    }
  }

  {
    std::ostringstream stdout_stream;
    std::ostringstream stderr_stream;
    const int exit_code = gabriel::greeting::cppholon::api::RunCLI(
        {"listLanguages", "--format", "json"}, stdout_stream, stderr_stream);
    if (!Expect(exit_code == 0, "listLanguages json should succeed")) {
      return 1;
    }
    if (!Expect(CountOccurrences(stdout_stream.str(), "\"code\":\"") == 56,
                "listLanguages json should contain 56 codes")) {
      return 1;
    }
    if (!Expect(Contains(stdout_stream.str(), "\"name\":\"English\""),
                "listLanguages json should contain English")) {
      return 1;
    }
  }

  {
    std::ostringstream stdout_stream;
    std::ostringstream stderr_stream;
    const int exit_code = gabriel::greeting::cppholon::api::RunCLI(
        {"sayHello", "Alice", "fr"}, stdout_stream, stderr_stream);
    if (!Expect(exit_code == 0, "sayHello text should succeed")) {
      return 1;
    }
    if (!Expect(stdout_stream.str() == "Bonjour Alice\n",
                "sayHello text should greet Alice in French")) {
      return 1;
    }
  }

  {
    std::ostringstream stdout_stream;
    std::ostringstream stderr_stream;
    const int exit_code = gabriel::greeting::cppholon::api::RunCLI(
        {"sayHello", "--json"}, stdout_stream, stderr_stream);
    if (!Expect(exit_code == 0, "sayHello json should succeed")) {
      return 1;
    }
    if (!Expect(Contains(stdout_stream.str(), "\"greeting\":\"Hello Mary\""),
                "sayHello json should include default greeting")) {
      return 1;
    }
    if (!Expect(Contains(stdout_stream.str(), "\"langCode\":\"en\""),
                "sayHello json should include camelCase langCode")) {
      return 1;
    }
  }

  return 0;
}
