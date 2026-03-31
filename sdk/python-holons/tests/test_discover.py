from __future__ import annotations

import importlib
from pathlib import Path

import pytest

from holons.discover import Discover
from holons.discovery_types import (
    ALL,
    BUILT,
    CACHED,
    CWD,
    DELEGATED,
    INSTALLED,
    LOCAL,
    NO_LIMIT,
    NO_TIMEOUT,
    PROXY,
    SIBLINGS,
    SOURCE,
    DiscoverResult,
    HolonInfo,
    HolonRef,
    IdentityInfo,
)

from ._fixtures import PackageSeed, file_url, sorted_slugs, write_package_holon

discover_module = importlib.import_module("holons.discover")


def runtime_fixture(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> tuple[Path, Path, Path]:
    root = tmp_path
    op_home = tmp_path / "runtime"
    op_bin = op_home / "bin"
    monkeypatch.setenv("OPPATH", str(op_home))
    monkeypatch.setenv("OPBIN", str(op_bin))
    return root, op_home, op_bin


def test_discover_all_layers(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, op_home, op_bin = runtime_fixture(tmp_path, monkeypatch)

    write_package_holon(
        root / "cwd-alpha.holon",
        PackageSeed(slug="cwd-alpha", uuid="uuid-cwd-alpha", given_name="Cwd", family_name="Alpha"),
    )
    write_package_holon(
        root / ".op" / "build" / "built-beta.holon",
        PackageSeed(slug="built-beta", uuid="uuid-built-beta", given_name="Built", family_name="Beta"),
    )
    write_package_holon(
        op_bin / "installed-gamma.holon",
        PackageSeed(
            slug="installed-gamma",
            uuid="uuid-installed-gamma",
            given_name="Installed",
            family_name="Gamma",
        ),
    )
    write_package_holon(
        op_home / "cache" / "deps" / "cached-delta.holon",
        PackageSeed(slug="cached-delta", uuid="uuid-cached-delta", given_name="Cached", family_name="Delta"),
    )

    app_executable = root / "TestApp.app" / "Contents" / "MacOS" / "TestApp"
    app_executable.parent.mkdir(parents=True, exist_ok=True)
    app_executable.write_text("#!/bin/sh\n", encoding="utf-8")
    bundle_root = root / "TestApp.app" / "Contents" / "Resources" / "Holons"
    write_package_holon(
        bundle_root / "bundle.holon",
        PackageSeed(slug="bundle", uuid="uuid-bundle", given_name="Bundle", family_name="Holon"),
    )
    monkeypatch.setattr(discover_module.sys, "executable", str(app_executable))

    result = Discover(LOCAL, None, str(root), ALL, NO_LIMIT, NO_TIMEOUT)

    assert result.error is None
    assert sorted_slugs(result) == [
        "built-beta",
        "bundle",
        "cached-delta",
        "cwd-alpha",
        "installed-gamma",
    ]


def test_discover_filter_by_specifiers(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, _, op_bin = runtime_fixture(tmp_path, monkeypatch)

    write_package_holon(
        root / "cwd-alpha.holon",
        PackageSeed(slug="cwd-alpha", uuid="uuid-cwd-alpha", given_name="Cwd", family_name="Alpha"),
    )
    write_package_holon(
        root / ".op" / "build" / "built-beta.holon",
        PackageSeed(slug="built-beta", uuid="uuid-built-beta", given_name="Built", family_name="Beta"),
    )
    write_package_holon(
        op_bin / "installed-gamma.holon",
        PackageSeed(
            slug="installed-gamma",
            uuid="uuid-installed-gamma",
            given_name="Installed",
            family_name="Gamma",
        ),
    )

    result = Discover(LOCAL, None, str(root), BUILT | INSTALLED, NO_LIMIT, NO_TIMEOUT)

    assert result.error is None
    assert sorted_slugs(result) == ["built-beta", "installed-gamma"]


def test_discover_match_by_slug(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, _, _ = runtime_fixture(tmp_path, monkeypatch)
    write_package_holon(
        root / "alpha.holon",
        PackageSeed(slug="alpha", uuid="uuid-alpha", given_name="Alpha", family_name="One"),
    )
    write_package_holon(
        root / "beta.holon",
        PackageSeed(slug="beta", uuid="uuid-beta", given_name="Beta", family_name="Two"),
    )

    result = Discover(LOCAL, "beta", str(root), CWD, NO_LIMIT, NO_TIMEOUT)

    assert result.error is None
    assert sorted_slugs(result) == ["beta"]


def test_discover_match_by_alias(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, _, _ = runtime_fixture(tmp_path, monkeypatch)
    write_package_holon(
        root / "alpha.holon",
        PackageSeed(
            slug="alpha",
            uuid="uuid-alpha",
            given_name="Alpha",
            family_name="One",
            aliases=["first"],
        ),
    )

    result = Discover(LOCAL, "first", str(root), CWD, NO_LIMIT, NO_TIMEOUT)

    assert result.error is None
    assert sorted_slugs(result) == ["alpha"]


def test_discover_match_by_uuid_prefix(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, _, _ = runtime_fixture(tmp_path, monkeypatch)
    write_package_holon(
        root / "alpha.holon",
        PackageSeed(slug="alpha", uuid="12345678-aaaa", given_name="Alpha", family_name="One"),
    )

    result = Discover(LOCAL, "12345678", str(root), CWD, NO_LIMIT, NO_TIMEOUT)

    assert result.error is None
    assert sorted_slugs(result) == ["alpha"]


def test_discover_match_by_path(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, _, _ = runtime_fixture(tmp_path, monkeypatch)
    package_dir = root / "nested" / "alpha.holon"
    write_package_holon(
        package_dir,
        PackageSeed(slug="alpha", uuid="uuid-alpha", given_name="Alpha", family_name="One"),
    )

    result = Discover(LOCAL, "nested/alpha.holon", str(root), CWD, NO_LIMIT, NO_TIMEOUT)

    assert result.error is None
    assert len(result.found) == 1
    assert result.found[0].url == file_url(package_dir)


def test_discover_limit_one(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, _, _ = runtime_fixture(tmp_path, monkeypatch)
    write_package_holon(
        root / "alpha.holon",
        PackageSeed(slug="alpha", uuid="uuid-alpha", given_name="Alpha", family_name="One"),
    )
    write_package_holon(
        root / "beta.holon",
        PackageSeed(slug="beta", uuid="uuid-beta", given_name="Beta", family_name="Two"),
    )

    result = Discover(LOCAL, None, str(root), CWD, 1, NO_TIMEOUT)

    assert result.error is None
    assert len(result.found) == 1


def test_discover_limit_zero_means_unlimited(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, _, _ = runtime_fixture(tmp_path, monkeypatch)
    write_package_holon(
        root / "alpha.holon",
        PackageSeed(slug="alpha", uuid="uuid-alpha", given_name="Alpha", family_name="One"),
    )
    write_package_holon(
        root / "beta.holon",
        PackageSeed(slug="beta", uuid="uuid-beta", given_name="Beta", family_name="Two"),
    )

    result = Discover(LOCAL, None, str(root), CWD, 0, NO_TIMEOUT)

    assert result.error is None
    assert len(result.found) == 2


def test_discover_negative_limit_returns_empty(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, _, _ = runtime_fixture(tmp_path, monkeypatch)

    result = Discover(LOCAL, None, str(root), CWD, -1, NO_TIMEOUT)

    assert result.error is None
    assert result.found == []


def test_discover_invalid_specifiers(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, _, _ = runtime_fixture(tmp_path, monkeypatch)

    result = Discover(LOCAL, None, str(root), 0xFF, NO_LIMIT, NO_TIMEOUT)

    assert result.error is not None
    assert result.found == []


def test_discover_specifiers_zero_treated_as_all(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, op_home, op_bin = runtime_fixture(tmp_path, monkeypatch)

    write_package_holon(
        root / "cwd-alpha.holon",
        PackageSeed(slug="cwd-alpha", uuid="uuid-cwd-alpha", given_name="Cwd", family_name="Alpha"),
    )
    write_package_holon(
        root / ".op" / "build" / "built-beta.holon",
        PackageSeed(slug="built-beta", uuid="uuid-built-beta", given_name="Built", family_name="Beta"),
    )
    write_package_holon(
        op_bin / "installed-gamma.holon",
        PackageSeed(
            slug="installed-gamma",
            uuid="uuid-installed-gamma",
            given_name="Installed",
            family_name="Gamma",
        ),
    )
    write_package_holon(
        op_home / "cache" / "cached-delta.holon",
        PackageSeed(slug="cached-delta", uuid="uuid-cached-delta", given_name="Cached", family_name="Delta"),
    )

    all_result = Discover(LOCAL, None, str(root), ALL, NO_LIMIT, NO_TIMEOUT)
    zero_result = Discover(LOCAL, None, str(root), 0, NO_LIMIT, NO_TIMEOUT)

    assert all_result.error is None
    assert zero_result.error is None
    assert sorted_slugs(all_result) == sorted_slugs(zero_result)


def test_discover_null_expression_returns_all(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, _, _ = runtime_fixture(tmp_path, monkeypatch)
    write_package_holon(
        root / "alpha.holon",
        PackageSeed(slug="alpha", uuid="uuid-alpha", given_name="Alpha", family_name="One"),
    )
    write_package_holon(
        root / "beta.holon",
        PackageSeed(slug="beta", uuid="uuid-beta", given_name="Beta", family_name="Two"),
    )

    result = Discover(LOCAL, None, str(root), CWD, NO_LIMIT, NO_TIMEOUT)

    assert result.error is None
    assert len(result.found) == 2


def test_discover_missing_expression_returns_empty(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, _, _ = runtime_fixture(tmp_path, monkeypatch)
    write_package_holon(
        root / "alpha.holon",
        PackageSeed(slug="alpha", uuid="uuid-alpha", given_name="Alpha", family_name="One"),
    )

    result = Discover(LOCAL, "missing", str(root), CWD, NO_LIMIT, NO_TIMEOUT)

    assert result.error is None
    assert result.found == []


def test_discover_skips_excluded_dirs(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, _, _ = runtime_fixture(tmp_path, monkeypatch)

    write_package_holon(
        root / "kept" / "alpha.holon",
        PackageSeed(slug="alpha", uuid="uuid-alpha", given_name="Alpha", family_name="One"),
    )
    for skipped in [
        root / ".git" / "hidden" / "ignored.holon",
        root / ".op" / "hidden" / "ignored.holon",
        root / "node_modules" / "hidden" / "ignored.holon",
        root / "vendor" / "hidden" / "ignored.holon",
        root / "build" / "hidden" / "ignored.holon",
        root / "testdata" / "hidden" / "ignored.holon",
        root / ".cache" / "hidden" / "ignored.holon",
    ]:
        write_package_holon(
            skipped,
            PackageSeed(
                slug=f"ignored-{skipped.parent.name}",
                uuid=f"uuid-{skipped.parent.name}",
                given_name="Ignored",
                family_name="Holon",
            ),
        )

    result = Discover(LOCAL, None, str(root), CWD, NO_LIMIT, NO_TIMEOUT)

    assert result.error is None
    assert sorted_slugs(result) == ["alpha"]


def test_discover_deduplicates_by_uuid(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, _, _ = runtime_fixture(tmp_path, monkeypatch)
    cwd_path = root / "alpha.holon"
    built_path = root / ".op" / "build" / "alpha-built.holon"
    write_package_holon(
        cwd_path,
        PackageSeed(slug="alpha", uuid="uuid-alpha", given_name="Alpha", family_name="One"),
    )
    write_package_holon(
        built_path,
        PackageSeed(slug="alpha-built", uuid="uuid-alpha", given_name="Alpha", family_name="One"),
    )

    result = Discover(LOCAL, None, str(root), ALL, NO_LIMIT, NO_TIMEOUT)

    assert result.error is None
    assert len(result.found) == 1
    assert result.found[0].url == file_url(cwd_path)


def test_discover_holon_json_fast_path(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, _, _ = runtime_fixture(tmp_path, monkeypatch)
    write_package_holon(
        root / "alpha.holon",
        PackageSeed(slug="alpha", uuid="uuid-alpha", given_name="Alpha", family_name="One"),
    )

    probe_calls = 0
    original_probe = discover_module._probe_package_entry

    def probe(*args, **kwargs):
        nonlocal probe_calls
        probe_calls += 1
        return original_probe(*args, **kwargs)

    monkeypatch.setattr(discover_module, "_probe_package_entry", probe)

    result = Discover(LOCAL, None, str(root), CWD, NO_LIMIT, NO_TIMEOUT)

    assert result.error is None
    assert probe_calls == 0


def test_discover_describe_fallback_when_holon_json_missing(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    root, _, _ = runtime_fixture(tmp_path, monkeypatch)
    write_package_holon(
        root / "alpha.holon",
        PackageSeed(slug="alpha", uuid="uuid-alpha", given_name="Alpha", family_name="One"),
        with_holon_json=False,
        with_binary=True,
    )

    result = Discover(LOCAL, None, str(root), CWD, NO_LIMIT, 5000)

    assert result.error is None
    assert len(result.found) == 1
    assert result.found[0].info is not None
    assert result.found[0].info.slug == "echo-server"


def test_discover_siblings_layer(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, _, _ = runtime_fixture(tmp_path, monkeypatch)
    app_executable = root / "TestApp.app" / "Contents" / "MacOS" / "TestApp"
    app_executable.parent.mkdir(parents=True, exist_ok=True)
    app_executable.write_text("#!/bin/sh\n", encoding="utf-8")
    bundle_root = root / "TestApp.app" / "Contents" / "Resources" / "Holons"
    write_package_holon(
        bundle_root / "bundle.holon",
        PackageSeed(slug="bundle", uuid="uuid-bundle", given_name="Bundle", family_name="Holon"),
    )
    monkeypatch.setattr(discover_module.sys, "executable", str(app_executable))

    result = Discover(LOCAL, None, str(root), SIBLINGS, NO_LIMIT, NO_TIMEOUT)

    assert result.error is None
    assert sorted_slugs(result) == ["bundle"]


def test_discover_source_layer_offloads_to_local_op(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, _, _ = runtime_fixture(tmp_path, monkeypatch)
    source_dir = root / "source-holon"
    source_dir.mkdir(parents=True)

    calls: list[tuple[int, str | None, str, int, int, int]] = []

    def fake_source_bridge(
        scope: int,
        expression: str | None,
        bridge_root: str,
        specifiers: int,
        limit: int,
        timeout: int,
    ) -> DiscoverResult:
        calls.append((scope, expression, bridge_root, specifiers, limit, timeout))
        return DiscoverResult(
            found=[
                HolonRef(
                    url=file_url(source_dir),
                    info=HolonInfo(
                        slug="source-alpha",
                        uuid="uuid-source-alpha",
                        identity=IdentityInfo(given_name="Source", family_name="Alpha"),
                        lang="python",
                        runner="python",
                        status="draft",
                        kind="native",
                        transport="stdio",
                        entrypoint="alpha",
                        architectures=[],
                        has_dist=False,
                        has_source=True,
                    ),
                    error=None,
                )
            ],
            error=None,
        )

    monkeypatch.setattr(discover_module, "_discover_source_with_local_op", fake_source_bridge)

    result = Discover(LOCAL, None, str(root), SOURCE, NO_LIMIT, 5000)

    assert result.error is None
    assert sorted_slugs(result) == ["source-alpha"]
    assert calls == [(LOCAL, None, str(root.resolve()), SOURCE, NO_LIMIT, 5000)]


def test_discover_built_layer(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, _, _ = runtime_fixture(tmp_path, monkeypatch)
    write_package_holon(
        root / ".op" / "build" / "built.holon",
        PackageSeed(slug="built", uuid="uuid-built", given_name="Built", family_name="Holon"),
    )

    result = Discover(LOCAL, None, str(root), BUILT, NO_LIMIT, NO_TIMEOUT)

    assert result.error is None
    assert sorted_slugs(result) == ["built"]


def test_discover_installed_layer(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, _, op_bin = runtime_fixture(tmp_path, monkeypatch)
    write_package_holon(
        op_bin / "installed.holon",
        PackageSeed(slug="installed", uuid="uuid-installed", given_name="Installed", family_name="Holon"),
    )

    result = Discover(LOCAL, None, str(root), INSTALLED, NO_LIMIT, NO_TIMEOUT)

    assert result.error is None
    assert sorted_slugs(result) == ["installed"]


def test_discover_cached_layer(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, op_home, _ = runtime_fixture(tmp_path, monkeypatch)
    write_package_holon(
        op_home / "cache" / "deep" / "cached.holon",
        PackageSeed(slug="cached", uuid="uuid-cached", given_name="Cached", family_name="Holon"),
    )

    result = Discover(LOCAL, None, str(root), CACHED, NO_LIMIT, NO_TIMEOUT)

    assert result.error is None
    assert sorted_slugs(result) == ["cached"]


def test_discover_nil_root_defaults_to_cwd(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, _, _ = runtime_fixture(tmp_path, monkeypatch)
    write_package_holon(
        root / "alpha.holon",
        PackageSeed(slug="alpha", uuid="uuid-alpha", given_name="Alpha", family_name="One"),
    )
    monkeypatch.chdir(root)

    result = Discover(LOCAL, None, None, CWD, NO_LIMIT, NO_TIMEOUT)

    assert result.error is None
    assert sorted_slugs(result) == ["alpha"]


def test_discover_empty_root_returns_error() -> None:
    result = Discover(LOCAL, None, "", ALL, NO_LIMIT, NO_TIMEOUT)

    assert result.error is not None
    assert result.found == []


def test_discover_unsupported_scope_returns_error(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root, _, _ = runtime_fixture(tmp_path, monkeypatch)

    proxy_result = Discover(PROXY, None, str(root), ALL, NO_LIMIT, NO_TIMEOUT)
    delegated_result = Discover(DELEGATED, None, str(root), ALL, NO_LIMIT, NO_TIMEOUT)

    assert proxy_result.error is not None
    assert delegated_result.error is not None
