from __future__ import annotations

from pathlib import Path

from holons.discover import discover, discover_local, find_by_slug, find_by_uuid


def write_holon(root: Path, relative_dir: str, *, uuid: str, given_name: str, family_name: str, binary: str) -> None:
    directory = root / relative_dir
    directory.mkdir(parents=True, exist_ok=True)
    (directory / "holon.proto").write_text(
        "\n".join(
            [
                'syntax = "proto3";',
                "",
                "package test.v1;",
                "",
                "option (holons.v1.manifest) = {",
                "  identity: {",
                f'    uuid: "{uuid}"',
                f'    given_name: "{given_name}"',
                f'    family_name: "{family_name}"',
                '    motto: "Test"',
                '    composer: "test"',
                '    clade: "deterministic/pure"',
                '    status: "draft"',
                '    born: "2026-03-07"',
                "  }",
                "  lineage: {",
                '    generated_by: "test"',
                "  }",
                '  kind: "native"',
                "  build: {",
                '    runner: "go-module"',
                "  }",
                "  artifacts: {",
                f'    binary: "{binary}"',
                "  }",
                "};",
                "",
            ]
        ),
        encoding="utf-8",
    )


def test_discover_recurses_skips_and_dedups(tmp_path: Path) -> None:
    write_holon(tmp_path, "holons/alpha", uuid="uuid-alpha", given_name="Alpha", family_name="Go", binary="alpha-go")
    write_holon(tmp_path, "nested/beta", uuid="uuid-beta", given_name="Beta", family_name="Rust", binary="beta-rust")
    write_holon(
        tmp_path,
        "nested/dup/alpha",
        uuid="uuid-alpha",
        given_name="Alpha",
        family_name="Go",
        binary="alpha-go",
    )

    for skipped in [
        ".git/hidden",
        ".op/hidden",
        "node_modules/hidden",
        "vendor/hidden",
        "build/hidden",
        "testdata/hidden",
        ".cache/hidden",
    ]:
        write_holon(
            tmp_path,
            skipped,
            uuid=f"ignored-{Path(skipped).name}",
            given_name="Ignored",
            family_name="Holon",
            binary="ignored-holon",
        )

    entries = discover(tmp_path)
    assert len(entries) == 2

    alpha = next(entry for entry in entries if entry.uuid == "uuid-alpha")
    assert alpha.slug == "alpha-go"
    assert alpha.relative_path == "holons/alpha"
    assert alpha.manifest is not None
    assert alpha.manifest.build.runner == "go-module"

    beta = next(entry for entry in entries if entry.uuid == "uuid-beta")
    assert beta.relative_path == "nested/beta"


def test_discover_local_and_find_helpers(tmp_path: Path, monkeypatch) -> None:
    write_holon(
        tmp_path,
        "rob-go",
        uuid="c7f3a1b2-1111-1111-1111-111111111111",
        given_name="Rob",
        family_name="Go",
        binary="rob-go",
    )

    monkeypatch.chdir(tmp_path)
    monkeypatch.setenv("OPPATH", str(tmp_path / "runtime"))
    monkeypatch.setenv("OPBIN", str(tmp_path / "runtime" / "bin"))

    local = discover_local()
    assert len(local) == 1
    assert local[0].slug == "rob-go"

    by_slug = find_by_slug("rob-go")
    assert by_slug is not None
    assert by_slug.uuid == "c7f3a1b2-1111-1111-1111-111111111111"

    by_uuid = find_by_uuid("c7f3a1b2")
    assert by_uuid is not None
    assert by_uuid.slug == "rob-go"

    assert find_by_slug("missing") is None


def test_discover_proto_backed_holon_uses_manifest_and_holon_root(tmp_path: Path) -> None:
    holon_dir = tmp_path / "proto-holon"
    proto_dir = holon_dir / "v1"
    proto_dir.mkdir(parents=True)
    (holon_dir / "cmd" / "daemon").mkdir(parents=True)
    (holon_dir / "go.mod").write_text("module example.com/protoholon\n", encoding="utf-8")
    (holon_dir / "cmd" / "daemon" / "main.go").write_text("package main\n", encoding="utf-8")
    (proto_dir / "holon.proto").write_text(
        "\n".join(
            [
                'syntax = "proto3";',
                "",
                "package proto.v1;",
                "",
                "option (holons.v1.manifest) = {",
                "  identity: {",
                '    uuid: "proto-uuid"',
                '    given_name: "Proto"',
                '    family_name: "Holon"',
                '    motto: "Proto-backed holon."',
                "  }",
                '  kind: "native"',
                '  lang: "go"',
                "  build: {",
                '    runner: "go-module"',
                '    main: "./cmd/daemon"',
                "  }",
                "  requires: {",
                '    files: ["go.mod"]',
                "  }",
                "  artifacts: {",
                '    binary: "proto-holon"',
                "  }",
                "};",
                "",
            ]
        ),
        encoding="utf-8",
    )

    entries = discover(tmp_path)
    assert len(entries) == 1

    entry = entries[0]
    assert entry.slug == "proto-holon"
    assert entry.relative_path == "proto-holon"
    assert entry.dir == str(holon_dir.resolve())
    assert entry.manifest is not None
    assert entry.manifest.build.runner == "go-module"
    assert entry.manifest.build.main == "./cmd/daemon"
    assert entry.manifest.artifacts.binary == "proto-holon"
