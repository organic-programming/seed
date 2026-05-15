from __future__ import annotations

"""Standard gRPC server runner for Python holons."""

import logging
import os
import shutil
import signal
import socket
import sys
import tempfile
import threading
import time
from concurrent import futures
from typing import Callable
from dataclasses import dataclass
from urllib.parse import urlparse

import grpc
from grpc_reflection.v1alpha import reflection

from holons import describe
from holons import grpcclient
from holons import observability
from holons.transport import DEFAULT_URI, scheme
from holons.v1 import observability_pb2, observability_pb2_grpc

logger = logging.getLogger("holons.serve")

RegisterFunc = Callable[[grpc.Server], None]
_MAX_GRPC_MESSAGE_BYTES = 1 << 20


@dataclass(frozen=True)
class ParsedFlags:
    listen_uri: str = DEFAULT_URI
    reflect: bool = False


@dataclass(frozen=True)
class MemberRef:
    slug: str
    address: str
    uid: str = ""


@dataclass(frozen=True)
class ChildSpec:
    slug: str
    binary: str


@dataclass(frozen=True)
class ServeOptions:
    reflect: bool = False
    member_endpoints: tuple[MemberRef, ...] = ()
    slug: str = ""


def parse_flags(args: list[str]) -> str:
    """Extract --listen or --port from command-line args."""
    return parse_options(args).listen_uri


def parse_options(args: list[str]) -> ParsedFlags:
    """Extract --listen, --port, and --reflect from command-line args."""
    listen_uri = DEFAULT_URI
    reflect_enabled = False

    for i, arg in enumerate(args):
        if arg == "--listen" and i + 1 < len(args):
            listen_uri = args[i + 1]
        if arg == "--port" and i + 1 < len(args):
            listen_uri = f"tcp://:{args[i + 1]}"
        if arg == "--reflect":
            reflect_enabled = True

    return ParsedFlags(listen_uri=listen_uri, reflect=reflect_enabled)


def parse_child_flags(args: list[str]) -> tuple[list[ChildSpec], list[str]]:
    """Extract repeated --child <slug>=<binary> flags from args."""
    children: list[ChildSpec] = []
    remaining: list[str] = []
    index = 0
    while index < len(args):
        arg = args[index]
        if arg == "--child" and index + 1 < len(args):
            child = _parse_child_spec(args[index + 1])
            if child is not None:
                children.append(child)
            index += 2
            continue
        if arg.startswith("--child="):
            child = _parse_child_spec(arg.removeprefix("--child="))
            if child is not None:
                children.append(child)
            index += 1
            continue
        remaining.append(arg)
        index += 1
    return children, remaining


def _parse_child_spec(raw: str) -> ChildSpec | None:
    if "=" not in raw:
        return None
    slug, binary = raw.split("=", 1)
    slug = slug.strip()
    binary = binary.strip()
    if not slug or not binary:
        return None
    return ChildSpec(slug=slug, binary=binary)


ParseChildFlags = parse_child_flags


def run(listen_uri: str, register_fn: RegisterFunc) -> None:
    """Start a gRPC server with reflection disabled by default."""
    run_with_options(listen_uri, register_fn, reflect=False)


def run_with_options(
    listen_uri: str,
    register_fn: RegisterFunc,
    reflect: bool = False,
    max_workers: int = 10,
    on_listen: Callable[[str], None] | None = None,
) -> None:
    run_with_serve_options(
        listen_uri,
        register_fn,
        ServeOptions(reflect=reflect),
        max_workers=max_workers,
        on_listen=on_listen,
    )


def run_with_serve_options(
    listen_uri: str,
    register_fn: RegisterFunc,
    options: ServeOptions | None = None,
    max_workers: int = 10,
    on_listen: Callable[[str], None] | None = None,
) -> None:
    """Start a gRPC server on the given transport URI.

    Native gRPC python transports: tcp://, unix://
    Bridged transports: stdio://
    """
    if options is None:
        options = ServeOptions()
    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=max_workers),
        options=[
            ("grpc.max_receive_message_length", _MAX_GRPC_MESSAGE_BYTES),
            ("grpc.max_send_message_length", _MAX_GRPC_MESSAGE_BYTES),
        ],
    )
    observability.check_env()
    obs = None
    if os.environ.get("OP_OBS", "").strip():
        active = observability.current()
        obs = active if active.families else observability.from_env(observability.Config(slug=options.slug))
    register_fn(server)
    _register_holon_meta(server)
    if obs is not None and obs.families:
        observability.register_service(server, obs)

    reflection_enabled = False
    if options.reflect:
        service_names = [reflection.SERVICE_NAME]
        for handler in getattr(server._state, "generic_handlers", ()):
            service_name = getattr(handler, "service_name", None)
            if callable(service_name):
                name = service_name()
                if name and name not in service_names:
                    service_names.append(name)
        reflection.enable_server_reflection(tuple(service_names), server)
        reflection_enabled = True

    transport = scheme(listen_uri)
    stdio_bridge = None

    if transport == "tcp":
        addr = listen_uri[6:]
        port = server.add_insecure_port(addr)
        host = addr.rpartition(":")[0] or "0.0.0.0"
        actual_uri = f"tcp://{host}:{port}"
    elif transport == "unix":
        path = listen_uri[7:]
        server.add_insecure_port(f"unix:{path}")
        actual_uri = listen_uri
    elif transport == "stdio":
        # Bridge stdin/stdout to a temp Unix socket that grpcio can listen on
        stdio_bridge = _StdioServeBridge()
        sock_path = stdio_bridge.start()
        server.add_insecure_port(f"unix:{sock_path}")
        actual_uri = "stdio://"
    else:
        raise ValueError(
            "gRPC Python server supports tcp://, unix://, and stdio:// "
            f"in run_with_options(). For {transport}://, use holons.transport.listen() "
            "with a custom server loop."
        )

    mode = "reflection ON" if reflection_enabled else "reflection OFF"
    prom_server = _start_prom_server(obs)
    metrics_addr = prom_server[1] if prom_server is not None else ""
    if obs is not None and obs.families:
        _start_observability_runtime(obs, actual_uri, transport, metrics_addr)
    if on_listen is not None:
        on_listen(actual_uri)
    logger.info("gRPC server listening on %s (%s)", actual_uri, mode)

    server.start()
    started_relays = _start_member_relays(obs, options.member_endpoints)

    # For stdio, start bridging after the server is ready
    if stdio_bridge is not None:
        stdio_bridge.connect_to_server()

    def _shutdown(*_args):
        logger.info("shutting down gRPC server")
        server.stop(10)

    signal.signal(signal.SIGTERM, _shutdown)
    signal.signal(signal.SIGINT, _shutdown)

    print(f"gRPC server listening on {actual_uri} ({mode})", file=sys.stderr)

    try:
        server.wait_for_termination()
    finally:
        for relay, channel in started_relays:
            relay.stop()
            _close_channel(channel)
        if prom_server is not None:
            prom_server[0].close()
        if stdio_bridge is not None:
            stdio_bridge.close()


def _start_observability_runtime(
    obs: observability.Observability,
    actual_uri: str,
    transport: str,
    metrics_addr: str = "",
) -> None:
    if not obs.cfg.run_dir:
        return
    observability.enable_disk_writers(obs.cfg.run_dir)
    if obs.enabled(observability.Family.EVENTS):
        obs.emit(
            observability.EventType.INSTANCE_READY,
            {"listener": actual_uri, "metrics_addr": metrics_addr},
        )
    observability.write_meta_json(
        obs.cfg.run_dir,
        observability.MetaJSON(
            slug=obs.cfg.slug,
            uid=obs.cfg.instance_uid,
            pid=os.getpid(),
            started_at=time.time(),
            transport=transport,
            address=actual_uri,
            metrics_addr=metrics_addr,
            log_path=os.path.join(obs.cfg.run_dir, "stdout.log") if obs.enabled(observability.Family.LOGS) else "",
            organism_uid=obs.cfg.organism_uid,
            organism_slug=obs.cfg.organism_slug,
        ),
    )


def _register_holon_meta(server: grpc.Server) -> None:
    try:
        describe.register(server)
    except describe.IncodeDescriptionError as exc:
        logger.error("HolonMeta registration failed: %s", exc)
        raise
    except Exception as exc:
        logger.exception("HolonMeta registration failed: %s", exc)
        raise RuntimeError(f"register HolonMeta: {exc}") from exc


def _start_prom_server(
    obs: observability.Observability | None,
) -> tuple[observability.PromServer, str] | None:
    if obs is None or not obs.enabled(observability.Family.PROM):
        return None
    addr = obs.cfg.prom_addr or ":0"
    server = observability.PromServer(addr)
    try:
        metrics_addr = server.start()
    except Exception as exc:
        logger.warning("warning: prom HTTP bind failed: %s", exc)
        return None
    logger.info("Prometheus /metrics listening on %s", metrics_addr)
    return server, metrics_addr


def _start_member_relays(
    obs: observability.Observability | None,
    members: tuple[MemberRef, ...],
) -> list[tuple[observability.MemberRelay, grpc.Channel]]:
    if (
        obs is None
        or (
            not obs.enabled(observability.Family.LOGS)
            and not obs.enabled(observability.Family.EVENTS)
        )
    ):
        return []
    started: list[tuple[observability.MemberRelay, grpc.Channel]] = []
    for raw in members:
        member = MemberRef(
            slug=(raw.slug or "").strip(),
            uid=(raw.uid or "").strip(),
            address=(raw.address or "").strip(),
        )
        if not member.slug or not member.address:
            logger.warning(
                'warning: observability relay skipped incomplete member ref: slug="%s" uid="%s" address="%s"',
                member.slug,
                member.uid,
                member.address,
            )
            continue
        channel = None
        try:
            channel = grpcclient.dial_uri(_normalize_relay_dial_target(member.address))
            member = _resolve_relay_member_identity(channel, member)
            if not member.uid:
                logger.warning(
                    "warning: observability relay uid unresolved for %s at %s; chain hops will have empty uid",
                    member.slug,
                    member.address,
                )
            relay = observability.MemberRelay(
                child_slug=member.slug,
                child_uid=member.uid,
                channel=channel,
                observability=obs,
            )
            relay.start()
            started.append((relay, channel))
            channel = None
        except Exception as exc:
            if channel is not None:
                _close_channel(channel)
            logger.warning(
                "warning: observability relay start %s/%s: %s",
                member.slug,
                member.uid,
                exc,
            )
    return started


def _resolve_relay_member_identity(channel: grpc.Channel, member: MemberRef) -> MemberRef:
    if member.uid:
        return member
    client = observability_pb2_grpc.HolonObservabilityStub(channel)
    try:
        stream = client.Events(
            observability_pb2.EventsRequest(
                types=[observability_pb2.INSTANCE_READY],
                follow=False,
            ),
            timeout=2.0,
        )
        for event in stream:
            if not event.instance_uid or event.chain:
                continue
            slug = event.slug.strip() or member.slug
            return MemberRef(slug=slug, uid=event.instance_uid.strip(), address=member.address)
    except Exception:
        pass

    try:
        snap = client.Metrics(observability_pb2.MetricsRequest(), timeout=2.0)
        if snap.instance_uid:
            slug = snap.slug.strip() or member.slug
            return MemberRef(slug=slug, uid=snap.instance_uid.strip(), address=member.address)
    except Exception:
        pass
    return member


def _normalize_relay_dial_target(target: str) -> str:
    trimmed = target.strip()
    if "://" not in trimmed:
        return trimmed
    parsed = urlparse(trimmed)
    if parsed.scheme != "tcp":
        return trimmed
    host = parsed.hostname or "127.0.0.1"
    if host in ("0.0.0.0", "::"):
        host = "127.0.0.1"
    if parsed.port is None:
        return trimmed
    return f"tcp://{host}:{parsed.port}"


def _close_channel(channel: grpc.Channel) -> None:
    close = getattr(channel, "close", None)
    if callable(close):
        close()


class _StdioServeBridge:
    """Bridge stdin/stdout to a Unix socket for grpcio server-side stdio.

    grpcio's Python binding cannot accept raw file descriptors, so we
    create a temporary Unix socket, make grpcio listen on it, then
    forward bytes between stdin/stdout and the socket.
    """

    def __init__(self) -> None:
        self._sock_dir: str | None = None
        self._sock_path: str | None = None
        self._conn: socket.socket | None = None
        self._closed = threading.Event()
        self._stdin_fd = os.dup(0)
        self._stdout_fd = os.dup(1)
        # Redirect stdout to stderr so print() doesn't corrupt the data channel
        os.dup2(2, 1)

    def start(self) -> str:
        """Create the Unix socket path (does not connect yet)."""
        self._sock_dir = tempfile.mkdtemp(prefix="holons-stdio-serve-")
        self._sock_path = os.path.join(self._sock_dir, "bridge.sock")
        return self._sock_path

    def connect_to_server(self) -> None:
        """Connect to the grpcio Unix socket and start bridging threads."""
        assert self._sock_path is not None

        # Retry connecting — grpcio may not be listening yet
        conn = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        for attempt in range(20):
            try:
                conn.connect(self._sock_path)
                break
            except (ConnectionRefusedError, FileNotFoundError):
                import time
                time.sleep(0.05)
        else:
            raise RuntimeError("failed to connect to grpcio Unix socket")
        self._conn = conn

        # stdin → socket (client data arriving on stdin goes to grpc server)
        t1 = threading.Thread(target=self._stdin_to_socket, daemon=True)
        t1.start()

        # socket → stdout (grpc server responses go out on stdout)
        t2 = threading.Thread(target=self._socket_to_stdout, daemon=True)
        t2.start()

    def close(self) -> None:
        if self._closed.is_set():
            return
        self._closed.set()

        conn = self._conn
        self._conn = None
        if conn is not None:
            try:
                conn.shutdown(socket.SHUT_RDWR)
            except OSError:
                pass
            try:
                conn.close()
            except OSError:
                pass

        for fd in (self._stdin_fd, self._stdout_fd):
            try:
                os.close(fd)
            except OSError:
                pass

        if self._sock_path:
            try:
                os.unlink(self._sock_path)
            except OSError:
                pass
        if self._sock_dir:
            shutil.rmtree(self._sock_dir, ignore_errors=True)

    def _stdin_to_socket(self) -> None:
        """Forward bytes from stdin to the Unix socket."""
        conn = self._conn
        assert conn is not None

        while not self._closed.is_set():
            try:
                data = os.read(self._stdin_fd, 64 * 1024)
            except OSError:
                break
            if not data:
                break
            try:
                conn.sendall(data)
            except OSError:
                break

        # EOF on stdin — shut down the write side of the socket
        try:
            conn.shutdown(socket.SHUT_WR)
        except OSError:
            pass

        # Stdin closed means the parent process disconnected; trigger shutdown
        os.kill(os.getpid(), signal.SIGTERM)

    def _socket_to_stdout(self) -> None:
        """Forward bytes from the Unix socket to stdout."""
        conn = self._conn
        assert conn is not None

        while not self._closed.is_set():
            try:
                data = conn.recv(64 * 1024)
            except OSError:
                break
            if not data:
                break
            try:
                os.write(self._stdout_fd, data)
            except OSError:
                break
