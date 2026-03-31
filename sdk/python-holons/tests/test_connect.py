from __future__ import annotations

from pathlib import Path

import pytest

from holons.connect import connect, disconnect
from holons.discovery_types import ConnectResult, INSTALLED, LOCAL

from ._fixtures import PackageSeed, invoke_ping, write_package_holon


def runtime_fixture(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> tuple[Path, Path]:
    root = tmp_path
    op_home = tmp_path / "runtime"
    op_bin = op_home / "bin"
    monkeypatch.setenv("OPPATH", str(op_home))
    monkeypatch.setenv("OPBIN", str(op_bin))
    return root, op_bin


def test_connect_unresolvable(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, _ = runtime_fixture(tmp_path, monkeypatch)

    result = connect(LOCAL, "missing", str(root), INSTALLED, 1000)

    assert result.error is not None
    assert result.channel is None
    assert result.origin is None


def test_connect_returns_connect_result(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, op_bin = runtime_fixture(tmp_path, monkeypatch)
    write_package_holon(
        op_bin / "known-slug.holon",
        PackageSeed(slug="known-slug", uuid="uuid-known", given_name="Known", family_name="Slug"),
        with_holon_json=True,
        with_binary=True,
    )

    result = connect(LOCAL, "known-slug", str(root), INSTALLED, 5000)
    try:
        assert isinstance(result, ConnectResult)
        assert result.error is None
        assert result.channel is not None
        assert invoke_ping(result.channel, "connect-python")["message"] == "connect-python"
    finally:
        disconnect(result)


def test_connect_returns_origin(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, op_bin = runtime_fixture(tmp_path, monkeypatch)
    package_root = op_bin / "origin-slug.holon"
    write_package_holon(
        package_root,
        PackageSeed(slug="origin-slug", uuid="uuid-origin", given_name="Origin", family_name="Slug"),
        with_holon_json=True,
        with_binary=True,
    )

    result = connect(LOCAL, "origin-slug", str(root), INSTALLED, 5000)
    try:
        assert result.error is None
        assert result.origin is not None
        assert result.origin.info is not None
        assert result.origin.info.slug == "origin-slug"
        assert result.origin.url == package_root.resolve().as_uri()
    finally:
        disconnect(result)


def test_disconnect_accepts_connect_result(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, op_bin = runtime_fixture(tmp_path, monkeypatch)
    write_package_holon(
        op_bin / "disconnect-slug.holon",
        PackageSeed(
            slug="disconnect-slug",
            uuid="uuid-disconnect",
            given_name="Disconnect",
            family_name="Slug",
        ),
        with_holon_json=True,
        with_binary=True,
    )

    result = connect(LOCAL, "disconnect-slug", str(root), INSTALLED, 5000)

    assert result.error is None
    assert result.channel is not None
    disconnect(result)
