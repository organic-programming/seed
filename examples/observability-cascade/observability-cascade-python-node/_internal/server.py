from __future__ import annotations

import sys
from pathlib import Path
from typing import Callable

import grpc

from support import ensure_import_paths

ensure_import_paths()

from holons import describe, relay
from holons.serve import ServeOptions, run_with_serve_options
from gen import describe_generated

describe.use_static_response(describe_generated.static_describe_response())


def listen_and_serve(
    listen_uri: str,
    reflect: bool = False,
    downstream_conn: grpc.Channel | None = None,
    on_listen: Callable[[str], None] | None = None,
) -> None:
    def register(server) -> None:
        relay.register_server(server, relay.RelayOptions(downstream_conn))

    run_with_serve_options(
        normalize_listen_uri(listen_uri),
        register,
        ServeOptions(reflect=reflect, slug="observability-cascade-python-node"),
        on_listen=on_listen,
    )


def normalize_listen_uri(listen_uri: str) -> str:
    if listen_uri.startswith("tcp://:"):
        return f"tcp://0.0.0.0:{listen_uri.removeprefix('tcp://:')}"
    return listen_uri
