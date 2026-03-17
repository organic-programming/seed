#include "api/cli.hpp"

#include <algorithm>
#include <iostream>
#include <string>
#include <vector>

int main(int argc, char **argv) {
  std::vector<std::string> args;
  args.reserve(static_cast<size_t>(std::max(argc - 1, 0)));
  for (int i = 1; i < argc; ++i) {
    args.emplace_back(argv[i]);
  }
  return gabriel::greeting::cppholon::api::RunCLI(args, std::cout, std::cerr);
}
