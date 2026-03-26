from __future__ import annotations

"""Client-side gRPC helpers for Python holons."""

import asyncio
import os
import shutil
import socket
import ssl
import subprocess
import tempfile
import threading
import time
from typing import Any, Callable

import grpc
import websockets
from websockets.exceptions import ConnectionClosed

from holons.transport import parse_uri


class _ManagedChannel(grpc.Channel):
    """grpc.Channel wrapper that can run transport cleanup on close()."""

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


class _WSDialProxy:
    """Local TCP proxy that tunnels bytes to a grpc-subprotocol WebSocket."""

    def __init__(self, uri: str, *, ssl_context: ssl.SSLContext | None = None):
        self._uri = uri
        self._ssl_context = ssl_context
        self._listen_socket: socket.socket | None = None
        self._accept_thread: threading.Thread | None = None
        self._closed = threading.Event()
        self._connections: set[socket.socket] = set()
        self._lock = threading.Lock()
        self.target: str | None = None

    def start(self) -> str:
        if self.target:
            return self.target

        listen = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        listen.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        listen.bind(("127.0.0.1", 0))
        listen.listen(16)
        listen.settimeout(0.2)

        self._listen_socket = listen
        host, port = listen.getsockname()
        self.target = f"{host}:{port}"

        self._accept_thread = threading.Thread(target=self._accept_loop, daemon=True)
        self._accept_thread.start()
        return self.target

    def close(self) -> None:
        if self._closed.is_set():
            return
        self._closed.set()

        if self._listen_socket is not None:
            try:
                self._listen_socket.close()
            except OSError:
                pass
            self._listen_socket = None

        with self._lock:
            conns = list(self._connections)
            self._connections.clear()

        for conn in conns:
            try:
                conn.shutdown(socket.SHUT_RDWR)
            except OSError:
                pass
            try:
                conn.close()
            except OSError:
                pass

    def _accept_loop(self) -> None:
        assert self._listen_socket is not None

        while not self._closed.is_set():
            try:
                conn, _ = self._listen_socket.accept()
            except socket.timeout:
                continue
            except OSError:
                break

            with self._lock:
                self._connections.add(conn)

            worker = threading.Thread(target=self._bridge_worker, args=(conn,), daemon=True)
            worker.start()

    def _bridge_worker(self, conn: socket.socket) -> None:
        loop = asyncio.new_event_loop()
        try:
            asyncio.set_event_loop(loop)
            loop.run_until_complete(self._bridge_connection(conn))
        except Exception:
            pass
        finally:
            try:
                conn.close()
            except OSError:
                pass
            with self._lock:
                self._connections.discard(conn)
            try:
                loop.close()
            except Exception:
                pass

    async def _bridge_connection(self, conn: socket.socket) -> None:
        conn.setblocking(False)
        loop = asyncio.get_running_loop()

        async with websockets.connect(
            self._uri,
            subprotocols=["grpc"],
            ping_interval=None,
            ping_timeout=None,
            max_size=None,
            ssl=self._ssl_context,
        ) as ws:

            async def tcp_to_ws() -> None:
                while not self._closed.is_set():
                    data = await loop.sock_recv(conn, 64 * 1024)
                    if not data:
                        await ws.close(code=1000, reason="tcp closed")
                        return
                    await ws.send(data)

            async def ws_to_tcp() -> None:
                try:
                    async for msg in ws:
                        if isinstance(msg, str):
                            data = msg.encode("utf-8")
                        else:
                            data = bytes(msg)
                        if data:
                            await loop.sock_sendall(conn, data)
                except ConnectionClosed:
                    return

            tasks = [
                asyncio.create_task(tcp_to_ws()),
                asyncio.create_task(ws_to_tcp()),
            ]

            done, pending = await asyncio.wait(tasks, return_when=asyncio.FIRST_COMPLETED)
            for task in pending:
                task.cancel()
            if pending:
                await asyncio.gather(*pending, return_exceptions=True)
            for task in done:
                _ = task.exception() if not task.cancelled() else None


class _StdioDialProxy:
    """Bridge child-process stdio pipes to a local gRPC dial target."""

    def __init__(
        self,
        command: tuple[str, ...],
        *,
        env: dict[str, str] | None = None,
        cwd: str | None = None,
    ):
        if not command:
            raise ValueError("command is required")

        self._command = list(command)
        self._env = env
        self._cwd = cwd

        self._proc: subprocess.Popen | None = None
        self._listen_socket: socket.socket | None = None
        self._listen_path: str | None = None
        self._listen_dir: str | None = None
        self._conn: socket.socket | None = None

        self._accept_thread: threading.Thread | None = None
        self._stdout_thread: threading.Thread | None = None
        self._stderr_thread: threading.Thread | None = None
        self._socket_to_stdin_thread: threading.Thread | None = None

        self._closed = threading.Event()
        self._lock = threading.Lock()
        self._pending_stdout: list[bytes] = []
        self._stderr_chunks: list[bytes] = []
        self.target: str | None = None

    def start(self) -> str:
        if self.target:
            return self.target

        proc = subprocess.Popen(
            self._command,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            cwd=self._cwd,
            env=self._env,
        )
        self._proc = proc

        if proc.stdin is None or proc.stdout is None:
            self.close()
            raise RuntimeError("stdio child process must expose stdin/stdout pipes")

        listen, target = self._create_listener()
        self._listen_socket = listen
        self.target = target

        self._accept_thread = threading.Thread(target=self._accept_loop, daemon=True)
        self._accept_thread.start()

        self._stdout_thread = threading.Thread(target=self._stdout_loop, daemon=True)
        self._stdout_thread.start()

        if proc.stderr is not None:
            self._stderr_thread = threading.Thread(target=self._stderr_loop, daemon=True)
            self._stderr_thread.start()

        self._raise_if_child_exited_early()
        return target

    def close(self) -> None:
        if self._closed.is_set():
            return
        self._closed.set()

        listen = self._listen_socket
        self._listen_socket = None
        if listen is not None:
            try:
                listen.close()
            except OSError:
                pass

        with self._lock:
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

        proc = self._proc
        self._proc = None
        if proc is not None:
            for stream in (proc.stdin, proc.stdout, proc.stderr):
                if stream is None:
                    continue
                try:
                    stream.close()
                except OSError:
                    pass

            if proc.poll() is None:
                try:
                    proc.terminate()
                    proc.wait(timeout=1.5)
                except subprocess.TimeoutExpired:
                    proc.kill()
                    proc.wait(timeout=1.0)
                except OSError:
                    pass

        self._cleanup_listener_path()

    def _create_listener(self) -> tuple[socket.socket, str]:
        if hasattr(socket, "AF_UNIX"):
            listen_dir = tempfile.mkdtemp(prefix="holons-stdio-")
            listen_path = os.path.join(listen_dir, "bridge.sock")
            unix_sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
            try:
                unix_sock.bind(listen_path)
                unix_sock.listen(1)
                unix_sock.settimeout(0.2)
                self._listen_dir = listen_dir
                self._listen_path = listen_path
                return unix_sock, f"unix://{listen_path}"
            except OSError:
                try:
                    unix_sock.close()
                except OSError:
                    pass
                shutil.rmtree(listen_dir, ignore_errors=True)

        listen = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        listen.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        listen.bind(("127.0.0.1", 0))
        listen.listen(1)
        listen.settimeout(0.2)
        host, port = listen.getsockname()
        return listen, f"{host}:{port}"

    def _accept_loop(self) -> None:
        assert self._listen_socket is not None

        while not self._closed.is_set():
            try:
                conn, _ = self._listen_socket.accept()
            except socket.timeout:
                continue
            except OSError:
                break

            conn.settimeout(0.2)

            with self._lock:
                if self._conn is not None:
                    conn.close()
                    continue

                self._conn = conn
                pending = self._pending_stdout
                self._pending_stdout = []

            for chunk in pending:
                try:
                    conn.sendall(chunk)
                except OSError:
                    break

            self._socket_to_stdin_thread = threading.Thread(
                target=self._socket_to_stdin_loop,
                args=(conn,),
                daemon=True,
            )
            self._socket_to_stdin_thread.start()

    def _stdout_loop(self) -> None:
        proc = self._proc
        if proc is None or proc.stdout is None:
            return

        while not self._closed.is_set():
            try:
                chunk = _read_pipe_chunk(proc.stdout, 64 * 1024)
            except OSError:
                break

            if not chunk:
                break

            with self._lock:
                conn = self._conn
                if conn is None:
                    self._pending_stdout.append(bytes(chunk))
                    continue

            try:
                conn.sendall(chunk)
            except OSError:
                with self._lock:
                    if self._conn is conn:
                        self._conn = None

        with self._lock:
            conn = self._conn
        if conn is not None:
            try:
                conn.shutdown(socket.SHUT_WR)
            except OSError:
                pass

    def _stderr_loop(self) -> None:
        proc = self._proc
        if proc is None or proc.stderr is None:
            return

        while not self._closed.is_set():
            try:
                chunk = _read_pipe_chunk(proc.stderr, 64 * 1024)
            except OSError:
                break
            if not chunk:
                return
            with self._lock:
                self._stderr_chunks.append(bytes(chunk))

    def _socket_to_stdin_loop(self, conn: socket.socket) -> None:
        proc = self._proc
        if proc is None or proc.stdin is None:
            return

        while not self._closed.is_set():
            try:
                data = conn.recv(64 * 1024)
            except socket.timeout:
                continue
            except OSError:
                break

            if not data:
                break

            try:
                proc.stdin.write(data)
                proc.stdin.flush()
            except (BrokenPipeError, OSError, ValueError):
                break

        try:
            proc.stdin.close()
        except OSError:
            pass

        with self._lock:
            if self._conn is conn:
                self._conn = None

        try:
            conn.shutdown(socket.SHUT_RDWR)
        except OSError:
            pass
        try:
            conn.close()
        except OSError:
            pass

    def _cleanup_listener_path(self) -> None:
        path = self._listen_path
        self._listen_path = None
        if path:
            try:
                os.unlink(path)
            except FileNotFoundError:
                pass
            except OSError:
                pass

        listen_dir = self._listen_dir
        self._listen_dir = None
        if listen_dir:
            shutil.rmtree(listen_dir, ignore_errors=True)

    def _raise_if_child_exited_early(self) -> None:
        proc = self._proc
        if proc is None:
            return

        deadline = time.time() + 0.25
        while time.time() < deadline and not self._closed.is_set():
            code = proc.poll()
            if code is not None:
                details = f"stdio child process exited with code {code}"
                stderr_text = self._stderr_text()
                if stderr_text:
                    details = f"{details}: {stderr_text}"
                self.close()
                raise RuntimeError(details)
            time.sleep(0.01)

    def _stderr_text(self, limit: int = 4096) -> str:
        with self._lock:
            if not self._stderr_chunks:
                return ""
            raw = b"".join(self._stderr_chunks)

        text = raw.decode("utf-8", errors="replace").strip()
        if len(text) > limit:
            return text[-limit:]
        return text


def _read_pipe_chunk(stream: Any, size: int) -> bytes:
    read1 = getattr(stream, "read1", None)
    if callable(read1):
        return read1(size)
    fileno = getattr(stream, "fileno", None)
    if callable(fileno):
        return os.read(fileno(), size)
    return stream.read(size)


def dial(address: str) -> grpc.Channel:
    """Dial a gRPC server at address.

    Accepted forms:
    - host:port
    - unix:///path.sock
    """
    return grpc.insecure_channel(address)


def dial_websocket(
    uri: str,
    *,
    ssl_context: ssl.SSLContext | None = None,
) -> grpc.Channel:
    """Dial gRPC over ws:// or wss:// using a local TCP bridge."""
    parsed = parse_uri(uri)
    if parsed.scheme not in {"ws", "wss"}:
        raise ValueError(f"dial_websocket expects ws:// or wss:// URI, got {uri!r}")

    proxy = _WSDialProxy(parsed.raw, ssl_context=ssl_context)
    target = proxy.start()
    channel = grpc.insecure_channel(target)
    return _ManagedChannel(channel, on_close=proxy.close)


def dial_stdio(
    command: str,
    *args: str,
    env: dict[str, str] | None = None,
    cwd: str | None = None,
) -> grpc.Channel:
    """Dial gRPC over stdio by proxying a child process through a local socket."""
    if not command:
        raise ValueError("command is required")

    proxy = _StdioDialProxy((command, *args), env=env, cwd=cwd)
    target = proxy.start()
    channel = grpc.insecure_channel(target)
    return _ManagedChannel(channel, on_close=proxy.close)


def dial_uri(
    uri: str,
    *,
    stdio_command: list[str] | tuple[str, ...] | None = None,
    stdio_env: dict[str, str] | None = None,
    stdio_cwd: str | None = None,
    websocket_ssl_context: ssl.SSLContext | None = None,
) -> grpc.Channel:
    """Dial using a transport URI.

    Supports:
    - tcp://
    - unix://
    - stdio:// (with stdio_command)
    - ws://, wss://
    """
    parsed = parse_uri(uri)

    if parsed.scheme == "tcp":
        host = parsed.host or "127.0.0.1"
        if host == "0.0.0.0":
            host = "127.0.0.1"
        return grpc.insecure_channel(f"{host}:{parsed.port}")

    if parsed.scheme == "unix":
        return grpc.insecure_channel(f"unix://{parsed.path}")

    if parsed.scheme == "stdio":
        if not stdio_command:
            raise ValueError("dial_uri(stdio://) requires stdio_command")
        cmd = list(stdio_command)
        return dial_stdio(cmd[0], *cmd[1:], env=stdio_env, cwd=stdio_cwd)

    if parsed.scheme in {"ws", "wss"}:
        return dial_websocket(parsed.raw, ssl_context=websocket_ssl_context)

    raise ValueError(
        f"dial_uri() supports tcp://, unix://, stdio://, ws://, and wss://. "
        f"Use transport-specific clients for {parsed.scheme}://"
    )
