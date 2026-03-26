"""Tests for holons.transport — listener factory and URI parsing."""

import socket

import pytest

from holons.transport import (
    DEFAULT_URI,
    ParsedURI,
    listen,
    parse_uri,
    scheme,
)


def _skip_if_bind_denied(exc: BaseException) -> None:
    msg = str(exc).lower()
    if isinstance(exc, PermissionError) or "operation not permitted" in msg:
        pytest.skip(f"local bind denied in this environment: {exc}")


def test_scheme_extraction():
    assert scheme("tcp://:9090") == "tcp"
    assert scheme("unix:///tmp/x.sock") == "unix"
    assert scheme("stdio://") == "stdio"
    assert scheme("ws://host:8080") == "ws"
    assert scheme("wss://host:443") == "wss"


def test_default_uri():
    assert DEFAULT_URI == "tcp://:9090"


def test_parse_uri_tcp():
    parsed = parse_uri("tcp://127.0.0.1:9090")
    assert parsed == ParsedURI(raw="tcp://127.0.0.1:9090", scheme="tcp", host="127.0.0.1", port=9090)


def test_parse_uri_wss_default_path():
    parsed = parse_uri("wss://example.com:8443")
    assert parsed.scheme == "wss"
    assert parsed.secure is True
    assert parsed.path == "/grpc"
    assert parsed.port == 8443


def test_parse_uri_ws_custom_path():
    parsed = parse_uri("ws://127.0.0.1:8080/holon")
    assert parsed.scheme == "ws"
    assert parsed.path == "/holon"


def test_tcp_listen():
    try:
        sock = listen("tcp://127.0.0.1:0")
    except (PermissionError, OSError) as exc:
        _skip_if_bind_denied(exc)
        raise
    try:
        assert isinstance(sock, socket.socket)
        addr = sock.getsockname()
        assert addr[0] == "127.0.0.1"
        assert addr[1] > 0
    finally:
        sock.close()


def test_unix_listen():
    import os
    import tempfile

    path = os.path.join(tempfile.gettempdir(), "holons_test.sock")
    try:
        sock = listen(f"unix://{path}")
    except (PermissionError, OSError) as exc:
        _skip_if_bind_denied(exc)
        raise
    try:
        assert isinstance(sock, socket.socket)
    finally:
        sock.close()

def test_stdio_listener_single_use():
    lis = listen("stdio://")
    lis.accept()
    try:
        lis.accept()
        assert False, "should have raised"
    except StopIteration as e:
        assert "single-use" in str(e)
    lis.close()


def test_unsupported_uri():
    try:
        listen("ftp://host")
        assert False, "should have raised"
    except ValueError as e:
        assert "unsupported" in str(e)
