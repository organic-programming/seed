#include "api/cli.hpp"

#include "api/public.hpp"
#include "internal/server.hpp"

#include <algorithm>
#include <cctype>
#include <exception>
#include <iostream>
#include <string>
#include <string_view>

namespace gabriel::greeting::cppholon::api {
namespace {

enum class OutputFormat { kText, kJson };

struct CommandOptions {
  OutputFormat format = OutputFormat::kText;
  std::string lang;
  std::vector<std::string> positional;
};

struct ServeOptions {
  std::vector<std::string> listeners;
  bool reflect = false;
};

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

bool ParseFormat(const std::string &raw, OutputFormat *format, std::string *error) {
  const std::string normalized = CanonicalCommand(raw);
  if (normalized.empty() || normalized == "text" || normalized == "txt") {
    *format = OutputFormat::kText;
    return true;
  }
  if (normalized == "json") {
    *format = OutputFormat::kJson;
    return true;
  }
  *error = "unsupported format \"" + raw + "\"";
  return false;
}

bool ParseCommandOptions(const std::vector<std::string> &args,
                         CommandOptions *options,
                         std::string *error) {
  for (size_t i = 0; i < args.size(); ++i) {
    const std::string &arg = args[i];
    if (arg == "--json") {
      options->format = OutputFormat::kJson;
      continue;
    }
    if (arg == "--format") {
      if (i + 1 >= args.size()) {
        *error = "--format requires a value";
        return false;
      }
      if (!ParseFormat(args[++i], &options->format, error)) {
        return false;
      }
      continue;
    }
    if (arg.rfind("--format=", 0) == 0) {
      if (!ParseFormat(arg.substr(9), &options->format, error)) {
        return false;
      }
      continue;
    }
    if (arg == "--lang") {
      if (i + 1 >= args.size()) {
        *error = "--lang requires a value";
        return false;
      }
      options->lang = Trim(args[++i]);
      continue;
    }
    if (arg.rfind("--lang=", 0) == 0) {
      options->lang = Trim(arg.substr(7));
      continue;
    }
    if (arg.rfind("--", 0) == 0) {
      *error = "unknown flag \"" + arg + "\"";
      return false;
    }
    options->positional.push_back(arg);
  }
  return true;
}

bool ParseServeOptions(const std::vector<std::string> &args,
                       ServeOptions *options,
                         std::string *error) {
  for (size_t i = 0; i < args.size(); ++i) {
    const std::string &arg = args[i];
    if (arg == "--listen") {
      if (i + 1 >= args.size()) {
        *error = "--listen requires a value";
        return false;
      }
      options->listeners.push_back(args[++i]);
      continue;
    }
    if (arg.rfind("--listen=", 0) == 0) {
      options->listeners.push_back(arg.substr(9));
      continue;
    }
    if (arg == "--port") {
      if (i + 1 >= args.size()) {
        *error = "--port requires a value";
        return false;
      }
      options->listeners.push_back("tcp://:" + args[++i]);
      continue;
    }
    if (arg.rfind("--port=", 0) == 0) {
      options->listeners.push_back("tcp://:" + arg.substr(7));
      continue;
    }
    if (arg == "--reflect") {
      options->reflect = true;
      continue;
    }
    if (arg.rfind("--", 0) == 0) {
      *error = "unknown flag \"" + arg + "\"";
      return false;
    }
    *error = "serve accepts only --listen, --port, and --reflect";
    return false;
  }

  if (options->listeners.empty()) {
    options->listeners.push_back("tcp://:9090");
  }
  return true;
}

std::string JsonEscape(std::string_view value) {
  std::string escaped;
  escaped.reserve(value.size());
  for (unsigned char ch : value) {
    switch (ch) {
    case '\\':
      escaped += "\\\\";
      break;
    case '"':
      escaped += "\\\"";
      break;
    case '\b':
      escaped += "\\b";
      break;
    case '\f':
      escaped += "\\f";
      break;
    case '\n':
      escaped += "\\n";
      break;
    case '\r':
      escaped += "\\r";
      break;
    case '\t':
      escaped += "\\t";
      break;
    default:
      escaped.push_back(static_cast<char>(ch));
      break;
    }
  }
  return escaped;
}

void WriteListLanguages(const ::greeting::v1::ListLanguagesResponse &response,
                        OutputFormat format,
                        std::ostream &output) {
  if (format == OutputFormat::kText) {
    for (const auto &language : response.languages()) {
      output << language.code() << '\t' << language.name() << '\t'
             << language.native() << '\n';
    }
    return;
  }

  output << "{\"languages\":[";
  for (int i = 0; i < response.languages_size(); ++i) {
    const auto &language = response.languages(i);
    if (i != 0) {
      output << ',';
    }
    output << "{\"code\":\"" << JsonEscape(language.code()) << "\","
           << "\"name\":\"" << JsonEscape(language.name()) << "\","
           << "\"native\":\"" << JsonEscape(language.native()) << "\"}";
  }
  output << "]}\n";
}

void WriteSayHello(const ::greeting::v1::SayHelloResponse &response,
                   OutputFormat format,
                   std::ostream &output) {
  if (format == OutputFormat::kText) {
    output << response.greeting() << '\n';
    return;
  }

  output << "{\"greeting\":\"" << JsonEscape(response.greeting()) << "\","
         << "\"language\":\"" << JsonEscape(response.language()) << "\","
         << "\"langCode\":\"" << JsonEscape(response.lang_code()) << "\"}\n";
}

int RunListLanguages(const std::vector<std::string> &args,
                     std::ostream &stdout_stream,
                     std::ostream &stderr_stream) {
  CommandOptions options;
  std::string error;
  if (!ParseCommandOptions(args, &options, &error)) {
    stderr_stream << "listLanguages: " << error << '\n';
    return 1;
  }
  if (!options.positional.empty()) {
    stderr_stream << "listLanguages: accepts no positional arguments\n";
    return 1;
  }

  WriteListLanguages(ListLanguages(), options.format, stdout_stream);
  return 0;
}

int RunSayHello(const std::vector<std::string> &args,
                std::ostream &stdout_stream,
                std::ostream &stderr_stream) {
  CommandOptions options;
  std::string error;
  if (!ParseCommandOptions(args, &options, &error)) {
    stderr_stream << "sayHello: " << error << '\n';
    return 1;
  }
  if (options.positional.size() > 2) {
    stderr_stream << "sayHello: accepts at most <name> [lang_code]\n";
    return 1;
  }

  ::greeting::v1::SayHelloRequest request;
  request.set_lang_code("en");

  if (!options.positional.empty()) {
    request.set_name(options.positional[0]);
  }
  if (options.positional.size() == 2) {
    if (!options.lang.empty()) {
      stderr_stream << "sayHello: use either a positional lang_code or --lang, not both\n";
      return 1;
    }
    request.set_lang_code(options.positional[1]);
  }
  if (!options.lang.empty()) {
    request.set_lang_code(options.lang);
  }

  WriteSayHello(SayHello(request), options.format, stdout_stream);
  return 0;
}

} // namespace

int RunCLI(const std::vector<std::string> &args,
           std::ostream &stdout_stream,
           std::ostream &stderr_stream) {
  if (args.empty()) {
    PrintUsage(stderr_stream);
    return 1;
  }

  const std::string command = CanonicalCommand(args.front());
  const std::vector<std::string> tail(args.begin() + 1, args.end());

  if (command == "serve") {
    ServeOptions serve_options;
    std::string error;
    if (!ParseServeOptions(tail, &serve_options, &error)) {
      stderr_stream << "serve: " << error << '\n';
      return 1;
    }
    try {
      internal::Serve(serve_options.listeners, serve_options.reflect);
      return 0;
    } catch (const std::exception &error) {
      stderr_stream << "serve: " << error.what() << '\n';
      return 1;
    }
  }

  if (command == "version") {
    stdout_stream << kVersion << '\n';
    return 0;
  }
  if (command == "help") {
    PrintUsage(stdout_stream);
    return 0;
  }
  if (command == "listlanguages") {
    return RunListLanguages(tail, stdout_stream, stderr_stream);
  }
  if (command == "sayhello") {
    return RunSayHello(tail, stdout_stream, stderr_stream);
  }

  stderr_stream << "unknown command \"" << args.front() << "\"\n";
  PrintUsage(stderr_stream);
  return 1;
}

void PrintUsage(std::ostream &output) {
  output << "usage: gabriel-greeting-cpp <command> [args] [flags]\n\n";
  output << "commands:\n";
  output << "  serve [--listen <uri>] [--reflect]        Start the gRPC server\n";
  output << "  version                                  Print version and exit\n";
  output << "  help                                     Print usage\n";
  output << "  listLanguages [--format text|json]       List supported languages\n";
  output << "  sayHello [name] [lang_code] [--format text|json] [--lang <code>]\n\n";
  output << "examples:\n";
  output << "  gabriel-greeting-cpp serve --listen tcp://:9090\n";
  output << "  gabriel-greeting-cpp serve --listen tcp://:9090 --reflect\n";
  output << "  gabriel-greeting-cpp listLanguages --format json\n";
  output << "  gabriel-greeting-cpp sayHello Bob fr\n";
  output << "  gabriel-greeting-cpp sayHello Bob --lang fr --format json\n";
}

} // namespace gabriel::greeting::cppholon::api
