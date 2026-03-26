from __future__ import annotations

"""Certification echo client for python-holons."""

import json
import os
from pathlib import Path
import sys
import time
from typing import Any, Callable, Sequence

from holons.grpcclient import dial_uri
from holons.transport import scheme

DEFAULT_SDK = "python-holons"
DEFAULT_SERVER_SDK = "go-holons"
DEFAULT_URI = "stdio://"
DEFAULT_MESSAGE = "hello"
DEFAULT_GO_BINARY = os.environ.get("GO_BIN", "go")
DEFAULT_TIMEOUT_MS = 5000


def parse_args(argv: Sequence[str]) -> dict[str, Any]:
    out: dict[str, Any] = {
        "uri": DEFAULT_URI,
        "sdk": DEFAULT_SDK,
        "server_sdk": DEFAULT_SERVER_SDK,
        "message": DEFAULT_MESSAGE,
        "go_binary": DEFAULT_GO_BINARY,
        "timeout_ms": DEFAULT_TIMEOUT_MS,
    }

    uri_set = False
    i = 0
    while i < len(argv):
        token = argv[i]
        if token == "--sdk" and i + 1 < len(argv):
            out["sdk"] = argv[i + 1]
            i += 2
            continue
        if token == "--server-sdk" and i + 1 < len(argv):
            out["server_sdk"] = argv[i + 1]
            i += 2
            continue
        if token == "--message" and i + 1 < len(argv):
            out["message"] = argv[i + 1]
            i += 2
            continue
        if token == "--go" and i + 1 < len(argv):
            out["go_binary"] = argv[i + 1]
            i += 2
            continue
        if token == "--timeout-ms" and i + 1 < len(argv):
            try:
                timeout_ms = int(argv[i + 1])
                if timeout_ms > 0:
                    out["timeout_ms"] = timeout_ms
            except ValueError:
                pass
            i += 2
            continue

        if not token.startswith("--") and not uri_set:
            out["uri"] = token
            uri_set = True

        i += 1

    return out


def build_go_echo_server_command(go_binary: str, server_sdk: str) -> list[str]:
    command = [go_binary, "run", "./cmd/echo-server", "--listen", "stdio://"]
    if server_sdk:
        command.extend(["--sdk", server_sdk])
    return command


def _go_holons_dir() -> Path:
    return Path(__file__).resolve().parents[2] / "go-holons"


def _invoke_ping(channel: Any, message: str, timeout_ms: int) -> dict[str, Any]:
    ping = channel.unary_unary(
        "/echo.v1.Echo/Ping",
        request_serializer=lambda payload: json.dumps(payload).encode("utf-8"),
        response_deserializer=lambda raw: json.loads(raw.decode("utf-8")),
    )
    timeout = max(0.1, timeout_ms / 1000.0)
    out = ping({"message": message}, timeout=timeout)
    if isinstance(out, dict):
        return out
    return {"value": out}


def run(
    argv: Sequence[str] | None = None,
    *,
    dial_uri_fn: Callable[..., Any] = dial_uri,
    now_ms: Callable[[], int] | None = None,
) -> dict[str, Any]:
    args = parse_args(argv if argv is not None else sys.argv[1:])
    now = now_ms if now_ms is not None else lambda: int(time.time() * 1000)
    started = now()

    if scheme(args["uri"]) == "stdio":
        command = build_go_echo_server_command(args["go_binary"], args["server_sdk"])
        channel = dial_uri_fn(
            args["uri"],
            stdio_command=command,
            stdio_env=dict(os.environ),
            stdio_cwd=str(_go_holons_dir()),
        )
    else:
        channel = dial_uri_fn(args["uri"])

    try:
        out = _invoke_ping(channel, str(args["message"]), int(args["timeout_ms"]))
        if out.get("message") != args["message"]:
            raise RuntimeError(f"unexpected echo message: {out.get('message')!r}")

        return {
            "status": "pass",
            "sdk": args["sdk"],
            "server_sdk": args["server_sdk"],
            "latency_ms": max(0, now() - started),
        }
    finally:
        channel.close()


def main() -> None:
    try:
        result = run()
    except Exception as exc:  # pragma: no cover - CLI guard
        print(str(exc), file=sys.stderr)
        raise SystemExit(1) from exc

    print(json.dumps(result, separators=(",", ":")))


if __name__ == "__main__":
    main()
