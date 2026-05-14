from __future__ import annotations

import os
import sys
from pathlib import Path
from typing import Callable, Iterable

from support import ensure_import_paths

ensure_import_paths()

from holons import describe
from holons import observability
from holons.serve import MemberRef, ServeOptions, run_with_serve_options
from gen import describe_generated
from relay.v1 import relay_pb2, relay_pb2_grpc

# Register the generated Incode Description at startup.
describe.use_static_response(describe_generated.static_describe_response())


class RelayService(relay_pb2_grpc.RelayServiceServicer):
    def Tick(self, request, context):
        del context
        return tick(request)


def tick(request: relay_pb2.TickRequest) -> relay_pb2.TickResponse:
    obs = observability.current()
    slug = responder_slug(obs)
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
    return relay_pb2.TickResponse(
        responder_slug=slug,
        responder_instance_uid=uid,
    )


def _register(server) -> None:
    relay_pb2_grpc.add_RelayServiceServicer_to_server(RelayService(), server)


def listen_and_serve(
    listen_uri: str,
    reflect: bool = False,
    members: Iterable[MemberRef] = (),
    on_listen: Callable[[str], None] | None = None,
) -> None:
    run_with_serve_options(
        normalize_listen_uri(listen_uri),
        _register,
        ServeOptions(
            reflect=reflect,
            member_endpoints=tuple(members),
            slug="observability-cascade-python-node",
        ),
        on_listen=on_listen,
    )


def normalize_listen_uri(listen_uri: str) -> str:
    if listen_uri.startswith("tcp://:"):
        return f"tcp://0.0.0.0:{listen_uri.removeprefix('tcp://:')}"
    return listen_uri


def responder_slug(obs: observability.Observability) -> str:
    configured = obs.cfg.slug.strip()
    if configured:
        return configured
    return Path(sys.argv[0]).name or "observability-cascade-python-node"
