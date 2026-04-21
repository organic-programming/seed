"""Tests for holons.echo_server."""

from __future__ import annotations

import json
from pathlib import Path
import subprocess
import sys

import grpc
import pytest

from holons.echo_server import parse_args
from holons.grpcclient import dial_stdio, dial_uri


def _sdk_dir() -> Path:
    return Path(__file__).resolve().parents[1]


def _is_bind_denied(stderr: str) -> bool:
    text = stderr.lower()
    return "bind" in text and "operation not permitted" in text


def _invoke_ping(channel: grpc.Channel, message: str, timeout: float = 2.0) -> dict:
    stub = channel.unary_unary(
        "/echo.v1.Echo/Ping",
        request_serializer=lambda value: json.dumps(value).encode("utf-8"),
        response_deserializer=lambda raw: json.loads(raw.decode("utf-8")),
    )
    return stub({"message": message}, timeout=timeout)


def _start_echo_server(*args: str) -> tuple[subprocess.Popen[str], str]:
    proc = subprocess.Popen(
        [sys.executable, "-m", "holons.echo_server", *args],
        cwd=_sdk_dir(),
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )

    assert proc.stdout is not None
    uri = proc.stdout.readline().strip()
    if uri:
        return proc, uri

    stderr = ""
    if proc.stderr is not None:
        stderr = proc.stderr.read()
    _stop_process(proc)

    if _is_bind_denied(stderr):
        pytest.skip("local bind denied in this environment")
    raise RuntimeError(f"echo_server failed to start: {stderr}")


def _stop_process(proc: subprocess.Popen[str]) -> int:
    if proc.poll() is not None:
        return int(proc.returncode)

    proc.terminate()
    try:
        proc.wait(timeout=5)
    except subprocess.TimeoutExpired:
        proc.kill()
        proc.wait(timeout=5)
    return int(proc.returncode)


def test_parse_args_defaults():
    args = parse_args([])
    assert args["listen_uri"] == "tcp://127.0.0.1:0"
    assert args["sdk"] == "python-holons"
    assert args["version"] == "0.1.0"
    assert args["sleep_ms"] == 0


def test_parse_args_serve_compatibility_and_overrides():
    args = parse_args(
        [
            "serve",
            "--port",
            "7123",
            "--sdk",
            "py-sdk",
            "--version",
            "9.9.9",
            "--sleep-ms",
            "1200",
        ]
    )
    assert args["listen_uri"] == "tcp://127.0.0.1:7123"
    assert args["sdk"] == "py-sdk"
    assert args["version"] == "9.9.9"
    assert args["sleep_ms"] == 1200


def test_echo_server_tcp_roundtrip_and_sigterm_exit_zero():
    proc, uri = _start_echo_server(
        "--listen",
        "tcp://127.0.0.1:0",
        "--sdk",
        "python-holons",
        "--version",
        "0.1.0",
    )
    try:
        channel = dial_uri(uri)
        try:
            out = _invoke_ping(channel, "hello-tcp")
            assert out["message"] == "hello-tcp"
            assert out["sdk"] == "python-holons"
            assert out["version"] == "0.1.0"
        finally:
            channel.close()
    finally:
        rc = _stop_process(proc)
        assert rc == 0


def test_echo_server_stdio_roundtrip_with_serve_prefix():
    channel = dial_stdio(
        sys.executable,
        "-m",
        "holons.echo_server",
        "serve",
        "--listen",
        "stdio://",
        "--sdk",
        "python-holons",
        "--version",
        "0.1.0",
        cwd=str(_sdk_dir()),
    )
    try:
        out = _invoke_ping(channel, "hello-stdio")
        assert out["message"] == "hello-stdio"
        assert out["sdk"] == "python-holons"
        assert out["version"] == "0.1.0"
    finally:
        channel.close()


def test_echo_server_rejects_oversized_message_and_stays_alive():
    proc, uri = _start_echo_server("--listen", "tcp://127.0.0.1:0")
    try:
        channel = dial_uri(uri)
        try:
            large = "x" * (2 * 1024 * 1024)
            with pytest.raises(grpc.RpcError) as exc_info:
                _invoke_ping(channel, large, timeout=4.0)
            assert exc_info.value.code() == grpc.StatusCode.RESOURCE_EXHAUSTED

            out = _invoke_ping(channel, "small", timeout=2.0)
            assert out["message"] == "small"
        finally:
            channel.close()
    finally:
        _stop_process(proc)


def test_echo_server_sleep_flag_allows_timeout_propagation():
    proc, uri = _start_echo_server("--listen", "tcp://127.0.0.1:0", "--sleep-ms", "600")
    try:
        channel = dial_uri(uri)
        try:
            with pytest.raises(grpc.RpcError) as exc_info:
                _invoke_ping(channel, "slow", timeout=0.2)
            assert exc_info.value.code() == grpc.StatusCode.DEADLINE_EXCEEDED

            out = _invoke_ping(channel, "fast-enough", timeout=2.0)
            assert out["message"] == "fast-enough"
        finally:
            channel.close()
    finally:
        _stop_process(proc)
