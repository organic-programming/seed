from __future__ import annotations

from typing import Callable

from support import ensure_import_paths

ensure_import_paths()

from holons.serve import run_with_options
from v1 import greeting_pb2_grpc

from api import public


class GreetingService(greeting_pb2_grpc.GreetingServiceServicer):
    def ListLanguages(self, request, context):
        del context
        return public.list_languages(request)

    def SayHello(self, request, context):
        del context
        return public.say_hello(request)


def _register(server) -> None:
    greeting_pb2_grpc.add_GreetingServiceServicer_to_server(GreetingService(), server)


def listen_and_serve(
    listen_uri: str,
    reflect: bool = False,
    on_listen: Callable[[str], None] | None = None,
) -> None:
    run_with_options(
        normalize_listen_uri(listen_uri),
        _register,
        reflect=reflect,
        on_listen=on_listen,
    )


def normalize_listen_uri(listen_uri: str) -> str:
    if listen_uri.startswith("tcp://:"):
        return f"tcp://0.0.0.0:{listen_uri.removeprefix('tcp://:')}"
    return listen_uri
