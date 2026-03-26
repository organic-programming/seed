"""Tests for holons.grpcclient."""

import json
from pathlib import Path
import shutil
import time

import grpc
import pytest

from holons.grpcclient import dial, dial_stdio, dial_uri, dial_websocket


def test_dial_tcp_address():
    ch = dial("127.0.0.1:9090")
    try:
        assert isinstance(ch, grpc.Channel)
    finally:
        ch.close()


def test_dial_uri_tcp():
    ch = dial_uri("tcp://127.0.0.1:9090")
    try:
        assert isinstance(ch, grpc.Channel)
    finally:
        ch.close()

def test_dial_uri_unsupported_scheme():
    try:
        dial_uri("ftp://127.0.0.1:21")
        assert False, "should have raised"
    except ValueError as e:
        assert "unsupported transport URI" in str(e)


def test_dial_uri_rest_sse_unsupported():
    with pytest.raises(ValueError, match="unsupported transport URI"):
        dial_uri("rest+sse://127.0.0.1:8080/api/v1/rpc")


def test_dial_websocket_requires_ws_scheme():
    try:
        dial_websocket("tcp://127.0.0.1:9090")
        assert False, "should have raised"
    except ValueError as e:
        assert "expects ws:// or wss://" in str(e)


def _resolve_go_binary() -> str:
    preferred = Path("/Users/bpds/go/go1.25.1/bin/go")
    if preferred.exists():
        return str(preferred)

    found = shutil.which("go")
    if not found:
        pytest.skip("go binary not found")
    return found


def _invoke_echo_ping(channel: grpc.Channel, message: str) -> dict:
    stub = channel.unary_unary(
        "/echo.v1.Echo/Ping",
        request_serializer=lambda value: json.dumps(value).encode("utf-8"),
        response_deserializer=lambda raw: json.loads(raw.decode("utf-8")),
    )
    return stub({"message": message}, timeout=1.0)


def _assert_echo_roundtrip(channel: grpc.Channel) -> None:
    deadline = time.time() + 6.0
    last_error = None
    while time.time() < deadline:
        try:
            out = _invoke_echo_ping(channel, "hello-stdio")
            assert out["message"] == "hello-stdio"
            assert out["sdk"] == "go-holons"
            return
        except grpc.RpcError as exc:
            last_error = exc
            time.sleep(0.1)

    assert False, f"stdio echo call did not succeed: {last_error}"


def _skip_if_local_bind_denied(exc: BaseException) -> None:
    msg = str(exc).lower()
    if isinstance(exc, PermissionError) or "operation not permitted" in msg:
        pytest.skip(f"local bind denied in this environment: {exc}")


def test_dial_uri_stdio_requires_command():
    with pytest.raises(ValueError, match="requires stdio_command"):
        dial_uri("stdio://")


def test_dial_stdio_reports_child_startup_stderr():
    shell = shutil.which("sh")
    if not shell:
        pytest.skip("sh not found")

    with pytest.raises(RuntimeError) as exc_info:
        dial_stdio(shell, "-c", "echo stdio-start-failed >&2; exit 23")

    msg = str(exc_info.value)
    assert "exited with code 23" in msg
    assert "stdio-start-failed" in msg


def test_dial_stdio_go_echo_roundtrip():
    go_bin = _resolve_go_binary()
    sdk_dir = Path(__file__).resolve().parents[2]
    go_holons_dir = sdk_dir / "go-holons"

    try:
        ch = dial_stdio(
            go_bin,
            "run",
            "./cmd/echo-server",
            "--listen",
            "stdio://",
            "--sdk",
            "go-holons",
            cwd=str(go_holons_dir),
        )
    except (PermissionError, OSError) as exc:
        _skip_if_local_bind_denied(exc)
        raise
    try:
        _assert_echo_roundtrip(ch)
    finally:
        ch.close()


def test_dial_uri_stdio_go_echo_roundtrip():
    go_bin = _resolve_go_binary()
    sdk_dir = Path(__file__).resolve().parents[2]
    go_holons_dir = sdk_dir / "go-holons"

    try:
        ch = dial_uri(
            "stdio://",
            stdio_command=[
                go_bin,
                "run",
                "./cmd/echo-server",
                "--listen",
                "stdio://",
                "--sdk",
                "go-holons",
            ],
            stdio_cwd=str(go_holons_dir),
        )
    except (PermissionError, OSError) as exc:
        _skip_if_local_bind_denied(exc)
        raise
    try:
        _assert_echo_roundtrip(ch)
    finally:
        ch.close()
