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
    os.environ.pop("OP_SESSIONS", None)


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


def test_check_env_otel_rejected():
    os.environ["OP_OBS"] = "logs,otel"
    with pytest.raises(obs.InvalidTokenError):
        obs.check_env()


def test_check_env_unknown_rejected():
    os.environ["OP_OBS"] = "bogus"
    with pytest.raises(obs.InvalidTokenError):
        obs.check_env()


def test_check_env_sessions_rejected():
    os.environ["OP_OBS"] = "logs,sessions"
    with pytest.raises(obs.InvalidTokenError):
        obs.check_env()


def test_check_env_op_sessions_rejected():
    os.environ["OP_SESSIONS"] = "metrics"
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


def test_logs_ring_and_level():
    os.environ["OP_OBS"] = "logs"
    o = obs.configure(obs.Config(slug="g", instance_uid="uid"))
    l = o.logger("r")
    l.info("hello", who="bob")
    l.debug("dropped")
    l.warn("retry")
    entries = o.log_ring.drain()
    assert len(entries) == 2
    assert entries[0].message == "hello"
    assert entries[0].fields["who"] == "bob"
    assert entries[0].slug == "g"
    assert entries[0].instance_uid == "uid"


def test_redaction():
    os.environ["OP_OBS"] = "logs"
    o = obs.configure(obs.Config(slug="g", redacted_fields=["password", "api_key"]))
    o.logger("auth").info("login", user="bob", password="secret", api_key="abc")
    e = o.log_ring.drain()[0]
    assert e.fields["user"] == "bob"
    assert e.fields["password"] == "<redacted>"
    assert e.fields["api_key"] == "<redacted>"


def test_counter_atomic():
    os.environ["OP_OBS"] = "metrics"
    o = obs.configure(obs.Config(slug="g"))
    c = o.counter("t_total", "")
    assert c is not None

    def worker():
        for _ in range(100):
            c.inc()

    threads = [threading.Thread(target=worker) for _ in range(20)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()
    assert c.value() == 2000


def test_histogram_percentile():
    os.environ["OP_OBS"] = "metrics"
    o = obs.configure(obs.Config(slug="g"))
    h = o.histogram("latency_s", "", bounds=[1e-3, 1e-2, 1e-1, 1.0])
    for _ in range(900):
        h.observe(0.5e-3)
    for _ in range(100):
        h.observe(0.5)
    snap = h.snapshot()
    assert snap.quantile(0.5) == 1e-3
    assert snap.quantile(0.99) == 1.0


def test_eventbus_fanout():
    os.environ["OP_OBS"] = "events"
    o = obs.configure(obs.Config(slug="g", instance_uid="uid"))
    received: list[obs.Event] = []
    o.event_bus.subscribe(lambda e: received.append(e))
    o.emit(obs.EventType.INSTANCE_READY, {"listener": "stdio://"})
    time.sleep(0.01)
    assert len(received) == 1
    assert received[0].type == obs.EventType.INSTANCE_READY


def test_chain_append_and_enrich():
    c1 = obs.append_direct_child([], "gabriel-greeting-rust", "1c2d")
    assert len(c1) == 1
    assert c1[0].slug == "gabriel-greeting-rust"
    c2 = obs.enrich_for_multilog(c1, "gabriel-greeting-go", "ea34")
    assert len(c2) == 2
    assert c2[0].slug == "gabriel-greeting-rust"
    assert c2[1].slug == "gabriel-greeting-go"


def test_is_organism_root():
    os.environ["OP_OBS"] = ""
    o = obs.configure(obs.Config(slug="g", instance_uid="x", organism_uid="x"))
    assert o.is_organism_root()
    obs.reset()
    o2 = obs.configure(obs.Config(slug="g", instance_uid="x", organism_uid="y"))
    assert not o2.is_organism_root()


def test_current_stub_when_unset():
    c = obs.current()
    assert c is not None
    c.logger("x").info("safe")


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
    o.emit(obs.EventType.INSTANCE_READY, {"listener": "tcp://127.0.0.1:123"})
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
    assert (tmp_path / "runs" / "gabriel" / "uid-1" / "events.jsonl").read_text().count("INSTANCE_READY") == 1
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
    o.emit(obs.EventType.INSTANCE_READY, {"listener": "stdio://"})

    svc = obs.HolonObservabilityService(o)
    ctx = _FakeContext()

    logs = list(svc.Logs(observability_pb2.LogsRequest(follow=False), ctx))
    assert [entry.message for entry in logs] == ["hello"]

    metrics = svc.Metrics(observability_pb2.MetricsRequest(), ctx)
    assert metrics.slug == "gabriel"
    assert any(sample.name == "requests_total" and sample.counter == 1 for sample in metrics.samples)

    events = list(svc.Events(observability_pb2.EventsRequest(follow=False), ctx))
    assert [event.type for event in events] == [observability_pb2.INSTANCE_READY]


def test_proto_roundtrip_helpers_preserve_chain():
    entry = obs.LogEntry(
        timestamp=1.25,
        level=obs.Level.INFO,
        slug="child",
        instance_uid="uid-child",
        message="tick received",
        fields={"sender": "sender-1"},
        chain=[obs.Hop("D", "uid-d")],
    )
    got_entry = obs.from_proto_log_entry(obs.to_proto_log_entry(entry))
    assert got_entry.message == "tick received"
    assert got_entry.fields["sender"] == "sender-1"
    assert [(hop.slug, hop.instance_uid) for hop in got_entry.chain] == [("D", "uid-d")]

    event = obs.Event(
        timestamp=2.5,
        type=obs.EventType.INSTANCE_READY,
        slug="child",
        instance_uid="uid-child",
        payload={"listener": "tcp://127.0.0.1:1"},
        chain=[obs.Hop("C", "uid-c")],
    )
    got_event = obs.from_proto_event(obs.to_proto_event(event))
    assert got_event.type == obs.EventType.INSTANCE_READY
    assert got_event.payload["listener"] == "tcp://127.0.0.1:1"
    assert [(hop.slug, hop.instance_uid) for hop in got_event.chain] == [("C", "uid-c")]


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
        child.emit(obs.EventType.INSTANCE_READY, {"listener": "tcp://127.0.0.1:1"})

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
            entry.message == "tick received"
            and [(hop.slug, hop.instance_uid) for hop in entry.chain] == [("child", "uid-child")]
            for entry in logs
        )
        assert any(
            event.type == obs.EventType.INSTANCE_READY
            and [(hop.slug, hop.instance_uid) for hop in event.chain] == [("child", "uid-child")]
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
