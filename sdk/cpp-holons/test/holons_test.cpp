#include "../include/holons/discover.hpp"
#include "../include/holons/describe.hpp"
#include "../include/holons/holons.hpp"
#include "../include/holons/identity.hpp"
#include "../include/holons/serve.hpp"

namespace {

std::filesystem::path make_temp_dir(const std::string &prefix) {
  auto stamp = std::to_string(
      std::chrono::high_resolution_clock::now().time_since_epoch().count());
  auto path = std::filesystem::temp_directory_path() / (prefix + stamp);
  std::filesystem::create_directories(path);
  return path;
}

void write_discovery_holon(const std::filesystem::path &dir,
                           const std::string &uuid,
                           const std::string &given_name,
                           const std::string &family_name,
                           const std::string &binary) {
  std::filesystem::create_directories(dir);
  std::ofstream out(dir / "holon.proto");
  out << "syntax = \"proto3\";\n"
      << "package holons.test.v1;\n\n"
      << "option (holons.v1.manifest) = {\n"
      << "  identity: {\n"
      << "    uuid: \"" << uuid << "\"\n"
      << "    given_name: \"" << given_name << "\"\n"
      << "    family_name: \"" << family_name << "\"\n"
      << "    motto: \"Test\"\n"
      << "    composer: \"test\"\n"
      << "    clade: \"deterministic/pure\"\n"
      << "    status: \"draft\"\n"
      << "    born: \"2026-03-07\"\n"
      << "  }\n"
      << "  kind: \"native\"\n"
      << "  build: {\n"
      << "    runner: \"go-module\"\n"
      << "  }\n"
      << "  artifacts: {\n"
      << "    binary: \"" << binary << "\"\n"
      << "  }\n"
      << "};\n";
}

void write_echo_holon(const std::filesystem::path &dir) {
  std::filesystem::create_directories(dir / "protos" / "echo" / "v1");

  {
    std::ofstream holon(dir / "holon.proto");
    holon << "syntax = \"proto3\";\n"
          << "package holons.test.v1;\n\n"
          << "option (holons.v1.manifest) = {\n"
          << "  identity: {\n"
          << "    given_name: \"Echo\"\n"
          << "    family_name: \"Server\"\n"
          << "    motto: \"Reply precisely.\"\n"
          << "  }\n"
          << "};\n";
  }

  {
    std::ofstream proto(dir / "protos" / "echo" / "v1" / "echo.proto");
    proto << "syntax = \"proto3\";\n"
          << "package echo.v1;\n\n"
          << "// Echo echoes request payloads for documentation tests.\n"
          << "service Echo {\n"
          << "  // Ping echoes the inbound message.\n"
          << "  // @example {\"message\":\"hello\",\"sdk\":\"go-holons\"}\n"
          << "  rpc Ping(PingRequest) returns (PingResponse);\n"
          << "}\n\n"
          << "message PingRequest {\n"
          << "  // Message to echo back.\n"
          << "  // @required\n"
          << "  // @example \"hello\"\n"
          << "  string message = 1;\n\n"
          << "  // SDK marker included in the response.\n"
          << "  // @example \"go-holons\"\n"
          << "  string sdk = 2;\n"
          << "}\n\n"
          << "message PingResponse {\n"
          << "  // Echoed message.\n"
          << "  string message = 1;\n\n"
          << "  // SDK marker from the server.\n"
          << "  string sdk = 2;\n"
          << "}\n";
  }
}

std::optional<std::string> capture_env(const char *name) {
  if (const char *value = std::getenv(name); value != nullptr) {
    return std::string(value);
  }
  return std::nullopt;
}

void restore_env(const char *name, const std::optional<std::string> &value) {
#ifdef _WIN32
  if (value.has_value()) {
    _putenv_s(name, value->c_str());
  } else {
    _putenv_s(name, "");
  }
#else
  if (value.has_value()) {
    ::setenv(name, value->c_str(), 1);
  } else {
    ::unsetenv(name);
  }
#endif
}

} // namespace

#ifdef _WIN32

#include <winsock2.h>
#include <ws2tcpip.h>
#include <cassert>
#include <cstdio>
#include <filesystem>
#include <fstream>
#include <string>

namespace {

int connect_tcp(const std::string &host, int port) {
  SOCKET fd = ::socket(AF_INET, SOCK_STREAM, 0);
  if (fd == INVALID_SOCKET) {
    return -1;
  }

  sockaddr_in addr{};
  addr.sin_family = AF_INET;
  addr.sin_port = htons(static_cast<uint16_t>(port));
  if (::inet_pton(AF_INET, host.c_str(), &addr.sin_addr) != 1) {
    ::closesocket(fd);
    return -1;
  }

  if (::connect(fd, reinterpret_cast<sockaddr *>(&addr), sizeof(addr)) != 0) {
    ::closesocket(fd);
    return -1;
  }

  return static_cast<int>(fd);
}

} // namespace

int main() {
  int passed = 0;

  assert(holons::scheme("tcp://:9090") == "tcp");
  ++passed;
  assert(holons::scheme("unix:///tmp/x.sock") == "unix");
  ++passed;
  assert(holons::scheme("stdio://") == "stdio");
  ++passed;

  auto parsed = holons::parse_uri("wss://example.com:8443");
  assert(parsed.scheme == "wss");
  ++passed;
  assert(parsed.host == "example.com");
  ++passed;
  assert(parsed.port == 8443);
  ++passed;
  assert(parsed.path == "/grpc");
  ++passed;
  assert(parsed.secure);
  ++passed;

  auto tcp_lis = holons::listen("tcp://127.0.0.1:0");
  auto *tcp = std::get_if<holons::tcp_listener>(&tcp_lis);
  assert(tcp != nullptr);
  ++passed;

  sockaddr_in addr{};
  int len = sizeof(addr);
  int rc = ::getsockname(static_cast<SOCKET>(tcp->fd),
                         reinterpret_cast<sockaddr *>(&addr), &len);
  assert(rc == 0);
  ++passed;

  int port = ntohs(addr.sin_port);
  assert(port > 0);
  ++passed;

  int cfd = connect_tcp("127.0.0.1", port);
  assert(cfd >= 0);
  ++passed;

  auto server_conn = holons::accept(tcp_lis);
  assert(server_conn.scheme == "tcp");
  ++passed;

  const char *ping = "ping";
  int wrote = ::send(static_cast<SOCKET>(cfd), ping, 4, 0);
  assert(wrote == 4);
  ++passed;

  char buf[8] = {0};
  auto read_n = holons::conn_read(server_conn, buf, sizeof(buf));
  assert(read_n == 4);
  ++passed;
  assert(std::string(buf, 4) == "ping");
  ++passed;

  holons::close_connection(server_conn);
  ::closesocket(static_cast<SOCKET>(cfd));
  holons::close_listener(tcp_lis);

  auto stdio_lis = holons::listen("stdio://");
  auto stdio_conn = holons::accept(stdio_lis);
  assert(stdio_conn.scheme == "stdio");
  ++passed;
  holons::close_connection(stdio_conn);

  try {
    (void)holons::listen("unix://C:/tmp/holons.sock");
    assert(false && "unix:// should throw on Windows");
  } catch (const std::runtime_error &) {
    ++passed;
  }

  assert(holons::parse_flags({"--listen", "tcp://:8080"}) == "tcp://:8080");
  ++passed;
  assert(holons::parse_flags({"--port", "3000"}) == "tcp://:3000");
  ++passed;
  assert(holons::parse_flags({}) == "tcp://:9090");
  ++passed;

  auto temp_path = std::filesystem::temp_directory_path() /
                   "holons_cpp_windows_test_holon.proto";
  {
    std::ofstream f(temp_path);
    f << "syntax = \"proto3\";\n"
      << "package test.v1;\n\n"
      << "option (holons.v1.manifest) = {\n"
      << "  identity: {\n"
      << "    uuid: \"abc-123\"\n"
      << "    given_name: \"test\"\n"
      << "    family_name: \"Test\"\n"
      << "  }\n"
      << "  lang: \"cpp\"\n"
      << "};\n";
  }
  auto id = holons::parse_holon(temp_path.string());
  assert(id.uuid == "abc-123");
  ++passed;
  assert(id.given_name == "test");
  ++passed;
  std::filesystem::remove(temp_path);

  auto describe_root = make_temp_dir("holons_cpp_describe_windows_");
  write_echo_holon(describe_root);

  auto response = holons::describe::build_response(describe_root / "protos");
  assert(response.manifest.identity.given_name == "Echo");
  ++passed;
  assert(response.manifest.identity.family_name == "Server");
  ++passed;
  assert(response.manifest.identity.motto == "Reply precisely.");
  ++passed;
  assert(response.services.size() == 1);
  ++passed;
  assert(response.services[0].name == "echo.v1.Echo");
  ++passed;
  assert(response.services[0].description ==
         "Echo echoes request payloads for documentation tests.");
  ++passed;
  assert(response.services[0].methods.size() == 1);
  ++passed;
  assert(response.services[0].methods[0].name == "Ping");
  ++passed;
  assert(response.services[0].methods[0].example_input ==
         "{\"message\":\"hello\",\"sdk\":\"go-holons\"}");
  ++passed;
  assert(response.services[0].methods[0].input_fields.size() == 2);
  ++passed;
  assert(response.services[0].methods[0].input_fields[0].name == "message");
  ++passed;
  assert(response.services[0].methods[0].input_fields[0].label ==
         holons::describe::field_label::required);
  ++passed;
  assert(response.services[0].methods[0].input_fields[0].example == "\"hello\"");
  ++passed;

  auto registration =
      holons::describe::make_registration(describe_root / "protos");
  assert(registration.service_name == "holons.v1.HolonMeta");
  ++passed;
  assert(registration.method_name == "Describe");
  ++passed;
  auto registered =
      registration.handler(holons::describe::describe_request{});
  assert(registered.manifest.identity.given_name == "Echo");
  ++passed;
  assert(registered.services.size() == 1);
  ++passed;

  auto empty_root = make_temp_dir("holons_cpp_describe_empty_windows_");
  {
    std::ofstream holon(empty_root / "holon.proto");
    holon << "syntax = \"proto3\";\n"
          << "package holons.test.v1;\n\n"
          << "option (holons.v1.manifest) = {\n"
          << "  identity: {\n"
          << "    given_name: \"Empty\"\n"
          << "    family_name: \"Holon\"\n"
          << "    motto: \"Still available.\"\n"
          << "  }\n"
          << "};\n";
  }
  auto empty_response =
      holons::describe::build_response(empty_root / "protos");
  assert(empty_response.manifest.identity.given_name == "Empty");
  ++passed;
  assert(empty_response.manifest.identity.family_name == "Holon");
  ++passed;
  assert(empty_response.manifest.identity.motto == "Still available.");
  ++passed;
  assert(empty_response.services.empty());
  ++passed;
  std::filesystem::remove_all(describe_root);
  std::filesystem::remove_all(empty_root);

  auto discover_root = make_temp_dir("holons_cpp_windows_discover_");
  write_discovery_holon(discover_root / "holons" / "alpha", "uuid-alpha", "Alpha",
                        "Go", "alpha-go");
  write_discovery_holon(discover_root / "nested" / "beta", "uuid-beta", "Beta",
                        "Rust", "beta-rust");
  write_discovery_holon(discover_root / "nested" / "dup" / "alpha", "uuid-alpha",
                        "Alpha", "Go", "alpha-go");
  write_discovery_holon(discover_root / ".git" / "hidden", "uuid-hidden", "Hidden",
                        "Holon", "hidden");

  auto discovered = holons::discover(discover_root);
  assert(discovered.size() == 2);
  ++passed;
  assert(discovered[0].slug == "alpha-go");
  ++passed;
  assert(discovered[0].relative_path.generic_string() == "holons/alpha");
  ++passed;
  assert(discovered[0].manifest.has_value());
  ++passed;
  assert(discovered[1].slug == "beta-rust");
  ++passed;
  std::filesystem::remove_all(discover_root);

  std::printf("%d passed, 0 failed\n", passed);
  return 0;
}

#else

#include <arpa/inet.h>
#include <cassert>
#include <cerrno>
#include <chrono>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <fcntl.h>
#include <netinet/in.h>
#include <poll.h>
#include <signal.h>
#include <string>
#include <sys/stat.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <sys/wait.h>
#include <unistd.h>

namespace {

using json = nlohmann::json;

int connect_tcp(const std::string &host, int port) {
  int fd = ::socket(AF_INET, SOCK_STREAM, 0);
  if (fd < 0) {
    return -1;
  }

  sockaddr_in addr{};
  addr.sin_family = AF_INET;
  addr.sin_port = htons(static_cast<uint16_t>(port));
  if (::inet_pton(AF_INET, host.c_str(), &addr.sin_addr) != 1) {
    ::close(fd);
    return -1;
  }

  if (::connect(fd, reinterpret_cast<sockaddr *>(&addr), sizeof(addr)) != 0) {
    ::close(fd);
    return -1;
  }

  return fd;
}

int connect_unix(const std::string &path) {
  int fd = ::socket(AF_UNIX, SOCK_STREAM, 0);
  if (fd < 0) {
    return -1;
  }

  sockaddr_un addr{};
  addr.sun_family = AF_UNIX;
  std::snprintf(addr.sun_path, sizeof(addr.sun_path), "%s", path.c_str());
  if (::connect(fd, reinterpret_cast<sockaddr *>(&addr), sizeof(addr)) != 0) {
    ::close(fd);
    return -1;
  }

  return fd;
}

std::string make_temp_proto_path() {
  char tmpl[] = "/tmp/holons_cpp_test_XXXXXX";
  int fd = ::mkstemp(tmpl);
  assert(fd >= 0);
  ::close(fd);
  std::string path = std::string(tmpl) + ".proto";
  std::remove(tmpl);
  return path;
}

std::string make_temp_unix_socket_path() {
  char tmpl[] = "/tmp/holons_cpp_sock_XXXXXX";
  int fd = ::mkstemp(tmpl);
  assert(fd >= 0);
  ::close(fd);
  std::string path = std::string(tmpl) + ".sock";
  std::remove(tmpl);
  return path;
}

#if HOLONS_HAS_HOLONMETA_GRPC
holons::v1::DescribeResponse make_static_describe_response() {
  holons::v1::DescribeResponse response;
  auto *identity = response.mutable_manifest()->mutable_identity();
  identity->set_schema("holon/v1");
  identity->set_uuid("static-holon-0000");
  identity->set_given_name("Static");
  identity->set_family_name("Holon");
  identity->set_motto("Registered from generated code.");
  identity->set_composer("cpp-holons-test");
  identity->set_status("draft");
  identity->set_born("2026-03-23");
  response.mutable_manifest()->set_lang("cpp");

  auto *service = response.add_services();
  service->set_name("static.v1.Echo");
  service->set_description("Static test service.");
  auto *method = service->add_methods();
  method->set_name("Ping");
  method->set_description("Replies with the payload.");
  return response;
}
#endif

std::string resolve_go_binary() {
  std::string preferred = "/Users/bpds/go/go1.25.1/bin/go";
  if (::access(preferred.c_str(), X_OK) == 0) {
    return preferred;
  }
  return "go";
}

std::string find_sdk_dir() {
  char cwd[4096] = {0};
  if (::getcwd(cwd, sizeof(cwd)) == nullptr) {
    throw std::runtime_error("getcwd failed");
  }

  std::string dir(cwd);
  for (int i = 0; i < 12; ++i) {
    std::string candidate = dir + "/go-holons";
    if (::access(candidate.c_str(), F_OK) == 0) {
      return dir;
    }
    auto slash = dir.find_last_of('/');
    if (slash == std::string::npos || slash == 0) {
      break;
    }
    dir = dir.substr(0, slash);
  }

  throw std::runtime_error("unable to locate sdk directory containing go-holons");
}

bool read_line_with_timeout(int fd, std::string &out, int timeout_ms) {
  out.clear();
  int elapsed = 0;

  while (elapsed < timeout_ms) {
    pollfd pfd{};
    pfd.fd = fd;
    pfd.events = POLLIN;
    int rc = ::poll(&pfd, 1, 100);
    if (rc < 0) {
      return false;
    }
    if (rc == 0) {
      elapsed += 100;
      continue;
    }
    if ((pfd.revents & POLLIN) == 0) {
      continue;
    }

    char ch = '\0';
    ssize_t n = ::read(fd, &ch, 1);
    if (n <= 0) {
      return false;
    }
    if (ch == '\n') {
      return true;
    }
    out.push_back(ch);
  }

  return false;
}

std::string read_available(int fd) {
  std::string out;
  char buf[1024];

  for (int i = 0; i < 20; ++i) {
    pollfd pfd{};
    pfd.fd = fd;
    pfd.events = POLLIN;
    int rc = ::poll(&pfd, 1, 50);
    if (rc <= 0 || (pfd.revents & POLLIN) == 0) {
      if (!out.empty()) {
        break;
      }
      continue;
    }

    ssize_t n = ::read(fd, buf, sizeof(buf));
    if (n <= 0) {
      break;
    }
    out.append(buf, static_cast<size_t>(n));
  }

  return out;
}

bool is_bind_restricted_errno(int err) {
  return err == EPERM || err == EACCES;
}

bool loopback_bind_restricted(std::string &reason) {
  reason.clear();

  int fd = ::socket(AF_INET, SOCK_STREAM, 0);
  if (fd < 0) {
    reason = "socket() failed: " + std::string(std::strerror(errno));
    return false;
  }

  int one = 1;
  ::setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &one, sizeof(one));

  sockaddr_in addr{};
  addr.sin_family = AF_INET;
  addr.sin_port = htons(0);
  if (::inet_pton(AF_INET, "127.0.0.1", &addr.sin_addr) != 1) {
    ::close(fd);
    reason = "inet_pton(127.0.0.1) failed";
    return false;
  }

  if (::bind(fd, reinterpret_cast<sockaddr *>(&addr), sizeof(addr)) != 0) {
    int err = errno;
    reason = std::strerror(err);
    ::close(fd);
    return is_bind_restricted_errno(err);
  }

  if (::listen(fd, 1) != 0) {
    int err = errno;
    reason = std::strerror(err);
    ::close(fd);
    return is_bind_restricted_errno(err);
  }

  ::close(fd);
  return false;
}

std::string read_file_text(const std::string &path) {
  std::ifstream file(path);
  if (!file.is_open()) {
    return "";
  }
  return std::string((std::istreambuf_iterator<char>(file)),
                     std::istreambuf_iterator<char>());
}

int command_exit_code(const std::string &cmd) {
  int status = ::system(cmd.c_str());
  if (status == -1 || !WIFEXITED(status)) {
    return -1;
  }
  return WEXITSTATUS(status);
}

int run_bash_script(const std::string &script_body) {
  char path[] = "/tmp/holons_cpp_script_XXXXXX";
  int fd = ::mkstemp(path);
  if (fd < 0) {
    return -1;
  }

  FILE *script = ::fdopen(fd, "w");
  if (script == nullptr) {
    ::close(fd);
    ::unlink(path);
    return -1;
  }

  std::fputs("#!/usr/bin/env bash\n", script);
  std::fputs("set -euo pipefail\n", script);
  std::fputs(script_body.c_str(), script);
  if (std::ferror(script)) {
    std::fclose(script);
    ::unlink(path);
    return -1;
  }
  std::fclose(script);

  if (::chmod(path, 0700) != 0) {
    ::unlink(path);
    return -1;
  }

  int rc = command_exit_code(path);
  ::unlink(path);
  return rc;
}

struct cwd_guard {
  std::filesystem::path previous;

  explicit cwd_guard(const std::filesystem::path &next)
      : previous(std::filesystem::current_path()) {
    std::filesystem::current_path(next);
  }

  ~cwd_guard() { std::filesystem::current_path(previous); }
};

struct env_guard {
  std::string name;
  std::optional<std::string> previous;

  env_guard(const char *env_name, const std::string &value)
      : name(env_name), previous(capture_env(env_name)) {
#ifdef _WIN32
    _putenv_s(name.c_str(), value.c_str());
#else
    ::setenv(name.c_str(), value.c_str(), 1);
#endif
  }

  ~env_guard() { restore_env(name.c_str(), previous); }
};

struct child_process {
  pid_t pid = -1;
  int stdout_fd = -1;
  int stderr_fd = -1;

  ~child_process() {
    if (pid > 0) {
      ::kill(pid, SIGTERM);
      int status = 0;
      for (int i = 0; i < 80; ++i) {
        auto rc = ::waitpid(pid, &status, WNOHANG);
        if (rc == pid || rc < 0) {
          pid = -1;
          break;
        }
        std::this_thread::sleep_for(std::chrono::milliseconds(25));
      }
      if (pid > 0) {
        ::kill(pid, SIGKILL);
        (void)::waitpid(pid, &status, 0);
        pid = -1;
      }
    }

    if (stdout_fd >= 0) {
      ::close(stdout_fd);
      stdout_fd = -1;
    }
    if (stderr_fd >= 0) {
      ::close(stderr_fd);
      stderr_fd = -1;
    }
  }
};

child_process start_child_process(const std::string &binary,
                                  const std::vector<std::string> &args,
                                  const std::filesystem::path &cwd = {}) {
  int stdout_pipe[2] = {-1, -1};
  int stderr_pipe[2] = {-1, -1};
  assert(::pipe(stdout_pipe) == 0);
  assert(::pipe(stderr_pipe) == 0);

  auto pid = ::fork();
  assert(pid >= 0);

  if (pid == 0) {
    ::dup2(stdout_pipe[1], STDOUT_FILENO);
    ::dup2(stderr_pipe[1], STDERR_FILENO);
    ::close(stdout_pipe[0]);
    ::close(stdout_pipe[1]);
    ::close(stderr_pipe[0]);
    ::close(stderr_pipe[1]);

    if (!cwd.empty()) {
      ::chdir(cwd.c_str());
    }

    std::vector<char *> argv;
    argv.reserve(args.size() + 2);
    argv.push_back(const_cast<char *>(binary.c_str()));
    for (const auto &arg : args) {
      argv.push_back(const_cast<char *>(arg.c_str()));
    }
    argv.push_back(nullptr);
    ::execv(binary.c_str(), argv.data());
    std::perror("execv");
    ::_exit(127);
  }

  ::close(stdout_pipe[1]);
  ::close(stderr_pipe[1]);
  return child_process{pid, stdout_pipe[0], stderr_pipe[0]};
}

std::string wait_for_child_uri(child_process &child, int timeout_ms) {
  std::string line;
  if (read_line_with_timeout(child.stdout_fd, line, timeout_ms)) {
    return holons::trim_copy(line);
  }
  auto stderr_text = read_available(child.stderr_fd);
  throw std::runtime_error("child did not advertise a URI: " + stderr_text);
}

bool pid_exists(pid_t pid) {
  if (pid <= 0) {
    return false;
  }
  int status = 0;
  auto waited = ::waitpid(pid, &status, WNOHANG);
  if (waited == pid) {
    return false;
  }
  if (waited < 0 && errno != ECHILD) {
    return false;
  }
  return ::kill(pid, 0) == 0 || errno == EPERM;
}

void wait_for_process_exit(pid_t pid, int timeout_ms = 2000) {
  auto deadline = std::chrono::steady_clock::now() +
                  std::chrono::milliseconds(timeout_ms);
  while (std::chrono::steady_clock::now() < deadline) {
    if (!pid_exists(pid)) {
      return;
    }
    std::this_thread::sleep_for(std::chrono::milliseconds(25));
  }
  assert(false && "process still running after timeout");
}

int reserve_loopback_port() {
  int fd = ::socket(AF_INET, SOCK_STREAM, 0);
  assert(fd >= 0);

  int one = 1;
  ::setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &one, sizeof(one));

  sockaddr_in addr{};
  addr.sin_family = AF_INET;
  addr.sin_port = htons(0);
  assert(::inet_pton(AF_INET, "127.0.0.1", &addr.sin_addr) == 1);
  assert(::bind(fd, reinterpret_cast<sockaddr *>(&addr), sizeof(addr)) == 0);
  assert(::listen(fd, 1) == 0);

  socklen_t len = sizeof(addr);
  assert(::getsockname(fd, reinterpret_cast<sockaddr *>(&addr), &len) == 0);
  int port = ntohs(addr.sin_port);
  ::close(fd);
  return port;
}

std::string connect_slug(const std::string &given_name,
                         const std::string &family_name) {
  auto join = given_name + "-" + family_name;
  std::string slug;
  slug.reserve(join.size());
  for (char ch : join) {
    if (std::isspace(static_cast<unsigned char>(ch))) {
      slug.push_back('-');
    } else {
      slug.push_back(static_cast<char>(
          std::tolower(static_cast<unsigned char>(ch))));
    }
  }
  return slug;
}

struct connect_fixture {
  std::filesystem::path root;
  std::string slug;
  std::filesystem::path pid_file;
  std::filesystem::path args_file;
  std::filesystem::path fd_mode_file;
  std::filesystem::path binary_path;
};

connect_fixture write_connect_fixture(const std::string &given_name,
                                      const std::string &family_name) {
  auto root = make_temp_dir("holons_cpp_connect_");
  auto slug = connect_slug(given_name, family_name);
  auto pid_file = root / (slug + ".pid");
  auto args_file = root / (slug + ".args");
  auto fd_mode_file = root / (slug + ".fdmode");
  auto binary_path = root / (slug + "-server.sh");
  auto sdk_root = std::filesystem::path(find_sdk_dir()) / "cpp-holons";
  auto echo_server = sdk_root / "bin" / "echo-server";

  {
    std::ofstream script(binary_path);
    assert(script.is_open());
    script << "#!/usr/bin/env bash\n"
           << "set -euo pipefail\n"
           << "echo $$ > " << pid_file << "\n"
           << ": > " << args_file << "\n"
           << "for arg in \"$@\"; do echo \"$arg\" >> " << args_file << "; done\n"
           << "python3 -c 'import os, pathlib; "
           << "a = os.fstat(0); "
           << "b = os.fstat(1); "
           << "pathlib.Path(r\"" << fd_mode_file.string() << "\").write_text("
           << "\"same\" if (a.st_dev, a.st_ino) == (b.st_dev, b.st_ino) else \"different\")'\n"
           << "exec " << echo_server << " \"$@\"\n";
  }
  assert(::chmod(binary_path.c_str(), 0700) == 0);

  auto holon_dir = root / "holons" / slug;
  std::filesystem::create_directories(holon_dir);
  {
    std::ofstream manifest(holon_dir / "holon.proto");
    assert(manifest.is_open());
    manifest << "syntax = \"proto3\";\n"
             << "package holons.test.v1;\n\n"
             << "option (holons.v1.manifest) = {\n"
             << "  identity: {\n"
             << "    uuid: \"" << slug << "-uuid\"\n"
             << "    given_name: \"" << given_name << "\"\n"
             << "    family_name: \"" << family_name << "\"\n"
             << "    composer: \"connect-test\"\n"
             << "  }\n"
             << "  kind: \"service\"\n"
             << "  build: {\n"
             << "    runner: \"shell\"\n"
             << "  }\n"
             << "  artifacts: {\n"
             << "    binary: \"" << binary_path.string() << "\"\n"
             << "  }\n"
             << "};\n";
  }

  return connect_fixture{root, slug, pid_file, args_file, fd_mode_file,
                         binary_path};
}

void write_port_file(const std::filesystem::path &path, const std::string &uri) {
  std::filesystem::create_directories(path.parent_path());
  std::ofstream out(path);
  assert(out.is_open());
  out << holons::trim_copy(uri) << "\n";
}

pid_t read_pid_file(const std::filesystem::path &path) {
  for (int i = 0; i < 80; ++i) {
    std::ifstream in(path);
    pid_t pid = -1;
    if (in.is_open() && (in >> pid) && pid > 0) {
      return pid;
    }
    std::this_thread::sleep_for(std::chrono::milliseconds(25));
  }
  return -1;
}

#if HOLONS_HAS_GRPCPP
bool channel_ready(const std::shared_ptr<grpc::Channel> &channel,
                   int timeout_ms = 1000) {
  if (!channel) {
    return false;
  }
  auto deadline = std::chrono::system_clock::now() +
                  std::chrono::milliseconds(timeout_ms);
  return channel->WaitForConnected(deadline);
}
#endif

const char *kGoHolonRPCServerSource = R"GO(
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"nhooyr.io/websocket"
)

type rpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

func main() {
	mode := "echo"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	var heartbeatCount int64
	var dropped int32

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			Subprotocols:       []string{"holon-rpc"},
			InsecureSkipVerify: true,
		})
		if err != nil {
			http.Error(w, "upgrade failed", http.StatusBadRequest)
			return
		}
		defer c.CloseNow()

		ctx := r.Context()
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}

			var msg rpcMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				_ = writeError(ctx, c, nil, -32700, "parse error")
				continue
			}
			if msg.JSONRPC != "2.0" {
				_ = writeError(ctx, c, msg.ID, -32600, "invalid request")
				continue
			}
			if msg.Method == "" {
				continue
			}

			switch msg.Method {
			case "rpc.heartbeat":
				atomic.AddInt64(&heartbeatCount, 1)
				_ = writeResult(ctx, c, msg.ID, map[string]interface{}{})
			case "echo.v1.Echo/Ping":
				var params map[string]interface{}
				_ = json.Unmarshal(msg.Params, &params)
				if params == nil {
					params = map[string]interface{}{}
				}
				_ = writeResult(ctx, c, msg.ID, params)
				if mode == "drop-once" && atomic.CompareAndSwapInt32(&dropped, 0, 1) {
					time.Sleep(100 * time.Millisecond)
					_ = c.Close(websocket.StatusNormalClosure, "drop once")
					return
				}
			case "echo.v1.Echo/HeartbeatCount":
				_ = writeResult(ctx, c, msg.ID, map[string]interface{}{"count": atomic.LoadInt64(&heartbeatCount)})
			case "echo.v1.Echo/CallClient":
				callID := "s1"
				if err := writeRequest(ctx, c, callID, "client.v1.Client/Hello", map[string]interface{}{"name": "go"}); err != nil {
					_ = writeError(ctx, c, msg.ID, 13, err.Error())
					continue
				}

				innerResult, callErr := waitForResponse(ctx, c, callID)
				if callErr != nil {
					_ = writeError(ctx, c, msg.ID, 13, callErr.Error())
					continue
				}
				_ = writeResult(ctx, c, msg.ID, innerResult)
			default:
				_ = writeError(ctx, c, msg.ID, -32601, fmt.Sprintf("method %q not found", msg.Method))
			}
		}
	})

	srv := &http.Server{Handler: h}
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("server error: %v", err)
		}
	}()

	fmt.Printf("ws://%s/rpc\n", ln.Addr().String())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func writeRequest(ctx context.Context, c *websocket.Conn, id interface{}, method string, params map[string]interface{}) error {
	payload, err := json.Marshal(rpcMessage{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  mustRaw(params),
	})
	if err != nil {
		return err
	}
	return c.Write(ctx, websocket.MessageText, payload)
}

func writeResult(ctx context.Context, c *websocket.Conn, id interface{}, result interface{}) error {
	payload, err := json.Marshal(rpcMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result:  mustRaw(result),
	})
	if err != nil {
		return err
	}
	return c.Write(ctx, websocket.MessageText, payload)
}

func writeError(ctx context.Context, c *websocket.Conn, id interface{}, code int, message string) error {
	payload, err := json.Marshal(rpcMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: &rpcError{
			Code:    code,
			Message: message,
		},
	})
	if err != nil {
		return err
	}
	return c.Write(ctx, websocket.MessageText, payload)
}

func waitForResponse(ctx context.Context, c *websocket.Conn, expectedID string) (map[string]interface{}, error) {
	deadlineCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	for {
		_, data, err := c.Read(deadlineCtx)
		if err != nil {
			return nil, err
		}

		var msg rpcMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		id, _ := msg.ID.(string)
		if id != expectedID {
			continue
		}
		if msg.Error != nil {
			return nil, fmt.Errorf("client error: %d %s", msg.Error.Code, msg.Error.Message)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(msg.Result, &out); err != nil {
			return nil, err
		}
		return out, nil
	}
}

func mustRaw(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return json.RawMessage(b)
}
)GO";

const char *kGoTransportHelperSource = R"GO(
package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/organic-programming/go-holons/pkg/holonrpc"
	"nhooyr.io/websocket"
)

type rpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

func main() {
	mode := "wss"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	certFile := ""
	if len(os.Args) > 2 {
		certFile = os.Args[2]
	}
	keyFile := ""
	if len(os.Args) > 3 {
		keyFile = os.Args[3]
	}

	switch mode {
	case "wss":
		if err := writeSelfSignedCert(certFile, keyFile); err != nil {
			log.Fatal(err)
		}
		runWSS(certFile, keyFile)
	case "http":
		runHTTP("http", "", "")
	case "https":
		if err := writeSelfSignedCert(certFile, keyFile); err != nil {
			log.Fatal(err)
		}
		runHTTP("https", certFile, keyFile)
	default:
		log.Fatalf("unsupported mode %q", mode)
	}
}

func runWSS(certFile, keyFile string) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			Subprotocols: []string{"holon-rpc"},
		})
		if err != nil {
			http.Error(w, "upgrade failed", http.StatusBadRequest)
			return
		}
		defer c.CloseNow()

		ctx := r.Context()
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}

			var msg rpcMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				_ = writeError(ctx, c, nil, -32700, "parse error")
				continue
			}
			if msg.JSONRPC != "2.0" {
				_ = writeError(ctx, c, msg.ID, -32600, "invalid request")
				continue
			}

			switch msg.Method {
			case "rpc.heartbeat":
				_ = writeResult(ctx, c, msg.ID, map[string]interface{}{})
			case "echo.v1.Echo/Ping":
				var params map[string]interface{}
				_ = json.Unmarshal(msg.Params, &params)
				if params == nil {
					params = map[string]interface{}{}
				}
				params["transport"] = "wss"
				_ = writeResult(ctx, c, msg.ID, params)
			default:
				_ = writeError(ctx, c, msg.ID, -32601, fmt.Sprintf("method %q not found", msg.Method))
			}
		}
	})

	srv := &http.Server{Handler: h}
	go func() {
		if err := srv.ServeTLS(ln, certFile, keyFile); err != nil && err != http.ErrServerClosed {
			log.Printf("server error: %v", err)
		}
	}()

	fmt.Printf("wss://%s/rpc?ca=%s\n", ln.Addr().String(), url.QueryEscape(certFile))
	waitForShutdown(func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	})
}

func runHTTP(scheme, certFile, keyFile string) {
	bindURL := fmt.Sprintf("%s://127.0.0.1:0/api/v1/rpc", scheme)
	if scheme == "https" {
		bindURL = fmt.Sprintf(
			"https://127.0.0.1:0/api/v1/rpc?cert=%s&key=%s",
			url.QueryEscape(certFile),
			url.QueryEscape(keyFile),
		)
	}

	server := holonrpc.NewHTTPServer(bindURL)
	server.Register("echo.v1.Echo/Ping", func(_ context.Context, params map[string]any) (map[string]any, error) {
		out := cloneParams(params)
		out["transport"] = scheme
		return out, nil
	})
	server.RegisterStream("echo.v1.Echo/Watch", func(_ context.Context, params map[string]any, send func(map[string]any) error) error {
		base := cloneParams(params)
		base["transport"] = scheme

		first := cloneParams(base)
		first["step"] = "1"
		if err := send(first); err != nil {
			return err
		}

		time.Sleep(20 * time.Millisecond)

		second := cloneParams(base)
		second["step"] = "2"
		return send(second)
	})

	addr, err := server.Start()
	if err != nil {
		log.Fatal(err)
	}

	if scheme == "https" {
		fmt.Printf("%s?ca=%s\n", addr, url.QueryEscape(certFile))
	} else {
		fmt.Println(addr)
	}

	waitForShutdown(func(ctx context.Context) error {
		return server.Close(ctx)
	})
}

func cloneParams(params map[string]any) map[string]any {
	out := make(map[string]any, len(params))
	for key, value := range params {
		out[key] = value
	}
	return out
}

func waitForShutdown(closeFn func(context.Context) error) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = closeFn(ctx)
}

func writeSelfSignedCert(certFile, keyFile string) error {
	if certFile == "" || keyFile == "" {
		return fmt.Errorf("cert and key paths are required")
	}
	if err := os.MkdirAll(filepath.Dir(certFile), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(keyFile), 0o755); err != nil {
		return err
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return err
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: "127.0.0.1",
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	if certPEM == nil || keyPEM == nil {
		return fmt.Errorf("encode pem failed")
	}

	if err := os.WriteFile(certFile, certPEM, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(keyFile, keyPEM, 0o600); err != nil {
		return err
	}
	return nil
}

func writeResult(ctx context.Context, c *websocket.Conn, id interface{}, result interface{}) error {
	payload, err := json.Marshal(rpcMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result:  mustRaw(result),
	})
	if err != nil {
		return err
	}
	return c.Write(ctx, websocket.MessageText, payload)
}

func writeError(ctx context.Context, c *websocket.Conn, id interface{}, code int, message string) error {
	payload, err := json.Marshal(rpcMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: &rpcError{
			Code:    code,
			Message: message,
		},
	})
	if err != nil {
		return err
	}
	return c.Write(ctx, websocket.MessageText, payload)
}

func mustRaw(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return json.RawMessage(b)
}
)GO";

struct go_helper_server {
  pid_t pid = -1;
  int stdout_fd = -1;
  int stderr_fd = -1;
  std::string helper_path;
  std::vector<std::string> cleanup_paths;

  ~go_helper_server() {
    if (pid > 0) {
      ::kill(pid, SIGTERM);
      int status = 0;
      for (int i = 0; i < 50; ++i) {
        pid_t rc = ::waitpid(pid, &status, WNOHANG);
        if (rc == pid) {
          break;
        }
        std::this_thread::sleep_for(std::chrono::milliseconds(100));
      }
      if (::waitpid(pid, &status, WNOHANG) == 0) {
        ::kill(pid, SIGKILL);
        ::waitpid(pid, &status, 0);
      }
    }

    if (stdout_fd >= 0) {
      ::close(stdout_fd);
    }
    if (stderr_fd >= 0) {
      ::close(stderr_fd);
    }
    if (!helper_path.empty()) {
      std::remove(helper_path.c_str());
    }
    for (const auto &path : cleanup_paths) {
      if (!path.empty()) {
        std::remove(path.c_str());
      }
    }
  }
};

go_helper_server start_go_helper(const std::string &mode) {
  auto sdk_dir = find_sdk_dir();
  auto go_dir = sdk_dir + "/go-holons";

  auto stamp = std::to_string(
      std::chrono::high_resolution_clock::now().time_since_epoch().count());
  std::string helper_path = go_dir + "/tmp-holonrpc-" + stamp + ".go";
  {
    std::ofstream out(helper_path);
    out << kGoHolonRPCServerSource;
  }

  int out_pipe[2] = {-1, -1};
  int err_pipe[2] = {-1, -1};
  if (::pipe(out_pipe) != 0 || ::pipe(err_pipe) != 0) {
    throw std::runtime_error("pipe() failed");
  }

  pid_t pid = ::fork();
  if (pid < 0) {
    throw std::runtime_error("fork() failed");
  }

  if (pid == 0) {
    ::dup2(out_pipe[1], STDOUT_FILENO);
    ::dup2(err_pipe[1], STDERR_FILENO);
    ::close(out_pipe[0]);
    ::close(out_pipe[1]);
    ::close(err_pipe[0]);
    ::close(err_pipe[1]);
    ::chdir(go_dir.c_str());

    auto go = resolve_go_binary();
    std::vector<char *> argv;
    argv.push_back(const_cast<char *>(go.c_str()));
    argv.push_back(const_cast<char *>("run"));
    argv.push_back(const_cast<char *>(helper_path.c_str()));
    argv.push_back(const_cast<char *>(mode.c_str()));
    argv.push_back(nullptr);
    ::execvp(go.c_str(), argv.data());
    std::perror("execvp");
    ::_exit(127);
  }

  ::close(out_pipe[1]);
  ::close(err_pipe[1]);

  go_helper_server server;
  server.pid = pid;
  server.stdout_fd = out_pipe[0];
  server.stderr_fd = err_pipe[0];
  server.helper_path = helper_path;
  return server;
}

template <typename Func> void with_go_helper(const std::string &mode, Func f) {
  auto server = start_go_helper(mode);
  std::string url;
  if (!read_line_with_timeout(server.stdout_fd, url, 20000)) {
    auto stderr_text = read_available(server.stderr_fd);
    throw std::runtime_error("Go holon-rpc helper did not output URL: " +
                             stderr_text);
  }
  f(url);
}

go_helper_server start_go_transport_helper(const std::string &mode) {
  auto sdk_dir = find_sdk_dir();
  auto go_dir = sdk_dir + "/go-holons";

  auto stamp = std::to_string(
      std::chrono::high_resolution_clock::now().time_since_epoch().count());
  std::string helper_path = go_dir + "/tmp-holonrpc-transport-" + stamp + ".go";
  std::string cert_path = go_dir + "/tmp-holonrpc-transport-" + stamp + ".crt";
  std::string key_path = go_dir + "/tmp-holonrpc-transport-" + stamp + ".key";
  {
    std::ofstream out(helper_path);
    out << kGoTransportHelperSource;
  }

  int out_pipe[2] = {-1, -1};
  int err_pipe[2] = {-1, -1};
  if (::pipe(out_pipe) != 0 || ::pipe(err_pipe) != 0) {
    throw std::runtime_error("pipe() failed");
  }

  pid_t pid = ::fork();
  if (pid < 0) {
    throw std::runtime_error("fork() failed");
  }

  if (pid == 0) {
    ::dup2(out_pipe[1], STDOUT_FILENO);
    ::dup2(err_pipe[1], STDERR_FILENO);
    ::close(out_pipe[0]);
    ::close(out_pipe[1]);
    ::close(err_pipe[0]);
    ::close(err_pipe[1]);
    ::chdir(go_dir.c_str());

    auto go = resolve_go_binary();
    std::vector<char *> argv;
    argv.push_back(const_cast<char *>(go.c_str()));
    argv.push_back(const_cast<char *>("run"));
    argv.push_back(const_cast<char *>(helper_path.c_str()));
    argv.push_back(const_cast<char *>(mode.c_str()));
    argv.push_back(const_cast<char *>(cert_path.c_str()));
    argv.push_back(const_cast<char *>(key_path.c_str()));
    argv.push_back(nullptr);
    ::execvp(go.c_str(), argv.data());
    std::perror("execvp");
    ::_exit(127);
  }

  ::close(out_pipe[1]);
  ::close(err_pipe[1]);

  go_helper_server server;
  server.pid = pid;
  server.stdout_fd = out_pipe[0];
  server.stderr_fd = err_pipe[0];
  server.helper_path = helper_path;
  server.cleanup_paths.push_back(cert_path);
  server.cleanup_paths.push_back(key_path);
  return server;
}

template <typename Func>
void with_go_transport_helper(const std::string &mode, Func f) {
  auto server = start_go_transport_helper(mode);
  std::string url;
  if (!read_line_with_timeout(server.stdout_fd, url, 20000)) {
    auto stderr_text = read_available(server.stderr_fd);
    throw std::runtime_error("Go transport helper did not output URL: " +
                             stderr_text);
  }
  f(url);
}

go_helper_server start_cpp_holonrpc_server(const std::string &bind_url,
                                           bool once) {
  auto sdk_dir = find_sdk_dir();
  auto cpp_dir = sdk_dir + "/cpp-holons";

  int out_pipe[2] = {-1, -1};
  int err_pipe[2] = {-1, -1};
  if (::pipe(out_pipe) != 0 || ::pipe(err_pipe) != 0) {
    throw std::runtime_error("pipe() failed");
  }

  pid_t pid = ::fork();
  if (pid < 0) {
    throw std::runtime_error("fork() failed");
  }

  if (pid == 0) {
    ::dup2(out_pipe[1], STDOUT_FILENO);
    ::dup2(err_pipe[1], STDERR_FILENO);
    ::close(out_pipe[0]);
    ::close(out_pipe[1]);
    ::close(err_pipe[0]);
    ::close(err_pipe[1]);
    ::chdir(cpp_dir.c_str());

    std::vector<std::string> args;
    args.emplace_back("./bin/holon-rpc-server");
    if (once) {
      args.emplace_back("--once");
    }
    args.emplace_back(bind_url);

    std::vector<char *> argv;
    argv.reserve(args.size() + 1);
    for (auto &arg : args) {
      argv.push_back(const_cast<char *>(arg.c_str()));
    }
    argv.push_back(nullptr);

    ::execv(args[0].c_str(), argv.data());
    std::perror("execv");
    ::_exit(127);
  }

  ::close(out_pipe[1]);
  ::close(err_pipe[1]);

  go_helper_server server;
  server.pid = pid;
  server.stdout_fd = out_pipe[0];
  server.stderr_fd = err_pipe[0];
  return server;
}

template <typename Func> void with_cpp_holonrpc_server(Func f) {
  auto server = start_cpp_holonrpc_server("ws://127.0.0.1:0/rpc", true);
  std::string url;
  if (!read_line_with_timeout(server.stdout_fd, url, 20000)) {
    auto stderr_text = read_available(server.stderr_fd);
    throw std::runtime_error(
        "cpp holon-rpc server did not output URL: " + stderr_text);
  }

  f(url);

  int status = 0;
  if (::waitpid(server.pid, &status, 0) != server.pid) {
    throw std::runtime_error("waitpid() failed for cpp holon-rpc server");
  }
  server.pid = -1;
  if (!WIFEXITED(status) || WEXITSTATUS(status) != 0) {
    throw std::runtime_error("cpp holon-rpc server exited with error");
  }
}

json invoke_eventually(holons::holon_rpc_client &client,
                       const std::string &method, const json &params) {
  std::exception_ptr last_error;
  for (int i = 0; i < 40; ++i) {
    try {
      return client.invoke(method, params);
    } catch (...) {
      last_error = std::current_exception();
      std::this_thread::sleep_for(std::chrono::milliseconds(120));
    }
  }
  if (last_error) {
    std::rethrow_exception(last_error);
  }
  throw std::runtime_error("invoke eventually failed");
}

} // namespace

int main() {
  int passed = 0;
  std::string bind_reason;
  bool bind_restricted = loopback_bind_restricted(bind_reason);

  // --- echo wrapper scripts ---
  {
    assert(::access("./bin/echo-client", F_OK) == 0);
    ++passed;
    assert(::access("./bin/echo-server", F_OK) == 0);
    ++passed;
    assert(::access("./bin/holon-rpc-server", F_OK) == 0);
    ++passed;
    assert(::access("./bin/echo-client", X_OK) == 0);
    ++passed;
    assert(::access("./bin/echo-server", X_OK) == 0);
    ++passed;
    assert(::access("./bin/holon-rpc-server", X_OK) == 0);
    ++passed;

    char fake_go[] = "/tmp/holons_cpp_fake_go_XXXXXX";
    char fake_log[] = "/tmp/holons_cpp_fake_go_log_XXXXXX";

    int fake_fd = ::mkstemp(fake_go);
    assert(fake_fd >= 0);
    int log_fd = ::mkstemp(fake_log);
    assert(log_fd >= 0);

    FILE *script = ::fdopen(fake_fd, "w");
    assert(script != nullptr);
    std::fprintf(
        script,
        "#!/usr/bin/env bash\n"
        "set -euo pipefail\n"
        ": \"${HOLONS_FAKE_GO_LOG:?missing HOLONS_FAKE_GO_LOG}\"\n"
        "{\n"
        "  printf 'PWD=%%s\\n' \"$PWD\"\n"
        "  i=0\n"
        "  for arg in \"$@\"; do\n"
        "    printf 'ARG%%d=%%s\\n' \"$i\" \"$arg\"\n"
        "    i=$((i+1))\n"
        "  done\n"
        "} >\"$HOLONS_FAKE_GO_LOG\"\n");
    std::fclose(script);
    ::close(log_fd);
    assert(::chmod(fake_go, 0700) == 0);
    ++passed;

    char *prev_go_bin = std::getenv("GO_BIN");
    char *prev_log = std::getenv("HOLONS_FAKE_GO_LOG");
    char *prev_gocache = std::getenv("GOCACHE");

    std::string prev_go_bin_value = prev_go_bin ? prev_go_bin : "";
    std::string prev_log_value = prev_log ? prev_log : "";
    std::string prev_gocache_value = prev_gocache ? prev_gocache : "";

    bool had_prev_go_bin = prev_go_bin != nullptr;
    bool had_prev_log = prev_log != nullptr;
    bool had_prev_gocache = prev_gocache != nullptr;

    ::setenv("GO_BIN", fake_go, 1);
    ::setenv("HOLONS_FAKE_GO_LOG", fake_log, 1);
    ::unsetenv("GOCACHE");

    int client_exit = command_exit_code(
        "./bin/echo-client stdio:// --message cert-stdio >/dev/null 2>&1");
    assert(client_exit == 0);
    ++passed;

    auto capture = read_file_text(fake_log);
    assert(!capture.empty());
    ++passed;
    assert(capture.find("PWD=") != std::string::npos &&
           capture.find("/sdk/go-holons") != std::string::npos);
    ++passed;
    assert(capture.find("ARG0=run") != std::string::npos);
    ++passed;
    assert(capture.find("echo-client-go/main.go") != std::string::npos);
    ++passed;
    assert(capture.find("--sdk") != std::string::npos &&
           capture.find("cpp-holons") != std::string::npos);
    ++passed;
    assert(capture.find("--server-sdk") != std::string::npos &&
           capture.find("go-holons") != std::string::npos);
    ++passed;
    assert(capture.find("stdio://") != std::string::npos);
    ++passed;
    assert(capture.find("--message") != std::string::npos &&
           capture.find("cert-stdio") != std::string::npos);
    ++passed;

    int server_exit =
        command_exit_code("./bin/echo-server --listen stdio:// >/dev/null 2>&1");
    assert(server_exit == 0);
    ++passed;

    capture = read_file_text(fake_log);
    assert(!capture.empty());
    ++passed;
    assert(capture.find("PWD=") != std::string::npos &&
           capture.find("/sdk/go-holons") != std::string::npos);
    ++passed;
    assert(capture.find("ARG0=run") != std::string::npos);
    ++passed;
    assert(capture.find("echo-server-go/main.go") != std::string::npos);
    ++passed;
    assert(capture.find("--sdk") != std::string::npos &&
           capture.find("cpp-holons") != std::string::npos);
    ++passed;
    assert(capture.find("--max-recv-bytes") != std::string::npos &&
           capture.find("1572864") != std::string::npos);
    ++passed;
    assert(capture.find("--max-send-bytes") != std::string::npos &&
           capture.find("1572864") != std::string::npos);
    ++passed;
    assert(capture.find("--listen") != std::string::npos &&
           capture.find("stdio://") != std::string::npos);
    ++passed;

    server_exit = command_exit_code(
        "./bin/echo-server serve --listen stdio:// >/dev/null 2>&1");
    assert(server_exit == 0);
    ++passed;

    capture = read_file_text(fake_log);
    assert(!capture.empty());
    ++passed;
    assert(capture.find("ARG0=run") != std::string::npos);
    ++passed;
    assert(capture.find("echo-server-go/main.go") != std::string::npos);
    ++passed;
    assert(capture.find("serve") != std::string::npos);
    ++passed;
    assert(capture.find("--sdk") != std::string::npos &&
           capture.find("cpp-holons") != std::string::npos);
    ++passed;
    assert(capture.find("--max-recv-bytes") != std::string::npos &&
           capture.find("1572864") != std::string::npos);
    ++passed;
    assert(capture.find("--max-send-bytes") != std::string::npos &&
           capture.find("1572864") != std::string::npos);
    ++passed;
    assert(capture.find("--listen") != std::string::npos &&
           capture.find("stdio://") != std::string::npos);
    ++passed;

    int holonrpc_exit = command_exit_code(
        "./bin/holon-rpc-server ws://127.0.0.1:8080/rpc >/dev/null 2>&1");
    assert(holonrpc_exit == 0);
    ++passed;

    capture = read_file_text(fake_log);
    assert(!capture.empty());
    ++passed;
    assert(capture.find("PWD=") != std::string::npos &&
           capture.find("/sdk/go-holons") != std::string::npos);
    ++passed;
    assert(capture.find("ARG0=run") != std::string::npos);
    ++passed;
    assert(capture.find("holon-rpc-server-go/main.go") != std::string::npos);
    ++passed;
    assert(capture.find("--sdk") != std::string::npos &&
           capture.find("cpp-holons") != std::string::npos);
    ++passed;
    assert(capture.find("ws://127.0.0.1:8080/rpc") != std::string::npos);
    ++passed;

    if (had_prev_go_bin) {
      ::setenv("GO_BIN", prev_go_bin_value.c_str(), 1);
    } else {
      ::unsetenv("GO_BIN");
    }
    if (had_prev_log) {
      ::setenv("HOLONS_FAKE_GO_LOG", prev_log_value.c_str(), 1);
    } else {
      ::unsetenv("HOLONS_FAKE_GO_LOG");
    }
    if (had_prev_gocache) {
      ::setenv("GOCACHE", prev_gocache_value.c_str(), 1);
    } else {
      ::unsetenv("GOCACHE");
    }

    ::unlink(fake_go);
    ::unlink(fake_log);
  }

  // --- certification runtime transports ---
  {
    int stdio_exit = command_exit_code(
        "./bin/echo-client --message cert-stdio stdio:// >/dev/null 2>&1");
    assert(stdio_exit == 0);
    ++passed;

    if (bind_restricted) {
      std::fprintf(stderr, "SKIP: echo-client ws:// (%s)\n",
                   bind_reason.c_str());
      ++passed;
    } else {
      int ws_exit = command_exit_code(
          "./bin/echo-client --server-sdk cpp-holons --message cert-ws "
          "ws://127.0.0.1:0/grpc >/dev/null 2>&1");
      assert(ws_exit == 0);
      ++passed;
    }
  }

  // --- resilience probes (L5) ---
  if (bind_restricted) {
    std::fprintf(stderr, "SKIP: graceful shutdown probe (%s)\n",
                 bind_reason.c_str());
    ++passed;
    std::fprintf(stderr, "SKIP: timeout propagation probe (%s)\n",
                 bind_reason.c_str());
    ++passed;
    std::fprintf(stderr, "SKIP: oversized message rejection probe (%s)\n",
                 bind_reason.c_str());
    ++passed;
  } else {
    auto go = resolve_go_binary();

    {
      char script[8192];
      std::snprintf(
          script, sizeof(script),
          "cleanup() {\n"
          "  if [ -n \"${S_PID:-}\" ] && kill -0 \"$S_PID\" >/dev/null 2>&1; then\n"
          "    kill -TERM \"$S_PID\" >/dev/null 2>&1 || true\n"
          "    wait \"$S_PID\" >/dev/null 2>&1 || true\n"
          "  fi\n"
          "}\n"
          "trap cleanup EXIT\n"
          "S_OUT=$(mktemp)\n"
          "S_ERR=$(mktemp)\n"
          "./bin/echo-server --sleep-ms 1200 --listen tcp://127.0.0.1:0 >\"$S_OUT\" 2>\"$S_ERR\" &\n"
          "S_PID=$!\n"
          "ADDR=\"\"\n"
          "for _ in $(seq 1 120); do\n"
          "  if [ -s \"$S_OUT\" ]; then\n"
          "    ADDR=$(head -n1 \"$S_OUT\" | tr -d '\\r\\n')\n"
          "    if [ -n \"$ADDR\" ]; then break; fi\n"
          "  fi\n"
          "  sleep 0.05\n"
          "done\n"
          "[ -n \"$ADDR\" ]\n"
          "(cd ../go-holons && '%s' run ./cmd/echo-client --server-sdk cpp-holons --timeout-ms 5000 --message cert-l5-graceful \"$ADDR\" >/dev/null 2>&1) &\n"
          "C_PID=$!\n"
          "sleep 0.2\n"
          "kill -TERM \"$S_PID\"\n"
          "wait \"$C_PID\"\n"
          "wait \"$S_PID\"\n"
          "trap - EXIT\n",
          go.c_str());
      assert(run_bash_script(script) == 0);
      ++passed;
    }

    {
      char script[12288];
      std::snprintf(
          script, sizeof(script),
          "cleanup() {\n"
          "  if [ -n \"${S_PID:-}\" ] && kill -0 \"$S_PID\" >/dev/null 2>&1; then\n"
          "    kill -TERM \"$S_PID\" >/dev/null 2>&1 || true\n"
          "    wait \"$S_PID\" >/dev/null 2>&1 || true\n"
          "  fi\n"
          "}\n"
          "trap cleanup EXIT\n"
          "S_OUT=$(mktemp)\n"
          "S_ERR=$(mktemp)\n"
          "./bin/echo-server --sleep-ms 5000 --listen tcp://127.0.0.1:0 >\"$S_OUT\" 2>\"$S_ERR\" &\n"
          "S_PID=$!\n"
          "ADDR=\"\"\n"
          "for _ in $(seq 1 120); do\n"
          "  if [ -s \"$S_OUT\" ]; then\n"
          "    ADDR=$(head -n1 \"$S_OUT\" | tr -d '\\r\\n')\n"
          "    if [ -n \"$ADDR\" ]; then break; fi\n"
          "  fi\n"
          "  sleep 0.05\n"
          "done\n"
          "[ -n \"$ADDR\" ]\n"
          "TIME_OUT=$(mktemp)\n"
          "TIME_ERR=$(mktemp)\n"
          "set +e\n"
          "(cd ../go-holons && '%s' run ./cmd/echo-client --server-sdk cpp-holons --timeout-ms 2000 --message cert-l5-timeout \"$ADDR\" >\"$TIME_OUT\" 2>\"$TIME_ERR\")\n"
          "TIME_RC=$?\n"
          "set -e\n"
          "[ \"$TIME_RC\" -ne 0 ]\n"
          "grep -Eiq 'DeadlineExceeded|deadline exceeded' \"$TIME_ERR\"\n"
          "(cd ../go-holons && '%s' run ./cmd/echo-client --server-sdk cpp-holons --timeout-ms 8000 --message cert-l5-timeout-followup \"$ADDR\" >/dev/null 2>&1)\n"
          "kill -TERM \"$S_PID\"\n"
          "wait \"$S_PID\"\n"
          "trap - EXIT\n",
          go.c_str(), go.c_str());
      assert(run_bash_script(script) == 0);
      ++passed;
    }

    {
      char script[8192];
      std::snprintf(
          script, sizeof(script),
          "cleanup() {\n"
          "  if [ -n \"${S_PID:-}\" ] && kill -0 \"$S_PID\" >/dev/null 2>&1; then\n"
          "    kill -TERM \"$S_PID\" >/dev/null 2>&1 || true\n"
          "    wait \"$S_PID\" >/dev/null 2>&1 || true\n"
          "  fi\n"
          "}\n"
          "trap cleanup EXIT\n"
          "S_OUT=$(mktemp)\n"
          "S_ERR=$(mktemp)\n"
          "./bin/echo-server --listen tcp://127.0.0.1:0 >\"$S_OUT\" 2>\"$S_ERR\" &\n"
          "S_PID=$!\n"
          "ADDR=\"\"\n"
          "for _ in $(seq 1 120); do\n"
          "  if [ -s \"$S_OUT\" ]; then\n"
          "    ADDR=$(head -n1 \"$S_OUT\" | tr -d '\\r\\n')\n"
          "    if [ -n \"$ADDR\" ]; then break; fi\n"
          "  fi\n"
          "  sleep 0.05\n"
          "done\n"
          "[ -n \"$ADDR\" ]\n"
          "(cd ../go-holons && '%s' run ../cpp-holons/test/go_large_ping.go \"$ADDR\" >/dev/null 2>&1)\n"
          "kill -TERM \"$S_PID\"\n"
          "wait \"$S_PID\"\n"
          "trap - EXIT\n",
          go.c_str());
      assert(run_bash_script(script) == 0);
      ++passed;
    }
  }

  // --- scheme ---
  assert(holons::scheme("tcp://:9090") == "tcp");
  ++passed;
  assert(holons::scheme("unix:///tmp/x.sock") == "unix");
  ++passed;
  assert(holons::scheme("stdio://") == "stdio");
  ++passed;
  assert(holons::scheme("ws://host:8080") == "ws");
  ++passed;
  assert(holons::scheme("wss://host:443") == "wss");
  ++passed;

  // --- default URI ---
  assert(holons::kDefaultURI == "tcp://:9090");
  ++passed;

  // --- parse_uri ---
  {
    auto parsed = holons::parse_uri("wss://example.com:8443");
    assert(parsed.scheme == "wss");
    ++passed;
    assert(parsed.host == "example.com");
    ++passed;
    assert(parsed.port == 8443);
    ++passed;
    assert(parsed.path == "/grpc");
    ++passed;
    assert(parsed.secure);
    ++passed;
  }

  {
    auto parsed = holons::parse_uri(
        "https://example.com:8443/api/v1/rpc?ca=%2Ftmp%2Ftest-cert.pem");
    assert(parsed.scheme == "https");
    ++passed;
    assert(parsed.host == "example.com");
    ++passed;
    assert(parsed.port == 8443);
    ++passed;
    assert(parsed.path == "/api/v1/rpc");
    ++passed;
    assert(parsed.query == "ca=%2Ftmp%2Ftest-cert.pem");
    ++passed;
    assert(parsed.secure);
    ++passed;
  }

  // --- listen tcp + runtime accept/read ---
  if (bind_restricted) {
    std::fprintf(stderr, "SKIP: listen tcp (%s)\n", bind_reason.c_str());
    ++passed;
  } else {
    auto lis = holons::listen("tcp://127.0.0.1:0");
    auto *tcp = std::get_if<holons::tcp_listener>(&lis);
    assert(tcp != nullptr);
    ++passed;

    sockaddr_in addr{};
    socklen_t len = sizeof(addr);
    int rc = ::getsockname(tcp->fd, reinterpret_cast<sockaddr *>(&addr), &len);
    assert(rc == 0);
    ++passed;

    int port = ntohs(addr.sin_port);
    assert(port > 0);
    ++passed;

    int cfd = connect_tcp("127.0.0.1", port);
    assert(cfd >= 0);
    ++passed;

    auto server_conn = holons::accept(lis);
    assert(server_conn.scheme == "tcp");
    ++passed;

    const char *msg = "ping";
    auto wrote = ::write(cfd, msg, 4);
    assert(wrote == 4);
    ++passed;

    char buf[8] = {0};
    auto read_n = holons::conn_read(server_conn, buf, sizeof(buf));
    assert(read_n == 4);
    ++passed;
    assert(std::string(buf, 4) == "ping");
    ++passed;

    holons::close_connection(server_conn);
    ::close(cfd);
    holons::close_listener(lis);
  }

  // --- listen unix + runtime accept/read ---
  {
    auto socket_path = make_temp_unix_socket_path();
    auto lis = holons::listen("unix://" + socket_path);
    auto *unix = std::get_if<holons::unix_listener>(&lis);
    assert(unix != nullptr);
    ++passed;
    assert(unix->path == socket_path);
    ++passed;

    int cfd = connect_unix(socket_path);
    assert(cfd >= 0);
    ++passed;

    auto server_conn = holons::accept(lis);
    assert(server_conn.scheme == "unix");
    ++passed;

    const char *msg = "unix";
    auto wrote = ::write(cfd, msg, 4);
    assert(wrote == 4);
    ++passed;

    char buf[8] = {0};
    auto read_n = holons::conn_read(server_conn, buf, sizeof(buf));
    assert(read_n == 4);
    ++passed;
    assert(std::string(buf, 4) == "unix");
    ++passed;

    holons::close_connection(server_conn);
    ::close(cfd);
    holons::close_listener(lis);
    std::remove(socket_path.c_str());
  }

  // --- listen stdio/ws ---
  {
    auto stdio_lis = holons::listen("stdio://");
    assert(std::holds_alternative<holons::stdio_listener>(stdio_lis));
    ++passed;

    auto stdio_conn = holons::accept(stdio_lis);
    assert(stdio_conn.scheme == "stdio");
    ++passed;
    holons::close_connection(stdio_conn);

    try {
      (void)holons::accept(stdio_lis);
      assert(false && "stdio second accept should throw");
    } catch (const std::runtime_error &) {
      ++passed;
    }

    auto ws_lis = holons::listen("ws://127.0.0.1:8080/holon");
    auto *ws = std::get_if<holons::ws_listener>(&ws_lis);
    assert(ws != nullptr);
    ++passed;
    assert(ws->host == "127.0.0.1");
    ++passed;
    assert(ws->port == 8080);
    ++passed;
    assert(ws->path == "/holon");
    ++passed;
    assert(!ws->secure);
    ++passed;

    try {
      (void)holons::accept(ws_lis);
      assert(false && "ws accept should throw");
    } catch (const std::runtime_error &) {
      ++passed;
    }
  }

  // --- unsupported URI ---
  try {
    (void)holons::listen("ftp://host");
    assert(false && "should have thrown");
  } catch (const std::invalid_argument &) {
    ++passed;
  }

  // --- parse_flags ---
  assert(holons::parse_flags({"--listen", "tcp://:8080"}) == "tcp://:8080");
  ++passed;
  assert(holons::parse_flags({"--port", "3000"}) == "tcp://:3000");
  ++passed;
  assert(holons::parse_flags({}) == "tcp://:9090");
  ++passed;
  auto serve_listeners = holons::serve::parse_flags(
      {"--listen", "tcp://:8080", "--listen", "unix:///tmp/holons.sock"});
  assert(serve_listeners.size() == 2);
  ++passed;
  assert(serve_listeners[0] == "tcp://:8080");
  ++passed;
  assert(serve_listeners[1] == "unix:///tmp/holons.sock");
  ++passed;
  auto serve_options = holons::serve::parse_options({"--reflect"});
  assert(serve_options.reflect);
  ++passed;

#if HOLONS_HAS_GRPCPP && HOLONS_HAS_HOLONMETA_GRPC
  // --- serve requires registered static describe ---
  {
    holons::describe::clear_static_response();

#ifdef _WIN32
    if (bind_restricted) {
      std::fprintf(stderr, "SKIP: serve missing describe (%s)\n",
                   bind_reason.c_str());
      ++passed;
    } else {
      try {
        (void)holons::serve::start("tcp://127.0.0.1:0",
                                   [](grpc::ServerBuilder &) {});
        assert(false && "serve should require registered static describe");
      } catch (const std::runtime_error &error) {
        assert(std::string(error.what()) ==
               "no Incode Description registered — run op build");
        ++passed;
      }
    }
#else
    auto socket_path = make_temp_unix_socket_path();
    try {
      (void)holons::serve::start("unix://" + socket_path,
                                 [](grpc::ServerBuilder &) {});
      assert(false && "serve should require registered static describe");
    } catch (const std::runtime_error &error) {
      assert(std::string(error.what()) ==
             "no Incode Description registered — run op build");
      ++passed;
    }
    std::remove(socket_path.c_str());
#endif
  }
#endif

#if HOLONS_HAS_GRPCPP && HOLONS_HAS_HOLONMETA_GRPC
  // --- describe is static-only at runtime, even without adjacent protos ---
  {
    auto static_response = make_static_describe_response();
    holons::describe::use_static_response(static_response);

#ifdef _WIN32
    if (bind_restricted) {
      std::fprintf(stderr, "SKIP: static-only describe without protos (%s)\n",
                   bind_reason.c_str());
      ++passed;
    } else {
      auto empty_root = make_temp_dir("holons_cpp_static_only_");
      cwd_guard cwd(empty_root);

      holons::serve::options opts;
      opts.announce = false;
      auto handle = holons::serve::start(
          "tcp://127.0.0.1:0", [](grpc::ServerBuilder &) {}, opts);
      auto advertised = handle.listeners().front().advertised;
      auto channel = holons::connect(advertised);
      auto stub = holons::v1::HolonMeta::NewStub(channel);
      grpc::ClientContext context;
      holons::v1::DescribeResponse response;
      auto status =
          stub->Describe(&context, holons::v1::DescribeRequest{}, &response);
      assert(status.ok());
      ++passed;
      assert(response.manifest().identity().given_name() == "Static");
      ++passed;
      assert(response.services_size() == 1);
      ++passed;
      handle.stop();
    }
#else
    auto empty_root = make_temp_dir("holons_cpp_static_only_");
    cwd_guard cwd(empty_root);
    auto socket_path = make_temp_unix_socket_path();

    holons::serve::options opts;
    opts.announce = false;
    auto handle = holons::serve::start(
        "unix://" + socket_path, [](grpc::ServerBuilder &) {}, opts);
    auto channel = holons::connect("unix://" + socket_path);
    auto stub = holons::v1::HolonMeta::NewStub(channel);
    grpc::ClientContext context;
    holons::v1::DescribeResponse response;
    auto status =
        stub->Describe(&context, holons::v1::DescribeRequest{}, &response);
    assert(status.ok());
    ++passed;
    assert(response.manifest().identity().given_name() == "Static");
    ++passed;
    assert(response.services_size() == 1);
    ++passed;
    handle.stop();
    std::remove(socket_path.c_str());
#endif

    holons::describe::clear_static_response();
  }
#endif

  // --- proto_field_value ---
  assert(holons::proto_field_value("uuid: \"abc-123\"", "uuid") == "abc-123");
  ++passed;
  assert(holons::proto_field_value("lang: rust", "lang") == "rust");
  ++passed;

  // --- parse_holon ---
  {
    auto root = make_temp_dir("holons_cpp_identity_");
    auto path = (root / "holon.proto").string();
    {
      std::ofstream f(path);
      f << "syntax = \"proto3\";\n"
        << "package test.v1;\n\n"
        << "option (holons.v1.manifest) = {\n"
        << "  identity: {\n"
        << "    uuid: \"abc-123\"\n"
        << "    given_name: \"test\"\n"
        << "    family_name: \"Test\"\n"
        << "  }\n"
        << "  lang: \"cpp\"\n"
        << "};\n";
    }
    auto id = holons::identity::read(path);
    assert(id.uuid == "abc-123");
    ++passed;
    assert(id.given_name == "test");
    ++passed;
    assert(id.lang == "cpp");
    ++passed;
    auto resolved = holons::identity::resolve(path);
    assert(resolved.manifest.lang == "cpp");
    ++passed;
    auto manifest_path = holons::identity::find_manifest(path);
    assert(manifest_path.has_value());
    ++passed;
    std::filesystem::remove_all(root);
  }

  // --- describe ---
  {
    auto root = make_temp_dir("holons_cpp_describe_");
    write_echo_holon(root);

    auto response = holons::describe::build_response(root / "protos");
    assert(response.manifest.identity.given_name == "Echo");
    ++passed;
    assert(response.manifest.identity.family_name == "Server");
    ++passed;
    assert(response.manifest.identity.motto == "Reply precisely.");
    ++passed;
    assert(response.services.size() == 1);
    ++passed;

    const auto &service = response.services[0];
    assert(service.name == "echo.v1.Echo");
    ++passed;
    assert(service.description ==
           "Echo echoes request payloads for documentation tests.");
    ++passed;
    assert(service.methods.size() == 1);
    ++passed;

    const auto &method = service.methods[0];
    assert(method.name == "Ping");
    ++passed;
    assert(method.description == "Ping echoes the inbound message.");
    ++passed;
    assert(method.input_type == "echo.v1.PingRequest");
    ++passed;
    assert(method.output_type == "echo.v1.PingResponse");
    ++passed;
    assert(method.example_input == "{\"message\":\"hello\",\"sdk\":\"go-holons\"}");
    ++passed;
    assert(method.input_fields.size() == 2);
    ++passed;
    assert(method.input_fields[0].name == "message");
    ++passed;
    assert(method.input_fields[0].type == "string");
    ++passed;
    assert(method.input_fields[0].number == 1);
    ++passed;
    assert(method.input_fields[0].description == "Message to echo back.");
    ++passed;
    assert(method.input_fields[0].label ==
           holons::describe::field_label::required);
    ++passed;
    assert(method.input_fields[0].required);
    ++passed;
    assert(method.input_fields[0].example == "\"hello\"");
    ++passed;

#if HOLONS_HAS_HOLONMETA_GRPC
    auto static_response = make_static_describe_response();
    holons::describe::use_static_response(static_response);
    auto registration = holons::describe::make_registration();
    assert(registration.service_name == "holons.v1.HolonMeta");
    ++passed;
    assert(registration.method_name == "Describe");
    ++passed;
    auto registered = registration.handler(holons::v1::DescribeRequest{});
    assert(registered.manifest().identity().given_name() == "Static");
    ++passed;
    assert(registered.services_size() == 1);
    ++passed;
    holons::describe::clear_static_response();
#endif

    std::filesystem::remove_all(root);
  }

  // --- describe without protos ---
  {
    auto root = make_temp_dir("holons_cpp_describe_empty_");
    {
      std::ofstream holon(root / "holon.proto");
      holon << "syntax = \"proto3\";\n"
            << "package holons.test.v1;\n\n"
            << "option (holons.v1.manifest) = {\n"
            << "  identity: {\n"
            << "    given_name: \"Empty\"\n"
            << "    family_name: \"Holon\"\n"
            << "    motto: \"Still available.\"\n"
            << "  }\n"
            << "};\n";
    }

    auto response = holons::describe::build_response(root / "protos");
    assert(response.manifest.identity.given_name == "Empty");
    ++passed;
    assert(response.manifest.identity.family_name == "Holon");
    ++passed;
    assert(response.manifest.identity.motto == "Still available.");
    ++passed;
    assert(response.services.empty());
    ++passed;

    std::filesystem::remove_all(root);
  }

  // --- parse_holon invalid mapping ---
  {
    std::string path = make_temp_proto_path();
    {
      std::ofstream f(path);
      f << "syntax = \"proto3\";\npackage test.v1;\n";
    }
    try {
      holons::parse_holon(path);
      assert(false && "should have thrown");
    } catch (const std::runtime_error &e) {
      assert(std::string(e.what()).find("manifest option") != std::string::npos);
      ++passed;
    }
    std::remove(path.c_str());
  }

  // --- discover ---
  {
    auto root = make_temp_dir("holons_cpp_discover_");
    auto op_root = make_temp_dir("holons_cpp_op_");
    auto previous_cwd = std::filesystem::current_path();
    auto previous_oppath = capture_env("OPPATH");
    auto previous_opbin = capture_env("OPBIN");

    write_discovery_holon(root / "holons" / "alpha", "uuid-alpha", "Alpha", "Go",
                          "alpha-go");
    write_discovery_holon(root / "nested" / "beta", "uuid-beta", "Beta", "Rust",
                          "beta-rust");
    write_discovery_holon(root / "nested" / "dup" / "alpha", "uuid-alpha", "Alpha",
                          "Go", "alpha-go");
    write_discovery_holon(root / ".git" / "hidden", "uuid-hidden", "Ignored",
                          "Holon", "ignored");
    write_discovery_holon(root / "node_modules" / "x", "uuid-node", "Ignored",
                          "Node", "ignored");
    write_discovery_holon(op_root / "bin" / "gamma", "uuid-gamma", "Gamma", "Bin",
                          "gamma-bin");
    write_discovery_holon(op_root / "cache" / "delta", "uuid-delta", "Delta",
                          "Cache", "delta-cache");

    auto discovered = holons::discover(root);
    assert(discovered.size() == 2);
    ++passed;
    assert(discovered[0].slug == "alpha-go");
    ++passed;
    assert(discovered[0].relative_path.generic_string() == "holons/alpha");
    ++passed;
    assert(discovered[0].manifest.has_value());
    ++passed;
    assert(discovered[0].manifest->build.runner == "go-module");
    ++passed;
    assert(discovered[1].slug == "beta-rust");
    ++passed;
    auto nearby = holons::find_nearby_by_slug(root, "alpha-go");
    assert(nearby.has_value());
    ++passed;
    assert(nearby->relative_path.generic_string() == "holons/alpha");
    ++passed;

    std::filesystem::current_path(root);
    restore_env("OPPATH", std::optional<std::string>(op_root.string()));
    restore_env("OPBIN", std::nullopt);

    auto discovered_all = holons::discover_all();
    assert(discovered_all.size() == 4);
    ++passed;

    auto by_slug = holons::find_by_slug("alpha-go");
    assert(by_slug.has_value());
    ++passed;
    assert(by_slug->uuid == "uuid-alpha");
    ++passed;

    auto by_uuid = holons::find_by_uuid("uuid-d");
    assert(by_uuid.has_value());
    ++passed;
    assert(by_uuid->origin == "cache");
    ++passed;

    std::filesystem::current_path(previous_cwd);
    restore_env("OPPATH", previous_oppath);
    restore_env("OPBIN", previous_opbin);
    std::filesystem::remove_all(root);
    std::filesystem::remove_all(op_root);
  }

  // --- connect ---
#if HOLONS_HAS_GRPCPP
  if (bind_restricted) {
    std::fprintf(stderr, "SKIP: connect direct dial (%s)\n",
                 bind_reason.c_str());
    ++passed;
    std::fprintf(stderr, "SKIP: connect ephemeral slug startup (%s)\n",
                 bind_reason.c_str());
    ++passed;
    std::fprintf(stderr, "SKIP: connect persistent port-file startup (%s)\n",
                 bind_reason.c_str());
    ++passed;
    std::fprintf(stderr, "SKIP: connect port-file reuse (%s)\n",
                 bind_reason.c_str());
    ++passed;
    std::fprintf(stderr, "SKIP: connect stale port-file recovery (%s)\n",
                 bind_reason.c_str());
    ++passed;
  } else {
    auto sdk_root = std::filesystem::path(find_sdk_dir()) / "cpp-holons";
    auto echo_server = (sdk_root / "bin" / "echo-server").string();

    {
      auto server = start_child_process(
          echo_server, {"--listen", "tcp://127.0.0.1:0"});
      auto uri = wait_for_child_uri(server, 20000);
      auto parsed = holons::parse_uri(uri);
      auto direct_target = parsed.host + ":" + std::to_string(parsed.port);

      auto channel = holons::connect(direct_target);
      assert(channel_ready(channel, 2000));
      ++passed;
      holons::disconnect(channel);
      assert(pid_exists(server.pid));
      ++passed;
    }

    {
      auto socket_path = make_temp_unix_socket_path();
      auto server = start_child_process(
          echo_server, {"--listen", "unix://" + socket_path});
      auto uri = wait_for_child_uri(server, 20000);
      assert(uri == "unix://" + socket_path);
      ++passed;

      auto channel = holons::connect(uri);
      assert(channel_ready(channel, 2000));
      ++passed;
      holons::disconnect(channel);
      assert(pid_exists(server.pid));
      ++passed;
      std::remove(socket_path.c_str());
    }

    {
      auto fixture = write_connect_fixture("Connect", "Ephemeral");
      cwd_guard cwd(fixture.root);
      env_guard oppath("OPPATH", (fixture.root / ".op-home").string());
      env_guard opbin("OPBIN", (fixture.root / ".op-bin").string());

      auto channel = holons::connect(fixture.slug);
      assert(channel_ready(channel, 2000));
      ++passed;

      auto pid = read_pid_file(fixture.pid_file);
      assert(pid > 0);
      ++passed;

      assert(holons::trim_copy(read_file_text(fixture.args_file.string())) ==
             "serve\n--listen\nstdio://");
      ++passed;
      assert(holons::trim_copy(read_file_text(fixture.fd_mode_file.string())) ==
             "same");
      ++passed;

      holons::disconnect(channel);
      wait_for_process_exit(pid);
      ++passed;

      auto port_file =
          fixture.root / ".op" / "run" / (fixture.slug + ".port");
      assert(!std::filesystem::exists(port_file));
      ++passed;
    }

    {
      auto fixture = write_connect_fixture("Connect", "Persistent");
      cwd_guard cwd(fixture.root);
      env_guard oppath("OPPATH", (fixture.root / ".op-home").string());
      env_guard opbin("OPBIN", (fixture.root / ".op-bin").string());

      holons::ConnectOptions opts;
      opts.timeout_ms = 5000;
      opts.transport = "tcp";
      auto channel = holons::connect(fixture.slug, opts);
      assert(channel_ready(channel, 2000));
      ++passed;

      auto pid = read_pid_file(fixture.pid_file);
      assert(pid > 0);
      ++passed;

      auto port_file =
          fixture.root / ".op" / "run" / (fixture.slug + ".port");
      assert(std::filesystem::exists(port_file));
      ++passed;

      auto advertised = holons::trim_copy(read_file_text(port_file.string()));
      assert(advertised.rfind("tcp://127.0.0.1:", 0) == 0);
      ++passed;

      holons::disconnect(channel);
      assert(pid_exists(pid));
      ++passed;

      auto reused = holons::connect(fixture.slug);
      assert(channel_ready(reused, 2000));
      ++passed;

      holons::disconnect(reused);
      assert(pid_exists(pid));
      ++passed;

      assert(::kill(pid, SIGTERM) == 0);
      wait_for_process_exit(pid);
      ++passed;
    }

    {
      auto fixture = write_connect_fixture("Connect", "Reuse");
      cwd_guard cwd(fixture.root);
      env_guard oppath("OPPATH", (fixture.root / ".op-home").string());
      env_guard opbin("OPBIN", (fixture.root / ".op-bin").string());

      auto server = start_child_process(
          echo_server, {"--listen", "tcp://127.0.0.1:0"});
      auto uri = wait_for_child_uri(server, 20000);
      auto port_file =
          fixture.root / ".op" / "run" / (fixture.slug + ".port");
      write_port_file(port_file, uri);

      auto channel = holons::connect(fixture.slug);
      assert(channel_ready(channel, 2000));
      ++passed;

      holons::disconnect(channel);
      assert(pid_exists(server.pid));
      ++passed;
    }

    {
      auto fixture = write_connect_fixture("Connect", "Stale");
      cwd_guard cwd(fixture.root);
      env_guard oppath("OPPATH", (fixture.root / ".op-home").string());
      env_guard opbin("OPBIN", (fixture.root / ".op-bin").string());

      auto port_file =
          fixture.root / ".op" / "run" / (fixture.slug + ".port");
      write_port_file(
          port_file,
          "tcp://127.0.0.1:" + std::to_string(reserve_loopback_port()));

      auto channel = holons::connect(fixture.slug);
      assert(channel_ready(channel, 2000));
      ++passed;

      auto pid = read_pid_file(fixture.pid_file);
      assert(pid > 0);
      ++passed;

      assert(!std::filesystem::exists(port_file));
      ++passed;

      holons::disconnect(channel);
      wait_for_process_exit(pid);
      ++passed;
    }
  }
#else
  std::fprintf(stderr, "SKIP: connect tests (grpc++ headers unavailable)\n");
  ++passed;
#endif

  // --- holon-rpc server interop (cpp wrapper) ---
  if (bind_restricted) {
    std::fprintf(stderr, "SKIP: holon-rpc server wrapper (%s)\n",
                 bind_reason.c_str());
    ++passed;
  } else {
    with_cpp_holonrpc_server([&](const std::string &url) {
      holons::holon_rpc_client client(250, 250, 100, 400);
      client.connect(url);
      auto out =
          client.invoke("echo.v1.Echo/Ping", json{{"message", "from-cpp"}});
      assert(out["message"].get<std::string>() == "from-cpp");
      ++passed;
      assert(out["sdk"].get<std::string>() == "cpp-holons");
      ++passed;
      client.close();
    });
    ++passed;
  }

  // --- holon-rpc client interop (Go helper) ---
  if (bind_restricted) {
    std::fprintf(stderr, "SKIP: holon-rpc Go helper (%s)\n",
                 bind_reason.c_str());
    ++passed;
  } else {
    {
      with_go_helper("echo", [&](const std::string &url) {
        holons::holon_rpc_client client(250, 250, 100, 400);
        client.connect(url);
        auto out =
            client.invoke("echo.v1.Echo/Ping", json{{"message", "hello"}});
        assert(out["message"].get<std::string>() == "hello");
        ++passed;
        client.close();
      });
    }

    {
      with_go_helper("echo", [&](const std::string &url) {
        holons::holon_rpc_client client(250, 250, 100, 400);
        client.register_handler("client.v1.Client/Hello",
                                [](const json &params) -> json {
                                  std::string name =
                                      params.value("name", std::string(""));
                                  return json{{"message", "hello " + name}};
                                });

        client.connect(url);
        auto out = client.invoke("echo.v1.Echo/CallClient", json::object());
        assert(out["message"].get<std::string>() == "hello go");
        ++passed;
        client.close();
      });
    }

    {
      with_go_helper("drop-once", [&](const std::string &url) {
        holons::holon_rpc_client client(200, 200, 100, 400);
        client.connect(url);

        auto first =
            client.invoke("echo.v1.Echo/Ping", json{{"message", "first"}});
        assert(first["message"].get<std::string>() == "first");
        ++passed;

        std::this_thread::sleep_for(std::chrono::milliseconds(700));

        auto second =
            invoke_eventually(client, "echo.v1.Echo/Ping",
                              json{{"message", "second"}});
        assert(second["message"].get<std::string>() == "second");
        ++passed;

        auto hb = invoke_eventually(client, "echo.v1.Echo/HeartbeatCount",
                                    json::object());
        assert(hb["count"].get<int>() >= 1);
        ++passed;

        client.close();
      });
    }

    {
      with_go_transport_helper("wss", [&](const std::string &url) {
        holons::holon_rpc_client client(250, 250, 100, 400);
        client.connect(url);
        auto out =
            client.invoke("echo.v1.Echo/Ping", json{{"message", "secure"}});
        assert(out["message"].get<std::string>() == "secure");
        ++passed;
        assert(out["transport"].get<std::string>() == "wss");
        ++passed;
        client.close();
      });
    }

    {
      with_go_transport_helper("http", [&](const std::string &url) {
        holons::holon_rpc_http_client client(url);

        auto unary =
            client.invoke("echo.v1.Echo/Ping", json{{"message", "http"}});
        assert(unary["message"].get<std::string>() == "http");
        ++passed;
        assert(unary["transport"].get<std::string>() == "http");
        ++passed;

        auto stream =
            client.stream("echo.v1.Echo/Watch", json{{"message", "post-stream"}});
        assert(stream.size() == 3);
        ++passed;
        assert(stream[0].event == "message");
        ++passed;
        assert(stream[0].id == "1");
        ++passed;
        assert(stream[0].result["message"].get<std::string>() == "post-stream");
        ++passed;
        assert(stream[1].result["step"].get<std::string>() == "2");
        ++passed;
        assert(stream[2].event == "done");
        ++passed;

        auto query = client.stream_query("echo.v1.Echo/Watch",
                                         {{"message", "query-stream"}});
        assert(query.size() == 3);
        ++passed;
        assert(query[0].result["message"].get<std::string>() == "query-stream");
        ++passed;
        assert(query[0].result["transport"].get<std::string>() == "http");
        ++passed;
        assert(query[2].event == "done");
        ++passed;
      });
    }

    {
      with_go_transport_helper("https", [&](const std::string &url) {
        holons::holon_rpc_http_client client(url);
        auto unary =
            client.invoke("echo.v1.Echo/Ping", json{{"message", "https"}});
        assert(unary["message"].get<std::string>() == "https");
        ++passed;
        assert(unary["transport"].get<std::string>() == "https");
        ++passed;
      });
    }
  }

  std::printf("%d passed, 0 failed\n", passed);
  return 0;
}

#endif
