# frozen_string_literal: true

require "fileutils"
require "minitest/autorun"
require "open3"
require "rbconfig"
require "timeout"
require "tmpdir"
require_relative "../lib/holons"

begin
  require_relative "../lib/gen/holons/v1/manifest_pb"
  require_relative "../lib/gen/holons/v1/describe_pb"
  require_relative "../lib/gen/holons/v1/describe_services_pb"
  HOLONMETA_RUNTIME_AVAILABLE = true
rescue LoadError
  HOLONMETA_RUNTIME_AVAILABLE = false
end

class ServeTest < Minitest::Test
  def setup
    skip "grpc gem is unavailable in this Ruby environment" unless Holons.grpc_available?
    skip "google-protobuf support is unavailable in this Ruby environment" unless HOLONMETA_RUNTIME_AVAILABLE

    begin
      server = TCPServer.new("127.0.0.1", 0)
      server.close
    rescue Errno::EPERM => e
      skip "local socket bind is denied in this environment: #{e.message}"
    end
  end

  def test_run_with_options_serves_registered_static_describe_over_tcp
    with_serve_fixture do |fixture|
      stdin, stdout, stderr, wait_thr = Open3.popen3(
        RbConfig.ruby,
        fixture[:script_path],
        "serve",
        "--listen",
        "tcp://127.0.0.1:0",
        chdir: fixture[:holon_dir]
      )

      begin
        uri = read_advertised_uri(stdout, stderr)
        channel = Holons.connect(uri)
        begin
          response = describe(channel)
          assert_equal "Serve", response.manifest.identity.given_name
          assert_equal "Helper", response.manifest.identity.family_name
          assert_equal "Reply precisely.", response.manifest.identity.motto
          assert_equal ["echo.v1.Echo"], response.services.map(&:name)
        ensure
          Holons.disconnect(channel)
        end
      ensure
        terminate_process(wait_thr.pid)
        stdin.close unless stdin.closed?
        stdout.close unless stdout.closed?
        stderr.close unless stderr.closed?
      end
    end
  end

  def test_run_with_options_fails_without_registered_static_describe
    with_serve_fixture(register_static: false) do |fixture|
      stdin, stdout, stderr, wait_thr = Open3.popen3(
        RbConfig.ruby,
        fixture[:script_path],
        "serve",
        "--listen",
        "tcp://127.0.0.1:0",
        chdir: fixture[:holon_dir]
      )

      status = wait_thr.value
      error_output = stderr.read

      refute status.success?
      assert_includes error_output, "no Incode Description registered — run op build"
    ensure
      stdin.close unless stdin.nil? || stdin.closed?
      stdout.close unless stdout.nil? || stdout.closed?
      stderr.close unless stderr.nil? || stderr.closed?
    end
  end

  def test_connect_slug_reaches_stdio_serve_helper
    with_serve_fixture do |fixture|
      with_holon_root(fixture[:workspace]) do
        channel = Holons.connect(fixture[:slug])
        begin
          response = describe(channel)
          assert_equal "Serve", response.manifest.identity.given_name
          assert_equal ["echo.v1.Echo"], response.services.map(&:name)
        ensure
          Holons.disconnect(channel)
        end
      end
    end
  end

  def test_run_with_options_serves_static_describe_without_adjacent_proto_files
    with_serve_fixture(include_manifest: false, include_proto_dir: false) do |fixture|
      assert_empty Dir.glob(File.join(fixture[:holon_dir], "**", "*.proto")).sort

      stdin, stdout, stderr, wait_thr = Open3.popen3(
        RbConfig.ruby,
        fixture[:script_path],
        "serve",
        "--listen",
        "tcp://127.0.0.1:0",
        chdir: fixture[:holon_dir]
      )

      begin
        uri = read_advertised_uri(stdout, stderr)
        channel = Holons.connect(uri)
        begin
          response = describe(channel)
          assert_equal "Serve", response.manifest.identity.given_name
          assert_equal ["echo.v1.Echo"], response.services.map(&:name)
        ensure
          Holons.disconnect(channel)
        end
      ensure
        terminate_process(wait_thr.pid)
        stdin.close unless stdin.closed?
        stdout.close unless stdout.closed?
        stderr.close unless stderr.closed?
      end
    end
  end

  private

  def describe(channel)
    stub = Holons::V1::HolonMeta::Stub.new(
      "unused",
      :this_channel_is_insecure,
      channel_override: channel,
      timeout: 5
    )

    stub.describe(Holons::V1::DescribeRequest.new)
  end

  def with_serve_fixture(register_static: true, include_manifest: true, include_proto_dir: true)
    Dir.mktmpdir("ruby-holons-serve-") do |workspace|
      holon_dir = File.join(workspace, "holons", "serve-helper")
      FileUtils.mkdir_p(holon_dir)
      FileUtils.cp_r(proto_fixture_dir, File.join(holon_dir, "protos")) if include_proto_dir

      script_path = File.join(holon_dir, "serve_helper.rb")
      wrapper_path = File.join(holon_dir, "serve-helper")
      File.write(script_path, helper_script(register_static: register_static))
      File.write(wrapper_path, helper_wrapper(script_path))
      File.chmod(0o755, script_path)
      File.chmod(0o755, wrapper_path)

      File.write(File.join(holon_dir, "holon.proto"), holon_manifest(wrapper_path)) if include_manifest

      yield(
        workspace: workspace,
        holon_dir: holon_dir,
        script_path: script_path,
        wrapper_path: wrapper_path,
        slug: "serve-helper"
      )
    end
  end

  def helper_script(register_static:)
    sdk_lib = File.expand_path("../lib", __dir__)
    static_registration = register_static ? helper_static_registration : ""

    <<~RUBY
      #!/usr/bin/env ruby
      # frozen_string_literal: true

      $LOAD_PATH.unshift(#{sdk_lib.inspect})
      require "holons"
      #{static_registration}

      begin
        args = ARGV.dup
        args.shift if args.first == "serve"

        parsed = Holons::Serve.parse_options(args)
        Holons::Serve.run_with_options(parsed.listen_uri, ->(_server) {}, parsed.reflect)
      rescue StandardError => e
        warn(e.message)
        exit 1
      end
    RUBY
  end

  def helper_static_registration
    <<~RUBY
      require "holons/v1/manifest_pb"
      require "holons/v1/describe_pb"
      require "holons/v1/describe_services_pb"

      Holons::Describe.use_static_response(
        Holons::V1::DescribeResponse.new(
          manifest: Holons::V1::HolonManifest.new(
            identity: Holons::V1::HolonManifest::Identity.new(
              schema: "holon/v1",
              uuid: "serve-helper-uuid",
              given_name: "Serve",
              family_name: "Helper",
              motto: "Reply precisely.",
              composer: "serve-test",
              status: "draft",
              born: "2026-03-23"
            ),
            lang: "ruby",
            kind: "service",
            build: Holons::V1::HolonManifest::Build.new(
              runner: "ruby",
              main: "serve_helper.rb"
            ),
            artifacts: Holons::V1::HolonManifest::Artifacts.new(
              binary: "serve-helper"
            )
          ),
          services: [
            Holons::V1::ServiceDoc.new(
              name: "echo.v1.Echo",
              description: "Echo helper service.",
              methods: [
                Holons::V1::MethodDoc.new(
                  name: "Ping",
                  description: "Echoes the inbound message.",
                  input_type: "echo.v1.PingRequest",
                  output_type: "echo.v1.PingResponse"
                )
              ]
            )
          ]
        )
      )
    RUBY
  end

  def helper_wrapper(script_path)
    bundle_path = File.expand_path("../vendor/bundle", __dir__)

    <<~SH
      #!/bin/sh
      set -eu

      cd #{File.dirname(script_path).inspect}
      export BUNDLE_GEMFILE=#{File.expand_path("../Gemfile", __dir__).inspect}
      export BUNDLE_PATH=#{bundle_path.inspect}

      exec arch -x86_64 bundle exec ruby #{script_path.inspect} "$@"
    SH
  end

  def holon_manifest(binary_path)
    <<~PROTO
      syntax = "proto3";

      package holons.test.v1;

      option (holons.v1.manifest) = {
        identity: {
          uuid: "serve-helper-uuid"
          given_name: "Serve"
          family_name: "Helper"
          motto: "Reply precisely."
          composer: "serve-test"
          status: "draft"
          born: "2026-03-17"
        }
        kind: "service"
        build: {
          runner: "ruby"
          main: "serve_helper.rb"
        }
        artifacts: {
          binary: "#{binary_path}"
        }
      };
    PROTO
  end

  def proto_fixture_dir
    File.expand_path("../../go-holons/pkg/describe/testdata/echoholon/protos", __dir__)
  end

  def with_holon_root(root)
    original_dir = Dir.pwd
    original_oppath = ENV["OPPATH"]
    original_opbin = ENV["OPBIN"]

    Dir.chdir(root)
    ENV["OPPATH"] = File.join(root, ".op-home")
    ENV["OPBIN"] = File.join(root, ".op-bin")

    yield
  ensure
    Dir.chdir(original_dir)
    ENV["OPPATH"] = original_oppath
    ENV["OPBIN"] = original_opbin
  end

  def read_advertised_uri(stdout, stderr)
    uri = nil

    Timeout.timeout(10) do
      uri = stdout.gets&.strip
    end
    return uri unless uri.nil? || uri.empty?

    error_output = stderr.read
    uri = error_output[/\b(?:tcp|unix):\/\/\S+/, 0]
    raise "serve helper did not advertise an address: #{error_output}" if uri.nil? || uri.empty?

    uri
  end

  def terminate_process(pid)
    Process.kill("TERM", pid)
    Timeout.timeout(2) { Process.wait(pid) }
  rescue Errno::ESRCH, Errno::ECHILD, Timeout::Error
    begin
      Process.kill("KILL", pid)
    rescue Errno::ESRCH
      nil
    end
  end
end
