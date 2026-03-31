#include "../include/holons/connect.hpp"
#include "../include/holons/discover.hpp"

#include "echo/v1/echo.grpc.pb.h"

#include <cassert>

namespace {

using holons::ALL;
using holons::BUILT;
using holons::CACHED;
using holons::CWD;
using holons::DELEGATED;
using holons::Discover;
using holons::DiscoverResult;
using holons::HolonInfo;
using holons::HolonRef;
using holons::IdentityInfo;
using holons::INSTALLED;
using holons::LOCAL;
using holons::NO_LIMIT;
using holons::NO_TIMEOUT;
using holons::PROXY;
using holons::ResolveResult;
using holons::SIBLINGS;
using holons::SOURCE;
using holons::connect;
using holons::disconnect;
using holons::resolve;

struct PackageSeed {
  std::string slug;
  std::string uuid;
  std::string given_name;
  std::string family_name;
  std::vector<std::string> aliases;
};

std::filesystem::path make_temp_dir(const std::string &prefix) {
  auto stamp = std::to_string(
      std::chrono::high_resolution_clock::now().time_since_epoch().count());
  auto path = std::filesystem::temp_directory_path() / (prefix + stamp);
  std::filesystem::create_directories(path);
  return path;
}

struct env_guard {
  std::string name;
  std::optional<std::string> previous;

  explicit env_guard(std::string env_name, std::optional<std::string> next)
      : name(std::move(env_name)) {
    if (const char *value = std::getenv(name.c_str()); value != nullptr) {
      previous = std::string(value);
    }
    set(next);
  }

  void set(const std::optional<std::string> &value) const {
#ifdef _WIN32
    if (value.has_value()) {
      _putenv_s(name.c_str(), value->c_str());
    } else {
      _putenv_s(name.c_str(), "");
    }
#else
    if (value.has_value()) {
      ::setenv(name.c_str(), value->c_str(), 1);
    } else {
      ::unsetenv(name.c_str());
    }
#endif
  }

  ~env_guard() { set(previous); }
};

struct cwd_guard {
  std::filesystem::path previous;

  explicit cwd_guard(const std::filesystem::path &next)
      : previous(std::filesystem::current_path()) {
    std::filesystem::current_path(next);
  }

  ~cwd_guard() { std::filesystem::current_path(previous); }
};

struct runtime_fixture {
  std::filesystem::path root;
  std::filesystem::path op_home;
  std::filesystem::path op_bin;
  env_guard oppath;
  env_guard opbin;
  env_guard siblings_root;

  explicit runtime_fixture(const std::string &prefix)
      : root(make_temp_dir(prefix)),
        op_home(root / "runtime"),
        op_bin(op_home / "bin"),
        oppath("OPPATH", op_home.string()),
        opbin("OPBIN", op_bin.string()),
        siblings_root("HOLONS_SIBLINGS_ROOT", std::nullopt) {
    std::filesystem::create_directories(op_bin);
    std::filesystem::create_directories(op_home / "cache");
  }
};

struct source_bridge_guard {
  source_bridge_guard() = default;
  ~source_bridge_guard() { holons::discovery_detail::clear_source_bridge(); }
};

std::filesystem::path helper_binary_path() {
  auto current = holons::discovery_detail::current_executable_path();
  auto binary = current.parent_path() / "uniform_echo_holon";
#ifdef _WIN32
  binary += ".exe";
#endif
  return binary;
}

void copy_helper_binary(const std::filesystem::path &destination) {
  auto source = helper_binary_path();
  assert(std::filesystem::exists(source));
  std::filesystem::create_directories(destination.parent_path());
  std::filesystem::copy_file(source, destination,
                             std::filesystem::copy_options::overwrite_existing);
#ifndef _WIN32
  std::filesystem::permissions(
      destination,
      std::filesystem::perms::owner_read | std::filesystem::perms::owner_write |
          std::filesystem::perms::owner_exec,
      std::filesystem::perm_options::replace);
#endif
}

std::string file_url(const std::filesystem::path &path) {
  return holons::discovery_detail::file_url(path);
}

std::vector<std::string> sorted_slugs(const DiscoverResult &result) {
  std::vector<std::string> slugs;
  for (const auto &ref : result.found) {
    if (ref.info.has_value()) {
      slugs.push_back(ref.info->slug);
    }
  }
  std::sort(slugs.begin(), slugs.end());
  return slugs;
}

void assert_slugs(const DiscoverResult &result,
                  std::vector<std::string> expected) {
  assert(sorted_slugs(result) == expected);
}

void write_package_holon(const std::filesystem::path &package_dir,
                         const PackageSeed &seed, bool with_holon_json = true,
                         bool with_binary = false) {
  std::filesystem::create_directories(package_dir);

  if (with_holon_json) {
    nlohmann::json payload = {
        {"schema", "holon-package/v1"},
        {"slug", seed.slug},
        {"uuid", seed.uuid},
        {"identity",
         {
             {"given_name", seed.given_name},
             {"family_name", seed.family_name},
             {"motto", "Test holon"},
             {"aliases", seed.aliases},
         }},
        {"lang", "cpp"},
        {"runner", "shell"},
        {"status", "draft"},
        {"kind", "native"},
        {"transport", "tcp"},
        {"entrypoint", "uniform_echo_holon"},
        {"architectures", {holons::discovery_detail::current_arch_dir()}},
        {"has_dist", false},
        {"has_source", false},
    };

    std::ofstream out(package_dir / ".holon.json");
    assert(out.is_open());
    out << payload.dump(2) << "\n";
  }

  if (with_binary) {
    copy_helper_binary(package_dir / "bin" /
                       holons::discovery_detail::current_arch_dir() /
                       "uniform_echo_holon");
  }
}

void expect_ping(const holons::ConnectResult &result,
                 const std::string &message) {
  auto channel = holons::uniform_connect_detail::unwrap_channel(result);
  assert(channel != nullptr);

  auto stub = echo::v1::Echo::NewStub(channel);
  grpc::ClientContext context;
  echo::v1::PingRequest request;
  echo::v1::PingResponse response;
  request.set_message(message);
  auto status = stub->Ping(&context, request, &response);
  assert(status.ok());
  assert(response.message() == message);
}

} // namespace

int main() {
  int passed = 0;

  {
    runtime_fixture fixture("cpp_uniform_all_layers_");
    auto siblings_dir = fixture.root / "bundle";
    write_package_holon(siblings_dir / "bundle.holon",
                        PackageSeed{"bundle", "uuid-bundle", "Bundle", "Holon"});
    env_guard siblings("HOLONS_SIBLINGS_ROOT", siblings_dir.string());

    write_package_holon(
        fixture.root / "cwd-alpha.holon",
        PackageSeed{"cwd-alpha", "uuid-cwd-alpha", "Cwd", "Alpha"});
    write_package_holon(
        fixture.root / ".op" / "build" / "built-beta.holon",
        PackageSeed{"built-beta", "uuid-built-beta", "Built", "Beta"});
    write_package_holon(
        fixture.op_bin / "installed-gamma.holon",
        PackageSeed{"installed-gamma", "uuid-installed-gamma", "Installed",
                    "Gamma"});
    write_package_holon(
        fixture.op_home / "cache" / "deps" / "cached-delta.holon",
        PackageSeed{"cached-delta", "uuid-cached-delta", "Cached", "Delta"});

    source_bridge_guard guard;
    auto source_path = fixture.root / "source-epsilon";
    holons::discovery_detail::set_source_bridge(
        [source_path](int scope, std::optional<std::string> expression,
                      std::optional<std::string> root, int specifiers, int limit,
                      int timeout) {
          assert(scope == LOCAL);
          assert(!expression.has_value());
          assert(root.has_value());
          assert(specifiers == SOURCE);
          assert(limit == NO_LIMIT);
          assert(timeout == NO_TIMEOUT);
          (void)root;
          HolonInfo info;
          info.slug = "source-epsilon";
          info.uuid = "uuid-source-epsilon";
          info.identity = IdentityInfo{"Source", "Epsilon", "", {}};
          info.lang = "cpp";
          info.has_source = true;
          return DiscoverResult{{HolonRef{file_url(source_path), info, std::nullopt}},
                                std::nullopt};
        });

    auto result = Discover(LOCAL, std::nullopt,
                           std::optional<std::string>(fixture.root.string()),
                           ALL, NO_LIMIT, NO_TIMEOUT);
    assert(!result.error.has_value());
    assert_slugs(result, {"built-beta", "bundle", "cached-delta", "cwd-alpha",
                          "installed-gamma", "source-epsilon"});
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_filter_");
    write_package_holon(
        fixture.root / "cwd-alpha.holon",
        PackageSeed{"cwd-alpha", "uuid-cwd-alpha", "Cwd", "Alpha"});
    write_package_holon(
        fixture.root / ".op" / "build" / "built-beta.holon",
        PackageSeed{"built-beta", "uuid-built-beta", "Built", "Beta"});
    write_package_holon(
        fixture.op_bin / "installed-gamma.holon",
        PackageSeed{"installed-gamma", "uuid-installed-gamma", "Installed",
                    "Gamma"});

    auto result = Discover(LOCAL, std::nullopt,
                           std::optional<std::string>(fixture.root.string()),
                           BUILT | INSTALLED, NO_LIMIT, NO_TIMEOUT);
    assert(!result.error.has_value());
    assert_slugs(result, {"built-beta", "installed-gamma"});
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_slug_");
    write_package_holon(fixture.root / "alpha.holon",
                        PackageSeed{"alpha", "uuid-alpha", "Alpha", "One"});
    write_package_holon(fixture.root / "beta.holon",
                        PackageSeed{"beta", "uuid-beta", "Beta", "Two"});

    auto result = Discover(LOCAL, std::optional<std::string>("beta"),
                           std::optional<std::string>(fixture.root.string()), CWD,
                           NO_LIMIT, NO_TIMEOUT);
    assert(!result.error.has_value());
    assert_slugs(result, {"beta"});
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_alias_");
    write_package_holon(fixture.root / "alpha.holon",
                        PackageSeed{"alpha", "uuid-alpha", "Alpha", "One",
                                    {"first"}});

    auto result = Discover(LOCAL, std::optional<std::string>("first"),
                           std::optional<std::string>(fixture.root.string()), CWD,
                           NO_LIMIT, NO_TIMEOUT);
    assert(!result.error.has_value());
    assert_slugs(result, {"alpha"});
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_uuid_");
    write_package_holon(
        fixture.root / "alpha.holon",
        PackageSeed{"alpha", "12345678-aaaa", "Alpha", "One"});

    auto result = Discover(LOCAL, std::optional<std::string>("12345678"),
                           std::optional<std::string>(fixture.root.string()), CWD,
                           NO_LIMIT, NO_TIMEOUT);
    assert(!result.error.has_value());
    assert_slugs(result, {"alpha"});
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_path_");
    auto package_dir = fixture.root / "nested" / "alpha.holon";
    write_package_holon(package_dir,
                        PackageSeed{"alpha", "uuid-alpha", "Alpha", "One"});

    auto result = Discover(LOCAL, std::optional<std::string>("nested/alpha.holon"),
                           std::optional<std::string>(fixture.root.string()), CWD,
                           NO_LIMIT, NO_TIMEOUT);
    assert(!result.error.has_value());
    assert(result.found.size() == 1);
    assert(result.found.front().url == file_url(package_dir));
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_limit_one_");
    write_package_holon(fixture.root / "alpha.holon",
                        PackageSeed{"alpha", "uuid-alpha", "Alpha", "One"});
    write_package_holon(fixture.root / "beta.holon",
                        PackageSeed{"beta", "uuid-beta", "Beta", "Two"});

    auto result = Discover(LOCAL, std::nullopt,
                           std::optional<std::string>(fixture.root.string()), CWD,
                           1, NO_TIMEOUT);
    assert(!result.error.has_value());
    assert(result.found.size() == 1);
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_limit_zero_");
    write_package_holon(fixture.root / "alpha.holon",
                        PackageSeed{"alpha", "uuid-alpha", "Alpha", "One"});
    write_package_holon(fixture.root / "beta.holon",
                        PackageSeed{"beta", "uuid-beta", "Beta", "Two"});

    auto result = Discover(LOCAL, std::nullopt,
                           std::optional<std::string>(fixture.root.string()), CWD,
                           0, NO_TIMEOUT);
    assert(!result.error.has_value());
    assert(result.found.size() == 2);
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_negative_limit_");
    auto result = Discover(LOCAL, std::nullopt,
                           std::optional<std::string>(fixture.root.string()), CWD,
                           -1, NO_TIMEOUT);
    assert(!result.error.has_value());
    assert(result.found.empty());
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_invalid_specifiers_");
    auto result = Discover(LOCAL, std::nullopt,
                           std::optional<std::string>(fixture.root.string()),
                           0xFF, NO_LIMIT, NO_TIMEOUT);
    assert(result.error.has_value());
    assert(result.found.empty());
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_zero_all_");
    write_package_holon(
        fixture.root / "cwd-alpha.holon",
        PackageSeed{"cwd-alpha", "uuid-cwd-alpha", "Cwd", "Alpha"});
    write_package_holon(
        fixture.root / ".op" / "build" / "built-beta.holon",
        PackageSeed{"built-beta", "uuid-built-beta", "Built", "Beta"});
    write_package_holon(
        fixture.op_bin / "installed-gamma.holon",
        PackageSeed{"installed-gamma", "uuid-installed-gamma", "Installed",
                    "Gamma"});
    write_package_holon(
        fixture.op_home / "cache" / "cached-delta.holon",
        PackageSeed{"cached-delta", "uuid-cached-delta", "Cached", "Delta"});

    auto all_result = Discover(LOCAL, std::nullopt,
                               std::optional<std::string>(fixture.root.string()),
                               ALL, NO_LIMIT, NO_TIMEOUT);
    auto zero_result = Discover(LOCAL, std::nullopt,
                                std::optional<std::string>(fixture.root.string()),
                                0, NO_LIMIT, NO_TIMEOUT);
    assert(!all_result.error.has_value());
    assert(!zero_result.error.has_value());
    assert(sorted_slugs(all_result) == sorted_slugs(zero_result));
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_null_expression_");
    write_package_holon(fixture.root / "alpha.holon",
                        PackageSeed{"alpha", "uuid-alpha", "Alpha", "One"});
    write_package_holon(fixture.root / "beta.holon",
                        PackageSeed{"beta", "uuid-beta", "Beta", "Two"});

    auto result = Discover(LOCAL, std::nullopt,
                           std::optional<std::string>(fixture.root.string()), CWD,
                           NO_LIMIT, NO_TIMEOUT);
    assert(!result.error.has_value());
    assert(result.found.size() == 2);
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_missing_expression_");
    write_package_holon(fixture.root / "alpha.holon",
                        PackageSeed{"alpha", "uuid-alpha", "Alpha", "One"});

    auto result =
        Discover(LOCAL, std::optional<std::string>(""), std::optional<std::string>(fixture.root.string()),
                 CWD, NO_LIMIT, NO_TIMEOUT);
    assert(!result.error.has_value());
    assert(result.found.empty());
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_skips_excluded_");
    write_package_holon(fixture.root / "kept" / "alpha.holon",
                        PackageSeed{"alpha", "uuid-alpha", "Alpha", "One"});
    write_package_holon(fixture.root / ".git" / "ignored" / "one.holon",
                        PackageSeed{"ignored-git", "uuid-git", "Ignored", "Git"});
    write_package_holon(fixture.root / ".op" / "ignored" / "two.holon",
                        PackageSeed{"ignored-op", "uuid-op", "Ignored", "Op"});
    write_package_holon(fixture.root / "node_modules" / "ignored" / "three.holon",
                        PackageSeed{"ignored-node", "uuid-node", "Ignored", "Node"});
    write_package_holon(fixture.root / "vendor" / "ignored" / "four.holon",
                        PackageSeed{"ignored-vendor", "uuid-vendor", "Ignored", "Vendor"});
    write_package_holon(fixture.root / "build" / "ignored" / "five.holon",
                        PackageSeed{"ignored-build", "uuid-build", "Ignored", "Build"});
    write_package_holon(fixture.root / "testdata" / "ignored" / "six.holon",
                        PackageSeed{"ignored-testdata", "uuid-testdata", "Ignored", "Testdata"});
    write_package_holon(fixture.root / ".cache" / "ignored" / "seven.holon",
                        PackageSeed{"ignored-cache", "uuid-cache", "Ignored", "Cache"});

    auto result = Discover(LOCAL, std::nullopt,
                           std::optional<std::string>(fixture.root.string()), CWD,
                           NO_LIMIT, NO_TIMEOUT);
    assert(!result.error.has_value());
    assert_slugs(result, {"alpha"});
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_dedupe_");
    auto cwd_dir = fixture.root / "alpha.holon";
    auto built_dir = fixture.root / ".op" / "build" / "alpha-built.holon";
    write_package_holon(cwd_dir,
                        PackageSeed{"alpha", "uuid-alpha", "Alpha", "One"});
    write_package_holon(built_dir,
                        PackageSeed{"alpha-built", "uuid-alpha", "Alpha", "One"});

    auto result = Discover(LOCAL, std::nullopt,
                           std::optional<std::string>(fixture.root.string()), ALL,
                           NO_LIMIT, NO_TIMEOUT);
    assert(!result.error.has_value());
    assert(result.found.size() == 1);
    assert(result.found.front().url == file_url(cwd_dir));
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_fast_path_");
    auto package_dir = fixture.root / "alpha.holon";
    write_package_holon(package_dir,
                        PackageSeed{"alpha", "uuid-alpha", "Alpha", "One"},
                        true, false);

    auto result = Discover(LOCAL, std::nullopt,
                           std::optional<std::string>(fixture.root.string()), CWD,
                           NO_LIMIT, NO_TIMEOUT);
    assert(!result.error.has_value());
    assert(result.found.size() == 1);
    assert(result.found.front().info.has_value());
    assert(result.found.front().info->slug == "alpha");
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_describe_fallback_");
    auto package_dir = fixture.root / "alpha.holon";
    write_package_holon(package_dir,
                        PackageSeed{"alpha", "uuid-alpha", "Alpha", "One"},
                        false, true);

    auto result = Discover(LOCAL, std::nullopt,
                           std::optional<std::string>(fixture.root.string()), CWD,
                           NO_LIMIT, 5000);
    assert(!result.error.has_value());
    assert(result.found.size() == 1);
    assert(result.found.front().info.has_value());
    assert(result.found.front().info->slug == "echo-server");
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_siblings_");
    auto siblings_dir = fixture.root / "siblings";
    write_package_holon(siblings_dir / "bundle.holon",
                        PackageSeed{"bundle", "uuid-bundle", "Bundle", "Holon"});
    env_guard siblings("HOLONS_SIBLINGS_ROOT", siblings_dir.string());

    auto result = Discover(LOCAL, std::nullopt,
                           std::optional<std::string>(fixture.root.string()),
                           SIBLINGS, NO_LIMIT, NO_TIMEOUT);
    assert(!result.error.has_value());
    assert_slugs(result, {"bundle"});
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_source_");
    auto source_dir = fixture.root / "source-holon";
    std::filesystem::create_directories(source_dir);

    source_bridge_guard guard;
    std::vector<std::tuple<int, std::optional<std::string>, std::optional<std::string>,
                           int, int, int>>
        calls;
    holons::discovery_detail::set_source_bridge(
        [&calls, source_dir](int scope, std::optional<std::string> expression,
                             std::optional<std::string> root, int specifiers,
                             int limit, int timeout) {
          calls.emplace_back(scope, expression, root, specifiers, limit, timeout);
          HolonInfo info;
          info.slug = "source-alpha";
          info.uuid = "uuid-source-alpha";
          info.identity = IdentityInfo{"Source", "Alpha", "", {}};
          info.lang = "cpp";
          info.runner = "shell";
          info.status = "draft";
          info.kind = "native";
          info.transport = "stdio";
          info.entrypoint = "uniform_echo_holon";
          info.has_source = true;
          return DiscoverResult{{HolonRef{file_url(source_dir), info, std::nullopt}},
                                std::nullopt};
        });

    auto result = Discover(LOCAL, std::nullopt,
                           std::optional<std::string>(fixture.root.string()),
                           SOURCE, NO_LIMIT, 5000);
    assert(!result.error.has_value());
    assert_slugs(result, {"source-alpha"});
    assert(calls.size() == 1);
    assert(std::get<0>(calls.front()) == LOCAL);
    assert(!std::get<1>(calls.front()).has_value());
    assert(std::get<2>(calls.front()).has_value());
    assert(*std::get<2>(calls.front()) ==
           std::filesystem::weakly_canonical(fixture.root).string());
    assert(std::get<3>(calls.front()) == SOURCE);
    assert(std::get<4>(calls.front()) == NO_LIMIT);
    assert(std::get<5>(calls.front()) == 5000);
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_built_");
    write_package_holon(
        fixture.root / ".op" / "build" / "built.holon",
        PackageSeed{"built", "uuid-built", "Built", "Holon"});

    auto result = Discover(LOCAL, std::nullopt,
                           std::optional<std::string>(fixture.root.string()),
                           BUILT, NO_LIMIT, NO_TIMEOUT);
    assert(!result.error.has_value());
    assert_slugs(result, {"built"});
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_installed_");
    write_package_holon(
        fixture.op_bin / "installed.holon",
        PackageSeed{"installed", "uuid-installed", "Installed", "Holon"});

    auto result = Discover(LOCAL, std::nullopt,
                           std::optional<std::string>(fixture.root.string()),
                           INSTALLED, NO_LIMIT, NO_TIMEOUT);
    assert(!result.error.has_value());
    assert_slugs(result, {"installed"});
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_cached_");
    write_package_holon(
        fixture.op_home / "cache" / "deep" / "cached.holon",
        PackageSeed{"cached", "uuid-cached", "Cached", "Holon"});

    auto result = Discover(LOCAL, std::nullopt,
                           std::optional<std::string>(fixture.root.string()),
                           CACHED, NO_LIMIT, NO_TIMEOUT);
    assert(!result.error.has_value());
    assert_slugs(result, {"cached"});
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_nil_root_");
    write_package_holon(fixture.root / "alpha.holon",
                        PackageSeed{"alpha", "uuid-alpha", "Alpha", "One"});
    cwd_guard cwd(fixture.root);

    auto result = Discover(LOCAL, std::nullopt, std::nullopt, CWD, NO_LIMIT,
                           NO_TIMEOUT);
    assert(!result.error.has_value());
    assert_slugs(result, {"alpha"});
    ++passed;
  }

  {
    auto result = Discover(LOCAL, std::nullopt, std::optional<std::string>(""),
                           ALL, NO_LIMIT, NO_TIMEOUT);
    assert(result.error.has_value());
    assert(result.found.empty());
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_scope_");
    auto proxy_result = Discover(PROXY, std::nullopt,
                                 std::optional<std::string>(fixture.root.string()),
                                 ALL, NO_LIMIT, NO_TIMEOUT);
    auto delegated_result =
        Discover(DELEGATED, std::nullopt,
                 std::optional<std::string>(fixture.root.string()), ALL,
                 NO_LIMIT, NO_TIMEOUT);
    assert(proxy_result.error.has_value());
    assert(delegated_result.error.has_value());
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_resolve_known_");
    write_package_holon(fixture.root / "alpha.holon",
                        PackageSeed{"alpha", "uuid-alpha", "Alpha", "One"});

    auto result =
        resolve(LOCAL, "alpha", std::optional<std::string>(fixture.root.string()),
                CWD, NO_TIMEOUT);
    assert(!result.error.has_value());
    assert(result.ref.has_value());
    assert(result.ref->info.has_value());
    assert(result.ref->info->slug == "alpha");
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_resolve_missing_");
    auto result =
        resolve(LOCAL, "missing",
                std::optional<std::string>(fixture.root.string()), CWD,
                NO_TIMEOUT);
    assert(result.error.has_value());
    assert(!result.ref.has_value());
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_resolve_invalid_");
    auto result =
        resolve(LOCAL, "alpha",
                std::optional<std::string>(fixture.root.string()), 0xFF,
                NO_TIMEOUT);
    assert(result.error.has_value());
    assert(!result.ref.has_value());
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_connect_missing_");
    auto result =
        connect(LOCAL, "missing",
                std::optional<std::string>(fixture.root.string()), INSTALLED,
                1000);
    assert(result.error.has_value());
    assert(result.channel == nullptr);
    assert(!result.origin.has_value());
    ++passed;
  }

  {
    runtime_fixture fixture("cpp_uniform_connect_result_");
    write_package_holon(
        fixture.op_bin / "known-slug.holon",
        PackageSeed{"known-slug", "uuid-known", "Known", "Slug"}, true, true);

    auto result =
        connect(LOCAL, "known-slug",
                std::optional<std::string>(fixture.root.string()), INSTALLED,
                5000);
    try {
      assert(result.error == std::nullopt);
      assert(result.channel != nullptr);
      expect_ping(result, "connect-cpp");
      ++passed;
    } catch (...) {
      disconnect(result);
      throw;
    }
    disconnect(result);
  }

  {
    runtime_fixture fixture("cpp_uniform_connect_origin_");
    auto package_root = fixture.op_bin / "origin-slug.holon";
    write_package_holon(package_root,
                        PackageSeed{"origin-slug", "uuid-origin", "Origin",
                                    "Slug"},
                        true, true);

    auto result =
        connect(LOCAL, "origin-slug",
                std::optional<std::string>(fixture.root.string()), INSTALLED,
                5000);
    try {
      assert(!result.error.has_value());
      assert(result.origin.has_value());
      assert(result.origin->info.has_value());
      assert(result.origin->info->slug == "origin-slug");
      ++passed;
    } catch (...) {
      disconnect(result);
      throw;
    }
    disconnect(result);
  }

  {
    runtime_fixture fixture("cpp_uniform_disconnect_");
    write_package_holon(
        fixture.op_bin / "disconnect-slug.holon",
        PackageSeed{"disconnect-slug", "uuid-disconnect", "Disconnect", "Slug"},
        true, true);

    auto result =
        connect(LOCAL, "disconnect-slug",
                std::optional<std::string>(fixture.root.string()), INSTALLED,
                5000);
    assert(!result.error.has_value());
    assert(result.channel != nullptr);
    disconnect(result);
    assert(result.channel == nullptr);
    ++passed;
  }

  std::printf("%d passed, 0 failed\n", passed);
  return 0;
}
