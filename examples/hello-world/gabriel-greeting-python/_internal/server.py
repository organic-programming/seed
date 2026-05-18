from __future__ import annotations

import time
from typing import Callable

from support import ensure_import_paths

ensure_import_paths()

from holons import describe, observability
from holons.serve import ServeOptions, run_with_serve_options
from gen import describe_generated
from v1 import greeting_pb2_grpc

from api import public
from _internal.greetings import lookup

# Register the generated Incode Description at startup.
describe.use_static_response(describe_generated.static_describe_response())


class GreetingService(greeting_pb2_grpc.GreetingServiceServicer):
    def ListLanguages(self, request, context):
        del context
        return public.list_languages(request)

    def SayHello(self, request, context):
        del context
        start_ns = time.perf_counter_ns()
        response = public.say_hello(request)
        name = request.name.strip() or lookup(response.lang_code).default_name
        # Python serve does not yet expose a handler-visible current transport.
        transport = "unknown"
        duration_ns = time.perf_counter_ns() - start_ns
        message = f"Greeted {name} in {response.language} ({response.lang_code})"
        obs = observability.current()
        obs.logger("greeting").info(
            message,
            lang_code=response.lang_code,
            language=response.language,
            name=name,
            greeting=response.greeting,
            transport=transport,
            duration_ns=duration_ns,
        )
        counter = obs.counter(
            "greeting_emitted_total",
            "Greetings emitted, partitioned by language and transport.",
            {
                "lang_code": response.lang_code,
                "language": response.language,
                "transport": transport,
            },
        )
        if counter is not None:
            counter.inc()
        return response


def _register(server) -> None:
    greeting_pb2_grpc.add_GreetingServiceServicer_to_server(GreetingService(), server)


def listen_and_serve(
    listen_uri: str,
    reflect: bool = False,
    on_listen: Callable[[str], None] | None = None,
) -> None:
    run_with_serve_options(
        normalize_listen_uri(listen_uri),
        _register,
        ServeOptions(reflect=reflect, slug="gabriel-greeting-python"),
        on_listen=on_listen,
    )


def normalize_listen_uri(listen_uri: str) -> str:
    if listen_uri.startswith("tcp://:"):
        return f"tcp://0.0.0.0:{listen_uri.removeprefix('tcp://:')}"
    return listen_uri
