#pragma once

#include "holons.hpp"
#include "observability.hpp"
#include "serve.hpp"
#include "holons/v1/describe.grpc.pb.h"
#include "holons/v1/observability.grpc.pb.h"

#include <atomic>
#include <deque>
#include <future>
#include <random>
#include <set>

namespace holons::composite {

struct ChildSpec {
  std::string slug;
  std::string binary;
};

struct DialOptions {
  std::optional<bool> transitive_observability;
};

inline DialOptions WithTransitiveObservability(bool enabled) {
  DialOptions opts;
  opts.transitive_observability = enabled;
  return opts;
}

struct SpawnOptions {
  std::string slug;
  std::string binary_path;
  std::string transport;
  std::string instance_uid;
  std::vector<ChildSpec> downstream_chain;
  std::map<std::string, std::string> extra_env;
  std::vector<DialOptions> dial_options;
};

struct CascadeOptions {
  std::string transport;
  std::vector<ChildSpec> members;
  std::map<std::string, std::string> extra_env;
};

struct CheckOutcome {
  bool pass{false};
  std::string evidence;
};

struct ChainHop {
  std::string slug;
  std::string instance_uid;
};

struct LogCheckOptions {
  std::shared_ptr<grpc::Channel> channel;
  std::string sender;
  std::string leaf_uid;
  std::vector<ChainHop> expected_chain;
  std::chrono::milliseconds timeout{3000};
  std::chrono::milliseconds poll_interval{100};
  bool live{false};
};

struct EventCheckOptions {
  std::shared_ptr<grpc::Channel> channel;
  observability::EventType event_type{observability::EventType::InstanceReady};
  std::string leaf_uid;
  std::vector<ChainHop> expected_chain;
  std::chrono::milliseconds timeout{3000};
  std::chrono::milliseconds poll_interval{100};
  bool live{false};
};

inline const std::vector<std::string> TransportCoverageSequence = {
    "stdio", "stdio", "tcp", "unix", "tcp", "tcp",
    "stdio", "unix", "unix", "stdio",
};

namespace detail {

inline std::string trim(std::string value) { return trim_copy(std::move(value)); }

inline std::string active_observability_families() {
  auto &obs = observability::current();
  std::vector<std::string> families;
  if (obs.enabled(observability::Family::Logs)) families.push_back("logs");
  if (obs.enabled(observability::Family::Metrics)) families.push_back("metrics");
  if (obs.enabled(observability::Family::Events)) families.push_back("events");
  if (obs.enabled(observability::Family::Prom)) families.push_back("prom");
  std::string out;
  for (size_t i = 0; i < families.size(); ++i) {
    if (i) out += ",";
    out += families[i];
  }
  return out;
}

inline std::string default_run_root() {
  if (const char *raw = std::getenv("OP_RUN_DIR"); raw != nullptr && *raw) {
    return raw;
  }
  if (const char *raw = std::getenv("OPPATH"); raw != nullptr && *raw) {
    return (std::filesystem::path(raw) / "run").string();
  }
  if (const char *raw = std::getenv("HOME"); raw != nullptr && *raw) {
    return (std::filesystem::path(raw) / ".op" / "run").string();
  }
  return (std::filesystem::temp_directory_path() / ".op" / "run").string();
}

inline std::string new_instance_uid() {
  std::random_device rd;
  std::mt19937_64 gen(rd());
  std::uniform_int_distribution<unsigned long long> dist;
  std::ostringstream out;
  out << std::hex << dist(gen) << dist(gen);
  return out.str().substr(0, 24);
}

inline std::string socket_token(std::string value) {
  value = trim(std::move(value));
  if (value.size() > 24) value.resize(24);
  for (auto &ch : value) {
    if (ch == '/' || ch == '\\' || ch == ':' || std::isspace(static_cast<unsigned char>(ch))) {
      ch = '-';
    }
  }
  return value.empty() ? new_instance_uid() : value;
}

inline std::pair<std::string, std::string>
listen_uri_for_spawn(const std::string &transport, const std::string &uid) {
  if (transport == "stdio") return {"stdio://", ""};
  if (transport == "tcp") return {"tcp://127.0.0.1:0", ""};
  if (transport == "unix") {
    auto path = std::filesystem::temp_directory_path() /
                ("op-" + socket_token(uid) + ".sock");
    return {"unix://" + path.string(), path.string()};
  }
  throw std::invalid_argument("unsupported transport \"" + transport + "\"");
}

inline std::vector<std::string> child_args(const std::vector<ChildSpec> &children) {
  std::vector<std::string> args;
  for (const auto &child : children) {
    if (trim(child.slug).empty() || trim(child.binary).empty()) {
      throw std::invalid_argument("downstream child requires slug and binary");
    }
    args.push_back("--child");
    args.push_back(child.slug + "=" + child.binary);
  }
  return args;
}

inline std::vector<char *> argv_from(std::vector<std::string> &args) {
  std::vector<char *> argv;
  argv.reserve(args.size() + 1);
  for (auto &arg : args) argv.push_back(arg.data());
  argv.push_back(nullptr);
  return argv;
}

struct process_state {
  pid_t pid{-1};
  int stdio_fd{-1};
  std::shared_ptr<connect_detail::process_handle> process;
  std::string proxy_target;
};

inline void apply_spawn_env(const SpawnOptions &opts,
                            const std::string &uid,
                            const std::string &run_root);

inline process_state fork_exec_stdio_child(std::vector<std::string> args,
                                           const SpawnOptions &opts,
                                           const std::string &uid,
                                           const std::string &run_root) {
#ifdef _WIN32
  (void)args;
  (void)opts;
  (void)uid;
  (void)run_root;
  throw std::runtime_error("stdio transport is not supported on Windows");
#else
  int transport_pair[2] = {-1, -1};
  int stderr_pipe[2] = {-1, -1};
  int listener_fd = -1;
  std::string proxy_uri;
  if (::socketpair(AF_UNIX, SOCK_STREAM, 0, transport_pair) != 0 ||
      ::pipe(stderr_pipe) != 0) {
    connect_detail::close_pipe_fd(&stderr_pipe[0]);
    connect_detail::close_pipe_fd(&stderr_pipe[1]);
    if (transport_pair[0] >= 0) close_fd(transport_pair[0], true);
    if (transport_pair[1] >= 0) close_fd(transport_pair[1], true);
    throw std::runtime_error("stdio transport setup failed");
  }

  try {
    listener_fd = connect_detail::create_loopback_listener(&proxy_uri);
  } catch (...) {
    connect_detail::close_pipe_fd(&stderr_pipe[0]);
    connect_detail::close_pipe_fd(&stderr_pipe[1]);
    if (transport_pair[0] >= 0) close_fd(transport_pair[0], true);
    if (transport_pair[1] >= 0) close_fd(transport_pair[1], true);
    throw;
  }

  auto cleanup_fds = [&]() {
    connect_detail::close_pipe_fd(&stderr_pipe[0]);
    connect_detail::close_pipe_fd(&stderr_pipe[1]);
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

  pid_t pid = ::fork();
  if (pid < 0) {
    cleanup_fds();
    throw std::runtime_error("fork() failed");
  }
  if (pid == 0) {
    ::dup2(transport_pair[1], STDIN_FILENO);
    ::dup2(transport_pair[1], STDOUT_FILENO);
    ::dup2(stderr_pipe[1], STDERR_FILENO);
    cleanup_fds();
    auto cwd = std::filesystem::path(opts.binary_path).parent_path();
    if (!cwd.empty()) ::chdir(cwd.c_str());
    apply_spawn_env(opts, uid, run_root);
    auto argv = argv_from(args);
    ::execv(opts.binary_path.c_str(), argv.data());
    std::perror("execv");
    ::_exit(127);
  }

  close_fd(transport_pair[1], true);
  transport_pair[1] = -1;
  connect_detail::close_pipe_fd(&stderr_pipe[1]);

  auto process = std::make_shared<connect_detail::process_handle>();
  process->pid = pid;
  process->stdin_fd = transport_pair[0];
  process->stdout_fd = transport_pair[0];
  process->stderr_fd = stderr_pipe[0];
  process->listener_fd = listener_fd;

  process->stderr_thread = std::thread([process]() {
    std::array<char, 4096> buffer{};
    for (;;) {
      auto n = ::read(process->stderr_fd, buffer.data(), buffer.size());
      if (n == 0) return;
      if (n < 0) {
        if (errno == EINTR) continue;
        return;
      }
      std::lock_guard<std::mutex> lock(process->stderr_mutex);
      process->stderr_capture.append(buffer.data(), static_cast<size_t>(n));
    }
  });

  process->accept_thread = std::thread([process]() {
    int accepted = ::accept(process->listener_fd, nullptr, nullptr);
    if (accepted < 0) return;
    {
      std::lock_guard<std::mutex> lock(process->client_mutex);
      if (process->closed) {
        close_fd(accepted, true);
        return;
      }
      process->client_fd = accepted;
    }
    std::thread upstream([process, accepted]() {
      connect_detail::relay_fd(accepted, process->stdin_fd);
    });
    std::thread downstream([process, accepted]() {
      connect_detail::relay_fd(process->stdout_fd, accepted);
    });
    connect_detail::join_thread(&upstream);
    connect_detail::join_thread(&downstream);
  });

  return {pid, -1, process, proxy_uri};
#endif
}

inline void apply_spawn_env(const SpawnOptions &opts,
                            const std::string &uid,
                            const std::string &run_root) {
  ::setenv("OP_INSTANCE_UID", uid.c_str(), 1);
  ::setenv("OP_RUN_DIR", run_root.c_str(), 1);
  ::setenv("HOLONS_PARENT_PID", std::to_string(::getppid()).c_str(), 1);
  if (const auto families = active_observability_families(); !families.empty()) {
    ::setenv("OP_OBS", families.c_str(), 1);
  }
  for (const auto &[key, value] : opts.extra_env) {
    ::setenv(key.c_str(), value.c_str(), 1);
  }
}

inline process_state fork_exec_child(const SpawnOptions &opts,
                                     const std::string &uid,
                                     const std::string &listen_uri,
                                     const std::string &transport,
                                     const std::string &run_root) {
  std::vector<std::string> args;
  if (transport == "stdio") {
    args = {opts.binary_path, "serve", "--listen", "tcp://127.0.0.1:0",
            "--listen", listen_uri, "--transport", transport};
  } else {
    args = {opts.binary_path, "serve", "--listen", listen_uri,
            "--transport", transport};
  }
  auto child_flags = child_args(opts.downstream_chain);
  args.insert(args.end(), child_flags.begin(), child_flags.end());

  if (transport == "stdio") {
    return fork_exec_stdio_child(std::move(args), opts, uid, run_root);
  }

  pid_t pid = ::fork();
  if (pid < 0) {
    throw std::runtime_error("fork() failed");
  }
  if (pid == 0) {
    auto cwd = std::filesystem::path(opts.binary_path).parent_path();
    if (!cwd.empty()) ::chdir(cwd.c_str());
    apply_spawn_env(opts, uid, run_root);
    auto argv = argv_from(args);
    ::execv(opts.binary_path.c_str(), argv.data());
    std::perror("execv");
    ::_exit(127);
  }
  return {pid, -1};
}

inline holons::v1::DescribeResponse describe_ready(
    const std::shared_ptr<grpc::Channel> &channel,
    std::chrono::milliseconds timeout) {
  auto deadline = std::chrono::steady_clock::now() + timeout;
  std::string last_error;
  while (std::chrono::steady_clock::now() < deadline) {
    auto stub = holons::v1::HolonMeta::NewStub(channel);
    grpc::ClientContext context;
    context.set_deadline(std::chrono::system_clock::now() + std::chrono::milliseconds(500));
    holons::v1::DescribeRequest request;
    holons::v1::DescribeResponse response;
    auto status = stub->Describe(&context, request, &response);
    if (status.ok()) return response;
    last_error = status.error_message();
    std::this_thread::sleep_for(std::chrono::milliseconds(50));
  }
  throw std::runtime_error(last_error.empty() ? "Describe did not become ready"
                                             : last_error);
}

struct meta_json {
  std::string uid;
  std::string address;
};

inline meta_json read_meta_json(const std::filesystem::path &path) {
  std::ifstream in(path);
  if (!in) throw std::runtime_error("meta.json not readable: " + path.string());
  nlohmann::json json = nlohmann::json::parse(in);
  return {json.value("uid", ""), json.value("address", "")};
}

inline meta_json wait_meta(const std::string &run_root,
                           const std::string &slug,
                           const std::string &uid,
                           std::chrono::milliseconds timeout) {
  auto path = std::filesystem::path(run_root) / slug / uid / "meta.json";
  auto deadline = std::chrono::steady_clock::now() + timeout;
  std::string last_error;
  while (std::chrono::steady_clock::now() < deadline) {
    try {
      auto meta = read_meta_json(path);
      if (meta.uid == uid && !trim(meta.address).empty()) return meta;
    } catch (const std::exception &error) {
      last_error = error.what();
    }
    std::this_thread::sleep_for(std::chrono::milliseconds(50));
  }
  throw std::runtime_error(last_error.empty() ? "meta not ready for " + slug + "/" + uid
                                             : last_error);
}

class transitive_relay {
public:
  transitive_relay(std::string slug, std::string uid, std::shared_ptr<grpc::Channel> channel)
      : slug_(std::move(slug)), uid_(std::move(uid)), channel_(std::move(channel)) {}

  ~transitive_relay() { stop(); }

  void start() {
    auto &obs = observability::current();
    if (obs.enabled(observability::Family::Logs) && obs.log_ring) {
      threads_.emplace_back([this]() { pump_logs(); });
    }
    if (obs.enabled(observability::Family::Events) && obs.event_bus) {
      threads_.emplace_back([this]() { pump_events(); });
    }
  }

  void stop() {
    if (stopped_.exchange(true)) return;
    {
      std::scoped_lock lk(contexts_mu_);
      for (auto &weak : contexts_) {
        if (auto ctx = weak.lock()) ctx->TryCancel();
      }
    }
    for (auto &thread : threads_) connect_detail::join_thread(&thread);
  }

private:
  std::shared_ptr<grpc::ClientContext> context() {
    auto ctx = std::make_shared<grpc::ClientContext>();
    std::scoped_lock lk(contexts_mu_);
    contexts_.push_back(ctx);
    return ctx;
  }

  void pump_logs() {
    auto stub = holons::v1::HolonObservability::NewStub(channel_);
    auto ctx = context();
    holons::v1::LogsRequest request;
    request.set_min_severity_number(holons::v1::SEVERITY_NUMBER_INFO);
    request.set_follow(true);
    auto reader = stub->Logs(ctx.get(), request);
    holons::v1::LogRecord proto;
    while (!stopped_.load(std::memory_order_relaxed) && reader->Read(&proto)) {
      auto entry = serve::detail::from_proto_log(proto);
      entry.chain = observability::append_direct_child(entry.chain, slug_, uid_);
      auto &obs = observability::current();
      if (obs.log_ring) obs.log_ring->push(entry);
    }
    reader->Finish();
  }

  void pump_events() {
    auto stub = holons::v1::HolonObservability::NewStub(channel_);
    auto ctx = context();
    holons::v1::EventsRequest request;
    request.set_follow(true);
    auto reader = stub->Events(ctx.get(), request);
    holons::v1::LogRecord proto;
    while (!stopped_.load(std::memory_order_relaxed) && reader->Read(&proto)) {
      auto event = serve::detail::from_proto_log(proto);
      event.chain = observability::append_direct_child(event.chain, slug_, uid_);
      auto &obs = observability::current();
      if (obs.event_bus) obs.event_bus->emit(event);
    }
    reader->Finish();
  }

  std::string slug_;
  std::string uid_;
  std::shared_ptr<grpc::Channel> channel_;
  std::atomic<bool> stopped_{false};
  std::vector<std::thread> threads_;
  std::mutex contexts_mu_;
  std::vector<std::weak_ptr<grpc::ClientContext>> contexts_;
};

inline DialOptions merge_options(const std::vector<DialOptions> &opts) {
  DialOptions out;
  for (const auto &opt : opts) {
    if (opt.transitive_observability.has_value()) {
      out.transitive_observability = opt.transitive_observability;
    }
  }
  return out;
}

inline std::string slug_from_describe(const holons::v1::DescribeResponse &desc) {
  const auto &identity = desc.manifest().identity();
  if (identity.aliases_size() > 0 && !trim(identity.aliases(0)).empty()) {
    return trim(identity.aliases(0));
  }
  auto value = identity.given_name() + "-" + identity.family_name();
  std::string out;
  bool dash = false;
  for (char raw : value) {
    auto ch = static_cast<unsigned char>(raw);
    if (std::isalnum(ch)) {
      out.push_back(static_cast<char>(std::tolower(ch)));
      dash = false;
    } else if (!dash) {
      out.push_back('-');
      dash = true;
    }
  }
  while (!out.empty() && out.front() == '-') out.erase(out.begin());
  while (!out.empty() && out.back() == '-') out.pop_back();
  return out;
}

inline std::optional<std::pair<std::string, std::string>>
identity_from_events(const std::shared_ptr<grpc::Channel> &channel,
                     const std::string &fallback_slug) {
  auto stub = holons::v1::HolonObservability::NewStub(channel);
  grpc::ClientContext context;
  context.set_deadline(std::chrono::system_clock::now() + std::chrono::seconds(1));
  holons::v1::EventsRequest request;
  auto reader = stub->Events(&context, request);
  holons::v1::LogRecord event;
  while (reader->Read(&event)) {
    auto record = serve::detail::from_proto_log(event);
    auto uid = observability::attribute_string(record, "holons.instance_uid");
    if (event.chain_size() == 0 && !uid.empty()) {
      auto slug = observability::attribute_string(record, "holons.slug");
      return {{slug.empty() ? fallback_slug : slug, uid}};
    }
  }
  reader->Finish();
  return std::nullopt;
}

inline std::optional<std::pair<std::string, std::string>>
identity_from_logs(const std::shared_ptr<grpc::Channel> &channel,
                   const std::string &fallback_slug) {
  auto stub = holons::v1::HolonObservability::NewStub(channel);
  grpc::ClientContext context;
  context.set_deadline(std::chrono::system_clock::now() + std::chrono::seconds(1));
  holons::v1::LogsRequest request;
  auto reader = stub->Logs(&context, request);
  holons::v1::LogRecord entry;
  while (reader->Read(&entry)) {
    auto record = serve::detail::from_proto_log(entry);
    auto uid = observability::attribute_string(record, "holons.instance_uid");
    if (entry.chain_size() == 0 && !uid.empty()) {
      auto slug = observability::attribute_string(record, "holons.slug");
      return {{slug.empty() ? fallback_slug : slug, uid}};
    }
  }
  reader->Finish();
  return std::nullopt;
}

inline std::pair<std::string, std::string>
resolve_identity(const std::shared_ptr<grpc::Channel> &channel,
                 const holons::v1::DescribeResponse &desc) {
  auto fallback = slug_from_describe(desc);
  if (auto identity = identity_from_events(channel, fallback)) return *identity;
  if (auto identity = identity_from_logs(channel, fallback)) return *identity;
  throw std::runtime_error("peer did not expose slug and instance_uid");
}

inline std::vector<observability::LogRecord>
read_log_entries(const std::shared_ptr<grpc::Channel> &channel) {
  if (!channel) {
    auto &obs = observability::current();
    if (!obs.log_ring) throw std::runtime_error("logs family is not enabled");
    return obs.log_ring->drain();
  }
  auto stub = holons::v1::HolonObservability::NewStub(channel);
  grpc::ClientContext context;
  context.set_deadline(std::chrono::system_clock::now() + std::chrono::seconds(2));
  holons::v1::LogsRequest request;
  request.set_min_severity_number(holons::v1::SEVERITY_NUMBER_INFO);
  auto reader = stub->Logs(&context, request);
  std::vector<observability::LogRecord> out;
  holons::v1::LogRecord entry;
  while (reader->Read(&entry)) out.push_back(serve::detail::from_proto_log(entry));
  reader->Finish();
  return out;
}

inline std::vector<observability::LogRecord>
read_event_entries(const std::shared_ptr<grpc::Channel> &channel) {
  if (!channel) {
    auto &obs = observability::current();
    if (!obs.event_bus) throw std::runtime_error("events family is not enabled");
    return obs.event_bus->drain();
  }
  auto stub = holons::v1::HolonObservability::NewStub(channel);
  grpc::ClientContext context;
  context.set_deadline(std::chrono::system_clock::now() + std::chrono::seconds(2));
  holons::v1::EventsRequest request;
  auto reader = stub->Events(&context, request);
  std::vector<observability::LogRecord> out;
  holons::v1::LogRecord event;
  while (reader->Read(&event)) out.push_back(serve::detail::from_proto_log(event));
  reader->Finish();
  return out;
}

inline std::string compact(std::string value) {
  std::string out;
  bool ws = false;
  for (char ch : value) {
    if (std::isspace(static_cast<unsigned char>(ch))) {
      if (!ws) out.push_back(' ');
      ws = true;
    } else {
      out.push_back(ch);
      ws = false;
    }
  }
  if (out.size() > 240) out = out.substr(0, 240) + "...";
  return out;
}

inline std::string compare_chain(const std::vector<ChainHop> &got,
                                 const std::vector<ChainHop> &want) {
  if (got.size() != want.size()) {
    return "chain length " + std::to_string(got.size()) + " want " +
           std::to_string(want.size());
  }
  for (size_t i = 0; i < want.size(); ++i) {
    if (got[i].slug != want[i].slug ||
        got[i].instance_uid != want[i].instance_uid) {
      return "hop " + std::to_string(i) + "=" + got[i].slug + "/" +
             got[i].instance_uid + " want " + want[i].slug + "/" +
             want[i].instance_uid;
    }
  }
  return {};
}

inline std::string compare_chain(const std::vector<std::string> &got,
                                 const std::vector<ChainHop> &want) {
  if (got.size() != want.size()) {
    return "chain length " + std::to_string(got.size()) + " want " +
           std::to_string(want.size());
  }
  for (size_t i = 0; i < want.size(); ++i) {
    if (got[i] != want[i].slug) {
      return "hop " + std::to_string(i) + "=" + got[i] + " want " + want[i].slug;
    }
  }
  return {};
}

inline std::mutex &dial_relays_mu() {
  static std::mutex mu;
  return mu;
}

inline std::vector<std::shared_ptr<transitive_relay>> &dial_relays() {
  static std::vector<std::shared_ptr<transitive_relay>> relays;
  return relays;
}

inline void remember_dial_relay(std::shared_ptr<transitive_relay> relay) {
  std::scoped_lock lk(dial_relays_mu());
  dial_relays().push_back(std::move(relay));
}

} // namespace detail

class SpawnedMember {
public:
  std::string slug;
  std::string uid;
  std::string listen_uri;
  std::shared_ptr<grpc::Channel> channel;

  SpawnedMember() = default;
  SpawnedMember(const SpawnedMember &) = delete;
  SpawnedMember &operator=(const SpawnedMember &) = delete;
  SpawnedMember(SpawnedMember &&other) noexcept { move_from(std::move(other)); }
  SpawnedMember &operator=(SpawnedMember &&other) noexcept {
    if (this != &other) {
      stop();
      move_from(std::move(other));
    }
    return *this;
  }
  ~SpawnedMember() { stop(std::chrono::milliseconds(3000)); }

  void stop(std::chrono::milliseconds timeout = std::chrono::milliseconds(3000)) {
    if (stopped_) return;
    stopped_ = true;
    if (relay_) {
      relay_->stop();
      relay_.reset();
    }
    channel.reset();
    if (process_.stdio_fd >= 0) {
      close_fd(process_.stdio_fd, true);
      process_.stdio_fd = -1;
    }
    if (process_.process) {
      connect_detail::stop_process(process_.process);
      process_.process.reset();
      process_.pid = -1;
      return;
    }
    if (process_.pid <= 0) return;
    ::kill(process_.pid, SIGTERM);
    auto deadline = std::chrono::steady_clock::now() + timeout;
    int status = 0;
    while (std::chrono::steady_clock::now() < deadline) {
      pid_t done = ::waitpid(process_.pid, &status, WNOHANG);
      if (done == process_.pid) {
        process_.pid = -1;
        return;
      }
      std::this_thread::sleep_for(std::chrono::milliseconds(50));
    }
    ::kill(process_.pid, SIGKILL);
    ::waitpid(process_.pid, &status, 0);
    process_.pid = -1;
  }

private:
  friend SpawnedMember SpawnMember(SpawnOptions opts);
  void move_from(SpawnedMember &&other) noexcept {
    slug = std::move(other.slug);
    uid = std::move(other.uid);
    listen_uri = std::move(other.listen_uri);
    channel = std::move(other.channel);
    process_ = std::move(other.process_);
    relay_ = std::move(other.relay_);
    stopped_ = other.stopped_;
    other.process_ = {};
    other.process_.pid = -1;
    other.process_.stdio_fd = -1;
    other.stopped_ = true;
  }

  detail::process_state process_;
  std::unique_ptr<detail::transitive_relay> relay_;
  bool stopped_{false};
};

inline SpawnedMember SpawnMember(SpawnOptions opts) {
  opts.slug = detail::trim(opts.slug.empty()
                              ? std::filesystem::path(opts.binary_path).stem().string()
                              : opts.slug);
  opts.binary_path = detail::trim(opts.binary_path);
  if (opts.slug.empty()) throw std::invalid_argument("spawn member: slug is required");
  if (opts.binary_path.empty()) {
    throw std::invalid_argument("spawn member " + opts.slug + ": binary path is required");
  }
  auto transport = connect_detail::lower_copy(detail::trim(opts.transport));
  if (transport.empty()) transport = "stdio";
  auto uid = detail::trim(opts.instance_uid);
  if (uid.empty()) uid = detail::new_instance_uid();
  auto [listen_uri, cleanup] = detail::listen_uri_for_spawn(transport, uid);
  if (!cleanup.empty()) std::filesystem::remove(cleanup);
  auto run_root = detail::default_run_root();

  SpawnedMember member;
  member.slug = opts.slug;
  member.uid = uid;
  member.listen_uri = listen_uri;
  member.process_ = detail::fork_exec_child(opts, uid, listen_uri, transport, run_root);
  try {
    std::shared_ptr<grpc::Channel> relay_channel;
    if (transport == "stdio") {
      auto meta = detail::wait_meta(run_root, opts.slug, uid, std::chrono::milliseconds(10000));
      relay_channel = connect_detail::dial_ready(meta.address, 10000);
      (void)detail::describe_ready(relay_channel, std::chrono::milliseconds(10000));
      member.channel = connect_detail::dial_ready(member.process_.proxy_target, 10000);
      (void)detail::describe_ready(member.channel, std::chrono::milliseconds(10000));
      member.listen_uri = "stdio://";
    } else {
      auto meta = detail::wait_meta(run_root, opts.slug, uid, std::chrono::milliseconds(10000));
      member.listen_uri = meta.address;
      member.channel = connect_detail::dial_ready(meta.address, 10000);
      (void)detail::describe_ready(member.channel, std::chrono::milliseconds(10000));
      relay_channel = member.channel;
    }
    auto dial_opts = detail::merge_options(opts.dial_options);
    bool transitive = true;
    if (dial_opts.transitive_observability.has_value()) {
      transitive = *dial_opts.transitive_observability;
    }
    if (transitive) {
      member.relay_ = std::make_unique<detail::transitive_relay>(
          opts.slug, uid, relay_channel);
      member.relay_->start();
    }
  } catch (...) {
    member.stop();
    throw;
  }
  return member;
}

class Cascade {
public:
  std::unique_ptr<SpawnedMember> top;
  Cascade() = default;
  Cascade(const Cascade &) = delete;
  Cascade &operator=(const Cascade &) = delete;
  Cascade(Cascade &&) noexcept = default;
  Cascade &operator=(Cascade &&) noexcept = default;
  void stop() {
    if (top) top->stop();
  }
  ~Cascade() { stop(); }
};

inline Cascade BuildCascade(CascadeOptions opts) {
  if (opts.members.empty()) {
    throw std::invalid_argument("build cascade: at least one member is required");
  }
  auto top_spec = opts.members.front();
  std::vector<ChildSpec> downstream(opts.members.begin() + 1, opts.members.end());
  auto spawned = SpawnMember(SpawnOptions{
      top_spec.slug,
      top_spec.binary,
      opts.transport,
      "",
      downstream,
      opts.extra_env,
      {},
  });
  Cascade cascade;
  cascade.top = std::make_unique<SpawnedMember>(std::move(spawned));
  return cascade;
}

inline std::shared_ptr<grpc::Channel>
Dial(const std::string &address, std::vector<DialOptions> opts = {}) {
  auto trimmed = detail::trim(address);
  if (trimmed.empty()) throw std::invalid_argument("dial address is required");
  if (trimmed.rfind("stdio://", 0) == 0) {
    throw std::invalid_argument("composite::Dial does not support stdio");
  }
  if (trimmed.find("://") != std::string::npos &&
      trimmed.rfind("tcp://", 0) != 0 &&
      trimmed.rfind("unix://", 0) != 0) {
    throw std::invalid_argument("unsupported dial address: " + address);
  }
  auto channel = connect_detail::dial_ready(trimmed, 10000);
  auto desc = detail::describe_ready(channel, std::chrono::milliseconds(10000));
  auto dial_opts = detail::merge_options(opts);
  bool transitive = dial_opts.transitive_observability.value_or(false);
  if (transitive) {
    auto identity = detail::resolve_identity(channel, desc);
    auto relay = std::make_shared<detail::transitive_relay>(
        identity.first, identity.second, channel);
    relay->start();
    detail::remember_dial_relay(std::move(relay));
  }
  return channel;
}

inline std::pair<std::vector<ChildSpec>, std::vector<std::string>>
ParseChildFlags(const std::vector<std::string> &args) {
  std::vector<ChildSpec> children;
  std::vector<std::string> remaining;
  for (size_t i = 0; i < args.size(); ++i) {
    std::string raw;
    if (args[i] == "--child") {
      if (i + 1 >= args.size()) {
        throw std::invalid_argument("--child requires <slug>=<binary>");
      }
      raw = args[++i];
    } else if (args[i].rfind("--child=", 0) == 0) {
      raw = args[i].substr(8);
    } else {
      remaining.push_back(args[i]);
      continue;
    }
    auto eq = raw.find('=');
    if (eq == std::string::npos || eq == 0 || eq + 1 >= raw.size()) {
      throw std::invalid_argument("--child requires <slug>=<binary>");
    }
    children.push_back({raw.substr(0, eq), raw.substr(eq + 1)});
  }
  return {children, remaining};
}

inline CheckOutcome CheckRelayedLog(LogCheckOptions opts) {
  auto timeout = opts.timeout.count() <= 0 ? std::chrono::milliseconds(3000) : opts.timeout;
  auto interval = opts.poll_interval.count() <= 0 ? std::chrono::milliseconds(100) : opts.poll_interval;
  auto deadline = std::chrono::steady_clock::now() + timeout;
  CheckOutcome last;
  while (true) {
    try {
      auto entries = detail::read_log_entries(opts.channel);
      for (const auto &entry : entries) {
        auto sender = observability::attribute_string(entry, "sender");
        auto uid = observability::attribute_string(entry, "responder_uid");
        if (observability::any_value_string(entry.body) != "tick received" ||
            sender != opts.sender || uid != opts.leaf_uid) {
          continue;
        }
        if (auto evidence = detail::compare_chain(entry.chain, opts.expected_chain);
            !evidence.empty()) {
          last = {false, detail::compact("matching log bad chain: " + evidence)};
        } else {
          return {true, ""};
        }
      }
      last = {false, detail::compact("no relayed tick log sender=" + opts.sender +
                                     " leaf_uid=" + opts.leaf_uid +
                                     " entries=" + std::to_string(entries.size()))};
    } catch (const std::exception &error) {
      last = {false, detail::compact(error.what())};
    }
    if (std::chrono::steady_clock::now() >= deadline) return last;
    std::this_thread::sleep_for(interval);
  }
}

inline CheckOutcome CheckRelayedEvent(EventCheckOptions opts) {
  auto timeout = opts.timeout.count() <= 0 ? std::chrono::milliseconds(3000) : opts.timeout;
  auto interval = opts.poll_interval.count() <= 0 ? std::chrono::milliseconds(100) : opts.poll_interval;
  auto deadline = std::chrono::steady_clock::now() + timeout;
  CheckOutcome last;
  while (true) {
    try {
      auto events = detail::read_event_entries(opts.channel);
      for (const auto &event : events) {
        if (event.event_name != observability::event_name(opts.event_type) ||
            observability::attribute_string(event, "holons.instance_uid") != opts.leaf_uid) {
          continue;
        }
        if (auto evidence = detail::compare_chain(event.chain, opts.expected_chain);
            !evidence.empty()) {
          last = {false, detail::compact("matching event bad chain: " + evidence)};
        } else {
          return {true, ""};
        }
      }
      last = {false, detail::compact("no relayed event leaf_uid=" + opts.leaf_uid +
                                     " events=" + std::to_string(events.size()))};
    } catch (const std::exception &error) {
      last = {false, detail::compact(error.what())};
    }
    if (std::chrono::steady_clock::now() >= deadline) return last;
    std::this_thread::sleep_for(interval);
  }
}

} // namespace holons::composite
