"""Unit tests for holons.observability, the Python port."""

from __future__ import annotations

import os
import threading
import time

import pytest

from holons import observability as obs


def setup_function(_):
    obs.reset()
    os.environ.pop("OP_OBS", None)


def test_parse_op_obs_basic():
    cases = [
        ("", set()),
        ("logs", {obs.Family.LOGS}),
        ("logs,metrics", {obs.Family.LOGS, obs.Family.METRICS}),
        ("all", {obs.Family.LOGS, obs.Family.METRICS, obs.Family.EVENTS, obs.Family.PROM}),
        ("all,otel", {obs.Family.LOGS, obs.Family.METRICS, obs.Family.EVENTS, obs.Family.PROM}),
        ("unknown", set()),
    ]
    for raw, want in cases:
        assert obs._parse_op_obs(raw) == want, raw


def test_check_env_otel_rejected():
    os.environ["OP_OBS"] = "logs,otel"
    with pytest.raises(obs.InvalidTokenError):
        obs.check_env()


def test_check_env_unknown_rejected():
    os.environ["OP_OBS"] = "bogus"
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
