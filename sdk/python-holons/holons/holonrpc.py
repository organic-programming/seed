from __future__ import annotations

"""Holon-RPC client (JSON-RPC 2.0 over WebSocket).

Protocol constraints (COMMUNICATION.md §4):
- WebSocket subprotocol: holon-rpc
- JSON-RPC version field: "2.0"
- Bidirectional requests (server-initiated IDs must start with "s")
"""

from collections.abc import Awaitable, Callable
import asyncio
from dataclasses import dataclass, field
import inspect
import json
import logging
import random
from typing import Any, Union
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode, urlparse, urlunparse
from urllib.request import Request, urlopen

import websockets
from websockets.exceptions import ConnectionClosed

logger = logging.getLogger("holons.holonrpc")

JsonObject = dict[str, Any]
RPCHandler = Callable[[JsonObject], Union[JsonObject, Awaitable[JsonObject]]]


class HolonRPCError(Exception):
    """Raised when a JSON-RPC error response is received."""

    def __init__(self, code: int, message: str, data: Any | None = None):
        super().__init__(f"rpc error {code}: {message}")
        self.code = int(code)
        self.message = str(message)
        self.data = data


@dataclass(frozen=True)
class SSEEvent:
    """One decoded Server-Sent Event from the HTTP+SSE transport."""

    event: str
    id: str = ""
    result: JsonObject = field(default_factory=dict)
    error: HolonRPCError | None = None


class HTTPClient:
    """Holon-RPC client for unary and server-streaming HTTP+SSE calls."""

    def __init__(self, base_url: str, *, ssl_context: Any | None = None, timeout: float = 10.0):
        self._base_url = _normalize_http_base_url(base_url)
        self._ssl_context = ssl_context
        self._timeout = float(timeout)

    def invoke(
        self,
        method: str,
        params: JsonObject | None = None,
        *,
        timeout: float | None = None,
    ) -> JsonObject:
        request = Request(
            self._method_url(method),
            data=_marshal_http_params(params),
            method="POST",
            headers={
                "Content-Type": "application/json",
                "Accept": "application/json",
            },
        )
        status, payload = self._request_bytes(request, timeout=timeout)
        return _decode_http_rpc_response(status, payload)

    def stream(
        self,
        method: str,
        params: JsonObject | None = None,
        *,
        timeout: float | None = None,
    ) -> list[SSEEvent]:
        request = Request(
            self._method_url(method),
            data=_marshal_http_params(params),
            method="POST",
            headers={
                "Content-Type": "application/json",
                "Accept": "text/event-stream",
            },
        )
        return self._read_sse(request, timeout=timeout)

    def stream_query(
        self,
        method: str,
        params: dict[str, str] | None = None,
        *,
        timeout: float | None = None,
    ) -> list[SSEEvent]:
        endpoint = self._method_url(method)
        if params:
            endpoint = endpoint + "?" + urlencode(params, doseq=True)
        request = Request(
            endpoint,
            method="GET",
            headers={"Accept": "text/event-stream"},
        )
        return self._read_sse(request, timeout=timeout)

    def _method_url(self, method: str) -> str:
        return self._base_url.rstrip("/") + "/" + method.strip().strip("/")

    def _request_bytes(
        self,
        request: Request,
        *,
        timeout: float | None = None,
    ) -> tuple[int, bytes]:
        try:
            with urlopen(request, timeout=timeout or self._timeout, context=self._ssl_context) as response:
                return int(response.status), response.read()
        except HTTPError as exc:
            return int(exc.code), exc.read()
        except URLError as exc:
            raise ConnectionError(f"http request failed: {exc.reason}") from exc

    def _open_stream(self, request: Request, *, timeout: float | None = None):
        try:
            return urlopen(request, timeout=timeout or self._timeout, context=self._ssl_context)
        except HTTPError as exc:
            status = int(exc.code)
            payload = exc.read()
            _decode_http_rpc_response(status, payload)
            raise ConnectionError(f"http stream failed with status {status}") from exc
        except URLError as exc:
            raise ConnectionError(f"http stream failed: {exc.reason}") from exc

    def _read_sse(self, request: Request, *, timeout: float | None = None) -> list[SSEEvent]:
        with self._open_stream(request, timeout=timeout) as response:
            status = int(response.status)
            if status >= 400:
                payload = response.read()
                _decode_http_rpc_response(status, payload)
                raise ConnectionError(f"http stream failed with status {status}")
            return _read_sse_events(response)


_ROUTE_MODE_DEFAULT = ""
_ROUTE_MODE_BROADCAST_RESPONSE = "broadcast-response"
_ROUTE_MODE_FULL_BROADCAST = "full-broadcast"
_MAX_HOLONRPC_MESSAGE_BYTES = 1 << 20


@dataclass(frozen=True)
class _RouteHints:
    target_peer_id: str = ""
    mode: str = _ROUTE_MODE_DEFAULT


def _parse_route_hints(
    method: str,
    params: JsonObject,
) -> tuple[str, bool, JsonObject, _RouteHints]:
    dispatch_method = method.strip()
    if not dispatch_method:
        raise HolonRPCError(-32600, "invalid request")

    cleaned = dict(params)

    mode = _ROUTE_MODE_DEFAULT
    if "_routing" in cleaned:
        raw_mode = cleaned.pop("_routing")
        if not isinstance(raw_mode, str):
            raise HolonRPCError(-32602, "_routing must be a string")
        mode = raw_mode.strip()
        if mode not in {
            _ROUTE_MODE_DEFAULT,
            _ROUTE_MODE_BROADCAST_RESPONSE,
            _ROUTE_MODE_FULL_BROADCAST,
        }:
            raise HolonRPCError(-32602, f"unsupported _routing {mode!r}")

    target_peer_id = ""
    if "_peer" in cleaned:
        raw_peer = cleaned.pop("_peer")
        if not isinstance(raw_peer, str):
            raise HolonRPCError(-32602, "_peer must be a string")
        target_peer_id = raw_peer.strip()
        if not target_peer_id:
            raise HolonRPCError(-32602, "_peer must be non-empty")

    fan_out = dispatch_method.startswith("*.")
    if fan_out:
        dispatch_method = dispatch_method[2:].strip()
        if not dispatch_method:
            raise HolonRPCError(-32600, "invalid fan-out method")

    if mode == _ROUTE_MODE_FULL_BROADCAST and not fan_out:
        raise HolonRPCError(-32602, "full-broadcast requires a fan-out method")

    return dispatch_method, fan_out, cleaned, _RouteHints(target_peer_id=target_peer_id, mode=mode)


def _to_rpc_error(exc: Exception) -> HolonRPCError:
    if isinstance(exc, HolonRPCError):
        return exc

    if isinstance(exc, asyncio.CancelledError):
        message = str(exc) or "canceled"
        return HolonRPCError(1, message)

    if isinstance(exc, (asyncio.TimeoutError, TimeoutError)):
        message = str(exc) or "deadline exceeded"
        return HolonRPCError(4, message)

    message = str(exc) or "unavailable"
    return HolonRPCError(14, message)


def _rpc_error_payload(exc: Exception) -> JsonObject:
    rpc_err = _to_rpc_error(exc)
    payload: JsonObject = {
        "code": rpc_err.code,
        "message": rpc_err.message,
    }
    if rpc_err.data is not None:
        payload["data"] = rpc_err.data
    return payload


def _normalize_http_base_url(base_url: str) -> str:
    trimmed = base_url.strip().rstrip("/")
    if not trimmed:
        raise ValueError("base_url is required")

    parsed = urlparse(trimmed)
    if parsed.scheme == "rest+sse":
        parsed = parsed._replace(scheme="http")
    if parsed.scheme not in {"http", "https"}:
        raise ValueError(f"HTTPClient requires http://, https://, or rest+sse:// URL, got {base_url!r}")
    return urlunparse(parsed).rstrip("/")


def _marshal_http_params(params: JsonObject | None) -> bytes:
    return json.dumps(params or {}, separators=(",", ":")).encode("utf-8")


def _decode_http_rpc_response(status: int, payload: bytes) -> JsonObject:
    text = payload.decode("utf-8", errors="replace").strip()
    if text:
        try:
            message = json.loads(text)
        except json.JSONDecodeError:
            message = None
        if isinstance(message, dict):
            error_payload = message.get("error")
            if isinstance(error_payload, dict):
                raise _rpc_error_from_payload(error_payload)
            if "result" in message:
                result = message.get("result")
                if isinstance(result, dict):
                    return result
                return {"value": result}
            return message

    if status >= 400:
        raise ConnectionError(f"http status {status}")
    return {}


def _rpc_error_from_payload(payload: dict[str, Any]) -> HolonRPCError:
    return HolonRPCError(
        int(payload.get("code", 13)),
        str(payload.get("message", "internal error")),
        payload.get("data"),
    )


def _read_sse_events(response: Any) -> list[SSEEvent]:
    events: list[SSEEvent] = []
    current_event = ""
    current_id = ""
    current_data: list[str] = []

    def flush() -> bool:
        nonlocal current_event, current_id, current_data
        if not current_event and not current_id and not current_data:
            return False

        event = SSEEvent(event=current_event, id=current_id)
        data = "\n".join(current_data).strip()
        if current_event in {"message", "error"} and data:
            payload = json.loads(data)
            if isinstance(payload, dict) and isinstance(payload.get("error"), dict):
                event = SSEEvent(
                    event=current_event,
                    id=current_id,
                    error=_rpc_error_from_payload(payload["error"]),
                )
            elif isinstance(payload, dict):
                result = payload.get("result", {})
                if isinstance(result, dict):
                    event = SSEEvent(event=current_event, id=current_id, result=result)
                else:
                    event = SSEEvent(event=current_event, id=current_id, result={"value": result})

        events.append(event)
        stop = current_event == "done"
        current_event = ""
        current_id = ""
        current_data = []
        return stop

    for raw_line in response:
        line = raw_line.decode("utf-8", errors="replace").rstrip("\r\n")
        if line == "":
            if flush():
                break
            continue
        if line.startswith("event:"):
            current_event = line.removeprefix("event:").strip()
        elif line.startswith("id:"):
            current_id = line.removeprefix("id:").strip()
        elif line.startswith("data:"):
            current_data.append(line.removeprefix("data:").strip())

    flush()
    return events


class HolonRPCClient:
    """Bidirectional Holon-RPC client with heartbeat and auto-reconnect."""

    def __init__(
        self,
        *,
        heartbeat_interval: float = 15.0,
        heartbeat_timeout: float = 5.0,
        reconnect_min_delay: float = 0.5,
        reconnect_max_delay: float = 30.0,
        reconnect_factor: float = 2.0,
        reconnect_jitter: float = 0.1,
    ):
        self._url: str | None = None
        self._ws: Any | None = None
        self._handlers: dict[str, RPCHandler] = {}
        self._pending: dict[str, asyncio.Future[Any]] = {}

        self._next_client_id = 0
        self._closed = False

        self._send_lock = asyncio.Lock()
        self._state_lock = asyncio.Lock()
        self._connected = asyncio.Event()

        self._receiver_task: asyncio.Task[None] | None = None
        self._heartbeat_task: asyncio.Task[None] | None = None
        self._reconnect_task: asyncio.Task[None] | None = None

        self._heartbeat_interval = float(heartbeat_interval)
        self._heartbeat_timeout = float(heartbeat_timeout)

        self._reconnect_min_delay = float(reconnect_min_delay)
        self._reconnect_max_delay = float(reconnect_max_delay)
        self._reconnect_factor = float(reconnect_factor)
        self._reconnect_jitter = float(reconnect_jitter)

    async def connect(self, url: str) -> None:
        """Open a WebSocket using the required `holon-rpc` subprotocol."""
        self._url = url
        self._closed = False

        await self._connect_once()
        self._start_runtime_tasks()

    async def invoke(
        self,
        method: str,
        params: JsonObject | None = None,
        *,
        timeout: float | None = None,
    ) -> JsonObject:
        """Send a JSON-RPC request and wait for its response."""
        if not method:
            raise ValueError("method is required")

        ws = await self._ensure_connected()

        self._next_client_id += 1
        req_id = f"c{self._next_client_id}"

        payload: JsonObject = {
            "jsonrpc": "2.0",
            "id": req_id,
            "method": method,
            "params": params if params is not None else {},
        }

        loop = asyncio.get_running_loop()
        fut: asyncio.Future[Any] = loop.create_future()
        self._pending[req_id] = fut

        try:
            await self._send_json(ws, payload)
            if timeout is None:
                result = await fut
            else:
                result = await asyncio.wait_for(fut, timeout=timeout)
            if isinstance(result, dict):
                return result
            return {"value": result}
        finally:
            self._pending.pop(req_id, None)

    def register(self, method: str, handler: RPCHandler) -> None:
        """Register a handler for server-initiated requests."""
        if not method:
            raise ValueError("method is required")
        self._handlers[method] = handler

    async def close(self) -> None:
        """Gracefully close connection and stop background tasks."""
        self._closed = True

        reconnect = self._reconnect_task
        self._reconnect_task = None
        if reconnect:
            reconnect.cancel()

        hb = self._heartbeat_task
        self._heartbeat_task = None
        if hb:
            hb.cancel()

        receiver = self._receiver_task
        self._receiver_task = None
        if receiver:
            receiver.cancel()

        async with self._state_lock:
            ws = self._ws
            self._ws = None
            self._connected.clear()

        if ws is not None:
            try:
                await ws.close()
            except Exception:
                pass

        self._fail_pending(ConnectionError("holon-rpc client closed"))

    async def _connect_once(self) -> None:
        assert self._url is not None

        ws = await websockets.connect(
            self._url,
            subprotocols=["holon-rpc"],
            ping_interval=None,
            ping_timeout=None,
            max_size=_MAX_HOLONRPC_MESSAGE_BYTES,
        )

        if ws.subprotocol != "holon-rpc":
            await ws.close(code=1002, reason="missing holon-rpc subprotocol")
            raise ConnectionError("server did not negotiate holon-rpc subprotocol")

        async with self._state_lock:
            self._ws = ws
            self._connected.set()

    async def _ensure_connected(self) -> Any:
        if self._connected.is_set() and self._ws is not None:
            return self._ws

        if self._closed:
            raise ConnectionError("holon-rpc client is closed")

        await asyncio.wait_for(self._connected.wait(), timeout=self._reconnect_max_delay + 5.0)
        if self._ws is None:
            raise ConnectionError("connection unavailable")
        return self._ws

    def _start_runtime_tasks(self) -> None:
        if self._receiver_task and not self._receiver_task.done():
            return
        self._receiver_task = asyncio.create_task(self._receiver_loop(), name="holonrpc-recv")
        self._heartbeat_task = asyncio.create_task(self._heartbeat_loop(), name="holonrpc-heartbeat")

    async def _receiver_loop(self) -> None:
        while not self._closed:
            ws = self._ws
            if ws is None:
                return

            try:
                message = await ws.recv()
            except ConnectionClosed as exc:
                logger.debug("holon-rpc disconnected: %s", exc)
                await self._handle_disconnect()
                return
            except asyncio.CancelledError:
                return

            if not isinstance(message, str):
                continue

            try:
                payload = json.loads(message)
            except json.JSONDecodeError:
                continue

            if not isinstance(payload, dict):
                continue

            if "method" in payload:
                await self._handle_request(payload)
            elif "result" in payload or "error" in payload:
                self._handle_response(payload)

    async def _handle_request(self, payload: JsonObject) -> None:
        method = payload.get("method")
        req_id = payload.get("id")
        params = payload.get("params", {})

        if payload.get("jsonrpc") != "2.0" or not isinstance(method, str):
            if req_id is not None:
                await self._send_error(req_id, -32600, "invalid request")
            return

        if method == "rpc.heartbeat":
            if req_id is not None:
                await self._send_result(req_id, {})
            return

        if req_id is not None:
            sid = str(req_id)
            if not sid.startswith("s"):
                await self._send_error(req_id, -32600, "server request id must start with 's'")
                return

        handler = self._handlers.get(method)
        if handler is None:
            if req_id is not None:
                await self._send_error(req_id, -32601, f"method {method!r} not found")
            return

        if not isinstance(params, dict):
            if req_id is not None:
                await self._send_error(req_id, -32602, "params must be an object")
            return

        try:
            result = handler(params)
            if inspect.isawaitable(result):
                result = await result
        except HolonRPCError as exc:
            if req_id is not None:
                await self._send_error(req_id, exc.code, exc.message, data=exc.data)
            return
        except Exception as exc:  # pragma: no cover - defensive fallback
            if req_id is not None:
                await self._send_error(req_id, 13, str(exc))
            return

        if req_id is not None:
            if isinstance(result, dict):
                await self._send_result(req_id, result)
            else:
                await self._send_result(req_id, {"value": result})

    def _handle_response(self, payload: JsonObject) -> None:
        req_id = payload.get("id")
        if req_id is None:
            return

        fut = self._pending.get(str(req_id))
        if fut is None or fut.done():
            return

        if payload.get("jsonrpc") != "2.0":
            fut.set_exception(HolonRPCError(-32600, "invalid response"))
            return

        if "error" in payload:
            error = payload.get("error") or {}
            code = int(error.get("code", -32603))
            message = str(error.get("message", "internal error"))
            fut.set_exception(HolonRPCError(code, message, error.get("data")))
            return

        fut.set_result(payload.get("result", {}))

    async def _send_json(self, ws: Any, payload: JsonObject) -> None:
        async with self._send_lock:
            await ws.send(json.dumps(payload, separators=(",", ":")))

    async def _send_result(self, req_id: Any, result: JsonObject) -> None:
        ws = self._ws
        if ws is None:
            return
        await self._send_json(
            ws,
            {
                "jsonrpc": "2.0",
                "id": req_id,
                "result": result,
            },
        )

    async def _send_error(self, req_id: Any, code: int, message: str, data: Any | None = None) -> None:
        ws = self._ws
        if ws is None:
            return
        err: JsonObject = {"code": int(code), "message": str(message)}
        if data is not None:
            err["data"] = data
        await self._send_json(
            ws,
            {
                "jsonrpc": "2.0",
                "id": req_id,
                "error": err,
            },
        )

    async def _heartbeat_loop(self) -> None:
        while not self._closed:
            await asyncio.sleep(self._heartbeat_interval)
            if self._closed:
                return

            try:
                await self.invoke(
                    "rpc.heartbeat",
                    {},
                    timeout=self._heartbeat_timeout,
                )
            except Exception:
                ws = self._ws
                if ws is not None:
                    try:
                        await ws.close(code=1001, reason="heartbeat timeout")
                    except Exception:
                        pass
                await self._handle_disconnect()
                return

    async def _handle_disconnect(self) -> None:
        async with self._state_lock:
            ws = self._ws
            self._ws = None
            self._connected.clear()

        if ws is not None:
            try:
                await ws.close()
            except Exception:
                pass

        self._fail_pending(ConnectionError("holon-rpc connection closed"))

        if not self._closed and (self._reconnect_task is None or self._reconnect_task.done()):
            self._reconnect_task = asyncio.create_task(self._reconnect_loop(), name="holonrpc-reconnect")

    async def _reconnect_loop(self) -> None:
        attempt = 0
        while not self._closed:
            try:
                await self._connect_once()
                self._start_runtime_tasks()
                return
            except Exception as exc:
                logger.debug("holon-rpc reconnect failed: %s", exc)

            base = min(
                self._reconnect_min_delay * (self._reconnect_factor ** attempt),
                self._reconnect_max_delay,
            )
            delay = base * (1.0 + random.random() * self._reconnect_jitter)
            await asyncio.sleep(delay)
            attempt += 1

    def _fail_pending(self, exc: Exception) -> None:
        pending = list(self._pending.values())
        self._pending.clear()
        for fut in pending:
            if not fut.done():
                fut.set_exception(exc)


@dataclass
class _ServerPeer:
    id: str
    websocket: Any
    pending: dict[str, asyncio.Future[Any]] = field(default_factory=dict)


class HolonRPCServer:
    """Holon-RPC server (JSON-RPC 2.0 over WebSocket) with bidirectional calls."""

    def __init__(self, url: str = "ws://127.0.0.1:0/rpc", *, ssl_context: Any | None = None):
        self._url = url
        self._ssl_context = ssl_context

        self._handlers: dict[str, RPCHandler] = {}
        self._clients: dict[str, _ServerPeer] = {}
        self._connections: "asyncio.Queue[str]" = asyncio.Queue()

        self._server: Any | None = None
        self._next_client_id = 0
        self._next_server_id = 0
        self._send_lock = asyncio.Lock()
        self._closed = False
        self.address = url
        self._path = "/rpc"

    def register(self, method: str, handler: RPCHandler) -> None:
        if not method:
            raise ValueError("method is required")
        self._handlers[method] = handler

    def unregister(self, method: str) -> None:
        self._handlers.pop(method, None)

    def client_ids(self) -> list[str]:
        return list(self._clients.keys())

    async def wait_for_client(self, timeout: float = 5.0) -> str:
        return await asyncio.wait_for(self._connections.get(), timeout=timeout)

    async def start(self) -> str:
        if self._server is not None:
            return self.address

        parsed = urlparse(self._url)
        if parsed.scheme not in {"ws", "wss"}:
            raise ValueError(f"holon-rpc server requires ws:// or wss:// URL, got {self._url!r}")
        if parsed.scheme == "wss" and self._ssl_context is None:
            raise ValueError("wss:// holon-rpc server requires ssl_context")

        host = parsed.hostname or "127.0.0.1"
        port = parsed.port if parsed.port is not None else (443 if parsed.scheme == "wss" else 80)
        self._path = parsed.path or "/rpc"

        self._closed = False
        self._server = await websockets.serve(
            self._handle_connection,
            host,
            port,
            subprotocols=["holon-rpc"],
            ping_interval=None,
            ping_timeout=None,
            max_size=_MAX_HOLONRPC_MESSAGE_BYTES,
            ssl=self._ssl_context if parsed.scheme == "wss" else None,
        )

        sockets = getattr(self._server, "sockets", [])
        if sockets:
            bound_port = sockets[0].getsockname()[1]
        else:
            bound_port = port
        self.address = f"{parsed.scheme}://{host}:{bound_port}{self._path}"
        return self.address

    async def close(self) -> None:
        self._closed = True
        peers = list(self._clients.values())
        self._clients.clear()

        for peer in peers:
            self._fail_peer_pending(peer, ConnectionError("holon-rpc server closed"))
            try:
                await peer.websocket.close(code=1001, reason="server shutdown")
            except Exception:
                pass

        if self._server is not None:
            self._server.close()
            await self._server.wait_closed()
            self._server = None

    async def invoke(
        self,
        client_id: str,
        method: str,
        params: JsonObject | None = None,
        *,
        timeout: float = 5.0,
    ) -> JsonObject:
        peer = self._clients.get(client_id)
        if peer is None:
            raise ConnectionError(f"unknown client: {client_id}")
        if not method:
            raise ValueError("method is required")

        self._next_server_id += 1
        req_id = f"s{self._next_server_id}"
        loop = asyncio.get_running_loop()
        fut: asyncio.Future[Any] = loop.create_future()
        peer.pending[req_id] = fut

        payload: JsonObject = {
            "jsonrpc": "2.0",
            "id": req_id,
            "method": method,
            "params": params if params is not None else {},
        }

        try:
            await self._send_json(peer.websocket, payload)
            out = await asyncio.wait_for(fut, timeout=timeout)
            if isinstance(out, dict):
                return out
            return {"value": out}
        finally:
            peer.pending.pop(req_id, None)

    async def _handle_connection(self, websocket: Any) -> None:
        path = getattr(websocket, "path", None)
        if not path:
            request = getattr(websocket, "request", None)
            path = getattr(request, "path", None)
        if not path:
            path = self._path
        if path != self._path:
            await websocket.close(code=1008, reason="invalid path")
            return

        protocol = websocket.subprotocol or ""
        if protocol != "holon-rpc":
            await websocket.close(code=1002, reason="missing holon-rpc subprotocol")
            return

        self._next_client_id += 1
        client_id = f"c{self._next_client_id}"
        peer = _ServerPeer(id=client_id, websocket=websocket)
        self._clients[client_id] = peer
        self._connections.put_nowait(client_id)

        try:
            async for message in websocket:
                if not isinstance(message, str):
                    continue
                await self._handle_message(peer, message)
        except ConnectionClosed:
            pass
        finally:
            self._clients.pop(client_id, None)
            self._fail_peer_pending(peer, ConnectionError("holon-rpc connection closed"))

    async def _handle_message(self, peer: _ServerPeer, raw: str) -> None:
        try:
            payload = json.loads(raw)
        except json.JSONDecodeError:
            await self._send_error(peer, None, -32700, "parse error")
            return

        if not isinstance(payload, dict):
            await self._send_error(peer, None, -32600, "invalid request")
            return

        if "method" in payload:
            await self._handle_request(peer, payload)
            return
        if "result" in payload or "error" in payload:
            self._handle_response(peer, payload)
            return

        await self._send_error(peer, payload.get("id"), -32600, "invalid request")

    async def _handle_request(self, peer: _ServerPeer, payload: JsonObject) -> None:
        method = payload.get("method")
        req_id = payload.get("id")

        if payload.get("jsonrpc") != "2.0":
            if req_id is not None:
                await self._send_error(peer, req_id, -32600, "invalid request")
            return

        if not isinstance(method, str) or not method:
            if req_id is not None:
                await self._send_error(peer, req_id, -32600, "invalid request")
            return

        if method == "rpc.heartbeat":
            if req_id is not None:
                await self._send_result(peer, req_id, {})
            return

        params = payload.get("params", {})
        if not isinstance(params, dict):
            if req_id is not None:
                await self._send_error(peer, req_id, -32602, "params must be an object")
            return

        try:
            dispatch_method, fan_out, cleaned_params, routing_hints = _parse_route_hints(method, params)
        except HolonRPCError as exc:
            if req_id is not None:
                await self._send_error(peer, req_id, exc.code, exc.message, data=exc.data)
            return

        routed = await self._route_peer_request(
            peer,
            req_id,
            dispatch_method,
            cleaned_params,
            routing_hints,
            fan_out,
        )
        if routed:
            return

        handler = self._handlers.get(dispatch_method)
        if handler is None:
            if req_id is not None:
                await self._send_error(peer, req_id, -32601, f"method {dispatch_method!r} not found")
            return

        try:
            result = handler(cleaned_params)
            if inspect.isawaitable(result):
                result = await result
        except HolonRPCError as exc:
            if req_id is not None:
                await self._send_error(peer, req_id, exc.code, exc.message, data=exc.data)
            return
        except Exception as exc:  # pragma: no cover - defensive fallback
            if req_id is not None:
                await self._send_error(peer, req_id, 13, str(exc))
            return

        if req_id is not None:
            if isinstance(result, dict):
                await self._send_result(peer, req_id, result)
            else:
                await self._send_result(peer, req_id, {"value": result})

    def _handle_response(self, peer: _ServerPeer, payload: JsonObject) -> None:
        req_id = payload.get("id")
        if req_id is None:
            return

        fut = peer.pending.get(str(req_id))
        if fut is None or fut.done():
            return

        if payload.get("jsonrpc") != "2.0":
            fut.set_exception(HolonRPCError(-32600, "invalid response"))
            return

        if "error" in payload:
            error = payload.get("error") or {}
            code = int(error.get("code", -32603))
            message = str(error.get("message", "internal error"))
            fut.set_exception(HolonRPCError(code, message, error.get("data")))
            return

        fut.set_result(payload.get("result", {}))

    async def _route_peer_request(
        self,
        caller: _ServerPeer,
        req_id: Any,
        method: str,
        params: JsonObject,
        hints: _RouteHints,
        fan_out: bool,
    ) -> bool:
        if fan_out:
            try:
                entries = await self._dispatch_fanout(caller, method, params)
            except HolonRPCError as exc:
                if req_id is not None:
                    await self._send_error(caller, req_id, exc.code, exc.message, data=exc.data)
                return True

            if hints.mode == _ROUTE_MODE_FULL_BROADCAST:
                for entry in entries:
                    source_peer = str(entry.get("peer", ""))
                    notify_payload: JsonObject = {"peer": source_peer}
                    if "error" in entry:
                        notify_payload["error"] = entry["error"]
                    else:
                        notify_payload["result"] = entry.get("result", {})
                    await self._broadcast_notification_many(
                        {caller.id, source_peer},
                        method,
                        notify_payload,
                    )

            if req_id is not None:
                await self._send_result(caller, req_id, entries)
            return True

        target_peer_id = hints.target_peer_id
        if not target_peer_id:
            return False

        if not self._peer_exists(target_peer_id):
            if req_id is not None:
                await self._send_error(caller, req_id, 5, f"peer {target_peer_id!r} not found")
            return True

        try:
            out = await self.invoke(target_peer_id, method, params)
        except Exception as exc:
            rpc_err = _to_rpc_error(exc)
            if req_id is not None:
                await self._send_error(caller, req_id, rpc_err.code, rpc_err.message, data=rpc_err.data)
            return True

        if hints.mode == _ROUTE_MODE_BROADCAST_RESPONSE:
            await self._broadcast_notification_many(
                {caller.id, target_peer_id},
                method,
                {
                    "peer": target_peer_id,
                    "result": out,
                },
            )

        if req_id is not None:
            await self._send_result(caller, req_id, out)
        return True

    async def _dispatch_fanout(
        self,
        caller: _ServerPeer,
        method: str,
        params: JsonObject,
    ) -> list[JsonObject]:
        target_peer_ids = self._snapshot_peer_ids_excluding(caller.id)
        if not target_peer_ids:
            raise HolonRPCError(5, "no connected peers")

        async def _invoke_target(target_peer_id: str) -> JsonObject:
            entry: JsonObject = {"peer": target_peer_id}
            try:
                entry["result"] = await self.invoke(target_peer_id, method, params)
            except Exception as exc:
                entry["error"] = _rpc_error_payload(exc)
            return entry

        tasks = [asyncio.create_task(_invoke_target(peer_id)) for peer_id in target_peer_ids]
        entries: list[JsonObject] = []
        for task in asyncio.as_completed(tasks):
            entries.append(await task)
        return entries

    def _snapshot_peer_ids_excluding(self, excluded_peer_id: str) -> list[str]:
        return [peer_id for peer_id in self._clients.keys() if peer_id != excluded_peer_id]

    def _peer_exists(self, peer_id: str) -> bool:
        return peer_id in self._clients

    async def _broadcast_notification_many(
        self,
        excluded_peer_ids: set[str],
        method: str,
        params: JsonObject,
    ) -> None:
        payload: JsonObject = {
            "jsonrpc": "2.0",
            "method": method,
            "params": params,
        }

        peers = [
            peer
            for peer_id, peer in self._clients.items()
            if peer_id not in excluded_peer_ids
        ]
        for peer in peers:
            try:
                await self._send_json(peer.websocket, payload)
            except Exception:
                continue

    async def _send_result(self, peer: _ServerPeer, req_id: Any, result: Any) -> None:
        payload: JsonObject = {
            "jsonrpc": "2.0",
            "id": req_id,
            "result": result,
        }
        await self._send_json(peer.websocket, payload)

    async def _send_error(
        self,
        peer: _ServerPeer,
        req_id: Any,
        code: int,
        message: str,
        *,
        data: Any | None = None,
    ) -> None:
        err: JsonObject = {"code": int(code), "message": str(message)}
        if data is not None:
            err["data"] = data

        payload: JsonObject = {
            "jsonrpc": "2.0",
            "id": req_id,
            "error": err,
        }
        await self._send_json(peer.websocket, payload)

    async def _send_json(self, ws: Any, payload: JsonObject) -> None:
        async with self._send_lock:
            await ws.send(json.dumps(payload, separators=(",", ":")))

    def _fail_peer_pending(self, peer: _ServerPeer, exc: Exception) -> None:
        pending = list(peer.pending.values())
        peer.pending.clear()
        for fut in pending:
            if not fut.done():
                fut.set_exception(exc)
