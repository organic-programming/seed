#include "../../../sdk/cpp-holons/include/holons/holons.hpp"
#include "hello.hpp"

#include <iostream>
#include <string>
#include <vector>

int main(int argc, char *argv[]) {
  std::vector<std::string> args(argv + 1, argv + argc);
  if (!args.empty() && args.front() == "serve") {
    std::vector<std::string> serve_args(args.begin() + 1, args.end());
    auto listen_uri = holons::parse_flags(serve_args);
    std::cerr << "cpp-hello-world listening on " << listen_uri << '\n';
    std::cout << "{\"message\":\"" << hello::greet("") << "\"}\n";
    return 0;
  }

  std::string name = argc > 1 ? argv[1] : "";
  std::cout << hello::greet(name) << '\n';
  return 0;
}
