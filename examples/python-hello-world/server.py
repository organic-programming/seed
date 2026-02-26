"""Python hello-world holon — gRPC server implementing HelloService."""

import sys
from concurrent import futures

import grpc
from grpc_reflection.v1alpha import reflection

# Generated proto stubs (run: python -m grpc_tools.protoc ...)
import hello_pb2
import hello_pb2_grpc


class HelloServicer(hello_pb2_grpc.HelloServiceServicer):
    """Implements the HelloService contract."""

    def Greet(self, request, context):
        name = request.name or "World"
        return hello_pb2.GreetResponse(message=f"Hello, {name}!")


def serve(listen_addr: str = "[::]:9090"):
    """Start the gRPC server."""
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=4))
    hello_pb2_grpc.add_HelloServiceServicer_to_server(HelloServicer(), server)

    # Enable reflection
    service_names = (
        hello_pb2.DESCRIPTOR.services_by_name["HelloService"].full_name,
        reflection.SERVICE_NAME,
    )
    reflection.enable_server_reflection(service_names, server)

    server.add_insecure_port(listen_addr)
    server.start()
    print(f"gRPC server listening on {listen_addr}", file=sys.stderr)
    server.wait_for_termination()


if __name__ == "__main__":
    addr = "[::]:9090"
    for i, arg in enumerate(sys.argv):
        if arg == "--port" and i + 1 < len(sys.argv):
            addr = f"[::]:{sys.argv[i + 1]}"
    serve(addr)
