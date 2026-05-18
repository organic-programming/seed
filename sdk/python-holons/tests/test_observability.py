"""Unit tests for holons.observability, the Python port."""

from __future__ import annotations

import os
import threading
import time
from concurrent import futures
from urllib.request import urlopen

import grpc
import pytest

from holons import observability as obs
from holons.v1 import observability_pb2


def setup_function(_):
    obs.reset()
    os.environ.pop("OP_OBS", None)


def test_parse_op_obs_basic():
    cases = [
        ("", set()),
        ("logs", {obs.Family.LOGS}),
        ("logs,metrics", {obs.Family.LOGS, obs.Family.METRICS}),
        ("all", {obs.Family.LOGS, obs.Family.METRICS, obs.Family.EVENTS, obs.Family.PROM}),
    ]
    for raw, want in cases:
        assert obs._parse_op_obs(raw) == want, raw
    for raw in ("all,otel", "all,sessions", "unknown"):
        with pytest.raises(obs.InvalidTokenError):
            obs._parse_op_obs(raw)


def test_check_env_rejects_unknown_tokens():
    os.environ["OP_OBS"] = "logs,otel"
    with pytest.raises(obs.InvalidTokenError):
        obs.check_env()
    os.environ["OP_OBS"] = "bogus"
    with pytest.raises(obs.InvalidTokenError):
        obs.check_env()
    os.environ["OP_OBS"] = "logs,sessions"
    with pytest.raises(obs.InvalidTokenError):
        obs.check_env()


def test_check_env_valid():
    os.environ["OP_OBS"] = "logs,metrics,events,prom,all"
    obs.check_env()


def test_disabled_is_noop():
    o = obs.configure(obs.Config(slug="t"))
    assert not o.enabled(obs.Family.LOGS)
    assert not o.enabled(obs.Family.METRICS)
    o.logger("x").info("dropped", k="v")
    assert o.counter("x", "") is None


def test_logger_emits_log_record_with_typed_any_values_and_resource_attrs():
    os.environ["OP_OBS"] = "logs"
    o = obs.configure(obs.Config(slug="g", instance_uid="uid"))
    logger = o.logger("r")
    with obs.session_context("sid-1", "/greeting.v1.Greeting/SayHello"):
        logger.info(
            "hello",
            who="bob",
            ok=True,
            count=7,
            ratio=1.25,
            other={"x": 1},
        )
    logger.debug("dropped")
    logger.warn("retry")

    entries = o.log_ring.drain()
    assert len(entries) == 2
    record = entries[0].record
    attrs = _attrs(record)

    assert record.DESCRIPTOR.full_name == "holons.v1.LogRecord"
    assert record.body.string_value == "hello"
    assert record.severity_number == observability_pb2.SEVERITY_NUMBER_INFO
    assert record.severity_text == "INFO"
    assert attrs[obs.ATTR_HOLONS_SLUG].string_value == "g"
    assert attrs[obs.ATTR_SERVICE_NAME].string_value == "g"
    assert attrs[obs.ATTR_HOLONS_INSTANCE_UID].string_value == "uid"
    assert attrs[obs.ATTR_SERVICE_INSTANCE_ID].string_value == "uid"
    assert attrs[obs.ATTR_HOLONS_SESSION_ID].string_value == "sid-1"
    assert attrs[obs.ATTR_RPC_METHOD].string_value == "/greeting.v1.Greeting/SayHello"
    assert attrs[obs.ATTR_LOGGER_NAME].string_value == "r"
    assert attrs["who"].string_value == "bob"
    assert attrs["ok"].bool_value is True
    assert attrs["count"].int_value == 7
    assert attrs["ratio"].double_value == 1.25
    assert attrs["other"].string_value == "{'x': 1}"
    assert entries[1].record.severity_number == observability_pb2.SEVERITY_NUMBER_WARN


def test_redaction():
    os.environ["OP_OBS"] = "logs"
    o = obs.configure(obs.Config(slug="g", redacted_fields=["password", "api_key"]))
    o.logger("auth").info("login", user="bob", password="secret", api_key="abc")
    attrs = _attrs(o.log_ring.drain()[0].record)
    assert attrs["user"].string_value == "bob"
    assert attrs["password"].string_value == "<redacted>"
    assert attrs["api_key"].string_value == "<redacted>"


def test_counter_atomic():
    os.environ["OP_OBS"] = "metrics"
    o = obs.configure(obs.Config(slug="g"))
    counter = o.counter("t_total", "")
    assert counter is not None

    def worker():
        for _ in range(100):
            counter.inc()

    threads = [threading.Thread(target=worker) for _ in range(20)]
    for thread in threads:
        thread.start()
    for thread in threads:
        thread.join()
    assert counter.value() == 2000


def test_histogram_percentile():
    os.environ["OP_OBS"] = "metrics"
    o = obs.configure(obs.Config(slug="g"))
    histogram = o.histogram("latency_s", "", bounds=[1e-3, 1e-2, 1e-1, 1.0])
    for _ in range(900):
        histogram.observe(0.5e-3)
    for _ in range(100):
        histogram.observe(0.5)
    snap = histogram.snapshot()
    assert snap.quantile(0.5) == 1e-3
    assert snap.quantile(0.99) == 1.0
    assert snap.min == 0.5e-3
    assert snap.max == 0.5


def test_metrics_emit_otlp_oneofs():
    os.environ["OP_OBS"] = "metrics"
    o = obs.configure(obs.Config(slug="g", instance_uid="uid"))
    counter = o.counter("requests_total", "requests", {"route": "sayHello"})
    gauge = o.gauge("queue_depth", "depth")
    histogram = o.histogram("latency_s", "latency", bounds=[0.1, 1.0])
    assert counter is not None
    assert gauge is not None
    assert histogram is not None
    counter.inc(3)
    gauge.set(2.5)
    histogram.observe(0.05)
    histogram.observe(2.0)

    metrics = {
        metric.name: metric
        for metric in obs.to_proto_metrics(
            o.registry.snapshot(),
            o.cfg.slug,
            o.cfg.instance_uid,
            o.start_wall,
        )
    }

    total = metrics["requests_total"]
    assert total.WhichOneof("data") == "sum"
    assert total.sum.is_monotonic is True
    assert total.sum.aggregation_temporality == observability_pb2.AGGREGATION_TEMPORALITY_CUMULATIVE
    point = total.sum.data_points[0]
    assert point.WhichOneof("value") == "as_int"
    assert point.as_int == 3
    assert _attrs_from(point.attributes)[obs.ATTR_HOLONS_SLUG].string_value == "g"
    assert _attrs_from(point.attributes)["route"].string_value == "sayHello"

    depth = metrics["queue_depth"]
    assert depth.WhichOneof("data") == "gauge"
    point = depth.gauge.data_points[0]
    assert point.WhichOneof("value") == "as_double"
    assert point.as_double == 2.5

    latency = metrics["latency_s"]
    assert latency.WhichOneof("data") == "histogram"
    hist_point = latency.histogram.data_points[0]
    assert hist_point.count == 2
    assert hist_point.sum == 2.05
    assert list(hist_point.explicit_bounds) == [0.1, 1.0]
    assert list(hist_point.bucket_counts) == [1, 0, 1]
    assert hist_point.min == 0.05
    assert hist_point.max == 2.0


def test_eventbus_fanout_emits_log_record_event_name():
    os.environ["OP_OBS"] = "events"
    o = obs.configure(obs.Config(slug="g", instance_uid="uid"))
    received: list[obs.LogRecord] = []
    o.event_bus.subscribe(lambda e: received.append(e))
    o.emit(obs.EVENT_INSTANCE_READY, {"listener": "stdio://"})
    time.sleep(0.01)
    assert len(received) == 1
    record = received[0].record
    assert record.event_name == obs.EVENT_INSTANCE_READY
    assert record.body.string_value == obs.EVENT_INSTANCE_READY
    assert _attrs(record)["listener"].string_value == "stdio://"


def test_event_name_must_be_canonical():
    os.environ["OP_OBS"] = "events"
    o = obs.configure(obs.Config(slug="g", instance_uid="uid"))
    with pytest.raises(ValueError):
        o.emit("not.canonical")


def test_chain_append_and_enrich():
    c1 = obs.append_direct_child([], "gabriel-greeting-rust")
    assert c1 == ["gabriel-greeting-rust"]
    c2 = obs.enrich_for_multilog(c1, "gabriel-greeting-go")
    assert c2 == ["gabriel-greeting-rust", "gabriel-greeting-go"]


def test_is_organism_root():
    os.environ["OP_OBS"] = ""
    o = obs.configure(obs.Config(slug="g", instance_uid="x", organism_uid="x"))
    assert o.is_organism_root()
    obs.reset()
    o2 = obs.configure(obs.Config(slug="g", instance_uid="x", organism_uid="y"))
    assert not o2.is_organism_root()


def test_current_stub_when_unset():
    current = obs.current()
    assert current is not None
    current.logger("x").info("safe")


def test_run_dir_uses_registry_root(tmp_path):
    os.environ["OP_OBS"] = "logs"
    root = tmp_path / "runs"
    o = obs.configure(obs.Config(slug="gabriel", instance_uid="uid-1", run_dir=str(root)))
    assert o.cfg.run_dir == str(root / "gabriel" / "uid-1")


def test_disk_writers_and_meta_json(tmp_path):
    os.environ["OP_OBS"] = "logs,events"
    o = obs.configure(obs.Config(slug="gabriel", instance_uid="uid-1", run_dir=str(tmp_path / "runs")))
    obs.enable_disk_writers(o.cfg.run_dir)
    o.logger("test").info("ready", port=123)
    o.emit(obs.EVENT_INSTANCE_READY, {"listener": "tcp://127.0.0.1:123"})
    obs.write_meta_json(
        o.cfg.run_dir,
        obs.MetaJSON(
            slug="gabriel",
            uid="uid-1",
            pid=42,
            started_at=1.0,
            transport="tcp",
            address="tcp://127.0.0.1:123",
            log_path=os.path.join(o.cfg.run_dir, "stdout.log"),
        ),
    )

    assert (tmp_path / "runs" / "gabriel" / "uid-1" / "stdout.log").read_text().count("ready") == 1
    assert (tmp_path / "runs" / "gabriel" / "uid-1" / "events.jsonl").read_text().count("instance.ready") == 1
    meta = obs.read_meta_json(o.cfg.run_dir)
    assert meta["slug"] == "gabriel"
    assert meta["uid"] == "uid-1"
    assert meta["address"] == "tcp://127.0.0.1:123"


def test_holon_observability_service_replays_rings():
    os.environ["OP_OBS"] = "logs,metrics,events"
    o = obs.configure(obs.Config(slug="gabriel", instance_uid="uid-1"))
    o.logger("test").info("hello")
    counter = o.counter("requests_total", "requests")
    assert counter is not None
    counter.inc()
    o.emit(obs.EVENT_INSTANCE_READY, {"listener": "stdio://"})

    svc = obs.HolonObservabilityService(o)
    ctx = _FakeContext()

    logs = list(svc.Logs(observability_pb2.LogsRequest(follow=False), ctx))
    assert [entry.body.string_value for entry in logs] == ["hello"]
    assert _attrs_from(logs[0].attributes)[obs.ATTR_HOLONS_INSTANCE_UID].string_value == "uid-1"

    metrics = list(svc.Metrics(observability_pb2.MetricsRequest(), ctx))
    assert any(
        metric.name == "requests_total"
        and metric.WhichOneof("data") == "sum"
        and metric.sum.data_points[0].as_int == 1
        for metric in metrics
    )

    events = list(svc.Events(observability_pb2.EventsRequest(follow=False), ctx))
    assert [event.event_name for event in events] == [obs.EVENT_INSTANCE_READY]


def test_logs_follow_replays_ring_on_subscribe():
    os.environ["OP_OBS"] = "logs"
    o = obs.configure(obs.Config(slug="gabriel", instance_uid="uid-1"))
    o.logger("test").info("before")
    svc = obs.HolonObservabilityService(o)
    ctx = _ActiveContext()
    stream = svc.Logs(observability_pb2.LogsRequest(follow=True), ctx)
    try:
        first = next(stream)
        assert first.body.string_value == "before"
        o.logger("test").info("after")
        second = _next_with_timeout(stream)
        assert second.body.string_value == "after"
    finally:
        ctx.cancel()
        stream.close()


def test_events_follow_replays_ring_on_subscribe():
    os.environ["OP_OBS"] = "events"
    o = obs.configure(obs.Config(slug="gabriel", instance_uid="uid-1"))
    o.emit(obs.EVENT_INSTANCE_READY, {"listener": "stdio://"})
    svc = obs.HolonObservabilityService(o)
    ctx = _ActiveContext()
    stream = svc.Events(observability_pb2.EventsRequest(follow=True), ctx)
    try:
        first = next(stream)
        assert first.event_name == obs.EVENT_INSTANCE_READY
        o.emit(obs.EVENT_CONFIG_RELOADED, {"listener": "tcp://127.0.0.1:1"})
        second = _next_with_timeout(stream)
        assert second.event_name == obs.EVENT_CONFIG_RELOADED
    finally:
        ctx.cancel()
        stream.close()


def test_proto_roundtrip_helpers_preserve_chain():
    proto = observability_pb2.LogRecord(
        time_unix_nano=1_250_000_000,
        severity_number=observability_pb2.SEVERITY_NUMBER_INFO,
        severity_text="INFO",
        body=obs.to_any_value("tick received"),
        attributes=[
            obs.key_value(obs.ATTR_HOLONS_SLUG, "child"),
            obs.key_value(obs.ATTR_HOLONS_INSTANCE_UID, "uid-child"),
            obs.key_value("sender", "sender-1"),
        ],
        chain=["D"],
    )
    got = obs.from_proto_log_record(obs.to_proto_log_record(obs.LogRecord(proto)))
    assert obs.body_string(got.record) == "tick received"
    assert _attrs(got.record)["sender"].string_value == "sender-1"
    assert list(got.record.chain) == ["D"]


def test_prometheus_text_and_server():
    os.environ["OP_OBS"] = "metrics,prom"
    o = obs.configure(obs.Config(slug="node", instance_uid="uid-node"))
    counter = o.counter(
        "cascade_ticks_total",
        "Ticks received by this cascade node.",
        {"responder_uid": "uid-node"},
    )
    assert counter is not None
    counter.inc()

    text = obs.to_prometheus_text(o)
    assert "# HELP cascade_ticks_total Ticks received by this cascade node." in text
    assert 'cascade_ticks_total{instance_uid="uid-node",responder_uid="uid-node",slug="node"} 1' in text

    server = obs.PromServer(":0")
    try:
        url = server.start()
        with urlopen(url, timeout=5) as response:
            body = response.read().decode("utf-8")
        assert "cascade_ticks_total" in body
        assert 'responder_uid="uid-node"' in body
    finally:
        server.close()


def test_member_relay_forwards_logs_and_events_with_chain():
    os.environ["OP_OBS"] = "logs,events"
    child = obs.configure(obs.Config(slug="child", instance_uid="uid-child"))
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=2))
    obs.register_service(server, child)
    port = server.add_insecure_port("127.0.0.1:0")
    server.start()

    parent = obs.configure(obs.Config(slug="parent", instance_uid="uid-parent"))
    channel = grpc.insecure_channel(f"127.0.0.1:{port}")
    relay = obs.MemberRelay("child", "uid-child", channel, parent, retry_delay=0.05)
    try:
        relay.start()
        child.logger("tick").info("tick received", sender="sender-1")
        child.emit(obs.EVENT_INSTANCE_READY, {"listener": "tcp://127.0.0.1:1"})

        deadline = time.time() + 5
        while time.time() < deadline:
            logs = parent.log_ring.drain()
            events = parent.event_bus.drain()
            if logs and events:
                break
            time.sleep(0.02)

        logs = parent.log_ring.drain()
        events = parent.event_bus.drain()
        assert any(
            obs.body_string(entry.record) == "tick received"
            and list(entry.record.chain) == ["child"]
            for entry in logs
        )
        assert any(
            event.record.event_name == obs.EVENT_INSTANCE_READY
            and list(event.record.chain) == ["child"]
            for event in events
        )
    finally:
        relay.stop()
        channel.close()
        server.stop(0)


class _FakeContext:
    def abort(self, _code, details):
        raise RuntimeError(details)

    def is_active(self):
        return False


class _ActiveContext:
    def __init__(self):
        self._active = True

    def abort(self, _code, details):
        raise RuntimeError(details)

    def is_active(self):
        return self._active

    def cancel(self):
        self._active = False


def _next_with_timeout(stream, timeout: float = 2.0):
    with futures.ThreadPoolExecutor(max_workers=1) as pool:
        return pool.submit(next, stream).result(timeout=timeout)


def _attrs(record: observability_pb2.LogRecord) -> dict[str, observability_pb2.AnyValue]:
    return _attrs_from(record.attributes)


def _attrs_from(attrs) -> dict[str, observability_pb2.AnyValue]:
    return {attr.key: attr.value for attr in attrs}
