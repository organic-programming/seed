#pragma once

#include "describe.hpp"
#include "holons.hpp"
#include "observability.hpp"

#include <array>
#include <csignal>
#include <deque>
#include <iomanip>

#if HOLONS_HAS_GRPCPP && __has_include(<grpcpp/ext/proto_server_reflection_plugin.h>)
#include <grpcpp/ext/proto_server_reflection_plugin.h>
#define HOLONS_HAS_GRPC_REFLECTION 1
#else
#define HOLONS_HAS_GRPC_REFLECTION 0
#endif

#if HOLONS_HAS_GRPCPP && __has_include(<grpcpp/health_check_service_interface.h>)
#include <grpcpp/health_check_service_interface.h>
#define HOLONS_HAS_GRPC_HEALTH 1
#else
#define HOLONS_HAS_GRPC_HEALTH 0
#endif

#if HOLONS_HAS_GRPCPP && __has_include("holons/v1/describe.pb.h") &&         \
    __has_include("holons/v1/describe.grpc.pb.h")
#include "holons/v1/describe.grpc.pb.h"
#include "holons/v1/manifest.pb.h"
#define HOLONS_HAS_HOLONMETA_GRPC 1
#else
#define HOLONS_HAS_HOLONMETA_GRPC 0
#endif

#if HOLONS_HAS_GRPCPP && __has_include("holons/v1/observability.grpc.pb.h")
#include "holons/v1/observability.grpc.pb.h"
#define HOLONS_HAS_OBSERVABILITY_GRPC 1
#else
#define HOLONS_HAS_OBSERVABILITY_GRPC 0
#endif

namespace holons::serve {

std::string CurrentTransport();

namespace detail {
extern thread_local std::string current_transport;
void set_current_transport(std::string transport);
void clear_current_transport();
} // namespace detail

struct member_ref {
  std::string slug;
  std::string address;
};

using MemberRef = member_ref;

struct options {
  bool enable_reflection = false;
  bool auto_register_holon_meta = true;
  bool announce = true;
  int graceful_shutdown_timeout_ms = 10000;
  std::string slug;
  std::vector<member_ref> member_endpoints;
};

struct parsed_flags {
  std::vector<std::string> listeners;
  std::vector<member_ref> member_endpoints;
  bool reflect = false;
};

struct bound_listener {
  std::string requested;
  std::string advertised;
};

using register_fn = std::function<void(grpc::ServerBuilder &)>;

inline parsed_flags parse_options(const std::vector<std::string> &args) {
  parsed_flags parsed;
  auto parse_member = [](const std::string &spec) -> member_ref {
    auto eq = spec.find('=');
    if (eq == std::string::npos || eq == 0 || eq + 1 >= spec.size()) {
      throw std::invalid_argument("--member expects <slug>=<address>");
    }
    return member_ref{spec.substr(0, eq), spec.substr(eq + 1)};
  };
  for (size_t i = 0; i < args.size(); ++i) {
    if (args[i] == "--listen" && i + 1 < args.size()) {
      parsed.listeners.push_back(args[i + 1]);
      ++i;
      continue;
    }
    if (args[i] == "--port" && i + 1 < args.size()) {
      parsed.listeners.push_back("tcp://:" + args[i + 1]);
      ++i;
      continue;
    }
    if (args[i] == "--reflect") {
      parsed.reflect = true;
      continue;
    }
    if (args[i] == "--member" && i + 1 < args.size()) {
      parsed.member_endpoints.push_back(parse_member(args[i + 1]));
      ++i;
      continue;
    }
    if (args[i].rfind("--member=", 0) == 0) {
      parsed.member_endpoints.push_back(parse_member(args[i].substr(9)));
      continue;
    }
  }
  if (parsed.listeners.empty()) {
    parsed.listeners.push_back(std::string(kDefaultURI));
  }
  return parsed;
}

inline std::vector<std::string> parse_flags(const std::vector<std::string> &args) {
  return parse_options(args).listeners;
}

inline std::string parse_flag(const std::vector<std::string> &args) {
  return parse_flags(args).front();
}

class server_handle {
public:
  server_handle() = default;

#if HOLONS_HAS_GRPCPP
  server_handle(std::unique_ptr<grpc::Server> server,
                std::vector<bound_listener> listeners,
                std::vector<std::shared_ptr<void>> owned_objects = {},
                bool clears_current_transport = false)
      : server_(std::move(server)),
        listeners_(std::move(listeners)),
        owned_objects_(std::move(owned_objects)),
        clears_current_transport_(clears_current_transport) {}
#endif

  server_handle(server_handle &&) noexcept = default;
  server_handle &operator=(server_handle &&) noexcept = default;
  server_handle(const server_handle &) = delete;
  server_handle &operator=(const server_handle &) = delete;

  explicit operator bool() const {
#if HOLONS_HAS_GRPCPP
    return static_cast<bool>(server_);
#else
    return false;
#endif
  }

  const std::vector<bound_listener> &listeners() const {
    return listeners_;
  }

  void stop(int graceful_shutdown_timeout_ms = 10000) {
#if HOLONS_HAS_GRPCPP
    if (!server_) {
      return;
    }
    auto deadline = std::chrono::system_clock::now() +
                    std::chrono::milliseconds(
                        std::max(graceful_shutdown_timeout_ms, 1));
    server_->Shutdown(deadline);
    if (clears_current_transport_) {
      detail::clear_current_transport();
      clears_current_transport_ = false;
    }
#else
    (void)graceful_shutdown_timeout_ms;
    throw std::runtime_error("grpc++ headers are required for serve()");
#endif
  }

  void wait() {
#if HOLONS_HAS_GRPCPP
    if (server_) {
      server_->Wait();
    }
#else
    throw std::runtime_error("grpc++ headers are required for serve()");
#endif
  }

private:
#if HOLONS_HAS_GRPCPP
  std::unique_ptr<grpc::Server> server_;
#endif
  std::vector<bound_listener> listeners_;
#if HOLONS_HAS_GRPCPP
  std::vector<std::shared_ptr<void>> owned_objects_;
  bool clears_current_transport_ = false;
#endif
};

namespace detail {

inline volatile std::sig_atomic_t &shutdown_requested() {
  static volatile std::sig_atomic_t requested = 0;
  return requested;
}

inline bool debug_parent_watch_enabled() {
  const char *raw = std::getenv("HOLONS_DEBUG_PARENT_WATCH");
  return raw != nullptr && *raw != '\0' && std::strcmp(raw, "0") != 0;
}

template <typename... Args>
inline void debug_parent_watch_log(const char *fmt, Args... args) {
  if (!debug_parent_watch_enabled()) {
    return;
  }
  char message[512];
  if constexpr (sizeof...(args) == 0) {
    std::snprintf(message, sizeof(message), "%s", fmt);
  } else {
    std::snprintf(message, sizeof(message), fmt, args...);
  }
  std::fprintf(stderr, "[holons parent-watch pid=%d] ", static_cast<int>(::getpid()));
  std::fputs(message, stderr);
  std::fputc('\n', stderr);
  std::fflush(stderr);
}

inline void signal_handler(int) { shutdown_requested() = 1; }

inline void request_shutdown(const char *reason, pid_t expected_parent = 0) {
  if (reason != nullptr) {
    debug_parent_watch_log("%s expected_parent=%d", reason,
                           static_cast<int>(expected_parent));
  }
  shutdown_requested() = 1;
}

inline pid_t configured_parent_pid() {
#ifdef _WIN32
  return 0;
#else
  const char *raw = std::getenv("HOLONS_PARENT_PID");
  if (raw == nullptr || *raw == '\0') {
    return 0;
  }
  char *end = nullptr;
  long long value = std::strtoll(raw, &end, 10);
  if (end == raw || (end != nullptr && *end != '\0') || value <= 1) {
    debug_parent_watch_log("ignoring HOLONS_PARENT_PID=%s", raw);
    return 0;
  }
  debug_parent_watch_log("configured HOLONS_PARENT_PID=%lld", value);
  return static_cast<pid_t>(value);
#endif
}

inline bool parent_process_gone(pid_t expected_parent) {
#ifdef _WIN32
  (void)expected_parent;
  return false;
#else
  if (expected_parent <= 1) {
    return false;
  }
  if (::getppid() != expected_parent) {
    return true;
  }
  if (::kill(expected_parent, 0) != 0 && errno == ESRCH) {
    return true;
  }
  return false;
#endif
}

class parent_watch {
public:
  parent_watch() : expected_parent_(configured_parent_pid()) {
#ifdef _WIN32
    expected_parent_ = 0;
#endif
    debug_parent_watch_log("parent watch ctor expected_parent=%d",
                           static_cast<int>(expected_parent_));
    if (expected_parent_ <= 1) {
      return;
    }
    thread_ = std::thread([this]() {
      while (!stop_.load(std::memory_order_relaxed)) {
        if (parent_process_gone(expected_parent_)) {
          request_shutdown("parent watch detected parent exit", expected_parent_);
          return;
        }
        std::this_thread::sleep_for(std::chrono::milliseconds(250));
      }
      debug_parent_watch_log("parent watch thread stopping cleanly");
    });
  }

  ~parent_watch() {
    stop_.store(true, std::memory_order_relaxed);
    if (thread_.joinable()) {
      thread_.join();
    }
  }

private:
  pid_t expected_parent_ = 0;
  std::atomic<bool> stop_{false};
  std::thread thread_;
};

class scoped_signal_handlers {
public:
  scoped_signal_handlers() {
    shutdown_requested() = 0;
    old_int_ = std::signal(SIGINT, signal_handler);
#ifdef SIGTERM
    old_term_ = std::signal(SIGTERM, signal_handler);
#endif
  }

  ~scoped_signal_handlers() {
    std::signal(SIGINT, old_int_);
#ifdef SIGTERM
    std::signal(SIGTERM, old_term_);
#endif
    shutdown_requested() = 0;
  }

private:
  using handler_fn = void (*)(int);
  handler_fn old_int_ = SIG_DFL;
#ifdef SIGTERM
  handler_fn old_term_ = SIG_DFL;
#endif
};

#if HOLONS_HAS_GRPCPP
struct pending_listener {
  std::string requested;
  parsed_uri parsed;
  std::shared_ptr<int> selected_port;
  bool attach_stdio = false;
};

#ifndef _WIN32
class stdio_bridge {
public:
  explicit stdio_bridge(int socket_fd) : socket_fd_(socket_fd) {
    input_fd_ = ::dup(STDIN_FILENO);
    if (input_fd_ < 0) {
      close_fd(socket_fd_, true);
      socket_fd_ = -1;
      throw std::runtime_error("dup(STDIN_FILENO) failed for stdio:// serve");
    }

    output_fd_ = ::dup(STDOUT_FILENO);
    if (output_fd_ < 0) {
      close_fd(input_fd_, false);
      input_fd_ = -1;
      close_fd(socket_fd_, true);
      socket_fd_ = -1;
      throw std::runtime_error("dup(STDOUT_FILENO) failed for stdio:// serve");
    }
  }

  ~stdio_bridge() { stop(); }

  static std::shared_ptr<stdio_bridge> connect_loopback(int port) {
    return std::make_shared<stdio_bridge>(connect_loopback_fd(port));
  }

  void start() {
    upstream_thread_ = std::thread([this]() {
      connect_detail::relay_fd(input_fd_, socket_fd_);
      if (socket_fd_ >= 0) {
        ::shutdown(socket_fd_, SHUT_WR);
      }
    });
    downstream_thread_ = std::thread([this]() {
      connect_detail::relay_fd(socket_fd_, output_fd_);
    });
  }

  void stop() {
    if (stopped_) {
      return;
    }
    stopped_ = true;

    if (socket_fd_ >= 0) {
      ::shutdown(socket_fd_, SHUT_RDWR);
      close_fd(socket_fd_, true);
      socket_fd_ = -1;
    }
    if (input_fd_ >= 0) {
      close_fd(input_fd_, false);
      input_fd_ = -1;
    }
    if (output_fd_ >= 0) {
      close_fd(output_fd_, false);
      output_fd_ = -1;
    }

    connect_detail::join_thread(&upstream_thread_);
    connect_detail::join_thread(&downstream_thread_);
  }

private:
  static int connect_loopback_fd(int port) {
    int fd = ::socket(AF_INET, SOCK_STREAM, 0);
    if (fd < 0) {
      throw std::runtime_error("socket() failed for stdio:// serve bridge: " +
                               std::string(std::strerror(errno)));
    }

    sockaddr_in addr{};
    addr.sin_family = AF_INET;
    addr.sin_port = htons(static_cast<uint16_t>(port));
    if (::inet_pton(AF_INET, "127.0.0.1", &addr.sin_addr) != 1) {
      close_fd(fd, true);
      throw std::runtime_error("inet_pton() failed for stdio:// serve bridge");
    }

    if (::connect(fd, reinterpret_cast<sockaddr *>(&addr), sizeof(addr)) != 0) {
      auto message = last_socket_error();
      close_fd(fd, true);
      throw std::runtime_error("connect(loopback) failed for stdio:// serve bridge: " +
                               message);
    }

    return fd;
  }

  int input_fd_ = -1;
  int output_fd_ = -1;
  int socket_fd_ = -1;
  bool stopped_ = false;
  std::thread upstream_thread_;
  std::thread downstream_thread_;
};
#endif

inline std::string grpc_listen_target(const parsed_uri &parsed) {
  if (parsed.scheme == "tcp") {
    auto host = parsed.host.empty() ? "0.0.0.0" : parsed.host;
    return host + ":" + std::to_string(parsed.port);
  }
  if (parsed.scheme == "unix") {
    return parsed.raw;
  }
  throw std::invalid_argument("unsupported serve transport: " + parsed.raw);
}

inline std::string advertised_listener(const pending_listener &listener) {
  if (listener.parsed.scheme == "tcp") {
    auto host = listener.parsed.host;
    if (host.empty() || host == "0.0.0.0" || host == "::" || host == "[::]") {
      host = "127.0.0.1";
    }
    int port = listener.parsed.port;
    if (listener.selected_port != nullptr && *listener.selected_port > 0) {
      port = *listener.selected_port;
    }
    return "tcp://" + host + ":" + std::to_string(port);
  }
  if (listener.parsed.scheme == "unix") {
    return listener.parsed.raw;
  }
  if (listener.parsed.scheme == "stdio") {
    return "stdio://";
  }
  return listener.requested;
}

inline void maybe_enable_reflection(const options &opts) {
#if HOLONS_HAS_GRPC_REFLECTION
  if (opts.enable_reflection) {
    static std::once_flag once;
    std::call_once(once, []() {
      grpc::reflection::InitProtoReflectionServerBuilderPlugin();
    });
  }
#else
  (void)opts;
#endif
}

inline void maybe_enable_health_check() {
#if HOLONS_HAS_GRPC_HEALTH
  grpc::EnableDefaultHealthCheckService(true);
#endif
}

#if HOLONS_HAS_HOLONMETA_GRPC
inline void fill_enum_value_doc(const describe::enum_value_doc &source,
                                holons::v1::EnumValueDoc *target) {
  target->set_name(source.name);
  target->set_number(source.number);
  target->set_description(source.description);
}

inline void fill_field_doc(const describe::field_doc &source,
                           holons::v1::FieldDoc *target) {
  target->set_name(source.name);
  target->set_type(source.type);
  target->set_number(source.number);
  target->set_description(source.description);
  target->set_label(
      static_cast<holons::v1::FieldLabel>(static_cast<int>(source.label)));
  target->set_map_key_type(source.map_key_type);
  target->set_map_value_type(source.map_value_type);
  target->set_required(source.required);
  target->set_example(source.example);
  for (const auto &nested : source.nested_fields) {
    fill_field_doc(nested, target->add_nested_fields());
  }
  for (const auto &value : source.enum_values) {
    fill_enum_value_doc(value, target->add_enum_values());
  }
}

inline void fill_method_doc(const describe::method_doc &source,
                            holons::v1::MethodDoc *target) {
  target->set_name(source.name);
  target->set_description(source.description);
  target->set_input_type(source.input_type);
  target->set_output_type(source.output_type);
  target->set_client_streaming(source.client_streaming);
  target->set_server_streaming(source.server_streaming);
  target->set_example_input(source.example_input);
  for (const auto &field : source.input_fields) {
    fill_field_doc(field, target->add_input_fields());
  }
  for (const auto &field : source.output_fields) {
    fill_field_doc(field, target->add_output_fields());
  }
}

inline void fill_service_doc(const describe::service_doc &source,
                             holons::v1::ServiceDoc *target) {
  target->set_name(source.name);
  target->set_description(source.description);
  for (const auto &method : source.methods) {
    fill_method_doc(method, target->add_methods());
  }
}

inline void fill_manifest_identity(const HolonIdentity &source,
                                   holons::v1::HolonManifest_Identity *target) {
  target->set_schema("holon/v1");
  target->set_uuid(source.uuid);
  target->set_given_name(source.given_name);
  target->set_family_name(source.family_name);
  target->set_motto(source.motto);
  target->set_composer(source.composer);
  target->set_status(source.status);
  target->set_born(source.born);
}

inline void fill_manifest(const HolonManifest &source,
                          holons::v1::HolonManifest *target) {
  fill_manifest_identity(source.identity, target->mutable_identity());
  target->set_lang(source.lang);
  target->set_kind(source.kind);
  target->mutable_build()->set_runner(source.build.runner);
  target->mutable_build()->set_main(source.build.main);
  target->mutable_artifacts()->set_binary(source.artifacts.binary);
  target->mutable_artifacts()->set_primary(source.artifacts.primary);
}

class holon_meta_service final : public holons::v1::HolonMeta::Service {
public:
  explicit holon_meta_service(const holons::v1::DescribeResponse &response) {
    response_.CopyFrom(response);
  }

  grpc::Status Describe(grpc::ServerContext *,
                        const holons::v1::DescribeRequest *,
                        holons::v1::DescribeResponse *response) override {
    if (response == nullptr) {
      return grpc::Status(grpc::StatusCode::INTERNAL,
                          "Describe response is required");
    }
    response->CopyFrom(response_);
    return grpc::Status();
  }

private:
  holons::v1::DescribeResponse response_;
};

inline std::shared_ptr<grpc::Service> maybe_make_holon_meta_service(
    const options &opts) {
  if (!opts.auto_register_holon_meta) {
    return nullptr;
  }

  auto response = describe::registered_static_response();
  if (!response) {
    throw std::runtime_error(
        std::string(describe::kNoIncodeDescriptionRegistered));
  }

  return std::make_shared<holon_meta_service>(*response);
}
#else
inline std::shared_ptr<grpc::Service> maybe_make_holon_meta_service(
    const options &) {
  return nullptr;
}
#endif
#endif

#if HOLONS_HAS_OBSERVABILITY_GRPC
inline std::chrono::system_clock::time_point since_cutoff(
    const google::protobuf::Duration &value) {
  auto duration = std::chrono::seconds(value.seconds()) +
                  std::chrono::nanoseconds(value.nanos());
  return std::chrono::system_clock::now() -
         std::chrono::duration_cast<std::chrono::system_clock::duration>(duration);
}

inline std::uint64_t unix_nano(std::chrono::system_clock::time_point value) {
  return static_cast<std::uint64_t>(
      std::chrono::duration_cast<std::chrono::nanoseconds>(
          value.time_since_epoch()).count());
}

inline std::chrono::system_clock::time_point from_unix_nano(std::uint64_t value) {
  return std::chrono::system_clock::time_point{
      std::chrono::duration_cast<std::chrono::system_clock::duration>(
          std::chrono::nanoseconds(value))};
}

inline holons::v1::SeverityNumber to_proto_level(observability::Level level) {
  switch (level) {
  case observability::Level::Trace: return holons::v1::SEVERITY_NUMBER_TRACE;
  case observability::Level::Debug: return holons::v1::SEVERITY_NUMBER_DEBUG;
  case observability::Level::Info: return holons::v1::SEVERITY_NUMBER_INFO;
  case observability::Level::Warn: return holons::v1::SEVERITY_NUMBER_WARN;
  case observability::Level::Error: return holons::v1::SEVERITY_NUMBER_ERROR;
  case observability::Level::Fatal: return holons::v1::SEVERITY_NUMBER_FATAL;
  default: return holons::v1::SEVERITY_NUMBER_UNSPECIFIED;
  }
}

inline observability::Level from_proto_level(holons::v1::SeverityNumber level) {
  switch (level) {
  case holons::v1::SEVERITY_NUMBER_TRACE: return observability::Level::Trace;
  case holons::v1::SEVERITY_NUMBER_DEBUG: return observability::Level::Debug;
  case holons::v1::SEVERITY_NUMBER_INFO: return observability::Level::Info;
  case holons::v1::SEVERITY_NUMBER_WARN: return observability::Level::Warn;
  case holons::v1::SEVERITY_NUMBER_ERROR: return observability::Level::Error;
  case holons::v1::SEVERITY_NUMBER_FATAL: return observability::Level::Fatal;
  default: return observability::Level::Unset;
  }
}

inline void set_any_value(holons::v1::AnyValue *target,
                          const observability::AnyValue &value) {
  if (target == nullptr) return;
  std::visit([target](const auto &item) {
    using T = std::decay_t<decltype(item)>;
    if constexpr (std::is_same_v<T, std::int64_t>) target->set_int_value(item);
    else if constexpr (std::is_same_v<T, double>) target->set_double_value(item);
    else if constexpr (std::is_same_v<T, bool>) target->set_bool_value(item);
    else target->set_string_value(item);
  }, value);
}

inline observability::AnyValue from_proto_any_value(const holons::v1::AnyValue &value) {
  switch (value.value_case()) {
  case holons::v1::AnyValue::kIntValue: return static_cast<std::int64_t>(value.int_value());
  case holons::v1::AnyValue::kDoubleValue: return value.double_value();
  case holons::v1::AnyValue::kBoolValue: return value.bool_value();
  case holons::v1::AnyValue::kStringValue: return value.string_value();
  default: return std::string{};
  }
}

inline void append_attrs(const std::vector<observability::Field> &attrs,
                         google::protobuf::RepeatedPtrField<holons::v1::KeyValue> *out) {
  if (out == nullptr) return;
  for (const auto &attr : attrs) {
    auto *kv = out->Add();
    kv->set_key(attr.key);
    set_any_value(kv->mutable_value(), attr.value);
  }
}

inline void append_chain(const std::vector<std::string> &chain,
                         google::protobuf::RepeatedPtrField<std::string> *out) {
  if (out == nullptr) return;
  for (const auto &slug : chain) *out->Add() = slug;
}

inline std::vector<std::string> from_proto_chain(
    const google::protobuf::RepeatedPtrField<std::string> &chain) {
  std::vector<std::string> out;
  out.reserve(static_cast<size_t>(chain.size()));
  for (const auto &slug : chain) out.push_back(slug);
  return out;
}

inline holons::v1::LogRecord to_proto_log(const observability::LogRecord &entry) {
  holons::v1::LogRecord proto;
  proto.set_time_unix_nano(unix_nano(entry.timestamp));
  proto.set_observed_time_unix_nano(unix_nano(entry.timestamp));
  proto.set_severity_number(to_proto_level(entry.level));
  proto.set_severity_text(observability::level_label(entry.level));
  set_any_value(proto.mutable_body(), entry.body);
  append_attrs(entry.attributes, proto.mutable_attributes());
  proto.set_event_name(entry.event_name);
  append_chain(entry.chain, proto.mutable_chain());
  return proto;
}

inline observability::LogRecord from_proto_log(const holons::v1::LogRecord &proto) {
  observability::LogRecord entry;
  entry.timestamp = proto.time_unix_nano() == 0 ? std::chrono::system_clock::now()
                                                : from_unix_nano(proto.time_unix_nano());
  entry.level = from_proto_level(proto.severity_number());
  entry.body = proto.has_body() ? from_proto_any_value(proto.body())
                                : observability::AnyValue{std::string{}};
  for (const auto &attr : proto.attributes()) {
    entry.attributes.push_back({attr.key(), attr.has_value()
      ? from_proto_any_value(attr.value()) : observability::AnyValue{std::string{}}});
  }
  entry.event_name = proto.event_name();
  entry.chain = from_proto_chain(proto.chain());
  return entry;
}

inline bool matches_log_request(const observability::LogRecord &entry,
                                const holons::v1::LogsRequest &request) {
  auto min_level = request.min_severity_number() == holons::v1::SEVERITY_NUMBER_UNSPECIFIED
      ? observability::Level::Info : from_proto_level(request.min_severity_number());
  if (static_cast<int>(entry.level) < static_cast<int>(min_level)) return false;
  if (request.session_ids_size() > 0 &&
      std::find(request.session_ids().begin(), request.session_ids().end(),
                observability::attribute_string(entry, "holons.session_id")) == request.session_ids().end()) return false;
  if (request.rpc_methods_size() > 0 &&
      std::find(request.rpc_methods().begin(), request.rpc_methods().end(),
                observability::attribute_string(entry, "rpc.method")) == request.rpc_methods().end()) return false;
  return true;
}

inline bool matches_event_request(const observability::LogRecord &event,
                                  const holons::v1::EventsRequest &request) {
  if (request.event_names_size() == 0) return true;
  return std::find(request.event_names().begin(), request.event_names().end(),
                   event.event_name) != request.event_names().end();
}

template <typename T> class follow_queue {
public:
  void push(const T &value) {
    {
      std::scoped_lock lk(mu_);
      if (closed_) {
        return;
      }
      queue_.push_back(value);
    }
    cv_.notify_one();
  }

  bool wait_pop(T *value, const std::function<bool()> &cancelled) {
    std::unique_lock<std::mutex> lk(mu_);
    while (queue_.empty() && !closed_) {
      if (cancelled()) {
        return false;
      }
      cv_.wait_for(lk, std::chrono::milliseconds(100));
    }
    if (queue_.empty()) {
      return false;
    }
    *value = queue_.front();
    queue_.pop_front();
    return true;
  }

  void close() {
    {
      std::scoped_lock lk(mu_);
      closed_ = true;
      queue_.clear();
    }
    cv_.notify_all();
  }

private:
  std::mutex mu_;
  std::condition_variable cv_;
  std::deque<T> queue_;
  bool closed_ = false;
};

class observability_grpc_service final
    : public holons::v1::HolonObservability::Service {
public:
  explicit observability_grpc_service(observability::Observability *obs)
      : obs_(obs) {}

  grpc::Status Logs(grpc::ServerContext *context,
                    const holons::v1::LogsRequest *request,
                    grpc::ServerWriter<holons::v1::LogRecord> *writer) override {
    if (obs_ == nullptr || !obs_->enabled(observability::Family::Logs) ||
        !obs_->log_ring) {
      return grpc::Status(grpc::StatusCode::FAILED_PRECONDITION,
                          "logs family is not enabled (OP_OBS)");
    }
    auto cutoff = request != nullptr && request->has_since()
                      ? since_cutoff(request->since())
                      : std::chrono::system_clock::time_point{};
    const bool follow = request != nullptr && request->follow();
    auto queue = std::make_shared<follow_queue<observability::LogRecord>>();
    std::function<void()> unsubscribe;
    std::vector<observability::LogRecord> entries =
        follow ? std::vector<observability::LogRecord>{}
               : (cutoff == std::chrono::system_clock::time_point{}
                      ? obs_->log_ring->drain()
                      : obs_->log_ring->drain_since(cutoff));
    if (follow) {
      auto replay = obs_->log_ring->replay_and_subscribe(
          cutoff, [queue](const observability::LogRecord &entry) {
            queue->push(entry);
          });
      entries = std::move(replay.first);
      unsubscribe = std::move(replay.second);
    }
    for (const auto &entry : entries) {
      if (entry.private_entry) {
        continue;
      }
      if (request != nullptr && !matches_log_request(entry, *request)) {
        continue;
      }
      if (!writer->Write(to_proto_log(entry))) {
        if (unsubscribe) {
          unsubscribe();
        }
        queue->close();
        return grpc::Status::OK;
      }
    }
    if (request == nullptr || !request->follow()) {
      return grpc::Status::OK;
    }

    observability::LogRecord entry;
    while (!context->IsCancelled() &&
           queue->wait_pop(&entry, [context]() { return context->IsCancelled(); })) {
      if (entry.private_entry) {
        continue;
      }
      if (!matches_log_request(entry, *request)) {
        continue;
      }
      if (!writer->Write(to_proto_log(entry))) {
        break;
      }
    }
    if (unsubscribe) {
      unsubscribe();
    }
    queue->close();
    return grpc::Status::OK;
  }

  grpc::Status Metrics(grpc::ServerContext *,
                       const holons::v1::MetricsRequest *request,
                       grpc::ServerWriter<holons::v1::Metric> *writer) override {
    if (obs_ == nullptr || !obs_->enabled(observability::Family::Metrics) ||
        !obs_->registry) {
      return grpc::Status(grpc::StatusCode::FAILED_PRECONDITION,
                          "metrics family is not enabled (OP_OBS)");
    }
    const auto start = unix_nano(obs_->start_wall);
    const auto now = unix_nano(std::chrono::system_clock::now());

    auto matches_name = [request](const std::string &name) {
      if (request == nullptr || request->name_prefixes_size() == 0) {
        return true;
      }
      for (const auto &prefix : request->name_prefixes()) {
        if (name.rfind(prefix, 0) == 0) {
          return true;
        }
      }
      return false;
    };

    for (const auto &counter : obs_->registry->counters()) {
      if (!matches_name(counter.name)) {
        continue;
      }
      holons::v1::Metric metric;
      metric.set_name(counter.name);
      metric.set_description(counter.help);
      auto *sum = metric.mutable_sum();
      sum->set_is_monotonic(true);
      sum->set_aggregation_temporality(
          holons::v1::AGGREGATION_TEMPORALITY_CUMULATIVE);
      auto *point = sum->add_data_points();
      point->set_start_time_unix_nano(start);
      point->set_time_unix_nano(now);
      point->set_as_int(counter.value);
      append_attrs(metric_attributes(counter.labels), point->mutable_attributes());
      if (!writer->Write(metric)) return grpc::Status::OK;
    }
    for (const auto &gauge : obs_->registry->gauges()) {
      if (!matches_name(gauge.name)) {
        continue;
      }
      holons::v1::Metric metric;
      metric.set_name(gauge.name);
      metric.set_description(gauge.help);
      auto *point = metric.mutable_gauge()->add_data_points();
      point->set_start_time_unix_nano(start);
      point->set_time_unix_nano(now);
      point->set_as_double(gauge.value);
      append_attrs(metric_attributes(gauge.labels), point->mutable_attributes());
      if (!writer->Write(metric)) return grpc::Status::OK;
    }
    for (const auto &histogram : obs_->registry->histograms()) {
      if (!matches_name(histogram.name)) {
        continue;
      }
      holons::v1::Metric metric;
      metric.set_name(histogram.name);
      metric.set_description(histogram.help);
      auto *proto_histogram = metric.mutable_histogram();
      proto_histogram->set_aggregation_temporality(
          holons::v1::AGGREGATION_TEMPORALITY_CUMULATIVE);
      auto *point = proto_histogram->add_data_points();
      point->set_start_time_unix_nano(start);
      point->set_time_unix_nano(now);
      point->set_count(static_cast<std::uint64_t>(histogram.value.total));
      point->set_sum(histogram.value.sum);
      if (histogram.value.total > 0) {
        point->set_min(histogram.value.min);
        point->set_max(histogram.value.max);
      }
      std::int64_t previous = 0;
      for (size_t i = 0; i < histogram.value.bounds.size(); ++i) {
        point->add_explicit_bounds(histogram.value.bounds[i]);
        const auto cumulative = i < histogram.value.counts.size()
                                    ? histogram.value.counts[i]
                                    : previous;
        point->add_bucket_counts(static_cast<std::uint64_t>(
            std::max<std::int64_t>(0, cumulative - previous)));
        previous = cumulative;
      }
      point->add_bucket_counts(static_cast<std::uint64_t>(
          std::max<std::int64_t>(0, histogram.value.total - previous)));
      append_attrs(metric_attributes(histogram.labels), point->mutable_attributes());
      if (!writer->Write(metric)) return grpc::Status::OK;
    }
    return grpc::Status::OK;
  }

  grpc::Status Events(grpc::ServerContext *context,
                      const holons::v1::EventsRequest *request,
                      grpc::ServerWriter<holons::v1::LogRecord> *writer) override {
    if (obs_ == nullptr || !obs_->enabled(observability::Family::Events) ||
        !obs_->event_bus) {
      return grpc::Status(grpc::StatusCode::FAILED_PRECONDITION,
                          "events family is not enabled (OP_OBS)");
    }
    auto cutoff = request != nullptr && request->has_since()
                      ? since_cutoff(request->since())
                      : std::chrono::system_clock::time_point{};
    const bool follow = request != nullptr && request->follow();
    auto queue = std::make_shared<follow_queue<observability::LogRecord>>();
    std::function<void()> unsubscribe;
    std::vector<observability::LogRecord> events =
        follow ? std::vector<observability::LogRecord>{}
               : (cutoff == std::chrono::system_clock::time_point{}
                      ? obs_->event_bus->drain()
                      : obs_->event_bus->drain_since(cutoff));
    if (follow) {
      auto replay = obs_->event_bus->replay_and_subscribe(
          cutoff, [queue](const observability::LogRecord &event) {
            queue->push(event);
          });
      events = std::move(replay.first);
      unsubscribe = std::move(replay.second);
    }
    for (const auto &event : events) {
      if (event.private_entry) {
        continue;
      }
      if (request != nullptr && !matches_event_request(event, *request)) {
        continue;
      }
      if (!writer->Write(to_proto_log(event))) {
        if (unsubscribe) {
          unsubscribe();
        }
        queue->close();
        return grpc::Status::OK;
      }
    }
    if (request == nullptr || !request->follow()) {
      return grpc::Status::OK;
    }

    observability::LogRecord event;
    while (!context->IsCancelled() &&
           queue->wait_pop(&event, [context]() { return context->IsCancelled(); })) {
      if (event.private_entry) {
        continue;
      }
      if (!matches_event_request(event, *request)) {
        continue;
      }
      if (!writer->Write(to_proto_log(event))) {
        break;
      }
    }
    if (unsubscribe) {
      unsubscribe();
    }
    queue->close();
    return grpc::Status::OK;
  }

private:
  std::vector<observability::Field>
  metric_attributes(const std::map<std::string, std::string> &labels) const {
    std::vector<observability::Field> attrs{{"holons.slug", obs_->cfg.slug},
                                            {"service.name", obs_->cfg.slug},
                                            {"holons.instance_uid", obs_->cfg.instance_uid},
                                            {"service.instance.id", obs_->cfg.instance_uid},
                                            {"holons.session_id", obs_->cfg.session_id}};
    for (const auto &[key, value] : labels) attrs.emplace_back(key, value);
    return attrs;
  }

  observability::Observability *obs_ = nullptr;
};

struct member_identity {
  std::string slug;
  std::string instance_uid;
};

inline std::string double_to_string(double value) {
  std::ostringstream out;
  out << std::setprecision(17) << value;
  return out.str();
}

inline std::string prom_escape_label(std::string value) {
  std::string out;
  out.reserve(value.size());
  for (char ch : value) {
    if (ch == '\\') out += "\\\\";
    else if (ch == '"') out += "\\\"";
    else if (ch == '\n') out += "\\n";
    else out.push_back(ch);
  }
  return out;
}

inline std::string prom_escape_help(std::string value) {
  std::string out;
  out.reserve(value.size());
  for (char ch : value) {
    if (ch == '\\') out += "\\\\";
    else if (ch == '\n') out += "\\n";
    else out.push_back(ch);
  }
  return out;
}

inline std::string prom_labels(
    const std::map<std::string, std::string> &labels) {
  if (labels.empty()) {
    return {};
  }
  std::string out = "{";
  bool first = true;
  for (const auto &[key, value] : labels) {
    if (!first) {
      out += ",";
    }
    first = false;
    out += key;
    out += "=\"";
    out += prom_escape_label(value);
    out += "\"";
  }
  out += "}";
  return out;
}

inline std::map<std::string, std::string> with_identity_labels(
    const observability::Observability &obs,
    std::map<std::string, std::string> labels) {
  if (!obs.cfg.slug.empty()) {
    labels.emplace("slug", obs.cfg.slug);
  }
  if (!obs.cfg.instance_uid.empty()) {
    labels.emplace("instance_uid", obs.cfg.instance_uid);
  }
  return labels;
}

inline std::string prometheus_text(const observability::Observability &obs) {
  if (!obs.registry) {
    return {};
  }
  std::ostringstream out;
  for (const auto &counter : obs.registry->counters()) {
    if (!counter.help.empty()) {
      out << "# HELP " << counter.name << ' ' << prom_escape_help(counter.help)
          << '\n';
    }
    out << "# TYPE " << counter.name << " counter\n";
    out << counter.name
        << prom_labels(with_identity_labels(obs, counter.labels)) << ' '
        << counter.value << '\n';
  }
  for (const auto &gauge : obs.registry->gauges()) {
    if (!gauge.help.empty()) {
      out << "# HELP " << gauge.name << ' ' << prom_escape_help(gauge.help)
          << '\n';
    }
    out << "# TYPE " << gauge.name << " gauge\n";
    out << gauge.name << prom_labels(with_identity_labels(obs, gauge.labels))
        << ' ' << double_to_string(gauge.value) << '\n';
  }
  for (const auto &histogram : obs.registry->histograms()) {
    if (!histogram.help.empty()) {
      out << "# HELP " << histogram.name << ' '
          << prom_escape_help(histogram.help) << '\n';
    }
    out << "# TYPE " << histogram.name << " histogram\n";
    auto labels = with_identity_labels(obs, histogram.labels);
    for (size_t i = 0; i < histogram.value.bounds.size(); ++i) {
      auto bucket_labels = labels;
      bucket_labels["le"] = double_to_string(histogram.value.bounds[i]);
      out << histogram.name << "_bucket" << prom_labels(bucket_labels) << ' '
          << (i < histogram.value.counts.size() ? histogram.value.counts[i] : 0)
          << '\n';
    }
    auto inf_labels = labels;
    inf_labels["le"] = "+Inf";
    out << histogram.name << "_bucket" << prom_labels(inf_labels) << ' '
        << histogram.value.total << '\n';
    out << histogram.name << "_sum" << prom_labels(labels) << ' '
        << double_to_string(histogram.value.sum) << '\n';
    out << histogram.name << "_count" << prom_labels(labels) << ' '
        << histogram.value.total << '\n';
  }
  return out.str();
}

#ifndef _WIN32
class prometheus_server {
public:
  prometheus_server(observability::Observability *obs, std::string bind)
      : obs_(obs) {
    open_socket(std::move(bind));
    thread_ = std::thread([this]() { run(); });
  }

  ~prometheus_server() { stop(); }

  const std::string &address() const { return address_; }

  void stop() {
    if (stopped_.exchange(true)) {
      return;
    }
    if (fd_ >= 0) {
      ::shutdown(fd_, SHUT_RDWR);
      close_fd(fd_, true);
      fd_ = -1;
    }
    connect_detail::join_thread(&thread_);
  }

private:
  void open_socket(std::string bind) {
    if (bind.empty()) {
      bind = "127.0.0.1:0";
    }
    if (bind.rfind("http://", 0) == 0) {
      bind = bind.substr(7);
      auto slash = bind.find('/');
      if (slash != std::string::npos) {
        bind = bind.substr(0, slash);
      }
    }
    auto colon = bind.rfind(':');
    auto host = colon == std::string::npos ? std::string("127.0.0.1")
                                           : bind.substr(0, colon);
    auto port_raw = colon == std::string::npos ? bind : bind.substr(colon + 1);
    int port = 0;
    try {
      port = std::stoi(port_raw);
    } catch (...) {
      port = 0;
    }
    auto advertised = host;
    if (host.empty() || host == "0.0.0.0" || host == "*") {
      host = "0.0.0.0";
      advertised = "127.0.0.1";
    }
    if (advertised.empty()) {
      advertised = "127.0.0.1";
    }

    fd_ = ::socket(AF_INET, SOCK_STREAM, 0);
    if (fd_ < 0) {
      throw std::runtime_error("socket() failed for Prometheus server: " +
                               last_socket_error());
    }
    int yes = 1;
    setsockopt(fd_, SOL_SOCKET, SO_REUSEADDR, &yes, sizeof(yes));

    sockaddr_in addr{};
    addr.sin_family = AF_INET;
    addr.sin_port = htons(static_cast<uint16_t>(port));
    if (::inet_pton(AF_INET, host.c_str(), &addr.sin_addr) != 1) {
      close_fd(fd_, true);
      fd_ = -1;
      throw std::runtime_error("invalid Prometheus bind address: " + bind);
    }
    if (::bind(fd_, reinterpret_cast<sockaddr *>(&addr), sizeof(addr)) != 0) {
      auto error = last_socket_error();
      close_fd(fd_, true);
      fd_ = -1;
      throw std::runtime_error("bind() failed for Prometheus server: " + error);
    }
    if (::listen(fd_, 16) != 0) {
      auto error = last_socket_error();
      close_fd(fd_, true);
      fd_ = -1;
      throw std::runtime_error("listen() failed for Prometheus server: " + error);
    }

    sockaddr_in actual{};
    socklen_t len = sizeof(actual);
    if (::getsockname(fd_, reinterpret_cast<sockaddr *>(&actual), &len) != 0) {
      auto error = last_socket_error();
      close_fd(fd_, true);
      fd_ = -1;
      throw std::runtime_error("getsockname() failed for Prometheus server: " +
                               error);
    }
    address_ = "http://" + advertised + ":" +
               std::to_string(ntohs(actual.sin_port)) + "/metrics";
  }

  void run() {
    while (!stopped_.load(std::memory_order_relaxed)) {
      pollfd pfd{fd_, POLLIN, 0};
      int ready = ::poll(&pfd, 1, 100);
      if (ready <= 0 || stopped_.load(std::memory_order_relaxed)) {
        continue;
      }
      int client = ::accept(fd_, nullptr, nullptr);
      if (client < 0) {
        continue;
      }
      handle_client(client);
      close_fd(client, true);
    }
  }

  void handle_client(int client) {
    char buffer[1024];
    ssize_t n = ::recv(client, buffer, sizeof(buffer) - 1, 0);
    std::string request;
    if (n > 0) {
      buffer[n] = '\0';
      request.assign(buffer, static_cast<size_t>(n));
    }
    bool ok = request.rfind("GET /metrics ", 0) == 0 ||
              request.rfind("GET / ", 0) == 0;
    auto body = ok && obs_ != nullptr ? prometheus_text(*obs_)
                                      : std::string("not found\n");
    auto header = std::string("HTTP/1.1 ") + (ok ? "200 OK" : "404 Not Found") +
                  "\r\nContent-Type: text/plain; version=0.0.4; charset=utf-8\r\n"
                  "Content-Length: " + std::to_string(body.size()) +
                  "\r\nConnection: close\r\n\r\n";
    send_all(client, header);
    send_all(client, body);
  }

  static void send_all(int fd, const std::string &data) {
    const char *ptr = data.data();
    size_t left = data.size();
    while (left > 0) {
      ssize_t sent = ::send(fd, ptr, left, 0);
      if (sent <= 0) {
        return;
      }
      ptr += sent;
      left -= static_cast<size_t>(sent);
    }
  }

  observability::Observability *obs_ = nullptr;
  int fd_ = -1;
  std::string address_;
  std::atomic<bool> stopped_{false};
  std::thread thread_;
};
#endif

class member_relay {
public:
  member_relay(observability::Observability *obs, std::string slug,
               std::string address)
      : obs_(obs), slug_(std::move(slug)), address_(std::move(address)) {}

  ~member_relay() { stop(); }

  void start() {
    if (obs_ == nullptr) {
      return;
    }
    if (obs_->enabled(observability::Family::Logs) && obs_->log_ring) {
      threads_.emplace_back([this]() { pump_logs(); });
    }
    if (obs_->enabled(observability::Family::Events) && obs_->event_bus) {
      threads_.emplace_back([this]() { pump_events(); });
    }
  }

  void stop() {
    if (stopped_.exchange(true)) {
      return;
    }
    cancel_contexts();
    for (auto &thread : threads_) {
      connect_detail::join_thread(&thread);
    }
  }

private:
  std::shared_ptr<grpc::ClientContext> make_context() {
    auto ctx = std::make_shared<grpc::ClientContext>();
    std::scoped_lock lk(contexts_mu_);
    contexts_.push_back(ctx);
    return ctx;
  }

  void cancel_contexts() {
    std::scoped_lock lk(contexts_mu_);
    for (auto it = contexts_.begin(); it != contexts_.end();) {
      if (auto ctx = it->lock()) {
        ctx->TryCancel();
        ++it;
      } else {
        it = contexts_.erase(it);
      }
    }
  }

  member_identity resolve_identity(
      holons::v1::HolonObservability::Stub &stub) {
    grpc::ClientContext context;
    context.set_deadline(std::chrono::system_clock::now() +
                         std::chrono::seconds(5));
    holons::v1::EventsRequest request;
    auto reader = stub.Events(&context, request);
    holons::v1::LogRecord event;
    member_identity fallback{slug_, ""};
    while (reader->Read(&event)) {
      auto record = from_proto_log(event);
      const auto uid = observability::attribute_string(record, "holons.instance_uid");
      if (record.event_name == observability::event_name(observability::EventType::InstanceReady) &&
          !uid.empty()) {
        auto slug = observability::attribute_string(record, "holons.slug");
        member_identity identity{slug.empty() ? slug_ : slug, uid};
        if (event.chain_size() == 0) {
          reader->Finish();
          return identity;
        }
        fallback = identity;
      }
    }
    reader->Finish();
    return fallback;
  }

  void pump_logs() {
    while (!stopped_.load(std::memory_order_relaxed)) {
      try {
        auto channel = connect_detail::dial_ready(address_, 5000);
        auto stub = holons::v1::HolonObservability::NewStub(channel);
        auto identity = resolve_identity(*stub);
        holons::v1::LogsRequest request;
        request.set_min_severity_number(holons::v1::SEVERITY_NUMBER_INFO);
        request.set_follow(true);
        auto context = make_context();
        auto reader = stub->Logs(context.get(), request);
        holons::v1::LogRecord proto;
        while (!stopped_.load(std::memory_order_relaxed) &&
               reader->Read(&proto)) {
          auto entry = from_proto_log(proto);
          entry.chain = observability::enrich_for_multilog(
              entry.chain, identity.slug, identity.instance_uid);
          if (obs_ != nullptr && obs_->log_ring) {
            obs_->log_ring->push(entry);
          }
        }
        reader->Finish();
      } catch (...) {
      }
      retry_delay();
    }
  }

  void pump_events() {
    while (!stopped_.load(std::memory_order_relaxed)) {
      try {
        auto channel = connect_detail::dial_ready(address_, 5000);
        auto stub = holons::v1::HolonObservability::NewStub(channel);
        auto identity = resolve_identity(*stub);
        holons::v1::EventsRequest request;
        request.set_follow(true);
        auto context = make_context();
        auto reader = stub->Events(context.get(), request);
        holons::v1::LogRecord proto;
        while (!stopped_.load(std::memory_order_relaxed) &&
               reader->Read(&proto)) {
          auto event = from_proto_log(proto);
          event.chain = observability::enrich_for_multilog(
              event.chain, identity.slug, identity.instance_uid);
          if (obs_ != nullptr && obs_->event_bus) {
            obs_->event_bus->emit(event);
          }
        }
        reader->Finish();
      } catch (...) {
      }
      retry_delay();
    }
  }

  void retry_delay() const {
    for (int i = 0; i < 20 && !stopped_.load(std::memory_order_relaxed); ++i) {
      std::this_thread::sleep_for(std::chrono::milliseconds(100));
    }
  }

  observability::Observability *obs_ = nullptr;
  std::string slug_;
  std::string address_;
  std::atomic<bool> stopped_{false};
  std::vector<std::thread> threads_;
  std::mutex contexts_mu_;
  std::vector<std::weak_ptr<grpc::ClientContext>> contexts_;
};

class observability_runtime {
public:
  explicit observability_runtime(observability::Observability *obs)
      : obs_(obs),
        service_(std::make_shared<observability_grpc_service>(obs)) {}

  ~observability_runtime() {
    stop();
    observability::reset();
  }

  void register_service(grpc::ServerBuilder &builder) {
    if (service_) {
      builder.RegisterService(service_.get());
    }
  }

  void start(const std::vector<bound_listener> &listeners,
             const options &opts) {
    if (obs_ == nullptr) {
      return;
    }
    const auto public_uri =
        listeners.empty() ? std::string{} : listeners.front().advertised;
    auto transport = std::string{};
    try {
      transport = parse_uri(public_uri).scheme;
    } catch (...) {
    }

#ifndef _WIN32
    if (obs_->enabled(observability::Family::Prom)) {
      auto bind = obs_->cfg.prom_addr.empty() ? std::string("127.0.0.1:0")
                                              : obs_->cfg.prom_addr;
      prom_ = std::make_unique<prometheus_server>(obs_, bind);
    }
#endif

    for (const auto &member : opts.member_endpoints) {
      if (member.slug.empty() || member.address.empty()) {
        continue;
      }
      auto relay = std::make_unique<member_relay>(obs_, member.slug,
                                                  member.address);
      relay->start();
      relays_.push_back(std::move(relay));
    }

    if (!obs_->cfg.run_dir.empty()) {
      observability::enable_disk_writers(obs_->cfg.run_dir);
    }
    if (obs_->enabled(observability::Family::Events)) {
      obs_->emit(observability::EventType::InstanceReady,
                 {{"listener", public_uri},
                  {"metrics_addr", prom_ ? prom_->address() : std::string{}}});
    }
    if (!obs_->cfg.run_dir.empty()) {
      observability::MetaJson meta;
      meta.slug = obs_->cfg.slug;
      meta.uid = obs_->cfg.instance_uid;
      meta.pid = static_cast<int>(::getpid());
      meta.transport = transport;
      meta.address = public_uri;
      meta.metrics_addr = prom_ ? prom_->address() : std::string{};
      meta.log_path = obs_->enabled(observability::Family::Logs)
                          ? (std::filesystem::path(obs_->cfg.run_dir) /
                             "stdout.log")
                                .string()
                          : std::string{};
      meta.organism_uid = obs_->cfg.organism_uid;
      meta.organism_slug = obs_->cfg.organism_slug;
      try {
        observability::write_meta_json(obs_->cfg.run_dir, meta);
      } catch (...) {
      }
    }
  }

  void stop() {
    for (auto &relay : relays_) {
      if (relay) {
        relay->stop();
      }
    }
    relays_.clear();
    if (prom_) {
      prom_->stop();
      prom_.reset();
    }
    if (obs_ != nullptr) {
      obs_->close();
    }
  }

private:
  observability::Observability *obs_ = nullptr;
  std::shared_ptr<observability_grpc_service> service_;
  std::vector<std::unique_ptr<member_relay>> relays_;
#ifndef _WIN32
  std::unique_ptr<prometheus_server> prom_;
#endif
};

inline std::shared_ptr<observability_runtime>
maybe_make_observability_runtime(const options &opts) {
  const char *raw = std::getenv("OP_OBS");
  if (raw == nullptr || trim_copy(raw).empty()) {
    observability::check_env();
    return nullptr;
  }
  auto &current = observability::current();
  if (current.families != 0) {
    return std::make_shared<observability_runtime>(&current);
  }
  auto slug = trim_copy(opts.slug);
  if (slug.empty()) {
    if (auto desc = describe::registered_static_response()) {
      const auto &identity = desc->manifest().identity();
      if (identity.aliases_size() > 0 && !trim_copy(identity.aliases(0)).empty()) {
        slug = trim_copy(identity.aliases(0));
      } else {
        slug = identity.given_name() + "-" + identity.family_name();
        std::string normalized;
        bool dash = false;
        for (char raw : slug) {
          auto ch = static_cast<unsigned char>(raw);
          if (std::isalnum(ch)) {
            normalized.push_back(static_cast<char>(std::tolower(ch)));
            dash = false;
          } else if (!dash) {
            normalized.push_back('-');
            dash = true;
          }
        }
        while (!normalized.empty() && normalized.front() == '-') normalized.erase(normalized.begin());
        while (!normalized.empty() && normalized.back() == '-') normalized.pop_back();
        slug = std::move(normalized);
      }
    }
  }
  auto &obs = observability::from_env(observability::Config{slug});
  if (obs.families == 0) {
    return nullptr;
  }
  return std::make_shared<observability_runtime>(&obs);
}
#else
class observability_runtime {
public:
  void register_service(grpc::ServerBuilder &) {}
  void start(const std::vector<bound_listener> &, const options &) {}
};

inline std::shared_ptr<observability_runtime>
maybe_make_observability_runtime(const options &) {
  observability::check_env();
  const char *raw = std::getenv("OP_OBS");
  if (raw != nullptr && !trim_copy(raw).empty()) {
    throw std::runtime_error(
        "HolonObservability gRPC stubs are required for OP_OBS in cpp-holons");
  }
  return nullptr;
}
#endif

} // namespace detail

inline server_handle start(
    const std::vector<std::string> &listen_uris,
    const register_fn &register_services, options opts = {},
    std::vector<std::shared_ptr<void>> extra_owned_objects = {}) {
#if !HOLONS_HAS_GRPCPP
  (void)listen_uris;
  (void)register_services;
  (void)opts;
  throw std::runtime_error("grpc++ headers are required for serve()");
#else
  detail::maybe_enable_health_check();
  detail::maybe_enable_reflection(opts);

  std::vector<std::string> listeners =
      listen_uris.empty() ? std::vector<std::string>{std::string(kDefaultURI)}
                          : listen_uris;
  std::vector<detail::pending_listener> pending;
  pending.reserve(listeners.size());

  grpc::ServerBuilder builder;
  int stdio_listeners = 0;
  for (const auto &uri : listeners) {
    auto parsed = parse_uri(uri);
    detail::pending_listener item{uri, parsed, std::make_shared<int>(0), false};

    if (parsed.scheme == "tcp" || parsed.scheme == "unix") {
      builder.AddListeningPort(detail::grpc_listen_target(parsed),
                               grpc::InsecureServerCredentials(),
                               item.selected_port.get());
      pending.push_back(std::move(item));
      continue;
    }

    if (parsed.scheme == "stdio") {
      ++stdio_listeners;
      item.attach_stdio = true;
      builder.AddListeningPort("127.0.0.1:0",
                               grpc::InsecureServerCredentials(),
                               item.selected_port.get());
      pending.push_back(std::move(item));
      continue;
    }

    throw std::invalid_argument("unsupported serve transport: " + uri);
  }

  if (stdio_listeners > 1) {
    throw std::invalid_argument("serve() supports at most one stdio:// listener");
  }

  std::vector<std::shared_ptr<void>> owned_objects =
      std::move(extra_owned_objects);
  auto observability_runtime = detail::maybe_make_observability_runtime(opts);
  if (observability_runtime) {
    observability_runtime->register_service(builder);
    owned_objects.push_back(observability_runtime);
  }
  try {
    auto holon_meta_service = detail::maybe_make_holon_meta_service(opts);
    if (holon_meta_service) {
      builder.RegisterService(holon_meta_service.get());
      owned_objects.push_back(holon_meta_service);
    }
  } catch (const std::exception &error) {
    std::fprintf(stderr, "HolonMeta registration failed: %s\n", error.what());
    throw;
  }
  if (register_services) {
    register_services(builder);
  }

  const auto active_scheme =
      pending.empty() ? std::string{} : pending.front().parsed.scheme;
  detail::set_current_transport(active_scheme);
  std::unique_ptr<grpc::Server> server;
  try {
    server = builder.BuildAndStart();
  } catch (...) {
    detail::clear_current_transport();
    throw;
  }
  if (!server) {
    detail::clear_current_transport();
    throw std::runtime_error("grpc::ServerBuilder::BuildAndStart() failed");
  }

  for (const auto &item : pending) {
    if (!item.attach_stdio) {
      continue;
    }
#ifdef _WIN32
    server->Shutdown();
    detail::clear_current_transport();
    throw std::runtime_error(
        "stdio:// serve is not supported on Windows in cpp-holons");
#else
    auto port = item.selected_port ? *item.selected_port : 0;
    if (port <= 0) {
      server->Shutdown();
      detail::clear_current_transport();
      throw std::runtime_error("stdio:// serve bridge did not get a loopback port");
    }
    auto bridge = detail::stdio_bridge::connect_loopback(port);
    bridge->start();
    owned_objects.push_back(std::move(bridge));
#endif
  }

  std::vector<bound_listener> bound;
  bound.reserve(pending.size());
  for (const auto &item : pending) {
    bound.push_back(bound_listener{item.requested,
                                   detail::advertised_listener(item)});
    if (opts.announce) {
      std::fprintf(stderr, "gRPC server listening on %s\n",
                   bound.back().advertised.c_str());
    }
  }

  if (observability_runtime) {
    observability_runtime->start(bound, opts);
  }

  return server_handle(std::move(server), std::move(bound),
                       std::move(owned_objects), true);
#endif
}

inline server_handle start(const std::string &listen_uri,
                           const register_fn &register_services,
                           options opts = {},
                           std::vector<std::shared_ptr<void>> extra_owned_objects = {}) {
  return start(std::vector<std::string>{listen_uri}, register_services,
               std::move(opts), std::move(extra_owned_objects));
}

inline void serve(const std::vector<std::string> &listen_uris,
                  const register_fn &register_services, options opts = {},
                  std::vector<std::shared_ptr<void>> extra_owned_objects = {}) {
  detail::scoped_signal_handlers signals;
  detail::parent_watch parent_watch;
  const pid_t expected_parent = detail::configured_parent_pid();
  detail::debug_parent_watch_log("serve start expected_parent=%d listeners=%zu",
                                 static_cast<int>(expected_parent),
                                 listen_uris.size());
  auto handle =
      start(listen_uris, register_services, opts, std::move(extra_owned_objects));

  std::thread waiter([&handle]() {
    handle.wait();
    detail::shutdown_requested() = 1;
  });

  while (!detail::shutdown_requested()) {
    if (detail::parent_process_gone(expected_parent)) {
      detail::request_shutdown("serve loop detected parent exit",
                               expected_parent);
      break;
    }
    std::this_thread::sleep_for(std::chrono::milliseconds(50));
  }

  detail::debug_parent_watch_log("serve stopping server");
  handle.stop(opts.graceful_shutdown_timeout_ms);
  connect_detail::join_thread(&waiter);
  detail::debug_parent_watch_log("serve finished");
}

inline void serve(const std::string &listen_uri,
                  const register_fn &register_services, options opts = {},
                  std::vector<std::shared_ptr<void>> extra_owned_objects = {}) {
  serve(std::vector<std::string>{listen_uri}, register_services,
        std::move(opts), std::move(extra_owned_objects));
}

} // namespace holons::serve
