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
import sys
import threading
import time
import typing
from collections import deque
from dataclasses import dataclass, field


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
        if tok == "otel":
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
    families = _parse_op_obs(os.environ.get("OP_OBS", ""))
    if not cfg.slug:
        cfg.slug = os.path.basename(sys.argv[0]) if sys.argv else ""
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


__all__ = [
    "Config", "Observability", "Family", "Level", "EventType", "LogEntry", "Event",
    "Counter", "Gauge", "Histogram", "Registry", "HistogramSnapshot",
    "LogRing", "EventBus", "Hop", "Logger",
    "configure", "from_env", "current", "reset",
    "check_env", "parse_level", "append_direct_child", "enrich_for_multilog",
    "DEFAULT_BUCKETS", "InvalidTokenError",
]
