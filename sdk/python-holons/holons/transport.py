from __future__ import annotations

"""URI-based listener factory for gRPC servers.

Supported transport URIs:
    tcp://<host>:<port>   — TCP socket (default: tcp://:9090)
    unix://<path>         — Unix domain socket
    stdio://              — stdin/stdout pipe (single connection)
    ws://<host>:<port>    — WebSocket
    wss://<host>:<port>   — WebSocket over TLS
"""

from dataclasses import dataclass
import asyncio
import os
import socket
import threading
from typing import Any

DEFAULT_URI = "tcp://:9090"


@dataclass(frozen=True)
class ParsedURI:
    raw: str
    scheme: str
    host: str | None = None
    port: int | None = None
    path: str | None = None
    secure: bool = False


def listen(uri: str) -> Any:
    """Parse a transport URI and return a bound listener."""
    parsed = parse_uri(uri)

    if parsed.scheme == "tcp":
        return _listen_tcp(parsed)
    if parsed.scheme == "unix":
        return _listen_unix(parsed)
    if parsed.scheme == "stdio":
        return StdioListener()
    if parsed.scheme in {"ws", "wss"}:
        listener = WSListener(parsed)
        listener.start()
        return listener

    raise ValueError(
        f"unsupported transport URI: {uri!r} "
        "(expected tcp://, unix://, stdio://, ws://, or wss://)"
    )


def scheme(uri: str) -> str:
    """Extract the transport scheme from a URI."""
    idx = uri.find("://")
    return uri[:idx] if idx >= 0 else uri


def parse_uri(uri: str) -> ParsedURI:
    """Parse a transport URI into a normalized structure."""
    s = scheme(uri)

    if s == "tcp":
        addr = uri[6:]
        host, port = _split_host_port(addr, default_port=9090)
        return ParsedURI(raw=uri, scheme="tcp", host=host, port=port)

    if s == "unix":
        path = uri[7:]
        if not path:
            raise ValueError(f"invalid unix:// URI: {uri!r}")
        return ParsedURI(raw=uri, scheme="unix", path=path)

    if s == "stdio":
        return ParsedURI(raw="stdio://", scheme="stdio")

    if s in {"ws", "wss"}:
        secure = s == "wss"
        trimmed = uri[(7 if secure else 5):]
        if "/" in trimmed:
            addr, path = trimmed.split("/", 1)
            path = "/" + path
        else:
            addr = trimmed
            path = "/grpc"
        host, port = _split_host_port(addr, default_port=(443 if secure else 80))
        return ParsedURI(
            raw=uri,
            scheme=s,
            host=host,
            port=port,
            path=path,
            secure=secure,
        )

    raise ValueError(f"unsupported transport URI: {uri!r}")


# --- TCP ---

def _listen_tcp(parsed: ParsedURI) -> socket.socket:
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    sock.bind((parsed.host or "0.0.0.0", int(parsed.port or 9090)))
    sock.listen(128)
    return sock


# --- Unix ---

def _listen_unix(parsed: ParsedURI) -> socket.socket:
    path = str(parsed.path)
    try:
        os.unlink(path)
    except FileNotFoundError:
        pass
    sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    sock.bind(path)
    sock.listen(128)
    return sock


# --- Stdio ---

class StdioListener:
    """Wraps stdin/stdout as a single-connection listener."""

    def __init__(self):
        self.stdin_fd = os.dup(0)
        self.stdout_fd = os.dup(1)
        self._consumed = False

    def accept(self) -> tuple[int, int]:
        if self._consumed:
            raise StopIteration("stdio listener is single-use")
        self._consumed = True
        return self.stdin_fd, self.stdout_fd

    def close(self) -> None:
        os.close(self.stdin_fd)
        os.close(self.stdout_fd)

    @property
    def address(self) -> str:
        return "stdio://"


# --- WebSocket ---

class WSListener:
    """WebSocket listener that accepts grpc-subprotocol connections."""

    def __init__(self, parsed: ParsedURI):
        self.parsed = parsed
        self.host = parsed.host or "0.0.0.0"
        self.port = int(parsed.port or (443 if parsed.secure else 80))
        self.path = parsed.path or "/grpc"
        self._connections: "queue.Queue[Any]" = queue.Queue()
        self._server = None
        self._loop: asyncio.AbstractEventLoop | None = None
        self._thread: threading.Thread | None = None
        self._ready = threading.Event()

    def start(self) -> None:
        import websockets

        async def _handler(websocket):
            self._connections.put(websocket)
            try:
                await websocket.wait_closed()
            except Exception:
                pass

        async def _serve() -> None:
            self._server = await websockets.serve(
                _handler,
                self.host,
                self.port,
                subprotocols=["grpc"],
                process_request=self._path_guard,
            )
            sockets = getattr(self._server, "sockets", [])
            if sockets:
                self.port = sockets[0].getsockname()[1]
            self._ready.set()
            await self._server.wait_closed()

        def _run() -> None:
            self._loop = asyncio.new_event_loop()
            asyncio.set_event_loop(self._loop)
            self._loop.run_until_complete(_serve())

        self._thread = threading.Thread(target=_run, daemon=True)
        self._thread.start()
        self._ready.wait(timeout=5.0)

    def accept(self, timeout: float = 5.0):
        return self._connections.get(timeout=timeout)

    def close(self) -> None:
        if self._server and self._loop:
            self._loop.call_soon_threadsafe(self._server.close)

    @property
    def address(self) -> str:
        s = "wss" if self.parsed.secure else "ws"
        return f"{s}://{self.host}:{self.port}{self.path}"

    async def _path_guard(self, path, request_headers):  # pragma: no cover
        if path != self.path:
            return (404, [], b"not found")
        return None


def _split_host_port(addr: str, default_port: int) -> tuple[str, int]:
    if not addr:
        return "0.0.0.0", default_port

    if ":" not in addr:
        return addr, default_port

    host, _, port = addr.rpartition(":")
    host = host or "0.0.0.0"
    return host, int(port) if port else default_port
