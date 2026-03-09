"""Python hello-world holon using python-holons serve helpers."""

from __future__ import annotations

import sys
from pathlib import Path

SDK_ROOT = Path(__file__).resolve().parents[2] / "sdk" / "python-holons"
if str(SDK_ROOT) not in sys.path:
    sys.path.insert(0, str(SDK_ROOT))

from holons.serve import parse_flags, run_with_options

# Generated proto stubs (run: python -m grpc_tools.protoc ...)
import hello_pb2
import hello_pb2_grpc


class HelloServicer(hello_pb2_grpc.HelloServiceServicer):
    """Implements the HelloService contract."""

    def Greet(self, request, context):
        name = request.name or "World"
        return hello_pb2.GreetResponse(message=f"Hello, {name}!")


def register(server) -> None:
    """Register the HelloService implementation with a grpc.Server."""
    hello_pb2_grpc.add_HelloServiceServicer_to_server(HelloServicer(), server)


def serve(args: list[str] | None = None) -> None:
    """Start the gRPC server using python-holons serve helpers."""
    argv = list(sys.argv[1:] if args is None else args)
    if argv[:1] == ["serve"]:
        argv = argv[1:]

    listen_uri = parse_flags(argv)
    run_with_options(listen_uri, register, reflect=True)


if __name__ == "__main__":
    serve()
