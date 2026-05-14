#include "holons/holons.hpp"
#include "holons/v1/observability.grpc.pb.h"
#include "relay/v1/relay.grpc.pb.h"

#include <fcntl.h>
#include <sys/wait.h>
#include <unistd.h>

#include <atomic>
#include <chrono>
#include <csignal>
#include <cstdio>
#include <cstdlib>
#include <filesystem>
#include <fstream>
#include <iomanip>
#include <iostream>
#include <map>
#include <mutex>
#include <optional>
#include <sstream>
#include <string>
#include <thread>
#include <vector>

using Clock = std::chrono::steady_clock;
using namespace std::chrono_literals;

namespace {

constexpr int kRunPhases = 4;
constexpr int kRunTicks = 3;
const std::vector<std::string> kRoleOrder = {"D", "C", "B", "A"};
const std::vector<std::string> kTransports = {"tcp", "unix", "tcp", "unix"};
constexpr const char *kCSlug = "observability-cascade-node-c";
constexpr const char *kGoSlug = "observability-cascade-node-go";

struct CheckResult {
  bool pass = false;
  std::string evidence;
};

struct TickOutcome {
  CheckResult log;
  CheckResult event;
  CheckResult metric;
  double metric_value = 0;
};

struct RoleSpec {
  std::string slug;
  std::string binary_path;
};

struct RoleRuntime {
  std::string role;
  std::string uid;
  std::string slug;
  std::string binary_path;
  std::string listen_uri;
  std::string relay_address;
  std::string member_address;
  std::string member_slug;
  std::string metrics_addr;
  pid_t pid = -1;
  std::shared_ptr<grpc::Channel> channel;
};

struct MetaJson {
  std::string uid;
  std::string address;
  std::string metrics_addr;
  int pid = 0;
};

std::string lower(std::string value) {
  for (auto &ch : value) {
    ch = static_cast<char>(std::tolower(static_cast<unsigned char>(ch)));
  }
  return value;
}

std::string repo_root_from(const std::filesystem::path &start) {
  auto current = std::filesystem::absolute(start);
  while (!current.empty()) {
    if (std::filesystem::exists(current / "sdk") &&
        std::filesystem::exists(current / "examples")) {
      return current.string();
    }
    auto parent = current.parent_path();
    if (parent == current) break;
    current = parent;
  }
  throw std::runtime_error("repository root not found");
}

std::string relay_root_from(const std::filesystem::path &start) {
  auto current = std::filesystem::absolute(start);
  while (!current.empty()) {
    if (std::filesystem::exists(current / "observability-cascade-node-go") &&
        std::filesystem::exists(current / "observability-cascade-node-c")) {
      return current.string();
    }
    auto nested = current / "examples" / "observability-cascade";
    if (std::filesystem::exists(nested / "observability-cascade-node-go") &&
        std::filesystem::exists(nested / "observability-cascade-node-c")) {
      return nested.string();
    }
    auto parent = current.parent_path();
    if (parent == current) break;
    current = parent;
  }
  throw std::runtime_error("observability-cascade root not found");
}

std::string trim(std::string value) {
  while (!value.empty() && std::isspace(static_cast<unsigned char>(value.front()))) {
    value.erase(value.begin());
  }
  while (!value.empty() && std::isspace(static_cast<unsigned char>(value.back()))) {
    value.pop_back();
  }
  return value;
}

std::string capture_command(const std::string &command) {
  std::array<char, 256> buffer{};
  std::string output;
  FILE *pipe = popen(command.c_str(), "r");
  if (pipe == nullptr) return {};
  while (fgets(buffer.data(), static_cast<int>(buffer.size()), pipe) != nullptr) {
    output += buffer.data();
  }
  int rc = pclose(pipe);
  if (rc != 0) return {};
  return trim(output);
}

std::optional<std::string> find_executable(const std::filesystem::path &root,
                                           const std::string &name) {
  if (!std::filesystem::exists(root)) return std::nullopt;
  for (const auto &entry : std::filesystem::recursive_directory_iterator(root)) {
    if (entry.is_regular_file() && entry.path().filename() == name) {
      return entry.path().string();
    }
  }
  return std::nullopt;
}

std::string find_binary(const std::string &slug, const std::string &relay_root) {
  auto suffix = slug.substr(std::string("observability-cascade-node-").size());
  for (auto &ch : suffix) {
    ch = ch == '-' ? '_' : static_cast<char>(std::toupper(static_cast<unsigned char>(ch)));
  }
  if (const char *env = std::getenv(("OBSERVABILITY_CASCADE_NODE_" + suffix + "_BIN").c_str());
      env != nullptr && *env != '\0') {
    return trim(env);
  }
  auto from_op = capture_command("cd '" + relay_root + "' && op --bin " + slug + " 2>/dev/null");
  if (!from_op.empty()) return from_op;
  const char *home = std::getenv("HOME");
  if (home != nullptr) {
    auto found = find_executable(std::filesystem::path(home) / ".op" / "bin" /
                                     (slug + ".holon") / "bin",
                                 slug);
    if (found) return *found;
  }
  throw std::runtime_error(slug + " binary not found; run op build " + slug + " --install");
}

std::filesystem::path make_temp_dir(const std::string &prefix) {
  auto base = std::filesystem::temp_directory_path() / (prefix + "XXXXXX");
  auto raw = base.string();
  std::vector<char> chars(raw.begin(), raw.end());
  chars.push_back('\0');
  char *created = mkdtemp(chars.data());
  if (created == nullptr) {
    throw std::runtime_error("mkdtemp failed");
  }
  return created;
}

std::string read_file(const std::filesystem::path &path) {
  std::ifstream in(path);
  std::ostringstream out;
  out << in.rdbuf();
  return out.str();
}

MetaJson wait_meta(const std::filesystem::path &run_root,
                   const RoleRuntime &runtime,
                   std::chrono::seconds timeout) {
  auto path = run_root / runtime.slug / runtime.uid / "meta.json";
  auto deadline = Clock::now() + timeout;
  std::string last_error;
  while (Clock::now() < deadline) {
    try {
      if (std::filesystem::exists(path)) {
        auto json = nlohmann::json::parse(read_file(path));
        MetaJson meta;
        meta.uid = json.value("uid", "");
        meta.address = json.value("address", "");
        meta.metrics_addr = json.value("metrics_addr", "");
        meta.pid = json.value("pid", 0);
        if (meta.uid == runtime.uid && meta.pid == runtime.pid &&
            !meta.metrics_addr.empty()) {
          return meta;
        }
      }
    } catch (const std::exception &error) {
      last_error = error.what();
    }
    std::this_thread::sleep_for(50ms);
  }
  throw std::runtime_error("meta not ready for " + runtime.slug + "/" +
                           runtime.uid + ": " + last_error);
}

void redirect_to_devnull() {
  int fd = ::open("/dev/null", O_WRONLY);
  if (fd >= 0) {
    ::dup2(fd, STDOUT_FILENO);
    ::dup2(fd, STDERR_FILENO);
    if (fd > STDERR_FILENO) ::close(fd);
  }
}

pid_t spawn_process(const RoleRuntime &runtime,
                    const std::filesystem::path &run_root,
                    const std::string &repo_root,
                    const std::string &organism_uid,
                    const std::string &organism_slug) {
  pid_t pid = fork();
  if (pid < 0) {
    throw std::runtime_error("fork failed");
  }
  if (pid == 0) {
    ::chdir(repo_root.c_str());
    setenv("OP_OBS", "logs,events,metrics,prom", 1);
    setenv("OP_RUN_DIR", run_root.c_str(), 1);
    setenv("OP_INSTANCE_UID", runtime.uid.c_str(), 1);
    setenv("OP_ORGANISM_UID", organism_uid.c_str(), 1);
    setenv("OP_ORGANISM_SLUG", organism_slug.c_str(), 1);
    setenv("OP_PROM_ADDR", "127.0.0.1:0", 1);
    setenv("HOLONS_PARENT_PID", std::to_string(getppid()).c_str(), 1);
    redirect_to_devnull();

    std::vector<std::string> args = {runtime.binary_path, "serve", "--listen",
                                     runtime.listen_uri};
    if (!runtime.member_address.empty()) {
      args.push_back("--member");
      args.push_back(runtime.member_slug + "=" + runtime.member_address);
    }
    std::vector<char *> argv;
    argv.reserve(args.size() + 1);
    for (auto &arg : args) argv.push_back(arg.data());
    argv.push_back(nullptr);
    execv(runtime.binary_path.c_str(), argv.data());
    _exit(127);
  }
  return pid;
}

std::string child_role(const std::string &role) {
  if (role == "A") return "B";
  if (role == "B") return "C";
  if (role == "C") return "D";
  return "";
}

RoleRuntime new_role_runtime(int phase, const std::string &transport,
                             const std::string &role, const RoleSpec &spec) {
  RoleRuntime runtime;
  runtime.role = role;
  runtime.uid = "relay-p" + (phase < 10 ? std::string("0") : std::string{}) +
                std::to_string(phase) + "-" + lower(role);
  runtime.slug = spec.slug;
  runtime.binary_path = spec.binary_path;
  if (transport == "tcp") {
    runtime.listen_uri = "tcp://127.0.0.1:0";
  } else if (transport == "unix") {
    auto path = "/tmp/observability-cascade-c-p" + std::to_string(phase) + "-" +
                lower(role) + "-" + std::to_string(getpid()) + ".sock";
    std::filesystem::remove(path);
    runtime.listen_uri = "unix://" + path;
    runtime.relay_address = runtime.listen_uri;
  } else {
    throw std::runtime_error("unknown transport " + transport);
  }
  return runtime;
}

std::vector<holons::v1::LogEntry>
read_logs(const std::shared_ptr<grpc::Channel> &channel) {
  auto stub = holons::v1::HolonObservability::NewStub(channel);
  grpc::ClientContext context;
  context.set_deadline(std::chrono::system_clock::now() + 2s);
  holons::v1::LogsRequest request;
  request.set_min_level(holons::v1::INFO);
  auto reader = stub->Logs(&context, request);
  std::vector<holons::v1::LogEntry> entries;
  holons::v1::LogEntry entry;
  while (reader->Read(&entry)) {
    entries.push_back(entry);
  }
  reader->Finish();
  return entries;
}

std::vector<holons::v1::EventInfo>
read_events(const std::shared_ptr<grpc::Channel> &channel) {
  auto stub = holons::v1::HolonObservability::NewStub(channel);
  grpc::ClientContext context;
  context.set_deadline(std::chrono::system_clock::now() + 2s);
  holons::v1::EventsRequest request;
  auto reader = stub->Events(&context, request);
  std::vector<holons::v1::EventInfo> events;
  holons::v1::EventInfo event;
  while (reader->Read(&event)) {
    events.push_back(event);
  }
  reader->Finish();
  return events;
}

std::string http_get(const std::string &url) {
  if (url.rfind("http://", 0) != 0) {
    throw std::runtime_error("unsupported metrics URL " + url);
  }
  auto without_scheme = url.substr(7);
  auto slash = without_scheme.find('/');
  auto host_port = slash == std::string::npos ? without_scheme
                                              : without_scheme.substr(0, slash);
  auto path = slash == std::string::npos ? std::string("/") : without_scheme.substr(slash);
  auto colon = host_port.rfind(':');
  if (colon == std::string::npos) {
    throw std::runtime_error("metrics URL missing port " + url);
  }
  auto host = host_port.substr(0, colon);
  int port = std::stoi(host_port.substr(colon + 1));
  int fd = ::socket(AF_INET, SOCK_STREAM, 0);
  if (fd < 0) throw std::runtime_error("socket failed");
  sockaddr_in addr{};
  addr.sin_family = AF_INET;
  addr.sin_port = htons(static_cast<uint16_t>(port));
  if (::inet_pton(AF_INET, host.c_str(), &addr.sin_addr) != 1 ||
      ::connect(fd, reinterpret_cast<sockaddr *>(&addr), sizeof(addr)) != 0) {
    ::close(fd);
    throw std::runtime_error("connect metrics failed " + url);
  }
  auto request = "GET " + path + " HTTP/1.1\r\nHost: " + host +
                 "\r\nConnection: close\r\n\r\n";
  ::send(fd, request.data(), request.size(), 0);
  std::string response;
  char buffer[4096];
  ssize_t n = 0;
  while ((n = ::recv(fd, buffer, sizeof(buffer), 0)) > 0) {
    response.append(buffer, static_cast<size_t>(n));
  }
  ::close(fd);
  auto split = response.find("\r\n\r\n");
  return split == std::string::npos ? response : response.substr(split + 4);
}

std::optional<double> parse_cascade_ticks(const std::string &body,
                                          const std::string &uid) {
  auto needle = "responder_uid=\"" + uid + "\"";
  std::istringstream in(body);
  std::string line;
  while (std::getline(in, line)) {
    if (line.rfind("cascade_ticks_total{", 0) != 0 ||
        line.find(needle) == std::string::npos) {
      continue;
    }
    std::istringstream parts(line);
    std::string metric;
    double value = 0;
    if (parts >> metric >> value) return value;
  }
  return std::nullopt;
}

std::shared_ptr<grpc::Channel> dial_ready_retry(const std::string &address,
                                                std::chrono::milliseconds timeout) {
  auto deadline = Clock::now() + timeout;
  std::string last_error;
  while (Clock::now() < deadline) {
    try {
      return holons::connect_detail::dial_ready(address, 1000);
    } catch (const std::exception &error) {
      last_error = error.what();
      std::this_thread::sleep_for(50ms);
    }
  }
  throw std::runtime_error(last_error.empty() ? "dial timeout " + address : last_error);
}

CheckResult wait_for(std::chrono::milliseconds timeout,
                     const std::function<CheckResult()> &fn,
                     std::chrono::milliseconds interval) {
  auto deadline = Clock::now() + timeout;
  CheckResult last;
  while (true) {
    last = fn();
    if (last.pass || Clock::now() > deadline) return last;
    std::this_thread::sleep_for(interval);
  }
}

std::string pass_text(bool value) { return value ? "PASS" : "FAIL"; }

std::string elapsed(Clock::time_point start) {
  auto ms = std::chrono::duration_cast<std::chrono::milliseconds>(Clock::now() - start).count();
  if (ms < 1000) return std::to_string(ms) + "ms";
  std::ostringstream out;
  out << std::fixed << std::setprecision(1) << (static_cast<double>(ms) / 1000.0) << "s";
  return out.str();
}

void print_failure_evidence(const std::string &family, const CheckResult &result) {
  if (!result.pass) {
    std::cout << "    " << family << " evidence: "
              << (result.evidence.empty() ? "<empty>" : result.evidence) << "\n";
  }
}

class LiveStreams {
public:
  explicit LiveStreams(std::string address) : address_(std::move(address)) {}

  void start() {
    channel_ = dial_ready_retry(address_, 10000ms);
    log_context_ = std::make_shared<grpc::ClientContext>();
    event_context_ = std::make_shared<grpc::ClientContext>();
    log_thread_ = std::thread([this]() { read_log_stream(); });
    event_thread_ = std::thread([this]() { read_event_stream(); });
  }

  void stop() {
    stopped_.store(true);
    if (log_context_) log_context_->TryCancel();
    if (event_context_) event_context_->TryCancel();
    holons::connect_detail::join_thread(&log_thread_);
    holons::connect_detail::join_thread(&event_thread_);
  }

  ~LiveStreams() { stop(); }

  std::vector<holons::v1::LogEntry> log_entries() const {
    std::scoped_lock lk(mu_);
    return logs_;
  }

  std::vector<holons::v1::EventInfo> event_entries() const {
    std::scoped_lock lk(mu_);
    return events_;
  }

  std::vector<std::string> errors() const {
    std::scoped_lock lk(mu_);
    return errors_;
  }

private:
  void read_log_stream() {
    try {
      auto stub = holons::v1::HolonObservability::NewStub(channel_);
      holons::v1::LogsRequest request;
      request.set_min_level(holons::v1::INFO);
      request.set_follow(true);
      auto reader = stub->Logs(log_context_.get(), request);
      holons::v1::LogEntry entry;
      while (!stopped_.load() && reader->Read(&entry)) {
        std::scoped_lock lk(mu_);
        logs_.push_back(entry);
      }
      reader->Finish();
    } catch (const std::exception &error) {
      std::scoped_lock lk(mu_);
      errors_.push_back(std::string("logs stream ended: ") + error.what());
    }
  }

  void read_event_stream() {
    try {
      auto stub = holons::v1::HolonObservability::NewStub(channel_);
      holons::v1::EventsRequest request;
      request.set_follow(true);
      auto reader = stub->Events(event_context_.get(), request);
      holons::v1::EventInfo event;
      while (!stopped_.load() && reader->Read(&event)) {
        std::scoped_lock lk(mu_);
        events_.push_back(event);
      }
      reader->Finish();
    } catch (const std::exception &error) {
      std::scoped_lock lk(mu_);
      errors_.push_back(std::string("events stream ended: ") + error.what());
    }
  }

  std::string address_;
  std::shared_ptr<grpc::Channel> channel_;
  std::shared_ptr<grpc::ClientContext> log_context_;
  std::shared_ptr<grpc::ClientContext> event_context_;
  std::atomic<bool> stopped_{false};
  mutable std::mutex mu_;
  std::vector<holons::v1::LogEntry> logs_;
  std::vector<holons::v1::EventInfo> events_;
  std::vector<std::string> errors_;
  std::thread log_thread_;
  std::thread event_thread_;
};

class Cascade {
public:
  Cascade() = default;
  Cascade(const Cascade &) = delete;
  Cascade &operator=(const Cascade &) = delete;
  Cascade(Cascade &&other) noexcept
      : phase(other.phase),
        transport(std::move(other.transport)),
        run_root(std::move(other.run_root)),
        roles(std::move(other.roles)),
        owns_processes(other.owns_processes) {
    other.owns_processes = false;
  }
  Cascade &operator=(Cascade &&other) noexcept {
    if (this == &other) return *this;
    stop();
    phase = other.phase;
    transport = std::move(other.transport);
    run_root = std::move(other.run_root);
    roles = std::move(other.roles);
    owns_processes = other.owns_processes;
    other.owns_processes = false;
    return *this;
  }

  int phase = 0;
  std::string transport;
  std::filesystem::path run_root;
  std::map<std::string, RoleRuntime> roles;

  void stop() {
    if (!owns_processes) return;
    for (const auto &role : {"A", "B", "C", "D"}) {
      auto it = roles.find(role);
      if (it == roles.end()) continue;
      if (it->second.pid > 0) {
        kill(it->second.pid, SIGTERM);
      }
    }
    for (const auto &role : {"A", "B", "C", "D"}) {
      auto it = roles.find(role);
      if (it == roles.end() || it->second.pid <= 0) continue;
      int status = 0;
      auto deadline = Clock::now() + 3s;
      while (Clock::now() < deadline) {
        pid_t done = waitpid(it->second.pid, &status, WNOHANG);
        if (done == it->second.pid) break;
        std::this_thread::sleep_for(50ms);
      }
      if (waitpid(it->second.pid, &status, WNOHANG) == 0) {
        kill(it->second.pid, SIGKILL);
        waitpid(it->second.pid, &status, 0);
      }
      it->second.pid = -1;
    }
    owns_processes = false;
  }

  ~Cascade() { stop(); }

  std::string check_chain(const google::protobuf::RepeatedPtrField<holons::v1::ChainHop> &chain) const {
    const std::vector<std::string> expected = {"D", "C", "B"};
    for (size_t i = 0; i < expected.size(); ++i) {
      if (static_cast<int>(i) >= chain.size()) {
        return "chain length " + std::to_string(chain.size()) + " < 3";
      }
      const auto &want = roles.at(expected[i]);
      const auto &hop = chain.Get(static_cast<int>(i));
      if (hop.slug() != want.slug || hop.instance_uid() != want.uid) {
        return "hop " + std::to_string(i) + " = " + hop.slug() + "/" +
               hop.instance_uid() + ", want " + want.slug + "/" + want.uid;
      }
    }
    return {};
  }

  CheckResult check_log(const std::string &sender) const {
    auto entries = read_logs(roles.at("A").channel);
    for (const auto &entry : entries) {
      auto fields = entry.fields();
      auto sender_it = fields.find("sender");
      auto uid_it = fields.find("responder_uid");
      if (entry.message() != "tick received" || sender_it == fields.end() ||
          sender_it->second != sender || uid_it == fields.end() ||
          uid_it->second != roles.at("D").uid) {
        continue;
      }
      auto err = check_chain(entry.chain());
      return {err.empty(), err.empty() ? entry.ShortDebugString()
                                       : "matching log has bad chain: " + err +
                                             " entry=" + entry.ShortDebugString()};
    }
    return {false, "no relayed D tick log for sender=" + sender + " in " +
                       std::to_string(entries.size()) + " A log entries"};
  }

  CheckResult check_event() const {
    auto events = read_events(roles.at("A").channel);
    for (const auto &event : events) {
      if (event.type() != holons::v1::INSTANCE_READY ||
          event.instance_uid() != roles.at("D").uid) {
        continue;
      }
      auto err = check_chain(event.chain());
      return {err.empty(), err.empty() ? event.ShortDebugString()
                                       : "matching event has bad chain: " + err +
                                             " event=" + event.ShortDebugString()};
    }
    return {false, "no relayed D INSTANCE_READY event in " +
                       std::to_string(events.size()) + " A events"};
  }

  CheckResult check_live_log(const LiveStreams &streams, const std::string &sender) const {
    auto entries = streams.log_entries();
    for (const auto &entry : entries) {
      auto fields = entry.fields();
      auto sender_it = fields.find("sender");
      auto uid_it = fields.find("responder_uid");
      if (entry.message() != "tick received" || sender_it == fields.end() ||
          sender_it->second != sender || uid_it == fields.end() ||
          uid_it->second != roles.at("D").uid) {
        continue;
      }
      auto err = check_chain(entry.chain());
      return {err.empty(), err.empty() ? entry.ShortDebugString()
                                       : "matching live log has bad chain: " + err +
                                             " entry=" + entry.ShortDebugString()};
    }
    return {false, "no live log found for sender=" + sender +
                       " buffer=" + std::to_string(entries.size())};
  }

  CheckResult check_live_event(const LiveStreams &streams) const {
    auto events = streams.event_entries();
    for (const auto &event : events) {
      if (event.type() != holons::v1::INSTANCE_READY ||
          event.instance_uid() != roles.at("D").uid) {
        continue;
      }
      auto err = check_chain(event.chain());
      return {err.empty(), err.empty() ? event.ShortDebugString()
                                       : "matching live event has bad chain: " + err +
                                             " event=" + event.ShortDebugString()};
    }
    return {false, "no live INSTANCE_READY event for D buffer=" +
                       std::to_string(events.size())};
  }

  CheckResult check_metric(double previous, double *value_out) const {
    try {
      auto body = http_get(roles.at("D").metrics_addr);
      auto value = parse_cascade_ticks(body, roles.at("D").uid);
      if (!value) return {false, body};
      *value_out = *value;
      if (*value <= previous) {
        return {false, "cascade_ticks_total=" + std::to_string(*value) +
                           " did not increase beyond " + std::to_string(previous)};
      }
    } catch (const std::exception &error) {
      return {false, error.what()};
    }
    return {true, "cascade_ticks_total=" + std::to_string(*value_out)};
  }

  TickOutcome run_tick_with_sender(const std::string &sender, double previous_metric) const {
    relay::v1::TickRequest request;
    request.set_sender(sender);
    request.set_note(transport);
    relay::v1::TickResponse response;
    auto stub = relay::v1::RelayService::NewStub(roles.at("D").channel);
    grpc::ClientContext context;
    context.set_deadline(std::chrono::system_clock::now() + 5s);
    auto status = stub->Tick(&context, request, &response);
    if (!status.ok()) {
      auto failed = CheckResult{false, status.error_message()};
      return {failed, failed, failed, previous_metric};
    }
    auto log = wait_for(3000ms, [&]() { return check_log(sender); }, 100ms);
    auto event = wait_for(3000ms, [&]() { return check_event(); }, 100ms);
    double metric_value = previous_metric;
    auto metric = wait_for(3000ms, [&]() { return check_metric(previous_metric, &metric_value); }, 100ms);
    return {log, event, metric, metric_value};
  }

  TickOutcome run_live_tick_with_sender(const LiveStreams *streams,
                                        const std::string &stream_error,
                                        const std::string &sender,
                                        double previous_metric) const {
    relay::v1::TickRequest request;
    request.set_sender(sender);
    request.set_note(transport);
    relay::v1::TickResponse response;
    auto stub = relay::v1::RelayService::NewStub(roles.at("D").channel);
    grpc::ClientContext context;
    context.set_deadline(std::chrono::system_clock::now() + 5s);
    auto status = stub->Tick(&context, request, &response);
    if (!status.ok()) {
      auto failed = CheckResult{false, status.error_message()};
      return {failed, failed, failed, previous_metric};
    }
    CheckResult log;
    CheckResult event;
    if (streams != nullptr && stream_error.empty()) {
      log = wait_for(1000ms, [&]() { return check_live_log(*streams, sender); }, 50ms);
      event = wait_for(1000ms, [&]() { return check_live_event(*streams); }, 50ms);
    } else {
      log = {false, "stream re-open failed: " + stream_error};
      event = log;
    }
    double metric_value = previous_metric;
    auto metric = wait_for(1000ms, [&]() { return check_metric(previous_metric, &metric_value); }, 50ms);
    return {log, event, metric, metric_value};
  }

private:
  bool owns_processes = true;
};

Cascade spawn_cascade(int phase, const std::string &transport,
                      const std::map<std::string, RoleSpec> &specs,
                      const std::filesystem::path &run_root,
                      const std::string &repo_root) {
  Cascade cascade;
  cascade.phase = phase;
  cascade.transport = transport;
  cascade.run_root = run_root;
  for (const auto &role : kRoleOrder) {
    cascade.roles[role] = new_role_runtime(phase, transport, role, specs.at(role));
  }
  for (const auto &role : kRoleOrder) {
    auto &runtime = cascade.roles[role];
    auto child = child_role(role);
    if (!child.empty()) {
      runtime.member_address = cascade.roles[child].relay_address;
      runtime.member_slug = cascade.roles[child].slug;
    }
    runtime.pid = spawn_process(runtime, run_root, repo_root,
                                cascade.roles["A"].uid, cascade.roles["A"].slug);
    auto meta = wait_meta(run_root, runtime, 15s);
    runtime.metrics_addr = meta.metrics_addr;
    runtime.relay_address = meta.address;
    try {
      runtime.channel = dial_ready_retry(runtime.relay_address, 10000ms);
    } catch (const std::exception &error) {
      throw std::runtime_error("dial " + role + " at " + runtime.relay_address +
                               ": " + error.what());
    }
  }
  auto relay_ready = wait_for(5000ms, [&]() { return cascade.check_event(); }, 50ms);
  if (!relay_ready.pass) {
    throw std::runtime_error("relay readiness failed: " + relay_ready.evidence);
  }
  return cascade;
}

std::map<std::string, RoleSpec> all_specs(const std::string &slug, const std::string &binary) {
  std::map<std::string, RoleSpec> specs;
  for (const auto &role : kRoleOrder) specs[role] = {slug, binary};
  return specs;
}

void run_default(const std::string &c_binary, const std::string &repo_root) {
  auto run_root = make_temp_dir("observability-cascade-c-");
  std::cout << "=== observability-cascade-c ===\n\n";
  int pass = 0, fail = 0;
  std::string previous;
  for (size_t index = 0; index < kTransports.size(); ++index) {
    int phase = static_cast<int>(index + 1);
    const auto &transport = kTransports[index];
    std::cout << "Phase " << phase << "/" << kRunPhases << ": transport=" << transport;
    if (!previous.empty()) std::cout << " (switching from " << previous << ")";
    std::cout << "\n";
    auto started = Clock::now();
    std::optional<Cascade> cascade;
    try {
      cascade.emplace(spawn_cascade(phase, transport, all_specs(kCSlug, c_binary),
                                    run_root, repo_root));
      std::cout << "  spawned 4 nodes in " << elapsed(started) << "\n";
    } catch (const std::exception &error) {
      fail += kRunTicks;
      std::cout << "  spawn FAIL: " << error.what() << "\n\n";
      previous = transport;
      continue;
    }
    double previous_metric = 0;
    for (int tick = 1; tick <= kRunTicks; ++tick) {
      auto tick_start = Clock::now();
      auto outcome = cascade->run_tick_with_sender(
          "phase-" + std::to_string(phase) + "-tick-" + std::to_string(tick),
          previous_metric);
      if (outcome.metric.pass) previous_metric = outcome.metric_value;
      bool overall = outcome.log.pass && outcome.event.pass && outcome.metric.pass;
      overall ? ++pass : ++fail;
      std::cout << "  Tick " << tick << "/" << kRunTicks << ": log "
                << pass_text(outcome.log.pass) << ", event "
                << pass_text(outcome.event.pass) << ", metric "
                << pass_text(outcome.metric.pass) << " (overall "
                << pass_text(overall) << " in " << elapsed(tick_start) << ")\n";
      print_failure_evidence("log", outcome.log);
      print_failure_evidence("event", outcome.event);
      print_failure_evidence("metric", outcome.metric);
    }
    cascade->stop();
    std::cout << "\n";
    previous = transport;
  }
  std::filesystem::remove_all(run_root);
  std::cout << "Summary: " << pass + fail << " ticks, " << pass << " PASS, "
            << fail << " FAIL\n";
  if (fail > 0) throw std::runtime_error(std::to_string(fail) + " tick(s) failed");
}

void run_live_stream(const std::string &c_binary, const std::string &repo_root) {
  auto run_root = make_temp_dir("observability-cascade-c-live-");
  std::cout << "=== observability-cascade-c --live-stream ===\n\n";
  std::cout << "Setup: opening long-lived Follow:true streams on A\n";
  std::cout << "       (initial transport: tcp)\n\n";
  int pass = 0, fail = 0;
  std::optional<Cascade> cascade;
  std::unique_ptr<LiveStreams> streams;
  for (size_t index = 0; index < kTransports.size(); ++index) {
    int phase = static_cast<int>(index + 1);
    const auto &transport = kTransports[index];
    if (phase == 1) {
      std::cout << "Phase " << phase << "/" << kRunPhases << ": initial chain (" << transport << ")\n";
    } else {
      std::cout << "Phase " << phase << "/" << kRunPhases << ": respawn on " << transport << "\n";
      auto kill_start = Clock::now();
      streams.reset();
      if (cascade) cascade->stop();
      std::cout << "  killed 4 nodes in " << elapsed(kill_start) << "\n";
    }
    auto spawn_start = Clock::now();
    try {
      cascade.emplace(spawn_cascade(phase, transport, all_specs(kCSlug, c_binary),
                                    run_root, repo_root));
      std::cout << "  spawned 4 nodes in " << elapsed(spawn_start) << "\n";
    } catch (const std::exception &error) {
      fail += kRunTicks;
      std::cout << "  spawn FAIL: " << error.what() << "\n\n";
      continue;
    }
    if (phase > 1) std::cout << "  re-opening Follow:true streams on new A\n";
    std::string stream_error;
    try {
      streams = std::make_unique<LiveStreams>(cascade->roles["A"].relay_address);
      streams->start();
    } catch (const std::exception &error) {
      stream_error = error.what();
      streams.reset();
      std::cout << "  stream re-open failed: " << stream_error << "\n";
    }
    double previous_metric = 0;
    for (int tick = 1; tick <= kRunTicks; ++tick) {
      auto tick_start = Clock::now();
      auto outcome = cascade->run_live_tick_with_sender(
          streams.get(), stream_error,
          "phase-" + std::to_string(phase) + "-tick-" + std::to_string(tick),
          previous_metric);
      if (outcome.metric.pass) previous_metric = outcome.metric_value;
      bool overall = outcome.log.pass && outcome.event.pass && outcome.metric.pass;
      overall ? ++pass : ++fail;
      std::cout << "  Tick " << tick << "/" << kRunTicks << ": log "
                << pass_text(outcome.log.pass) << ", event "
                << pass_text(outcome.event.pass) << ", metric "
                << pass_text(outcome.metric.pass) << " (overall "
                << pass_text(overall) << " in " << elapsed(tick_start) << ")\n";
      print_failure_evidence("log", outcome.log);
      print_failure_evidence("event", outcome.event);
      print_failure_evidence("metric", outcome.metric);
    }
    std::cout << "\n";
  }
  streams.reset();
  if (cascade) cascade->stop();
  std::filesystem::remove_all(run_root);
  std::cout << "Summary: " << pass << " PASS / " << fail << " FAIL across "
            << pass + fail << " ticks\n";
  if (fail > 0) throw std::runtime_error(std::to_string(fail) + " tick(s) failed");
}

void run_multi_pattern(const std::string &c_binary, const std::string &go_binary,
                       const std::string &repo_root) {
  struct Pattern {
    std::string name;
    std::map<std::string, RoleSpec> roles;
  };
  std::vector<Pattern> patterns = {
      {"c-c-c-c", all_specs(kCSlug, c_binary)},
      {"c-c-go-c",
       {{"A", {kCSlug, c_binary}}, {"B", {kCSlug, c_binary}},
        {"C", {kGoSlug, go_binary}}, {"D", {kCSlug, c_binary}}}},
      {"c-c-go-go",
       {{"A", {kCSlug, c_binary}}, {"B", {kCSlug, c_binary}},
        {"C", {kGoSlug, go_binary}}, {"D", {kGoSlug, go_binary}}}},
  };
  auto run_root = make_temp_dir("observability-cascade-c-multi-");
  std::cout << "=== observability-cascade-c (multi-pattern) ===\n\n";
  int pass = 0, fail = 0;
  for (size_t pattern_index = 0; pattern_index < patterns.size(); ++pattern_index) {
    const auto &pattern = patterns[pattern_index];
    std::cout << "Pattern " << pattern_index + 1 << "/" << patterns.size()
              << ": " << pattern.name << "\n";
    int pattern_pass = 0;
    for (size_t index = 0; index < kTransports.size(); ++index) {
      int phase = static_cast<int>(index + 1);
      const auto &transport = kTransports[index];
      auto started = Clock::now();
      std::optional<Cascade> cascade;
      try {
        cascade.emplace(spawn_cascade(phase, transport, pattern.roles,
                                      run_root, repo_root));
      } catch (const std::exception &error) {
        fail += kRunTicks;
        std::cout << "  Phase " << phase << "/" << kRunPhases << " ("
                  << transport << "): spawn FAIL (" << error.what() << ")\n";
        continue;
      }
      std::unique_ptr<LiveStreams> streams;
      std::string stream_error;
      try {
        streams = std::make_unique<LiveStreams>(cascade->roles["A"].relay_address);
        streams->start();
        auto ready = wait_for(5000ms, [&]() { return cascade->check_live_event(*streams); }, 50ms);
        if (!ready.pass) stream_error = "live relay readiness: " + ready.evidence;
      } catch (const std::exception &error) {
        stream_error = error.what();
      }
      double previous_metric = 0;
      std::vector<std::string> results;
      std::vector<std::string> evidence;
      for (int tick = 1; tick <= kRunTicks; ++tick) {
        auto sender = pattern.name + "-phase-" + std::to_string(phase) +
                      "-tick-" + std::to_string(tick);
        auto outcome = cascade->run_live_tick_with_sender(
            streams.get(), stream_error, sender, previous_metric);
        if (outcome.metric.pass) previous_metric = outcome.metric_value;
        bool overall = outcome.log.pass && outcome.event.pass && outcome.metric.pass;
        if (overall) {
          ++pass;
          ++pattern_pass;
          results.push_back("tick " + std::to_string(tick) + " PASS");
        } else {
          ++fail;
          results.push_back("tick " + std::to_string(tick) + " FAIL");
          if (!outcome.log.pass) evidence.push_back("log=" + outcome.log.evidence);
          if (!outcome.event.pass) evidence.push_back("event=" + outcome.event.evidence);
          if (!outcome.metric.pass) evidence.push_back("metric=" + outcome.metric.evidence);
        }
      }
      streams.reset();
      cascade->stop();
      std::cout << "  Phase " << phase << "/" << kRunPhases << " (" << transport
                << "): ";
      for (size_t i = 0; i < results.size(); ++i) {
        if (i) std::cout << ", ";
        std::cout << results[i];
      }
      std::cout << " (spawned in " << elapsed(started) << ")\n";
      if (!evidence.empty()) {
        std::cout << "    evidence: ";
        for (size_t i = 0; i < evidence.size(); ++i) {
          if (i) std::cout << " | ";
          std::cout << evidence[i];
        }
        std::cout << "\n";
      }
    }
    std::cout << "Pattern summary: " << pattern_pass << " PASS / "
              << (kRunPhases * kRunTicks - pattern_pass) << " FAIL\n\n";
  }
  std::filesystem::remove_all(run_root);
  std::cout << "Summary: " << pass << " PASS / " << fail << " FAIL across "
            << pass + fail << " ticks\n";
  if (fail > 0) throw std::runtime_error(std::to_string(fail) + " tick(s) failed");
}

} // namespace

int main(int argc, char **argv) {
  try {
    bool live_stream = false;
    bool multi_pattern = false;
    for (int i = 1; i < argc; ++i) {
      std::string arg = argv[i];
      if (arg == "--live-stream") live_stream = true;
      if (arg == "--multi-pattern") multi_pattern = true;
    }
    auto cwd = std::filesystem::current_path();
    auto relay_root = relay_root_from(cwd);
    auto repo_root = repo_root_from(relay_root);
    auto c_binary = find_binary(kCSlug, relay_root);
    if (multi_pattern) {
      auto go_binary = find_binary(kGoSlug, relay_root);
      run_multi_pattern(c_binary, go_binary, repo_root);
    } else if (live_stream) {
      run_live_stream(c_binary, repo_root);
    } else {
      run_default(c_binary, repo_root);
    }
  } catch (const std::exception &error) {
    std::cerr << "\nFAIL: " << error.what() << "\n";
    return 1;
  }
  return 0;
}
