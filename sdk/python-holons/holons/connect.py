from __future__ import annotations

"""Resolve holons to ready-to-use gRPC channels."""

from dataclasses import dataclass
from pathlib import Path
from queue import Empty, Queue
import shutil
import subprocess
import threading
import time
from typing import Any, Callable

import grpc

from . import discover, grpcclient
from .transport import parse_uri

DEFAULT_TIMEOUT = 5.0


@dataclass(slots=True)
class ConnectOptions:
    timeout: float = DEFAULT_TIMEOUT
    transport: str = "stdio"
    start: bool = True
    port_file: str = ""


class _ManagedConnectChannel(grpc.Channel):
    def __init__(self, inner: grpc.Channel, on_close: Callable[[], None] | None = None):
        self._inner = inner
        self._on_close = on_close
        self._closed = False

    def subscribe(self, callback: Any, try_to_connect: bool = False) -> None:
        self._inner.subscribe(callback, try_to_connect=try_to_connect)

    def unsubscribe(self, callback: Any) -> None:
        self._inner.unsubscribe(callback)

    def unary_unary(self, *args: Any, **kwargs: Any):
        return self._inner.unary_unary(*args, **kwargs)

    def unary_stream(self, *args: Any, **kwargs: Any):
        return self._inner.unary_stream(*args, **kwargs)

    def stream_unary(self, *args: Any, **kwargs: Any):
        return self._inner.stream_unary(*args, **kwargs)

    def stream_stream(self, *args: Any, **kwargs: Any):
        return self._inner.stream_stream(*args, **kwargs)

    def close(self) -> None:
        if self._closed:
            return
        self._closed = True
        try:
            self._inner.close()
        finally:
            if self._on_close is not None:
                self._on_close()

    def __getattr__(self, name: str) -> Any:
        return getattr(self._inner, name)


def connect(target: str, opts: ConnectOptions | None = None) -> grpc.Channel:
    trimmed = target.strip()
    if not trimmed:
        raise ValueError("target is required")

    options = _normalize_options(opts)
    ephemeral = opts is None or options.transport == "stdio"

    if _is_direct_target(trimmed):
        return _dial_ready(_normalize_dial_target(trimmed), options.timeout)

    entry = discover.find_by_slug(trimmed)
    if entry is None:
        raise ValueError(f'holon "{trimmed}" not found')

    binary_path = _resolve_binary_path(entry)
    port_file = options.port_file or _default_port_file_path(entry.slug)
    reusable = _usable_port_file(port_file, options.timeout)
    if reusable is not None:
        return reusable
    if not options.start:
        raise ValueError(f'holon "{trimmed}" is not running')

    if options.transport == "stdio":
        channel = grpcclient.dial_stdio(binary_path, "serve", "--listen", "stdio://")
        _wait_ready(channel, options.timeout)
        return channel

    if options.transport == "unix":
        advertised_uri, proc = _start_unix_holon(binary_path, entry.slug, port_file, options.timeout)
    else:
        advertised_uri, proc = _start_tcp_holon(binary_path, options.timeout)
    try:
        channel = _dial_ready(_normalize_dial_target(advertised_uri), options.timeout)
    except Exception:
        _stop_process(proc)
        raise

    def _cleanup() -> None:
        if ephemeral:
            _stop_process(proc)

    if not ephemeral:
        _reap_process(proc)
        try:
            _write_port_file(port_file, advertised_uri)
        except Exception:
            channel.close()
            _stop_process(proc)
            raise

    return _ManagedConnectChannel(channel, on_close=_cleanup)


def disconnect(channel: grpc.Channel) -> None:
    if channel is None:
        return
    channel.close()


def _normalize_options(opts: ConnectOptions | None) -> ConnectOptions:
    options = opts or ConnectOptions()
    timeout = options.timeout if options.timeout and options.timeout > 0 else DEFAULT_TIMEOUT
    transport = (options.transport or "stdio").strip().lower()
    if transport not in {"tcp", "stdio", "unix"}:
        raise ValueError(f"unsupported transport {options.transport!r}")
    return ConnectOptions(
        timeout=timeout,
        transport=transport,
        start=options.start,
        port_file=(options.port_file or "").strip(),
    )


def _dial_ready(target: str, timeout: float) -> grpc.Channel:
    channel = grpc.insecure_channel(target)
    try:
        _wait_ready(channel, timeout)
        return _ManagedConnectChannel(channel)
    except Exception:
        channel.close()
        raise


def _wait_ready(channel: grpc.Channel, timeout: float) -> None:
    grpc.channel_ready_future(channel).result(timeout=timeout)


def _usable_port_file(port_file: str, timeout: float) -> grpc.Channel | None:
    path = Path(port_file)
    try:
        raw_target = path.read_text(encoding="utf-8").strip()
    except OSError:
        return None

    if not raw_target:
        path.unlink(missing_ok=True)
        return None

    try:
        return _dial_ready(_normalize_dial_target(raw_target), min(max(timeout / 4.0, 0.25), 1.0))
    except Exception:
        path.unlink(missing_ok=True)
        return None


def _start_tcp_holon(binary_path: str, timeout: float) -> tuple[str, subprocess.Popen[str]]:
    proc = subprocess.Popen(
        [binary_path, "serve", "--listen", "tcp://127.0.0.1:0"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )

    queue: Queue[tuple[str, str | int]] = Queue()
    stderr_lines: list[str] = []

    def _reader(stream: Any, kind: str) -> None:
        try:
            while True:
                line = stream.readline()
                if not line:
                    return
                if kind == "stderr":
                    stderr_lines.append(line)
                queue.put(("line", line))
        finally:
            queue.put(("stream_closed", kind))

    if proc.stdout is None or proc.stderr is None:
        _stop_process(proc)
        raise RuntimeError("child process must expose stdout/stderr pipes")

    stdout_thread = threading.Thread(target=_reader, args=(proc.stdout, "stdout"), daemon=True)
    stderr_thread = threading.Thread(target=_reader, args=(proc.stderr, "stderr"), daemon=True)
    stdout_thread.start()
    stderr_thread.start()

    deadline = time.time() + timeout
    while time.time() < deadline:
        if proc.poll() is not None:
            stderr_text = "".join(stderr_lines).strip()
            details = f": {stderr_text}" if stderr_text else ""
            raise RuntimeError(f"holon exited before advertising an address ({proc.returncode}){details}")

        try:
            event, payload = queue.get(timeout=0.05)
        except Empty:
            continue

        if event != "line":
            continue

        uri = _first_uri(str(payload))
        if uri:
            return uri, proc

    _stop_process(proc)
    raise RuntimeError("timed out waiting for holon startup")


def _start_unix_holon(
    binary_path: str, slug: str, port_file: str, timeout: float
) -> tuple[str, subprocess.Popen[str]]:
    uri = _default_unix_socket_uri(slug, port_file)
    socket_path = uri.removeprefix("unix://")
    proc = subprocess.Popen(
        [binary_path, "serve", "--listen", uri],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.PIPE,
        text=True,
    )

    stderr_lines: list[str] = []

    if proc.stderr is None:
        _stop_process(proc)
        raise RuntimeError("child process must expose stderr pipe")

    def _stderr_reader() -> None:
        assert proc.stderr is not None
        for line in proc.stderr:
            stderr_lines.append(line)

    threading.Thread(target=_stderr_reader, daemon=True).start()

    deadline = time.time() + timeout
    while time.time() < deadline:
        if proc.poll() is not None:
            stderr_text = "".join(stderr_lines).strip()
            details = f": {stderr_text}" if stderr_text else ""
            raise RuntimeError(f"holon exited before binding unix socket ({proc.returncode}){details}")

        if Path(socket_path).exists():
            return uri, proc

        time.sleep(0.02)

    _stop_process(proc)
    stderr_text = "".join(stderr_lines).strip()
    details = f": {stderr_text}" if stderr_text else ""
    raise RuntimeError(f"timed out waiting for unix holon startup{details}")


def _resolve_binary_path(entry: discover.HolonEntry) -> str:
    if entry.manifest is None:
        raise ValueError(f'holon "{entry.slug}" has no manifest')

    binary = entry.manifest.artifacts.binary.strip()
    if not binary:
        raise ValueError(f'holon "{entry.slug}" has no artifacts.binary')

    binary_path = Path(binary)
    if binary_path.is_absolute() and binary_path.is_file():
        return str(binary_path)

    candidate = Path(entry.dir).joinpath(".op", "build", "bin", binary_path.name)
    if candidate.is_file():
        return str(candidate)

    looked_up = shutil.which(binary_path.name)
    if looked_up:
        return looked_up

    raise ValueError(f'built binary not found for holon "{entry.slug}"')


def _default_port_file_path(slug: str) -> str:
    return str(Path.cwd().joinpath(".op", "run", f"{slug}.port"))


def _default_unix_socket_uri(slug: str, port_file: str) -> str:
    label = _socket_label(slug)
    hash_value = _fnv1a64(port_file.encode("utf-8")) & 0xFFFFFFFFFFFF
    return f"unix:///tmp/holons-{label}-{hash_value:012x}.sock"


def _socket_label(slug: str) -> str:
    chars: list[str] = []
    last_dash = False
    for char in slug.strip().lower():
        if char.isascii() and (char.isalpha() or char.isdigit()):
            chars.append(char)
            last_dash = False
        elif char in "-_" and chars and not last_dash:
            chars.append("-")
            last_dash = True

        if len(chars) >= 24:
            break

    label = "".join(chars).strip("-")
    return label or "socket"


def _fnv1a64(data: bytes) -> int:
    hash_value = 0xCBF29CE484222325
    for byte in data:
        hash_value ^= byte
        hash_value = (hash_value * 0x100000001B3) & 0xFFFFFFFFFFFFFFFF
    return hash_value


def _write_port_file(port_file: str, uri: str) -> None:
    path = Path(port_file)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(f"{uri.strip()}\n", encoding="utf-8")


def _stop_process(proc: subprocess.Popen[str] | None) -> None:
    if proc is None or proc.poll() is not None:
        return

    proc.terminate()
    try:
        proc.wait(timeout=2)
    except subprocess.TimeoutExpired:
        proc.kill()
        proc.wait(timeout=2)


def _reap_process(proc: subprocess.Popen[str]) -> None:
    def _waiter() -> None:
        try:
            proc.wait()
        except Exception:
            return

    threading.Thread(target=_waiter, daemon=True).start()


def _is_direct_target(target: str) -> bool:
    return "://" in target or ":" in target


def _normalize_dial_target(target: str) -> str:
    if "://" not in target:
        return target

    parsed = parse_uri(target)
    if parsed.scheme == "tcp":
        host = parsed.host or "127.0.0.1"
        if host == "0.0.0.0":
            host = "127.0.0.1"
        return f"{host}:{parsed.port}"
    if parsed.scheme == "unix":
        return f"unix://{parsed.path}"
    return target


def _first_uri(line: str) -> str:
    for field in line.split():
        trimmed = field.strip().strip("\"'()[]{}.,")
        if trimmed.startswith(("tcp://", "unix://", "stdio://", "ws://", "wss://")):
            return trimmed
    return ""
