# frozen_string_literal: true

require "fileutils"
require "minitest/autorun"
require "tmpdir"
require "uri"
require_relative "../lib/holons"

class DiscoverTest < Minitest::Test
  def teardown
    Holons::Discover.set_probe(nil)
    Holons::Discover.set_source_bridge(nil)
    Holons::Discover.set_executable_path_resolver(nil)
  end

  def test_discover_all_layers
    with_runtime_fixture do |root, op_home, op_bin|
      write_package_holon(File.join(bundle_root(root), "siblings-alpha.holon"), slug: "siblings-alpha", uuid: "uuid-siblings", given_name: "Siblings", family_name: "Alpha", entrypoint: "siblings-alpha")
      write_package_holon(File.join(root, "cwd-beta.holon"), slug: "cwd-beta", uuid: "uuid-cwd", given_name: "Cwd", family_name: "Beta", entrypoint: "cwd-beta")
      write_package_holon(File.join(root, ".op", "build", "built-delta.holon"), slug: "built-delta", uuid: "uuid-built", given_name: "Built", family_name: "Delta", entrypoint: "built-delta")
      write_package_holon(File.join(op_bin, "installed-epsilon.holon"), slug: "installed-epsilon", uuid: "uuid-installed", given_name: "Installed", family_name: "Epsilon", entrypoint: "installed-epsilon")
      write_package_holon(File.join(op_home, "cache", "deps", "cached-zeta.holon"), slug: "cached-zeta", uuid: "uuid-cached", given_name: "Cached", family_name: "Zeta", entrypoint: "cached-zeta")

      Holons::Discover.set_executable_path_resolver(-> { fake_app_executable(root) })
      Holons::Discover.set_source_bridge(lambda do |_scope, _expression, bridge_root, _specifiers, _limit, _timeout|
        source_ref(bridge_root, "source-gamma", uuid: "uuid-source", given_name: "Source", family_name: "Gamma")
      end)

      result = Holons.discover(Holons::LOCAL, nil, root, Holons::ALL, Holons::NO_LIMIT, Holons::NO_TIMEOUT)
      assert_nil result.error
      assert_equal %w[built-delta cached-zeta cwd-beta installed-epsilon siblings-alpha source-gamma], sorted_slugs(result)
    end
  end

  def test_filter_by_specifiers
    with_runtime_fixture do |root, _op_home, op_bin|
      write_package_holon(File.join(root, "cwd-alpha.holon"), slug: "cwd-alpha", uuid: "uuid-cwd", given_name: "Cwd", family_name: "Alpha", entrypoint: "cwd-alpha")
      write_package_holon(File.join(root, ".op", "build", "built-beta.holon"), slug: "built-beta", uuid: "uuid-built", given_name: "Built", family_name: "Beta", entrypoint: "built-beta")
      write_package_holon(File.join(op_bin, "installed-gamma.holon"), slug: "installed-gamma", uuid: "uuid-installed", given_name: "Installed", family_name: "Gamma", entrypoint: "installed-gamma")

      result = Holons.discover(Holons::LOCAL, nil, root, Holons::BUILT | Holons::INSTALLED, Holons::NO_LIMIT, Holons::NO_TIMEOUT)
      assert_nil result.error
      assert_equal %w[built-beta installed-gamma], sorted_slugs(result)
    end
  end

  def test_match_by_slug
    with_runtime_fixture do |root, _op_home, _op_bin|
      write_package_holon(File.join(root, "alpha.holon"), slug: "alpha", uuid: "uuid-alpha", given_name: "Alpha", family_name: "One", entrypoint: "alpha")
      write_package_holon(File.join(root, "beta.holon"), slug: "beta", uuid: "uuid-beta", given_name: "Beta", family_name: "Two", entrypoint: "beta")

      result = Holons.discover(Holons::LOCAL, "beta", root, Holons::CWD, Holons::NO_LIMIT, Holons::NO_TIMEOUT)
      assert_nil result.error
      assert_equal ["beta"], sorted_slugs(result)
    end
  end

  def test_match_by_alias
    with_runtime_fixture do |root, _op_home, _op_bin|
      write_package_holon(File.join(root, "alpha.holon"), slug: "alpha", uuid: "uuid-alpha", given_name: "Alpha", family_name: "One", aliases: ["first"], entrypoint: "alpha")

      result = Holons.discover(Holons::LOCAL, "first", root, Holons::CWD, Holons::NO_LIMIT, Holons::NO_TIMEOUT)
      assert_nil result.error
      assert_equal ["alpha"], sorted_slugs(result)
    end
  end

  def test_match_by_uuid_prefix
    with_runtime_fixture do |root, _op_home, _op_bin|
      write_package_holon(File.join(root, "alpha.holon"), slug: "alpha", uuid: "12345678-aaaa", given_name: "Alpha", family_name: "One", entrypoint: "alpha")

      result = Holons.discover(Holons::LOCAL, "12345678", root, Holons::CWD, Holons::NO_LIMIT, Holons::NO_TIMEOUT)
      assert_nil result.error
      assert_equal ["alpha"], sorted_slugs(result)
    end
  end

  def test_match_by_path
    with_runtime_fixture do |root, _op_home, _op_bin|
      dir = File.join(root, "alpha.holon")
      write_package_holon(dir, slug: "alpha", uuid: "uuid-alpha", given_name: "Alpha", family_name: "One", entrypoint: "alpha")

      result = Holons.discover(Holons::LOCAL, dir, root, Holons::CWD, Holons::NO_LIMIT, Holons::NO_TIMEOUT)
      assert_nil result.error
      assert_equal 1, result.found.length
      assert_equal "alpha", result.found.first.info.slug
    end
  end

  def test_limit_one
    with_runtime_fixture do |root, _op_home, _op_bin|
      write_package_holon(File.join(root, "alpha.holon"), slug: "alpha", uuid: "uuid-alpha", given_name: "Alpha", family_name: "One", entrypoint: "alpha")
      write_package_holon(File.join(root, "beta.holon"), slug: "beta", uuid: "uuid-beta", given_name: "Beta", family_name: "Two", entrypoint: "beta")

      result = Holons.discover(Holons::LOCAL, nil, root, Holons::CWD, 1, Holons::NO_TIMEOUT)
      assert_nil result.error
      assert_equal 1, result.found.length
    end
  end

  def test_limit_zero_means_unlimited
    with_runtime_fixture do |root, _op_home, _op_bin|
      write_package_holon(File.join(root, "alpha.holon"), slug: "alpha", uuid: "uuid-alpha", given_name: "Alpha", family_name: "One", entrypoint: "alpha")
      write_package_holon(File.join(root, "beta.holon"), slug: "beta", uuid: "uuid-beta", given_name: "Beta", family_name: "Two", entrypoint: "beta")

      result = Holons.discover(Holons::LOCAL, nil, root, Holons::CWD, 0, Holons::NO_TIMEOUT)
      assert_nil result.error
      assert_equal 2, result.found.length
    end
  end

  def test_negative_limit_returns_empty
    with_runtime_fixture do |root, _op_home, _op_bin|
      result = Holons.discover(Holons::LOCAL, nil, root, Holons::CWD, -1, Holons::NO_TIMEOUT)
      assert_nil result.error
      assert_empty result.found
    end
  end

  def test_invalid_specifiers
    with_runtime_fixture do |root, _op_home, _op_bin|
      result = Holons.discover(Holons::LOCAL, nil, root, 0xFF, Holons::NO_LIMIT, Holons::NO_TIMEOUT)
      refute_nil result.error
    end
  end

  def test_specifiers_zero_treated_as_all
    with_runtime_fixture do |root, op_home, op_bin|
      write_package_holon(File.join(root, "cwd-alpha.holon"), slug: "cwd-alpha", uuid: "uuid-cwd", given_name: "Cwd", family_name: "Alpha", entrypoint: "cwd-alpha")
      write_package_holon(File.join(root, ".op", "build", "built-beta.holon"), slug: "built-beta", uuid: "uuid-built", given_name: "Built", family_name: "Beta", entrypoint: "built-beta")
      write_package_holon(File.join(op_bin, "installed-gamma.holon"), slug: "installed-gamma", uuid: "uuid-installed", given_name: "Installed", family_name: "Gamma", entrypoint: "installed-gamma")
      write_package_holon(File.join(op_home, "cache", "cached-delta.holon"), slug: "cached-delta", uuid: "uuid-cached", given_name: "Cached", family_name: "Delta", entrypoint: "cached-delta")

      all_result = Holons.discover(Holons::LOCAL, nil, root, Holons::ALL, Holons::NO_LIMIT, Holons::NO_TIMEOUT)
      zero_result = Holons.discover(Holons::LOCAL, nil, root, 0, Holons::NO_LIMIT, Holons::NO_TIMEOUT)
      assert_equal sorted_slugs(all_result), sorted_slugs(zero_result)
    end
  end

  def test_null_expression_returns_all
    with_runtime_fixture do |root, _op_home, _op_bin|
      write_package_holon(File.join(root, "alpha.holon"), slug: "alpha", uuid: "uuid-alpha", given_name: "Alpha", family_name: "One", entrypoint: "alpha")
      write_package_holon(File.join(root, "beta.holon"), slug: "beta", uuid: "uuid-beta", given_name: "Beta", family_name: "Two", entrypoint: "beta")

      result = Holons.discover(Holons::LOCAL, nil, root, Holons::CWD, Holons::NO_LIMIT, Holons::NO_TIMEOUT)
      assert_nil result.error
      assert_equal %w[alpha beta], sorted_slugs(result)
    end
  end

  def test_missing_expression_returns_empty
    with_runtime_fixture do |root, _op_home, _op_bin|
      write_package_holon(File.join(root, "alpha.holon"), slug: "alpha", uuid: "uuid-alpha", given_name: "Alpha", family_name: "One", entrypoint: "alpha")

      result = Holons.discover(Holons::LOCAL, "missing", root, Holons::CWD, Holons::NO_LIMIT, Holons::NO_TIMEOUT)
      assert_nil result.error
      assert_empty result.found
    end
  end

  def test_excluded_dirs_skipped
    with_runtime_fixture do |root, _op_home, _op_bin|
      write_source_holon(File.join(root, "kept"), uuid: "uuid-kept", given_name: "Kept", family_name: "Holon", binary: "kept")
      %w[.git .op node_modules vendor build testdata .cache].each do |name|
        write_source_holon(File.join(root, name, "ignored"), uuid: "uuid-#{name}", given_name: "Ignored", family_name: "Holon", binary: "ignored")
      end

      result = Holons.discover(Holons::LOCAL, nil, root, Holons::SOURCE, Holons::NO_LIMIT, Holons::NO_TIMEOUT)
      assert_nil result.error
      assert_equal ["kept-holon"], sorted_slugs(result)
    end
  end

  def test_deduplicate_by_uuid
    with_runtime_fixture do |root, _op_home, _op_bin|
      cwd_dir = File.join(root, "alpha.holon")
      built_dir = File.join(root, ".op", "build", "alpha-built.holon")
      write_package_holon(cwd_dir, slug: "alpha", uuid: "uuid-alpha", given_name: "Alpha", family_name: "One", entrypoint: "alpha")
      write_package_holon(built_dir, slug: "alpha-built", uuid: "uuid-alpha", given_name: "Alpha", family_name: "One", entrypoint: "alpha-built")

      result = Holons.discover(Holons::LOCAL, nil, root, Holons::ALL, Holons::NO_LIMIT, Holons::NO_TIMEOUT)
      assert_nil result.error
      assert_equal 1, result.found.length
      assert_equal "alpha", result.found.first.info.slug
      assert_equal file_url(cwd_dir), result.found.first.url
    end
  end

  def test_holon_json_fast_path
    with_runtime_fixture do |root, _op_home, _op_bin|
      write_package_holon(File.join(root, "alpha.holon"), slug: "alpha", uuid: "uuid-alpha", given_name: "Alpha", family_name: "One", entrypoint: "alpha")

      probe_calls = 0
      Holons::Discover.set_probe(lambda do |_dir|
        probe_calls += 1
        nil
      end)

      result = Holons.discover(Holons::LOCAL, nil, root, Holons::CWD, Holons::NO_LIMIT, Holons::NO_TIMEOUT)
      assert_nil result.error
      assert_equal ["alpha"], sorted_slugs(result)
      assert_equal 0, probe_calls
    end
  end

  def test_describe_fallback_when_holon_json_missing
    with_runtime_fixture do |root, _op_home, _op_bin|
      package_dir = File.join(root, "alpha.holon")
      FileUtils.mkdir_p(package_dir)

      probe_calls = 0
      Holons::Discover.set_probe(lambda do |dir|
        probe_calls += 1
        next nil unless File.expand_path(dir) == File.expand_path(package_dir)

        Holons::HolonInfo.new(
          slug: "alpha",
          uuid: "uuid-alpha",
          identity: Holons::IdentityInfo.new(given_name: "Alpha", family_name: "One", motto: "", aliases: []),
          lang: "ruby",
          runner: "ruby",
          status: "draft",
          kind: "native",
          transport: "stdio",
          entrypoint: "alpha",
          architectures: [],
          has_dist: false,
          has_source: false
        )
      end)

      result = Holons.discover(Holons::LOCAL, nil, root, Holons::CWD, Holons::NO_LIMIT, Holons::NO_TIMEOUT)
      assert_nil result.error
      assert_equal ["alpha"], sorted_slugs(result)
      assert_equal 1, probe_calls
    end
  end

  def test_siblings_layer
    with_runtime_fixture do |root, _op_home, _op_bin|
      write_package_holon(File.join(bundle_root(root), "bundle.holon"), slug: "bundle", uuid: "uuid-bundle", given_name: "Bundle", family_name: "Holon", entrypoint: "bundle")
      Holons::Discover.set_executable_path_resolver(-> { fake_app_executable(root) })

      result = Holons.discover(Holons::LOCAL, nil, root, Holons::SIBLINGS, Holons::NO_LIMIT, Holons::NO_TIMEOUT)
      assert_nil result.error
      assert_equal ["bundle"], sorted_slugs(result)
    end
  end

  def test_source_layer_offloads_to_local_op
    with_runtime_fixture do |root, _op_home, _op_bin|
      calls = []
      Holons::Discover.set_source_bridge(lambda do |scope, expression, bridge_root, specifiers, limit, timeout|
        calls << [scope, expression, bridge_root, specifiers, limit, timeout]
        Holons::DiscoverResult.new(found: [source_ref(bridge_root, "source-offloaded", uuid: "uuid-source", given_name: "Source", family_name: "Offloaded")], error: nil)
      end)

      result = Holons.discover(Holons::LOCAL, nil, root, Holons::SOURCE, Holons::NO_LIMIT, 1234)
      assert_nil result.error
      assert_equal ["source-offloaded"], sorted_slugs(result)
      assert_equal [[Holons::LOCAL, nil, root, Holons::SOURCE, Holons::NO_LIMIT, 1234]], calls
    end
  end

  def test_built_layer
    with_runtime_fixture do |root, _op_home, _op_bin|
      write_package_holon(File.join(root, ".op", "build", "built.holon"), slug: "built", uuid: "uuid-built", given_name: "Built", family_name: "Holon", entrypoint: "built")

      result = Holons.discover(Holons::LOCAL, nil, root, Holons::BUILT, Holons::NO_LIMIT, Holons::NO_TIMEOUT)
      assert_nil result.error
      assert_equal ["built"], sorted_slugs(result)
    end
  end

  def test_installed_layer
    with_runtime_fixture do |root, _op_home, op_bin|
      write_package_holon(File.join(op_bin, "installed.holon"), slug: "installed", uuid: "uuid-installed", given_name: "Installed", family_name: "Holon", entrypoint: "installed")

      result = Holons.discover(Holons::LOCAL, nil, root, Holons::INSTALLED, Holons::NO_LIMIT, Holons::NO_TIMEOUT)
      assert_nil result.error
      assert_equal ["installed"], sorted_slugs(result)
    end
  end

  def test_cached_layer
    with_runtime_fixture do |root, op_home, _op_bin|
      write_package_holon(File.join(op_home, "cache", "deep", "cached.holon"), slug: "cached", uuid: "uuid-cached", given_name: "Cached", family_name: "Holon", entrypoint: "cached")

      result = Holons.discover(Holons::LOCAL, nil, root, Holons::CACHED, Holons::NO_LIMIT, Holons::NO_TIMEOUT)
      assert_nil result.error
      assert_equal ["cached"], sorted_slugs(result)
    end
  end

  def test_nil_root_defaults_to_cwd
    with_runtime_fixture do |root, _op_home, _op_bin|
      write_package_holon(File.join(root, "alpha.holon"), slug: "alpha", uuid: "uuid-alpha", given_name: "Alpha", family_name: "One", entrypoint: "alpha")

      Dir.chdir(root) do
        result = Holons.discover(Holons::LOCAL, nil, nil, Holons::CWD, Holons::NO_LIMIT, Holons::NO_TIMEOUT)
        assert_nil result.error
        assert_equal ["alpha"], sorted_slugs(result)
      end
    end
  end

  def test_empty_root_returns_error
    result = Holons.discover(Holons::LOCAL, nil, "", Holons::ALL, Holons::NO_LIMIT, Holons::NO_TIMEOUT)
    refute_nil result.error
  end

  def test_unsupported_scope_returns_error
    with_runtime_fixture do |root, _op_home, _op_bin|
      proxy_result = Holons.discover(Holons::PROXY, nil, root, Holons::ALL, Holons::NO_LIMIT, Holons::NO_TIMEOUT)
      delegated_result = Holons.discover(Holons::DELEGATED, nil, root, Holons::ALL, Holons::NO_LIMIT, Holons::NO_TIMEOUT)

      refute_nil proxy_result.error
      refute_nil delegated_result.error
    end
  end

  def test_resolve_known_slug
    with_runtime_fixture do |root, _op_home, _op_bin|
      write_package_holon(File.join(root, "alpha.holon"), slug: "alpha", uuid: "uuid-alpha", given_name: "Alpha", family_name: "One", entrypoint: "alpha")

      result = Holons.resolve(Holons::LOCAL, "alpha", root, Holons::CWD, Holons::NO_TIMEOUT)
      assert_nil result.error
      assert_equal "alpha", result.ref.info.slug
    end
  end

  def test_resolve_missing_target
    with_runtime_fixture do |root, _op_home, _op_bin|
      result = Holons.resolve(Holons::LOCAL, "missing", root, Holons::ALL, Holons::NO_TIMEOUT)
      refute_nil result.error
    end
  end

  def test_resolve_invalid_specifiers
    with_runtime_fixture do |root, _op_home, _op_bin|
      result = Holons.resolve(Holons::LOCAL, "alpha", root, 0xFF, Holons::NO_TIMEOUT)
      refute_nil result.error
    end
  end

  private

  def with_runtime_fixture
    Dir.mktmpdir("ruby-holons-discover-") do |root|
      op_home = File.join(root, "runtime")
      op_bin = File.join(op_home, "bin")
      old_oppath = ENV["OPPATH"]
      old_opbin = ENV["OPBIN"]
      ENV["OPPATH"] = op_home
      ENV["OPBIN"] = op_bin
      yield root, op_home, op_bin
    ensure
      ENV["OPPATH"] = old_oppath
      ENV["OPBIN"] = old_opbin
    end
  end

  def write_package_holon(dir, slug:, uuid:, given_name:, family_name:, entrypoint:, aliases: [])
    FileUtils.mkdir_p(dir)
    File.write(File.join(dir, ".holon.json"), <<~JSON)
      {
        "schema": "holon-package/v1",
        "slug": #{slug.inspect},
        "uuid": #{uuid.inspect},
        "identity": {
          "given_name": #{given_name.inspect},
          "family_name": #{family_name.inspect},
          "aliases": #{aliases.inspect}
        },
        "lang": "ruby",
        "runner": "ruby",
        "status": "draft",
        "kind": "native",
        "transport": "stdio",
        "entrypoint": #{entrypoint.inspect},
        "architectures": [],
        "has_dist": false,
        "has_source": false
      }
    JSON
  end

  def write_source_holon(dir, uuid:, given_name:, family_name:, binary:, aliases: [])
    FileUtils.mkdir_p(dir)
    aliases_block = aliases.empty? ? "" : "    aliases: [#{aliases.map(&:inspect).join(', ')}]\n"
    File.write(File.join(dir, "holon.proto"), <<~PROTO)
      syntax = "proto3";

      package discover.v1;

      option (holons.v1.manifest) = {
        identity: {
          uuid: #{uuid.inspect}
          given_name: #{given_name.inspect}
          family_name: #{family_name.inspect}
#{aliases_block}        }
        kind: "native"
        lang: "ruby"
        build: {
          runner: "ruby"
          main: "./cmd/main.rb"
        }
        artifacts: {
          binary: #{binary.inspect}
        }
      };
    PROTO
  end

  def source_ref(root, relative_dir, uuid:, given_name:, family_name:)
    dir = File.join(root, relative_dir)
    FileUtils.mkdir_p(dir)
    Holons::HolonRef.new(
      url: file_url(dir),
      info: Holons::HolonInfo.new(
        slug: "#{given_name}-#{family_name}".downcase,
        uuid: uuid,
        identity: Holons::IdentityInfo.new(given_name: given_name, family_name: family_name, motto: "", aliases: []),
        lang: "ruby",
        runner: "ruby",
        status: "draft",
        kind: "native",
        transport: "stdio",
        entrypoint: "source-bin",
        architectures: [],
        has_dist: false,
        has_source: true
      ),
      error: nil
    )
  end

  def sorted_slugs(result)
    result.found.map { |ref| ref.info&.slug }.compact.sort
  end

  def bundle_root(root)
    File.join(root, "TestApp.app", "Contents", "Resources", "Holons")
  end

  def fake_app_executable(root)
    path = File.join(root, "TestApp.app", "Contents", "MacOS", "TestApp")
    FileUtils.mkdir_p(File.dirname(path))
    File.write(path, "#!/bin/sh\n")
    File.chmod(0o755, path)
    path
  end

  def file_url(path)
    URI::Generic.build(scheme: "file", path: File.expand_path(path).tr(File::SEPARATOR, "/")).to_s
  end
end
