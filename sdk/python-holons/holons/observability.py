"""Python reference implementation of the cross-SDK observability layer.

Mirrors sdk/go-holons/pkg/observability — same activation model
(OP_OBS env, zero cost when disabled), same public surface
(Logger/Counter/Gauge/Histogram/EventBus/chain helpers), same proto
types (holons.v1.HolonObservability). See OBSERVABILITY.md.
"""

from __future__ import annotations

import enum
import inspect
import json
import math
import os
import queue
import sys
import threading
import time
import typing
import uuid
from collections import deque
from dataclasses import dataclass, field

import grpc
from google.protobuf.timestamp_pb2 import Timestamp

from holons.v1 import observability_pb2, observability_pb2_grpc


# --- OP_OBS parsing -----------------------------------------------------------


class Family(str, enum.Enum):
    LOGS = "logs"
    METRICS = "metrics"
    EVENTS = "events"
    PROM = "prom"
    OTEL = "otel"  # reserved v2


_V1_TOKENS = {"logs", "metrics", "events", "prom", "all"}


class InvalidTokenError(ValueError):
    """Raised when OP_OBS contains an unknown or v2-only token."""


def _parse_op_obs(raw: str) -> set[Family]:
    out: set[Family] = set()
    if not raw or not raw.strip():
        return out
    for tok in (t.strip() for t in raw.split(",")):
        if not tok:
            continue
        if tok in {"otel", "sessions"}:
            # v2 reserved token; swallowed silently here, rejected by check_env.
            continue
        if tok not in _V1_TOKENS:
            continue
        if tok == "all":
            out.update({Family.LOGS, Family.METRICS, Family.EVENTS, Family.PROM})
        else:
            out.add(Family(tok))
    return out


def check_env() -> None:
    """Raise InvalidTokenError if OP_OBS contains an unknown or v2 token."""
    sessions = os.environ.get("OP_SESSIONS", "").strip()
    if sessions:
        raise InvalidTokenError(
            f"OP_SESSIONS is reserved for v2; not implemented in v1: {sessions}"
        )
    raw = os.environ.get("OP_OBS", "").strip()
    if not raw:
        return
    for tok in (t.strip() for t in raw.split(",")):
        if not tok:
            continue
        if tok == "otel":
            raise InvalidTokenError(
                "otel export is reserved for v2; not implemented in v1"
            )
        if tok == "sessions":
            raise InvalidTokenError(
                "sessions are reserved for v2; not implemented in v1"
            )
        if tok not in _V1_TOKENS:
            raise InvalidTokenError(f"unknown OP_OBS token: {tok}")


# --- Levels and log entries ---------------------------------------------------


class Level(enum.IntEnum):
    UNSET = 0
    TRACE = 1
    DEBUG = 2
    INFO = 3
    WARN = 4
    ERROR = 5
    FATAL = 6


_LEVEL_NAMES = {
    Level.TRACE: "TRACE",
    Level.DEBUG: "DEBUG",
    Level.INFO: "INFO",
    Level.WARN: "WARN",
    Level.ERROR: "ERROR",
    Level.FATAL: "FATAL",
}

_PROTO_LOG_LEVEL = {
    Level.TRACE: observability_pb2.TRACE,
    Level.DEBUG: observability_pb2.DEBUG,
    Level.INFO: observability_pb2.INFO,
    Level.WARN: observability_pb2.WARN,
    Level.ERROR: observability_pb2.ERROR,
    Level.FATAL: observability_pb2.FATAL,
}


def parse_level(s: str) -> Level:
    if not s:
        return Level.INFO
    u = s.strip().upper()
    return {
        "TRACE": Level.TRACE, "DEBUG": Level.DEBUG, "INFO": Level.INFO,
        "WARN": Level.WARN, "WARNING": Level.WARN,
        "ERROR": Level.ERROR, "FATAL": Level.FATAL,
    }.get(u, Level.INFO)


@dataclass
class Hop:
    slug: str = ""
    instance_uid: str = ""


@dataclass
class LogEntry:
    timestamp: float = 0.0       # unix seconds
    level: Level = Level.INFO
    slug: str = ""
    instance_uid: str = ""
    session_id: str = ""
    rpc_method: str = ""
    message: str = ""
    fields: dict[str, str] = field(default_factory=dict)
    caller: str = ""
    chain: list[Hop] = field(default_factory=list)


# --- Chain helpers ------------------------------------------------------------


def append_direct_child(src: list[Hop], child_slug: str, child_uid: str) -> list[Hop]:
    """SDK rule on relay: append the direct child whose stream was just read."""
    return list(src) + [Hop(slug=child_slug, instance_uid=child_uid)]


def enrich_for_multilog(wire: list[Hop], stream_source_slug: str, stream_source_uid: str) -> list[Hop]:
    """Root-side rule: append the stream source before writing a multilog entry."""
    return append_direct_child(wire, stream_source_slug, stream_source_uid)


# --- Ring buffer --------------------------------------------------------------


class LogRing:
    def __init__(self, capacity: int = 1024) -> None:
        self._capacity = max(1, capacity)
        self._buf: deque[LogEntry] = deque(maxlen=self._capacity)
        self._lock = threading.Lock()
        self._subs: list[typing.Callable[[LogEntry], None]] = []

    def push(self, entry: LogEntry) -> None:
        with self._lock:
            self._buf.append(entry)
            subs = list(self._subs)
        for s in subs:
            try:
                s(entry)
            except Exception:
                pass

    def drain(self) -> list[LogEntry]:
        with self._lock:
            return list(self._buf)

    def drain_since(self, cutoff: float) -> list[LogEntry]:
        with self._lock:
            return [e for e in self._buf if e.timestamp >= cutoff]

    def subscribe(self, fn: typing.Callable[[LogEntry], None]) -> typing.Callable[[], None]:
        with self._lock:
            self._subs.append(fn)

        def unsub() -> None:
            with self._lock:
                try:
                    self._subs.remove(fn)
                except ValueError:
                    pass
        return unsub

    def capacity(self) -> int:
        return self._capacity

    def __len__(self) -> int:
        return len(self._buf)


# --- Metrics ------------------------------------------------------------------


@dataclass
class HistogramSnapshot:
    bounds: list[float]
    counts: list[int]
    total: int
    sum: float

    def quantile(self, q: float) -> float:
        if self.total == 0:
            return float("nan")
        target = self.total * q
        for i, c in enumerate(self.counts):
            if c >= target:
                return self.bounds[i]
        return math.inf


DEFAULT_BUCKETS = [
    50e-6, 100e-6, 250e-6, 500e-6,
    1e-3, 2.5e-3, 5e-3, 10e-3, 25e-3, 50e-3, 100e-3, 250e-3, 500e-3,
    1.0, 2.5, 5.0, 10.0, 30.0, 60.0,
]


class Counter:
    def __init__(self, name: str, help_: str = "", labels: typing.Optional[dict[str, str]] = None) -> None:
        self.name = name
        self.help = help_
        self.labels = dict(labels or {})
        self._value = 0
        self._lock = threading.Lock()

    def inc(self, n: int = 1) -> None:
        if n < 0:
            return
        with self._lock:
            self._value += n

    def add(self, n: int) -> None:
        self.inc(n)

    def value(self) -> int:
        with self._lock:
            return self._value


class Gauge:
    def __init__(self, name: str, help_: str = "", labels: typing.Optional[dict[str, str]] = None) -> None:
        self.name = name
        self.help = help_
        self.labels = dict(labels or {})
        self._value = 0.0
        self._lock = threading.Lock()

    def set(self, v: float) -> None:
        with self._lock:
            self._value = float(v)

    def add(self, delta: float) -> None:
        with self._lock:
            self._value += float(delta)

    def value(self) -> float:
        with self._lock:
            return self._value


class Histogram:
    def __init__(
        self,
        name: str,
        help_: str = "",
        labels: typing.Optional[dict[str, str]] = None,
        bounds: typing.Optional[list[float]] = None,
    ) -> None:
        self.name = name
        self.help = help_
        self.labels = dict(labels or {})
        self._bounds = sorted(bounds or DEFAULT_BUCKETS)
        self._counts = [0] * len(self._bounds)
        self._total = 0
        self._sum = 0.0
        self._lock = threading.Lock()

    def observe(self, v: float) -> None:
        with self._lock:
            self._total += 1
            self._sum += float(v)
            for i, b in enumerate(self._bounds):
                if v <= b:
                    self._counts[i] += 1

    def observe_duration(self, seconds: float) -> None:
        self.observe(seconds)

    def snapshot(self) -> HistogramSnapshot:
        with self._lock:
            return HistogramSnapshot(
                bounds=list(self._bounds),
                counts=list(self._counts),
                total=self._total,
                sum=self._sum,
            )


def _metric_key(name: str, labels: dict[str, str]) -> str:
    if not labels:
        return name
    return name + "|" + ",".join(f"{k}={v}" for k, v in sorted(labels.items()))


class Registry:
    def __init__(self) -> None:
        self._counters: dict[str, Counter] = {}
        self._gauges: dict[str, Gauge] = {}
        self._histograms: dict[str, Histogram] = {}
        self._lock = threading.Lock()

    def counter(self, name: str, help_: str = "", labels: typing.Optional[dict[str, str]] = None) -> Counter:
        labels = labels or {}
        key = _metric_key(name, labels)
        with self._lock:
            c = self._counters.get(key)
            if c is None:
                c = Counter(name, help_, labels)
                self._counters[key] = c
            return c

    def gauge(self, name: str, help_: str = "", labels: typing.Optional[dict[str, str]] = None) -> Gauge:
        labels = labels or {}
        key = _metric_key(name, labels)
        with self._lock:
            g = self._gauges.get(key)
            if g is None:
                g = Gauge(name, help_, labels)
                self._gauges[key] = g
            return g

    def histogram(
        self,
        name: str,
        help_: str = "",
        labels: typing.Optional[dict[str, str]] = None,
        bounds: typing.Optional[list[float]] = None,
    ) -> Histogram:
        labels = labels or {}
        key = _metric_key(name, labels)
        with self._lock:
            h = self._histograms.get(key)
            if h is None:
                h = Histogram(name, help_, labels, bounds)
                self._histograms[key] = h
            return h

    def snapshot(self) -> dict[str, typing.Any]:
        with self._lock:
            counters = [(c.name, c.help, dict(c.labels), c.value()) for c in self._counters.values()]
            gauges = [(g.name, g.help, dict(g.labels), g.value()) for g in self._gauges.values()]
            hists = [(h.name, h.help, dict(h.labels), h.snapshot()) for h in self._histograms.values()]
        counters.sort(key=lambda x: x[0])
        gauges.sort(key=lambda x: x[0])
        hists.sort(key=lambda x: x[0])
        return {
            "captured_at": time.time(),
            "counters": counters,
            "gauges": gauges,
            "histograms": hists,
        }


# --- Events -------------------------------------------------------------------


class EventType(enum.IntEnum):
    UNSPECIFIED = 0
    INSTANCE_SPAWNED = 1
    INSTANCE_READY = 2
    INSTANCE_EXITED = 3
    INSTANCE_CRASHED = 4
    SESSION_STARTED = 5
    SESSION_ENDED = 6
    HANDLER_PANIC = 7
    CONFIG_RELOADED = 8


_PROTO_EVENT_TYPE = {
    EventType.INSTANCE_SPAWNED: observability_pb2.INSTANCE_SPAWNED,
    EventType.INSTANCE_READY: observability_pb2.INSTANCE_READY,
    EventType.INSTANCE_EXITED: observability_pb2.INSTANCE_EXITED,
    EventType.INSTANCE_CRASHED: observability_pb2.INSTANCE_CRASHED,
    EventType.SESSION_STARTED: observability_pb2.SESSION_STARTED,
    EventType.SESSION_ENDED: observability_pb2.SESSION_ENDED,
    EventType.HANDLER_PANIC: observability_pb2.HANDLER_PANIC,
    EventType.CONFIG_RELOADED: observability_pb2.CONFIG_RELOADED,
}


@dataclass
class Event:
    timestamp: float = 0.0
    type: EventType = EventType.UNSPECIFIED
    slug: str = ""
    instance_uid: str = ""
    session_id: str = ""
    payload: dict[str, str] = field(default_factory=dict)
    chain: list[Hop] = field(default_factory=list)


class EventBus:
    def __init__(self, capacity: int = 256) -> None:
        self._capacity = max(1, capacity)
        self._buf: deque[Event] = deque(maxlen=self._capacity)
        self._lock = threading.Lock()
        self._subs: list[typing.Callable[[Event], None]] = []
        self._closed = False

    def emit(self, e: Event) -> None:
        with self._lock:
            if self._closed:
                return
            self._buf.append(e)
            subs = list(self._subs)
        for s in subs:
            try:
                s(e)
            except Exception:
                pass

    def drain(self) -> list[Event]:
        with self._lock:
            return list(self._buf)

    def drain_since(self, cutoff: float) -> list[Event]:
        with self._lock:
            return [e for e in self._buf if e.timestamp >= cutoff]

    def subscribe(self, fn: typing.Callable[[Event], None]) -> typing.Callable[[], None]:
        with self._lock:
            self._subs.append(fn)

        def unsub() -> None:
            with self._lock:
                try:
                    self._subs.remove(fn)
                except ValueError:
                    pass
        return unsub

    def close(self) -> None:
        with self._lock:
            self._closed = True
            self._subs.clear()


# --- Configuration and the observability singleton ---------------------------


@dataclass
class Config:
    slug: str = ""
    default_log_level: Level = Level.INFO
    prom_addr: str = ""
    redacted_fields: list[str] = field(default_factory=list)
    logs_ring_size: int = 1024
    events_ring_size: int = 256
    run_dir: str = ""
    instance_uid: str = ""
    organism_uid: str = ""
    organism_slug: str = ""


class Logger:
    def __init__(self, obs: "Observability", name: str) -> None:
        self._obs = obs
        self._name = name
        self._level = obs.cfg.default_log_level

    @property
    def name(self) -> str:
        return self._name

    def set_level(self, level: Level) -> None:
        self._level = level

    def enabled(self, level: Level) -> bool:
        return self._obs is not None and level >= self._level

    def _log(self, level: Level, message: str, **kv: typing.Any) -> None:
        if not self.enabled(level):
            return
        fields: dict[str, str] = {}
        redact = set(self._obs.cfg.redacted_fields)
        for k, v in kv.items():
            if not k:
                continue
            if k in redact:
                fields[k] = "<redacted>"
            else:
                fields[k] = _stringify(v)
        frame = inspect.currentframe()
        caller = ""
        try:
            if frame and frame.f_back and frame.f_back.f_back:
                f = frame.f_back.f_back
                caller = f"{os.path.basename(f.f_code.co_filename)}:{f.f_lineno}"
        finally:
            del frame
        entry = LogEntry(
            timestamp=time.time(),
            level=level,
            slug=self._obs.cfg.slug,
            instance_uid=self._obs.cfg.instance_uid,
            message=message,
            fields=fields,
            caller=caller,
        )
        if self._obs.log_ring is not None:
            self._obs.log_ring.push(entry)

    def trace(self, msg: str, **kv: typing.Any) -> None: self._log(Level.TRACE, msg, **kv)
    def debug(self, msg: str, **kv: typing.Any) -> None: self._log(Level.DEBUG, msg, **kv)
    def info(self, msg: str, **kv: typing.Any) -> None: self._log(Level.INFO, msg, **kv)
    def warn(self, msg: str, **kv: typing.Any) -> None: self._log(Level.WARN, msg, **kv)
    def error(self, msg: str, **kv: typing.Any) -> None: self._log(Level.ERROR, msg, **kv)
    def fatal(self, msg: str, **kv: typing.Any) -> None: self._log(Level.FATAL, msg, **kv)


def _stringify(v: typing.Any) -> str:
    if v is None:
        return ""
    if isinstance(v, bool):
        return "true" if v else "false"
    return str(v)


class Observability:
    def __init__(self, cfg: Config, families: set[Family]) -> None:
        self.cfg = cfg
        self.families = families
        self.log_ring: typing.Optional[LogRing] = LogRing(cfg.logs_ring_size) if Family.LOGS in families else None
        self.registry: typing.Optional[Registry] = Registry() if Family.METRICS in families else None
        self.event_bus: typing.Optional[EventBus] = EventBus(cfg.events_ring_size) if Family.EVENTS in families else None
        self._loggers: dict[str, Logger] = {}
        self._lock = threading.Lock()

    def enabled(self, family: Family) -> bool:
        return family in self.families

    def is_organism_root(self) -> bool:
        return bool(self.cfg.organism_uid) and self.cfg.organism_uid == self.cfg.instance_uid

    def logger(self, name: str) -> Logger:
        if Family.LOGS not in self.families:
            return _DISABLED_LOGGER
        with self._lock:
            l = self._loggers.get(name)
            if l is None:
                l = Logger(self, name)
                self._loggers[name] = l
            return l

    def counter(self, name: str, help_: str = "", labels: typing.Optional[dict[str, str]] = None) -> typing.Optional[Counter]:
        return self.registry.counter(name, help_, labels) if self.registry else None

    def gauge(self, name: str, help_: str = "", labels: typing.Optional[dict[str, str]] = None) -> typing.Optional[Gauge]:
        return self.registry.gauge(name, help_, labels) if self.registry else None

    def histogram(self, name: str, help_: str = "", labels: typing.Optional[dict[str, str]] = None, bounds: typing.Optional[list[float]] = None) -> typing.Optional[Histogram]:
        return self.registry.histogram(name, help_, labels, bounds) if self.registry else None

    def emit(self, event_type: EventType, payload: typing.Optional[dict[str, str]] = None) -> None:
        if self.event_bus is None:
            return
        p = dict(payload or {})
        redact = set(self.cfg.redacted_fields)
        for k in list(p.keys()):
            if k in redact:
                p[k] = "<redacted>"
        self.event_bus.emit(Event(
            timestamp=time.time(),
            type=event_type,
            slug=self.cfg.slug,
            instance_uid=self.cfg.instance_uid,
            payload=p,
        ))

    def close(self) -> None:
        if self.event_bus is not None:
            self.event_bus.close()


# No-op logger for the disabled path: instantiate once, share.
class _NoopObs:
    cfg = Config()


_DISABLED_OBS = _NoopObs()
_DISABLED_LOGGER = Logger.__new__(Logger)
_DISABLED_LOGGER._obs = None  # type: ignore[attr-defined]
_DISABLED_LOGGER._name = ""
_DISABLED_LOGGER._level = Level.FATAL  # high watermark so all levels fail enabled()


# Package-scope singleton.
_current_lock = threading.Lock()
_current: typing.Optional[Observability] = None


def configure(cfg: Config) -> Observability:
    check_env()
    families = _parse_op_obs(os.environ.get("OP_OBS", ""))
    if not cfg.slug:
        cfg.slug = os.path.basename(sys.argv[0]) if sys.argv else ""
    if not cfg.instance_uid:
        cfg.instance_uid = str(uuid.uuid4())
    if cfg.run_dir:
        cfg.run_dir = derive_run_dir(cfg.run_dir, cfg.slug, cfg.instance_uid)
    obs = Observability(cfg, families)
    global _current
    with _current_lock:
        _current = obs
    return obs


def from_env(base: typing.Optional[Config] = None) -> Observability:
    cfg = base if base is not None else Config()
    if not cfg.instance_uid:
        cfg.instance_uid = os.environ.get("OP_INSTANCE_UID", "")
    if not cfg.organism_uid:
        cfg.organism_uid = os.environ.get("OP_ORGANISM_UID", "")
    if not cfg.organism_slug:
        cfg.organism_slug = os.environ.get("OP_ORGANISM_SLUG", "")
    if not cfg.prom_addr:
        cfg.prom_addr = os.environ.get("OP_PROM_ADDR", "")
    if not cfg.run_dir:
        cfg.run_dir = os.environ.get("OP_RUN_DIR", "")
    return configure(cfg)


def current() -> Observability:
    with _current_lock:
        if _current is None:
            # Disabled stub: return a fresh Observability with no families.
            return Observability(Config(), set())
        return _current


def reset() -> None:
    global _current
    with _current_lock:
        if _current is not None:
            _current.close()
        _current = None


def derive_run_dir(root: str, slug: str, uid: str) -> str:
    """Return the v1 instance directory under the OP_RUN_DIR registry root."""
    if not root or not slug or not uid:
        return root
    return os.path.join(root, slug, uid)


# --- Proto conversion + gRPC service -----------------------------------------


def _timestamp(unix_seconds: float) -> Timestamp:
    ts = Timestamp()
    ts.FromSeconds(int(unix_seconds))
    ts.nanos = int((unix_seconds - int(unix_seconds)) * 1_000_000_000)
    return ts


def _hop_to_proto(hop: Hop) -> observability_pb2.ChainHop:
    return observability_pb2.ChainHop(slug=hop.slug, instance_uid=hop.instance_uid)


def to_proto_log_entry(entry: LogEntry) -> observability_pb2.LogEntry:
    return observability_pb2.LogEntry(
        ts=_timestamp(entry.timestamp),
        level=_PROTO_LOG_LEVEL.get(entry.level, observability_pb2.LOG_LEVEL_UNSPECIFIED),
        slug=entry.slug,
        instance_uid=entry.instance_uid,
        session_id=entry.session_id,
        rpc_method=entry.rpc_method,
        message=entry.message,
        fields=dict(entry.fields),
        caller=entry.caller,
        chain=[_hop_to_proto(h) for h in entry.chain],
    )


def _histogram_to_proto(snapshot: HistogramSnapshot) -> observability_pb2.HistogramSample:
    return observability_pb2.HistogramSample(
        buckets=[
            observability_pb2.Bucket(upper_bound=upper, count=count)
            for upper, count in zip(snapshot.bounds, snapshot.counts)
        ],
        count=snapshot.total,
        sum=snapshot.sum,
    )


def to_proto_metric_samples(snapshot: dict[str, typing.Any]) -> list[observability_pb2.MetricSample]:
    samples: list[observability_pb2.MetricSample] = []
    for name, help_, labels, value in snapshot.get("counters", []):
        samples.append(observability_pb2.MetricSample(
            name=name,
            help=help_,
            labels=labels,
            counter=value,
        ))
    for name, help_, labels, value in snapshot.get("gauges", []):
        samples.append(observability_pb2.MetricSample(
            name=name,
            help=help_,
            labels=labels,
            gauge=value,
        ))
    for name, help_, labels, hist in snapshot.get("histograms", []):
        samples.append(observability_pb2.MetricSample(
            name=name,
            help=help_,
            labels=labels,
            histogram=_histogram_to_proto(hist),
        ))
    return samples


def to_proto_event(event: Event) -> observability_pb2.EventInfo:
    return observability_pb2.EventInfo(
        ts=_timestamp(event.timestamp),
        type=_PROTO_EVENT_TYPE.get(event.type, observability_pb2.EVENT_TYPE_UNSPECIFIED),
        slug=event.slug,
        instance_uid=event.instance_uid,
        session_id=event.session_id,
        payload=dict(event.payload),
        chain=[_hop_to_proto(h) for h in event.chain],
    )


class HolonObservabilityService(observability_pb2_grpc.HolonObservabilityServicer):
    def __init__(self, obs: Observability):
        self._obs = obs

    def Logs(self, request, context):
        if not self._obs.enabled(Family.LOGS) or self._obs.log_ring is None:
            context.abort(grpc.StatusCode.FAILED_PRECONDITION, "logs family is not enabled (OP_OBS)")
        min_level = Level(request.min_level or observability_pb2.INFO)
        cutoff = time.time() - (request.since.seconds + request.since.nanos / 1e9) if request.HasField("since") else 0
        entries = self._obs.log_ring.drain_since(cutoff) if cutoff else self._obs.log_ring.drain()
        for entry in entries:
            if _match_log(entry, min_level, request.session_ids, request.rpc_methods):
                yield to_proto_log_entry(entry)
        if not request.follow:
            return
        q: queue.Queue[LogEntry | None] = queue.Queue(maxsize=128)
        stop = self._obs.log_ring.subscribe(lambda e: _offer(q, e))
        try:
            while context.is_active():
                try:
                    entry = q.get(timeout=0.1)
                except queue.Empty:
                    continue
                if entry is not None and _match_log(entry, min_level, request.session_ids, request.rpc_methods):
                    yield to_proto_log_entry(entry)
        finally:
            stop()

    def Metrics(self, request, context):
        if not self._obs.enabled(Family.METRICS) or self._obs.registry is None:
            context.abort(grpc.StatusCode.FAILED_PRECONDITION, "metrics family is not enabled (OP_OBS)")
        snapshot = self._obs.registry.snapshot()
        samples = to_proto_metric_samples(snapshot)
        if request.name_prefixes:
            prefixes = tuple(p for p in request.name_prefixes if p)
            samples = [s for s in samples if s.name.startswith(prefixes)]
        return observability_pb2.MetricsSnapshot(
            captured_at=_timestamp(snapshot["captured_at"]),
            slug=self._obs.cfg.slug,
            instance_uid=self._obs.cfg.instance_uid,
            samples=samples,
        )

    def Events(self, request, context):
        if not self._obs.enabled(Family.EVENTS) or self._obs.event_bus is None:
            context.abort(grpc.StatusCode.FAILED_PRECONDITION, "events family is not enabled (OP_OBS)")
        wanted = set(request.types)
        cutoff = time.time() - (request.since.seconds + request.since.nanos / 1e9) if request.HasField("since") else 0
        events = self._obs.event_bus.drain_since(cutoff) if cutoff else self._obs.event_bus.drain()
        for event in events:
            if _match_event(event, wanted):
                yield to_proto_event(event)
        if not request.follow:
            return
        q: queue.Queue[Event | None] = queue.Queue(maxsize=64)
        stop = self._obs.event_bus.subscribe(lambda e: _offer(q, e))
        try:
            while context.is_active():
                try:
                    event = q.get(timeout=0.1)
                except queue.Empty:
                    continue
                if event is not None and _match_event(event, wanted):
                    yield to_proto_event(event)
        finally:
            stop()


def register_service(server, obs: typing.Optional[Observability] = None) -> None:
    observability_pb2_grpc.add_HolonObservabilityServicer_to_server(
        HolonObservabilityService(obs or current()),
        server,
    )


def _offer(q: queue.Queue[typing.Any], value: typing.Any) -> None:
    try:
        q.put_nowait(value)
    except queue.Full:
        pass


def _match_log(entry: LogEntry, min_level: Level, session_ids, rpc_methods) -> bool:
    if entry.level < min_level:
        return False
    if session_ids and entry.session_id not in set(session_ids):
        return False
    if rpc_methods and entry.rpc_method not in set(rpc_methods):
        return False
    return True


def _match_event(event: Event, wanted: set[int]) -> bool:
    if not wanted:
        return True
    return _PROTO_EVENT_TYPE.get(event.type, observability_pb2.EVENT_TYPE_UNSPECIFIED) in wanted


# --- Disk writers + meta.json -------------------------------------------------


def enable_disk_writers(run_dir: str) -> None:
    obs = current()
    if run_dir:
        obs.cfg.run_dir = run_dir
    if obs is None or not obs.cfg.run_dir:
        return
    os.makedirs(obs.cfg.run_dir, exist_ok=True)
    if obs.enabled(Family.LOGS) and obs.log_ring is not None:
        fp = os.path.join(obs.cfg.run_dir, "stdout.log")
        obs.log_ring.subscribe(lambda e, fp=fp: _append_jsonl(fp, _log_json(e)))
    if obs.enabled(Family.EVENTS) and obs.event_bus is not None:
        fp = os.path.join(obs.cfg.run_dir, "events.jsonl")
        obs.event_bus.subscribe(lambda e, fp=fp: _append_jsonl(fp, _event_json(e)))


@dataclass
class MetaJSON:
    slug: str
    uid: str
    pid: int
    started_at: float
    mode: str = "persistent"
    transport: str = ""
    address: str = ""
    metrics_addr: str = ""
    log_path: str = ""
    log_bytes_rotated: int = 0
    organism_uid: str = ""
    organism_slug: str = ""
    is_default: bool = False


def write_meta_json(run_dir: str, meta: MetaJSON) -> None:
    if not run_dir:
        raise ValueError("write_meta_json: empty run_dir")
    os.makedirs(run_dir, exist_ok=True)
    payload: dict[str, typing.Any] = {
        "slug": meta.slug,
        "uid": meta.uid,
        "pid": meta.pid,
        "started_at": _iso8601(meta.started_at),
        "mode": meta.mode,
        "transport": meta.transport,
        "address": meta.address,
    }
    if meta.metrics_addr:
        payload["metrics_addr"] = meta.metrics_addr
    if meta.log_path:
        payload["log_path"] = meta.log_path
    if meta.log_bytes_rotated:
        payload["log_bytes_rotated"] = meta.log_bytes_rotated
    if meta.organism_uid:
        payload["organism_uid"] = meta.organism_uid
    if meta.organism_slug:
        payload["organism_slug"] = meta.organism_slug
    if meta.is_default:
        payload["default"] = True
    path = os.path.join(run_dir, "meta.json")
    tmp = path + ".tmp"
    with open(tmp, "w", encoding="utf-8") as f:
        json.dump(payload, f, indent=2)
    os.replace(tmp, path)


def read_meta_json(run_dir: str) -> dict[str, typing.Any]:
    with open(os.path.join(run_dir, "meta.json"), encoding="utf-8") as f:
        return json.load(f)


def _log_json(entry: LogEntry) -> dict[str, typing.Any]:
    rec: dict[str, typing.Any] = {
        "kind": "log",
        "ts": _iso8601(entry.timestamp),
        "level": _LEVEL_NAMES.get(entry.level, "UNSPECIFIED"),
        "slug": entry.slug,
        "instance_uid": entry.instance_uid,
        "message": entry.message,
    }
    if entry.session_id:
        rec["session_id"] = entry.session_id
    if entry.rpc_method:
        rec["rpc_method"] = entry.rpc_method
    if entry.fields:
        rec["fields"] = entry.fields
    if entry.caller:
        rec["caller"] = entry.caller
    if entry.chain:
        rec["chain"] = [h.__dict__ for h in entry.chain]
    return rec


def _event_json(event: Event) -> dict[str, typing.Any]:
    rec: dict[str, typing.Any] = {
        "kind": "event",
        "ts": _iso8601(event.timestamp),
        "type": event.type.name,
        "slug": event.slug,
        "instance_uid": event.instance_uid,
    }
    if event.session_id:
        rec["session_id"] = event.session_id
    if event.payload:
        rec["payload"] = event.payload
    if event.chain:
        rec["chain"] = [h.__dict__ for h in event.chain]
    return rec


def _append_jsonl(path: str, rec: dict[str, typing.Any]) -> None:
    try:
        with open(path, "a", encoding="utf-8") as f:
            f.write(json.dumps(rec, separators=(",", ":")) + "\n")
    except OSError:
        pass


def _iso8601(unix_seconds: float) -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%S", time.gmtime(unix_seconds)) + (
        ".%09dZ" % int((unix_seconds - int(unix_seconds)) * 1_000_000_000)
    )


__all__ = [
    "Config", "Observability", "Family", "Level", "EventType", "LogEntry", "Event",
    "Counter", "Gauge", "Histogram", "Registry", "HistogramSnapshot",
    "LogRing", "EventBus", "Hop", "Logger",
    "configure", "from_env", "current", "reset",
    "check_env", "parse_level", "append_direct_child", "enrich_for_multilog",
    "derive_run_dir", "enable_disk_writers", "write_meta_json", "read_meta_json",
    "register_service", "HolonObservabilityService", "MetaJSON",
    "DEFAULT_BUCKETS", "InvalidTokenError",
]
