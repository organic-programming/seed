#pragma once

#include "describe.hpp"
#include "holons.hpp"

#include <array>
#include <csignal>

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

namespace holons::serve {

struct options {
  bool enable_reflection = false;
  bool auto_register_holon_meta = true;
  bool announce = true;
  int graceful_shutdown_timeout_ms = 10000;
};

struct parsed_flags {
  std::vector<std::string> listeners;
  bool reflect = false;
};

struct bound_listener {
  std::string requested;
  std::string advertised;
};

using register_fn = std::function<void(grpc::ServerBuilder &)>;

inline parsed_flags parse_options(const std::vector<std::string> &args) {
  parsed_flags parsed;
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
                std::vector<std::shared_ptr<void>> owned_objects = {})
      : server_(std::move(server)),
        listeners_(std::move(listeners)),
        owned_objects_(std::move(owned_objects)) {}
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
#endif
};

namespace detail {

inline volatile std::sig_atomic_t &shutdown_requested() {
  static volatile std::sig_atomic_t requested = 0;
  return requested;
}

inline void signal_handler(int) { shutdown_requested() = 1; }

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
  explicit stdio_bridge(int port) : port_(port) {
    input_fd_ = ::dup(STDIN_FILENO);
    if (input_fd_ < 0) {
      throw std::runtime_error("dup(STDIN_FILENO) failed for stdio:// serve");
    }

    output_fd_ = ::dup(STDOUT_FILENO);
    if (output_fd_ < 0) {
      close_fd(input_fd_, false);
      input_fd_ = -1;
      throw std::runtime_error("dup(STDOUT_FILENO) failed for stdio:// serve");
    }
  }

  ~stdio_bridge() { stop(); }

  void start() {
    socket_fd_ = connect_loopback();
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
  int connect_loopback() const {
    int fd = ::socket(AF_INET, SOCK_STREAM, 0);
    if (fd < 0) {
      throw std::runtime_error("socket() failed for stdio:// serve bridge: " +
                               std::string(std::strerror(errno)));
    }

    sockaddr_in addr{};
    addr.sin_family = AF_INET;
    addr.sin_port = htons(static_cast<uint16_t>(port_));
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

  int port_ = 0;
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

} // namespace detail

inline server_handle start(const std::vector<std::string> &listen_uris,
                           const register_fn &register_services,
                           options opts = {}) {
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

  std::vector<std::shared_ptr<void>> owned_objects;
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

  auto server = builder.BuildAndStart();
  if (!server) {
    throw std::runtime_error("grpc::ServerBuilder::BuildAndStart() failed");
  }

  for (const auto &item : pending) {
    if (!item.attach_stdio) {
      continue;
    }
#ifdef _WIN32
    server->Shutdown();
    throw std::runtime_error(
        "stdio:// serve is not supported on Windows in cpp-holons");
#else
    auto port = item.selected_port ? *item.selected_port : 0;
    if (port <= 0) {
      server->Shutdown();
      throw std::runtime_error("stdio:// serve bridge did not get a loopback port");
    }
    auto bridge = std::make_shared<detail::stdio_bridge>(port);
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

  return server_handle(std::move(server), std::move(bound),
                       std::move(owned_objects));
#endif
}

inline server_handle start(const std::string &listen_uri,
                           const register_fn &register_services,
                           options opts = {}) {
  return start(std::vector<std::string>{listen_uri}, register_services,
               std::move(opts));
}

inline void serve(const std::vector<std::string> &listen_uris,
                  const register_fn &register_services, options opts = {}) {
  detail::scoped_signal_handlers signals;
  auto handle = start(listen_uris, register_services, opts);

  std::thread waiter([&handle]() {
    handle.wait();
    detail::shutdown_requested() = 1;
  });

  while (!detail::shutdown_requested()) {
    std::this_thread::sleep_for(std::chrono::milliseconds(50));
  }

  handle.stop(opts.graceful_shutdown_timeout_ms);
  connect_detail::join_thread(&waiter);
}

inline void serve(const std::string &listen_uri,
                  const register_fn &register_services, options opts = {}) {
  serve(std::vector<std::string>{listen_uri}, register_services,
        std::move(opts));
}

} // namespace holons::serve
