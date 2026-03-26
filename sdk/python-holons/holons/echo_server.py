from __future__ import annotations

"""Certification echo server for python-holons."""

import json
import sys
import time
from typing import Any, Sequence

import grpc

from holons import describe
from holons.serve import run_with_options
from holons.transport import scheme
from holons.v1 import describe_pb2, manifest_pb2

DEFAULT_LISTEN_URI = "tcp://127.0.0.1:0"
DEFAULT_SDK = "python-holons"
DEFAULT_VERSION = "0.1.0"
DEFAULT_SLEEP_MS = 0


def parse_args(argv: Sequence[str]) -> dict[str, Any]:
    args = list(argv)
    if args and args[0] == "serve":
        args = args[1:]

    out: dict[str, Any] = {
        "listen_uri": DEFAULT_LISTEN_URI,
        "sdk": DEFAULT_SDK,
        "version": DEFAULT_VERSION,
        "sleep_ms": DEFAULT_SLEEP_MS,
    }

    uri_set = False
    i = 0
    while i < len(args):
        token = args[i]

        if token == "--listen" and i + 1 < len(args):
            out["listen_uri"] = args[i + 1]
            uri_set = True
            i += 2
            continue

        if token == "--port" and i + 1 < len(args):
            out["listen_uri"] = f"tcp://127.0.0.1:{args[i + 1]}"
            uri_set = True
            i += 2
            continue

        if token == "--sdk" and i + 1 < len(args):
            out["sdk"] = args[i + 1]
            i += 2
            continue

        if token == "--version" and i + 1 < len(args):
            out["version"] = args[i + 1]
            i += 2
            continue

        if token == "--sleep-ms" and i + 1 < len(args):
            try:
                sleep_ms = int(args[i + 1])
                if sleep_ms >= 0:
                    out["sleep_ms"] = sleep_ms
            except ValueError:
                pass
            i += 2
            continue

        if not token.startswith("--") and not uri_set:
            out["listen_uri"] = token
            uri_set = True

        i += 1

    return out


def _decode_request(raw: bytes) -> dict[str, Any]:
    if not raw:
        return {}

    payload = json.loads(raw.decode("utf-8"))
    if not isinstance(payload, dict):
        raise ValueError("request must be a JSON object")
    return payload


def _encode_response(payload: dict[str, Any]) -> bytes:
    return json.dumps(payload, separators=(",", ":")).encode("utf-8")


class _EchoHandler(grpc.GenericRpcHandler):
    def __init__(self, sdk: str, version: str, sleep_ms: int):
        self._sdk = sdk
        self._version = version
        self._sleep_ms = max(0, int(sleep_ms))

    def service(self, handler_call_details: grpc.HandlerCallDetails):
        if handler_call_details.method != "/echo.v1.Echo/Ping":
            return None

        return grpc.unary_unary_rpc_method_handler(
            self._ping,
            request_deserializer=_decode_request,
            response_serializer=_encode_response,
        )

    def _ping(self, request: dict[str, Any], _context: grpc.ServicerContext) -> dict[str, Any]:
        if self._sleep_ms > 0:
            time.sleep(self._sleep_ms / 1000.0)

        message = ""
        raw_message = request.get("message", "")
        if isinstance(raw_message, str):
            message = raw_message
        elif raw_message is not None:
            message = str(raw_message)

        return {
            "message": message,
            "sdk": self._sdk,
            "version": self._version,
        }


def run(argv: Sequence[str] | None = None) -> None:
    args = parse_args(argv if argv is not None else sys.argv[1:])
    describe.use_static_response(_echo_describe_response(str(args["version"])))

    def _register(server: grpc.Server) -> None:
        server.add_generic_rpc_handlers(
            [_EchoHandler(str(args["sdk"]), str(args["version"]), int(args["sleep_ms"]))]
        )

    def _announce(uri: str) -> None:
        if scheme(uri) != "stdio":
            print(uri, flush=True)

    run_with_options(
        str(args["listen_uri"]),
        _register,
        reflect=False,
        on_listen=_announce,
    )


def main() -> None:
    try:
        run()
    except Exception as exc:  # pragma: no cover - CLI guard
        print(str(exc), file=sys.stderr)
        raise SystemExit(1) from exc

def _echo_describe_response(version: str) -> describe_pb2.DescribeResponse:
    return describe_pb2.DescribeResponse(
        manifest=manifest_pb2.HolonManifest(
            identity=manifest_pb2.HolonManifest.Identity(
                schema="holon/v1",
                uuid="echo-server-sdk-0000",
                given_name="Echo",
                family_name="Server",
                motto="Echoes payloads for SDK transport tests.",
                composer="python-holons",
                status="draft",
                born="2026-03-23",
                version=version,
            ),
            lang="python",
        ),
        services=[
            describe_pb2.ServiceDoc(
                name="echo.v1.Echo",
                description="Echo test server used by python-holons integration tests.",
                methods=[
                    describe_pb2.MethodDoc(
                        name="Ping",
                        description="Returns the inbound message with SDK metadata.",
                        input_type="echo.v1.PingRequest",
                        output_type="echo.v1.PingResponse",
                    )
                ],
            )
        ],
    )


if __name__ == "__main__":
    main()
