"""Canonical RelayService implementation for cascade examples."""

from __future__ import annotations

import os
import sys
import threading
from pathlib import Path

import grpc

from holons import observability

_SDK_GEN_ROOT = Path(__file__).resolve().parents[1] / "gen" / "python"
if _SDK_GEN_ROOT.is_dir() and str(_SDK_GEN_ROOT) not in sys.path:
    sys.path.insert(0, str(_SDK_GEN_ROOT))

from relay.v1 import relay_pb2, relay_pb2_grpc  # noqa: E402


class RelayOptions:
    def __init__(self, downstream_conn: grpc.Channel | None = None) -> None:
        self.downstream_conn = downstream_conn


class _RelayService(relay_pb2_grpc.RelayServiceServicer):
    def __init__(self, downstream_conn: grpc.Channel | None = None) -> None:
        self._downstream_conn = downstream_conn
        self._received = 0
        self._lock = threading.Lock()

    def Tick(self, request, context):
        del context
        with self._lock:
            self._received += 1
            received = self._received

        obs = observability.current()
        slug = _responder_slug(obs)
        uid = obs.cfg.instance_uid
        obs.logger("tick").info(
            "tick received",
            sender=request.sender,
            note=request.note,
            responder_slug=slug,
            responder_uid=uid,
        )
        counter = obs.counter(
            "cascade_ticks_total",
            "Ticks received by this cascade node.",
            {"responder_uid": uid},
        )
        if counter is not None:
            counter.inc()

        hops = []
        if self._downstream_conn is not None:
            downstream = relay_pb2_grpc.RelayServiceStub(self._downstream_conn)
            response = downstream.Tick(request, timeout=5.0)
            hops.extend(response.hops)
        hops.append(relay_pb2.HopReceipt(slug=slug, uid=uid, received=received))
        return relay_pb2.TickResponse(
            responder_slug=slug,
            responder_instance_uid=uid,
            hops=hops,
        )


def register_server(server: grpc.Server, options: RelayOptions | None = None) -> None:
    options = options or RelayOptions()
    relay_pb2_grpc.add_RelayServiceServicer_to_server(
        _RelayService(options.downstream_conn),
        server,
    )


def _responder_slug(obs: observability.Observability) -> str:
    configured = obs.cfg.slug.strip()
    if configured:
        return configured
    if sys.argv:
        return Path(sys.argv[0]).name
    return ""


__all__ = ["RelayOptions", "register_server", "relay_pb2", "relay_pb2_grpc"]
