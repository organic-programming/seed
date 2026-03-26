"""Routing topology tests for HolonRPCServer."""

from __future__ import annotations

import asyncio
from dataclasses import dataclass, field
import inspect
from typing import Any, Callable

from holons.holonrpc import HolonRPCClient, HolonRPCError, HolonRPCServer


def _is_bridge_notification(params: dict[str, Any]) -> bool:
    return "peer" in params and ("result" in params or "error" in params)


def _clone_map(payload: dict[str, Any]) -> dict[str, Any]:
    return dict(payload)


async def _recv_params(queue: "asyncio.Queue[dict[str, Any]]", timeout: float = 2.0) -> dict[str, Any]:
    return await asyncio.wait_for(queue.get(), timeout=timeout)


def _parse_fanout_entries(result: dict[str, Any]) -> list[dict[str, Any]]:
    raw = result.get("value")
    if not isinstance(raw, list):
        raise AssertionError(f"fan-out result must be wrapped list, got {result!r}")

    entries: list[dict[str, Any]] = []
    for item in raw:
        if not isinstance(item, dict):
            raise AssertionError(f"fan-out entry must be object, got {item!r}")
        entries.append(item)
    return entries


def _assert_routing_fields_stripped(params: dict[str, Any]) -> None:
    assert "_routing" not in params
    assert "_peer" not in params


@dataclass
class _RoutingPeer:
    label: str
    on_request: Callable[[dict[str, Any]], Any] | None = None
    client: HolonRPCClient = field(default_factory=lambda: HolonRPCClient(heartbeat_interval=1.0, heartbeat_timeout=1.0))
    peer_id: str = ""
    request_count: int = 0
    notification_count: int = 0
    request_params: "asyncio.Queue[dict[str, Any]]" = field(default_factory=asyncio.Queue)
    notification_params: "asyncio.Queue[dict[str, Any]]" = field(default_factory=asyncio.Queue)

    async def connect(self, server: HolonRPCServer, url: str) -> None:
        self.client.register("Echo/Ping", self._handle_ping)
        await self.client.connect(url)
        self.peer_id = await server.wait_for_client(timeout=5.0)

    async def close(self) -> None:
        await self.client.close()

    async def _handle_ping(self, params: dict[str, Any]) -> dict[str, Any]:
        cloned = _clone_map(params)
        if _is_bridge_notification(cloned):
            self.notification_count += 1
            await self.notification_params.put(cloned)
            return {}

        self.request_count += 1
        await self.request_params.put(cloned)
        if self.on_request is not None:
            out = self.on_request(cloned)
            if inspect.isawaitable(out):
                out = await out
            return out
        return {"from": self.label, "message": cloned.get("message")}


def test_holonrpc_routing_unicast_target_peer():
    async def _run() -> None:
        server = HolonRPCServer("ws://127.0.0.1:0/rpc")
        url = await server.start()

        peer_a = _RoutingPeer("A")
        peer_b = _RoutingPeer("B")
        peer_c = _RoutingPeer("C")
        peer_d = _RoutingPeer("D")
        peers = [peer_a, peer_b, peer_c, peer_d]

        try:
            for peer in peers:
                await peer.connect(server, url)

            out = await peer_a.client.invoke(
                "Echo/Ping",
                {
                    "_peer": peer_b.peer_id,
                    "message": "hello-unicast",
                },
            )
            assert out["from"] == "B"

            params_b = await _recv_params(peer_b.request_params)
            _assert_routing_fields_stripped(params_b)

            await asyncio.sleep(0.1)
            assert peer_c.request_count == 0
            assert peer_d.request_count == 0
        finally:
            for peer in peers:
                await peer.close()
            await server.close()

    asyncio.run(_run())


def test_holonrpc_routing_fanout_aggregates_results():
    async def _run() -> None:
        server = HolonRPCServer("ws://127.0.0.1:0/rpc")
        url = await server.start()

        peer_a = _RoutingPeer("A")
        peer_b = _RoutingPeer("B")
        peer_c = _RoutingPeer("C")
        peer_d = _RoutingPeer("D")
        peers = [peer_a, peer_b, peer_c, peer_d]

        try:
            for peer in peers:
                await peer.connect(server, url)

            out = await peer_a.client.invoke("*.Echo/Ping", {"message": "hello-fanout"})
            entries = _parse_fanout_entries(out)
            assert len(entries) == 3

            seen = {entry["peer"] for entry in entries}
            assert seen == {peer_b.peer_id, peer_c.peer_id, peer_d.peer_id}
            for entry in entries:
                assert isinstance(entry.get("result"), dict)
        finally:
            for peer in peers:
                await peer.close()
            await server.close()

    asyncio.run(_run())


def test_holonrpc_routing_broadcast_response():
    async def _run() -> None:
        server = HolonRPCServer("ws://127.0.0.1:0/rpc")
        url = await server.start()

        peer_a = _RoutingPeer("A")
        peer_b = _RoutingPeer("B")
        peer_c = _RoutingPeer("C")
        peer_d = _RoutingPeer("D")
        peers = [peer_a, peer_b, peer_c, peer_d]

        try:
            for peer in peers:
                await peer.connect(server, url)

            out = await peer_a.client.invoke(
                "Echo/Ping",
                {
                    "_peer": peer_b.peer_id,
                    "_routing": "broadcast-response",
                    "message": "hello-broadcast-response",
                },
            )
            assert out["from"] == "B"

            params_b = await _recv_params(peer_b.request_params)
            _assert_routing_fields_stripped(params_b)

            notif_c = await _recv_params(peer_c.notification_params)
            notif_d = await _recv_params(peer_d.notification_params)
            for notif in (notif_c, notif_d):
                assert notif["peer"] == peer_b.peer_id
                assert isinstance(notif.get("result"), dict)

            assert peer_b.notification_count == 0
        finally:
            for peer in peers:
                await peer.close()
            await server.close()

    asyncio.run(_run())


def test_holonrpc_routing_full_broadcast():
    async def _run() -> None:
        server = HolonRPCServer("ws://127.0.0.1:0/rpc")
        url = await server.start()

        peer_a = _RoutingPeer("A")
        peer_b = _RoutingPeer("B")
        peer_c = _RoutingPeer("C")
        peer_d = _RoutingPeer("D")
        peers = [peer_a, peer_b, peer_c, peer_d]

        try:
            for peer in peers:
                await peer.connect(server, url)

            out = await peer_a.client.invoke(
                "*.Echo/Ping",
                {
                    "_routing": "full-broadcast",
                    "message": "hello-full-broadcast",
                },
            )
            entries = _parse_fanout_entries(out)
            assert len(entries) == 3

            params_b = await _recv_params(peer_b.request_params)
            params_c = await _recv_params(peer_c.request_params)
            params_d = await _recv_params(peer_d.request_params)
            for params in (params_b, params_c, params_d):
                _assert_routing_fields_stripped(params)

            for peer in (peer_b, peer_c, peer_d):
                seen_sources: set[str] = set()
                for _ in range(2):
                    notif = await _recv_params(peer.notification_params, timeout=3.0)
                    src = str(notif.get("peer"))
                    assert src != peer.peer_id
                    seen_sources.add(src)
                    assert isinstance(notif.get("result"), dict)
                assert len(seen_sources) == 2
        finally:
            for peer in peers:
                await peer.close()
            await server.close()

    asyncio.run(_run())


def test_holonrpc_routing_full_broadcast_requires_fanout_method():
    async def _run() -> None:
        server = HolonRPCServer("ws://127.0.0.1:0/rpc")
        url = await server.start()

        peer_a = _RoutingPeer("A")
        peer_b = _RoutingPeer("B")
        peers = [peer_a, peer_b]

        try:
            for peer in peers:
                await peer.connect(server, url)

            try:
                await peer_a.client.invoke(
                    "Echo/Ping",
                    {
                        "_peer": peer_b.peer_id,
                        "_routing": "full-broadcast",
                        "message": "invalid",
                    },
                )
                assert False, "expected HolonRPCError"
            except HolonRPCError as exc:
                assert exc.code == -32602
                assert "full-broadcast requires a fan-out method" in exc.message
        finally:
            for peer in peers:
                await peer.close()
            await server.close()

    asyncio.run(_run())
