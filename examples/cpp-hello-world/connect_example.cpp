#include "../../sdk/cpp-holons/include/holons/holons.hpp"

#include <chrono>
#include <cstdio>
#include <cstdlib>
#include <filesystem>
#include <fstream>
#include <iostream>
#include <optional>
#include <sstream>
#include <stdexcept>
#include <string>
#include <thread>

namespace {

std::string shell_quote(const std::string &value) {
  std::string quoted = "'";
  for (char ch : value) {
    if (ch == '\'') {
      quoted += "'\"'\"'";
    } else {
      quoted.push_back(ch);
    }
  }
  quoted += "'";
  return quoted;
}

std::filesystem::path project_root() {
  return std::filesystem::current_path();
}

std::filesystem::path sdk_binary(const std::string &name) {
  auto path = project_root() / "../../sdk/cpp-holons/bin" / name;
  path = std::filesystem::weakly_canonical(path);
  if (!std::filesystem::is_regular_file(path)) {
    throw std::runtime_error("missing SDK helper: " + path.string());
  }
  return path;
}

void write_executable(const std::filesystem::path &path,
                      const std::string &content) {
  std::ofstream out(path);
  if (!out.is_open()) {
    throw std::runtime_error("failed to write " + path.string());
  }
  out << content;
  out.close();

  auto perms = std::filesystem::status(path).permissions();
  std::filesystem::permissions(
      path,
      perms | std::filesystem::perms::owner_exec |
          std::filesystem::perms::group_exec |
          std::filesystem::perms::others_exec,
      std::filesystem::perm_options::replace);
}

void write_echo_holon(const std::filesystem::path &root,
                      const std::filesystem::path &wrapper_path) {
  auto holon_dir = root / "holons" / "echo-server";
  std::filesystem::create_directories(holon_dir);

  std::ofstream out(holon_dir / "holon.yaml");
  if (!out.is_open()) {
    throw std::runtime_error("failed to write holon.yaml");
  }

  out << "uuid: \"echo-server-connect-example\"\n"
      << "given_name: Echo\n"
      << "family_name: Server\n"
      << "motto: Reply precisely.\n"
      << "composer: \"connect-example\"\n"
      << "kind: service\n"
      << "build:\n"
      << "  runner: cpp\n"
      << "artifacts:\n"
      << "  binary: " << wrapper_path.string() << "\n";
}

std::optional<std::string> wait_for_target(const std::filesystem::path &path,
                                           std::chrono::milliseconds timeout) {
  auto deadline = std::chrono::steady_clock::now() + timeout;
  while (std::chrono::steady_clock::now() < deadline) {
    std::ifstream in(path);
    std::string line;
    if (in.is_open() && std::getline(in, line)) {
      line = holons::trim_copy(line);
      if (!line.empty()) {
        return line;
      }
    }
    std::this_thread::sleep_for(std::chrono::milliseconds(50));
  }
  return std::nullopt;
}

std::string go_target_for(const std::string &target) {
  if (target.rfind("tcp://", 0) == 0) {
    auto parsed = holons::parse_uri(target);
    auto host = parsed.host;
    if (host.empty() || host == "0.0.0.0" || host == "::" ||
        host == "[::]") {
      host = "127.0.0.1";
    }
    return host + ":" + std::to_string(parsed.port);
  }
  if (target.rfind("unix://", 0) == 0) {
    return target;
  }
  return target;
}

std::string run_ping(const std::string &target) {
  auto go_dir = std::filesystem::temp_directory_path() /
                std::filesystem::path("cpp-holons-connect-go");
  std::error_code ec;
  std::filesystem::remove_all(go_dir, ec);
  std::filesystem::create_directories(go_dir);

  write_executable(
      go_dir / "go.mod",
      "module cppholonsconnect\n\n"
      "go 1.22.0\n\n"
      "require google.golang.org/grpc v1.78.0\n");
  std::ofstream(go_dir / "main.go")
      << "package main\n"
         "import (\n"
         "  \"context\"\n"
         "  \"encoding/json\"\n"
         "  \"fmt\"\n"
         "  \"os\"\n"
         "  \"time\"\n"
         "  \"google.golang.org/grpc\"\n"
         "  \"google.golang.org/grpc/credentials/insecure\"\n"
         ")\n"
         "type PingRequest struct { Message string `json:\"message\"` }\n"
         "type PingResponse struct { Message string `json:\"message\"`; SDK string `json:\"sdk\"`; Version string `json:\"version\"` }\n"
         "type jsonCodec struct{}\n"
         "func (jsonCodec) Name() string { return \"json\" }\n"
         "func (jsonCodec) Marshal(v any) ([]byte, error) { return json.Marshal(v) }\n"
         "func (jsonCodec) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }\n"
         "func main() {\n"
         "  if len(os.Args) != 2 {\n"
         "    fmt.Fprintln(os.Stderr, \"target is required\")\n"
         "    os.Exit(2)\n"
         "  }\n"
         "  ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)\n"
         "  defer cancel()\n"
         "  conn, err := grpc.DialContext(ctx, os.Args[1], grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock(), grpc.WithDefaultCallOptions(grpc.ForceCodec(jsonCodec{})))\n"
         "  if err != nil {\n"
         "    fmt.Fprintln(os.Stderr, err)\n"
         "    os.Exit(1)\n"
         "  }\n"
         "  defer conn.Close()\n"
         "  var out PingResponse\n"
         "  if err := conn.Invoke(ctx, \"/echo.v1.Echo/Ping\", &PingRequest{Message: \"hello-from-cpp\"}, &out); err != nil {\n"
         "    fmt.Fprintln(os.Stderr, err)\n"
         "    os.Exit(1)\n"
         "  }\n"
         "  if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {\n"
         "    fmt.Fprintln(os.Stderr, err)\n"
         "    os.Exit(1)\n"
         "  }\n"
         "}\n";

  auto command = "cd " + shell_quote(go_dir.string()) + " && go run -mod=mod . " +
                 shell_quote(go_target_for(target));
  FILE *pipe = ::popen(command.c_str(), "r");
  if (pipe == nullptr) {
    std::filesystem::remove_all(go_dir, ec);
    throw std::runtime_error("failed to launch go");
  }

  std::string output;
  char buffer[256];
  while (std::fgets(buffer, sizeof(buffer), pipe) != nullptr) {
    output += buffer;
  }

  int status = ::pclose(pipe);
  std::filesystem::remove_all(go_dir, ec);
  if (status != 0) {
    throw std::runtime_error("go client exited with status " +
                             std::to_string(status));
  }
  return holons::trim_copy(output);
}

} // namespace

int main() {
  const auto root = std::filesystem::temp_directory_path() /
                    std::filesystem::path("cpp-holons-connect-example");
  std::error_code ec;
  std::filesystem::remove_all(root, ec);
  std::filesystem::create_directories(root);

  const auto advertised_target = root / "echo.addr";
  const auto echo_server = sdk_binary("echo-server");
  const auto wrapper_path = root / "echo-wrapper.sh";

  write_executable(
      wrapper_path,
      "#!/usr/bin/env bash\n"
      "set -euo pipefail\n"
      "ADDR_FILE=\"" + advertised_target.string() + "\"\n"
      "SERVER=\"" + echo_server.string() + "\"\n"
      "child=''\n"
      "cleanup() {\n"
      "  if [[ -n \"$child\" ]] && kill -0 \"$child\" >/dev/null 2>&1; then\n"
      "    kill -TERM \"$child\" >/dev/null 2>&1 || true\n"
      "    wait \"$child\" >/dev/null 2>&1 || true\n"
      "  fi\n"
      "}\n"
      "trap cleanup EXIT INT TERM\n"
      "\"$SERVER\" \"$@\" > >(tee \"$ADDR_FILE\") &\n"
      "child=$!\n"
      "wait \"$child\"\n");

  write_echo_holon(root, wrapper_path);

  const auto previous_cwd = std::filesystem::current_path();
  std::shared_ptr<grpc::Channel> channel;

  try {
    std::filesystem::current_path(root);
    channel = holons::connect("echo-server");

    const auto target =
        wait_for_target(advertised_target, std::chrono::seconds(5));
    if (!target.has_value()) {
      throw std::runtime_error("echo-server did not advertise a target");
    }

    std::cout << run_ping(*target) << '\n';
    holons::disconnect(channel);
    channel.reset();
  } catch (...) {
    holons::disconnect(channel);
    channel.reset();
    std::filesystem::current_path(previous_cwd);
    std::filesystem::remove_all(root, ec);
    throw;
  }

  std::filesystem::current_path(previous_cwd);
  std::filesystem::remove_all(root, ec);
  return 0;
}
