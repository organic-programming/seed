# frozen_string_literal: true

require "minitest/autorun"
require "tmpdir"
require_relative "../lib/holons"
require_relative "../lib/holons/observability"

class ObservabilityTest < Minitest::Test
  def test_parse_op_obs_drops_v2_tokens
    all = Set[:logs, :metrics, :events, :prom]
    assert_equal all, Holons::Observability.parse_op_obs("all,otel")
    assert_equal all, Holons::Observability.parse_op_obs("all,sessions")
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
