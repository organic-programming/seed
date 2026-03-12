#!/usr/bin/env python3
from __future__ import annotations

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SDK = ROOT.parent.parent.parent / "sdk" / "python-holons"
sys.path.insert(0, str(ROOT))
sys.path.insert(0, str(SDK))
sys.path.insert(0, str(ROOT / "gen" / "python"))

from holons.serve import parse_flags, run_with_options
from greeting.v1 import greeting_pb2, greeting_pb2_grpc  # noqa: E402
from greetings import GREETINGS, lookup  # noqa: E402


class GreetingService(greeting_pb2_grpc.GreetingServiceServicer):
    def ListLanguages(self, request, context):
        del request, context
        response = greeting_pb2.ListLanguagesResponse()
        for greeting in GREETINGS:
            response.languages.add(
                code=greeting["code"],
                name=greeting["name"],
                native=greeting["native"],
            )
        return response

    def SayHello(self, request, context):
        del context
        greeting = lookup(request.lang_code)
        name = request.name.strip() or "World"
        return greeting_pb2.SayHelloResponse(
            greeting=greeting["template"] % name,
            language=greeting["name"],
            lang_code=greeting["code"],
        )


def _register(server):
    greeting_pb2_grpc.add_GreetingServiceServicer_to_server(GreetingService(), server)


def _usage() -> None:
    print("usage: gudule-daemon-greeting-python <serve|version> [flags]", file=sys.stderr)
    raise SystemExit(1)


def main(argv: list[str]) -> int:
    if len(argv) < 2:
        _usage()

    command = argv[1]
    if command == "serve":
        listen_uri = parse_flags(argv[2:])
        run_with_options(listen_uri, _register, reflect=True)
        return 0
    if command == "version":
        print("gudule-daemon-greeting-python v0.4.2")
        return 0

    _usage()
    return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
