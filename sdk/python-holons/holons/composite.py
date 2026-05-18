"""Helpers for composite holons."""

from __future__ import annotations

import os
import re
import secrets
import stat
import subprocess
import sys
import tempfile
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Callable, Iterable, Optional
from urllib.parse import urlparse

import grpc

from holons import grpcclient, observability
from holons.v1 import describe_pb2, describe_pb2_grpc, observability_pb2, observability_pb2_grpc


TransportCoverageSequence = [
    "stdio", "stdio", "tcp", "unix", "tcp", "tcp",
    "stdio", "unix", "unix", "stdio",
]


@dataclass(frozen=True)
class ChildSpec:
    slug: str
    binary: str


@dataclass(frozen=True)
class SpawnOptions:
    slug: str = ""
    binary_path: str = ""
    transport: str = "stdio"
    instance_uid: str = ""
    downstream_chain: tuple[ChildSpec, ...] = ()
    extra_env: dict[str, str] = field(default_factory=dict)
    dial_options: tuple[Callable[["_DialOptions"], None], ...] = ()


@dataclass
class SpawnedMember:
    slug: str
    uid: str
    listen_uri: str
    conn: grpc.Channel
    process: subprocess.Popen | None = None
    relay: observability.MemberRelay | None = None
    _stopped: bool = False

    def stop(self, timeout: float = 3.0) -> None:
        if self._stopped:
            return
        self._stopped = True
        close = getattr(self.conn, "close", None)
        if callable(close):
            close()
        if self.relay is not None:
            self.relay.stop()
        if self.process is None:
            return
        if self.process.poll() is not None:
            return
        try:
            self.process.terminate()
            self.process.wait(timeout=timeout)
        except subprocess.TimeoutExpired:
            self.process.kill()
            self.process.wait(timeout=1.0)


@dataclass(frozen=True)
class CascadeOptions:
    transport: str = "stdio"
    members: tuple[ChildSpec, ...] = ()
    extra_env: dict[str, str] = field(default_factory=dict)


@dataclass
class Cascade:
    top: SpawnedMember

    def stop(self, timeout: float = 3.0) -> None:
        self.top.stop(timeout)


@dataclass(frozen=True)
class _DialOptions:
    transitive_observability: Optional[bool] = None


def WithTransitiveObservability(enabled: bool) -> Callable[[_DialOptions], None]:
    def apply(opts: _DialOptions) -> None:
        object.__setattr__(opts, "transitive_observability", bool(enabled))

    return apply


def _apply_dial_options(opts: Iterable[Callable[[_DialOptions], None]]) -> _DialOptions:
    out = _DialOptions()
    for opt in opts:
        if opt is not None:
            opt(out)
    return out


def SpawnMember(opts: SpawnOptions) -> SpawnedMember:
    slug = (opts.slug or Path(opts.binary_path).name).strip()
    binary = opts.binary_path.strip()
    if not slug:
        raise ValueError("spawn member: slug is required")
    if not binary:
        raise ValueError(f"spawn member {slug}: binary path is required")

    uid = opts.instance_uid.strip() or _new_instance_uid()
    transport = (opts.transport or "stdio").strip().lower()
    listen_uri, cleanup_path = _listen_uri_for_spawn(transport, uid)
    if cleanup_path:
        try:
            os.remove(cleanup_path)
        except FileNotFoundError:
            pass

    args = [binary, "serve", "--listen", listen_uri, "--transport", transport]
    for child in opts.downstream_chain:
        if not child.slug or not child.binary:
            raise ValueError(f"spawn member {slug}: downstream child requires slug and binary")
        args.extend(["--child", f"{child.slug}={child.binary}"])

    env = _spawn_environment(uid, opts.extra_env)
    # Leave cwd unset so CPython can use posix_spawn on platforms that support
    # it; forking after grpcio has active poller threads is flaky.
    cwd = None
    process: subprocess.Popen | None = None

    if transport == "stdio":
        conn = grpcclient.dial_uri("stdio://", stdio_command=args, stdio_env=env, stdio_cwd=cwd)
        _describe_ready(conn, 10.0)
        member = SpawnedMember(slug=slug, uid=uid, listen_uri="stdio://", conn=conn)
    else:
        process = subprocess.Popen(
            args,
            cwd=cwd,
            env=env,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.PIPE,
            text=True,
            close_fds=False,
        )
        member = SpawnedMember(slug=slug, uid=uid, listen_uri=listen_uri, conn=None, process=process)  # type: ignore[arg-type]
        try:
            meta = _wait_spawn_meta(_spawn_run_root(env), slug, uid, 10.0)
            member.listen_uri = meta["address"]
            conn, _ = _dial_ready(meta["address"], 10.0)
            member.conn = conn
        except Exception:
            stderr = _collect_stderr(process)
            member.stop()
            detail = f": {stderr}" if stderr else ""
            raise RuntimeError(f"spawn member {slug}{detail}") from None

    dial_opts = _apply_dial_options(opts.dial_options)
    transitive = True if dial_opts.transitive_observability is None else dial_opts.transitive_observability
    if transitive:
        relay = observability.MemberRelay(slug, uid, member.conn, observability.current(), retry_delay=0.2)
        relay.start()
        member.relay = relay
    return member


def BuildCascade(opts: CascadeOptions) -> Cascade:
    members = list(opts.members)
    if not members:
        raise ValueError("build cascade: at least one member is required")
    top = members[0]
    spawned = SpawnMember(
        SpawnOptions(
            slug=top.slug,
            binary_path=top.binary,
            transport=opts.transport,
            downstream_chain=tuple(members[1:]),
            extra_env=dict(opts.extra_env),
        )
    )
    return Cascade(top=spawned)


def Dial(address: str, *opts: Callable[[_DialOptions], None]) -> grpc.Channel:
    conn, desc = _dial_ready(_normalize_address_for_dial(address), 10.0)
    dial_opts = _apply_dial_options(opts)
    transitive = False if dial_opts.transitive_observability is None else dial_opts.transitive_observability
    if not transitive:
        return conn
    identity = _resolve_relay_identity(conn, desc)
    relay = observability.MemberRelay(identity.slug, identity.uid, conn, observability.current(), retry_delay=0.2)
    relay.start()
    return grpcclient._ManagedChannel(conn, on_close=relay.stop)  # type: ignore[attr-defined]


@dataclass(frozen=True)
class CheckOutcome:
    pass_: bool = False
    evidence: str = ""


ChainSlug = str


@dataclass(frozen=True)
class LogCheckOptions:
    conn: grpc.Channel | None = None
    sender: str = ""
    leaf_uid: str = ""
    expected_chain: tuple[ChainSlug, ...] = ()
    timeout: float = 3.0
    poll_interval: float = 0.1
    live: bool = False


@dataclass(frozen=True)
class EventCheckOptions:
    conn: grpc.Channel | None = None
    event_name: str = observability.EVENT_INSTANCE_READY
    leaf_uid: str = ""
    expected_chain: tuple[ChainSlug, ...] = ()
    timeout: float = 3.0
    poll_interval: float = 0.1
    live: bool = False


def CheckRelayedLog(opts: LogCheckOptions) -> CheckOutcome:
    deadline = time.time() + (opts.timeout or 3.0)
    last = CheckOutcome(evidence="not checked")
    while True:
        try:
            entries = _read_log_entries(opts.conn)
            last = _match_relayed_log(entries, opts)
            if last.pass_:
                return last
        except Exception as exc:
            last = CheckOutcome(evidence=_compact(str(exc)))
        if time.time() >= deadline:
            return last
        time.sleep(opts.poll_interval or 0.1)


def CheckRelayedEvent(opts: EventCheckOptions) -> CheckOutcome:
    deadline = time.time() + (opts.timeout or 3.0)
    last = CheckOutcome(evidence="not checked")
    while True:
        try:
            events = _read_event_entries(opts.conn)
            last = _match_relayed_event(events, opts)
            if last.pass_:
                return last
        except Exception as exc:
            last = CheckOutcome(evidence=_compact(str(exc)))
        if time.time() >= deadline:
            return last
        time.sleep(opts.poll_interval or 0.1)


def member(member_id: str) -> str:
    """Resolve a declared member's binary relative to this composite."""

    executable = os.environ.get("OP_HOLON_EXECUTABLE") or sys.executable
    return member_from_executable(executable, member_id)


def member_from_executable(executable: str | os.PathLike[str], member_id: str) -> str:
    member_id = member_id.strip()
    if not member_id:
        raise ValueError("member id is required")
    member_dir = Path(executable).resolve().parent / "holons" / member_id
    for entry in sorted(member_dir.iterdir()):
        if entry.is_file() and _is_executable(entry):
            return str(entry)
    raise FileNotFoundError(f"no executable found in {member_dir}")


def _is_executable(path: Path) -> bool:
    if os.name == "nt":
        return path.suffix.lower() == ".exe"
    return bool(path.stat().st_mode & (stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH))


def _listen_uri_for_spawn(transport: str, uid: str) -> tuple[str, str]:
    if transport == "stdio":
        return "stdio://", ""
    if transport == "tcp":
        return "tcp://127.0.0.1:0", ""
    if transport == "unix":
        path = os.path.join(tempfile.gettempdir(), f"op-{_clean_socket_token(uid)}.sock")
        return f"unix://{path}", path
    raise ValueError(f"unsupported transport {transport!r}")


def _spawn_environment(uid: str, extra: dict[str, str]) -> dict[str, str]:
    env = dict(os.environ)
    env["OP_INSTANCE_UID"] = uid
    env["OP_RUN_DIR"] = _run_root_from_env(env)
    env["HOLONS_PARENT_PID"] = str(os.getpid())
    active = _active_observability_families()
    if active:
        env["OP_OBS"] = active
    env.update(extra)
    return env


def _active_observability_families() -> str:
    current = observability.current()
    names = [
        (observability.Family.LOGS, "logs"),
        (observability.Family.METRICS, "metrics"),
        (observability.Family.EVENTS, "events"),
        (observability.Family.PROM, "prom"),
    ]
    return ",".join(name for family, name in names if current.enabled(family))


def _run_root_from_env(env: dict[str, str]) -> str:
    if env.get("OP_RUN_DIR", "").strip():
        return env["OP_RUN_DIR"]
    if env.get("OPPATH", "").strip():
        return str(Path(env["OPPATH"]) / "run")
    if env.get("HOME", "").strip():
        return str(Path(env["HOME"]) / ".op" / "run")
    return str(Path(tempfile.gettempdir()) / ".op" / "run")


def _spawn_run_root(env: dict[str, str]) -> str:
    return _run_root_from_env(env)


def _wait_spawn_meta(run_root: str, slug: str, uid: str, timeout: float) -> dict[str, str]:
    path = Path(run_root) / slug / uid / "meta.json"
    deadline = time.time() + timeout
    last: Exception | None = None
    while time.time() < deadline:
        try:
            meta = observability.read_meta_json(str(path.parent))
            if meta.get("uid") == uid and meta.get("address"):
                return meta
        except Exception as exc:
            last = exc
        time.sleep(0.05)
    raise RuntimeError(f"meta not ready for {slug}/{uid}: {last}")


def _dial_ready(address: str, timeout: float) -> tuple[grpc.Channel, describe_pb2.DescribeResponse]:
    deadline = time.time() + timeout
    last: Exception | None = None
    while time.time() < deadline:
        conn = _dial_channel(address)
        try:
            desc = _describe_ready(conn, 1.0)
            return conn, desc
        except Exception as exc:
            last = exc
            close = getattr(conn, "close", None)
            if callable(close):
                close()
            time.sleep(0.05)
    raise RuntimeError(f"dial {address}: {last}")


def _describe_ready(conn: grpc.Channel, timeout: float) -> describe_pb2.DescribeResponse:
    deadline = time.time() + timeout
    last: Exception | None = None
    while time.time() < deadline:
        try:
            return describe_pb2_grpc.HolonMetaStub(conn).Describe(
                describe_pb2.DescribeRequest(),
                timeout=0.5,
            )
        except Exception as exc:
            last = exc
            time.sleep(0.05)
    raise RuntimeError(f"describe: {last}")


def _dial_channel(address: str) -> grpc.Channel:
    if address.startswith("tcp://") or address.startswith("unix://"):
        return grpcclient.dial_uri(address)
    return grpc.insecure_channel(address)


def _normalize_address_for_dial(address: str) -> str:
    trimmed = address.strip()
    if not trimmed:
        raise ValueError("dial address is required")
    if trimmed.startswith("stdio://"):
        raise ValueError("composite.Dial does not support stdio addresses; use SpawnMember")
    if trimmed.startswith("tcp://") or trimmed.startswith("unix://"):
        return _normalize_dial_target(trimmed)
    if "://" in trimmed:
        raise ValueError(f"unsupported dial address {address!r}")
    if not re.match(r"^[^:]+:\d+$", trimmed):
        raise ValueError(f"dial address must be tcp://host:port, unix:///path, or host:port: {address!r}")
    return trimmed


def _normalize_dial_target(target: str) -> str:
    if not target.startswith("tcp://"):
        return target
    parsed = urlparse(target)
    host = parsed.hostname or "127.0.0.1"
    if host in ("0.0.0.0", "::"):
        host = "127.0.0.1"
    if parsed.port is None:
        return target
    return f"tcp://{host}:{parsed.port}"


@dataclass(frozen=True)
class _RelayIdentity:
    slug: str
    uid: str


def _resolve_relay_identity(conn: grpc.Channel, desc: describe_pb2.DescribeResponse) -> _RelayIdentity:
    fallback = _slug_from_describe(desc)
    client = observability_pb2_grpc.HolonObservabilityStub(conn)
    try:
        for event in client.Events(observability_pb2.EventsRequest(follow=False), timeout=1.0):
            uid = observability.string_attribute(
                event.attributes,
                observability.ATTR_HOLONS_INSTANCE_UID,
            )
            if not event.chain and uid:
                slug = observability.string_attribute(
                    event.attributes,
                    observability.ATTR_HOLONS_SLUG,
                )
                return _RelayIdentity(slug=slug or fallback, uid=uid)
    except Exception:
        pass
    try:
        for entry in client.Logs(observability_pb2.LogsRequest(follow=False), timeout=1.0):
            uid = observability.string_attribute(
                entry.attributes,
                observability.ATTR_HOLONS_INSTANCE_UID,
            )
            if not entry.chain and uid:
                slug = observability.string_attribute(
                    entry.attributes,
                    observability.ATTR_HOLONS_SLUG,
                )
                return _RelayIdentity(slug=slug or fallback, uid=uid)
    except Exception:
        pass
    raise RuntimeError("resolve relay identity: peer did not expose a local log or event with slug and instance_uid")


def _slug_from_describe(desc: describe_pb2.DescribeResponse) -> str:
    identity = desc.manifest.identity
    for alias in identity.aliases:
        if alias.strip():
            return alias.strip()
    return _slugify(f"{identity.given_name}-{identity.family_name}") or _slugify(identity.family_name)


def _slugify(value: str) -> str:
    value = value.strip().lower()
    value = re.sub(r"[^a-z0-9]+", "-", value)
    return value.strip("-")


def _read_log_entries(conn: grpc.Channel | None) -> list[observability.LogRecord]:
    if conn is None:
        ring = observability.current().log_ring
        if ring is None:
            raise RuntimeError("logs family is not enabled")
        return ring.drain()
    stream = observability_pb2_grpc.HolonObservabilityStub(conn).Logs(
        observability_pb2.LogsRequest(
            min_severity_number=observability_pb2.SEVERITY_NUMBER_INFO,
            follow=False,
        ),
        timeout=2.0,
    )
    return [observability.from_proto_log_record(entry) for entry in stream]


def _read_event_entries(conn: grpc.Channel | None) -> list[observability.LogRecord]:
    if conn is None:
        bus = observability.current().event_bus
        if bus is None:
            raise RuntimeError("events family is not enabled")
        return bus.drain()
    stream = observability_pb2_grpc.HolonObservabilityStub(conn).Events(
        observability_pb2.EventsRequest(follow=False),
        timeout=2.0,
    )
    return [observability.from_proto_log_record(event) for event in stream]


def _match_relayed_log(entries: list[observability.LogRecord], opts: LogCheckOptions) -> CheckOutcome:
    for entry in entries:
        record = entry.record
        if observability.body_string(record) != "tick received":
            continue
        fields = observability.attributes_map(record.attributes)
        if fields.get("sender") != opts.sender or fields.get("responder_uid") != opts.leaf_uid:
            continue
        chain_err = _compare_chain(record.chain, opts.expected_chain)
        if chain_err:
            return CheckOutcome(evidence=_compact(f"matching log bad chain: {chain_err}"))
        return CheckOutcome(pass_=True)
    return CheckOutcome(evidence=_compact(f"no relayed tick log sender={opts.sender} leaf_uid={opts.leaf_uid} entries={len(entries)}"))


def _match_relayed_event(events: list[observability.LogRecord], opts: EventCheckOptions) -> CheckOutcome:
    wanted = opts.event_name
    for event in events:
        record = event.record
        uid = observability.string_attribute(
            record.attributes,
            observability.ATTR_HOLONS_INSTANCE_UID,
        )
        if record.event_name != wanted or uid != opts.leaf_uid:
            continue
        chain_err = _compare_chain(record.chain, opts.expected_chain)
        if chain_err:
            return CheckOutcome(evidence=_compact(f"matching event bad chain: {chain_err}"))
        return CheckOutcome(pass_=True)
    return CheckOutcome(evidence=_compact(f"no relayed {wanted} event leaf_uid={opts.leaf_uid} events={len(events)}"))


def _compare_chain(got: Iterable[str], want: Iterable[str]) -> str:
    got_list = list(got)
    want_list = list(want)
    if len(got_list) != len(want_list):
        return f"chain length {len(got_list)} want {len(want_list)}"
    for idx, (actual, expected) in enumerate(zip(got_list, want_list)):
        if actual != expected:
            return f"hop {idx}={actual} want {expected}"
    return ""


def _compact(value: str) -> str:
    value = " ".join(str(value).split())
    if not value:
        return "<empty>"
    if len(value) <= 240:
        return value
    return value[:240] + "..."


def _new_instance_uid() -> str:
    return secrets.token_hex(12)


def _clean_socket_token(value: str) -> str:
    value = value.strip()[:24]
    return re.sub(r"[/\\: ]+", "-", value)


def _collect_stderr(process: subprocess.Popen | None) -> str:
    if process is None or process.stderr is None:
        return ""
    try:
        if process.poll() is None:
            process.terminate()
        _, stderr = process.communicate(timeout=1.0)
    except Exception:
        return ""
    return (stderr or "").strip()


spawn_member = SpawnMember
build_cascade = BuildCascade
dial = Dial
