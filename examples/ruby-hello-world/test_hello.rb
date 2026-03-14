# frozen_string_literal: true

require "minitest/autorun"
require "open3"
require "rbconfig"
require "timeout"
require_relative "hello"

class HelloServiceTest < Minitest::Test
  def test_greet_with_name
    assert_equal "Hello, Alice!", HelloService.greet("Alice")
  end

  def test_greet_default
    assert_equal "Hello, World!", HelloService.greet("")
  end

  def test_greet_nil
    assert_equal "Hello, World!", HelloService.greet(nil)
  end

  def test_serve_exposes_greet_over_grpc
    skip "grpc gem is unavailable in this Ruby environment" unless grpc_runtime_available?

    stdin, stdout, stderr, wait_thr = Open3.popen3(
      RbConfig.ruby,
      "hello.rb",
      "serve",
      "--listen",
      "tcp://127.0.0.1:0",
      chdir: example_dir
    )

    begin
      uri = read_advertised_uri(stdout, stderr)
      channel = Holons.connect(uri)
      begin
        stub = Hello::V1::HelloService::Stub.new(
          "unused",
          :this_channel_is_insecure,
          channel_override: channel,
          timeout: 5
        )

        response = stub.greet(Hello::V1::GreetRequest.new(name: "Alice"))
        assert_equal "Hello, Alice!", response.message
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

  private

  def grpc_runtime_available?
    require_relative "../../sdk/ruby-holons/lib/holons"
    return false unless Holons.grpc_available?

    require_relative "hello_pb"
    require_relative "hello_services_pb"
    true
  rescue LoadError
    false
  end

  def example_dir
    File.expand_path(__dir__)
  end

  def read_advertised_uri(stdout, stderr)
    uri = nil

    Timeout.timeout(10) do
      uri = stdout.gets&.strip
    end
    return uri unless uri.nil? || uri.empty?

    error_output = stderr.read
    uri = error_output[/\b(?:tcp|unix):\/\/\S+/, 0]
    raise "hello.rb did not advertise an address: #{error_output}" if uri.nil? || uri.empty?

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
