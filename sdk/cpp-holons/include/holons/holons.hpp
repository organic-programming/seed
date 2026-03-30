#pragma once

#include <algorithm>
#include <array>
#include <atomic>
#include <cerrno>
#include <cmath>
#include <cctype>
#include <chrono>
#include <cstdint>
#include <condition_variable>
#include <cstdlib>
#include <cstdio>
#include <cstring>
#include <filesystem>
#include <fstream>
#include <functional>
#include <limits>
#include <map>
#include <memory>
#include <mutex>
#include <regex>
#ifdef _WIN32
#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#include <winsock2.h>
#include <ws2tcpip.h>
#include <windows.h>
#include <io.h>
#include <fcntl.h>
#ifdef _MSC_VER
#pragma comment(lib, "ws2_32.lib")
#endif

#if defined(_MSC_VER) && !defined(_SSIZE_T_DEFINED)
using ssize_t = intptr_t;
#define _SSIZE_T_DEFINED
#endif

#ifndef STDIN_FILENO
#define STDIN_FILENO 0
#endif
#ifndef STDOUT_FILENO
#define STDOUT_FILENO 1
#endif
#else
#include <arpa/inet.h>
#include <netinet/in.h>
#include <poll.h>
#include <signal.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <sys/wait.h>
#include <unistd.h>
#endif
#if __has_include(<nlohmann/json.hpp>)
#include <nlohmann/json.hpp>
#elif __has_include("/opt/homebrew/include/nlohmann/json.hpp")
#include "/opt/homebrew/include/nlohmann/json.hpp"
#elif __has_include("/usr/local/include/nlohmann/json.hpp")
#include "/usr/local/include/nlohmann/json.hpp"
#else
#error "nlohmann/json.hpp is required for holon_rpc_client"
#endif
#if __has_include(<grpcpp/grpcpp.h>)
#include <grpcpp/grpcpp.h>
#define HOLONS_HAS_GRPCPP 1
#else
#define HOLONS_HAS_GRPCPP 0
namespace grpc {
class ChannelCredentials {
public:
  virtual ~ChannelCredentials() = default;
};

class Channel {
public:
  virtual ~Channel() = default;

  template <typename Deadline>
  bool WaitForConnected(const Deadline &) {
    return true;
  }
};

inline std::shared_ptr<ChannelCredentials> InsecureChannelCredentials() {
  return std::make_shared<ChannelCredentials>();
}

inline std::shared_ptr<Channel>
CreateChannel(const std::string &, std::shared_ptr<ChannelCredentials>) {
  return std::make_shared<Channel>();
}

class ServerCredentials {
public:
  virtual ~ServerCredentials() = default;
};

class ServerContext {
public:
  virtual ~ServerContext() = default;
};

class Service {
public:
  virtual ~Service() = default;
};

class Status {
public:
  Status() = default;
};

class Server {
public:
  virtual ~Server() = default;

  template <typename Deadline>
  void Shutdown(const Deadline &) {}

  void Wait() {}
};

class ServerBuilder {
public:
  void AddListeningPort(const std::string &, std::shared_ptr<ServerCredentials>,
                        int *selected_port = nullptr) {
    if (selected_port != nullptr) {
      *selected_port = 0;
    }
  }

  void RegisterService(Service *) {}

  std::unique_ptr<Server> BuildAndStart() {
    return std::make_unique<Server>();
  }
};

inline std::shared_ptr<ServerCredentials> InsecureServerCredentials() {
  return std::make_shared<ServerCredentials>();
}
} // namespace grpc
#endif
#if __has_include(<openssl/err.h>) && __has_include(<openssl/ssl.h>)
#include <openssl/err.h>
#include <openssl/ssl.h>
#define HOLONS_HAS_OPENSSL 1
#else
#define HOLONS_HAS_OPENSSL 0
#endif
#if HOLONS_HAS_GRPCPP && !defined(_WIN32) &&                                   \
    __has_include(<grpcpp/create_channel_posix.h>) &&                          \
    __has_include(<grpcpp/server_posix.h>)
#include <grpcpp/create_channel_posix.h>
#include <grpcpp/server_posix.h>
#define HOLONS_HAS_GRPC_FD 1
#else
#define HOLONS_HAS_GRPC_FD 0
#endif
#include <random>
#include <sstream>
#include <thread>
#include <stdexcept>
#include <string>
#include <string_view>
#include <tuple>
#include <optional>
#include <unordered_map>
#include <variant>
#include <vector>

namespace holons {

#ifdef _WIN32
namespace detail {
struct winsock_init {
  winsock_init() {
    WSADATA wsa;
    if (WSAStartup(MAKEWORD(2, 2), &wsa) != 0) {
      throw std::runtime_error("WSAStartup failed");
    }
  }

  ~winsock_init() { WSACleanup(); }
};

inline void ensure_winsock() {
  static winsock_init instance;
  (void)instance;
}
} // namespace detail
#endif

inline void ensure_sigpipe_ignored() {
#ifndef _WIN32
  static std::once_flag once;
  std::call_once(once, []() { ::signal(SIGPIPE, SIG_IGN); });
#endif
}

inline int close_fd(int fd, bool is_socket) {
#ifdef _WIN32
  return is_socket ? static_cast<int>(::closesocket(static_cast<SOCKET>(fd)))
                   : ::_close(fd);
#else
  (void)is_socket;
  return ::close(fd);
#endif
}

inline int unlink_path(const char *path) {
#ifdef _WIN32
  return ::_unlink(path);
#else
  return ::unlink(path);
#endif
}

inline int socket_shutdown_both() {
#ifdef _WIN32
  return SD_BOTH;
#else
  return SHUT_RDWR;
#endif
}

inline std::string last_socket_error() {
#ifdef _WIN32
  return "winsock error " + std::to_string(WSAGetLastError());
#else
  return std::strerror(errno);
#endif
}

#ifdef _WIN32
inline int win_socketpair(int fds[2]) {
  fds[0] = -1;
  fds[1] = -1;

  SOCKET listener = ::socket(AF_INET, SOCK_STREAM, 0);
  if (listener == INVALID_SOCKET) {
    return -1;
  }

  int one = 1;
  ::setsockopt(listener, SOL_SOCKET, SO_REUSEADDR,
               reinterpret_cast<const char *>(&one), sizeof(one));

  sockaddr_in addr{};
  addr.sin_family = AF_INET;
  addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
  addr.sin_port = 0;
  if (::bind(listener, reinterpret_cast<sockaddr *>(&addr), sizeof(addr)) !=
      0) {
    ::closesocket(listener);
    return -1;
  }

  int addrlen = sizeof(addr);
  if (::getsockname(listener, reinterpret_cast<sockaddr *>(&addr), &addrlen) !=
      0) {
    ::closesocket(listener);
    return -1;
  }

  if (::listen(listener, 1) != 0) {
    ::closesocket(listener);
    return -1;
  }

  SOCKET client = ::socket(AF_INET, SOCK_STREAM, 0);
  if (client == INVALID_SOCKET) {
    ::closesocket(listener);
    return -1;
  }

  if (::connect(client, reinterpret_cast<sockaddr *>(&addr), sizeof(addr)) !=
      0) {
    ::closesocket(client);
    ::closesocket(listener);
    return -1;
  }

  SOCKET server = ::accept(listener, nullptr, nullptr);
  ::closesocket(listener);
  if (server == INVALID_SOCKET) {
    ::closesocket(client);
    return -1;
  }

  fds[0] = static_cast<int>(server);
  fds[1] = static_cast<int>(client);
  return 0;
}
#endif

/// Default transport URI when --listen is omitted.
constexpr std::string_view kDefaultURI = "tcp://:9090";

/// Extract the scheme from a transport URI.
inline std::string scheme(std::string_view uri) {
  auto pos = uri.find("://");
  return pos != std::string_view::npos ? std::string(uri.substr(0, pos))
                                       : std::string(uri);
}

/// Parse --listen or --port from command-line args.
inline std::string parse_flags(const std::vector<std::string> &args) {
  for (size_t i = 0; i < args.size(); ++i) {
    if (args[i] == "--listen" && i + 1 < args.size())
      return args[i + 1];
    if (args[i] == "--port" && i + 1 < args.size())
      return "tcp://:" + args[i + 1];
  }
  return std::string(kDefaultURI);
}

/// Parsed transport URI.
struct parsed_uri {
  std::string raw;
  std::string scheme;
  std::string host;
  int port = 0;
  std::string path;
  std::string query;
  bool secure = false;
};

struct tcp_listener {
  int fd = -1;
  std::string host;
  int port = 0;
};

struct unix_listener {
  int fd = -1;
  std::string path;
};

struct stdio_listener {
  std::string address = "stdio://";
  bool consumed = false;
};

struct ws_listener {
  std::string host;
  int port = 0;
  std::string path;
  bool secure = false;
};

using listener =
    std::variant<tcp_listener, unix_listener, stdio_listener, ws_listener>;

struct connection {
  int read_fd = -1;
  int write_fd = -1;
  std::string scheme;
  bool owns_read_fd = true;
  bool owns_write_fd = true;
};

inline std::tuple<std::string, int> split_host_port(const std::string &addr,
                                                     int default_port) {
  if (addr.empty())
    return {"0.0.0.0", default_port};

  auto pos = addr.rfind(':');
  if (pos == std::string::npos)
    return {addr, default_port};

  std::string host = addr.substr(0, pos);
  if (host.empty())
    host = "0.0.0.0";
  std::string port_text = addr.substr(pos + 1);
  int port = port_text.empty() ? default_port : std::stoi(port_text);
  return {host, port};
}

inline parsed_uri parse_uri(const std::string &uri) {
  std::string s = scheme(uri);

  if (s == "tcp") {
    if (uri.rfind("tcp://", 0) != 0)
      throw std::invalid_argument("invalid tcp URI: " + uri);
    auto [host, port] = split_host_port(uri.substr(6), 9090);
    return {uri, "tcp", host, port, "", "", false};
  }

  if (s == "unix") {
    if (uri.rfind("unix://", 0) != 0)
      throw std::invalid_argument("invalid unix URI: " + uri);
    auto path = uri.substr(7);
    if (path.empty())
      throw std::invalid_argument("invalid unix URI: " + uri);
    return {uri, "unix", "", 0, path, "", false};
  }

  if (s == "stdio") {
    return {"stdio://", "stdio", "", 0, "", "", false};
  }

  if (s == "ws" || s == "wss") {
    bool secure = s == "wss";
    std::string prefix = secure ? "wss://" : "ws://";
    if (uri.rfind(prefix, 0) != 0)
      throw std::invalid_argument("invalid ws URI: " + uri);

    std::string trimmed = uri.substr(prefix.size());
    auto slash = trimmed.find('/');
    std::string addr = slash == std::string::npos ? trimmed : trimmed.substr(0, slash);
    std::string path = slash == std::string::npos ? "/grpc" : trimmed.substr(slash);
    if (path.empty())
      path = "/grpc";
    std::string query;
    auto query_pos = path.find('?');
    if (query_pos != std::string::npos) {
      query = path.substr(query_pos + 1);
      path = path.substr(0, query_pos);
      if (path.empty()) {
        path = "/grpc";
      }
    }

    auto [host, port] = split_host_port(addr, secure ? 443 : 80);
    return {uri, s, host, port, path, query, secure};
  }

  if (s == "http" || s == "https") {
    bool secure = s == "https";
    std::string prefix = secure ? "https://" : "http://";
    if (uri.rfind(prefix, 0) != 0)
      throw std::invalid_argument("invalid http URI: " + uri);

    std::string trimmed = uri.substr(prefix.size());
    auto slash = trimmed.find('/');
    std::string addr = slash == std::string::npos ? trimmed : trimmed.substr(0, slash);
    std::string path = slash == std::string::npos ? "/api/v1/rpc"
                                                  : trimmed.substr(slash);
    if (path.empty())
      path = "/api/v1/rpc";
    std::string query;
    auto query_pos = path.find('?');
    if (query_pos != std::string::npos) {
      query = path.substr(query_pos + 1);
      path = path.substr(0, query_pos);
      if (path.empty()) {
        path = "/api/v1/rpc";
      }
    }

    auto [host, port] = split_host_port(addr, secure ? 443 : 80);
    return {uri, s, host, port, path, query, secure};
  }

  throw std::invalid_argument("unsupported transport URI: " + uri);
}

inline int uri_hex_digit(char ch) {
  if (ch >= '0' && ch <= '9')
    return ch - '0';
  if (ch >= 'a' && ch <= 'f')
    return 10 + (ch - 'a');
  if (ch >= 'A' && ch <= 'F')
    return 10 + (ch - 'A');
  return -1;
}

inline std::string uri_decode(std::string_view value) {
  std::string out;
  out.reserve(value.size());
  for (size_t i = 0; i < value.size(); ++i) {
    if (value[i] == '+' ) {
      out.push_back(' ');
      continue;
    }
    if (value[i] == '%' && i + 2 < value.size()) {
      int hi = uri_hex_digit(value[i + 1]);
      int lo = uri_hex_digit(value[i + 2]);
      if (hi >= 0 && lo >= 0) {
        out.push_back(static_cast<char>((hi << 4) | lo));
        i += 2;
        continue;
      }
    }
    out.push_back(value[i]);
  }
  return out;
}

inline std::string uri_encode(std::string_view value) {
  static const char *hex = "0123456789ABCDEF";
  std::string out;
  out.reserve(value.size() * 3);
  for (unsigned char ch : value) {
    if ((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
        (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' || ch == '.' ||
        ch == '~') {
      out.push_back(static_cast<char>(ch));
      continue;
    }
    out.push_back('%');
    out.push_back(hex[(ch >> 4) & 0x0F]);
    out.push_back(hex[ch & 0x0F]);
  }
  return out;
}

inline std::unordered_map<std::string, std::string>
parse_query_params(std::string_view query) {
  std::unordered_map<std::string, std::string> params;
  size_t start = 0;
  while (start <= query.size()) {
    auto amp = query.find('&', start);
    auto end = amp == std::string_view::npos ? query.size() : amp;
    auto piece = query.substr(start, end - start);
    if (!piece.empty()) {
      auto eq = piece.find('=');
      auto key = uri_decode(piece.substr(0, eq));
      auto value = eq == std::string_view::npos
                       ? std::string()
                       : uri_decode(piece.substr(eq + 1));
      params[key] = value;
    }
    if (amp == std::string_view::npos) {
      break;
    }
    start = amp + 1;
  }
  return params;
}

inline listener listen(const std::string &uri) {
#ifdef _WIN32
  detail::ensure_winsock();
#endif
  auto parsed = parse_uri(uri);

  if (parsed.scheme == "tcp") {
    int fd = ::socket(AF_INET, SOCK_STREAM, 0);
    if (fd < 0)
      throw std::runtime_error("socket() failed");

    int one = 1;
#ifdef _WIN32
    ::setsockopt(fd, SOL_SOCKET, SO_REUSEADDR,
                 reinterpret_cast<const char *>(&one), sizeof(one));
#else
    ::setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &one, sizeof(one));
#endif

    sockaddr_in addr{};
    addr.sin_family = AF_INET;
    addr.sin_port = htons(static_cast<uint16_t>(parsed.port));
    if (parsed.host == "0.0.0.0") {
      addr.sin_addr.s_addr = htonl(INADDR_ANY);
    } else if (::inet_pton(AF_INET, parsed.host.c_str(), &addr.sin_addr) != 1) {
      close_fd(fd, true);
      throw std::runtime_error("invalid tcp host: " + parsed.host);
    }

    if (::bind(fd, reinterpret_cast<sockaddr *>(&addr), sizeof(addr)) < 0) {
      close_fd(fd, true);
      throw std::runtime_error("bind() failed");
    }
    if (::listen(fd, 16) < 0) {
      close_fd(fd, true);
      throw std::runtime_error("listen() failed");
    }
    return tcp_listener{fd, parsed.host, parsed.port};
  }

  if (parsed.scheme == "unix") {
#ifdef _WIN32
    throw std::runtime_error("unix:// not supported on Windows");
#else
    int fd = ::socket(AF_UNIX, SOCK_STREAM, 0);
    if (fd < 0)
      throw std::runtime_error("socket() failed");

    unlink_path(parsed.path.c_str());
    sockaddr_un addr{};
    addr.sun_family = AF_UNIX;
    std::snprintf(addr.sun_path, sizeof(addr.sun_path), "%s", parsed.path.c_str());

    if (::bind(fd, reinterpret_cast<sockaddr *>(&addr), sizeof(addr)) < 0) {
      close_fd(fd, true);
      throw std::runtime_error("bind(unix) failed");
    }
    if (::listen(fd, 16) < 0) {
      close_fd(fd, true);
      throw std::runtime_error("listen(unix) failed");
    }
    return unix_listener{fd, parsed.path};
#endif
  }

  if (parsed.scheme == "stdio")
    return stdio_listener{parsed.raw, false};
  if (parsed.scheme == "ws" || parsed.scheme == "wss")
    return ws_listener{parsed.host, parsed.port, parsed.path, parsed.secure};

  throw std::invalid_argument("unsupported transport URI: " + uri);
}

/// Accept one connection from a listener.
/// - tcp/unix: OS socket accept
/// - stdio: single connection over stdin/stdout
inline connection accept(listener &lis) {
  if (auto *tcp = std::get_if<tcp_listener>(&lis)) {
    int fd = ::accept(tcp->fd, nullptr, nullptr);
    if (fd < 0) {
      throw std::runtime_error("accept(tcp) failed: " + last_socket_error());
    }
    return connection{fd, fd, "tcp", true, true};
  }

  if (auto *unix_lis = std::get_if<unix_listener>(&lis)) {
    int fd = ::accept(unix_lis->fd, nullptr, nullptr);
    if (fd < 0) {
      throw std::runtime_error("accept(unix) failed: " + last_socket_error());
    }
    return connection{fd, fd, "unix", true, true};
  }

  if (auto *stdio = std::get_if<stdio_listener>(&lis)) {
    if (stdio->consumed) {
      throw std::runtime_error("stdio:// accepts exactly one connection");
    }
    stdio->consumed = true;
    return connection{STDIN_FILENO, STDOUT_FILENO, "stdio", false, false};
  }

  if (std::holds_alternative<ws_listener>(lis)) {
    throw std::runtime_error(
        "ws/wss runtime accept is unsupported (metadata-only listener)");
  }

  throw std::runtime_error("listener variant cannot accept");
}

inline ssize_t conn_read(const connection &conn, void *buf, size_t n) {
  if (conn.scheme == "stdio") {
#ifdef _WIN32
    return ::_read(conn.read_fd, buf, static_cast<unsigned int>(n));
#else
    return ::read(conn.read_fd, buf, n);
#endif
  }
#ifdef _WIN32
  return ::recv(static_cast<SOCKET>(conn.read_fd), static_cast<char *>(buf),
                static_cast<int>(n), 0);
#else
  return ::recv(conn.read_fd, static_cast<char *>(buf), n, 0);
#endif
}

inline ssize_t conn_write(const connection &conn, const void *buf, size_t n) {
  ensure_sigpipe_ignored();
  if (conn.scheme == "stdio") {
#ifdef _WIN32
    return ::_write(conn.write_fd, buf, static_cast<unsigned int>(n));
#else
    return ::write(conn.write_fd, buf, n);
#endif
  }
#ifdef _WIN32
  return ::send(static_cast<SOCKET>(conn.write_fd),
                static_cast<const char *>(buf), static_cast<int>(n), 0);
#else
  return ::send(conn.write_fd, static_cast<const char *>(buf), n, 0);
#endif
}

inline void close_connection(connection &conn) {
  const int read_fd = conn.read_fd;
  const int write_fd = conn.write_fd;
  const bool socket_fds = conn.scheme != "stdio";

  if (conn.owns_read_fd && read_fd >= 0) {
    close_fd(read_fd, socket_fds);
  }
  if (conn.owns_write_fd && write_fd >= 0 && write_fd != read_fd) {
    close_fd(write_fd, socket_fds);
  }

  conn.read_fd = -1;
  conn.write_fd = -1;
}

inline void close_listener(listener &lis) {
  if (auto *tcp = std::get_if<tcp_listener>(&lis)) {
    if (tcp->fd >= 0) {
      close_fd(tcp->fd, true);
      tcp->fd = -1;
    }
    return;
  }
  if (auto *unix_lis = std::get_if<unix_listener>(&lis)) {
    if (unix_lis->fd >= 0) {
      close_fd(unix_lis->fd, true);
      unix_lis->fd = -1;
    }
    if (!unix_lis->path.empty())
      unlink_path(unix_lis->path.c_str());
    return;
  }
}

class holon_rpc_error : public std::runtime_error {
public:
  holon_rpc_error(int code, const std::string &message,
                  nlohmann::json data = nullptr)
      : std::runtime_error("rpc error " + std::to_string(code) + ": " +
                           message),
        code_(code), data_(std::move(data)) {}

  int code() const { return code_; }
  const nlohmann::json &data() const { return data_; }

private:
  int code_;
  nlohmann::json data_;
};

class holon_rpc_client {
public:
  using json = nlohmann::json;
  using handler_fn = std::function<json(const json &)>;

  holon_rpc_client(int heartbeat_interval_ms = 15000,
                   int heartbeat_timeout_ms = 5000,
                   int reconnect_min_delay_ms = 500,
                   int reconnect_max_delay_ms = 30000,
                   double reconnect_factor = 2.0,
                   double reconnect_jitter = 0.1,
                   int connect_timeout_ms = 10000,
                   int request_timeout_ms = 10000)
      : heartbeat_interval_ms_(heartbeat_interval_ms),
        heartbeat_timeout_ms_(heartbeat_timeout_ms),
        reconnect_min_delay_ms_(reconnect_min_delay_ms),
        reconnect_max_delay_ms_(reconnect_max_delay_ms),
        reconnect_factor_(reconnect_factor),
        reconnect_jitter_(reconnect_jitter),
        connect_timeout_ms_(connect_timeout_ms),
        request_timeout_ms_(request_timeout_ms) {}

  ~holon_rpc_client() { close(); }

  void connect(const std::string &url) {
    if (url.empty()) {
      throw std::invalid_argument("url is required");
    }

    close();

    endpoint_ = url;
    next_id_.store(0);
    reconnect_attempt_ = 0;
    running_.store(true);
    closed_.store(false);
    {
      std::lock_guard<std::mutex> lock(state_mu_);
      connected_ = false;
      last_error_.clear();
    }

    io_thread_ = std::thread([this]() { io_loop(); });
    heartbeat_thread_ = std::thread([this]() { heartbeat_loop(); });

    std::unique_lock<std::mutex> lock(state_mu_);
    bool ready = connected_cv_.wait_for(
        lock, std::chrono::milliseconds(connect_timeout_ms_),
        [this]() { return connected_ || !running_.load(); });

    if (!ready || !connected_) {
      std::string error =
          last_error_.empty() ? "holon-rpc connect timeout" : last_error_;
      lock.unlock();
      close();
      throw std::runtime_error(error);
    }
  }

  void register_handler(const std::string &method, handler_fn handler) {
    if (method.empty()) {
      throw std::invalid_argument("method is required");
    }
    std::lock_guard<std::mutex> lock(handlers_mu_);
    handlers_[method] = std::move(handler);
  }

  json invoke(const std::string &method, const json &params = json::object(),
              int timeout_ms = -1) {
    if (method.empty()) {
      throw std::invalid_argument("method is required");
    }

    wait_connected(connect_timeout_ms_);

    auto id = std::string("c") + std::to_string(next_id_.fetch_add(1) + 1);
    auto call = std::make_shared<pending_call>();
    {
      std::lock_guard<std::mutex> lock(pending_mu_);
      pending_[id] = call;
    }

    json payload = {{"jsonrpc", "2.0"}, {"id", id}, {"method", method},
                    {"params", params.is_object() ? params : json::object()}};

    try {
      send_json(payload);
    } catch (...) {
      remove_pending(id);
      throw;
    }

    int timeout = timeout_ms > 0 ? timeout_ms : request_timeout_ms_;
    std::unique_lock<std::mutex> lock(call->mu);
    bool done = call->cv.wait_for(lock, std::chrono::milliseconds(timeout),
                                  [&call]() { return call->done; });
    if (!done) {
      remove_pending(id);
      throw std::runtime_error("invoke timeout");
    }
    if (call->has_error) {
      throw holon_rpc_error(call->code, call->message, call->data);
    }
    return call->result;
  }

  void close() {
    if (!running_.load() && closed_.load()) {
      return;
    }

    closed_.store(true);
    running_.store(false);
    force_disconnect();

    {
      std::lock_guard<std::mutex> lock(state_mu_);
      connected_ = false;
    }
    connected_cv_.notify_all();

    if (io_thread_.joinable()) {
      io_thread_.join();
    }
    if (heartbeat_thread_.joinable()) {
      heartbeat_thread_.join();
    }

    close_socket();
    fail_all_pending(-32000, "holon-rpc client closed");
  }

private:
  struct pending_call {
    std::mutex mu;
    std::condition_variable cv;
    bool done = false;
    bool has_error = false;
    int code = -32603;
    std::string message = "internal error";
    json data = nullptr;
    json result = json::object();
  };

#if HOLONS_HAS_OPENSSL
  struct tls_state {
    SSL_CTX *ctx = nullptr;
    SSL *ssl = nullptr;

    ~tls_state() {
      if (ssl != nullptr) {
        SSL_free(ssl);
      }
      if (ctx != nullptr) {
        SSL_CTX_free(ctx);
      }
    }
  };
#endif

  enum class transport_mode { websocket, http_sse };

  struct transport_snapshot {
    int fd = -1;
#if HOLONS_HAS_OPENSSL
    std::shared_ptr<tls_state> tls;
#endif
  };

  void io_loop() {
    while (running_.load()) {
      if (socket_fd() < 0) {
        try {
          if (!open_socket()) {
            if (!running_.load()) {
              return;
            }
            auto delay = compute_backoff_delay_ms(reconnect_attempt_++);
            std::this_thread::sleep_for(std::chrono::milliseconds(delay));
            continue;
          }
        } catch (const std::exception &e) {
          {
            std::lock_guard<std::mutex> lock(state_mu_);
            connected_ = false;
            last_error_ = e.what();
          }
          connected_cv_.notify_all();
          running_.store(false);
          closed_.store(true);
          fail_all_pending(-32000, e.what());
          return;
        }

        reconnect_attempt_ = 0;
        {
          std::lock_guard<std::mutex> lock(state_mu_);
          connected_ = true;
          last_error_.clear();
        }
        connected_cv_.notify_all();
      }

      std::string text;
      if (!read_text_frame(text)) {
        mark_disconnected("holon-rpc connection closed");
        continue;
      }

      handle_incoming(text);
    }
  }

  void heartbeat_loop() {
    while (running_.load()) {
      sleep_interruptible(heartbeat_interval_ms_);
      if (!running_.load()) {
        return;
      }
      if (!is_connected()) {
        continue;
      }

      try {
        (void)invoke("rpc.heartbeat", json::object(), heartbeat_timeout_ms_);
      } catch (...) {
        force_disconnect();
      }
    }
  }

  void sleep_interruptible(int duration_ms) const {
    int slept = 0;
    while (running_.load() && slept < duration_ms) {
      int step = std::min(100, duration_ms - slept);
      std::this_thread::sleep_for(std::chrono::milliseconds(step));
      slept += step;
    }
  }

  void wait_connected(int timeout_ms) {
    std::unique_lock<std::mutex> lock(state_mu_);
    bool ready = connected_cv_.wait_for(
        lock, std::chrono::milliseconds(timeout_ms),
        [this]() { return connected_ || !running_.load(); });
    if (!ready || !connected_) {
      throw std::runtime_error(last_error_.empty() ? "not connected"
                                                   : last_error_);
    }
  }

  bool is_connected() const {
    std::lock_guard<std::mutex> lock(state_mu_);
    return connected_;
  }

  transport_snapshot current_transport() const {
    std::lock_guard<std::mutex> lock(state_mu_);
    transport_snapshot snapshot;
    snapshot.fd = sockfd_;
#if HOLONS_HAS_OPENSSL
    snapshot.tls = tls_state_;
#endif
    return snapshot;
  }

  int socket_fd() const { return current_transport().fd; }

  void set_transport(int fd
#if HOLONS_HAS_OPENSSL
                     ,
                     std::shared_ptr<tls_state> tls = nullptr
#endif
  ) {
    std::lock_guard<std::mutex> lock(state_mu_);
    sockfd_ = fd;
#if HOLONS_HAS_OPENSSL
    tls_state_ = std::move(tls);
#endif
  }

  void close_socket() {
    std::lock_guard<std::mutex> lock(state_mu_);
    if (sockfd_ >= 0) {
      close_fd(sockfd_, true);
      sockfd_ = -1;
    }
#if HOLONS_HAS_OPENSSL
    tls_state_.reset();
#endif
  }

  void force_disconnect() {
    int fd = -1;
    {
      std::lock_guard<std::mutex> lock(state_mu_);
      fd = sockfd_;
    }
    if (fd >= 0) {
      ::shutdown(fd, socket_shutdown_both());
    }
  }

  void mark_disconnected(const std::string &reason) {
    close_socket();
    {
      std::lock_guard<std::mutex> lock(state_mu_);
      connected_ = false;
      last_error_ = reason;
    }
    connected_cv_.notify_all();
    fail_all_pending(-32000, reason);
  }

  bool send_all(const void *data, size_t size) {
    ensure_sigpipe_ignored();
    auto transport = current_transport();
    if (transport.fd < 0) {
      return false;
    }

    const auto *ptr = static_cast<const uint8_t *>(data);
    size_t sent = 0;
    while (sent < size) {
#if HOLONS_HAS_OPENSSL
      if (transport.tls && transport.tls->ssl != nullptr) {
        int n = SSL_write(
            transport.tls->ssl, ptr + sent,
            static_cast<int>(std::min(
                size - sent,
                static_cast<size_t>(std::numeric_limits<int>::max()))));
        if (n <= 0) {
          return false;
        }
        sent += static_cast<size_t>(n);
        continue;
      }
#endif
      size_t chunk = std::min(size - sent,
                              static_cast<size_t>(std::numeric_limits<int>::max()));
      ssize_t n = ::send(transport.fd, reinterpret_cast<const char *>(ptr + sent),
                         static_cast<int>(chunk), 0);
      if (n <= 0) {
        return false;
      }
      sent += static_cast<size_t>(n);
    }
    return true;
  }

  bool read_exact(void *data, size_t size) {
    auto transport = current_transport();
    if (transport.fd < 0) {
      return false;
    }

    auto *ptr = static_cast<uint8_t *>(data);
    size_t got = 0;
    while (got < size) {
#if HOLONS_HAS_OPENSSL
      if (transport.tls && transport.tls->ssl != nullptr) {
        int n = SSL_read(
            transport.tls->ssl, ptr + got,
            static_cast<int>(std::min(
                size - got,
                static_cast<size_t>(std::numeric_limits<int>::max()))));
        if (n <= 0) {
          return false;
        }
        got += static_cast<size_t>(n);
        continue;
      }
#endif
      size_t chunk = std::min(size - got,
                              static_cast<size_t>(std::numeric_limits<int>::max()));
      ssize_t n = ::recv(transport.fd, reinterpret_cast<char *>(ptr + got),
                         static_cast<int>(chunk), 0);
      if (n <= 0) {
        return false;
      }
      got += static_cast<size_t>(n);
    }
    return true;
  }

#if HOLONS_HAS_OPENSSL
  static void ensure_openssl_initialized() {
    static std::once_flag once;
    std::call_once(once, []() {
      OPENSSL_init_ssl(0, nullptr);
      SSL_load_error_strings();
    });
  }

  std::shared_ptr<tls_state> connect_tls(const parsed_uri &parsed, int fd) {
    ensure_openssl_initialized();

    auto params = parse_query_params(parsed.query);
    bool insecure = false;
    if (auto it = params.find("insecure"); it != params.end()) {
      auto lowered = it->second;
      std::transform(lowered.begin(), lowered.end(), lowered.begin(),
                     [](unsigned char ch) {
                       return static_cast<char>(std::tolower(ch));
                     });
      insecure = lowered == "1" || lowered == "true" || lowered == "yes";
    }

    auto tls = std::make_shared<tls_state>();
    tls->ctx = SSL_CTX_new(TLS_client_method());
    if (tls->ctx == nullptr) {
      throw std::runtime_error("failed to create TLS context");
    }

    if (insecure) {
      SSL_CTX_set_verify(tls->ctx, SSL_VERIFY_NONE, nullptr);
    } else {
      SSL_CTX_set_verify(tls->ctx, SSL_VERIFY_PEER, nullptr);
      (void)SSL_CTX_set_default_verify_paths(tls->ctx);
      if (auto it = params.find("ca"); it != params.end() && !it->second.empty()) {
        if (SSL_CTX_load_verify_locations(tls->ctx, it->second.c_str(),
                                          nullptr) != 1) {
          throw std::runtime_error("failed to load TLS CA file: " + it->second);
        }
      }
    }

    tls->ssl = SSL_new(tls->ctx);
    if (tls->ssl == nullptr) {
      throw std::runtime_error("failed to create TLS session");
    }

    if (!parsed.host.empty()) {
      (void)SSL_set_tlsext_host_name(tls->ssl, parsed.host.c_str());
    }

    if (SSL_set_fd(tls->ssl, fd) != 1) {
      throw std::runtime_error("failed to bind TLS session to socket");
    }

    if (SSL_connect(tls->ssl) != 1) {
      throw std::runtime_error("TLS handshake failed");
    }

    if (!insecure && SSL_get_verify_result(tls->ssl) != X509_V_OK) {
      throw std::runtime_error("TLS certificate verification failed");
    }

    return tls;
  }
#endif

  bool open_socket() {
#ifdef _WIN32
    detail::ensure_winsock();
#endif
    auto parsed = parse_uri(endpoint_);
    if (parsed.scheme != "ws" && parsed.scheme != "wss") {
      throw std::runtime_error("holon-rpc requires ws:// or wss:// endpoint");
    }

    int fd = ::socket(AF_INET, SOCK_STREAM, 0);
    if (fd < 0) {
      return false;
    }

    sockaddr_in addr{};
    addr.sin_family = AF_INET;
    addr.sin_port = htons(static_cast<uint16_t>(parsed.port));
    if (::inet_pton(AF_INET, parsed.host.c_str(), &addr.sin_addr) != 1) {
      close_fd(fd, true);
      throw std::runtime_error("invalid ws host: " + parsed.host);
    }

    if (::connect(fd, reinterpret_cast<sockaddr *>(&addr), sizeof(addr)) != 0) {
      close_fd(fd, true);
      return false;
    }

#if HOLONS_HAS_OPENSSL
    std::shared_ptr<tls_state> tls;
    if (parsed.secure) {
      try {
        tls = connect_tls(parsed, fd);
      } catch (...) {
        close_fd(fd, true);
        throw;
      }
    }
#else
    if (parsed.secure) {
      close_fd(fd, true);
      throw std::runtime_error(
          "wss:// requires OpenSSL support in cpp-holons");
    }
#endif

    set_transport(fd
#if HOLONS_HAS_OPENSSL
                  ,
                  tls
#endif
    );

    std::ostringstream req;
    req << "GET " << (parsed.path.empty() ? "/rpc" : parsed.path)
        << " HTTP/1.1\r\n";
    req << "Host: " << parsed.host << ":" << parsed.port << "\r\n";
    req << "Upgrade: websocket\r\n";
    req << "Connection: Upgrade\r\n";
    req << "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n";
    req << "Sec-WebSocket-Version: 13\r\n";
    req << "Sec-WebSocket-Protocol: holon-rpc\r\n\r\n";

    auto req_str = req.str();
    if (!send_all(req_str.data(), req_str.size())) {
      close_socket();
      return false;
    }

    std::string headers;
    headers.reserve(4096);
    char ch = 0;
    while (headers.find("\r\n\r\n") == std::string::npos) {
      if (!read_exact(&ch, 1)) {
        close_socket();
        return false;
      }
      headers.push_back(ch);
      if (headers.size() > 16384) {
        close_socket();
        throw std::runtime_error("websocket handshake too large");
      }
    }

    std::string lower = headers;
    for (auto &c : lower) {
      c = static_cast<char>(std::tolower(static_cast<unsigned char>(c)));
    }
    if (lower.find(" 101 ") == std::string::npos ||
        lower.find("sec-websocket-protocol: holon-rpc") ==
            std::string::npos) {
      close_socket();
      throw std::runtime_error(
          "server did not negotiate holon-rpc websocket protocol");
    }

    return true;
  }

  void send_json(const json &payload) {
    std::string data = payload.dump();
    if (!send_frame(0x1, data)) {
      throw std::runtime_error("websocket send failed");
    }
  }

  bool send_frame(uint8_t opcode, const std::string &payload) {
    if (socket_fd() < 0) {
      return false;
    }

    std::vector<uint8_t> frame;
    frame.reserve(payload.size() + 16);
    frame.push_back(static_cast<uint8_t>(0x80 | (opcode & 0x0F)));

    uint64_t len = payload.size();
    if (len < 126) {
      frame.push_back(static_cast<uint8_t>(0x80 | len));
    } else if (len <= 0xFFFF) {
      frame.push_back(static_cast<uint8_t>(0x80 | 126));
      frame.push_back(static_cast<uint8_t>((len >> 8) & 0xFF));
      frame.push_back(static_cast<uint8_t>(len & 0xFF));
    } else {
      frame.push_back(static_cast<uint8_t>(0x80 | 127));
      for (int i = 7; i >= 0; --i) {
        frame.push_back(static_cast<uint8_t>((len >> (i * 8)) & 0xFF));
      }
    }

    std::array<uint8_t, 4> mask{};
    for (auto &b : mask) {
      b = static_cast<uint8_t>(random_device_());
      frame.push_back(b);
    }

    for (size_t i = 0; i < payload.size(); ++i) {
      frame.push_back(static_cast<uint8_t>(payload[i]) ^ mask[i % 4]);
    }

    std::lock_guard<std::mutex> lock(send_mu_);
    return send_all(frame.data(), frame.size());
  }

  bool read_text_frame(std::string &out) {
    out.clear();
    std::string fragmented;
    bool reading_fragment = false;

    while (running_.load()) {
      if (socket_fd() < 0) {
        return false;
      }

      uint8_t header[2];
      if (!read_exact(header, 2)) {
        return false;
      }

      bool fin = (header[0] & 0x80) != 0;
      uint8_t opcode = static_cast<uint8_t>(header[0] & 0x0F);
      bool masked = (header[1] & 0x80) != 0;
      uint64_t len = static_cast<uint64_t>(header[1] & 0x7F);

      if (len == 126) {
        uint8_t ext[2];
        if (!read_exact(ext, 2)) {
          return false;
        }
        len = (static_cast<uint64_t>(ext[0]) << 8) | ext[1];
      } else if (len == 127) {
        uint8_t ext[8];
        if (!read_exact(ext, 8)) {
          return false;
        }
        len = 0;
        for (int i = 0; i < 8; ++i) {
          len = (len << 8) | ext[i];
        }
      }

      std::array<uint8_t, 4> mask{};
      if (masked) {
        if (!read_exact(mask.data(), mask.size())) {
          return false;
        }
      }

      std::string payload(len, '\0');
      if (len > 0 && !read_exact(payload.data(), len)) {
        return false;
      }
      if (masked) {
        for (size_t i = 0; i < payload.size(); ++i) {
          payload[i] = static_cast<char>(payload[i] ^ mask[i % 4]);
        }
      }

      if (opcode == 0x8) { // close
        return false;
      }
      if (opcode == 0x9) { // ping
        if (!send_frame(0xA, payload)) {
          return false;
        }
        continue;
      }
      if (opcode == 0xA) { // pong
        continue;
      }

      if (opcode == 0x1 || opcode == 0x0) { // text or continuation
        if (opcode == 0x1 && !reading_fragment) {
          fragmented.clear();
        }
        fragmented.append(payload);
        reading_fragment = !fin;
        if (fin) {
          out = fragmented;
          return true;
        }
        continue;
      }
    }

    return false;
  }

  void handle_incoming(const std::string &text) {
    json msg;
    try {
      msg = json::parse(text);
    } catch (...) {
      return;
    }

    if (!msg.is_object()) {
      return;
    }

    if (msg.contains("method")) {
      handle_request(msg);
      return;
    }
    if (msg.contains("result") || msg.contains("error")) {
      handle_response(msg);
    }
  }

  void handle_request(const json &msg) {
    json id = msg.contains("id") ? msg["id"] : json();
    bool has_id = !id.is_null();

    std::string method;
    if (msg.contains("method") && msg["method"].is_string()) {
      method = msg["method"].get<std::string>();
    }
    std::string jsonrpc;
    if (msg.contains("jsonrpc") && msg["jsonrpc"].is_string()) {
      jsonrpc = msg["jsonrpc"].get<std::string>();
    }

    if (jsonrpc != "2.0" || method.empty()) {
      if (has_id) {
        send_error(id, -32600, "invalid request");
      }
      return;
    }

    if (method == "rpc.heartbeat") {
      if (has_id) {
        send_result(id, json::object());
      }
      return;
    }

    if (has_id) {
      if (!id.is_string()) {
        send_error(id, -32600, "server request id must start with 's'");
        return;
      }
      auto sid = id.get<std::string>();
      if (sid.empty() || sid[0] != 's') {
        send_error(id, -32600, "server request id must start with 's'");
        return;
      }
    }

    handler_fn handler;
    {
      std::lock_guard<std::mutex> lock(handlers_mu_);
      auto it = handlers_.find(method);
      if (it == handlers_.end()) {
        if (has_id) {
          send_error(id, -32601, "method \"" + method + "\" not found");
        }
        return;
      }
      handler = it->second;
    }

    json params = msg.contains("params") && msg["params"].is_object()
                      ? msg["params"]
                      : json::object();

    try {
      json result = handler(params);
      if (has_id) {
        send_result(id, result.is_object() ? result : json::object());
      }
    } catch (const holon_rpc_error &rpc_error) {
      if (has_id) {
        send_error(id, rpc_error.code(), rpc_error.what(), rpc_error.data());
      }
    } catch (const std::exception &e) {
      if (has_id) {
        send_error(id, 13, e.what());
      }
    } catch (...) {
      if (has_id) {
        send_error(id, 13, "internal error");
      }
    }
  }

  void handle_response(const json &msg) {
    if (!msg.contains("id")) {
      return;
    }

    std::string id;
    if (msg["id"].is_string()) {
      id = msg["id"].get<std::string>();
    } else {
      id = msg["id"].dump();
    }

    std::shared_ptr<pending_call> call;
    {
      std::lock_guard<std::mutex> lock(pending_mu_);
      auto it = pending_.find(id);
      if (it == pending_.end()) {
        return;
      }
      call = it->second;
      pending_.erase(it);
    }

    std::lock_guard<std::mutex> lock(call->mu);
    if (msg.contains("error") && msg["error"].is_object()) {
      const auto &err = msg["error"];
      call->has_error = true;
      call->code = err.contains("code") && err["code"].is_number_integer()
                       ? err["code"].get<int>()
                       : -32603;
      call->message = err.contains("message") && err["message"].is_string()
                          ? err["message"].get<std::string>()
                          : "internal error";
      call->data = err.contains("data") ? err["data"] : nullptr;
    } else {
      call->result = msg.contains("result") && msg["result"].is_object()
                         ? msg["result"]
                         : json::object();
    }
    call->done = true;
    call->cv.notify_all();
  }

  void send_result(const json &id, const json &result) {
    json payload = {{"jsonrpc", "2.0"}, {"id", id},
                    {"result", result.is_object() ? result : json::object()}};
    send_json(payload);
  }

  void send_error(const json &id, int code, const std::string &message,
                  const json &data = nullptr) {
    json err = {{"code", code}, {"message", message}};
    if (!data.is_null()) {
      err["data"] = data;
    }
    json payload = {{"jsonrpc", "2.0"}, {"id", id}, {"error", err}};
    send_json(payload);
  }

  void fail_all_pending(int code, const std::string &message) {
    std::unordered_map<std::string, std::shared_ptr<pending_call>> snapshot;
    {
      std::lock_guard<std::mutex> lock(pending_mu_);
      snapshot.swap(pending_);
    }

    for (auto &kv : snapshot) {
      auto &call = kv.second;
      std::lock_guard<std::mutex> lock(call->mu);
      call->done = true;
      call->has_error = true;
      call->code = code;
      call->message = message;
      call->data = nullptr;
      call->cv.notify_all();
    }
  }

  void remove_pending(const std::string &id) {
    std::lock_guard<std::mutex> lock(pending_mu_);
    pending_.erase(id);
  }

  int compute_backoff_delay_ms(int attempt) const {
    double base = std::min(
        reconnect_min_delay_ms_ * std::pow(reconnect_factor_, attempt),
        static_cast<double>(reconnect_max_delay_ms_));
    double jitter = base * reconnect_jitter_ *
                    std::uniform_real_distribution<double>(0.0, 1.0)(
                        mutable_rng_);
    int delay = static_cast<int>(base + jitter);
    return std::max(1, delay);
  }

  int heartbeat_interval_ms_;
  int heartbeat_timeout_ms_;
  int reconnect_min_delay_ms_;
  int reconnect_max_delay_ms_;
  double reconnect_factor_;
  double reconnect_jitter_;
  int connect_timeout_ms_;
  int request_timeout_ms_;

  mutable std::mutex state_mu_;
  mutable std::condition_variable connected_cv_;
  int sockfd_ = -1;
#if HOLONS_HAS_OPENSSL
  std::shared_ptr<tls_state> tls_state_;
#endif
  bool connected_ = false;
  std::string last_error_;

  std::string endpoint_;
  transport_mode transport_mode_ = transport_mode::websocket;
  std::atomic<bool> running_{false};
  std::atomic<bool> closed_{true};
  std::thread io_thread_;
  std::thread heartbeat_thread_;
  int reconnect_attempt_ = 0;

  std::mutex send_mu_;
  mutable std::mt19937 mutable_rng_{std::random_device{}()};
  std::random_device random_device_;

  std::mutex handlers_mu_;
  std::unordered_map<std::string, handler_fn> handlers_;

  std::mutex pending_mu_;
  std::unordered_map<std::string, std::shared_ptr<pending_call>> pending_;

  std::atomic<uint64_t> next_id_{0};
};

struct holon_rpc_sse_event {
  nlohmann::json result = nlohmann::json::object();
  nlohmann::json error_data = nullptr;
  std::string event;
  std::string id;
  std::string error_message;
  int error_code = 0;
  bool has_error = false;
};

class holon_rpc_http_client {
public:
  using json = nlohmann::json;

  explicit holon_rpc_http_client(std::string base_url, int timeout_ms = 10000)
      : base_url_(std::move(base_url)), timeout_ms_(timeout_ms) {}

  json invoke(const std::string &method, const json &params = json::object()) const {
    auto parsed = parse_base_url();
    auto path = method_path(parsed, method);
    auto body = params.is_object() ? params.dump() : json::object().dump();
    auto response = send_request(parsed, "POST", path, body, "application/json",
                                 "application/json");
    return decode_json_rpc_body(response.status_code, response.body);
  }

  std::vector<holon_rpc_sse_event> stream(
      const std::string &method,
      const json &params = json::object()) const {
    auto parsed = parse_base_url();
    auto path = method_path(parsed, method);
    auto body = params.is_object() ? params.dump() : json::object().dump();
    auto response = send_request(parsed, "POST", path, body, "application/json",
                                 "text/event-stream");
    return decode_sse(response);
  }

  std::vector<holon_rpc_sse_event> stream_query(
      const std::string &method,
      const std::map<std::string, std::string> &params) const {
    auto parsed = parse_base_url();
    auto path = method_path(parsed, method);
    bool first = true;
    for (const auto &[key, value] : params) {
      path += first ? "?" : "&";
      first = false;
      path += uri_encode(key);
      path += "=";
      path += uri_encode(value);
    }
    auto response = send_request(parsed, "GET", path, "", "", "text/event-stream");
    return decode_sse(response);
  }

private:
#if HOLONS_HAS_OPENSSL
  struct tls_state {
    SSL_CTX *ctx = nullptr;
    SSL *ssl = nullptr;

    ~tls_state() {
      if (ssl != nullptr) {
        SSL_free(ssl);
      }
      if (ctx != nullptr) {
        SSL_CTX_free(ctx);
      }
    }
  };
#endif

  struct transport {
    int fd = -1;
#if HOLONS_HAS_OPENSSL
    std::shared_ptr<tls_state> tls;
#endif

    ~transport() {
      if (fd >= 0) {
        close_fd(fd, true);
      }
    }
  };

  struct http_response {
    int status_code = 0;
    std::string body;
    std::vector<holon_rpc_sse_event> sse_events;
  };

  parsed_uri parse_base_url() const {
    auto parsed = parse_uri(base_url_);
    if (parsed.scheme != "http" && parsed.scheme != "https") {
      throw std::invalid_argument(
          "holon-rpc http client requires http:// or https:// endpoint");
    }
    return parsed;
  }

  static std::string trim_method(std::string method) {
    while (!method.empty() && method.front() == '/') {
      method.erase(method.begin());
    }
    while (!method.empty() && method.back() == '/') {
      method.pop_back();
    }
    return method;
  }

  static std::string method_path(const parsed_uri &parsed,
                                 const std::string &method) {
    std::string path = parsed.path.empty() ? "/api/v1/rpc" : parsed.path;
    if (!path.empty() && path.back() == '/') {
      path.pop_back();
    }
    return path + "/" + trim_method(method);
  }

  static std::string lower_header_name(std::string value) {
    std::transform(value.begin(), value.end(), value.begin(),
                   [](unsigned char ch) {
                     return static_cast<char>(std::tolower(ch));
                   });
    return value;
  }

  static std::string header_value(
      const std::unordered_map<std::string, std::string> &headers,
      const std::string &name) {
    auto it = headers.find(lower_header_name(name));
    return it == headers.end() ? std::string() : it->second;
  }

  static void ensure_openssl() {
#if HOLONS_HAS_OPENSSL
    static std::once_flag once;
    std::call_once(once, []() {
      OPENSSL_init_ssl(0, nullptr);
      SSL_load_error_strings();
    });
#endif
  }

  transport open_transport(const parsed_uri &parsed) const {
    transport out;
#ifdef _WIN32
    detail::ensure_winsock();
#endif
    out.fd = ::socket(AF_INET, SOCK_STREAM, 0);
    if (out.fd < 0) {
      throw std::runtime_error("socket() failed");
    }

    sockaddr_in addr{};
    addr.sin_family = AF_INET;
    addr.sin_port = htons(static_cast<uint16_t>(parsed.port));
    if (::inet_pton(AF_INET, parsed.host.c_str(), &addr.sin_addr) != 1) {
      throw std::runtime_error("invalid host: " + parsed.host);
    }
    if (::connect(out.fd, reinterpret_cast<sockaddr *>(&addr), sizeof(addr)) != 0) {
      throw std::runtime_error("connect() failed: " + last_socket_error());
    }

#if HOLONS_HAS_OPENSSL
    if (parsed.secure) {
      ensure_openssl();
      auto params = parse_query_params(parsed.query);
      bool insecure = false;
      if (auto it = params.find("insecure"); it != params.end()) {
        auto lowered = it->second;
        std::transform(lowered.begin(), lowered.end(), lowered.begin(),
                       [](unsigned char ch) {
                         return static_cast<char>(std::tolower(ch));
                       });
        insecure = lowered == "1" || lowered == "true" || lowered == "yes";
      }

      out.tls = std::make_shared<tls_state>();
      out.tls->ctx = SSL_CTX_new(TLS_client_method());
      if (out.tls->ctx == nullptr) {
        throw std::runtime_error("failed to create TLS context");
      }
      if (insecure) {
        SSL_CTX_set_verify(out.tls->ctx, SSL_VERIFY_NONE, nullptr);
      } else {
        SSL_CTX_set_verify(out.tls->ctx, SSL_VERIFY_PEER, nullptr);
        (void)SSL_CTX_set_default_verify_paths(out.tls->ctx);
        if (auto it = params.find("ca"); it != params.end() && !it->second.empty()) {
          if (SSL_CTX_load_verify_locations(out.tls->ctx, it->second.c_str(),
                                            nullptr) != 1) {
            throw std::runtime_error("failed to load TLS CA file: " + it->second);
          }
        }
      }
      out.tls->ssl = SSL_new(out.tls->ctx);
      if (out.tls->ssl == nullptr) {
        throw std::runtime_error("failed to create TLS session");
      }
      if (!parsed.host.empty()) {
        (void)SSL_set_tlsext_host_name(out.tls->ssl, parsed.host.c_str());
      }
      if (SSL_set_fd(out.tls->ssl, out.fd) != 1 || SSL_connect(out.tls->ssl) != 1) {
        throw std::runtime_error("TLS handshake failed");
      }
      if (!insecure && SSL_get_verify_result(out.tls->ssl) != X509_V_OK) {
        throw std::runtime_error("TLS certificate verification failed");
      }
    }
#else
    if (parsed.secure) {
      throw std::runtime_error("https:// requires OpenSSL support in cpp-holons");
    }
#endif

    return out;
  }

  static bool send_all(const transport &transport, const void *data, size_t size) {
    ensure_sigpipe_ignored();
    const auto *ptr = static_cast<const uint8_t *>(data);
    size_t sent = 0;
    while (sent < size) {
#if HOLONS_HAS_OPENSSL
      if (transport.tls && transport.tls->ssl != nullptr) {
        int n = SSL_write(
            transport.tls->ssl, ptr + sent,
            static_cast<int>(std::min(
                size - sent,
                static_cast<size_t>(std::numeric_limits<int>::max()))));
        if (n <= 0) {
          return false;
        }
        sent += static_cast<size_t>(n);
        continue;
      }
#endif
      size_t chunk = std::min(size - sent,
                              static_cast<size_t>(std::numeric_limits<int>::max()));
      ssize_t n = ::send(transport.fd, reinterpret_cast<const char *>(ptr + sent),
                         static_cast<int>(chunk), 0);
      if (n <= 0) {
        return false;
      }
      sent += static_cast<size_t>(n);
    }
    return true;
  }

  static ssize_t read_some(const transport &transport, void *data, size_t size) {
#if HOLONS_HAS_OPENSSL
    if (transport.tls && transport.tls->ssl != nullptr) {
      return SSL_read(transport.tls->ssl, data,
                      static_cast<int>(std::min(
                          size, static_cast<size_t>(std::numeric_limits<int>::max()))));
    }
#endif
    return ::recv(transport.fd, reinterpret_cast<char *>(data),
                  static_cast<int>(std::min(
                      size, static_cast<size_t>(std::numeric_limits<int>::max()))),
                  0);
  }

  static bool read_exact(const transport &transport, void *data, size_t size) {
    auto *ptr = static_cast<uint8_t *>(data);
    size_t got = 0;
    while (got < size) {
      ssize_t n = read_some(transport, ptr + got, size - got);
      if (n <= 0) {
        return false;
      }
      got += static_cast<size_t>(n);
    }
    return true;
  }

  static void append_header(std::ostringstream *request, const std::string &name,
                            const std::string &value) {
    *request << name << ": " << value << "\r\n";
  }

  http_response send_request(const parsed_uri &parsed, const std::string &verb,
                             const std::string &path, const std::string &body,
                             const std::string &content_type,
                             const std::string &accept) const {
    auto transport = open_transport(parsed);

    std::ostringstream request;
    request << verb << " " << path << " HTTP/1.1\r\n";
    append_header(&request, "Host",
                  parsed.host + ":" + std::to_string(parsed.port));
    append_header(&request, "Connection", "close");
    append_header(&request, "Accept", accept);
    if (!content_type.empty()) {
      append_header(&request, "Content-Type", content_type);
    }
    if (!body.empty()) {
      append_header(&request, "Content-Length", std::to_string(body.size()));
    } else if (verb == "POST") {
      append_header(&request, "Content-Length", "0");
    }
    request << "\r\n";
    request << body;

    auto wire = request.str();
    if (!send_all(transport, wire.data(), wire.size())) {
      throw std::runtime_error("http request write failed");
    }

    std::string headers;
    headers.reserve(4096);
    char ch = '\0';
    while (headers.find("\r\n\r\n") == std::string::npos) {
      if (!read_exact(transport, &ch, 1)) {
        throw std::runtime_error("http response header read failed");
      }
      headers.push_back(ch);
      if (headers.size() > 32768) {
        throw std::runtime_error("http response headers too large");
      }
    }

    auto separator = headers.find("\r\n\r\n");
    auto header_text = headers.substr(0, separator);
    auto body_prefix = headers.substr(separator + 4);

    std::istringstream lines(header_text);
    std::string status_line;
    std::getline(lines, status_line);
    if (!status_line.empty() && status_line.back() == '\r') {
      status_line.pop_back();
    }
    std::istringstream status_stream(status_line);
    std::string http_version;
    http_response response;
    status_stream >> http_version >> response.status_code;
    if (response.status_code <= 0) {
      throw std::runtime_error("invalid http response status line");
    }

    std::unordered_map<std::string, std::string> header_map;
    std::string line;
    while (std::getline(lines, line)) {
      if (!line.empty() && line.back() == '\r') {
        line.pop_back();
      }
      auto colon = line.find(':');
      if (colon == std::string::npos) {
        continue;
      }
      auto name = lower_header_name(line.substr(0, colon));
      auto value = line.substr(colon + 1);
      while (!value.empty() && std::isspace(static_cast<unsigned char>(value.front()))) {
        value.erase(value.begin());
      }
      header_map[name] = value;
    }

    auto content_length_text = header_value(header_map, "content-length");
    if (!content_length_text.empty() &&
        (accept != "text/event-stream" || response.status_code >= 400)) {
      size_t content_length = static_cast<size_t>(std::stoull(content_length_text));
      response.body = body_prefix;
      if (response.body.size() < content_length) {
        std::string tail(content_length - response.body.size(), '\0');
        if (!read_exact(transport, tail.data(), tail.size())) {
          throw std::runtime_error("http response body read failed");
        }
        response.body += tail;
      } else if (response.body.size() > content_length) {
        response.body.resize(content_length);
      }
      return response;
    }

    if (accept != "text/event-stream" || response.status_code >= 400) {
      response.body = body_prefix;
      std::array<char, 4096> buffer{};
      for (;;) {
        auto n = read_some(transport, buffer.data(), buffer.size());
        if (n <= 0) {
          break;
        }
        response.body.append(buffer.data(), static_cast<size_t>(n));
      }
      return response;
    }

    std::string sse_text = body_prefix;
    std::array<char, 4096> buffer{};
    for (;;) {
      auto n = read_some(transport, buffer.data(), buffer.size());
      if (n <= 0) {
        break;
      }
      sse_text.append(buffer.data(), static_cast<size_t>(n));
    }
    response.sse_events = parse_sse_events(response.status_code, sse_text);
    return response;
  }

  static json decode_json_rpc_body(int status_code, const std::string &body) {
    json payload;
    try {
      payload = json::parse(body);
    } catch (...) {
      throw std::runtime_error("invalid http json-rpc response");
    }

    if (payload.contains("error") && payload["error"].is_object()) {
      const auto &err = payload["error"];
      throw holon_rpc_error(
          err.value("code", -32603), err.value("message", std::string("internal error")),
          err.contains("data") ? err["data"] : nullptr);
    }

    if (status_code >= 400) {
      throw std::runtime_error("http status " + std::to_string(status_code));
    }

    if (payload.contains("result") && payload["result"].is_object()) {
      return payload["result"];
    }
    return json::object();
  }

  static std::vector<holon_rpc_sse_event> decode_sse(
      const http_response &response) {
    if (response.status_code >= 400) {
      if (!response.body.empty()) {
        (void)decode_json_rpc_body(response.status_code, response.body);
      }
      throw std::runtime_error("http status " +
                               std::to_string(response.status_code));
    }
    return response.sse_events;
  }

  static std::vector<holon_rpc_sse_event> parse_sse_events(
      int status_code, const std::string &body) {
    std::vector<holon_rpc_sse_event> events;
    if (status_code >= 400) {
      return events;
    }

    struct raw_event {
      std::string event;
      std::string id;
      std::string data;
    } current;

    auto flush = [&]() {
      if (current.event.empty() && current.id.empty() && current.data.empty()) {
        return false;
      }

      holon_rpc_sse_event event;
      event.event = current.event;
      event.id = current.id;

      if (current.event == "message" || current.event == "error") {
        json payload = json::parse(current.data);
        if (payload.contains("error") && payload["error"].is_object()) {
          const auto &err = payload["error"];
          event.has_error = true;
          event.error_code = err.value("code", -32603);
          event.error_message = err.value("message", std::string("internal error"));
          event.error_data = err.contains("data") ? err["data"] : nullptr;
        } else if (payload.contains("result") && payload["result"].is_object()) {
          event.result = payload["result"];
        }
      }

      events.push_back(std::move(event));
      current = raw_event{};
      return !events.empty() && events.back().event == "done";
    };

    std::istringstream in(body);
    std::string line;
    while (std::getline(in, line)) {
      if (!line.empty() && line.back() == '\r') {
        line.pop_back();
      }
      if (line.empty()) {
        if (flush()) {
          break;
        }
        continue;
      }
      if (line.rfind("event:", 0) == 0) {
        current.event = line.substr(6);
        while (!current.event.empty() &&
               std::isspace(static_cast<unsigned char>(current.event.front()))) {
          current.event.erase(current.event.begin());
        }
      } else if (line.rfind("id:", 0) == 0) {
        current.id = line.substr(3);
        while (!current.id.empty() &&
               std::isspace(static_cast<unsigned char>(current.id.front()))) {
          current.id.erase(current.id.begin());
        }
      } else if (line.rfind("data:", 0) == 0) {
        current.data = line.substr(5);
        while (!current.data.empty() &&
               std::isspace(static_cast<unsigned char>(current.data.front()))) {
          current.data.erase(current.data.begin());
        }
      }
    }
    (void)flush();
    return events;
  }

  std::string base_url_;
  int timeout_ms_ = 10000;
};

/// Parsed holon identity from a holon manifest.
struct HolonIdentity {
  std::string uuid;
  std::string given_name;
  std::string family_name;
  std::string motto;
  std::string composer;
  std::string clade;
  std::string status;
  std::string born;
  std::string lang;
};

struct HolonBuild {
  std::string runner;
  std::string main;
};

struct HolonArtifacts {
  std::string binary;
  std::string primary;
};

struct HolonManifest {
  HolonIdentity identity;
  std::string lang;
  std::string kind;
  HolonBuild build;
  HolonArtifacts artifacts;
};

struct HolonEntry {
  std::string slug;
  std::string uuid;
  std::filesystem::path dir;
  std::filesystem::path relative_path;
  std::string origin;
  HolonIdentity identity;
  std::optional<HolonManifest> manifest;
};

inline std::string trim_copy(std::string value) {
  auto start = value.find_first_not_of(" \t\r\n");
  if (start == std::string::npos) {
    return "";
  }
  auto end = value.find_last_not_of(" \t\r\n");
  return value.substr(start, end - start + 1);
}

inline std::string strip_quotes(std::string value) {
  if (value.size() >= 2 &&
      ((value.front() == '"' && value.back() == '"') ||
       (value.front() == '\'' && value.back() == '\''))) {
    return value.substr(1, value.size() - 2);
  }
  return value;
}

inline std::string escape_regex(std::string value) {
  static constexpr std::string_view special = R"(\.^$|()[]{}*+?)";
  std::string escaped;
  escaped.reserve(value.size() * 2);
  for (char ch : value) {
    if (special.find(ch) != std::string_view::npos) {
      escaped.push_back('\\');
    }
    escaped.push_back(ch);
  }
  return escaped;
}

inline std::string unescape_proto_string(std::string value) {
  std::string out;
  out.reserve(value.size());
  bool escaped = false;
  for (char ch : value) {
    if (escaped) {
      out.push_back(ch);
      escaped = false;
      continue;
    }
    if (ch == '\\') {
      escaped = true;
      continue;
    }
    out.push_back(ch);
  }
  if (escaped) {
    out.push_back('\\');
  }
  return out;
}

inline std::optional<std::string> balanced_block_contents(
    const std::string &source, size_t opening_brace) {
  int depth = 0;
  bool inside_string = false;
  bool escaped = false;
  const auto content_start = opening_brace + 1;

  for (size_t index = opening_brace; index < source.size(); ++index) {
    const char ch = source[index];
    if (inside_string) {
      if (escaped) {
        escaped = false;
      } else if (ch == '\\') {
        escaped = true;
      } else if (ch == '"') {
        inside_string = false;
      }
      continue;
    }

    if (ch == '"') {
      inside_string = true;
    } else if (ch == '{') {
      depth += 1;
    } else if (ch == '}') {
      depth -= 1;
      if (depth == 0) {
        return source.substr(content_start, index - content_start);
      }
    }
  }

  return std::nullopt;
}

inline std::optional<std::string> extract_manifest_block(
    const std::string &source) {
  static const std::regex re(
      R"(option\s*\(\s*holons\.v1\.manifest\s*\)\s*=\s*\{)");
  std::smatch match;
  if (!std::regex_search(source, match, re)) {
    return std::nullopt;
  }
  auto brace = source.find('{', static_cast<size_t>(match.position()));
  if (brace == std::string::npos) {
    return std::nullopt;
  }
  return balanced_block_contents(source, brace);
}

inline std::optional<std::string> extract_proto_block(
    const std::string &name, const std::string &source) {
  const std::regex re("\\b" + escape_regex(name) + R"(\s*:\s*\{)");
  std::smatch match;
  if (!std::regex_search(source, match, re)) {
    return std::nullopt;
  }
  auto brace = source.find('{', static_cast<size_t>(match.position()));
  if (brace == std::string::npos) {
    return std::nullopt;
  }
  return balanced_block_contents(source, brace);
}

inline std::string proto_field_value(const std::string &source,
                                     const std::string &name) {
  const std::regex quoted_re("\\b" + escape_regex(name) +
                             R"re(\s*:\s*"((?:[^"\\]|\\.)*)")re");
  std::smatch quoted_match;
  if (std::regex_search(source, quoted_match, quoted_re)) {
    return unescape_proto_string(quoted_match[1].str());
  }

  const std::regex bare_re("\\b" + escape_regex(name) +
                           R"(\s*:\s*([^\s,\]\}]+))");
  std::smatch bare_match;
  if (std::regex_search(source, bare_match, bare_re)) {
    return bare_match[1].str();
  }
  return "";
}

struct ResolvedHolonManifest {
  HolonIdentity identity;
  HolonManifest manifest;
};

inline ResolvedHolonManifest parse_resolved_manifest(
    const std::string &path) {
  std::ifstream file(path);
  if (!file.is_open()) {
    throw std::runtime_error("cannot open: " + path);
  }

  const std::string text((std::istreambuf_iterator<char>(file)),
                         std::istreambuf_iterator<char>());
  auto manifest_block = extract_manifest_block(text);
  if (!manifest_block.has_value()) {
    throw std::runtime_error(path +
                             ": missing holons.v1.manifest option in holon.proto");
  }

  const auto identity_block =
      extract_proto_block("identity", *manifest_block).value_or("");
  const auto build_block =
      extract_proto_block("build", *manifest_block).value_or("");
  const auto artifacts_block =
      extract_proto_block("artifacts", *manifest_block).value_or("");

  ResolvedHolonManifest resolved;
  resolved.identity.uuid = proto_field_value(identity_block, "uuid");
  resolved.identity.given_name = proto_field_value(identity_block, "given_name");
  resolved.identity.family_name =
      proto_field_value(identity_block, "family_name");
  resolved.identity.motto = proto_field_value(identity_block, "motto");
  resolved.identity.composer = proto_field_value(identity_block, "composer");
  resolved.identity.clade = proto_field_value(identity_block, "clade");
  resolved.identity.status = proto_field_value(identity_block, "status");
  resolved.identity.born = proto_field_value(identity_block, "born");
  resolved.identity.lang = proto_field_value(*manifest_block, "lang");

  resolved.manifest.identity = resolved.identity;
  resolved.manifest.lang = resolved.identity.lang;
  resolved.manifest.kind = proto_field_value(*manifest_block, "kind");
  resolved.manifest.build.runner = proto_field_value(build_block, "runner");
  if (resolved.manifest.build.runner.empty()) {
    resolved.manifest.build.runner = proto_field_value(*manifest_block, "runner");
  }
  resolved.manifest.build.main = proto_field_value(build_block, "main");
  if (resolved.manifest.build.main.empty()) {
    resolved.manifest.build.main = proto_field_value(*manifest_block, "main");
  }
  resolved.manifest.artifacts.binary =
      proto_field_value(artifacts_block, "binary");
  if (resolved.manifest.artifacts.binary.empty()) {
    resolved.manifest.artifacts.binary =
        proto_field_value(*manifest_block, "binary");
  }
  resolved.manifest.artifacts.primary =
      proto_field_value(artifacts_block, "primary");
  if (resolved.manifest.artifacts.primary.empty()) {
    resolved.manifest.artifacts.primary =
        proto_field_value(*manifest_block, "primary");
  }
  return resolved;
}

inline std::string slug_from_identity(const HolonIdentity &id) {
  auto append_part = [](std::string *out, const std::string &value) {
    auto trimmed = trim_copy(value);
    for (char ch : trimmed) {
      if (ch == '?') {
        continue;
      }
      if (std::isspace(static_cast<unsigned char>(ch))) {
        out->push_back('-');
      } else {
        out->push_back(static_cast<char>(std::tolower(static_cast<unsigned char>(ch))));
      }
    }
    while (!out->empty() && out->back() == '-') {
      out->pop_back();
    }
  };

  std::string slug;
  append_part(&slug, id.given_name);
  if (!slug.empty() && (!id.family_name.empty())) {
    slug.push_back('-');
  }
  append_part(&slug, id.family_name);
  while (!slug.empty() && slug.back() == '-') {
    slug.pop_back();
  }
  return slug;
}

inline std::optional<HolonManifest> parse_manifest(const std::string &path) {
  return parse_resolved_manifest(path).manifest;
}

/// Parse a holon.proto file.
inline HolonIdentity parse_holon(const std::string &path) {
  return parse_resolved_manifest(path).identity;
}

inline std::filesystem::path discover_resolve_root(const std::filesystem::path &root) {
  std::error_code ec;
  auto absolute = std::filesystem::absolute(root, ec);
  if (ec) {
    absolute = root;
    ec.clear();
  }
  auto canonical = std::filesystem::weakly_canonical(absolute, ec);
  return ec ? absolute : canonical;
}

inline std::optional<std::filesystem::path> find_holon_proto(
    const std::filesystem::path &root) {
  std::error_code ec;
  auto resolved = discover_resolve_root(root);
  if (std::filesystem::is_regular_file(resolved, ec)) {
    ec.clear();
    return resolved.filename() == "holon.proto"
               ? std::optional<std::filesystem::path>(resolved)
               : std::nullopt;
  }
  ec.clear();
  if (!std::filesystem::exists(resolved, ec) ||
      !std::filesystem::is_directory(resolved, ec)) {
    ec.clear();
    return std::nullopt;
  }

  auto direct = resolved / "holon.proto";
  if (std::filesystem::is_regular_file(direct, ec)) {
    ec.clear();
    return direct;
  }
  ec.clear();

  auto api_v1 = resolved / "api" / "v1" / "holon.proto";
  if (std::filesystem::is_regular_file(api_v1, ec)) {
    ec.clear();
    return api_v1;
  }
  ec.clear();

  std::vector<std::filesystem::path> candidates;
  std::filesystem::recursive_directory_iterator it(
      resolved, std::filesystem::directory_options::skip_permission_denied, ec);
  std::filesystem::recursive_directory_iterator end;
  for (; it != end; it.increment(ec)) {
    if (ec) {
      ec.clear();
      continue;
    }
    const auto &path = it->path();
    if (!it->is_regular_file(ec)) {
      ec.clear();
      continue;
    }
    if (path.filename() == "holon.proto") {
      candidates.push_back(path);
    }
  }
  if (candidates.empty()) {
    return std::nullopt;
  }
  std::sort(candidates.begin(), candidates.end());
  return candidates.front();
}

inline std::filesystem::path resolve_manifest_path(
    const std::filesystem::path &root) {
  auto resolved = discover_resolve_root(root);
  std::vector<std::filesystem::path> search_roots{resolved};
  if (resolved.filename() == "protos" && resolved.has_parent_path()) {
    search_roots.push_back(resolved.parent_path());
  } else if (resolved.has_parent_path()) {
    search_roots.push_back(resolved.parent_path());
  }

  for (const auto &candidate_root : search_roots) {
    if (auto candidate = find_holon_proto(candidate_root); candidate.has_value()) {
      return *candidate;
    }
  }

  throw std::runtime_error("no holon.proto found near " +
                           resolved.generic_string());
}

inline std::filesystem::path manifest_root(
    const std::filesystem::path &manifest_path) {
  auto manifest_dir = manifest_path.parent_path();
  if (manifest_dir.empty()) {
    return ".";
  }
  static const std::regex version_re(R"(^v[0-9]+(?:[A-Za-z0-9._-]*)?$)");
  const auto version_dir = manifest_dir.filename().string();
  const auto api_dir = manifest_dir.parent_path().filename().string();
  if (std::regex_match(version_dir, version_re) && api_dir == "api") {
    auto holon_root = manifest_dir.parent_path().parent_path();
    if (!holon_root.empty()) {
      return holon_root;
    }
  }
  return manifest_dir;
}

inline bool should_skip_discovery_dir(const std::string &name) {
  return name == ".git" || name == ".op" || name == "node_modules" ||
         name == "vendor" || name == "build" ||
         (!name.empty() && name.front() == '.');
}

inline size_t discovery_depth(const std::filesystem::path &path) {
  if (path.empty() || path == ".") {
    return 0;
  }
  return static_cast<size_t>(std::distance(path.begin(), path.end()));
}

inline void append_or_replace_entry(
    std::vector<HolonEntry> &entries,
    std::unordered_map<std::string, size_t> &index_by_key,
    const HolonEntry &candidate) {
  auto key = candidate.uuid.empty() ? candidate.dir.generic_string() : candidate.uuid;
  auto existing = index_by_key.find(key);
  if (existing != index_by_key.end()) {
    auto &current = entries[existing->second];
    if (discovery_depth(candidate.relative_path) <
        discovery_depth(current.relative_path)) {
      current = candidate;
    }
    return;
  }

  index_by_key.emplace(key, entries.size());
  entries.push_back(candidate);
}

inline HolonEntry parse_holon_entry(const std::filesystem::path &manifest_path,
                                    const std::filesystem::path &root,
                                    const std::string &origin) {
  HolonEntry entry;
  entry.identity = parse_holon(manifest_path.string());
  entry.slug = slug_from_identity(entry.identity);
  entry.uuid = entry.identity.uuid;
  entry.dir = discover_resolve_root(manifest_root(manifest_path));
  entry.relative_path = entry.dir.lexically_relative(root);
  if (entry.relative_path.empty()) {
    entry.relative_path = ".";
  }
  entry.origin = origin;
  entry.manifest = parse_manifest(manifest_path.string());
  return entry;
}

inline std::vector<HolonEntry> discover_with_origin(
    const std::filesystem::path &root, const std::string &origin) {
  std::error_code ec;
  auto resolved_root = discover_resolve_root(root);
  if (!std::filesystem::exists(resolved_root, ec) ||
      !std::filesystem::is_directory(resolved_root, ec)) {
    return {};
  }

  std::vector<HolonEntry> entries;
  std::unordered_map<std::string, size_t> index_by_key;
  std::filesystem::recursive_directory_iterator it(
      resolved_root, std::filesystem::directory_options::skip_permission_denied, ec);
  std::filesystem::recursive_directory_iterator end;

  for (; it != end; it.increment(ec)) {
    if (ec) {
      ec.clear();
      continue;
    }

    const auto &path = it->path();
    if (it->is_directory(ec)) {
      if (ec) {
        ec.clear();
        continue;
      }
      if (should_skip_discovery_dir(path.filename().string())) {
        it.disable_recursion_pending();
      }
      continue;
    }

    if (ec || !it->is_regular_file(ec) || path.filename() != "holon.proto") {
      ec.clear();
      continue;
    }

    try {
      append_or_replace_entry(entries, index_by_key,
                              parse_holon_entry(path, resolved_root, origin));
    } catch (const std::exception &) {
      continue;
    }
  }

  std::sort(entries.begin(), entries.end(),
            [](const HolonEntry &left, const HolonEntry &right) {
              auto left_rel = left.relative_path.generic_string();
              auto right_rel = right.relative_path.generic_string();
              if (left_rel != right_rel) {
                return left_rel < right_rel;
              }
              return left.uuid < right.uuid;
            });
  return entries;
}

inline std::filesystem::path oppath() {
  if (const char *configured = std::getenv("OPPATH");
      configured != nullptr && *configured != '\0') {
    return std::filesystem::path(configured);
  }
  if (const char *home = std::getenv("HOME"); home != nullptr && *home != '\0') {
    return std::filesystem::path(home) / ".op";
  }
  return ".op";
}

inline std::filesystem::path opbin() {
  if (const char *configured = std::getenv("OPBIN");
      configured != nullptr && *configured != '\0') {
    return std::filesystem::path(configured);
  }
  return oppath() / "bin";
}

inline std::filesystem::path cache_dir() { return oppath() / "cache"; }

inline std::vector<HolonEntry> discover(const std::filesystem::path &root) {
  return discover_with_origin(root, "local");
}

inline std::vector<HolonEntry> discover_local() {
  return discover(std::filesystem::current_path());
}

inline std::vector<HolonEntry> discover_all() {
  std::vector<HolonEntry> merged;
  std::unordered_map<std::string, size_t> index_by_key;

  for (const auto &[root, origin] :
       std::vector<std::pair<std::filesystem::path, std::string>>{
           {std::filesystem::current_path(), "local"},
           {opbin(), "$OPBIN"},
           {cache_dir(), "cache"},
       }) {
    for (const auto &entry : discover_with_origin(root, origin)) {
      append_or_replace_entry(merged, index_by_key, entry);
    }
  }

  std::sort(merged.begin(), merged.end(),
            [](const HolonEntry &left, const HolonEntry &right) {
              auto left_rel = left.relative_path.generic_string();
              auto right_rel = right.relative_path.generic_string();
              if (left_rel != right_rel) {
                return left_rel < right_rel;
              }
              return left.uuid < right.uuid;
            });
  return merged;
}

inline std::optional<HolonEntry> find_by_slug(const std::string &slug) {
  for (const auto &entry : discover_all()) {
    if (entry.slug == slug) {
      return entry;
    }
  }
  return std::nullopt;
}

inline std::optional<HolonEntry> find_by_uuid(const std::string &prefix) {
  for (const auto &entry : discover_all()) {
    if (entry.uuid.rfind(prefix, 0) == 0) {
      return entry;
    }
  }
  return std::nullopt;
}

struct ConnectOptions {
  int timeout_ms = 5000;
#ifdef _WIN32
  std::string transport = "tcp";
#else
  std::string transport = "stdio";
#endif
  bool start = true;
  std::string port_file;
};

namespace connect_detail {

struct process_handle {
#ifdef _WIN32
  intptr_t pid = 0;
  HANDLE process = nullptr;
  HANDLE thread = nullptr;
  HANDLE stdin_handle = nullptr;
  HANDLE stdout_handle = nullptr;
  HANDLE stderr_handle = nullptr;
  int listener_fd = -1;
  int client_fd = -1;
  std::atomic<bool> closed{false};
  std::mutex client_mutex;
  std::mutex stderr_mutex;
  std::string stderr_capture;
  std::thread accept_thread;
  std::thread stderr_thread;
#else
  pid_t pid = -1;
  int stdin_fd = -1;
  int stdout_fd = -1;
  int stderr_fd = -1;
  int listener_fd = -1;
  int client_fd = -1;
  std::atomic<bool> closed{false};
  std::mutex client_mutex;
  std::mutex stderr_mutex;
  std::string stderr_capture;
  std::thread accept_thread;
  std::thread stderr_thread;
#endif
};

struct startup_result {
  std::string target;
  int direct_fd = -1;
  std::shared_ptr<process_handle> process;
};

struct channel_handle {
  std::shared_ptr<process_handle> process;
  bool ephemeral = false;
  std::string target;
};

inline std::mutex &started_mutex() {
  static std::mutex mu;
  return mu;
}

inline std::map<const grpc::Channel *, channel_handle> &started_channels() {
  static std::map<const grpc::Channel *, channel_handle> started;
  return started;
}

inline std::string lower_copy(std::string value) {
  std::transform(value.begin(), value.end(), value.begin(),
                 [](unsigned char ch) { return static_cast<char>(std::tolower(ch)); });
  return value;
}

inline bool is_direct_target(const std::string &target) {
  return target.find("://") != std::string::npos ||
         target.find(':') != std::string::npos;
}

inline std::string normalize_dial_target(const std::string &target) {
  auto trimmed = trim_copy(target);
  if (trimmed.find("://") == std::string::npos) {
    return trimmed;
  }

  auto parsed = parse_uri(trimmed);
  if (parsed.scheme == "tcp") {
    auto host = parsed.host;
    if (host.empty() || host == "0.0.0.0" || host == "::" || host == "[::]") {
      host = "127.0.0.1";
    }
    return host + ":" + std::to_string(parsed.port);
  }

  if (parsed.scheme == "unix") {
    return parsed.raw;
  }

  return trimmed;
}

inline std::shared_ptr<grpc::Channel> dial_ready(const std::string &target,
                                                 int timeout_ms) {
#if !HOLONS_HAS_GRPCPP
  (void)target;
  (void)timeout_ms;
  throw std::runtime_error("grpc++ headers are required for connect()");
#else
  auto normalized = normalize_dial_target(target);
  auto channel = grpc::CreateChannel(normalized,
                                     grpc::InsecureChannelCredentials());
  if (!channel) {
    throw std::runtime_error("grpc::CreateChannel returned null");
  }
  auto deadline = std::chrono::system_clock::now() +
                  std::chrono::milliseconds(std::max(timeout_ms, 1));
  if (!channel->WaitForConnected(deadline)) {
    throw std::runtime_error("timed out waiting for gRPC readiness: " +
                             normalized);
  }
  return channel;
#endif
}

inline std::shared_ptr<grpc::Channel> dial_ready_from_fd(int fd, int timeout_ms) {
#if HOLONS_HAS_GRPC_FD
  auto channel = grpc::CreateInsecureChannelFromFd("stdio://", fd);
  if (!channel) {
    close_fd(fd, true);
    throw std::runtime_error("grpc::CreateInsecureChannelFromFd returned null");
  }
  auto deadline = std::chrono::system_clock::now() +
                  std::chrono::milliseconds(std::max(timeout_ms, 1));
  if (!channel->WaitForConnected(deadline)) {
    throw std::runtime_error("timed out waiting for gRPC readiness: stdio://");
  }
  return channel;
#else
  close_fd(fd, true);
  (void)timeout_ms;
  throw std::runtime_error("direct stdio gRPC transport is unavailable");
#endif
}

inline void remember(const std::shared_ptr<grpc::Channel> &channel,
                     channel_handle handle) {
  std::lock_guard<std::mutex> lock(started_mutex());
  started_channels()[channel.get()] = std::move(handle);
}

inline std::filesystem::path default_port_file_path(const std::string &slug) {
  return std::filesystem::current_path() / ".op" / "run" / (slug + ".port");
}

inline void write_port_file(const std::filesystem::path &path,
                            const std::string &uri) {
  std::error_code ec;
  std::filesystem::create_directories(path.parent_path(), ec);
  if (ec) {
    throw std::runtime_error("cannot create port-file directory: " +
                             path.parent_path().string());
  }

  std::ofstream out(path);
  if (!out.is_open()) {
    throw std::runtime_error("cannot write port file: " + path.string());
  }
  out << trim_copy(uri) << "\n";
}

inline std::optional<std::string>
usable_port_file(const std::filesystem::path &path, int timeout_ms) {
  std::ifstream in(path);
  if (!in.is_open()) {
    return std::nullopt;
  }

  std::string target;
  std::getline(in, target);
  target = trim_copy(target);
  if (target.empty()) {
    std::error_code ec;
    std::filesystem::remove(path, ec);
    return std::nullopt;
  }

  int check_timeout = timeout_ms / 4;
  if (check_timeout <= 0) {
    check_timeout = 1000;
  }
  check_timeout = std::min(check_timeout, 1000);

  try {
    auto channel = dial_ready(target, check_timeout);
    (void)channel;
    return target;
  } catch (const std::exception &) {
    std::error_code ec;
    std::filesystem::remove(path, ec);
    return std::nullopt;
  }
}

inline std::string first_uri(const std::string &line) {
  std::istringstream in(line);
  std::string field;
  while (in >> field) {
    while (!field.empty() &&
           std::string("\"'()[]{}.,").find(field.front()) != std::string::npos) {
      field.erase(field.begin());
    }
    while (!field.empty() &&
           std::string("\"'()[]{}.,").find(field.back()) != std::string::npos) {
      field.pop_back();
    }

    if (field.rfind("tcp://", 0) == 0 || field.rfind("unix://", 0) == 0 ||
        field.rfind("ws://", 0) == 0 || field.rfind("wss://", 0) == 0 ||
        field.rfind("stdio://", 0) == 0) {
      return field;
    }
  }
  return "";
}

inline std::optional<std::string> drain_startup_buffer(std::string *buffer,
                                                       std::string *capture,
                                                       bool flush_tail) {
  size_t end = buffer->find('\n');
  while (end != std::string::npos) {
    auto line = buffer->substr(0, end);
    if (!line.empty() && line.back() == '\r') {
      line.pop_back();
    }
    capture->append(line);
    capture->push_back('\n');

    if (auto uri = first_uri(line); !uri.empty()) {
      buffer->erase(0, end + 1);
      return uri;
    }
    buffer->erase(0, end + 1);
    end = buffer->find('\n');
  }

  if (flush_tail && !buffer->empty()) {
    auto line = *buffer;
    if (!line.empty() && line.back() == '\r') {
      line.pop_back();
    }
    capture->append(line);
    if (auto uri = first_uri(line); !uri.empty()) {
      buffer->clear();
      return uri;
    }
    buffer->clear();
  }

  return std::nullopt;
}

inline void join_thread(std::thread *thread) {
  if (thread != nullptr && thread->joinable()) {
    thread->join();
  }
}

inline std::string stderr_text(const std::shared_ptr<process_handle> &process) {
  if (process == nullptr) {
    return "";
  }
  std::lock_guard<std::mutex> lock(process->stderr_mutex);
  return trim_copy(process->stderr_capture);
}

#ifdef _WIN32
inline void close_pipe_handle(HANDLE *handle) {
  if (handle == nullptr) {
    return;
  }
  if (*handle != nullptr && *handle != INVALID_HANDLE_VALUE) {
    ::CloseHandle(*handle);
  }
  *handle = nullptr;
}

inline std::string win32_error_message(const std::string &prefix, DWORD code) {
  return prefix + ": win32 error " + std::to_string(code);
}

inline std::wstring widen_utf8(const std::string &value) {
  if (value.empty()) {
    return {};
  }
  int size =
      ::MultiByteToWideChar(CP_UTF8, 0, value.c_str(), -1, nullptr, 0);
  if (size <= 0) {
    throw std::runtime_error("failed to convert UTF-8 to UTF-16");
  }
  std::wstring wide(static_cast<size_t>(size), L'\0');
  if (::MultiByteToWideChar(CP_UTF8, 0, value.c_str(), -1, wide.data(),
                            size) <= 0) {
    throw std::runtime_error("failed to convert UTF-8 to UTF-16");
  }
  wide.resize(static_cast<size_t>(size - 1));
  return wide;
}

inline std::wstring quote_windows_arg(const std::wstring &arg) {
  if (arg.empty()) {
    return L"\"\"";
  }

  bool needs_quotes = false;
  for (wchar_t ch : arg) {
    if (ch == L' ' || ch == L'\t' || ch == L'"') {
      needs_quotes = true;
      break;
    }
  }
  if (!needs_quotes) {
    return arg;
  }

  std::wstring out;
  out.push_back(L'"');
  size_t backslashes = 0;
  for (wchar_t ch : arg) {
    if (ch == L'\\') {
      ++backslashes;
      continue;
    }
    if (ch == L'"') {
      out.append(backslashes * 2 + 1, L'\\');
      out.push_back(L'"');
      backslashes = 0;
      continue;
    }
    out.append(backslashes, L'\\');
    backslashes = 0;
    out.push_back(ch);
  }
  out.append(backslashes * 2, L'\\');
  out.push_back(L'"');
  return out;
}

inline std::wstring build_command_line(const std::string &binary_path,
                                       const std::vector<std::string> &args) {
  std::wstring command = quote_windows_arg(widen_utf8(binary_path));
  for (const auto &arg : args) {
    command.push_back(L' ');
    command += quote_windows_arg(widen_utf8(arg));
  }
  return command;
}

inline void create_pipe_pair(HANDLE *read_handle, HANDLE *write_handle) {
  SECURITY_ATTRIBUTES attrs{};
  attrs.nLength = sizeof(attrs);
  attrs.bInheritHandle = TRUE;
  attrs.lpSecurityDescriptor = nullptr;

  if (!::CreatePipe(read_handle, write_handle, &attrs, 0)) {
    throw std::runtime_error(
        win32_error_message("CreatePipe failed", ::GetLastError()));
  }
}

inline void set_no_inherit(HANDLE handle) {
  if (handle != nullptr && handle != INVALID_HANDLE_VALUE &&
      !::SetHandleInformation(handle, HANDLE_FLAG_INHERIT, 0)) {
    throw std::runtime_error(
        win32_error_message("SetHandleInformation failed", ::GetLastError()));
  }
}

inline void read_available_pipe(HANDLE handle, std::string *target, bool *closed) {
  if (handle == nullptr || handle == INVALID_HANDLE_VALUE || target == nullptr) {
    return;
  }

  for (;;) {
    DWORD available = 0;
    if (!::PeekNamedPipe(handle, nullptr, 0, nullptr, &available, nullptr)) {
      DWORD error = ::GetLastError();
      if (closed != nullptr &&
          (error == ERROR_BROKEN_PIPE || error == ERROR_PIPE_NOT_CONNECTED)) {
        *closed = true;
      }
      return;
    }
    if (available == 0) {
      return;
    }

    std::array<char, 512> buffer{};
    DWORD to_read =
        std::min<DWORD>(available, static_cast<DWORD>(buffer.size()));
    DWORD read = 0;
    if (!::ReadFile(handle, buffer.data(), to_read, &read, nullptr) ||
        read == 0) {
      DWORD error = ::GetLastError();
      if (closed != nullptr &&
          (error == ERROR_BROKEN_PIPE || error == ERROR_PIPE_NOT_CONNECTED)) {
        *closed = true;
      }
      return;
    }
    target->append(buffer.data(), static_cast<size_t>(read));
  }
}

inline void drain_handle(HANDLE handle, std::string *target) {
  if (handle == nullptr || handle == INVALID_HANDLE_VALUE || target == nullptr) {
    return;
  }

  std::array<char, 512> buffer{};
  for (;;) {
    DWORD read = 0;
    if (!::ReadFile(handle, buffer.data(), static_cast<DWORD>(buffer.size()),
                    &read, nullptr) ||
        read == 0) {
      return;
    }
    target->append(buffer.data(), static_cast<size_t>(read));
  }
}

inline void relay_socket_to_handle(int read_fd, HANDLE write_handle) {
  std::array<char, 16 * 1024> buffer{};
  for (;;) {
    int received = ::recv(static_cast<SOCKET>(read_fd), buffer.data(),
                          static_cast<int>(buffer.size()), 0);
    if (received <= 0) {
      return;
    }

    int offset = 0;
    while (offset < received) {
      DWORD written = 0;
      if (!::WriteFile(write_handle, buffer.data() + offset,
                       static_cast<DWORD>(received - offset), &written,
                       nullptr) ||
          written == 0) {
        return;
      }
      offset += static_cast<int>(written);
    }
  }
}

inline void relay_handle_to_socket(HANDLE read_handle, int write_fd) {
  std::array<char, 16 * 1024> buffer{};
  for (;;) {
    DWORD read = 0;
    if (!::ReadFile(read_handle, buffer.data(), static_cast<DWORD>(buffer.size()),
                    &read, nullptr) ||
        read == 0) {
      return;
    }

    DWORD offset = 0;
    while (offset < read) {
      int written =
          ::send(static_cast<SOCKET>(write_fd), buffer.data() + offset,
                 static_cast<int>(read - offset), 0);
      if (written <= 0) {
        return;
      }
      offset += static_cast<DWORD>(written);
    }
  }
}

inline int create_loopback_listener(std::string *uri) {
  detail::ensure_winsock();

  SOCKET socket_fd = ::socket(AF_INET, SOCK_STREAM, 0);
  if (socket_fd == INVALID_SOCKET) {
    throw std::runtime_error("socket() failed: " + last_socket_error());
  }

  int one = 1;
  (void)::setsockopt(socket_fd, SOL_SOCKET, SO_REUSEADDR,
                     reinterpret_cast<const char *>(&one), sizeof(one));

  sockaddr_in addr{};
  addr.sin_family = AF_INET;
  addr.sin_port = htons(0);
  if (::inet_pton(AF_INET, "127.0.0.1", &addr.sin_addr) != 1) {
    ::closesocket(socket_fd);
    throw std::runtime_error("inet_pton() failed for loopback listener");
  }
  if (::bind(socket_fd, reinterpret_cast<sockaddr *>(&addr), sizeof(addr)) !=
      0) {
    auto message = last_socket_error();
    ::closesocket(socket_fd);
    throw std::runtime_error("bind(loopback) failed: " + message);
  }
  if (::listen(socket_fd, 1) != 0) {
    auto message = last_socket_error();
    ::closesocket(socket_fd);
    throw std::runtime_error("listen(loopback) failed: " + message);
  }

  int len = sizeof(addr);
  if (::getsockname(socket_fd, reinterpret_cast<sockaddr *>(&addr), &len) != 0) {
    auto message = last_socket_error();
    ::closesocket(socket_fd);
    throw std::runtime_error("getsockname(loopback) failed: " + message);
  }

  if (uri != nullptr) {
    *uri = "tcp://127.0.0.1:" + std::to_string(ntohs(addr.sin_port));
  }
  return static_cast<int>(socket_fd);
}

inline std::shared_ptr<process_handle>
spawn_child_process(const std::string &binary_path,
                    const std::vector<std::string> &args, HANDLE stdin_handle,
                    HANDLE stdout_handle, HANDLE stderr_handle) {
  STARTUPINFOW startup{};
  startup.cb = sizeof(startup);
  startup.dwFlags = STARTF_USESTDHANDLES;
  startup.hStdInput =
      stdin_handle != nullptr ? stdin_handle : ::GetStdHandle(STD_INPUT_HANDLE);
  startup.hStdOutput = stdout_handle != nullptr
                           ? stdout_handle
                           : ::GetStdHandle(STD_OUTPUT_HANDLE);
  startup.hStdError = stderr_handle != nullptr
                          ? stderr_handle
                          : ::GetStdHandle(STD_ERROR_HANDLE);

  PROCESS_INFORMATION info{};
  auto application = widen_utf8(binary_path);
  auto command = build_command_line(binary_path, args);
  std::vector<wchar_t> command_buffer(command.begin(), command.end());
  command_buffer.push_back(L'\0');

  if (!::CreateProcessW(application.c_str(), command_buffer.data(), nullptr,
                        nullptr, TRUE, CREATE_NEW_PROCESS_GROUP, nullptr,
                        nullptr, &startup, &info)) {
    throw std::runtime_error(
        win32_error_message("CreateProcessW failed", ::GetLastError()));
  }

  auto process = std::make_shared<process_handle>();
  process->pid = static_cast<intptr_t>(info.dwProcessId);
  process->process = info.hProcess;
  process->thread = info.hThread;
  return process;
}

inline std::string resolve_binary_path(const HolonEntry &entry) {
  if (!entry.manifest.has_value()) {
    throw std::runtime_error("holon \"" + entry.slug + "\" has no manifest");
  }

  auto name = trim_copy(entry.manifest->artifacts.binary);
  auto is_file = [](const std::filesystem::path &path) {
    std::error_code ec;
    return std::filesystem::exists(path, ec) &&
           std::filesystem::is_regular_file(path, ec);
  };

  if (name.empty()) {
    auto bin_dir = entry.dir / ".op" / "build" / "bin";
    std::error_code ec;
    if (std::filesystem::exists(bin_dir, ec) &&
        std::filesystem::is_directory(bin_dir, ec)) {
      for (const auto &candidate : std::filesystem::directory_iterator(bin_dir, ec)) {
        if (ec) {
          break;
        }
        if (candidate.is_regular_file(ec)) {
          return candidate.path().string();
        }
      }
    }
    throw std::runtime_error("holon \"" + entry.slug + "\" has no artifacts.binary");
  }

  std::filesystem::path direct(name);
  if (direct.is_absolute() && is_file(direct)) {
    return direct.string();
  }

  auto built = entry.dir / ".op" / "build" / "bin" / direct.filename();
  if (is_file(built)) {
    return built.string();
  }

  if (const char *path_env = std::getenv("PATH");
      path_env != nullptr && *path_env != '\0') {
    std::istringstream parts(path_env);
    std::string part;
    while (std::getline(parts, part, ';')) {
      if (part.empty()) {
        continue;
      }
      auto candidate = std::filesystem::path(part) / direct.filename();
      if (is_file(candidate)) {
        return candidate.string();
      }
    }
  }

  throw std::runtime_error("built binary not found for holon \"" + entry.slug +
                           "\"");
}

inline bool process_alive(intptr_t pid, HANDLE process) {
  if (pid <= 0 || process == nullptr) {
    return false;
  }
  return ::WaitForSingleObject(process, 0) == WAIT_TIMEOUT;
}

inline void close_process_io(const std::shared_ptr<process_handle> &process) {
  if (process == nullptr) {
    return;
  }

  process->closed = true;

  if (process->listener_fd >= 0) {
    close_fd(process->listener_fd, true);
    process->listener_fd = -1;
  }
  {
    std::lock_guard<std::mutex> lock(process->client_mutex);
    if (process->client_fd >= 0) {
      close_fd(process->client_fd, true);
      process->client_fd = -1;
    }
  }

  close_pipe_handle(&process->stdin_handle);
  close_pipe_handle(&process->stdout_handle);
  close_pipe_handle(&process->stderr_handle);
}

inline void close_process_handles(const std::shared_ptr<process_handle> &process) {
  if (process == nullptr) {
    return;
  }
  close_pipe_handle(&process->thread);
  close_pipe_handle(&process->process);
  process->pid = 0;
}

inline startup_result start_tcp_holon(const std::string &binary_path,
                                      int timeout_ms) {
  HANDLE stdout_read = nullptr;
  HANDLE stdout_write = nullptr;
  HANDLE stderr_read = nullptr;
  HANDLE stderr_write = nullptr;
  HANDLE nul_handle = nullptr;

  create_pipe_pair(&stdout_read, &stdout_write);
  create_pipe_pair(&stderr_read, &stderr_write);
  try {
    set_no_inherit(stdout_read);
    set_no_inherit(stderr_read);
    nul_handle = ::CreateFileW(L"NUL", GENERIC_READ, FILE_SHARE_READ | FILE_SHARE_WRITE,
                               nullptr, OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL,
                               nullptr);
    if (nul_handle == INVALID_HANDLE_VALUE) {
      nul_handle = nullptr;
    }

    auto process = spawn_child_process(binary_path,
                                       {"serve", "--listen",
                                        "tcp://127.0.0.1:0"},
                                       nul_handle, stdout_write, stderr_write);

    close_pipe_handle(&stdout_write);
    close_pipe_handle(&stderr_write);
    close_pipe_handle(&nul_handle);

    std::string stdout_buffer;
    std::string stderr_buffer;
    std::string capture;
    bool stdout_closed = false;
    bool stderr_closed = false;
    auto deadline = std::chrono::steady_clock::now() +
                    std::chrono::milliseconds(std::max(timeout_ms, 1));

    while (std::chrono::steady_clock::now() < deadline) {
      read_available_pipe(stdout_read, &stdout_buffer, &stdout_closed);
      if (auto uri =
              drain_startup_buffer(&stdout_buffer, &capture, false);
          uri.has_value()) {
        close_pipe_handle(&stdout_read);
        process->stderr_handle = stderr_read;
        process->stderr_thread = std::thread([process]() {
          std::string capture_chunk;
          drain_handle(process->stderr_handle, &capture_chunk);
          std::lock_guard<std::mutex> lock(process->stderr_mutex);
          process->stderr_capture.append(capture_chunk);
        });
        return startup_result{*uri, -1, process};
      }

      read_available_pipe(stderr_read, &stderr_buffer, &stderr_closed);
      if (auto uri =
              drain_startup_buffer(&stderr_buffer, &capture, false);
          uri.has_value()) {
        close_pipe_handle(&stdout_read);
        process->stderr_handle = stderr_read;
        process->stderr_thread = std::thread([process]() {
          std::string capture_chunk;
          drain_handle(process->stderr_handle, &capture_chunk);
          std::lock_guard<std::mutex> lock(process->stderr_mutex);
          process->stderr_capture.append(capture_chunk);
        });
        return startup_result{*uri, -1, process};
      }

      if (!process_alive(process->pid, process->process)) {
        process->pid = 0;
        drain_handle(stdout_read, &stdout_buffer);
        drain_handle(stderr_read, &stderr_buffer);
        close_pipe_handle(&stdout_read);
        close_pipe_handle(&stderr_read);
        (void)drain_startup_buffer(&stdout_buffer, &capture, true);
        (void)drain_startup_buffer(&stderr_buffer, &capture, true);
        close_process_handles(process);
        throw std::runtime_error(
            "holon exited before advertising an address: " + trim_copy(capture));
      }

      if (stdout_closed && stderr_closed) {
        break;
      }
      std::this_thread::sleep_for(std::chrono::milliseconds(20));
    }

    ::TerminateProcess(process->process, 1);
    ::WaitForSingleObject(process->process, 2000);
    close_process_handles(process);
    close_pipe_handle(&stdout_read);
    close_pipe_handle(&stderr_read);
    throw std::runtime_error("timed out waiting for holon startup");
  } catch (...) {
    close_pipe_handle(&stdout_read);
    close_pipe_handle(&stdout_write);
    close_pipe_handle(&stderr_read);
    close_pipe_handle(&stderr_write);
    close_pipe_handle(&nul_handle);
    throw;
  }
}

inline startup_result start_stdio_holon(const std::string &binary_path,
                                        int timeout_ms) {
  HANDLE stdin_read = nullptr;
  HANDLE stdin_write = nullptr;
  HANDLE stdout_read = nullptr;
  HANDLE stdout_write = nullptr;
  HANDLE stderr_read = nullptr;
  HANDLE stderr_write = nullptr;
  std::string proxy_uri;
  int listener_fd = -1;

  create_pipe_pair(&stdin_read, &stdin_write);
  create_pipe_pair(&stdout_read, &stdout_write);
  create_pipe_pair(&stderr_read, &stderr_write);
  try {
    set_no_inherit(stdin_write);
    set_no_inherit(stdout_read);
    set_no_inherit(stderr_read);
    listener_fd = create_loopback_listener(&proxy_uri);

    auto process = spawn_child_process(binary_path, {"serve", "--listen", "stdio://"},
                                       stdin_read, stdout_write, stderr_write);

    close_pipe_handle(&stdin_read);
    close_pipe_handle(&stdout_write);
    close_pipe_handle(&stderr_write);

    process->stdin_handle = stdin_write;
    process->stdout_handle = stdout_read;
    process->stderr_handle = stderr_read;
    process->listener_fd = listener_fd;

    process->stderr_thread = std::thread([process]() {
      std::string capture_chunk;
      drain_handle(process->stderr_handle, &capture_chunk);
      std::lock_guard<std::mutex> lock(process->stderr_mutex);
      process->stderr_capture.append(capture_chunk);
    });

    process->accept_thread = std::thread([process]() {
      int accepted = ::accept(process->listener_fd, nullptr, nullptr);
      if (accepted < 0) {
        return;
      }

      {
        std::lock_guard<std::mutex> lock(process->client_mutex);
        if (process->closed) {
          close_fd(accepted, true);
          return;
        }
        process->client_fd = accepted;
      }

      std::thread upstream(
          [process, accepted]() { relay_socket_to_handle(accepted, process->stdin_handle); });
      std::thread downstream(
          [process, accepted]() { relay_handle_to_socket(process->stdout_handle, accepted); });
      join_thread(&upstream);
      join_thread(&downstream);
    });

    auto startup_deadline = std::chrono::steady_clock::now() +
                            std::chrono::milliseconds(
                                std::max(1, std::min(timeout_ms, 200)));
    while (std::chrono::steady_clock::now() < startup_deadline) {
      if (!process_alive(process->pid, process->process)) {
        process->pid = 0;
        auto message = stderr_text(process);
        close_process_io(process);
        join_thread(&process->accept_thread);
        join_thread(&process->stderr_thread);
        close_process_handles(process);
        throw std::runtime_error("holon exited before stdio startup" +
                                 (message.empty() ? std::string()
                                                  : std::string(": ") + message));
      }
      std::this_thread::sleep_for(std::chrono::milliseconds(10));
    }

    return startup_result{proxy_uri, -1, process};
  } catch (...) {
    close_pipe_handle(&stdin_read);
    close_pipe_handle(&stdin_write);
    close_pipe_handle(&stdout_read);
    close_pipe_handle(&stdout_write);
    close_pipe_handle(&stderr_read);
    close_pipe_handle(&stderr_write);
    if (listener_fd >= 0) {
      close_fd(listener_fd, true);
    }
    throw;
  }
}

inline void stop_process(const std::shared_ptr<process_handle> &process) {
  if (process == nullptr) {
    return;
  }
  if (process->pid <= 0 || !process_alive(process->pid, process->process)) {
    close_process_io(process);
    join_thread(&process->accept_thread);
    join_thread(&process->stderr_thread);
    close_process_handles(process);
    return;
  }

  close_process_io(process);

  if (!::GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT,
                                  static_cast<DWORD>(process->pid))) {
    DWORD error = ::GetLastError();
    if (error != ERROR_INVALID_PARAMETER) {
      (void)error;
    }
  }

  DWORD waited = ::WaitForSingleObject(process->process, 2000);
  if (waited == WAIT_TIMEOUT) {
    ::TerminateProcess(process->process, 1);
    (void)::WaitForSingleObject(process->process, 2000);
  }

  join_thread(&process->accept_thread);
  join_thread(&process->stderr_thread);
  close_process_handles(process);
}
#else
inline bool process_alive(pid_t pid) {
  if (pid <= 0) {
    return false;
  }
  return ::kill(pid, 0) == 0 || errno == EPERM;
}

inline void close_pipe_fd(int *fd) {
  if (*fd >= 0) {
    ::close(*fd);
    *fd = -1;
  }
}

inline void drain_fd(int fd, std::string *target) {
  if (fd < 0) {
    return;
  }
  char buffer[512];
  for (;;) {
    auto n = ::read(fd, buffer, sizeof(buffer));
    if (n <= 0) {
      return;
    }
    target->append(buffer, static_cast<size_t>(n));
  }
}

inline std::string resolve_binary_path(const HolonEntry &entry) {
  if (!entry.manifest.has_value()) {
    throw std::runtime_error("holon \"" + entry.slug + "\" has no manifest");
  }

  auto name = trim_copy(entry.manifest->artifacts.binary);
  auto is_file = [](const std::filesystem::path &path) {
    std::error_code ec;
    return std::filesystem::exists(path, ec) &&
           std::filesystem::is_regular_file(path, ec);
  };

  if (name.empty()) {
    auto bin_dir = entry.dir / ".op" / "build" / "bin";
    std::error_code ec;
    if (std::filesystem::exists(bin_dir, ec) &&
        std::filesystem::is_directory(bin_dir, ec)) {
      for (const auto &candidate : std::filesystem::directory_iterator(bin_dir, ec)) {
        if (ec) {
          break;
        }
        if (candidate.is_regular_file(ec) &&
            ::access(candidate.path().c_str(), X_OK) == 0) {
          return candidate.path().string();
        }
      }
    }
    throw std::runtime_error("holon \"" + entry.slug + "\" has no artifacts.binary");
  }

  std::filesystem::path direct(name);
  if (direct.is_absolute() && is_file(direct) && ::access(direct.c_str(), X_OK) == 0) {
    return direct.string();
  }

  auto built = entry.dir / ".op" / "build" / "bin" / direct.filename();
  if (is_file(built) && ::access(built.c_str(), X_OK) == 0) {
    return built.string();
  }

  if (const char *path_env = std::getenv("PATH");
      path_env != nullptr && *path_env != '\0') {
    std::istringstream parts(path_env);
    std::string part;
    while (std::getline(parts, part, ':')) {
      if (part.empty()) {
        continue;
      }
      auto candidate = std::filesystem::path(part) / direct.filename();
      if (is_file(candidate) && ::access(candidate.c_str(), X_OK) == 0) {
        return candidate.string();
      }
    }
  }

  throw std::runtime_error("built binary not found for holon \"" + entry.slug +
                           "\"");
}

inline void close_process_io(const std::shared_ptr<process_handle> &process) {
  if (process == nullptr) {
    return;
  }

  process->closed = true;

  if (process->listener_fd >= 0) {
    close_fd(process->listener_fd, true);
    process->listener_fd = -1;
  }
  {
    std::lock_guard<std::mutex> lock(process->client_mutex);
    if (process->client_fd >= 0) {
      close_fd(process->client_fd, true);
      process->client_fd = -1;
    }
  }
  int stdin_fd = process->stdin_fd;
  int stdout_fd = process->stdout_fd;
  process->stdin_fd = -1;
  process->stdout_fd = -1;
  if (stdin_fd >= 0) {
    close_fd(stdin_fd, false);
  }
  if (stdout_fd >= 0 && stdout_fd != stdin_fd) {
    close_fd(stdout_fd, false);
  }
  if (process->stderr_fd >= 0) {
    close_fd(process->stderr_fd, false);
    process->stderr_fd = -1;
  }
}

inline void relay_fd(int read_fd, int write_fd) {
  ensure_sigpipe_ignored();
  std::array<char, 16 * 1024> buffer{};
  for (;;) {
    auto n = ::read(read_fd, buffer.data(), buffer.size());
    if (n == 0) {
      return;
    }
    if (n < 0) {
      if (errno == EINTR) {
        continue;
      }
      return;
    }

    ssize_t offset = 0;
    while (offset < n) {
      auto written =
          ::write(write_fd, buffer.data() + offset, static_cast<size_t>(n - offset));
      if (written < 0) {
        if (errno == EINTR) {
          continue;
        }
        return;
      }
      offset += written;
    }
  }
}

inline int create_loopback_listener(std::string *uri) {
  int fd = ::socket(AF_INET, SOCK_STREAM, 0);
  if (fd < 0) {
    throw std::runtime_error("socket() failed: " + std::string(std::strerror(errno)));
  }

  int one = 1;
  (void)::setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &one, sizeof(one));

  sockaddr_in addr{};
  addr.sin_family = AF_INET;
  addr.sin_port = htons(0);
  if (::inet_pton(AF_INET, "127.0.0.1", &addr.sin_addr) != 1) {
    close_fd(fd, true);
    throw std::runtime_error("inet_pton() failed for loopback listener");
  }
  if (::bind(fd, reinterpret_cast<sockaddr *>(&addr), sizeof(addr)) != 0) {
    auto message = last_socket_error();
    close_fd(fd, true);
    throw std::runtime_error("bind(loopback) failed: " + message);
  }
  if (::listen(fd, 1) != 0) {
    auto message = last_socket_error();
    close_fd(fd, true);
    throw std::runtime_error("listen(loopback) failed: " + message);
  }

  socklen_t len = sizeof(addr);
  if (::getsockname(fd, reinterpret_cast<sockaddr *>(&addr), &len) != 0) {
    auto message = last_socket_error();
    close_fd(fd, true);
    throw std::runtime_error("getsockname(loopback) failed: " + message);
  }
  if (uri != nullptr) {
    *uri = "tcp://127.0.0.1:" + std::to_string(ntohs(addr.sin_port));
  }
  return fd;
}

inline void stop_process(const std::shared_ptr<process_handle> &process) {
  if (process == nullptr) {
    return;
  }
  if (process->pid <= 0) {
    close_process_io(process);
    join_thread(&process->accept_thread);
    join_thread(&process->stderr_thread);
    return;
  }
  if (!process_alive(process->pid)) {
    close_process_io(process);
    join_thread(&process->accept_thread);
    join_thread(&process->stderr_thread);
    process->pid = -1;
    return;
  }

  close_process_io(process);

  if (::kill(process->pid, SIGTERM) != 0 && errno != ESRCH) {
    throw std::runtime_error("failed to terminate child process");
  }

  int status = 0;
  auto deadline = std::chrono::steady_clock::now() + std::chrono::seconds(2);
  while (std::chrono::steady_clock::now() < deadline) {
    auto waited = ::waitpid(process->pid, &status, WNOHANG);
    if (waited == process->pid || waited < 0) {
      process->pid = -1;
      join_thread(&process->accept_thread);
      join_thread(&process->stderr_thread);
      return;
    }
    std::this_thread::sleep_for(std::chrono::milliseconds(25));
  }

  if (::kill(process->pid, SIGKILL) != 0 && errno != ESRCH) {
    throw std::runtime_error("failed to kill child process");
  }
  (void)::waitpid(process->pid, &status, 0);
  process->pid = -1;
  join_thread(&process->accept_thread);
  join_thread(&process->stderr_thread);
}

inline startup_result start_tcp_holon(const std::string &binary_path,
                                      int timeout_ms) {
  int stdout_pipe[2] = {-1, -1};
  int stderr_pipe[2] = {-1, -1};
  if (::pipe(stdout_pipe) != 0 || ::pipe(stderr_pipe) != 0) {
    close_pipe_fd(&stdout_pipe[0]);
    close_pipe_fd(&stdout_pipe[1]);
    close_pipe_fd(&stderr_pipe[0]);
    close_pipe_fd(&stderr_pipe[1]);
    throw std::runtime_error("pipe() failed");
  }

  auto cleanup_pipes = [&]() {
    close_pipe_fd(&stdout_pipe[0]);
    close_pipe_fd(&stdout_pipe[1]);
    close_pipe_fd(&stderr_pipe[0]);
    close_pipe_fd(&stderr_pipe[1]);
  };

  auto pid = ::fork();
  if (pid < 0) {
    cleanup_pipes();
    throw std::runtime_error("fork() failed");
  }

  if (pid == 0) {
    ::dup2(stdout_pipe[1], STDOUT_FILENO);
    ::dup2(stderr_pipe[1], STDERR_FILENO);
    cleanup_pipes();

    std::array<char *, 5> argv{
        const_cast<char *>(binary_path.c_str()), const_cast<char *>("serve"),
        const_cast<char *>("--listen"),
        const_cast<char *>("tcp://127.0.0.1:0"), nullptr};
    ::execv(binary_path.c_str(), argv.data());
    std::perror("execv");
    ::_exit(127);
  }

  close_pipe_fd(&stdout_pipe[1]);
  close_pipe_fd(&stderr_pipe[1]);

  auto process = std::make_shared<process_handle>();
  process->pid = pid;

  std::string stdout_buffer;
  std::string stderr_buffer;
  std::string capture;
  auto deadline = std::chrono::steady_clock::now() +
                  std::chrono::milliseconds(std::max(timeout_ms, 1));

  while (std::chrono::steady_clock::now() < deadline) {
    if (auto uri = drain_startup_buffer(&stdout_buffer, &capture, false);
        uri.has_value()) {
      close_pipe_fd(&stdout_pipe[0]);
      close_pipe_fd(&stderr_pipe[0]);
      return startup_result{*uri, -1, process};
    }
    if (auto uri = drain_startup_buffer(&stderr_buffer, &capture, false);
        uri.has_value()) {
      close_pipe_fd(&stdout_pipe[0]);
      close_pipe_fd(&stderr_pipe[0]);
      return startup_result{*uri, -1, process};
    }

    int status = 0;
    auto waited = ::waitpid(pid, &status, WNOHANG);
    if (waited == pid) {
      process->pid = -1;
      drain_fd(stdout_pipe[0], &stdout_buffer);
      drain_fd(stderr_pipe[0], &stderr_buffer);
      close_pipe_fd(&stdout_pipe[0]);
      close_pipe_fd(&stderr_pipe[0]);
      (void)drain_startup_buffer(&stdout_buffer, &capture, true);
      (void)drain_startup_buffer(&stderr_buffer, &capture, true);
      throw std::runtime_error(
          "holon exited before advertising an address: " + trim_copy(capture));
    }

    pollfd fds[2]{};
    nfds_t nfds = 0;
    if (stdout_pipe[0] >= 0) {
      fds[nfds].fd = stdout_pipe[0];
      fds[nfds].events = POLLIN;
      ++nfds;
    }
    if (stderr_pipe[0] >= 0) {
      fds[nfds].fd = stderr_pipe[0];
      fds[nfds].events = POLLIN;
      ++nfds;
    }

    auto remaining = std::chrono::duration_cast<std::chrono::milliseconds>(
                         deadline - std::chrono::steady_clock::now())
                         .count();
    int poll_timeout = static_cast<int>(std::max<int64_t>(1, std::min<int64_t>(100, remaining)));
    if (nfds == 0) {
      std::this_thread::sleep_for(std::chrono::milliseconds(poll_timeout));
      continue;
    }

    int rc = ::poll(fds, nfds, poll_timeout);
    if (rc < 0 && errno != EINTR) {
      stop_process(process);
      close_pipe_fd(&stdout_pipe[0]);
      close_pipe_fd(&stderr_pipe[0]);
      throw std::runtime_error("poll() failed while waiting for startup");
    }
    if (rc <= 0) {
      continue;
    }

    char buffer[512];
    for (nfds_t i = 0; i < nfds; ++i) {
      if ((fds[i].revents & (POLLIN | POLLHUP)) == 0) {
        continue;
      }
      int fd = fds[i].fd;
      auto *target = fd == stdout_pipe[0] ? &stdout_buffer : &stderr_buffer;
      auto n = ::read(fd, buffer, sizeof(buffer));
      if (n > 0) {
        target->append(buffer, static_cast<size_t>(n));
      } else if (n == 0) {
        if (fd == stdout_pipe[0]) {
          close_pipe_fd(&stdout_pipe[0]);
        } else {
          close_pipe_fd(&stderr_pipe[0]);
        }
      }
    }
  }

  stop_process(process);
  close_pipe_fd(&stdout_pipe[0]);
  close_pipe_fd(&stderr_pipe[0]);
  throw std::runtime_error("timed out waiting for holon startup");
}

inline startup_result start_stdio_holon(const std::string &binary_path,
                                        int timeout_ms) {
#if HOLONS_HAS_GRPC_FD
  int transport_pair[2] = {-1, -1};
  int stderr_pipe[2] = {-1, -1};
  int listener_fd = -1;
  std::string proxy_uri;
#ifdef _WIN32
  if (win_socketpair(transport_pair) != 0 || ::pipe(stderr_pipe) != 0) {
#else
  if (::socketpair(AF_UNIX, SOCK_STREAM, 0, transport_pair) != 0 ||
      ::pipe(stderr_pipe) != 0) {
#endif
    close_pipe_fd(&stderr_pipe[0]);
    close_pipe_fd(&stderr_pipe[1]);
    if (transport_pair[0] >= 0) {
      close_fd(transport_pair[0], true);
    }
    if (transport_pair[1] >= 0) {
      close_fd(transport_pair[1], true);
    }
    throw std::runtime_error("stdio transport setup failed");
  }

  try {
    listener_fd = create_loopback_listener(&proxy_uri);
  } catch (const std::exception &) {
    close_pipe_fd(&stderr_pipe[0]);
    close_pipe_fd(&stderr_pipe[1]);
    if (transport_pair[0] >= 0) {
      close_fd(transport_pair[0], true);
    }
    if (transport_pair[1] >= 0) {
      close_fd(transport_pair[1], true);
    }
    throw;
  }

  auto cleanup_fds = [&]() {
    close_pipe_fd(&stderr_pipe[0]);
    close_pipe_fd(&stderr_pipe[1]);
    if (transport_pair[0] >= 0) {
      close_fd(transport_pair[0], true);
      transport_pair[0] = -1;
    }
    if (transport_pair[1] >= 0) {
      close_fd(transport_pair[1], true);
      transport_pair[1] = -1;
    }
    if (listener_fd >= 0) {
      close_fd(listener_fd, true);
      listener_fd = -1;
    }
  };

  auto pid = ::fork();
  if (pid < 0) {
    cleanup_fds();
    throw std::runtime_error("fork() failed");
  }

  if (pid == 0) {
    ::dup2(transport_pair[1], STDIN_FILENO);
    ::dup2(transport_pair[1], STDOUT_FILENO);
    ::dup2(stderr_pipe[1], STDERR_FILENO);
    cleanup_fds();

    std::array<char *, 5> argv{
        const_cast<char *>(binary_path.c_str()), const_cast<char *>("serve"),
        const_cast<char *>("--listen"), const_cast<char *>("stdio://"),
        nullptr};
    ::execv(binary_path.c_str(), argv.data());
    std::perror("execv");
    ::_exit(127);
  }

  close_fd(transport_pair[1], true);
  close_pipe_fd(&stderr_pipe[1]);

  auto process = std::make_shared<process_handle>();
  process->pid = pid;
  process->stdin_fd = transport_pair[0];
  process->stdout_fd = transport_pair[0];
  process->stderr_fd = stderr_pipe[0];
  process->listener_fd = listener_fd;
  process->stderr_thread = std::thread([process]() {
    std::array<char, 4096> buffer{};
    for (;;) {
      auto n = ::read(process->stderr_fd, buffer.data(), buffer.size());
      if (n == 0) {
        return;
      }
      if (n < 0) {
        if (errno == EINTR) {
          continue;
        }
        return;
      }
      std::lock_guard<std::mutex> lock(process->stderr_mutex);
      process->stderr_capture.append(buffer.data(), static_cast<size_t>(n));
    }
  });

  process->accept_thread = std::thread([process]() {
    int accepted = ::accept(process->listener_fd, nullptr, nullptr);
    if (accepted < 0) {
      return;
    }

    {
      std::lock_guard<std::mutex> lock(process->client_mutex);
      if (process->closed) {
        close_fd(accepted, true);
        return;
      }
      process->client_fd = accepted;
    }

    std::thread upstream([process, accepted]() {
      relay_fd(accepted, process->stdin_fd);
    });
    std::thread downstream([process, accepted]() {
      relay_fd(process->stdout_fd, accepted);
    });
    join_thread(&upstream);
    join_thread(&downstream);
  });

  auto startup_deadline = std::chrono::steady_clock::now() +
                          std::chrono::milliseconds(
                              std::max(1, std::min(timeout_ms, 200)));
  while (std::chrono::steady_clock::now() < startup_deadline) {
    int status = 0;
    auto waited = ::waitpid(pid, &status, WNOHANG);
    if (waited == pid) {
      process->pid = -1;
      auto message = stderr_text(process);
      close_process_io(process);
      join_thread(&process->accept_thread);
      join_thread(&process->stderr_thread);
      throw std::runtime_error("holon exited before stdio startup" +
                               (message.empty() ? std::string()
                                                : std::string(": ") + message));
    }
    std::this_thread::sleep_for(std::chrono::milliseconds(10));
  }

  return startup_result{proxy_uri, -1, process};
#else
  int stdin_pipe[2] = {-1, -1};
  int stdout_pipe[2] = {-1, -1};
  int stderr_pipe[2] = {-1, -1};
  std::string proxy_uri;
  int listener_fd = -1;

  if (::pipe(stdin_pipe) != 0 || ::pipe(stdout_pipe) != 0 || ::pipe(stderr_pipe) != 0) {
    close_pipe_fd(&stdin_pipe[0]);
    close_pipe_fd(&stdin_pipe[1]);
    close_pipe_fd(&stdout_pipe[0]);
    close_pipe_fd(&stdout_pipe[1]);
    close_pipe_fd(&stderr_pipe[0]);
    close_pipe_fd(&stderr_pipe[1]);
    throw std::runtime_error("pipe() failed");
  }

  try {
    listener_fd = create_loopback_listener(&proxy_uri);
  } catch (const std::exception &) {
    close_pipe_fd(&stdin_pipe[0]);
    close_pipe_fd(&stdin_pipe[1]);
    close_pipe_fd(&stdout_pipe[0]);
    close_pipe_fd(&stdout_pipe[1]);
    close_pipe_fd(&stderr_pipe[0]);
    close_pipe_fd(&stderr_pipe[1]);
    throw;
  }

  auto cleanup_fds = [&]() {
    close_pipe_fd(&stdin_pipe[0]);
    close_pipe_fd(&stdin_pipe[1]);
    close_pipe_fd(&stdout_pipe[0]);
    close_pipe_fd(&stdout_pipe[1]);
    close_pipe_fd(&stderr_pipe[0]);
    close_pipe_fd(&stderr_pipe[1]);
    if (listener_fd >= 0) {
      close_fd(listener_fd, true);
      listener_fd = -1;
    }
  };

  auto pid = ::fork();
  if (pid < 0) {
    cleanup_fds();
    throw std::runtime_error("fork() failed");
  }

  if (pid == 0) {
    ::dup2(stdin_pipe[0], STDIN_FILENO);
    ::dup2(stdout_pipe[1], STDOUT_FILENO);
    ::dup2(stderr_pipe[1], STDERR_FILENO);
    cleanup_fds();

    std::array<char *, 5> argv{
        const_cast<char *>(binary_path.c_str()), const_cast<char *>("serve"),
        const_cast<char *>("--listen"), const_cast<char *>("stdio://"),
        nullptr};
    ::execv(binary_path.c_str(), argv.data());
    std::perror("execv");
    ::_exit(127);
  }

  close_pipe_fd(&stdin_pipe[0]);
  close_pipe_fd(&stdout_pipe[1]);
  close_pipe_fd(&stderr_pipe[1]);

  auto process = std::make_shared<process_handle>();
  process->pid = pid;
  process->stdin_fd = stdin_pipe[1];
  process->stdout_fd = stdout_pipe[0];
  process->stderr_fd = stderr_pipe[0];
  process->listener_fd = listener_fd;

  process->stderr_thread = std::thread([process]() {
    std::array<char, 4096> buffer{};
    for (;;) {
      auto n = ::read(process->stderr_fd, buffer.data(), buffer.size());
      if (n == 0) {
        return;
      }
      if (n < 0) {
        if (errno == EINTR) {
          continue;
        }
        return;
      }
      std::lock_guard<std::mutex> lock(process->stderr_mutex);
      process->stderr_capture.append(buffer.data(), static_cast<size_t>(n));
    }
  });

  process->accept_thread = std::thread([process]() {
    int accepted = ::accept(process->listener_fd, nullptr, nullptr);
    if (accepted < 0) {
      return;
    }

    {
      std::lock_guard<std::mutex> lock(process->client_mutex);
      if (process->closed) {
        close_fd(accepted, true);
        return;
      }
      process->client_fd = accepted;
    }

    std::thread upstream([process, accepted]() { relay_fd(accepted, process->stdin_fd); });
    std::thread downstream([process, accepted]() { relay_fd(process->stdout_fd, accepted); });
    join_thread(&upstream);
    join_thread(&downstream);
  });

  auto startup_deadline = std::chrono::steady_clock::now() +
                          std::chrono::milliseconds(std::max(1, std::min(timeout_ms, 200)));
  while (std::chrono::steady_clock::now() < startup_deadline) {
    int status = 0;
    auto waited = ::waitpid(pid, &status, WNOHANG);
    if (waited == pid) {
      process->pid = -1;
      auto message = stderr_text(process);
      close_process_io(process);
      join_thread(&process->accept_thread);
      join_thread(&process->stderr_thread);
      throw std::runtime_error("holon exited before stdio startup" +
                               (message.empty() ? std::string()
                                                : std::string(": ") + message));
    }
    std::this_thread::sleep_for(std::chrono::milliseconds(10));
  }

  return startup_result{proxy_uri, -1, process};
#endif
}
#endif

} // namespace connect_detail

inline std::shared_ptr<grpc::Channel>
connect_with_mode(const std::string &target, const ConnectOptions &input_opts,
                  bool ephemeral) {
  auto trimmed = trim_copy(target);
  if (trimmed.empty()) {
    throw std::invalid_argument("target is required");
  }

  ConnectOptions opts = input_opts;
  if (opts.timeout_ms <= 0) {
    opts.timeout_ms = 5000;
  }

  auto requested_transport = connect_detail::lower_copy(trim_copy(opts.transport));
  if (requested_transport.empty()) {
    requested_transport = "stdio";
  }
  if (requested_transport != "stdio" && requested_transport != "tcp") {
    throw std::invalid_argument("unsupported transport: " + opts.transport);
  }
  const bool ephemeral_mode = ephemeral || requested_transport == "stdio";

  if (connect_detail::is_direct_target(trimmed)) {
    try {
      auto channel = connect_detail::dial_ready(trimmed, opts.timeout_ms);
      connect_detail::remember(
          channel, connect_detail::channel_handle{nullptr, false, trimmed});
      return channel;
    } catch (const std::exception &ex) {
      throw std::runtime_error("connect(\"" + trimmed + "\") failed: " +
                               ex.what());
    }
  }

  auto entry = find_by_slug(trimmed);
  if (!entry.has_value()) {
    throw std::runtime_error("holon \"" + trimmed + "\" not found");
  }

  auto port_file = trim_copy(opts.port_file).empty()
                       ? connect_detail::default_port_file_path(entry->slug)
                       : std::filesystem::path(opts.port_file);
  if (auto existing =
          connect_detail::usable_port_file(port_file, opts.timeout_ms);
      existing.has_value()) {
    try {
      auto channel = connect_detail::dial_ready(*existing, opts.timeout_ms);
      connect_detail::remember(
          channel, connect_detail::channel_handle{nullptr, false, *existing});
      return channel;
    } catch (const std::exception &ex) {
      throw std::runtime_error("connect(\"" + trimmed +
                               "\") failed via port file: " + ex.what());
    }
  }

  if (!opts.start) {
    throw std::runtime_error("holon \"" + trimmed + "\" is not running");
  }

  auto binary_path = connect_detail::resolve_binary_path(*entry);
  auto startup = requested_transport == "stdio"
                     ? connect_detail::start_stdio_holon(binary_path, opts.timeout_ms)
                     : connect_detail::start_tcp_holon(binary_path, opts.timeout_ms);

  std::shared_ptr<grpc::Channel> channel;
  try {
    if (startup.direct_fd >= 0) {
      channel =
          connect_detail::dial_ready_from_fd(startup.direct_fd, opts.timeout_ms);
    } else {
      channel = connect_detail::dial_ready(startup.target, opts.timeout_ms);
    }
    if (!ephemeral_mode && requested_transport == "tcp") {
      connect_detail::write_port_file(port_file, startup.target);
    }
  } catch (const std::exception &ex) {
    connect_detail::stop_process(startup.process);
    throw std::runtime_error("connect(\"" + trimmed +
                             "\") failed after startup: " + ex.what());
  }
  connect_detail::remember(
      channel, connect_detail::channel_handle{startup.process, ephemeral_mode,
                                              startup.target});
  return channel;
}

inline std::shared_ptr<grpc::Channel> connect(const std::string &target) {
  return connect_with_mode(target, ConnectOptions{}, true);
}

inline std::shared_ptr<grpc::Channel> connect(const std::string &target,
                                              const ConnectOptions &opts) {
  return connect_with_mode(target, opts, false);
}

inline void disconnect(std::shared_ptr<grpc::Channel> channel) {
  if (!channel) {
    return;
  }

  connect_detail::channel_handle handle;
  bool found = false;
  {
    std::lock_guard<std::mutex> lock(connect_detail::started_mutex());
    auto &started = connect_detail::started_channels();
    auto it = started.find(channel.get());
    if (it != started.end()) {
      handle = it->second;
      started.erase(it);
      found = true;
    }
  }

  if (found && handle.process != nullptr && handle.ephemeral) {
    connect_detail::stop_process(handle.process);
  }
}

inline std::string channel_target(const std::shared_ptr<grpc::Channel> &channel) {
  if (!channel) {
    return {};
  }

  std::lock_guard<std::mutex> lock(connect_detail::started_mutex());
  auto &started = connect_detail::started_channels();
  auto it = started.find(channel.get());
  if (it == started.end()) {
    return {};
  }
  return it->second.target;
}

} // namespace holons
