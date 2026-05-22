"""Python reference implementation of the cross-SDK observability layer.

Mirrors sdk/go-holons/pkg/observability: OP_OBS controls activation,
logs and events are OTLP-shaped LogRecord values, and metrics are
OTLP-shaped Metric oneofs.
"""

from __future__ import annotations

import contextlib
import contextvars
import enum
import inspect
import json
import math
import os
import queue
import socketserver
import sys
import threading
import time
import typing
import uuid
from collections import deque
from dataclasses import dataclass, field
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import grpc

from holons.v1 import observability_pb2, observability_pb2_grpc


# --- OP_OBS parsing -----------------------------------------------------------


class Family(str, enum.Enum):
    LOGS = "logs"
    METRICS = "metrics"
    EVENTS = "events"
    PROM = "prom"


_V1_TOKENS = {"logs", "metrics", "events", "prom", "all"}


class InvalidTokenError(ValueError):
    """Raised when OP_OBS contains an unknown token."""


def _parse_op_obs(raw: str) -> set[Family]:
    out: set[Family] = set()
    if not raw or not raw.strip():
        return out
    for tok in (t.strip() for t in raw.split(",")):
        if not tok:
            continue
        if tok not in _V1_TOKENS:
            raise InvalidTokenError(f"unknown OP_OBS token: {tok}")
        if tok == "all":
            out.update({Family.LOGS, Family.METRICS, Family.EVENTS, Family.PROM})
        else:
            out.add(Family(tok))
    return out


def check_env() -> None:
    """Raise InvalidTokenError if OP_OBS contains an unknown token."""
    raw = os.environ.get("OP_OBS", "").strip()
    if not raw:
        return
    for tok in (t.strip() for t in raw.split(",")):
        if not tok:
            continue
        if tok not in _V1_TOKENS:
            raise InvalidTokenError(f"unknown OP_OBS token: {tok}")


# --- Levels, attributes, and event names -------------------------------------


class Level(enum.IntEnum):
    UNSET = 0
    TRACE = 1
    DEBUG = 5
    INFO = 9
    WARN = 13
    ERROR = 17
    FATAL = 21


_LEVEL_NAMES = {
    Level.TRACE: "TRACE",
    Level.DEBUG: "DEBUG",
    Level.INFO: "INFO",
    Level.WARN: "WARN",
    Level.ERROR: "ERROR",
    Level.FATAL: "FATAL",
}


_SEVERITY_TO_LEVEL = {
    observability_pb2.SEVERITY_NUMBER_TRACE: Level.TRACE,
    observability_pb2.SEVERITY_NUMBER_DEBUG: Level.DEBUG,
    observability_pb2.SEVERITY_NUMBER_INFO: Level.INFO,
    observability_pb2.SEVERITY_NUMBER_WARN: Level.WARN,
    observability_pb2.SEVERITY_NUMBER_ERROR: Level.ERROR,
    observability_pb2.SEVERITY_NUMBER_FATAL: Level.FATAL,
}


ATTR_HOLONS_SLUG = "holons.slug"
ATTR_HOLONS_INSTANCE_UID = "holons.instance_uid"
ATTR_HOLONS_SESSION_ID = "holons.session_id"
ATTR_HOLONS_TRANSPORT = "holons.transport"
ATTR_SERVICE_NAME = "service.name"
ATTR_SERVICE_INSTANCE_ID = "service.instance.id"
ATTR_RPC_METHOD = "rpc.method"
ATTR_LOGGER_NAME = "logger.name"
ATTR_CODE_CALLER = "code.caller"


_SYSTEM_ATTRIBUTES = {
    ATTR_HOLONS_SLUG,
    ATTR_HOLONS_INSTANCE_UID,
    ATTR_HOLONS_SESSION_ID,
    ATTR_HOLONS_TRANSPORT,
    ATTR_SERVICE_NAME,
    ATTR_SERVICE_INSTANCE_ID,
    ATTR_RPC_METHOD,
    ATTR_LOGGER_NAME,
    ATTR_CODE_CALLER,
}


EVENT_INSTANCE_SPAWNED = "instance.spawned"
EVENT_INSTANCE_READY = "instance.ready"
EVENT_INSTANCE_EXITED = "instance.exited"
EVENT_INSTANCE_CRASHED = "instance.crashed"
EVENT_SESSION_STARTED = "session.started"
EVENT_SESSION_ENDED = "session.ended"
EVENT_HANDLER_PANIC = "handler.panic"
EVENT_CONFIG_RELOADED = "config.reloaded"


CANONICAL_EVENT_NAMES = frozenset(
    {
        EVENT_INSTANCE_SPAWNED,
        EVENT_INSTANCE_READY,
        EVENT_INSTANCE_EXITED,
        EVENT_INSTANCE_CRASHED,
        EVENT_SESSION_STARTED,
        EVENT_SESSION_ENDED,
        EVENT_HANDLER_PANIC,
        EVENT_CONFIG_RELOADED,
    }
)


def parse_level(s: str) -> Level:
    if not s:
        return Level.INFO
    u = s.strip().upper()
    return {
        "TRACE": Level.TRACE,
        "DEBUG": Level.DEBUG,
        "INFO": Level.INFO,
        "WARN": Level.WARN,
        "WARNING": Level.WARN,
        "ERROR": Level.ERROR,
        "FATAL": Level.FATAL,
    }.get(u, Level.INFO)


@dataclass(frozen=True)
class ContextValues:
    session_id: str = ""
    rpc_method: str = ""


_context_values: contextvars.ContextVar[ContextValues] = contextvars.ContextVar(
    "holons_observability_context",
    default=ContextValues(),
)


def current_context() -> ContextValues:
    return _context_values.get()


@contextlib.contextmanager
def session_context(
    session_id: str = "",
    rpc_method: str = "",
) -> typing.Iterator[None]:
    token = _context_values.set(ContextValues(session_id=session_id, rpc_method=rpc_method))
    try:
        yield
    finally:
        _context_values.reset(token)


class _PrivateMarker:
    pass


def Private() -> _PrivateMarker:
    """Mark a single log or event emission as local-only."""
    return _PrivateMarker()


def _is_private_marker(value: typing.Any) -> bool:
    return isinstance(value, _PrivateMarker)


def to_any_value(value: typing.Any) -> observability_pb2.AnyValue:
    if isinstance(value, bool):
        return observability_pb2.AnyValue(bool_value=value)
    if isinstance(value, int):
        return observability_pb2.AnyValue(int_value=value)
    if isinstance(value, float):
        return observability_pb2.AnyValue(double_value=value)
    if isinstance(value, str):
        return observability_pb2.AnyValue(string_value=value)
    return observability_pb2.AnyValue(string_value=str(value))


def any_value_to_python(value: observability_pb2.AnyValue) -> typing.Any:
    which = value.WhichOneof("value")
    if which == "bool_value":
        return value.bool_value
    if which == "int_value":
        return value.int_value
    if which == "double_value":
        return value.double_value
    if which == "string_value":
        return value.string_value
    return ""


def any_value_string(value: observability_pb2.AnyValue | None) -> str:
    if value is None:
        return ""
    which = value.WhichOneof("value")
    if which == "bool_value":
        return "true" if value.bool_value else "false"
    if which == "int_value":
        return str(value.int_value)
    if which == "double_value":
        return f"{value.double_value:g}"
    if which == "string_value":
        return value.string_value
    return ""


def key_value(key: str, value: typing.Any) -> observability_pb2.KeyValue:
    return observability_pb2.KeyValue(key=key, value=to_any_value(value))


def resource_attributes(slug: str, instance_uid: str) -> list[observability_pb2.KeyValue]:
    attrs: list[observability_pb2.KeyValue] = []
    if slug:
        attrs.append(key_value(ATTR_HOLONS_SLUG, slug))
        attrs.append(key_value(ATTR_SERVICE_NAME, slug))
    if instance_uid:
        attrs.append(key_value(ATTR_HOLONS_INSTANCE_UID, instance_uid))
        attrs.append(key_value(ATTR_SERVICE_INSTANCE_ID, instance_uid))
    return attrs


def sorted_map_attributes(
    values: typing.Mapping[str, typing.Any] | None,
) -> list[observability_pb2.KeyValue]:
    if not values:
        return []
    return [key_value(k, values[k]) for k in sorted(values)]


def string_attribute(
    attrs: typing.Iterable[observability_pb2.KeyValue],
    key: str,
) -> str:
    for attr in attrs:
        if attr.key == key:
            return any_value_string(attr.value)
    return ""


def attribute_value(
    attrs: typing.Iterable[observability_pb2.KeyValue],
    key: str,
) -> typing.Any:
    for attr in attrs:
        if attr.key == key:
            return any_value_to_python(attr.value)
    return None


def attributes_map(
    attrs: typing.Iterable[observability_pb2.KeyValue],
    *,
    include_system: bool = False,
) -> dict[str, typing.Any]:
    out: dict[str, typing.Any] = {}
    for attr in attrs:
        if not include_system and attr.key in _SYSTEM_ATTRIBUTES:
            continue
        out[attr.key] = any_value_to_python(attr.value)
    return out


def severity_label(number: int) -> str:
    level = _SEVERITY_TO_LEVEL.get(number, Level.UNSET)
    return _LEVEL_NAMES.get(level, "UNSPECIFIED")


@dataclass
class LogRecord:
    record: observability_pb2.LogRecord = field(default_factory=observability_pb2.LogRecord)
    private: bool = False

    def timestamp(self) -> float:
        if self.record.time_unix_nano == 0:
            return 0.0
        return self.record.time_unix_nano / 1_000_000_000


def clone_log_record(record: observability_pb2.LogRecord | None) -> observability_pb2.LogRecord:
    out = observability_pb2.LogRecord()
    if record is not None:
        out.CopyFrom(record)
    return out


def body_string(record: observability_pb2.LogRecord | LogRecord | None) -> str:
    if record is None:
        return ""
    proto = record.record if isinstance(record, LogRecord) else record
    return any_value_string(proto.body)


# --- Chain helpers ------------------------------------------------------------


def append_direct_child(src: typing.Iterable[str], child_slug: str) -> list[str]:
    """SDK rule on relay: append the direct child whose stream was just read."""
    out = list(src)
    if child_slug:
        out.append(child_slug)
    return out


def enrich_for_multilog(wire: typing.Iterable[str], stream_source_slug: str) -> list[str]:
    """Root-side rule: append the stream source before writing a multilog entry."""
    return append_direct_child(wire, stream_source_slug)


def _replace_chain(record: observability_pb2.LogRecord, chain: typing.Iterable[str]) -> None:
    record.ClearField("chain")
    record.chain.extend(chain)


# --- Ring buffer --------------------------------------------------------------


class LogRing:
    def __init__(self, capacity: int = 1024) -> None:
        self._capacity = max(1, capacity)
        self._buf: deque[LogRecord] = deque(maxlen=self._capacity)
        self._lock = threading.Lock()
        self._subs: list[typing.Callable[[LogRecord], None]] = []

    def push(self, entry: LogRecord) -> None:
        with self._lock:
            self._buf.append(entry)
            subs = list(self._subs)
        for sub in subs:
            try:
                sub(entry)
            except Exception:
                pass

    def drain(self) -> list[LogRecord]:
        with self._lock:
            return list(self._buf)

    def drain_since(self, cutoff: float) -> list[LogRecord]:
        with self._lock:
            return [e for e in self._buf if e.timestamp() >= cutoff]

    def subscribe(self, fn: typing.Callable[[LogRecord], None]) -> typing.Callable[[], None]:
        with self._lock:
            self._subs.append(fn)

        def unsub() -> None:
            with self._lock:
                try:
                    self._subs.remove(fn)
                except ValueError:
                    pass

        return unsub

    def replay_and_subscribe(
        self,
        fn: typing.Callable[[LogRecord], None],
        cutoff: float = 0.0,
    ) -> tuple[list[LogRecord], typing.Callable[[], None]]:
        with self._lock:
            replay = [e for e in self._buf if not cutoff or e.timestamp() >= cutoff]
            self._subs.append(fn)

        def unsub() -> None:
            with self._lock:
                try:
                    self._subs.remove(fn)
                except ValueError:
                    pass

        return list(replay), unsub

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
    min: float = 0.0
    max: float = 0.0

    def quantile(self, q: float) -> float:
        if self.total == 0:
            return float("nan")
        target = self.total * q
        for i, count in enumerate(self.counts):
            if count >= target:
                return self.bounds[i]
        return math.inf


DEFAULT_BUCKETS = [
    50e-6,
    100e-6,
    250e-6,
    500e-6,
    1e-3,
    2.5e-3,
    5e-3,
    10e-3,
    25e-3,
    50e-3,
    100e-3,
    250e-3,
    500e-3,
    1.0,
    2.5,
    5.0,
    10.0,
    30.0,
    60.0,
]


class Counter:
    def __init__(
        self,
        name: str,
        help_: str = "",
        labels: typing.Optional[dict[str, str]] = None,
    ) -> None:
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
    def __init__(
        self,
        name: str,
        help_: str = "",
        labels: typing.Optional[dict[str, str]] = None,
    ) -> None:
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
        self._min = 0.0
        self._max = 0.0
        self._lock = threading.Lock()

    def observe(self, v: float) -> None:
        value = float(v)
        with self._lock:
            self._total += 1
            self._sum += value
            if self._total == 1:
                self._min = value
                self._max = value
            else:
                self._min = min(self._min, value)
                self._max = max(self._max, value)
            for i, bound in enumerate(self._bounds):
                if value <= bound:
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
                min=self._min,
                max=self._max,
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

    def counter(
        self,
        name: str,
        help_: str = "",
        labels: typing.Optional[dict[str, str]] = None,
    ) -> Counter:
        labels = labels or {}
        key = _metric_key(name, labels)
        with self._lock:
            counter = self._counters.get(key)
            if counter is None:
                counter = Counter(name, help_, labels)
                self._counters[key] = counter
            return counter

    def gauge(
        self,
        name: str,
        help_: str = "",
        labels: typing.Optional[dict[str, str]] = None,
    ) -> Gauge:
        labels = labels or {}
        key = _metric_key(name, labels)
        with self._lock:
            gauge = self._gauges.get(key)
            if gauge is None:
                gauge = Gauge(name, help_, labels)
                self._gauges[key] = gauge
            return gauge

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
            hist = self._histograms.get(key)
            if hist is None:
                hist = Histogram(name, help_, labels, bounds)
                self._histograms[key] = hist
            return hist

    def snapshot(self) -> dict[str, typing.Any]:
        with self._lock:
            counters = [
                (c.name, c.help, dict(c.labels), c.value())
                for c in self._counters.values()
            ]
            gauges = [
                (g.name, g.help, dict(g.labels), g.value())
                for g in self._gauges.values()
            ]
            histograms = [
                (h.name, h.help, dict(h.labels), h.snapshot())
                for h in self._histograms.values()
            ]
        counters.sort(key=lambda x: x[0])
        gauges.sort(key=lambda x: x[0])
        histograms.sort(key=lambda x: x[0])
        return {
            "captured_at": time.time(),
            "counters": counters,
            "gauges": gauges,
            "histograms": histograms,
        }


# --- Events -------------------------------------------------------------------


class EventBus:
    def __init__(self, capacity: int = 256) -> None:
        self._capacity = max(1, capacity)
        self._buf: deque[LogRecord] = deque(maxlen=self._capacity)
        self._lock = threading.Lock()
        self._subs: list[typing.Callable[[LogRecord], None]] = []
        self._closed = False

    def emit(self, entry: LogRecord) -> None:
        with self._lock:
            if self._closed:
                return
            self._buf.append(entry)
            subs = list(self._subs)
        for sub in subs:
            try:
                sub(entry)
            except Exception:
                pass

    def drain(self) -> list[LogRecord]:
        with self._lock:
            return list(self._buf)

    def drain_since(self, cutoff: float) -> list[LogRecord]:
        with self._lock:
            return [e for e in self._buf if e.timestamp() >= cutoff]

    def subscribe(self, fn: typing.Callable[[LogRecord], None]) -> typing.Callable[[], None]:
        with self._lock:
            self._subs.append(fn)

        def unsub() -> None:
            with self._lock:
                try:
                    self._subs.remove(fn)
                except ValueError:
                    pass

        return unsub

    def replay_and_subscribe(
        self,
        fn: typing.Callable[[LogRecord], None],
        cutoff: float = 0.0,
    ) -> tuple[list[LogRecord], typing.Callable[[], None]]:
        with self._lock:
            if self._closed:
                return [], lambda: None
            replay = [e for e in self._buf if not cutoff or e.timestamp() >= cutoff]
            self._subs.append(fn)

        def unsub() -> None:
            with self._lock:
                try:
                    self._subs.remove(fn)
                except ValueError:
                    pass

        return list(replay), unsub

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
    session_id: str = ""
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
        attrs = resource_attributes(self._obs.cfg.slug, self._obs.cfg.instance_uid)
        if self._name:
            attrs.append(key_value(ATTR_LOGGER_NAME, self._name))
        redact = set(self._obs.cfg.redacted_fields)
        private = False
        for key, value in kv.items():
            if not key:
                continue
            if key == "private":
                private = _is_private_marker(value) or bool(value)
                continue
            if _is_private_marker(value):
                private = True
                continue
            attrs.append(key_value(key, "<redacted>" if key in redact else value))

        context = current_context()
        session_id = context.session_id or self._obs.cfg.session_id
        if session_id:
            attrs.append(key_value(ATTR_HOLONS_SESSION_ID, session_id))
        if context.rpc_method:
            attrs.append(key_value(ATTR_RPC_METHOD, context.rpc_method))
        if caller := _caller_frame(3):
            attrs.append(key_value(ATTR_CODE_CALLER, caller))

        now = time.time_ns()
        record = observability_pb2.LogRecord(
            time_unix_nano=now,
            observed_time_unix_nano=now,
            severity_number=int(level),
            severity_text=_LEVEL_NAMES.get(level, "UNSPECIFIED"),
            body=to_any_value(message),
            attributes=attrs,
        )
        if self._obs.log_ring is not None:
            self._obs.log_ring.push(LogRecord(record=record, private=private))

    def trace(self, msg: str, **kv: typing.Any) -> None:
        self._log(Level.TRACE, msg, **kv)

    def debug(self, msg: str, **kv: typing.Any) -> None:
        self._log(Level.DEBUG, msg, **kv)

    def info(self, msg: str, **kv: typing.Any) -> None:
        self._log(Level.INFO, msg, **kv)

    def warn(self, msg: str, **kv: typing.Any) -> None:
        self._log(Level.WARN, msg, **kv)

    def error(self, msg: str, **kv: typing.Any) -> None:
        self._log(Level.ERROR, msg, **kv)

    def fatal(self, msg: str, **kv: typing.Any) -> None:
        self._log(Level.FATAL, msg, **kv)


def _caller_frame(skip: int) -> str:
    frame = inspect.currentframe()
    try:
        current = frame
        for _ in range(skip):
            if current is None:
                return ""
            current = current.f_back
        if current is None:
            return ""
        return f"{os.path.basename(current.f_code.co_filename)}:{current.f_lineno}"
    finally:
        del frame


class Observability:
    def __init__(self, cfg: Config, families: set[Family]) -> None:
        self.cfg = cfg
        self.families = families
        self.log_ring: typing.Optional[LogRing] = (
            LogRing(cfg.logs_ring_size) if Family.LOGS in families else None
        )
        self.registry: typing.Optional[Registry] = (
            Registry() if Family.METRICS in families else None
        )
        self.event_bus: typing.Optional[EventBus] = (
            EventBus(cfg.events_ring_size) if Family.EVENTS in families else None
        )
        self.start_wall = time.time()
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
            logger = self._loggers.get(name)
            if logger is None:
                logger = Logger(self, name)
                self._loggers[name] = logger
            return logger

    def counter(
        self,
        name: str,
        help_: str = "",
        labels: typing.Optional[dict[str, str]] = None,
    ) -> typing.Optional[Counter]:
        return self.registry.counter(name, help_, labels) if self.registry else None

    def gauge(
        self,
        name: str,
        help_: str = "",
        labels: typing.Optional[dict[str, str]] = None,
    ) -> typing.Optional[Gauge]:
        return self.registry.gauge(name, help_, labels) if self.registry else None

    def histogram(
        self,
        name: str,
        help_: str = "",
        labels: typing.Optional[dict[str, str]] = None,
        bounds: typing.Optional[list[float]] = None,
    ) -> typing.Optional[Histogram]:
        return self.registry.histogram(name, help_, labels, bounds) if self.registry else None

    def emit(
        self,
        event_name: str,
        payload: typing.Optional[dict[str, typing.Any]] = None,
        *,
        private: typing.Union[bool, _PrivateMarker] = False,
    ) -> None:
        if self.event_bus is None:
            return
        if event_name not in CANONICAL_EVENT_NAMES:
            raise ValueError(f"unknown canonical event_name: {event_name}")
        redact = set(self.cfg.redacted_fields)
        attrs = resource_attributes(self.cfg.slug, self.cfg.instance_uid)
        context = current_context()
        session_id = context.session_id or self.cfg.session_id
        if session_id:
            attrs.append(key_value(ATTR_HOLONS_SESSION_ID, session_id))
        payload = payload or {}
        for key in sorted(payload):
            attrs.append(key_value(key, "<redacted>" if key in redact else payload[key]))

        now = time.time_ns()
        record = observability_pb2.LogRecord(
            time_unix_nano=now,
            observed_time_unix_nano=now,
            severity_number=observability_pb2.SEVERITY_NUMBER_INFO,
            severity_text="INFO",
            body=to_any_value(event_name),
            attributes=attrs,
            event_name=event_name,
        )
        self.event_bus.emit(
            LogRecord(record=record, private=_is_private_marker(private) or bool(private))
        )

    def close(self) -> None:
        if self.event_bus is not None:
            self.event_bus.close()


class _NoopObs:
    cfg = Config()


_DISABLED_OBS = _NoopObs()
_DISABLED_LOGGER = Logger.__new__(Logger)
_DISABLED_LOGGER._obs = None  # type: ignore[attr-defined]
_DISABLED_LOGGER._name = ""
_DISABLED_LOGGER._level = Level.FATAL


_current_lock = threading.Lock()
_current: typing.Optional[Observability] = None


def configure(cfg: Config) -> Observability:
    check_env()
    families = _parse_op_obs(os.environ.get("OP_OBS", ""))
    if cfg.default_log_level == Level.UNSET:
        cfg.default_log_level = Level.INFO
    if not cfg.slug:
        cfg.slug = os.path.basename(sys.argv[0]) if sys.argv else ""
    if not cfg.instance_uid:
        cfg.instance_uid = str(uuid.uuid4())
    if not cfg.session_id:
        cfg.session_id = str(uuid.uuid4())
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


def to_proto_log_record(entry: LogRecord) -> observability_pb2.LogRecord:
    return clone_log_record(entry.record)


def from_proto_log_record(record: observability_pb2.LogRecord) -> LogRecord:
    return LogRecord(record=clone_log_record(record))


def _histogram_bucket_counts(snapshot: HistogramSnapshot) -> list[int]:
    counts: list[int] = []
    previous = 0
    for count in snapshot.counts:
        delta = count - previous
        counts.append(max(0, delta))
        previous = count
    counts.append(max(0, snapshot.total - previous))
    return counts


def to_proto_metrics(
    snapshot: dict[str, typing.Any],
    slug: str,
    instance_uid: str,
    start_wall: float,
) -> list[observability_pb2.Metric]:
    metrics: list[observability_pb2.Metric] = []
    start_nano = _unix_nano(start_wall)
    time_nano = _unix_nano(snapshot["captured_at"])
    temporality = observability_pb2.AGGREGATION_TEMPORALITY_CUMULATIVE

    for name, help_, labels, value in snapshot.get("counters", []):
        attrs = resource_attributes(slug, instance_uid) + sorted_map_attributes(labels)
        metrics.append(
            observability_pb2.Metric(
                name=name,
                description=help_,
                sum=observability_pb2.Sum(
                    aggregation_temporality=temporality,
                    is_monotonic=True,
                    data_points=[
                        observability_pb2.NumberDataPoint(
                            start_time_unix_nano=start_nano,
                            time_unix_nano=time_nano,
                            as_int=int(value),
                            attributes=attrs,
                        )
                    ],
                ),
            )
        )

    for name, help_, labels, value in snapshot.get("gauges", []):
        attrs = resource_attributes(slug, instance_uid) + sorted_map_attributes(labels)
        metrics.append(
            observability_pb2.Metric(
                name=name,
                description=help_,
                gauge=observability_pb2.Gauge(
                    data_points=[
                        observability_pb2.NumberDataPoint(
                            start_time_unix_nano=start_nano,
                            time_unix_nano=time_nano,
                            as_double=float(value),
                            attributes=attrs,
                        )
                    ],
                ),
            )
        )

    for name, help_, labels, hist in snapshot.get("histograms", []):
        attrs = resource_attributes(slug, instance_uid) + sorted_map_attributes(labels)
        metrics.append(
            observability_pb2.Metric(
                name=name,
                description=help_,
                histogram=observability_pb2.Histogram(
                    aggregation_temporality=temporality,
                    data_points=[
                        observability_pb2.HistogramDataPoint(
                            start_time_unix_nano=start_nano,
                            time_unix_nano=time_nano,
                            count=hist.total,
                            sum=hist.sum,
                            bucket_counts=_histogram_bucket_counts(hist),
                            explicit_bounds=list(hist.bounds),
                            attributes=attrs,
                            min=hist.min,
                            max=hist.max,
                        )
                    ],
                ),
            )
        )
    return metrics


class HolonObservabilityService(observability_pb2_grpc.HolonObservabilityServicer):
    def __init__(self, obs: Observability):
        self._obs = obs

    def Logs(self, request, context):
        if not self._obs.enabled(Family.LOGS) or self._obs.log_ring is None:
            context.abort(
                grpc.StatusCode.FAILED_PRECONDITION,
                "logs family is not enabled (OP_OBS)",
            )
        min_level = Level(
            request.min_severity_number or observability_pb2.SEVERITY_NUMBER_INFO
        )
        cutoff = _cutoff_from_since(request) if request.HasField("since") else 0.0
        q: queue.Queue[LogRecord | None] | None = None
        stop: typing.Callable[[], None] | None = None
        if request.follow:
            q = queue.Queue(maxsize=128)
            entries, stop = self._obs.log_ring.replay_and_subscribe(
                lambda e: _offer(q, e),
                cutoff,
            )
        else:
            entries = self._obs.log_ring.drain_since(cutoff) if cutoff else self._obs.log_ring.drain()
        for entry in entries:
            if entry.private:
                continue
            if _match_log(entry, min_level, request.session_ids, request.rpc_methods):
                yield to_proto_log_record(entry)
        if not request.follow:
            return
        assert q is not None and stop is not None
        try:
            while context.is_active():
                try:
                    entry = q.get(timeout=0.1)
                except queue.Empty:
                    continue
                if entry is not None and not entry.private:
                    if _match_log(entry, min_level, request.session_ids, request.rpc_methods):
                        yield to_proto_log_record(entry)
        finally:
            stop()

    def Metrics(self, request, context):
        if not self._obs.enabled(Family.METRICS) or self._obs.registry is None:
            context.abort(
                grpc.StatusCode.FAILED_PRECONDITION,
                "metrics family is not enabled (OP_OBS)",
            )
        snapshot = self._obs.registry.snapshot()
        metrics = to_proto_metrics(
            snapshot,
            self._obs.cfg.slug,
            self._obs.cfg.instance_uid,
            self._obs.start_wall,
        )
        prefixes = tuple(prefix for prefix in request.name_prefixes if prefix)
        for metric in metrics:
            if prefixes and not metric.name.startswith(prefixes):
                continue
            yield metric

    def Events(self, request, context):
        if not self._obs.enabled(Family.EVENTS) or self._obs.event_bus is None:
            context.abort(
                grpc.StatusCode.FAILED_PRECONDITION,
                "events family is not enabled (OP_OBS)",
            )
        wanted = set(request.event_names)
        cutoff = _cutoff_from_since(request) if request.HasField("since") else 0.0
        q: queue.Queue[LogRecord | None] | None = None
        stop: typing.Callable[[], None] | None = None
        if request.follow:
            q = queue.Queue(maxsize=64)
            events, stop = self._obs.event_bus.replay_and_subscribe(
                lambda e: _offer(q, e),
                cutoff,
            )
        else:
            events = self._obs.event_bus.drain_since(cutoff) if cutoff else self._obs.event_bus.drain()
        for event in events:
            if event.private:
                continue
            if _match_event(event, wanted):
                yield to_proto_log_record(event)
        if not request.follow:
            return
        assert q is not None and stop is not None
        try:
            while context.is_active():
                try:
                    event = q.get(timeout=0.1)
                except queue.Empty:
                    continue
                if event is not None and not event.private and _match_event(event, wanted):
                    yield to_proto_log_record(event)
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


def _match_log(
    entry: LogRecord,
    min_level: Level,
    session_ids: typing.Iterable[str],
    rpc_methods: typing.Iterable[str],
) -> bool:
    record = entry.record
    if Level(record.severity_number) < min_level:
        return False
    session_filter = set(session_ids)
    if session_filter and string_attribute(record.attributes, ATTR_HOLONS_SESSION_ID) not in session_filter:
        return False
    method_filter = set(rpc_methods)
    if method_filter and string_attribute(record.attributes, ATTR_RPC_METHOD) not in method_filter:
        return False
    return True


def _match_event(entry: LogRecord, wanted: set[str]) -> bool:
    if not wanted:
        return True
    return entry.record.event_name in wanted


def _cutoff_from_since(request: typing.Any) -> float:
    return time.time() - (request.since.seconds + request.since.nanos / 1e9)


def _unix_nano(unix_seconds: float) -> int:
    return int(unix_seconds * 1_000_000_000)


# --- Prometheus exposition ----------------------------------------------------


class _FastThreadingHTTPServer(ThreadingHTTPServer):
    def server_bind(self) -> None:
        socketserver.TCPServer.server_bind(self)
        host, port = self.server_address[:2]
        self.server_name = str(host)
        self.server_port = int(port)


class PromServer:
    def __init__(self, addr: str = ":0") -> None:
        self.addr = addr or ":0"
        self._server: ThreadingHTTPServer | None = None
        self._thread: threading.Thread | None = None
        self._lock = threading.Lock()

    def start(self) -> str:
        with self._lock:
            if self._server is not None:
                return self._addr_url_unlocked()
            host, port = _parse_prom_addr(self.addr)
            server = _FastThreadingHTTPServer((host, port), _PromHandler)
            server.daemon_threads = True
            self._server = server
            self._thread = threading.Thread(target=server.serve_forever, daemon=True)
            self._thread.start()
            return self._addr_url_unlocked()

    def addr_url(self) -> str:
        with self._lock:
            return self._addr_url_unlocked()

    def _addr_url_unlocked(self) -> str:
        if self._server is None:
            return ""
        host, port = self._server.server_address[:2]
        return f"http://{_advertised_prom_host(str(host))}:{port}/metrics"

    def close(self) -> None:
        with self._lock:
            server = self._server
            thread = self._thread
            self._server = None
            self._thread = None
        if server is None:
            return
        server.shutdown()
        server.server_close()
        if thread is not None:
            thread.join(timeout=1.0)


class _PromHandler(BaseHTTPRequestHandler):
    def do_GET(self) -> None:  # noqa: N802 - stdlib callback name
        if self.path.split("?", 1)[0] != "/metrics":
            self.send_response(HTTPStatus.NOT_FOUND)
            self.end_headers()
            return

        obs = current()
        status = HTTPStatus.OK
        if not obs.enabled(Family.METRICS):
            body = "# metrics family disabled (OP_OBS)\n"
            status = HTTPStatus.SERVICE_UNAVAILABLE
        elif not obs.enabled(Family.PROM):
            body = "# prom family disabled (OP_OBS)\n"
            status = HTTPStatus.SERVICE_UNAVAILABLE
        else:
            body = to_prometheus_text(obs)

        data = body.encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "text/plain; version=0.0.4")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def log_message(self, _format: str, *_args: typing.Any) -> None:
        return


def to_prometheus_text(obs: Observability) -> str:
    if not obs.enabled(Family.METRICS) or obs.registry is None:
        return "# metrics family disabled (OP_OBS)\n"
    snapshot = obs.registry.snapshot()
    groups: dict[str, dict[str, typing.Any]] = {}

    def ensure(name: str, help_: str, kind: str) -> dict[str, typing.Any]:
        group = groups.get(name)
        if group is None:
            group = {
                "name": name,
                "help": help_,
                "type": kind,
                "counters": [],
                "gauges": [],
                "histograms": [],
            }
            groups[name] = group
        if not group["help"]:
            group["help"] = help_
        return group

    for counter in snapshot.get("counters", []):
        ensure(counter[0], counter[1], "counter")["counters"].append(counter)
    for gauge in snapshot.get("gauges", []):
        ensure(gauge[0], gauge[1], "gauge")["gauges"].append(gauge)
    for histogram in snapshot.get("histograms", []):
        ensure(histogram[0], histogram[1], "histogram")["histograms"].append(histogram)

    injected = {"slug": obs.cfg.slug}
    if obs.cfg.instance_uid:
        injected["instance_uid"] = obs.cfg.instance_uid

    lines: list[str] = []
    for name in sorted(groups):
        group = groups[name]
        lines.append(f"# HELP {name} {_prom_escape_help(group['help'])}")
        lines.append(f"# TYPE {name} {group['type']}")
        for metric_name, _help, labels, value in group["counters"]:
            lines.append(f"{metric_name}{_prom_labels(_merge_labels(labels, injected))} {value}")
        for metric_name, _help, labels, value in group["gauges"]:
            lines.append(f"{metric_name}{_prom_labels(_merge_labels(labels, injected))} {_format_float(value)}")
        for metric_name, _help, labels, hist in group["histograms"]:
            merged = _merge_labels(labels, injected)
            for upper, count in zip(hist.bounds, hist.counts):
                bucket_labels = dict(merged)
                bucket_labels["le"] = _format_float(upper)
                lines.append(f"{metric_name}_bucket{_prom_labels(bucket_labels)} {count}")
            bucket_labels = dict(merged)
            bucket_labels["le"] = "+Inf"
            lines.append(f"{metric_name}_bucket{_prom_labels(bucket_labels)} {hist.total}")
            lines.append(f"{metric_name}_sum{_prom_labels(merged)} {_format_float(hist.sum)}")
            lines.append(f"{metric_name}_count{_prom_labels(merged)} {hist.total}")
    return "\n".join(lines) + ("\n" if lines else "")


def _parse_prom_addr(raw: str) -> tuple[str, int]:
    trimmed = (raw or ":0").strip() or ":0"
    if trimmed.startswith(":"):
        return "0.0.0.0", int(trimmed[1:] or "0")
    host, sep, port = trimmed.rpartition(":")
    if not sep or not port:
        raise ValueError(f'invalid Prometheus address "{raw}"')
    return host or "0.0.0.0", int(port)


def _advertised_prom_host(host: str) -> str:
    if host in ("", "0.0.0.0"):
        return "127.0.0.1"
    if host == "::":
        return "::1"
    return host


def _merge_labels(base: dict[str, str], extra: dict[str, str]) -> dict[str, str]:
    out = {k: v for k, v in extra.items() if v}
    out.update(base)
    return out


def _prom_labels(labels: dict[str, str]) -> str:
    if not labels:
        return ""
    parts = [f'{key}="{_prom_escape_value(labels[key])}"' for key in sorted(labels)]
    return "{" + ",".join(parts) + "}"


def _prom_escape_value(value: str) -> str:
    return value.replace("\\", "\\\\").replace("\n", "\\n").replace('"', '\\"')


def _prom_escape_help(value: str) -> str:
    return value.replace("\\", "\\\\").replace("\n", "\\n")


def _format_float(value: float) -> str:
    if math.isinf(value):
        return "+Inf" if value > 0 else "-Inf"
    if math.isnan(value):
        return "NaN"
    return f"{value:g}"


# --- Member observability relay ----------------------------------------------


class MemberRelay:
    def __init__(
        self,
        child_slug: str,
        child_uid: str,
        channel: grpc.Channel,
        observability: Observability | None = None,
        retry_delay: float = 2.0,
    ) -> None:
        self.child_slug = child_slug
        self.child_uid = child_uid
        self.channel = channel
        self.observability = observability or current()
        self.retry_delay = retry_delay
        self._stop = threading.Event()
        self._threads: list[threading.Thread] = []

    def start(self) -> None:
        obs = self.observability
        if not obs.enabled(Family.LOGS) and not obs.enabled(Family.EVENTS):
            return
        client = observability_pb2_grpc.HolonObservabilityStub(self.channel)
        if obs.enabled(Family.LOGS) and obs.log_ring is not None:
            thread = threading.Thread(target=self._pump_logs, args=(client,), daemon=True)
            self._threads.append(thread)
            thread.start()
        if obs.enabled(Family.EVENTS) and obs.event_bus is not None:
            thread = threading.Thread(target=self._pump_events, args=(client,), daemon=True)
            self._threads.append(thread)
            thread.start()

    def stop(self) -> None:
        self._stop.set()
        for thread in self._threads:
            thread.join(timeout=2.0)

    def _pump_logs(self, client: observability_pb2_grpc.HolonObservabilityStub) -> None:
        while not self._stop.is_set():
            try:
                stream = client.Logs(observability_pb2.LogsRequest(follow=True))
                for proto in stream:
                    if self._stop.is_set():
                        return
                    obs = self.observability
                    if not obs.enabled(Family.LOGS) or obs.log_ring is None:
                        continue
                    entry = from_proto_log_record(proto)
                    _replace_chain(
                        entry.record,
                        append_direct_child(entry.record.chain, self.child_slug),
                    )
                    obs.log_ring.push(entry)
            except Exception:
                if self._stop.wait(self.retry_delay):
                    return

    def _pump_events(self, client: observability_pb2_grpc.HolonObservabilityStub) -> None:
        while not self._stop.is_set():
            try:
                stream = client.Events(observability_pb2.EventsRequest(follow=True))
                for proto in stream:
                    if self._stop.is_set():
                        return
                    obs = self.observability
                    if not obs.enabled(Family.EVENTS) or obs.event_bus is None:
                        continue
                    event = from_proto_log_record(proto)
                    _replace_chain(
                        event.record,
                        append_direct_child(event.record.chain, self.child_slug),
                    )
                    obs.event_bus.emit(event)
            except Exception:
                if self._stop.wait(self.retry_delay):
                    return


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


def _log_json(entry: LogRecord) -> dict[str, typing.Any]:
    record = entry.record
    rec: dict[str, typing.Any] = {
        "kind": "log",
        "ts": _iso8601(entry.timestamp()),
        "level": record.severity_text or severity_label(record.severity_number),
        "slug": string_attribute(record.attributes, ATTR_HOLONS_SLUG),
        "instance_uid": string_attribute(record.attributes, ATTR_HOLONS_INSTANCE_UID),
        "message": body_string(record),
    }
    if session_id := string_attribute(record.attributes, ATTR_HOLONS_SESSION_ID):
        rec["session_id"] = session_id
    if rpc_method := string_attribute(record.attributes, ATTR_RPC_METHOD):
        rec["rpc_method"] = rpc_method
    fields = attributes_map(record.attributes)
    if fields:
        rec["fields"] = fields
    if caller := string_attribute(record.attributes, ATTR_CODE_CALLER):
        rec["caller"] = caller
    if record.chain:
        rec["chain"] = list(record.chain)
    return rec


def _event_json(event: LogRecord) -> dict[str, typing.Any]:
    record = event.record
    rec: dict[str, typing.Any] = {
        "kind": "event",
        "ts": _iso8601(event.timestamp()),
        "event_name": record.event_name,
        "slug": string_attribute(record.attributes, ATTR_HOLONS_SLUG),
        "instance_uid": string_attribute(record.attributes, ATTR_HOLONS_INSTANCE_UID),
    }
    if session_id := string_attribute(record.attributes, ATTR_HOLONS_SESSION_ID):
        rec["session_id"] = session_id
    payload = attributes_map(record.attributes)
    if payload:
        rec["payload"] = payload
    if record.chain:
        rec["chain"] = list(record.chain)
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
    "ATTR_CODE_CALLER",
    "ATTR_HOLONS_INSTANCE_UID",
    "ATTR_HOLONS_SESSION_ID",
    "ATTR_HOLONS_SLUG",
    "ATTR_HOLONS_TRANSPORT",
    "ATTR_LOGGER_NAME",
    "ATTR_RPC_METHOD",
    "ATTR_SERVICE_INSTANCE_ID",
    "ATTR_SERVICE_NAME",
    "CANONICAL_EVENT_NAMES",
    "Config",
    "ContextValues",
    "Counter",
    "DEFAULT_BUCKETS",
    "EVENT_CONFIG_RELOADED",
    "EVENT_HANDLER_PANIC",
    "EVENT_INSTANCE_CRASHED",
    "EVENT_INSTANCE_EXITED",
    "EVENT_INSTANCE_READY",
    "EVENT_INSTANCE_SPAWNED",
    "EVENT_SESSION_ENDED",
    "EVENT_SESSION_STARTED",
    "EventBus",
    "Family",
    "Gauge",
    "Histogram",
    "HistogramSnapshot",
    "HolonObservabilityService",
    "InvalidTokenError",
    "Level",
    "LogRecord",
    "LogRing",
    "Logger",
    "MemberRelay",
    "MetaJSON",
    "Private",
    "PromServer",
    "Registry",
    "any_value_string",
    "any_value_to_python",
    "append_direct_child",
    "attribute_value",
    "attributes_map",
    "body_string",
    "check_env",
    "clone_log_record",
    "configure",
    "current",
    "current_context",
    "derive_run_dir",
    "enable_disk_writers",
    "enrich_for_multilog",
    "from_env",
    "from_proto_log_record",
    "parse_level",
    "read_meta_json",
    "register_service",
    "reset",
    "resource_attributes",
    "session_context",
    "severity_label",
    "string_attribute",
    "to_any_value",
    "to_prometheus_text",
    "to_proto_log_record",
    "to_proto_metrics",
    "write_meta_json",
]
