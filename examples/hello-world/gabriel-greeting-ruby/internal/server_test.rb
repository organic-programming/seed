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
      Greeting::V1::SayHelloRequest.new(name: "Bob", lang_code: "fr")
    )

    assert_equal "Bonjour Bob", response.greeting
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

  def test_say_hello_emits_observability_signals
    old_op_obs = ENV["OP_OBS"]
    ENV["OP_OBS"] = "logs,metrics"
    Holons::Observability.reset
    obs = Holons::Observability.configure(
      Holons::Observability::Config.new(slug: "gabriel-greeting-ruby")
    )

    response = @greeting_stub.say_hello(
      Greeting::V1::SayHelloRequest.new(name: " Bob ", lang_code: "en")
    )

    assert_equal "Hello Bob", response.greeting
    counter = obs.registry.counters.find { |sample| sample.name == "greeting_emitted_total" }
    refute_nil counter
    assert_equal(
      {
        "lang_code" => "en",
        "language" => "English",
        "transport" => "unknown"
      },
      counter.labels
    )
    assert_equal 1, counter.value

    entry = obs.log_ring.drain.find { |log_entry| Holons::Observability.body_string(log_entry.record) == "Greeted Bob in English (en)" }
    refute_nil entry
    fields = Holons::Observability.attributes_hash(entry.record.attributes)
    assert_equal(
      %w[duration_ns greeting lang_code language name transport],
      fields.keys.sort
    )
    assert_equal "en", fields["lang_code"]
    assert_equal "English", fields["language"]
    assert_equal "Bob", fields["name"]
    assert_equal "Hello Bob", fields["greeting"]
    assert_equal "unknown", fields["transport"]
    assert_instance_of Integer, fields["duration_ns"]
    assert_operator fields["duration_ns"], :>=, 0
    assert_equal(
      "gabriel-greeting-ruby",
      Holons::Observability.attribute_string(entry.record.attributes, Holons::Observability::ATTR_HOLONS_SLUG)
    )
    assert_equal(
      "gabriel-greeting-ruby",
      Holons::Observability.attribute_string(entry.record.attributes, Holons::Observability::ATTR_SERVICE_NAME)
    )
    refute_empty Holons::Observability.attribute_string(entry.record.attributes, Holons::Observability::ATTR_HOLONS_SESSION_ID)
    duration_attr = Holons::Observability.attribute_value(entry.record.attributes, "duration_ns")
    assert_equal :int_value, duration_attr.value
  ensure
    Holons::Observability.reset
    if old_op_obs.nil?
      ENV.delete("OP_OBS")
    else
      ENV["OP_OBS"] = old_op_obs
    end
  end

  def test_say_hello_emits_stdio_transport_under_stdio_serve
    Holons::Observability.require_grpc_observability_support!
    env = ENV.to_h.merge(
      "OP_OBS" => "logs",
      "OP_INSTANCE_UID" => "ruby-stdio-fixture"
    )
    proxy = Holons::DiscoverySupport::StdioDialProxy.new(
      RbConfig.ruby,
      args: ["cmd/main.rb"],
      chdir: GabrielGreetingRuby::ROOT,
      env: env
    )
    proxy.start
    channel = Holons::DiscoverySupport.dial_ready(proxy.target, 5000)

    greeting_stub = Greeting::V1::GreetingService::Stub.new(
      "unused",
      :this_channel_is_insecure,
      channel_override: channel,
      timeout: 5
    )
    observability_stub = Holons::V1::HolonObservability::Stub.new(
      "unused",
      :this_channel_is_insecure,
      channel_override: channel,
      timeout: 5
    )

    response = greeting_stub.say_hello(
      Greeting::V1::SayHelloRequest.new(name: "Ana", lang_code: "es")
    )
    assert_equal "Hola Ana", response.greeting

    records = observability_stub.logs(Holons::V1::LogsRequest.new).to_a
    record = records.find { |candidate| Holons::Observability.body_string(candidate) == "Greeted Ana in Spanish (es)" }
    refute_nil record
    fields = Holons::Observability.attributes_hash(record.attributes)
    assert_equal "stdio", fields["transport"]
    assert_instance_of Integer, fields["duration_ns"]
    assert_equal :int_value, Holons::Observability.attribute_value(record.attributes, "duration_ns").value
  ensure
    Holons::DiscoverySupport.close_channel(channel) unless channel.nil?
    proxy&.close
  end

  def test_describe_uses_static_manifest
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
