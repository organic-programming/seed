# frozen_string_literal: true

require "json"
require "fileutils"
require "minitest/autorun"
require "open3"
require "shellwords"
require "socket"
require "tmpdir"
require "timeout"
require "uri"
require_relative "../lib/holons"

class ConnectTest < Minitest::Test
  def setup
    skip "grpc gem is unavailable in this Ruby environment" unless Holons.grpc_available?

    begin
      server = TCPServer.new("127.0.0.1", 0)
      server.close
    rescue Errno::EPERM => e
      skip "local socket bind is denied in this environment: #{e.message}"
    end
  end

  def test_connect_unresolvable_target
    Dir.mktmpdir("ruby-holons-connect-") do |root|
      result = Holons.connect(Holons::LOCAL, "missing", root, Holons::CWD, 1000)
      refute_nil result.error
      assert_nil result.channel
    end
  end

  def test_connect_returns_connect_result
    with_echo_package_fixture do |fixture|
      result = Holons.connect(Holons::LOCAL, fixture[:slug], fixture[:root], Holons::CWD, 5000)
      assert_instance_of Holons::ConnectResult, result
      assert_nil result.error
      refute_nil result.channel

      begin
        out = invoke_ping(result.channel, "connect-result")
        assert_equal "connect-result", out["message"]
      ensure
        Holons.disconnect(result)
      end
    end
  end

  def test_connect_populates_origin
    with_echo_package_fixture do |fixture|
      result = Holons.connect(Holons::LOCAL, fixture[:slug], fixture[:root], Holons::CWD, 5000)
      assert_nil result.error
      refute_nil result.origin
      refute_nil result.origin.info
      assert_equal fixture[:slug], result.origin.info.slug
      assert_equal fixture[:package_url], result.origin.url
    ensure
      Holons.disconnect(result) if defined?(result)
    end
  end

  def test_disconnect_accepts_connect_result
    with_echo_package_fixture do |fixture|
      result = Holons.connect(Holons::LOCAL, fixture[:slug], fixture[:root], Holons::CWD, 5000)
      assert_nil result.error
      assert_nil Holons.disconnect(result)
    end
  end

  private

  def sdk_dir
    File.expand_path("..", __dir__)
  end

  def echo_server_script
    File.join(sdk_dir, "bin", "echo-server")
  end

  def invoke_ping(channel, message, timeout: 5)
    stub = GRPC::ClientStub.new(
      "unused",
      :this_channel_is_insecure,
      channel_override: channel,
      timeout: timeout
    )

    stub.request_response(
      "/echo.v1.Echo/Ping",
      { "message" => message },
      ->(value) { JSON.generate(value) },
      ->(payload) { JSON.parse(payload) },
      deadline: Time.now + timeout
    )
  end

  def with_echo_package_fixture
    Dir.mktmpdir("ruby-holons-connect-") do |root|
      slug = "connect-helper"
      package_dir = File.join(root, "#{slug}.holon")
      arch_dir = File.join(package_dir, "bin", Holons::DiscoverySupport.package_arch_dir)
      binary_path = File.join(arch_dir, slug)

      FileUtils.mkdir_p(arch_dir)
      File.write(binary_path, wrapper_script)
      File.chmod(0o755, binary_path)

      File.write(File.join(package_dir, ".holon.json"), <<~JSON)
        {
          "schema": "holon-package/v1",
          "slug": #{slug.inspect},
          "uuid": "connect-helper-uuid",
          "identity": {
            "given_name": "Connect",
            "family_name": "Helper"
          },
          "lang": "ruby",
          "runner": "ruby",
          "status": "draft",
          "kind": "native",
          "transport": "stdio",
          "entrypoint": #{slug.inspect},
          "architectures": [#{Holons::DiscoverySupport.package_arch_dir.inspect}],
          "has_dist": false,
          "has_source": false
        }
      JSON

      yield(
        root: root,
        slug: slug,
        package_url: file_url(package_dir)
      )
    end
  end

  def wrapper_script
    [
      "#!/bin/sh",
      "exec #{Shellwords.escape(echo_server_script)} \"$@\"",
      ""
    ].join("\n")
  end

  def file_url(path)
    URI::Generic.build(scheme: "file", path: File.expand_path(path).tr(File::SEPARATOR, "/")).to_s
  end
end
