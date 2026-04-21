#pragma once

#include <iosfwd>
#include <string>
#include <vector>

namespace gabriel::greeting::cppholon::api {

inline constexpr char kVersion[] = "gabriel-greeting-cpp 0.1.152";

int RunCLI(const std::vector<std::string> &args,
           std::ostream &stdout_stream,
           std::ostream &stderr_stream);

void PrintUsage(std::ostream &output);

} // namespace gabriel::greeting::cppholon::api
