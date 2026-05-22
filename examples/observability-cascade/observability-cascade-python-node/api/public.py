from __future__ import annotations

from support import ensure_import_paths

ensure_import_paths()

from _internal import server
from relay.v1 import relay_pb2


def tick(request: relay_pb2.TickRequest) -> relay_pb2.TickResponse:
    return server.tick(request)
