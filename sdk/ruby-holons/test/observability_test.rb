# frozen_string_literal: true

require "minitest/autorun"
require "net/http"
require "timeout"
require "tmpdir"
require_relative "../lib/holons"
require_relative "../lib/holons/observability"

class ObservabilityTest < Minitest::Test
  def test_parse_op_obs_rejects_unknown_tokens
    all = Set[:logs, :metrics, :events, :prom]
    assert_equal all, Holons::Observability.parse_op_obs("all")
    assert_raises(Holons::Observability::InvalidTokenError) do
      Holons::Observability.parse_op_obs("unknown")
    end
  end

  def test_check_env_rejects_unknown_tokens
    assert_raises(Holons::Observability::InvalidTokenError) do
      Holons::Observability.check_env("OP_OBS" => "logs,unknown")
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

  def test_logger_emits_log_record_with_typed_any_value_attributes
    obs = Holons::Observability.configure(
      Holons::Observability::Config.new(slug: "typed-ruby", instance_uid: "typed-uid"),
      env: { "OP_OBS" => "logs" }
    )
    object = Object.new

    obs.logger("typed").info(
      "typed-log",
      "truth" => true,
      "falsehood" => false,
      "count" => 7,
      "ratio" => 2.5,
      "language" => "Ruby",
      "code" => :rb,
      "object" => object
    )

    record = obs.log_ring.drain.first.record
    assert_equal :string_value, record.body.value
    assert_equal "typed-log", record.body.string_value
    assert_equal 9, Holons::Observability.severity_number(record.severity_number)
    assert_equal "INFO", record.severity_text
    assert record.attributes.all? { |attribute| attribute.key.is_a?(String) }
    assert_equal "typed-ruby", attr_string(record, Holons::Observability::ATTR_HOLONS_SLUG)
    assert_equal "typed-ruby", attr_string(record, Holons::Observability::ATTR_SERVICE_NAME)
    assert_equal "typed-uid", attr_string(record, Holons::Observability::ATTR_HOLONS_INSTANCE_UID)
    assert_equal "typed-uid", attr_string(record, Holons::Observability::ATTR_SERVICE_INSTANCE_ID)
    assert_equal :bool_value, attr_value(record, "truth").value
    assert_equal true, attr_value(record, "truth").bool_value
    assert_equal :bool_value, attr_value(record, "falsehood").value
    assert_equal false, attr_value(record, "falsehood").bool_value
    assert_equal :int_value, attr_value(record, "count").value
    assert_equal 7, attr_value(record, "count").int_value
    assert_equal :double_value, attr_value(record, "ratio").value
    assert_in_delta 2.5, attr_value(record, "ratio").double_value
    assert_equal :string_value, attr_value(record, "language").value
    assert_equal "Ruby", attr_value(record, "language").string_value
    assert_equal "rb", attr_value(record, "code").string_value
    assert_equal object.to_s, attr_value(record, "object").string_value
  ensure
    Holons::Observability.reset
  end

  def test_metrics_emit_otlp_metric_oneofs
    obs = Holons::Observability.configure(
      Holons::Observability::Config.new(slug: "metric-ruby", instance_uid: "metric-uid"),
      env: { "OP_OBS" => "metrics" }
    )
    obs.counter("requests_total", "Request count", "path" => "/hello").add(3)
    obs.gauge("temperature", "Temperature", "room" => "lab").set(21.5)
    obs.histogram("latency_seconds", "Latency", { "route" => "say_hello" }, [0.1, 1.0]).observe(0.2)

    metrics = Holons::Observability.to_proto_metrics(obs.registry, obs.cfg.slug, obs.cfg.instance_uid, obs.start_wall)
    counter = metrics.find { |metric| metric.name == "requests_total" }
    gauge = metrics.find { |metric| metric.name == "temperature" }
    histogram = metrics.find { |metric| metric.name == "latency_seconds" }

    assert_equal :sum, counter.data
    assert counter.sum.is_monotonic
    assert_equal :AGGREGATION_TEMPORALITY_CUMULATIVE, counter.sum.aggregation_temporality
    assert_equal :as_int, counter.sum.data_points.first.value
    assert_equal 3, counter.sum.data_points.first.as_int
    assert_equal "metric-ruby", attr_string(counter.sum.data_points.first, Holons::Observability::ATTR_HOLONS_SLUG)
    assert_equal :gauge, gauge.data
    assert_equal :as_double, gauge.gauge.data_points.first.value
    assert_in_delta 21.5, gauge.gauge.data_points.first.as_double
    assert_equal :histogram, histogram.data
    point = histogram.histogram.data_points.first
    assert_equal 1, point.count
    assert_in_delta 0.2, point.sum
    assert_equal [0, 1, 0], point.bucket_counts
    assert_equal [0.1, 1.0], point.explicit_bounds
    assert_in_delta 0.2, point.min
    assert_in_delta 0.2, point.max
  ensure
    Holons::Observability.reset
  end

  def test_events_emit_log_record_with_canonical_event_name
    obs = Holons::Observability.configure(
      Holons::Observability::Config.new(slug: "event-ruby", instance_uid: "event-uid"),
      env: { "OP_OBS" => "events" }
    )

    obs.emit(Holons::Observability::EVENT_INSTANCE_READY, "listener" => "stdio://")

    record = obs.event_bus.drain.first.record
    assert_equal Holons::Observability::EVENT_INSTANCE_READY, record.event_name
    assert_equal Holons::Observability::EVENT_INSTANCE_READY, record.body.string_value
    assert_equal "stdio://", attr_string(record, "listener")
    assert_equal "event-ruby", attr_string(record, Holons::Observability::ATTR_HOLONS_SLUG)
  ensure
    Holons::Observability.reset
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
      obs.emit(Holons::Observability::EVENT_INSTANCE_READY, "listener" => "tcp://127.0.0.1:1")
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
      assert_includes File.read(File.join(obs.cfg.run_dir, "events.jsonl")), '"event_name":"instance.ready"'
      assert_equal "uid-2", JSON.parse(File.read(File.join(obs.cfg.run_dir, "meta.json"))).fetch("uid")

      with_observability_server(obs) do |target|
        stub = Holons::V1::HolonObservability::Stub.new(target, :this_channel_is_insecure, timeout: 5)
        logs = stub.logs(Holons::V1::LogsRequest.new(min_severity_number: :SEVERITY_NUMBER_INFO)).to_a
        assert logs.any? { |entry| Holons::Observability.body_string(entry) == "service-log" }

        metrics = stub.metrics(Holons::V1::MetricsRequest.new).to_a
        assert metrics.any? { |metric| metric.name == "ruby_requests_total" && metric.data == :sum }

        events = stub.events(Holons::V1::EventsRequest.new).to_a
        assert events.any? { |event| event.event_name == Holons::Observability::EVENT_INSTANCE_READY }
      end
    ensure
      Holons::Observability.reset
    end
  end

  def test_logs_follow_replays_ring_on_subscribe
    skip "grpc gem is unavailable in this Ruby environment" unless Holons.grpc_available?

    obs = Holons::Observability.configure(
      Holons::Observability::Config.new(slug: "replay-ruby", instance_uid: "log-uid"),
      env: { "OP_OBS" => "logs" }
    )
    obs.logger("test").info("replay")

    with_observability_server(obs) do |target|
      stub = Holons::V1::HolonObservability::Stub.new(target, :this_channel_is_insecure, timeout: 5)
      q = Queue.new
      reader = Thread.new do
        stub.logs(Holons::V1::LogsRequest.new(min_severity_number: :SEVERITY_NUMBER_INFO, follow: true)).each do |entry|
          q << entry
          break if Holons::Observability.body_string(entry) == "live"
        end
      end
      first = Timeout.timeout(3) { q.pop }
      assert_equal "replay", Holons::Observability.body_string(first)

      obs.logger("test").info("live")
      second = Timeout.timeout(3) { q.pop }
      assert_equal "live", Holons::Observability.body_string(second)
    ensure
      reader&.kill
      Holons::Observability.reset
    end
  end

  def test_events_follow_replays_ring_on_subscribe
    skip "grpc gem is unavailable in this Ruby environment" unless Holons.grpc_available?

    obs = Holons::Observability.configure(
      Holons::Observability::Config.new(slug: "replay-ruby", instance_uid: "event-uid"),
      env: { "OP_OBS" => "events" }
    )
    obs.emit(Holons::Observability::EVENT_INSTANCE_READY, "phase" => "replay")

    with_observability_server(obs) do |target|
      stub = Holons::V1::HolonObservability::Stub.new(target, :this_channel_is_insecure, timeout: 5)
      q = Queue.new
      reader = Thread.new do
        stub.events(Holons::V1::EventsRequest.new(follow: true)).each do |event|
          q << event
          break if Holons::Observability.attributes_hash(event.attributes)["phase"] == "live"
        end
      end
      first = Timeout.timeout(3) { q.pop }
      assert_equal "replay", Holons::Observability.attributes_hash(first.attributes)["phase"]

      obs.emit(Holons::Observability::EVENT_INSTANCE_READY, "phase" => "live")
      second = Timeout.timeout(3) { q.pop }
      assert_equal "live", Holons::Observability.attributes_hash(second.attributes)["phase"]
    ensure
      reader&.kill
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
      child.emit(Holons::Observability::EVENT_INSTANCE_READY, "listener" => "tcp://127.0.0.1:1")

      Timeout.timeout(3) do
        loop do
          forwarded_log = parent.log_ring.drain.find { |entry| Holons::Observability.body_string(entry.record) == "relay-log" }
          forwarded_event = parent.event_bus.drain.find { |event| event.record.event_name == Holons::Observability::EVENT_INSTANCE_READY }
          if forwarded_log && forwarded_event
            assert_equal ["child-ruby"], forwarded_log.record.chain
            assert_equal ["child-ruby"], forwarded_event.record.chain
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

  def attr_value(carrier, key)
    attributes = carrier.respond_to?(:attributes) ? carrier.attributes : carrier
    Holons::Observability.attribute_value(attributes, key)
  end

  def attr_string(carrier, key)
    Holons::Observability.any_value_string(attr_value(carrier, key))
  end

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
