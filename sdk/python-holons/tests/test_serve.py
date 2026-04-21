"""Tests for holons.serve — flag parsing."""

import logging

import pytest

from holons import describe as describe_module
from holons.serve import parse_flags, parse_options
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
