# frozen_string_literal: true

require "minitest/autorun"
require "net/http"
require "timeout"
require "tmpdir"
require_relative "../lib/holons"
require_relative "../lib/holons/observability"

class ObservabilityTest < Minitest::Test
  def test_parse_op_obs_rejects_v2_tokens
    all = Set[:logs, :metrics, :events, :prom]
    assert_equal all, Holons::Observability.parse_op_obs("all")
    assert_raises(Holons::Observability::InvalidTokenError) do
      Holons::Observability.parse_op_obs("all,otel")
    end
    assert_raises(Holons::Observability::InvalidTokenError) do
      Holons::Observability.parse_op_obs("all,sessions")
    end
    assert_raises(Holons::Observability::InvalidTokenError) do
      Holons::Observability.parse_op_obs("unknown")
    end
  end

  def test_check_env_rejects_v2_tokens_and_op_sessions
    assert_raises(Holons::Observability::InvalidTokenError) do
      Holons::Observability.check_env("OP_OBS" => "logs,otel")
    end
    assert_raises(Holons::Observability::InvalidTokenError) do
      Holons::Observability.check_env("OP_OBS" => "logs,sessions")
    end
    assert_raises(Holons::Observability::InvalidTokenError) do
      Holons::Observability.check_env("OP_SESSIONS" => "metrics")
    end
  end

  def test_run_dir_derives_from_registry_root
    Dir.mktmpdir("ruby-holons-obs-") do |root|
      obs = Holons::Observability.configure(
        Holons::Observability::Config.new(
          slug: "gabriel-greeting-ruby",
          instance_uid: "uid-1",
          run_dir: root
        ),
        env: { "OP_OBS" => "logs" }
      )

      assert_equal File.join(root, "gabriel-greeting-ruby", "uid-1"), obs.cfg.run_dir
    ensure
      Holons::Observability.reset
    end
  end

  def test_service_replays_logs_metrics_events_and_disk_files
    skip "grpc gem is unavailable in this Ruby environment" unless Holons.grpc_available?

    Dir.mktmpdir("ruby-holons-obs-") do |root|
      obs = Holons::Observability.configure(
        Holons::Observability::Config.new(
          slug: "gabriel-greeting-ruby",
          instance_uid: "uid-2",
          run_dir: root
        ),
        env: { "OP_OBS" => "logs,metrics,events" }
      )
      Holons::Observability.enable_disk_writers(obs.cfg.run_dir)
      obs.logger("test").info("service-log", "component" => "ruby")
      obs.counter("ruby_requests_total").inc
      obs.emit(Holons::Observability::EVENT_TYPES[:instance_ready], "listener" => "tcp://127.0.0.1:1")
      Holons::Observability.write_meta_json(
        obs.cfg.run_dir,
        Holons::Observability::MetaJson.new(
          slug: obs.cfg.slug,
          uid: obs.cfg.instance_uid,
          pid: 123,
          started_at: Time.now,
          transport: "tcp",
          address: "tcp://127.0.0.1:1",
          log_path: File.join(obs.cfg.run_dir, "stdout.log")
        )
      )

      assert_includes File.read(File.join(obs.cfg.run_dir, "stdout.log")), '"message":"service-log"'
      assert_includes File.read(File.join(obs.cfg.run_dir, "events.jsonl")), '"type":"INSTANCE_READY"'
      assert_equal "uid-2", JSON.parse(File.read(File.join(obs.cfg.run_dir, "meta.json"))).fetch("uid")

      with_observability_server(obs) do |target|
        stub = Holons::V1::HolonObservability::Stub.new(target, :this_channel_is_insecure, timeout: 5)
        logs = stub.logs(Holons::V1::LogsRequest.new(min_level: Holons::V1::LogLevel::INFO)).to_a
        assert logs.any? { |entry| entry.message == "service-log" }

        metrics = stub.metrics(Holons::V1::MetricsRequest.new)
        assert metrics.samples.any? { |sample| sample.name == "ruby_requests_total" }

        events = stub.events(Holons::V1::EventsRequest.new).to_a
        assert events.any? { |event| event.type == :INSTANCE_READY }
      end
    ensure
      Holons::Observability.reset
    end
  end

  def test_prometheus_text_and_http_server
    Dir.mktmpdir("ruby-holons-prom-") do
      obs = Holons::Observability.configure(
        Holons::Observability::Config.new(slug: "prom-ruby", instance_uid: "prom-uid"),
        env: { "OP_OBS" => "all" }
      )
      obs.counter("cascade_ticks_total", "Ticks received by this cascade node.", "responder_uid" => "prom-uid").inc

      text = Holons::Observability.to_prometheus_text(obs)
      assert_includes text, "# HELP cascade_ticks_total Ticks received by this cascade node."
      assert_includes text, 'cascade_ticks_total{instance_uid="prom-uid",responder_uid="prom-uid",slug="prom-ruby"} 1'

      server = Holons::Observability::PromServer.new("127.0.0.1:0")
      addr = server.start
      body = Net::HTTP.get(URI(addr))
      assert_includes body, "cascade_ticks_total"
    ensure
      server&.close
      Holons::Observability.reset
    end
  end

  def test_member_relay_forwards_logs_and_events_with_child_chain
    skip "grpc gem is unavailable in this Ruby environment" unless Holons.grpc_available?

    child = Holons::Observability.configure(
      Holons::Observability::Config.new(slug: "child-ruby", instance_uid: "child-uid"),
      env: { "OP_OBS" => "logs,events" }
    )

    with_observability_server(child) do |target|
      parent = Holons::Observability.configure(
        Holons::Observability::Config.new(slug: "parent-ruby", instance_uid: "parent-uid"),
        env: { "OP_OBS" => "logs,events" }
      )
      stub = Holons::V1::HolonObservability::Stub.new(target, :this_channel_is_insecure, timeout: 5)
      relay = Holons::Observability::MemberRelay.new(
        child_slug: "child-ruby",
        child_uid: "child-uid",
        stub: stub,
        observability: parent,
        retry_delay: 0.1
      )
      relay.start
      child.logger("tick").info("relay-log", "component" => "ruby")
      child.emit(Holons::Observability::EVENT_TYPES[:instance_ready], "listener" => "tcp://127.0.0.1:1")

      Timeout.timeout(3) do
        loop do
          forwarded_log = parent.log_ring.drain.find { |entry| entry[:message] == "relay-log" }
          forwarded_event = parent.event_bus.drain.find { |event| event[:type] == Holons::Observability::EVENT_TYPES[:instance_ready] }
          if forwarded_log && forwarded_event
            assert_equal [{ slug: "child-ruby", instance_uid: "child-uid" }], forwarded_log[:chain]
            assert_equal [{ slug: "child-ruby", instance_uid: "child-uid" }], forwarded_event[:chain]
            break
          end
          sleep 0.05
        end
      end
    ensure
      relay&.stop
    end
  ensure
    Holons::Observability.reset
  end

  private

  def with_observability_server(obs)
    Holons::Observability.require_grpc_observability_support!
    server = GRPC::RpcServer.new
    port = server.add_http2_port("127.0.0.1:0", :this_port_is_insecure)
    server.handle(Holons::Observability.observability_service_class.new(obs))
    thread = Thread.new { server.run_till_terminated }
    server.wait_till_running(5)
    yield "127.0.0.1:#{port}"
  ensure
    server&.stop
    thread&.join(1)
  end
end
