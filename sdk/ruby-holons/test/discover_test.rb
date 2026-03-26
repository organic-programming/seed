# frozen_string_literal: true

require "fileutils"
require "minitest/autorun"
require "tmpdir"
require_relative "../lib/holons"

class DiscoverTest < Minitest::Test
  def test_discover_recurses_skips_and_dedups
    Dir.mktmpdir("holons-ruby-discover-") do |root|
      write_holon(root, "holons/alpha", uuid: "uuid-alpha", given_name: "Alpha", family_name: "Go", binary: "alpha-go")
      write_holon(root, "nested/beta", uuid: "uuid-beta", given_name: "Beta", family_name: "Rust", binary: "beta-rust")
      write_holon(root, "nested/dup/alpha", uuid: "uuid-alpha", given_name: "Alpha", family_name: "Go", binary: "alpha-go")

      %w[.git/hidden .op/hidden node_modules/hidden vendor/hidden build/hidden .cache/hidden].each do |skipped|
        write_holon(root, skipped, uuid: "ignored-#{File.basename(skipped)}", given_name: "Ignored", family_name: "Holon", binary: "ignored-holon")
      end

      entries = Holons::Discover.discover(root)
      assert_equal 2, entries.length

      alpha = entries.find { |entry| entry.uuid == "uuid-alpha" }
      assert_equal "alpha-go", alpha.slug
      assert_equal "holons/alpha", alpha.relative_path
      assert_equal "go-module", alpha.manifest.build.runner

      beta = entries.find { |entry| entry.uuid == "uuid-beta" }
      assert_equal "nested/beta", beta.relative_path
    end
  end

  def test_discover_local_and_find_helpers
    Dir.mktmpdir("holons-ruby-find-") do |root|
      write_holon(
        root,
        "rob-go",
        uuid: "c7f3a1b2-1111-1111-1111-111111111111",
        given_name: "Rob",
        family_name: "Go",
        binary: "rob-go"
      )

      original_dir = Dir.pwd
      original_oppath = ENV["OPPATH"]
      original_opbin = ENV["OPBIN"]
      begin
        Dir.chdir(root)
        ENV["OPPATH"] = File.join(root, "runtime")
        ENV["OPBIN"] = File.join(root, "runtime", "bin")

        local = Holons::Discover.discover_local
        assert_equal 1, local.length
        assert_equal "rob-go", local.first.slug

        by_slug = Holons::Discover.find_by_slug("rob-go")
        refute_nil by_slug
        assert_equal "c7f3a1b2-1111-1111-1111-111111111111", by_slug.uuid

        by_uuid = Holons::Discover.find_by_uuid("c7f3a1b2")
        refute_nil by_uuid
        assert_equal "rob-go", by_uuid.slug

        assert_nil Holons::Discover.find_by_slug("missing")
      ensure
        Dir.chdir(original_dir)
        ENV["OPPATH"] = original_oppath
        ENV["OPBIN"] = original_opbin
      end
    end
  end

  def test_discover_proto_manifest_holon
    Dir.mktmpdir("holons-ruby-proto-discover-") do |root|
      write_proto_holon(
        root,
        "gabriel-greeting-ruby",
        uuid: "0d371dd4-2948-4192-8638-cee294fb8320",
        given_name: "Gabriel",
        family_name: "Greeting-Ruby",
        binary: "gabriel-greeting-ruby"
      )

      entries = Holons::Discover.discover(root)
      assert_equal 1, entries.length

      entry = entries.first
      assert_equal "gabriel-greeting-ruby", entry.slug
      assert_equal "gabriel-greeting-ruby", entry.relative_path
      assert_equal "ruby", entry.manifest.build.runner
      assert_equal "./cmd/main.rb", entry.manifest.build.main
      assert_equal "gabriel-greeting-ruby", entry.manifest.artifacts.binary
    end
  end

  private

  def write_holon(root, relative_dir, uuid:, given_name:, family_name:, binary:)
    dir = File.join(root, relative_dir)
    FileUtils.mkdir_p(dir)
    File.write(File.join(dir, "holon.proto"), <<~PROTO)
      syntax = "proto3";

      package test.v1;

      option (holons.v1.manifest) = {
        identity: {
          uuid: "#{uuid}"
          given_name: "#{given_name}"
          family_name: "#{family_name}"
          motto: "Test"
          composer: "test"
          clade: "deterministic/pure"
          status: "draft"
          born: "2026-03-07"
        }
        lineage: {
          generated_by: "test"
        }
        kind: "native"
        build: {
          runner: "go-module"
        }
        artifacts: {
          binary: "#{binary}"
        }
      };
    PROTO
  end

  def write_proto_holon(root, relative_dir, uuid:, given_name:, family_name:, binary:)
    dir = File.join(root, relative_dir, "api", "v1")
    FileUtils.mkdir_p(dir)
    File.write(File.join(dir, "holon.proto"), <<~PROTO)
      syntax = "proto3";

      package greeting.v1;

      import "holons/v1/manifest.proto";
      import "v1/greeting.proto";

      option (holons.v1.manifest) = {
        identity: {
          schema: "holon/v1"
          uuid: "#{uuid}"
          given_name: "#{given_name}"
          family_name: "#{family_name}"
          motto: "Greets users."
          composer: "test"
          status: "draft"
          born: "2026-03-16"
        }
        description: "Proto discover fixture."
        lang: "ruby"
        kind: "native"
        build: {
          runner: "ruby"
          main: "./cmd/main.rb"
        }
        artifacts: {
          binary: "#{binary}"
        }
      };
    PROTO
  end
end
