#include "gen/describe_generated.hpp"
#include "holons/composite.hpp"
#include "holons/describe.hpp"
#include "holons/relay.hpp"
#include "holons/serve.hpp"

#include <algorithm>
#include <exception>
#include <iostream>
#include <memory>
#include <string>
#include <vector>

namespace {

struct ServeOptions {
  std::vector<std::string> listeners;
  std::string transport{"stdio"};
  std::vector<holons::composite::ChildSpec> children;
};

ServeOptions ParseArgs(const std::vector<std::string> &args) {
  auto parsed_children = holons::composite::ParseChildFlags(args);
  ServeOptions options;
  options.children = std::move(parsed_children.first);
  const auto &remaining = parsed_children.second;
  for (size_t i = 0; i < remaining.size(); ++i) {
    const auto &arg = remaining[i];
    if (arg == "--listen") {
      if (i + 1 >= remaining.size()) throw std::invalid_argument("--listen requires a value");
      options.listeners.push_back(remaining[++i]);
      continue;
    }
    if (arg.rfind("--listen=", 0) == 0) {
      options.listeners.push_back(arg.substr(9));
      continue;
    }
    if (arg == "--transport") {
      if (i + 1 >= remaining.size()) throw std::invalid_argument("--transport requires a value");
      options.transport = remaining[++i];
      continue;
    }
    if (arg.rfind("--transport=", 0) == 0) {
      options.transport = arg.substr(12);
      continue;
    }
    if (arg == "--reflect") {
      continue;
    }
    if (arg.rfind("--", 0) == 0) {
      throw std::invalid_argument("unknown flag \"" + arg + "\"");
    }
  }
  if (options.listeners.empty()) {
    options.listeners.push_back("tcp://127.0.0.1:0");
  }
  return options;
}

} // namespace

int main(int argc, char **argv) {
  holons::describe::use_static_response(gen::StaticDescribeResponse());
  std::vector<std::string> args;
  args.reserve(static_cast<size_t>(std::max(argc - 1, 0)));
  for (int i = 1; i < argc; ++i) {
    args.emplace_back(argv[i]);
  }
  try {
    if (!args.empty() && args.front() == "serve") {
      args.erase(args.begin());
    }
    auto options = ParseArgs(args);
    std::unique_ptr<holons::composite::SpawnedMember> child;
    std::shared_ptr<grpc::Channel> downstream;
    if (!options.children.empty()) {
      auto first = options.children.front();
      std::vector<holons::composite::ChildSpec> rest(options.children.begin() + 1,
                                                     options.children.end());
      auto &obs = holons::observability::current();
      if (obs.families == 0) {
        holons::observability::from_env(
            holons::observability::Config{"observability-cascade-cpp-node"});
      }
      auto spawned = holons::composite::SpawnMember(
          holons::composite::SpawnOptions{
              first.slug,
              first.binary,
              options.transport,
              "",
              rest,
              {},
              {},
          });
      downstream = spawned.channel;
      child = std::make_unique<holons::composite::SpawnedMember>(std::move(spawned));
    }

    auto relay = std::make_shared<holons::relay::RelayService>(downstream);
    holons::serve::options serve_options;
    serve_options.slug = "observability-cascade-cpp-node";
    serve_options.auto_register_holon_meta = true;
    serve_options.announce = true;
    holons::serve::serve(
        options.listeners,
        [relay](grpc::ServerBuilder &builder) { builder.RegisterService(relay.get()); },
        serve_options,
        {relay});
    return 0;
  } catch (const std::exception &error) {
    std::cerr << "serve: " << error.what() << "\n";
    return 1;
  }
}
