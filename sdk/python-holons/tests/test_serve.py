"""Tests for holons.serve — flag parsing."""

import logging

import pytest

from holons import describe as describe_module
from holons.serve import CurrentTransport, parse_flags, parse_options
from holons import serve as serve_module
from holons.transport import DEFAULT_URI


def test_parse_listen():
    assert parse_flags(["--listen", "tcp://:8080"]) == "tcp://:8080"


def test_parse_port():
    assert parse_flags(["--port", "3000"]) == "tcp://:3000"


def test_parse_default():
    assert parse_flags([]) == DEFAULT_URI
    assert parse_flags(["--verbose"]) == DEFAULT_URI


def test_parse_options_reflect():
    parsed = parse_options(["--listen", "tcp://:8080", "--reflect"])
    assert parsed.listen_uri == "tcp://:8080"
    assert parsed.reflect is True


def test_run_with_options_requires_static_describe_response():
    with pytest.raises(
        describe_module.IncodeDescriptionError,
        match=describe_module.NO_INCODE_DESCRIPTION_MESSAGE,
    ):
        serve_module.run_with_options("tcp://127.0.0.1:0", lambda server: None)
    assert CurrentTransport() == ""


def test_current_transport_tracks_stdio_run_lifecycle(monkeypatch):
    class FakeServer:
        _state = type("State", (), {"generic_handlers": ()})()

        def add_insecure_port(self, address):
            assert address.startswith("unix:")
            return 1

        def start(self):
            assert CurrentTransport() == "stdio"

        def wait_for_termination(self):
            assert CurrentTransport() == "stdio"

        def stop(self, grace):
            assert grace == 10

    class FakeStdioBridge:
        def start(self):
            return "/tmp/holons-fake-stdio.sock"

        def connect_to_server(self):
            assert CurrentTransport() == "stdio"

        def close(self):
            assert CurrentTransport() == "stdio"

    monkeypatch.setattr(serve_module.grpc, "server", lambda *args, **kwargs: FakeServer())
    monkeypatch.setattr(serve_module.describe, "register", lambda server: None)
    monkeypatch.setattr(serve_module, "_StdioServeBridge", FakeStdioBridge)
    monkeypatch.setattr(serve_module.signal, "signal", lambda *args, **kwargs: None)

    assert CurrentTransport() == ""
    serve_module.run_with_options("stdio://", lambda server: None)
    assert CurrentTransport() == ""


def test_run_with_options_logs_other_holon_meta_registration_errors(monkeypatch, caplog):
    def _boom(_server):
        raise RuntimeError("boom")

    monkeypatch.setattr(serve_module.describe, "register", _boom)

    with caplog.at_level(logging.ERROR, logger="holons.serve"):
        with pytest.raises(RuntimeError, match="register HolonMeta: boom"):
            serve_module.run_with_options("tcp://127.0.0.1:0", lambda server: None)

    assert "HolonMeta registration failed: boom" in caplog.text


def test_run_with_options_rest_sse_is_not_supported(monkeypatch):
    monkeypatch.setattr(serve_module.describe, "register", lambda server: None)

    with pytest.raises(ValueError, match="gRPC Python server supports tcp://, unix://, and stdio://"):
        serve_module.run_with_options("rest+sse://127.0.0.1:8080/api/v1/rpc", lambda server: None)
