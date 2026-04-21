"""Tests for holons.echo_client."""

from __future__ import annotations

from pathlib import Path

from holons.echo_client import (
    build_go_echo_server_command,
    parse_args,
    run,
)


class _FakeChannel:
    def __init__(self):
        self.closed = False
        self.calls = []

    def unary_unary(self, method, request_serializer=None, response_deserializer=None):
        self.calls.append(method)

        def _invoke(payload, timeout=None):
            return {"message": payload.get("message", "")}

        return _invoke

    def close(self):
        self.closed = True


def test_parse_args_defaults():
    args = parse_args([])
    assert args["uri"] == "stdio://"
    assert args["sdk"] == "python-holons"
    assert args["server_sdk"] == "go-holons"
    assert args["message"] == "hello"
    assert args["timeout_ms"] == 5000


def test_parse_args_overrides():
    args = parse_args(
        [
            "tcp://127.0.0.1:3000",
            "--sdk",
            "py",
            "--server-sdk",
            "go",
            "--message",
            "hola",
            "--go",
            "/tmp/go",
            "--timeout-ms",
            "1200",
        ]
    )
    assert args["uri"] == "tcp://127.0.0.1:3000"
    assert args["sdk"] == "py"
    assert args["server_sdk"] == "go"
    assert args["message"] == "hola"
    assert args["go_binary"] == "/tmp/go"
    assert args["timeout_ms"] == 1200


def test_build_go_echo_server_command():
    command = build_go_echo_server_command("go", "go-holons")
    assert command[0] == "go"
    assert command[1] == "run"
    assert command[2] == "./cmd/echo-server"
    assert command[3:] == ["--listen", "stdio://", "--sdk", "go-holons"]


def test_run_stdio_uses_child_command():
    fake_channel = _FakeChannel()
    captured = {}

    def _dial(uri, **kwargs):
        captured["uri"] = uri
        captured["kwargs"] = kwargs
        return fake_channel

    ticks = iter([1000, 1017])
    result = run(
        ["stdio://", "--message", "hola"],
        dial_uri_fn=_dial,
        now_ms=lambda: next(ticks),
    )

    assert captured["uri"] == "stdio://"
    command = captured["kwargs"]["stdio_command"]
    assert command[1] == "run"
    assert command[2] == "./cmd/echo-server"
    assert command[-4:] == ["--listen", "stdio://", "--sdk", "go-holons"]
    assert "stdio_env" in captured["kwargs"]
    go_cwd = Path(captured["kwargs"]["stdio_cwd"])
    assert go_cwd.name == "go-holons"
    assert go_cwd.parent.name == "sdk"
    assert result == {
        "status": "pass",
        "sdk": "python-holons",
        "server_sdk": "go-holons",
        "latency_ms": 17,
    }
    assert fake_channel.closed is True
    assert fake_channel.calls == ["/echo.v1.Echo/Ping"]


def test_run_tcp_dials_without_stdio_options():
    fake_channel = _FakeChannel()
    captured = {}

    def _dial(uri, **kwargs):
        captured["uri"] = uri
        captured["kwargs"] = kwargs
        return fake_channel

    ticks = iter([300, 340])
    result = run(
        ["tcp://127.0.0.1:9090", "--message", "ok"],
        dial_uri_fn=_dial,
        now_ms=lambda: next(ticks),
    )

    assert captured["uri"] == "tcp://127.0.0.1:9090"
    assert captured["kwargs"] == {}
    assert result["status"] == "pass"
    assert result["latency_ms"] == 40
    assert fake_channel.closed is True
