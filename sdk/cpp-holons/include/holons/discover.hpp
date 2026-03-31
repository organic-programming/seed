#pragma once

#include "holons.hpp"

#include <iomanip>

#if defined(__APPLE__)
#include <mach-o/dyld.h>
#endif

#if HOLONS_HAS_GRPCPP && __has_include("holons/v1/describe.pb.h") &&          \
    __has_include("holons/v1/describe.grpc.pb.h")
#include "holons/v1/describe.pb.h"
#include "holons/v1/describe.grpc.pb.h"
#define HOLONS_HAS_UNIFORM_DISCOVER_DESCRIBE 1
#else
#define HOLONS_HAS_UNIFORM_DISCOVER_DESCRIBE 0
#endif

namespace holons {

constexpr int LOCAL = 0;
constexpr int PROXY = 1;
constexpr int DELEGATED = 2;

constexpr int SIBLINGS = 0x01;
constexpr int CWD = 0x02;
constexpr int SOURCE = 0x04;
constexpr int BUILT = 0x08;
constexpr int INSTALLED = 0x10;
constexpr int CACHED = 0x20;
constexpr int ALL = 0x3F;

constexpr int NO_LIMIT = 0;
constexpr int NO_TIMEOUT = 0;

struct IdentityInfo {
  std::string given_name;
  std::string family_name;
  std::string motto;
  std::vector<std::string> aliases;
};

struct HolonInfo {
  std::string slug;
  std::string uuid;
  IdentityInfo identity;
  std::string lang;
  std::string runner;
  std::string status;
  std::string kind;
  std::string transport;
  std::string entrypoint;
  std::vector<std::string> architectures;
  bool has_dist = false;
  bool has_source = false;
};

struct HolonRef {
  std::string url;
  std::optional<HolonInfo> info;
  std::optional<std::string> error;
};

struct DiscoverResult {
  std::vector<HolonRef> found;
  std::optional<std::string> error;
};

struct ResolveResult {
  std::optional<HolonRef> ref;
  std::optional<std::string> error;
};

using discovered_holon = HolonEntry;

namespace discovery_detail {

struct discovered_entry {
  HolonRef ref;
  std::filesystem::path dir_path;
  std::string relative_path;
};

struct path_discovery_result {
  std::vector<HolonRef> refs;
  bool handled = false;
};

using source_bridge_callback = std::function<DiscoverResult(
    int, std::optional<std::string>, std::optional<std::string>, int, int, int)>;

inline source_bridge_callback &source_bridge_override() {
  static source_bridge_callback callback;
  return callback;
}

inline void set_source_bridge(source_bridge_callback callback) {
  source_bridge_override() = std::move(callback);
}

inline void clear_source_bridge() { source_bridge_override() = nullptr; }

inline bool is_unreserved_uri_char(unsigned char ch) {
  return std::isalnum(ch) || ch == '-' || ch == '_' || ch == '.' || ch == '~' ||
         ch == '/';
}

inline std::string percent_encode_uri_path(const std::string &value) {
  std::ostringstream out;
  out << std::uppercase << std::hex;
  for (unsigned char ch : value) {
    if (is_unreserved_uri_char(ch)) {
      out << static_cast<char>(ch);
      continue;
    }
    out << '%' << std::setw(2) << std::setfill('0')
        << static_cast<int>(ch);
  }
  return out.str();
}

inline std::string percent_decode_uri_path(const std::string &value) {
  std::string out;
  out.reserve(value.size());
  for (size_t i = 0; i < value.size(); ++i) {
    if (value[i] == '%' && i + 2 < value.size()) {
      unsigned int decoded = 0;
      std::istringstream in(value.substr(i + 1, 2));
      in >> std::hex >> decoded;
      if (!in.fail()) {
        out.push_back(static_cast<char>(decoded));
        i += 2;
        continue;
      }
    }
    out.push_back(value[i]);
  }
  return out;
}

inline std::filesystem::path normalized_existing_path(
    const std::filesystem::path &path) {
  std::error_code ec;
  auto absolute = std::filesystem::absolute(path, ec);
  if (ec) {
    absolute = path;
    ec.clear();
  }
  auto canonical = std::filesystem::weakly_canonical(absolute, ec);
  return ec ? absolute : canonical;
}

inline std::string file_url(const std::filesystem::path &path) {
  auto absolute = normalized_existing_path(path);
  auto generic = absolute.generic_string();
#ifdef _WIN32
  if (generic.size() >= 2 && std::isalpha(static_cast<unsigned char>(generic[0])) &&
      generic[1] == ':') {
    generic.insert(generic.begin(), '/');
  }
#endif
  return "file://" + percent_encode_uri_path(generic);
}

inline std::filesystem::path path_from_file_url(const std::string &raw_url) {
  auto trimmed = trim_copy(raw_url);
  if (trimmed.rfind("file://", 0) != 0) {
    throw std::invalid_argument("holon URL \"" + raw_url +
                                "\" is not a local file target");
  }

  auto rest = trimmed.substr(std::string("file://").size());
  std::string decoded_path;
  if (rest.rfind("localhost/", 0) == 0) {
    decoded_path = "/" + percent_decode_uri_path(rest.substr(10));
  } else if (!rest.empty() && rest.front() != '/') {
    auto slash = rest.find('/');
    if (slash == std::string::npos) {
      throw std::invalid_argument("holon URL \"" + raw_url + "\" has no path");
    }
    decoded_path = "//" + percent_decode_uri_path(rest.substr(0, slash)) +
                   percent_decode_uri_path(rest.substr(slash));
  } else {
    decoded_path = percent_decode_uri_path(rest);
  }

  if (decoded_path.empty()) {
    throw std::invalid_argument("holon URL \"" + raw_url + "\" has no path");
  }

#ifdef _WIN32
  if (decoded_path.size() >= 3 && decoded_path[0] == '/' &&
      std::isalpha(static_cast<unsigned char>(decoded_path[1])) &&
      decoded_path[2] == ':') {
    decoded_path.erase(decoded_path.begin());
  }
#endif

  return normalized_existing_path(decoded_path);
}

inline std::string string_or_empty(const nlohmann::json &value) {
  if (value.is_string()) {
    return trim_copy(value.get<std::string>());
  }
  if (value.is_boolean()) {
    return value.get<bool>() ? "true" : "false";
  }
  if (value.is_number_integer()) {
    return std::to_string(value.get<long long>());
  }
  if (value.is_number_unsigned()) {
    return std::to_string(value.get<unsigned long long>());
  }
  if (value.is_number_float()) {
    std::ostringstream out;
    out << value.get<double>();
    return out.str();
  }
  return "";
}

inline std::string json_string(const nlohmann::json &object,
                               std::initializer_list<const char *> keys) {
  if (!object.is_object()) {
    return "";
  }
  for (const char *key : keys) {
    auto it = object.find(key);
    if (it != object.end()) {
      auto value = string_or_empty(*it);
      if (!value.empty()) {
        return value;
      }
    }
  }
  return "";
}

inline std::vector<std::string>
json_string_list(const nlohmann::json &object,
                 std::initializer_list<const char *> keys) {
  if (!object.is_object()) {
    return {};
  }
  for (const char *key : keys) {
    auto it = object.find(key);
    if (it == object.end() || !it->is_array()) {
      continue;
    }
    std::vector<std::string> out;
    out.reserve(it->size());
    for (const auto &value : *it) {
      auto item = string_or_empty(value);
      if (!item.empty()) {
        out.push_back(item);
      }
    }
    return out;
  }
  return {};
}

inline std::string slug_for(const IdentityInfo &identity) {
  auto append = [](std::string *target, const std::string &part) {
    auto trimmed = trim_copy(part);
    for (char ch : trimmed) {
      if (ch == '?') {
        continue;
      }
      if (std::isspace(static_cast<unsigned char>(ch))) {
        target->push_back('-');
      } else {
        target->push_back(static_cast<char>(
            std::tolower(static_cast<unsigned char>(ch))));
      }
    }
    while (!target->empty() && target->back() == '-') {
      target->pop_back();
    }
  };

  std::string slug;
  append(&slug, identity.given_name);
  if (!slug.empty() && !trim_copy(identity.family_name).empty()) {
    slug.push_back('-');
  }
  append(&slug, identity.family_name);
  while (!slug.empty() && slug.back() == '-') {
    slug.pop_back();
  }
  return slug;
}

inline std::filesystem::path resolve_discover_root(
    const std::optional<std::string> &root) {
  if (!root.has_value()) {
    return normalized_existing_path(std::filesystem::current_path());
  }

  auto trimmed = trim_copy(*root);
  if (trimmed.empty()) {
    throw std::invalid_argument("root cannot be empty");
  }

  auto resolved = normalized_existing_path(trimmed);
  std::error_code ec;
  if (!std::filesystem::exists(resolved, ec) ||
      !std::filesystem::is_directory(resolved, ec)) {
    throw std::invalid_argument("root \"" + trimmed + "\" is not a directory");
  }
  return resolved;
}

inline std::filesystem::path normalize_search_root(
    const std::filesystem::path &root) {
  if (root.empty()) {
    return normalized_existing_path(std::filesystem::current_path());
  }
  return normalized_existing_path(root);
}

inline std::string relative_path(const std::filesystem::path &root,
                                 const std::filesystem::path &path) {
  auto normalized_root = normalize_search_root(root);
  auto normalized_path = normalized_existing_path(path);
  auto relative = normalized_path.lexically_relative(normalized_root);
  if (relative.empty()) {
    return normalized_path.generic_string();
  }
  auto text = relative.generic_string();
  return text.empty() ? "." : text;
}

inline size_t path_depth(const std::string &relative) {
  auto trimmed = trim_copy(relative);
  if (trimmed.empty() || trimmed == ".") {
    return 0;
  }
  size_t depth = 0;
  std::stringstream in(trimmed);
  std::string segment;
  while (std::getline(in, segment, '/')) {
    if (!segment.empty() && segment != ".") {
      ++depth;
    }
  }
  return depth;
}

inline bool should_skip_dir(const std::filesystem::path &root,
                            const std::filesystem::path &path) {
  if (normalize_search_root(path) == normalize_search_root(root)) {
    return false;
  }

  auto name = path.filename().string();
  if (name.size() >= 6 && name.substr(name.size() - 6) == ".holon") {
    return false;
  }
  if (name == ".git" || name == ".op" || name == "node_modules" ||
      name == "vendor" || name == "build" || name == "testdata") {
    return true;
  }
  return !name.empty() && name.front() == '.';
}

inline std::string format_invalid_specifiers(int specifiers) {
  std::ostringstream out;
  out << "invalid specifiers 0x" << std::uppercase << std::hex
      << std::setw(2) << std::setfill('0')
      << static_cast<unsigned int>(specifiers)
      << ": valid range is 0x00-0x3F";
  return out.str();
}

inline std::string ref_path(const HolonRef &ref) {
  if (trim_copy(ref.url).empty()) {
    return {};
  }
  if (trim_copy(ref.url).rfind("file://", 0) == 0) {
    try {
      return path_from_file_url(ref.url).generic_string();
    } catch (const std::exception &) {
      return {};
    }
  }
  return {};
}

inline std::string entry_key(const discovered_entry &entry) {
  if (entry.ref.info.has_value() && !trim_copy(entry.ref.info->uuid).empty()) {
    return trim_copy(entry.ref.info->uuid);
  }
  if (!entry.dir_path.empty()) {
    return normalize_search_root(entry.dir_path).generic_string();
  }
  return trim_copy(entry.ref.url);
}

inline std::string entry_sort_key(const discovered_entry &entry) {
  if (entry.ref.info.has_value() && !trim_copy(entry.ref.info->uuid).empty()) {
    return trim_copy(entry.ref.info->uuid);
  }
  return entry.ref.url;
}

inline bool should_replace_entry(const discovered_entry &current,
                                 const discovered_entry &next_entry) {
  return path_depth(next_entry.relative_path) < path_depth(current.relative_path);
}

inline std::vector<std::filesystem::path>
package_dirs_direct(const std::filesystem::path &root) {
  auto absolute_root = normalize_search_root(root);
  std::error_code ec;
  if (!std::filesystem::exists(absolute_root, ec) ||
      !std::filesystem::is_directory(absolute_root, ec)) {
    return {};
  }

  std::vector<std::filesystem::path> dirs;
  for (const auto &entry : std::filesystem::directory_iterator(absolute_root, ec)) {
    if (ec) {
      ec.clear();
      continue;
    }
    if (!entry.is_directory(ec)) {
      ec.clear();
      continue;
    }
    auto name = entry.path().filename().string();
    if (name.size() >= 6 && name.substr(name.size() - 6) == ".holon") {
      dirs.push_back(entry.path());
    }
  }
  std::sort(dirs.begin(), dirs.end());
  return dirs;
}

inline std::vector<std::filesystem::path>
package_dirs_recursive(const std::filesystem::path &root) {
  auto absolute_root = normalize_search_root(root);
  std::error_code ec;
  if (!std::filesystem::exists(absolute_root, ec) ||
      !std::filesystem::is_directory(absolute_root, ec)) {
    return {};
  }

  std::vector<std::filesystem::path> dirs;
  std::filesystem::recursive_directory_iterator it(
      absolute_root, std::filesystem::directory_options::skip_permission_denied,
      ec);
  std::filesystem::recursive_directory_iterator end;
  for (; it != end; it.increment(ec)) {
    if (ec) {
      ec.clear();
      continue;
    }

    const auto path = it->path();
    if (!it->is_directory(ec)) {
      ec.clear();
      continue;
    }

    auto name = path.filename().string();
    if (name.size() >= 6 && name.substr(name.size() - 6) == ".holon") {
      dirs.push_back(path);
      it.disable_recursion_pending();
      continue;
    }
    if (should_skip_dir(absolute_root, path)) {
      it.disable_recursion_pending();
    }
  }

  std::sort(dirs.begin(), dirs.end());
  return dirs;
}

inline std::string current_arch_dir() {
#if defined(_WIN32)
  const char *system = "windows";
#elif defined(__APPLE__)
  const char *system = "darwin";
#elif defined(__linux__)
  const char *system = "linux";
#else
  const char *system = "unknown";
#endif

#if defined(__aarch64__) || defined(_M_ARM64)
  const char *arch = "arm64";
#elif defined(__x86_64__) || defined(_M_X64) || defined(__amd64__)
  const char *arch = "amd64";
#else
  const char *arch = "unknown";
#endif

  return std::string(system) + "_" + arch;
}

inline std::vector<std::string>
package_architectures(const std::filesystem::path &package_dir) {
  std::vector<std::string> found;
  std::error_code ec;
  auto bin_root = package_dir / "bin";
  if (!std::filesystem::exists(bin_root, ec) ||
      !std::filesystem::is_directory(bin_root, ec)) {
    return found;
  }

  for (const auto &entry : std::filesystem::directory_iterator(bin_root, ec)) {
    if (ec) {
      ec.clear();
      continue;
    }
    if (entry.is_directory(ec)) {
      found.push_back(entry.path().filename().string());
    }
  }
  std::sort(found.begin(), found.end());
  return found;
}

inline std::filesystem::path package_binary_path(
    const std::filesystem::path &package_dir) {
  auto arch_root = package_dir / "bin" / current_arch_dir();
  std::error_code ec;
  if (!std::filesystem::exists(arch_root, ec) ||
      !std::filesystem::is_directory(arch_root, ec)) {
    throw std::runtime_error("no package binary for arch " + current_arch_dir());
  }

  std::vector<std::filesystem::path> candidates;
  for (const auto &entry : std::filesystem::directory_iterator(arch_root, ec)) {
    if (ec) {
      ec.clear();
      continue;
    }
    if (entry.is_regular_file(ec)) {
      candidates.push_back(entry.path());
    }
  }

  if (candidates.empty()) {
    throw std::runtime_error("no package binary for arch " + current_arch_dir());
  }

  std::sort(candidates.begin(), candidates.end());
  return candidates.front();
}

inline HolonInfo holon_info_from_json(const nlohmann::json &payload) {
  auto identity_payload =
      payload.contains("identity") && payload["identity"].is_object()
          ? payload["identity"]
          : nlohmann::json::object();

  IdentityInfo identity;
  identity.given_name = json_string(identity_payload, {"given_name", "givenName"});
  identity.family_name =
      json_string(identity_payload, {"family_name", "familyName"});
  identity.motto = json_string(identity_payload, {"motto"});
  identity.aliases = json_string_list(identity_payload, {"aliases"});

  HolonInfo info;
  info.slug = json_string(payload, {"slug"});
  if (info.slug.empty()) {
    info.slug = slug_for(identity);
  }
  info.uuid = json_string(payload, {"uuid"});
  info.identity = std::move(identity);
  info.lang = json_string(payload, {"lang"});
  info.runner = json_string(payload, {"runner"});
  info.status = json_string(payload, {"status"});
  info.kind = json_string(payload, {"kind"});
  info.transport = json_string(payload, {"transport"});
  info.entrypoint = json_string(payload, {"entrypoint"});
  info.architectures = json_string_list(payload, {"architectures"});
  info.has_dist = payload.value("has_dist", false);
  info.has_source = payload.value("has_source", false);
  return info;
}

inline discovered_entry load_package_entry(const std::filesystem::path &root,
                                           const std::filesystem::path &package_dir) {
  std::ifstream in(package_dir / ".holon.json");
  if (!in.is_open()) {
    throw std::runtime_error("missing .holon.json");
  }

  nlohmann::json payload = nlohmann::json::parse(in);
  auto schema = json_string(payload, {"schema"});
  if (!schema.empty() && schema != "holon-package/v1") {
    throw std::runtime_error("unsupported package schema \"" + schema + "\"");
  }

  auto absolute_dir = normalized_existing_path(package_dir);
  HolonRef ref;
  ref.url = file_url(absolute_dir);
  ref.info = holon_info_from_json(payload);

  discovered_entry entry;
  entry.ref = std::move(ref);
  entry.dir_path = absolute_dir;
  entry.relative_path = relative_path(root, absolute_dir);
  return entry;
}

#if HOLONS_HAS_UNIFORM_DISCOVER_DESCRIBE
inline std::chrono::system_clock::time_point describe_deadline(int timeout_ms) {
  int effective_timeout = timeout_ms <= 0 ? 5000 : std::max(timeout_ms, 1);
  return std::chrono::system_clock::now() +
         std::chrono::milliseconds(effective_timeout);
}

inline HolonInfo holon_info_from_describe_response(
    const holons::v1::DescribeResponse &response) {
  if (!response.has_manifest()) {
    throw std::runtime_error("Describe returned no manifest");
  }

  const auto &manifest = response.manifest();
  if (!manifest.has_identity()) {
    throw std::runtime_error("Describe returned no manifest identity");
  }

  const auto &manifest_identity = manifest.identity();
  IdentityInfo identity;
  identity.given_name = manifest_identity.given_name();
  identity.family_name = manifest_identity.family_name();
  identity.motto = manifest_identity.motto();
  identity.aliases.assign(manifest_identity.aliases().begin(),
                          manifest_identity.aliases().end());

  HolonInfo info;
  info.slug = slug_for(identity);
  info.uuid = manifest_identity.uuid();
  info.identity = std::move(identity);
  info.lang = manifest.lang();
  info.runner = manifest.has_build() ? manifest.build().runner() : "";
  info.status = manifest_identity.status();
  info.kind = manifest.kind();
  info.transport = manifest.transport();
  info.entrypoint =
      manifest.has_artifacts() ? manifest.artifacts().binary() : "";
  info.architectures.assign(manifest.platforms().begin(),
                            manifest.platforms().end());
  return info;
}

inline HolonInfo describe_channel(const std::shared_ptr<grpc::Channel> &channel,
                                  int timeout_ms) {
  if (!channel) {
    throw std::runtime_error("Describe requires a live channel");
  }

  auto stub = holons::v1::HolonMeta::NewStub(channel);
  grpc::ClientContext context;
  context.set_deadline(describe_deadline(timeout_ms));

  holons::v1::DescribeRequest request;
  holons::v1::DescribeResponse response;
  auto status = stub->Describe(&context, request, &response);
  if (!status.ok()) {
    throw std::runtime_error("Describe failed: " + status.error_message());
  }
  return holon_info_from_describe_response(response);
}

inline HolonInfo describe_binary_target(const std::filesystem::path &binary_path,
                                        int timeout_ms) {
  int effective_timeout = timeout_ms <= 0 ? 5000 : std::max(timeout_ms, 1);

  connect_detail::startup_result startup;
  try {
#if HOLONS_HAS_GRPC_FD
    startup = connect_detail::start_stdio_holon(binary_path.string(),
                                                effective_timeout);
#else
    startup = connect_detail::start_tcp_holon(binary_path.string(),
                                              effective_timeout);
#endif
  } catch (const std::exception &error) {
    throw std::runtime_error("Describe startup failed: " +
                             std::string(error.what()));
  }

  try {
    auto channel = startup.direct_fd >= 0
                       ? connect_detail::dial_ready_from_fd(startup.direct_fd,
                                                            effective_timeout)
                       : connect_detail::dial_ready(startup.target,
                                                   effective_timeout);
    auto info = describe_channel(channel, effective_timeout);
    if (startup.process != nullptr) {
      connect_detail::stop_process(startup.process);
    }
    return info;
  } catch (...) {
    if (startup.process != nullptr) {
      try {
        connect_detail::stop_process(startup.process);
      } catch (const std::exception &) {
      }
    }
    throw;
  }
}
#endif

inline discovered_entry probe_package_entry(const std::filesystem::path &root,
                                            const std::filesystem::path &package_dir,
                                            int timeout_ms) {
#if !HOLONS_HAS_UNIFORM_DISCOVER_DESCRIBE
  (void)root;
  (void)package_dir;
  (void)timeout_ms;
  throw std::runtime_error("Describe fallback requires grpc++ HolonMeta support");
#else
  auto absolute_dir = normalized_existing_path(package_dir);
  auto info = describe_binary_target(package_binary_path(absolute_dir), timeout_ms);
  info.has_dist = info.has_dist ||
                  std::filesystem::is_directory(absolute_dir / "dist");
  info.has_source = info.has_source ||
                    std::filesystem::is_directory(absolute_dir / "git");
  if (info.architectures.empty()) {
    info.architectures = package_architectures(absolute_dir);
  }

  HolonRef ref;
  ref.url = file_url(absolute_dir);
  ref.info = std::move(info);

  discovered_entry entry;
  entry.ref = std::move(ref);
  entry.dir_path = absolute_dir;
  entry.relative_path = relative_path(root, absolute_dir);
  return entry;
#endif
}

inline discovered_entry probe_binary_entry(const std::filesystem::path &path,
                                           int timeout_ms) {
#if !HOLONS_HAS_UNIFORM_DISCOVER_DESCRIBE
  (void)path;
  (void)timeout_ms;
  throw std::runtime_error("Describe fallback requires grpc++ HolonMeta support");
#else
  auto absolute_path = normalized_existing_path(path);
  auto info = describe_binary_target(absolute_path, timeout_ms);
  if (trim_copy(info.entrypoint).empty()) {
    info.entrypoint = absolute_path.string();
  }

  discovered_entry entry;
  entry.ref.url = file_url(absolute_path);
  entry.ref.info = std::move(info);
  entry.dir_path = absolute_path;
  entry.relative_path = absolute_path.filename().generic_string();
  return entry;
#endif
}

inline std::vector<discovered_entry>
discover_packages_from_dirs(const std::filesystem::path &root,
                            const std::vector<std::filesystem::path> &dirs,
                            int timeout_ms) {
  auto absolute_root = normalize_search_root(root);
  std::unordered_map<std::string, discovered_entry> by_key;
  std::vector<std::string> keys;

  for (const auto &dir : dirs) {
    discovered_entry entry;
    try {
      entry = load_package_entry(absolute_root, dir);
    } catch (const std::exception &) {
      try {
        entry = probe_package_entry(absolute_root, dir, timeout_ms);
      } catch (const std::exception &probe_error) {
        auto absolute_dir = normalized_existing_path(dir);
        entry.ref.url = file_url(absolute_dir);
        entry.ref.error = std::string(probe_error.what());
        entry.dir_path = absolute_dir;
        entry.relative_path = relative_path(absolute_root, absolute_dir);
      }
    }

    auto key = entry_key(entry);
    auto existing = by_key.find(key);
    if (existing != by_key.end()) {
      if (should_replace_entry(existing->second, entry)) {
        existing->second = entry;
      }
      continue;
    }

    by_key.emplace(key, entry);
    keys.push_back(key);
  }

  std::vector<discovered_entry> entries;
  entries.reserve(keys.size());
  for (const auto &key : keys) {
    auto it = by_key.find(key);
    if (it != by_key.end()) {
      entries.push_back(it->second);
    }
  }

  std::sort(entries.begin(), entries.end(),
            [](const discovered_entry &left, const discovered_entry &right) {
              if (left.relative_path != right.relative_path) {
                return left.relative_path < right.relative_path;
              }
              return entry_sort_key(left) < entry_sort_key(right);
            });
  return entries;
}

inline std::vector<discovered_entry>
entries_from_refs(const std::filesystem::path &root,
                  const std::vector<HolonRef> &refs) {
  std::vector<discovered_entry> entries;
  entries.reserve(refs.size());
  for (const auto &ref : refs) {
    discovered_entry entry;
    entry.ref = ref;
    auto path = ref_path(ref);
    entry.dir_path = path.empty() ? std::filesystem::path(ref.url)
                                  : std::filesystem::path(path);
    entry.relative_path = relative_path(root, entry.dir_path);
    entries.push_back(std::move(entry));
  }

  std::sort(entries.begin(), entries.end(),
            [](const discovered_entry &left, const discovered_entry &right) {
              if (left.relative_path != right.relative_path) {
                return left.relative_path < right.relative_path;
              }
              return entry_sort_key(left) < entry_sort_key(right);
            });
  return entries;
}

inline bool matches_expression(const discovered_entry &entry,
                               const std::optional<std::string> &expression) {
  if (!expression.has_value()) {
    return true;
  }

  auto needle = trim_copy(*expression);
  if (needle.empty()) {
    return false;
  }

  if (entry.ref.info.has_value()) {
    if (entry.ref.info->slug == needle) {
      return true;
    }
    if (!entry.ref.info->uuid.empty() &&
        entry.ref.info->uuid.rfind(needle, 0) == 0) {
      return true;
    }
    for (const auto &alias : entry.ref.info->identity.aliases) {
      if (alias == needle) {
        return true;
      }
    }
  }

  auto base = entry.dir_path.filename().string();
  if (base.size() >= 6 && base.substr(base.size() - 6) == ".holon") {
    auto stripped = base.substr(0, base.size() - 6);
    return stripped == needle || base == needle;
  }
  return base == needle;
}

inline std::optional<std::string>
normalized_expression(const std::optional<std::string> &expression) {
  if (!expression.has_value()) {
    return std::nullopt;
  }
  return trim_copy(*expression);
}

inline std::string transport_scheme(const std::string &expression) {
  auto trimmed = trim_copy(expression);
  auto pos = trimmed.find("://");
  if (pos == std::string::npos) {
    return "";
  }
  auto scheme = trimmed.substr(0, pos);
  std::transform(scheme.begin(), scheme.end(), scheme.begin(),
                 [](unsigned char ch) {
                   return static_cast<char>(std::tolower(ch));
                 });
  return scheme;
}

inline bool looks_like_path_expression(const std::string &expression) {
  auto trimmed = trim_copy(expression);
  if (trimmed.empty()) {
    return false;
  }

  if (trimmed.rfind("file://", 0) == 0) {
    return true;
  }

  if (transport_scheme(trimmed).size() > 0) {
    return false;
  }

  const bool explicit_path =
      std::filesystem::path(trimmed).is_absolute() || trimmed.rfind(".", 0) == 0 ||
      trimmed.find(std::filesystem::path::preferred_separator) != std::string::npos ||
      trimmed.find('/') != std::string::npos || trimmed.find('\\') != std::string::npos;
  if (explicit_path) {
    return true;
  }

  return false;
}

inline std::filesystem::path path_expression_candidate(
    const std::string &expression,
    const std::function<std::filesystem::path()> &root_resolver) {
  auto trimmed = trim_copy(expression);
  if (trimmed.empty()) {
    return {};
  }
  if (trimmed.rfind("file://", 0) == 0) {
    return path_from_file_url(trimmed);
  }
  if (!looks_like_path_expression(trimmed)) {
    return {};
  }
  if (std::filesystem::path(trimmed).is_absolute()) {
    return trimmed;
  }
  return root_resolver() / trimmed;
}

inline std::filesystem::path current_executable_path() {
#ifdef _WIN32
  std::wstring buffer(32768, L'\0');
  auto length = ::GetModuleFileNameW(nullptr, buffer.data(),
                                     static_cast<DWORD>(buffer.size()));
  if (length == 0) {
    return {};
  }
  buffer.resize(length);
  return std::filesystem::path(buffer);
#elif defined(__APPLE__)
  uint32_t size = 0;
  (void)_NSGetExecutablePath(nullptr, &size);
  std::vector<char> buffer(size + 1, '\0');
  if (_NSGetExecutablePath(buffer.data(), &size) != 0) {
    return {};
  }
  return normalized_existing_path(buffer.data());
#else
  std::array<char, 4096> buffer{};
  auto length = ::readlink("/proc/self/exe", buffer.data(), buffer.size() - 1);
  if (length <= 0) {
    return {};
  }
  buffer[static_cast<size_t>(length)] = '\0';
  return normalized_existing_path(buffer.data());
#endif
}

inline std::filesystem::path bundle_holons_root() {
  if (const char *configured = std::getenv("HOLONS_SIBLINGS_ROOT");
      configured != nullptr && *configured != '\0') {
    return normalized_existing_path(configured);
  }

  auto executable = current_executable_path();
  if (executable.empty()) {
    return {};
  }

  auto current = executable.parent_path();
  while (!current.empty()) {
    auto name = current.filename().string();
    if (name.size() >= 4 && name.substr(name.size() - 4) == ".app") {
      auto candidate = current / "Contents" / "Resources" / "Holons";
      std::error_code ec;
      if (std::filesystem::exists(candidate, ec) &&
          std::filesystem::is_directory(candidate, ec)) {
        return candidate;
      }
    }
    if (current.parent_path() == current) {
      break;
    }
    current = current.parent_path();
  }

  return {};
}

inline std::filesystem::path installed_root() { return opbin(); }

inline std::filesystem::path cache_root() { return cache_dir(); }

inline std::filesystem::path built_root(const std::filesystem::path &root) {
  return normalize_search_root(root) / ".op" / "build";
}

inline std::filesystem::path source_root(const std::filesystem::path &root) {
  return normalize_search_root(root);
}

struct command_result {
  int exit_code = -1;
  std::string stdout_text;
};

inline std::string shell_quote(const std::string &value) {
#ifdef _WIN32
  std::string out = "\"";
  for (char ch : value) {
    if (ch == '"') {
      out += "\\\"";
    } else {
      out.push_back(ch);
    }
  }
  out.push_back('"');
  return out;
#else
  std::string out = "'";
  for (char ch : value) {
    if (ch == '\'') {
      out += "'\\''";
    } else {
      out.push_back(ch);
    }
  }
  out.push_back('\'');
  return out;
#endif
}

inline command_result run_command_capture(
    const std::filesystem::path &executable, const std::vector<std::string> &args,
    const std::filesystem::path &cwd) {
  std::ostringstream command;
#ifdef _WIN32
  command << "cd /D " << shell_quote(normalize_search_root(cwd).string())
          << " && " << shell_quote(executable.string());
#else
  command << "cd " << shell_quote(normalize_search_root(cwd).string()) << " && "
          << shell_quote(executable.string());
#endif
  for (const auto &arg : args) {
    command << " " << shell_quote(arg);
  }
#ifdef _WIN32
  command << " 2>nul";
#else
  command << " 2>/dev/null";
#endif

#ifdef _WIN32
  FILE *pipe = ::_popen(command.str().c_str(), "r");
#else
  FILE *pipe = ::popen(command.str().c_str(), "r");
#endif
  if (pipe == nullptr) {
    return {};
  }

  std::array<char, 4096> buffer{};
  command_result result;
  while (std::fgets(buffer.data(), static_cast<int>(buffer.size()), pipe) !=
         nullptr) {
    result.stdout_text.append(buffer.data());
  }
#ifdef _WIN32
  result.exit_code = ::_pclose(pipe);
#else
  result.exit_code = ::pclose(pipe);
#endif
  return result;
}

inline std::filesystem::path find_executable_in_path(const std::string &name) {
  auto is_executable = [](const std::filesystem::path &candidate) {
    std::error_code ec;
    if (!std::filesystem::exists(candidate, ec) ||
        !std::filesystem::is_regular_file(candidate, ec)) {
      return false;
    }
#ifdef _WIN32
    return true;
#else
    return ::access(candidate.c_str(), X_OK) == 0;
#endif
  };

  auto direct = std::filesystem::path(name);
  if (direct.is_absolute() && is_executable(direct)) {
    return direct;
  }

  const char *path_env = std::getenv("PATH");
  if (path_env == nullptr || *path_env == '\0') {
    return {};
  }

#ifdef _WIN32
  const char delimiter = ';';
  std::vector<std::string> suffixes = {"", ".exe", ".bat", ".cmd"};
#else
  const char delimiter = ':';
  std::vector<std::string> suffixes = {""};
#endif

  std::stringstream parts(path_env);
  std::string part;
  while (std::getline(parts, part, delimiter)) {
    if (part.empty()) {
      continue;
    }
    for (const auto &suffix : suffixes) {
      auto candidate = std::filesystem::path(part) / (name + suffix);
      if (is_executable(candidate)) {
        return candidate;
      }
    }
  }

  return {};
}

inline DiscoverResult default_source_bridge(int scope,
                                            std::optional<std::string> expression,
                                            std::optional<std::string> root,
                                            int specifiers, int limit,
                                            int timeout) {
  (void)timeout;
  if (scope != LOCAL) {
    return DiscoverResult{{},
                          std::string("scope ") + std::to_string(scope) +
                              " not supported"};
  }
  if (specifiers != SOURCE) {
    return DiscoverResult{
        {}, std::string("invalid source bridge specifiers 0x") +
                [] (int value) {
                  std::ostringstream out;
                  out << std::uppercase << std::hex << std::setw(2)
                      << std::setfill('0') << static_cast<unsigned int>(value);
                  return out.str();
                }(specifiers)};
  }

  auto bridge_root = resolve_discover_root(root);
  auto executable = find_executable_in_path("op");
  if (executable.empty()) {
    return DiscoverResult{};
  }

  auto output = run_command_capture(executable, {"discover", "--json"},
                                    bridge_root);
  if (output.exit_code != 0 || trim_copy(output.stdout_text).empty()) {
    return DiscoverResult{};
  }

  nlohmann::json payload;
  try {
    payload = nlohmann::json::parse(output.stdout_text);
  } catch (const std::exception &) {
    return DiscoverResult{};
  }

  std::vector<HolonRef> refs;
  if (payload.contains("entries") && payload["entries"].is_array()) {
    for (const auto &entry : payload["entries"]) {
      if (!entry.is_object()) {
        continue;
      }
      auto origin = json_string(entry, {"origin"});
      if (!origin.empty() && origin != "local" && origin != "source" &&
          origin != "cwd") {
        continue;
      }

      IdentityInfo identity;
      if (entry.contains("identity") && entry["identity"].is_object()) {
        identity.given_name =
            json_string(entry["identity"], {"given_name", "givenName"});
        identity.family_name =
            json_string(entry["identity"], {"family_name", "familyName"});
        identity.motto = json_string(entry["identity"], {"motto"});
        identity.aliases = json_string_list(entry["identity"], {"aliases"});
      } else {
        identity.given_name = json_string(entry, {"given_name", "givenName"});
        identity.family_name =
            json_string(entry, {"family_name", "familyName"});
        identity.motto = json_string(entry, {"motto"});
      }

      auto relative =
          json_string(entry, {"relative_path", "relativePath"});
      auto absolute = relative.empty() ? bridge_root : bridge_root / relative;

      HolonInfo info;
      info.slug = json_string(entry, {"slug"});
      if (info.slug.empty()) {
        info.slug = slug_for(identity);
      }
      info.uuid = json_string(entry, {"uuid"});
      info.identity = std::move(identity);
      info.lang = json_string(entry, {"lang"});
      info.status = json_string(entry, {"status"});
      info.has_source = true;

      HolonRef ref;
      ref.url = file_url(absolute);
      ref.info = std::move(info);
      refs.push_back(std::move(ref));
    }
  }

  auto normalized = normalized_expression(expression);
  if (normalized.has_value()) {
    auto entries = entries_from_refs(bridge_root, refs);
    std::vector<HolonRef> filtered;
    for (const auto &entry : entries) {
      if (matches_expression(entry, normalized)) {
        filtered.push_back(entry.ref);
      }
    }
    refs = std::move(filtered);
  }

  if (limit > 0 && refs.size() > static_cast<size_t>(limit)) {
    refs.resize(static_cast<size_t>(limit));
  }
  return DiscoverResult{std::move(refs), std::nullopt};
}

inline DiscoverResult discover_source_with_local_op(
    int scope, std::optional<std::string> expression,
    const std::filesystem::path &root, int specifiers, int limit, int timeout) {
  auto &override = source_bridge_override();
  if (override) {
    return override(scope, std::move(expression),
                    std::optional<std::string>(normalize_search_root(root).string()),
                    specifiers, limit, timeout);
  }
  return default_source_bridge(
      scope, std::move(expression),
      std::optional<std::string>(normalize_search_root(root).string()),
      specifiers, limit, timeout);
}

inline std::vector<discovered_entry>
discover_entries(const std::filesystem::path &root, int specifiers,
                 int timeout_ms) {
  struct layer {
    int flag;
    std::function<std::vector<discovered_entry>(const std::filesystem::path &)>
        scan;
  };

  std::vector<layer> layers{
      {SIBLINGS,
       [&](const std::filesystem::path &) {
         auto bundle_root = bundle_holons_root();
         if (bundle_root.empty()) {
           return std::vector<discovered_entry>{};
         }
         return discover_packages_from_dirs(bundle_root,
                                            package_dirs_direct(bundle_root),
                                            timeout_ms);
       }},
      {CWD,
       [&](const std::filesystem::path &current_root) {
         return discover_packages_from_dirs(
             current_root, package_dirs_recursive(current_root), timeout_ms);
       }},
      {SOURCE,
       [&](const std::filesystem::path &current_root) {
         auto result = discover_source_with_local_op(
             LOCAL, std::nullopt, current_root, SOURCE, NO_LIMIT, timeout_ms);
         if (result.error.has_value()) {
           throw std::runtime_error(*result.error);
         }
         return entries_from_refs(current_root, result.found);
       }},
      {BUILT,
       [&](const std::filesystem::path &current_root) {
         auto layer_root = built_root(current_root);
         return discover_packages_from_dirs(layer_root,
                                            package_dirs_direct(layer_root),
                                            timeout_ms);
       }},
      {INSTALLED,
       [&](const std::filesystem::path &) {
         auto layer_root = installed_root();
         return discover_packages_from_dirs(layer_root,
                                            package_dirs_direct(layer_root),
                                            timeout_ms);
       }},
      {CACHED,
       [&](const std::filesystem::path &) {
         auto layer_root = cache_root();
         return discover_packages_from_dirs(layer_root,
                                            package_dirs_recursive(layer_root),
                                            timeout_ms);
       }},
  };

  std::unordered_map<std::string, discovered_entry> by_key;
  std::vector<std::string> keys;
  for (const auto &layer : layers) {
    if ((specifiers & layer.flag) == 0) {
      continue;
    }
    for (const auto &entry : layer.scan(root)) {
      auto key = entry_key(entry);
      auto existing = by_key.find(key);
      if (existing != by_key.end()) {
        if (should_replace_entry(existing->second, entry)) {
          existing->second = entry;
        }
        continue;
      }
      by_key.emplace(key, entry);
      keys.push_back(key);
    }
  }

  std::vector<discovered_entry> entries;
  entries.reserve(keys.size());
  for (const auto &key : keys) {
    auto it = by_key.find(key);
    if (it != by_key.end()) {
      entries.push_back(it->second);
    }
  }
  return entries;
}

inline path_discovery_result discover_path_expression(
    const std::string &expression,
    const std::function<std::filesystem::path()> &root_resolver,
    int timeout_ms) {
  auto candidate = path_expression_candidate(expression, root_resolver);
  if (candidate.empty()) {
    return {};
  }

  auto absolute = normalized_existing_path(candidate);
  std::error_code ec;
  if (!std::filesystem::exists(absolute, ec)) {
    return {{}, true};
  }

  if (std::filesystem::is_directory(absolute, ec)) {
    if (absolute.filename().string().size() >= 6 &&
        absolute.filename().string().substr(
            absolute.filename().string().size() - 6) == ".holon") {
      try {
        auto entry = load_package_entry(absolute.parent_path(), absolute);
        return {{entry.ref}, true};
      } catch (const std::exception &) {
        try {
          auto entry = probe_package_entry(absolute.parent_path(), absolute,
                                           timeout_ms);
          return {{entry.ref}, true};
        } catch (const std::exception &probe_error) {
          HolonRef ref;
          ref.url = file_url(absolute);
          ref.error = std::string(probe_error.what());
          return {{ref}, true};
        }
      }
    }

    auto result = discover_source_with_local_op(LOCAL, std::nullopt, absolute,
                                                SOURCE, NO_LIMIT, timeout_ms);
    if (result.error.has_value()) {
      throw std::runtime_error(*result.error);
    }
    if (result.found.size() == 1) {
      return {{result.found.front()}, true};
    }
    for (const auto &ref : result.found) {
      auto path = ref_path(ref);
      if (!path.empty() &&
          normalize_search_root(path) == normalized_existing_path(absolute)) {
        return {{ref}, true};
      }
    }
    return {{}, true};
  }

  if (absolute.filename() == "holon.proto") {
    auto result = discover_source_with_local_op(LOCAL, std::nullopt,
                                                absolute.parent_path(), SOURCE,
                                                NO_LIMIT, timeout_ms);
    if (result.error.has_value()) {
      throw std::runtime_error(*result.error);
    }
    if (result.found.size() == 1) {
      return {{result.found.front()}, true};
    }
    for (const auto &ref : result.found) {
      auto path = ref_path(ref);
      if (!path.empty() && normalize_search_root(path) ==
                               normalized_existing_path(absolute.parent_path())) {
        return {{ref}, true};
      }
    }
    return {{}, true};
  }

  try {
    auto entry = probe_binary_entry(absolute, timeout_ms);
    return {{entry.ref}, true};
  } catch (const std::exception &probe_error) {
    HolonRef ref;
    ref.url = file_url(absolute);
    ref.error = std::string(probe_error.what());
    return {{ref}, true};
  }
}

} // namespace discovery_detail

inline DiscoverResult Discover(int scope,
                               std::optional<std::string> expression,
                               std::optional<std::string> root,
                               int specifiers, int limit, int timeout) {
  if (scope != LOCAL) {
    return DiscoverResult{{},
                          std::string("scope ") + std::to_string(scope) +
                              " not supported"};
  }
  if (specifiers < 0 || (specifiers & ~ALL) != 0) {
    return DiscoverResult{{},
                          discovery_detail::format_invalid_specifiers(specifiers)};
  }
  if (specifiers == 0) {
    specifiers = ALL;
  }
  if (limit < 0) {
    return DiscoverResult{};
  }

  auto normalized_expression =
      discovery_detail::normalized_expression(expression);
  if (normalized_expression.has_value()) {
    auto scheme = discovery_detail::transport_scheme(*normalized_expression);
    if (!scheme.empty() && scheme != "file") {
      return DiscoverResult{{},
                            std::string("direct URL expressions are not yet "
                                        "supported")};
    }
  }

  std::optional<std::filesystem::path> search_root;
  auto resolve_root = [&]() -> std::filesystem::path {
    if (!search_root.has_value()) {
      search_root = discovery_detail::resolve_discover_root(root);
    }
    return *search_root;
  };

  try {
    if (normalized_expression.has_value()) {
      auto path_result = discovery_detail::discover_path_expression(
          *normalized_expression, resolve_root, timeout);
      if (path_result.handled) {
        return DiscoverResult{std::move(path_result.refs), std::nullopt};
      }
    }

    auto entries =
        discovery_detail::discover_entries(resolve_root(), specifiers, timeout);
    std::vector<HolonRef> found;
    for (const auto &entry : entries) {
      if (!discovery_detail::matches_expression(entry, normalized_expression)) {
        continue;
      }
      found.push_back(entry.ref);
      if (limit > 0 && found.size() >= static_cast<size_t>(limit)) {
        break;
      }
    }
    return DiscoverResult{std::move(found), std::nullopt};
  } catch (const std::exception &error) {
    return DiscoverResult{{}, std::string(error.what())};
  }
}

inline ResolveResult resolve(int scope, const std::string &expression,
                             std::optional<std::string> root, int specifiers,
                             int timeout) {
  auto result = Discover(scope, std::optional<std::string>(expression), root,
                         specifiers, 1, timeout);
  if (result.error.has_value()) {
    return ResolveResult{std::nullopt, result.error};
  }
  if (result.found.empty()) {
    return ResolveResult{
        std::nullopt,
        std::string("holon \"") + expression + "\" not found"};
  }

  auto ref = result.found.front();
  if (ref.error.has_value()) {
    return ResolveResult{ref, ref.error};
  }
  return ResolveResult{ref, std::nullopt};
}

inline std::optional<HolonEntry> find_nearby_by_slug(
    const std::filesystem::path &root, const std::string &slug) {
  for (const auto &entry : discover(root)) {
    if (entry.slug == slug) {
      return entry;
    }
  }
  return std::nullopt;
}

} // namespace holons
