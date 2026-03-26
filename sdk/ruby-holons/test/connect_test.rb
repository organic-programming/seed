# frozen_string_literal: true

require "json"
require "fileutils"
require "minitest/autorun"
require "open3"
require "shellwords"
require "socket"
require "tmpdir"
require "timeout"
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

  def test_connect_dials_direct_tcp_target
    with_echo_server("--listen", "tcp://127.0.0.1:0") do |uri|
      channel = Holons.connect(uri)
      begin
        assert_instance_of GRPC::Core::Channel, channel

        out = invoke_ping(channel, "direct-ruby")
        assert_equal "direct-ruby", out["message"]
        assert_equal "ruby-holons", out["sdk"]
      ensure
        Holons.disconnect(channel)
      end
    end
  end

  def test_connect_resolves_slug_and_stops_ephemeral_process
    Dir.mktmpdir("ruby-holons-connect-") do |root|
      fixture = create_holon_fixture(root, "Connect", "Ephemeral")

      with_holon_root(root) do
        channel = Holons.connect(fixture[:slug])
        pid = wait_for_pid_file(fixture[:pid_file])

        begin
          out = invoke_ping(channel, "ephemeral-ruby")
          assert_equal "ephemeral-ruby", out["message"]
        ensure
          Holons.disconnect(channel)
        end

        wait_for_pid_exit(pid)
        refute File.exist?(fixture[:port_file]), "unexpected port file #{fixture[:port_file]}"
      end
    end
  end

  def test_connect_reuses_existing_port_file
    Dir.mktmpdir("ruby-holons-connect-") do |root|
      fixture = create_holon_fixture(root, "Connect", "Reuse")

      with_holon_root(root) do
        stdin, stdout, stderr, wait_thr = Open3.popen3(
          fixture[:binary_path],
          "serve",
          "--listen",
          "tcp://127.0.0.1:0"
        )

        begin
          uri = read_advertised_uri(stdout, stderr)
          pid = wait_for_pid_file(fixture[:pid_file])

          FileUtils.mkdir_p(File.dirname(fixture[:port_file]))
          File.write(fixture[:port_file], "#{uri}\n")

          channel = Holons.connect(fixture[:slug])
          begin
            out = invoke_ping(channel, "reuse-ruby")
            assert_equal "reuse-ruby", out["message"]
            assert pid_alive?(pid), "expected reused process #{pid} to stay alive"
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
  end

  def test_connect_writes_unix_port_file_in_persistent_mode
    Dir.mktmpdir("ruby-holons-connect-") do |root|
      fixture = create_holon_fixture(root, "Connect", "Unix")

      with_holon_root(root) do
        channel = Holons.connect(
          fixture[:slug],
          transport: "unix",
          timeout: 5,
          start: true
        )
        pid = wait_for_pid_file(fixture[:pid_file])

        begin
          out = invoke_ping(channel, "unix-ruby")
          assert_equal "unix-ruby", out["message"]
        ensure
          Holons.disconnect(channel)
        end

        target = File.read(fixture[:port_file]).strip
        assert_match(/^unix:\/\/\/tmp\/holons-/, target)
        assert pid_alive?(pid), "expected persistent process #{pid} to stay alive"

        reused = Holons.connect(fixture[:slug])
        begin
          out = invoke_ping(reused, "unix-reuse-ruby")
          assert_equal "unix-reuse-ruby", out["message"]
        ensure
          Holons.disconnect(reused)
          terminate_process(pid)
        end
      end
    end
  end

  def test_connect_removes_stale_port_file_and_starts_fresh
    Dir.mktmpdir("ruby-holons-connect-") do |root|
      fixture = create_holon_fixture(root, "Connect", "Stale")

      with_holon_root(root) do
        stale_server = TCPServer.new("127.0.0.1", 0)
        stale_port = stale_server.local_address.ip_port
        stale_server.close

        FileUtils.mkdir_p(File.dirname(fixture[:port_file]))
        File.write(fixture[:port_file], "tcp://127.0.0.1:#{stale_port}\n")

        channel = Holons.connect(fixture[:slug])
        pid = wait_for_pid_file(fixture[:pid_file])

        begin
          out = invoke_ping(channel, "stale-ruby")
          assert_equal "stale-ruby", out["message"]
          refute File.exist?(fixture[:port_file]), "stale port file should be removed"
        ensure
          Holons.disconnect(channel)
        end

        wait_for_pid_exit(pid)
      end
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

  def with_echo_server(*args)
    env = {
      "GOCACHE" => ENV.fetch("GOCACHE", "/tmp/go-cache-ruby-holons-tests")
    }
    stdin, stdout, stderr, wait_thr = Open3.popen3(
      env,
      echo_server_script,
      *args,
      chdir: sdk_dir
    )

    begin
      uri = read_advertised_uri(stdout, stderr)
      yield(uri)
    ensure
      terminate_process(wait_thr.pid)
      stdin.close unless stdin.closed?
      stdout.close unless stdout.closed?
      stderr.close unless stderr.closed?
    end
  end

  def read_advertised_uri(stdout, stderr)
    uri = nil

    Timeout.timeout(20) do
      uri = stdout.gets&.strip
    end

    return uri unless uri.nil? || uri.empty?

    error_output = stderr.read
    if bind_denied?(error_output)
      skip "echo-server requires local bind permissions in this environment"
    end

    raise "echo-server did not output URL: #{error_output}"
  end

  def create_holon_fixture(root, given_name, family_name)
    slug = "#{given_name}-#{family_name}".downcase
    holon_dir = File.join(root, "holons", slug)
    binary_dir = File.join(holon_dir, ".op", "build", "bin")
    pid_file = File.join(root, "#{slug}.pid")
    binary_path = File.join(binary_dir, "echo-wrapper")
    port_file = File.join(root, ".op", "run", "#{slug}.port")

    FileUtils.mkdir_p(binary_dir)
    File.write(binary_path, wrapper_script(pid_file))
    File.chmod(0o755, binary_path)

    File.write(File.join(holon_dir, "holon.proto"), <<~PROTO)
      syntax = "proto3";

      package holons.test.v1;

      option (holons.v1.manifest) = {
        identity: {
          uuid: "#{slug}-uuid"
          given_name: "#{given_name}"
          family_name: "#{family_name}"
          composer: "connect-test"
        }
        kind: "service"
        build: {
          runner: "ruby"
          main: "bin/echo-server"
        }
        artifacts: {
          binary: "echo-wrapper"
        }
      };
    PROTO

    {
      slug: slug,
      pid_file: pid_file,
      binary_path: binary_path,
      port_file: port_file
    }
  end

  def wrapper_script(pid_file)
    [
      "#!/bin/sh",
      "printf '%s\\n' \"$$\" > #{Shellwords.escape(pid_file)}",
      "exec #{Shellwords.escape(echo_server_script)} \"$@\"",
      ""
    ].join("\n")
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

  def wait_for_pid_file(path, timeout: 5)
    deadline = Time.now + timeout
    while Time.now < deadline
      begin
        raw = File.read(path).strip
        pid = Integer(raw, 10)
        return pid if pid.positive?
      rescue Errno::ENOENT, ArgumentError
        nil
      end
      sleep(0.025)
    end

    flunk "timed out waiting for pid file #{path}"
  end

  def wait_for_pid_exit(pid, timeout: 2)
    deadline = Time.now + timeout
    while Time.now < deadline
      return unless pid_alive?(pid)

      sleep(0.025)
    end

    flunk "process #{pid} did not exit"
  end

  def pid_alive?(pid)
    Process.kill(0, pid)
    true
  rescue Errno::EPERM
    true
  rescue Errno::ESRCH
    false
  end

  def terminate_process(pid)
    return unless pid_alive?(pid)

    Process.kill("TERM", pid)
    deadline = Time.now + 2
    while Time.now < deadline
      return unless pid_alive?(pid)

      sleep(0.025)
    end

    Process.kill("KILL", pid) if pid_alive?(pid)
  rescue Errno::ESRCH
    nil
  end

  def bind_denied?(text)
    normalized = text.to_s.downcase
    normalized.include?("bind: operation not permitted") ||
      normalized.include?("operation not permitted - bind")
  end
end
