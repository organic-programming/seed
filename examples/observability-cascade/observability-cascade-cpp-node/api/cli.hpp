#pragma once

#include <iosfwd>
#include <string>
#include <vector>

namespace cascade::node::cppholon::api {

inline constexpr char kVersion[] = "observability-cascade-cpp-node {{ .Version }}";

int RunCLI(const std::vector<std::string> &args,
           std::ostream &stdout_stream,
           std::ostream &stderr_stream);

void PrintUsage(std::ostream &output);

} // namespace cascade::node::cppholon::api
