#include "api/cli.hpp"

#include "api/public.hpp"
#include "internal/server.hpp"

#include <algorithm>
#include <cctype>
#include <exception>
#include <string>
#include <string_view>

namespace cascade::node::cppholon::api {
namespace {

struct ServeOptions {
  std::vector<std::string> listeners;
  std::vector<holons::serve::MemberRef> members;
  bool reflect = false;
};

std::string CanonicalCommand(std::string_view raw) {
  std::string result;
  result.reserve(raw.size());
  for (char ch : raw) {
    if (ch == '-' || ch == '_' || std::isspace(static_cast<unsigned char>(ch))) {
      continue;
    }
    result.push_back(static_cast<char>(std::tolower(static_cast<unsigned char>(ch))));
  }
  return result;
}

holons::serve::MemberRef ParseMember(const std::string &raw) {
  const auto eq = raw.find('=');
  if (eq == std::string::npos || eq == 0 || eq + 1 >= raw.size()) {
    throw std::invalid_argument("--member requires <slug>=<address>");
  }
  return holons::serve::MemberRef{raw.substr(0, eq), raw.substr(eq + 1)};
}

ServeOptions ParseServeOptions(const std::vector<std::string> &args) {
  ServeOptions options;
  for (size_t i = 0; i < args.size(); ++i) {
    const auto &arg = args[i];
    if (arg == "--listen") {
      if (i + 1 >= args.size()) {
        throw std::invalid_argument("--listen requires a value");
      }
      options.listeners.push_back(args[++i]);
      continue;
    }
    if (arg.rfind("--listen=", 0) == 0) {
      options.listeners.push_back(arg.substr(9));
      continue;
    }
    if (arg == "--port") {
      if (i + 1 >= args.size()) {
        throw std::invalid_argument("--port requires a value");
      }
      options.listeners.push_back("tcp://:" + args[++i]);
      continue;
    }
    if (arg.rfind("--port=", 0) == 0) {
      options.listeners.push_back("tcp://:" + arg.substr(7));
      continue;
    }
    if (arg == "--reflect") {
      options.reflect = true;
      continue;
    }
    if (arg == "--member") {
      if (i + 1 >= args.size()) {
        throw std::invalid_argument("--member requires <slug>=<address>");
      }
      options.members.push_back(ParseMember(args[++i]));
      continue;
    }
    if (arg.rfind("--member=", 0) == 0) {
      options.members.push_back(ParseMember(arg.substr(9)));
      continue;
    }
    if (arg.rfind("--", 0) == 0) {
      throw std::invalid_argument("unknown flag \"" + arg + "\"");
    }
    throw std::invalid_argument("serve accepts only --listen, --port, --reflect, and --member");
  }
  if (options.listeners.empty()) {
    options.listeners.push_back("tcp://:9090");
  }
  return options;
}

std::string JsonEscape(std::string_view value) {
  std::string escaped;
  escaped.reserve(value.size());
  for (unsigned char ch : value) {
    switch (ch) {
    case '\\': escaped += "\\\\"; break;
    case '"': escaped += "\\\""; break;
    case '\n': escaped += "\\n"; break;
    case '\r': escaped += "\\r"; break;
    case '\t': escaped += "\\t"; break;
    default: escaped.push_back(static_cast<char>(ch)); break;
    }
  }
  return escaped;
}

int RunTick(const std::vector<std::string> &args,
            std::ostream &stdout_stream,
            std::ostream &stderr_stream) {
  ::relay::v1::TickRequest request;
  std::vector<std::string> positional;
  try {
    for (size_t i = 0; i < args.size(); ++i) {
      const auto &arg = args[i];
      if (arg == "--sender") {
        if (i + 1 >= args.size()) {
          throw std::invalid_argument("--sender requires a value");
        }
        request.set_sender(args[++i]);
        continue;
      }
      if (arg.rfind("--sender=", 0) == 0) {
        request.set_sender(arg.substr(9));
        continue;
      }
      if (arg == "--note") {
        if (i + 1 >= args.size()) {
          throw std::invalid_argument("--note requires a value");
        }
        request.set_note(args[++i]);
        continue;
      }
      if (arg.rfind("--note=", 0) == 0) {
        request.set_note(arg.substr(7));
        continue;
      }
      if (arg.rfind("--", 0) == 0) {
        throw std::invalid_argument("unknown flag \"" + arg + "\"");
      }
      positional.push_back(arg);
    }
    if (request.sender().empty() && !positional.empty()) {
      request.set_sender(positional[0]);
    }
    if (request.note().empty() && positional.size() > 1) {
      request.set_note(positional[1]);
    }
    const auto response = Tick(request);
    stdout_stream << "{\"responder_slug\":\"" << JsonEscape(response.responder_slug())
                  << "\",\"responder_instance_uid\":\""
                  << JsonEscape(response.responder_instance_uid()) << "\"}\n";
    return 0;
  } catch (const std::exception &error) {
    stderr_stream << "tick: " << error.what() << '\n';
    return 1;
  }
}

} // namespace

int RunCLI(const std::vector<std::string> &args,
           std::ostream &stdout_stream,
           std::ostream &stderr_stream) {
  if (args.empty()) {
    PrintUsage(stderr_stream);
    return 1;
  }

  const auto command = CanonicalCommand(args.front());
  std::vector<std::string> rest(args.begin() + 1, args.end());
  if (command == "serve") {
    try {
      auto options = ParseServeOptions(rest);
      internal::Serve(options.listeners, options.reflect, std::move(options.members));
      return 0;
    } catch (const std::exception &error) {
      stderr_stream << "serve: " << error.what() << '\n';
      return 1;
    }
  }
  if (command == "tick") {
    return RunTick(rest, stdout_stream, stderr_stream);
  }
  if (command == "version") {
    stdout_stream << kVersion << '\n';
    return 0;
  }
  if (command == "help") {
    PrintUsage(stdout_stream);
    return 0;
  }

  stderr_stream << "unknown command \"" << args.front() << "\"\n";
  PrintUsage(stderr_stream);
  return 1;
}

void PrintUsage(std::ostream &output) {
  output << "usage: cascade-node-cpp <command> [args] [flags]\n\n";
  output << "commands:\n";
  output << "  serve [--listen <uri>] [--member <slug>=<address>]  Start the gRPC server\n";
  output << "  tick [sender] [note]                                Emit one local tick\n";
  output << "  version                                             Print version and exit\n";
  output << "  help                                                Print usage\n";
}

} // namespace cascade::node::cppholon::api
