from __future__ import annotations

from pathlib import Path

import pytest

from holons.discover import resolve
from holons.discovery_types import CWD, LOCAL, NO_TIMEOUT

from ._fixtures import PackageSeed, write_package_holon


def runtime_fixture(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
    root = tmp_path
    monkeypatch.setenv("OPPATH", str(tmp_path / "runtime"))
    monkeypatch.setenv("OPBIN", str(tmp_path / "runtime" / "bin"))
    return root


def test_resolve_known_slug(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root = runtime_fixture(tmp_path, monkeypatch)
    write_package_holon(
        root / "alpha.holon",
        PackageSeed(slug="alpha", uuid="uuid-alpha", given_name="Alpha", family_name="One"),
    )

    result = resolve(LOCAL, "alpha", str(root), CWD, NO_TIMEOUT)

    assert result.error is None
    assert result.ref is not None
    assert result.ref.info is not None
    assert result.ref.info.slug == "alpha"


def test_resolve_missing(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root = runtime_fixture(tmp_path, monkeypatch)

    result = resolve(LOCAL, "missing", str(root), CWD, NO_TIMEOUT)

    assert result.error is not None
    assert result.ref is None


def test_resolve_invalid_specifiers(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root = runtime_fixture(tmp_path, monkeypatch)

    result = resolve(LOCAL, "alpha", str(root), 0xFF, NO_TIMEOUT)

    assert result.error is not None
    assert result.ref is None
