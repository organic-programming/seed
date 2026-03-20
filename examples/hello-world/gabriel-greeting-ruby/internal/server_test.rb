# frozen_string_literal: true

require "minitest/autorun"
require_relative "../support"
require "grpc"
require "gen/holons/v1/manifest_pb"
require "gen/holons/v1/describe_pb"
require "gen/holons/v1/describe_services_pb"
require "v1/greeting_services_pb"
require_relative "server"

class GreetingServerTest < Minitest::Test
  def setup
    @server = GRPC::RpcServer.new
    GabrielGreetingRuby::Internal::Server.register_services(@server)
    @port = @server.add_http2_port("127.0.0.1:0", :this_port_is_insecure)
    @thread = Thread.new { @server.run_till_terminated }
    raise "gRPC server failed to start" unless @server.wait_till_running(5)

    target = "127.0.0.1:#{@port}"
    @greeting_stub = Greeting::V1::GreetingService::Stub.new(target, :this_channel_is_insecure, timeout: 5)
    @meta_stub = Holons::V1::HolonMeta::Stub.new(target, :this_channel_is_insecure, timeout: 5)
  end

  def teardown
    return if @server.nil?

    @server.stop if @server.respond_to?(:running?) && @server.running?
    @thread&.join(1)
  end

  def test_list_languages_returns_all_languages
    response = @greeting_stub.list_languages(Greeting::V1::ListLanguagesRequest.new)
    assert_equal 56, response.languages.length
  end

  def test_say_hello_uses_requested_language
    response = @greeting_stub.say_hello(
      Greeting::V1::SayHelloRequest.new(name: "Alice", lang_code: "fr")
    )

    assert_equal "Bonjour Alice", response.greeting
    assert_equal "French", response.language
    assert_equal "fr", response.lang_code
  end

  def test_say_hello_uses_localized_default_name
    response = @greeting_stub.say_hello(
      Greeting::V1::SayHelloRequest.new(lang_code: "fr")
    )

    assert_equal "Bonjour Marie", response.greeting
    assert_equal "fr", response.lang_code
  end

  def test_describe_uses_proto_manifest
    response = @meta_stub.describe(Holons::V1::DescribeRequest.new)

    assert_equal "Gabriel", response.manifest.identity.given_name
    assert_equal "Greeting-Ruby", response.manifest.identity.family_name
    assert_equal "Greets users in 56 languages — a Ruby daemon example.", response.manifest.identity.motto
    assert_equal ["greeting.v1.GreetingService"], response.services.map(&:name)
  end

  def test_normalize_listen_uri_expands_empty_tcp_host
    assert_equal "tcp://0.0.0.0:9090", GabrielGreetingRuby::Internal::Server.normalize_listen_uri("tcp://:9090")
    assert_equal "tcp://127.0.0.1:9090", GabrielGreetingRuby::Internal::Server.normalize_listen_uri("tcp://127.0.0.1:9090")
  end
end
