"""Tests for holons.identity holon.proto parsing."""

from pathlib import Path

import pytest

from holons.identity import (
    PROTO_MANIFEST_FILE_NAME,
    parse_holon,
    resolve,
    resolve_manifest,
    resolve_manifest_path,
    resolve_proto_file,
)


def test_parse_holon(tmp_path):
    holon_proto = tmp_path / "holon.proto"
    holon_proto.write_text(
        'syntax = "proto3";\n'
        "\n"
        "package test.v1;\n"
        "\n"
        "option (holons.v1.manifest) = {\n"
        "  identity: {\n"
        '    uuid: "abc-123"\n'
        '    given_name: "test-holon"\n'
        '    family_name: "Test"\n'
        '    motto: "A test holon."\n'
        '    composer: "Tester"\n'
        '    clade: "deterministic/pure"\n'
        '    status: "draft"\n'
        '    born: "2026-01-01"\n'
        '    proto_status: "draft"\n'
        "  }\n"
        "  lineage: {\n"
        "    parents: []\n"
        '    reproduction: "manual"\n'
        '    generated_by: "dummy-test"\n'
        "  }\n"
        '  lang: "python"\n'
        "};\n"
    )

    identity = parse_holon(holon_proto)
    assert identity.uuid == "abc-123"
    assert identity.given_name == "test-holon"
    assert identity.family_name == "Test"
    assert identity.motto == "A test holon."
    assert identity.clade == "deterministic/pure"
    assert identity.lang == "python"


def test_parse_holon_missing_fields(tmp_path):
    holon_proto = tmp_path / "holon.proto"
    holon_proto.write_text(
        'syntax = "proto3";\n'
        "\n"
        "package test.v1;\n"
        "\n"
        "option (holons.v1.manifest) = {\n"
        "  identity: {\n"
        '    uuid: "minimal"\n'
        "  }\n"
        "};\n"
    )

    identity = parse_holon(holon_proto)
    assert identity.uuid == "minimal"
    assert identity.given_name == ""


def test_parse_holon_invalid_mapping(tmp_path):
    holon_proto = tmp_path / "holon.proto"
    holon_proto.write_text(
        'syntax = "proto3";\n'
        "\n"
        "package test.v1;\n"
        "\n"
        "message Placeholder {}\n"
    )

    try:
        parse_holon(holon_proto)
        assert False, "should have raised"
    except ValueError as e:
        assert "missing holons.v1.manifest" in str(e)


def test_resolve_proto_file_returns_extended_manifest_fields(tmp_path: Path):
    holon_proto = tmp_path / PROTO_MANIFEST_FILE_NAME
    holon_proto.write_text(
        'syntax = "proto3";\n'
        "\n"
        "package test.v1;\n"
        "\n"
        "option (holons.v1.manifest) = {\n"
        '  description: "Sample holon."\n'
        "  identity: {\n"
        '    uuid: "resolve-uuid"\n'
        '    given_name: "Resolve"\n'
        '    family_name: "Proto"\n'
        '    version: "1.2.3"\n'
        '    aliases: ["resolve-proto", "resolver"]\n'
        "  }\n"
        '  lang: "python"\n'
        '  kind: "native"\n'
        "  build: {\n"
        '    runner: "python"\n'
        '    main: "./cmd/main.py"\n'
        '    members: { path: "members/alpha" }\n'
        '    members: { path: "members/beta" }\n'
        "  }\n"
        "  requires: {\n"
        '    files: ["pyproject.toml", "README.md"]\n'
        "  }\n"
        "  artifacts: {\n"
        '    binary: "resolve-proto"\n'
        '    primary: "dist/main.py"\n'
        "  }\n"
        "  skills: {\n"
        '    name: "echo"\n'
        '    description: "Echo a value."\n'
        '    when: "When testing."\n'
        '    steps: ["Call Ping.", "Inspect output."]\n'
        "  }\n"
        "  sequences: {\n"
        '    name: "ping-once"\n'
        '    description: "Run one ping."\n'
        '    params: { name: "message" description: "Message to send." required: true default: "hello" }\n'
        '    steps: ["Ping once."]\n'
        "  }\n"
        "};\n"
    )

    resolved = resolve_proto_file(holon_proto)
    assert resolved.source_path == str(holon_proto.resolve())
    assert resolved.description == "Sample holon."
    assert resolved.identity.version == "1.2.3"
    assert resolved.identity.aliases == ["resolve-proto", "resolver"]
    assert resolved.build_runner == "python"
    assert resolved.build_main == "./cmd/main.py"
    assert resolved.artifact_binary == "resolve-proto"
    assert resolved.artifact_primary == "dist/main.py"
    assert resolved.required_files == ["pyproject.toml", "README.md"]
    assert resolved.member_paths == ["members/alpha", "members/beta"]
    assert resolved.skills[0].name == "echo"
    assert resolved.sequences[0].name == "ping-once"
    assert resolved.sequences[0].params[0].required is True


def test_resolve_scans_directory_and_preserves_original_api(tmp_path: Path):
    nested = tmp_path / "api" / "v1"
    nested.mkdir(parents=True)
    holon_proto = nested / PROTO_MANIFEST_FILE_NAME
    holon_proto.write_text(
        'syntax = "proto3";\n'
        'option (holons.v1.manifest) = { identity: { uuid: "dir-uuid" given_name: "Dir" family_name: "Resolve" } };\n'
    )

    resolved = resolve(tmp_path)
    assert resolved.identity.uuid == "dir-uuid"
    assert resolved.source_path == str(holon_proto.resolve())

    identity_value, source_path = resolve_manifest(tmp_path)
    assert identity_value.uuid == "dir-uuid"
    assert source_path == str(holon_proto.resolve())


def test_resolve_manifest_path_prefers_api_layout(tmp_path: Path):
    api_v1 = tmp_path / "api" / "v1"
    api_v1.mkdir(parents=True)
    holon_proto = api_v1 / PROTO_MANIFEST_FILE_NAME
    holon_proto.write_text(
        'syntax = "proto3";\n'
        'option (holons.v1.manifest) = { identity: { uuid: "api-uuid" } };\n'
    )

    assert resolve_manifest_path(tmp_path / "protos") == holon_proto.resolve()


def test_resolve_proto_file_requires_holon_proto_name(tmp_path: Path):
    other = tmp_path / "not-holon.proto"
    other.write_text('syntax = "proto3";\n', encoding="utf-8")

    with pytest.raises(ValueError, match="is not a holon.proto file"):
        resolve_proto_file(other)
