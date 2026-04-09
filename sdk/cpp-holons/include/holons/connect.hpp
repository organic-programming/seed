#pragma once

#include "discover.hpp"

namespace holons {

struct ConnectResult {
  void *channel = nullptr;
  std::string uid;
  std::optional<HolonRef> origin;
  std::optional<std::string> error;
};

namespace uniform_connect_detail {

struct launch_target {
  std::vector<std::string> command;
  std::filesystem::path cwd;
};

struct channel_owner {
  std::shared_ptr<grpc::Channel> channel;
  std::shared_ptr<connect_detail::process_handle> process;
  bool ephemeral = false;
  std::string target;
};

inline bool is_executable_file(const std::filesystem::path &path) {
  std::error_code ec;
  if (!std::filesystem::exists(path, ec) ||
      !std::filesystem::is_regular_file(path, ec)) {
    return false;
  }
#ifdef _WIN32
  return true;
#else
  return ::access(path.c_str(), X_OK) == 0;
#endif
}

inline std::string default_transport_for_platform() {
#if defined(_WIN32) || !HOLONS_HAS_GRPC_FD
  return "tcp";
#else
  return "stdio";
#endif
}

inline std::string preferred_transport(const HolonInfo &info) {
  auto transport = connect_detail::lower_copy(trim_copy(info.transport));
  if (transport.empty()) {
    return default_transport_for_platform();
  }
  if (transport == "stdio") {
#if HOLONS_HAS_GRPC_FD
    return "stdio";
#else
    return "tcp";
#endif
  }
  if (transport == "tcp") {
    return "tcp";
  }
  return default_transport_for_platform();
}

inline std::optional<launch_target>
launch_target_for_runner(const std::string &runner,
                         const std::filesystem::path &entrypoint,
                         const std::filesystem::path &cwd) {
  auto runner_name = connect_detail::lower_copy(trim_copy(runner));
  if (runner_name.empty()) {
    if (is_executable_file(entrypoint)) {
      return launch_target{{entrypoint.string()}, cwd};
    }
    return std::nullopt;
  }

  if (runner_name == "go" || runner_name == "go-module") {
    return launch_target{{"go", "run", entrypoint.string()}, cwd};
  }
  if (runner_name == "python") {
#ifdef _WIN32
    return launch_target{{"python", entrypoint.string()}, cwd};
#else
    return launch_target{{"python3", entrypoint.string()}, cwd};
#endif
  }
  if (runner_name == "node" || runner_name == "typescript" ||
      runner_name == "npm") {
    return launch_target{{"node", entrypoint.string()}, cwd};
  }
  if (runner_name == "ruby") {
    return launch_target{{"ruby", entrypoint.string()}, cwd};
  }
  if (runner_name == "dart") {
    return launch_target{{"dart", "run", entrypoint.string()}, cwd};
  }
  if (runner_name == "shell" || runner_name == "sh" || runner_name == "bash") {
    if (is_executable_file(entrypoint)) {
      return launch_target{{entrypoint.string()}, cwd};
    }
#ifdef _WIN32
    return launch_target{{"bash", entrypoint.string()}, cwd};
#else
    return launch_target{{"bash", entrypoint.string()}, cwd};
#endif
  }

  if (is_executable_file(entrypoint)) {
    return launch_target{{entrypoint.string()}, cwd};
  }
  return std::nullopt;
}

inline std::optional<launch_target>
source_launch_target(const std::filesystem::path &source_dir,
                     const HolonInfo &info) {
  auto entrypoint = trim_copy(info.entrypoint.empty() ? info.slug : info.entrypoint);
  if (entrypoint.empty()) {
    return std::nullopt;
  }

  std::filesystem::path entry_path(entrypoint);
  if (entry_path.is_absolute() && is_executable_file(entry_path)) {
    return launch_target{{entry_path.string()}, {}};
  }

  auto filename = entry_path.filename();
  auto packaged_binary = source_dir / ".op" / "build" /
                         (info.slug + ".holon") / "bin" /
                         discovery_detail::current_arch_dir() / filename;
  if (is_executable_file(packaged_binary)) {
    return launch_target{{packaged_binary.string()}, {}};
  }

  auto built_binary = source_dir / ".op" / "build" / "bin" / filename;
  if (is_executable_file(built_binary)) {
    return launch_target{{built_binary.string()}, {}};
  }

  auto direct_entry = source_dir / entrypoint;
  if (auto target =
          launch_target_for_runner(info.runner, direct_entry, source_dir);
      target.has_value()) {
    return target;
  }

  return std::nullopt;
}

inline std::optional<launch_target>
package_launch_target(const std::filesystem::path &package_dir,
                      const HolonInfo &info) {
  auto entrypoint = trim_copy(info.entrypoint.empty() ? info.slug : info.entrypoint);
  if (entrypoint.empty()) {
    return std::nullopt;
  }

  auto filename = std::filesystem::path(entrypoint).filename();
  auto binary_path = package_dir / "bin" / discovery_detail::current_arch_dir() /
                     filename;
  if (is_executable_file(binary_path)) {
    return launch_target{{binary_path.string()}, {}};
  }

  auto dist_entry = package_dir / "dist" / entrypoint;
  if (auto target =
          launch_target_for_runner(info.runner, dist_entry, package_dir);
      target.has_value()) {
    return target;
  }

  auto git_root = package_dir / "git";
  std::error_code ec;
  if (std::filesystem::exists(git_root, ec) &&
      std::filesystem::is_directory(git_root, ec)) {
    return source_launch_target(git_root, info);
  }

  return std::nullopt;
}

inline launch_target launch_target_from_ref(const HolonRef &ref) {
  if (!ref.info.has_value()) {
    throw std::runtime_error("holon metadata unavailable");
  }

  auto path = discovery_detail::path_from_file_url(ref.url);
  std::error_code ec;
  if (std::filesystem::exists(path, ec) &&
      std::filesystem::is_regular_file(path, ec)) {
    return launch_target{{path.string()}, {}};
  }

  if (!std::filesystem::exists(path, ec) || !std::filesystem::is_directory(path, ec)) {
    throw std::runtime_error("target path \"" + path.string() +
                             "\" is not launchable");
  }

  auto dirname = path.filename().string();
  if (dirname.size() >= 6 && dirname.substr(dirname.size() - 6) == ".holon") {
    if (auto target = package_launch_target(path, *ref.info); target.has_value()) {
      return *target;
    }
  }

  if (auto target = source_launch_target(path, *ref.info); target.has_value()) {
    return *target;
  }

  throw std::runtime_error("target unreachable");
}

#ifdef _WIN32
inline connect_detail::startup_result
start_command_stdio(const launch_target &target, int timeout_ms) {
  if (target.command.empty()) {
    throw std::runtime_error("launch command is required");
  }
  if (target.command.size() != 1 || !target.cwd.empty()) {
    throw std::runtime_error("runner launches over stdio are unavailable on Windows");
  }
  return connect_detail::start_stdio_holon(target.command.front(), timeout_ms);
}

inline connect_detail::startup_result
start_command_tcp(const launch_target &target, int timeout_ms) {
  if (target.command.empty()) {
    throw std::runtime_error("launch command is required");
  }
  if (target.command.size() != 1 || !target.cwd.empty()) {
    throw std::runtime_error("runner launches over tcp are unavailable on Windows");
  }
  return connect_detail::start_tcp_holon(target.command.front(), timeout_ms);
}
#else
inline std::vector<char *> build_exec_argv(const std::vector<std::string> &command) {
  std::vector<char *> argv;
  argv.reserve(command.size() + 1);
  for (const auto &part : command) {
    argv.push_back(const_cast<char *>(part.c_str()));
  }
  argv.push_back(nullptr);
  return argv;
}

inline void export_parent_pid_env() {
  auto parent = ::getppid();
  if (parent <= 1) {
    return;
  }
  auto value = std::to_string(parent);
  ::setenv("HOLONS_PARENT_PID", value.c_str(), 1);
}

inline connect_detail::startup_result
start_command_tcp(const launch_target &target, int timeout_ms) {
  if (target.command.empty()) {
    throw std::runtime_error("launch command is required");
  }
  if (target.command.size() == 1 && target.cwd.empty()) {
    return connect_detail::start_tcp_holon(target.command.front(), timeout_ms);
  }

  int stdout_pipe[2] = {-1, -1};
  int stderr_pipe[2] = {-1, -1};
  if (::pipe(stdout_pipe) != 0 || ::pipe(stderr_pipe) != 0) {
    connect_detail::close_pipe_fd(&stdout_pipe[0]);
    connect_detail::close_pipe_fd(&stdout_pipe[1]);
    connect_detail::close_pipe_fd(&stderr_pipe[0]);
    connect_detail::close_pipe_fd(&stderr_pipe[1]);
    throw std::runtime_error("pipe() failed");
  }

  auto cleanup = [&]() {
    connect_detail::close_pipe_fd(&stdout_pipe[0]);
    connect_detail::close_pipe_fd(&stdout_pipe[1]);
    connect_detail::close_pipe_fd(&stderr_pipe[0]);
    connect_detail::close_pipe_fd(&stderr_pipe[1]);
  };

  auto pid = ::fork();
  if (pid < 0) {
    cleanup();
    throw std::runtime_error("fork() failed");
  }

  if (pid == 0) {
    ::dup2(stdout_pipe[1], STDOUT_FILENO);
    ::dup2(stderr_pipe[1], STDERR_FILENO);
    cleanup();

    if (!target.cwd.empty()) {
      ::chdir(target.cwd.c_str());
    }

    auto command = target.command;
    command.push_back("serve");
    command.push_back("--listen");
    command.push_back("tcp://127.0.0.1:0");
    export_parent_pid_env();
    auto argv = build_exec_argv(command);
    ::execvp(command.front().c_str(), argv.data());
    std::perror("execvp");
    ::_exit(127);
  }

  connect_detail::close_pipe_fd(&stdout_pipe[1]);
  connect_detail::close_pipe_fd(&stderr_pipe[1]);

  auto process = std::make_shared<connect_detail::process_handle>();
  process->pid = pid;

  std::string stdout_buffer;
  std::string stderr_buffer;
  std::string capture;
  auto deadline = std::chrono::steady_clock::now() +
                  std::chrono::milliseconds(std::max(timeout_ms, 1));

  while (std::chrono::steady_clock::now() < deadline) {
    if (auto uri =
            connect_detail::drain_startup_buffer(&stdout_buffer, &capture, false);
        uri.has_value()) {
      connect_detail::close_pipe_fd(&stdout_pipe[0]);
      connect_detail::close_pipe_fd(&stderr_pipe[0]);
      return connect_detail::startup_result{*uri, -1, process};
    }
    if (auto uri =
            connect_detail::drain_startup_buffer(&stderr_buffer, &capture, false);
        uri.has_value()) {
      connect_detail::close_pipe_fd(&stdout_pipe[0]);
      connect_detail::close_pipe_fd(&stderr_pipe[0]);
      return connect_detail::startup_result{*uri, -1, process};
    }
    if (!connect_detail::process_alive(pid)) {
      int status = 0;
      (void)::waitpid(pid, &status, 0);
      connect_detail::close_pipe_fd(&stdout_pipe[0]);
      connect_detail::close_pipe_fd(&stderr_pipe[0]);
      throw std::runtime_error("holon exited before advertising a listener");
    }
    std::this_thread::sleep_for(std::chrono::milliseconds(25));
  }

  connect_detail::stop_process(process);
  connect_detail::close_pipe_fd(&stdout_pipe[0]);
  connect_detail::close_pipe_fd(&stderr_pipe[0]);
  throw std::runtime_error("timed out waiting for holon startup");
}

inline connect_detail::startup_result
start_command_stdio(const launch_target &target, int timeout_ms) {
  if (target.command.empty()) {
    throw std::runtime_error("launch command is required");
  }
  if (target.command.size() == 1 && target.cwd.empty()) {
    return connect_detail::start_stdio_holon(target.command.front(), timeout_ms);
  }

#if !HOLONS_HAS_GRPC_FD
  return start_command_tcp(target, timeout_ms);
#else
  int transport_pair[2] = {-1, -1};
  int stderr_pipe[2] = {-1, -1};
  int listener_fd = -1;
  std::string proxy_uri;
  if (::socketpair(AF_UNIX, SOCK_STREAM, 0, transport_pair) != 0 ||
      ::pipe(stderr_pipe) != 0) {
    connect_detail::close_pipe_fd(&stderr_pipe[0]);
    connect_detail::close_pipe_fd(&stderr_pipe[1]);
    if (transport_pair[0] >= 0) {
      close_fd(transport_pair[0], true);
    }
    if (transport_pair[1] >= 0) {
      close_fd(transport_pair[1], true);
    }
    throw std::runtime_error("stdio transport setup failed");
  }

  try {
    listener_fd = connect_detail::create_loopback_listener(&proxy_uri);
  } catch (const std::exception &) {
    connect_detail::close_pipe_fd(&stderr_pipe[0]);
    connect_detail::close_pipe_fd(&stderr_pipe[1]);
    if (transport_pair[0] >= 0) {
      close_fd(transport_pair[0], true);
    }
    if (transport_pair[1] >= 0) {
      close_fd(transport_pair[1], true);
    }
    throw;
  }

  auto cleanup = [&]() {
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

  auto pid = ::fork();
  if (pid < 0) {
    cleanup();
    throw std::runtime_error("fork() failed");
  }

  if (pid == 0) {
    ::dup2(transport_pair[1], STDIN_FILENO);
    ::dup2(transport_pair[1], STDOUT_FILENO);
    ::dup2(stderr_pipe[1], STDERR_FILENO);
    cleanup();

    if (!target.cwd.empty()) {
      ::chdir(target.cwd.c_str());
    }

    auto command = target.command;
    command.push_back("serve");
    command.push_back("--listen");
    command.push_back("stdio://");
    export_parent_pid_env();
    auto argv = build_exec_argv(command);
    ::execvp(command.front().c_str(), argv.data());
    std::perror("execvp");
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

  process->accept_thread = std::thread([process, proxy_uri]() {
    sockaddr_storage addr{};
    socklen_t len = sizeof(addr);
    int accepted =
        ::accept(process->listener_fd, reinterpret_cast<sockaddr *>(&addr), &len);
    if (accepted < 0) {
      if (!process->closed) {
        std::lock_guard<std::mutex> lock(process->stderr_mutex);
        process->stderr_capture.append(
            "accept(loopback) failed for stdio:// bridge\n");
      }
      return;
    }
    std::lock_guard<std::mutex> lock(process->client_mutex);
    process->client_fd = accepted;
  });

  process->stderr_thread = std::thread([process]() {
    char buffer[512];
    while (!process->closed) {
      auto n = ::read(process->stderr_fd, buffer, sizeof(buffer));
      if (n <= 0) {
        return;
      }
      std::lock_guard<std::mutex> lock(process->stderr_mutex);
      process->stderr_capture.append(buffer, static_cast<size_t>(n));
    }
  });

  auto deadline = std::chrono::steady_clock::now() +
                  std::chrono::milliseconds(std::max(timeout_ms, 1));
  while (std::chrono::steady_clock::now() < deadline) {
    if (!connect_detail::process_alive(pid)) {
      connect_detail::stop_process(process);
      throw std::runtime_error("holon exited before stdio gRPC bridge connected");
    }
    {
      std::lock_guard<std::mutex> lock(process->client_mutex);
      if (process->client_fd >= 0) {
        int client_fd = process->client_fd;
        process->client_fd = -1;
        cleanup();
        return connect_detail::startup_result{proxy_uri, client_fd, process};
      }
    }
    std::this_thread::sleep_for(std::chrono::milliseconds(25));
  }

  connect_detail::stop_process(process);
  cleanup();
  throw std::runtime_error("timed out waiting for holon startup");
#endif
}
#endif

inline std::shared_ptr<grpc::Channel>
unwrap_channel(const ConnectResult &result) {
  auto *owner = static_cast<channel_owner *>(result.channel);
  if (owner == nullptr) {
    return {};
  }
  return owner->channel;
}

inline ConnectResult connect_resolved(const HolonRef &ref, int timeout_ms) {
  auto effective_timeout = timeout_ms <= 0 ? 5000 : std::max(timeout_ms, 1);
  auto origin = ref;
  auto scheme = discovery_detail::transport_scheme(ref.url);
  if (scheme == "tcp" || scheme == "unix") {
    try {
      auto channel =
          connect_detail::dial_ready(ref.url, effective_timeout);
      auto *owner = new channel_owner{
          channel, nullptr, false, connect_detail::normalize_dial_target(ref.url)};
      return ConnectResult{owner, "", origin, std::nullopt};
    } catch (const std::exception &error) {
      return ConnectResult{nullptr, "", origin, std::string(error.what())};
    }
  }

  if (scheme != "file") {
    return ConnectResult{nullptr,
                         "",
                         origin,
                         std::string("unsupported target URL \"") + ref.url +
                             "\""};
  }

  try {
    auto target = launch_target_from_ref(ref);
    auto transport =
        ref.info.has_value() ? preferred_transport(*ref.info)
                             : default_transport_for_platform();
    auto startup = transport == "stdio"
                       ? start_command_stdio(target, effective_timeout)
                       : start_command_tcp(target, effective_timeout);

    std::shared_ptr<grpc::Channel> channel;
    try {
      channel = startup.direct_fd >= 0
                    ? connect_detail::dial_ready_from_fd(startup.direct_fd,
                                                         effective_timeout)
                    : connect_detail::dial_ready(startup.target,
                                                effective_timeout);
    } catch (const std::exception &) {
      if (startup.process != nullptr) {
        connect_detail::stop_process(startup.process);
      }
      throw;
    }

    if (startup.direct_fd < 0 && !trim_copy(startup.target).empty()) {
      origin.url = startup.target;
    }

    auto *owner = new channel_owner{channel, startup.process, true,
                                    startup.target};
    return ConnectResult{owner, "", origin, std::nullopt};
  } catch (const std::exception &error) {
    return ConnectResult{nullptr, "", origin, std::string(error.what())};
  }
}

} // namespace uniform_connect_detail

inline ConnectResult connect(int scope, const std::string &expression,
                             std::optional<std::string> root, int specifiers,
                             int timeout) {
  if (scope != LOCAL) {
    return ConnectResult{nullptr,
                         "",
                         std::nullopt,
                         std::string("scope ") + std::to_string(scope) +
                             " not supported"};
  }

  auto target = trim_copy(expression);
  if (target.empty()) {
    return ConnectResult{nullptr, "", std::nullopt, "expression is required"};
  }

  auto resolved = resolve(scope, target, root, specifiers, timeout);
  if (resolved.error.has_value()) {
    return ConnectResult{nullptr, "", resolved.ref, resolved.error};
  }
  if (!resolved.ref.has_value()) {
    return ConnectResult{nullptr,
                         "",
                         std::nullopt,
                         std::string("holon \"") + target + "\" not found"};
  }
  if (resolved.ref->error.has_value()) {
    return ConnectResult{nullptr, "", resolved.ref, resolved.ref->error};
  }

  return uniform_connect_detail::connect_resolved(*resolved.ref, timeout);
}

inline void disconnect(ConnectResult &result) {
  auto *owner =
      static_cast<uniform_connect_detail::channel_owner *>(result.channel);
  if (owner == nullptr) {
    return;
  }

  owner->channel.reset();
  if (owner->ephemeral && owner->process != nullptr) {
    try {
      connect_detail::stop_process(owner->process);
    } catch (const std::exception &) {
    }
  }

  delete owner;
  result.channel = nullptr;
  result.uid.clear();
}

} // namespace holons
