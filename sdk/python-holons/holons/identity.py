from __future__ import annotations

"""Resolve holon manifest data from holon.proto files."""

from dataclasses import dataclass, field
from pathlib import Path
import re

PROTO_MANIFEST_FILE_NAME = "holon.proto"

_MANIFEST_OPTION_RE = re.compile(
    r"option\s*\(\s*holons\.v1\.manifest\s*\)\s*=\s*\{",
    re.DOTALL,
)


@dataclass
class HolonIdentity:
    """Parsed identity from a holon.proto manifest."""

    uuid: str = ""
    given_name: str = ""
    family_name: str = ""
    motto: str = ""
    composer: str = ""
    clade: str = ""
    status: str = ""
    born: str = ""
    version: str = ""
    lang: str = ""
    parents: list[str] = field(default_factory=list)
    reproduction: str = ""
    generated_by: str = ""
    proto_status: str = ""
    aliases: list[str] = field(default_factory=list)

    def slug(self) -> str:
        given = self.given_name.strip()
        family = self.family_name.strip().removesuffix("?")
        if not given and not family:
            return ""
        return f"{given}-{family}".strip().lower().replace(" ", "-").strip("-")


@dataclass
class ResolvedSkill:
    name: str = ""
    description: str = ""
    when: str = ""
    steps: list[str] = field(default_factory=list)


@dataclass
class ResolvedSequenceParam:
    name: str = ""
    description: str = ""
    required: bool = False
    default: str = ""


@dataclass
class ResolvedSequence:
    name: str = ""
    description: str = ""
    params: list[ResolvedSequenceParam] = field(default_factory=list)
    steps: list[str] = field(default_factory=list)


@dataclass
class ResolvedManifest:
    identity: HolonIdentity = field(default_factory=HolonIdentity)
    source_path: str = ""
    description: str = ""
    kind: str = ""
    build_runner: str = ""
    build_main: str = ""
    artifact_binary: str = ""
    artifact_primary: str = ""
    required_files: list[str] = field(default_factory=list)
    member_paths: list[str] = field(default_factory=list)
    skills: list[ResolvedSkill] = field(default_factory=list)
    sequences: list[ResolvedSequence] = field(default_factory=list)


def parse_holon(path: str | Path) -> HolonIdentity:
    """Parse a holon.proto file and return its identity."""
    return resolve_proto_file(path).identity


def parse_manifest(path: str | Path) -> ResolvedManifest:
    """Parse manifest fields from a specific holon.proto file."""
    return resolve_proto_file(path)


def resolve(root: str | Path) -> ResolvedManifest:
    """Resolve a holon's manifest from a directory or proto path."""
    candidate = Path(root).expanduser()
    if candidate.is_file():
        return resolve_proto_file(candidate)

    proto_files = _collect_proto_files(candidate)
    if not proto_files:
        raise FileNotFoundError(f"no {PROTO_MANIFEST_FILE_NAME} found in {candidate}")

    last_error: Exception | None = None
    for proto_path in proto_files:
        try:
            return resolve_proto_file(proto_path)
        except Exception as exc:  # pragma: no cover - guarded by tests via success path
            last_error = exc

    if last_error is not None:
        raise last_error
    raise FileNotFoundError(f"no {PROTO_MANIFEST_FILE_NAME} found in {candidate}")


def resolve_manifest(root: str | Path) -> tuple[HolonIdentity, str]:
    """Preserve the original identity-plus-source-path API."""
    resolved = resolve(root)
    return resolved.identity, resolved.source_path


def resolve_proto_file(path: str | Path) -> ResolvedManifest:
    """Resolve a specific holon.proto file into manifest data."""
    path = Path(path).expanduser().resolve()
    if path.name != PROTO_MANIFEST_FILE_NAME:
        raise ValueError(f"{path} is not a {PROTO_MANIFEST_FILE_NAME} file")

    text = path.read_text(encoding="utf-8")
    manifest_block = _extract_manifest_block(text)
    if manifest_block is None:
        raise ValueError(f"{path}: missing holons.v1.manifest option in holon.proto")

    manifest = ResolvedManifest(source_path=str(path))

    identity_block = _extract_block("identity", manifest_block)
    if identity_block is not None:
        manifest.identity.uuid = _scalar("uuid", identity_block)
        manifest.identity.given_name = _scalar("given_name", identity_block)
        manifest.identity.family_name = _scalar("family_name", identity_block)
        manifest.identity.motto = _scalar("motto", identity_block)
        manifest.identity.composer = _scalar("composer", identity_block)
        manifest.identity.clade = _scalar("clade", identity_block)
        manifest.identity.status = _scalar("status", identity_block)
        manifest.identity.born = _scalar("born", identity_block)
        manifest.identity.version = _scalar("version", identity_block)
        manifest.identity.proto_status = _scalar("proto_status", identity_block)
        manifest.identity.aliases = _compact_strings(_string_list("aliases", identity_block))

    lineage_block = _extract_block("lineage", manifest_block)
    if lineage_block is not None:
        manifest.identity.parents = _compact_strings(_string_list("parents", lineage_block))
        manifest.identity.reproduction = _scalar("reproduction", lineage_block)
        manifest.identity.generated_by = _scalar("generated_by", lineage_block)

    manifest.description = _scalar("description", manifest_block)
    manifest.identity.lang = _scalar("lang", manifest_block)
    manifest.kind = _scalar("kind", manifest_block)

    build_block = _extract_block("build", manifest_block)
    if build_block is not None:
        manifest.build_runner = _scalar("runner", build_block)
        manifest.build_main = _scalar("main", build_block)
        manifest.member_paths = _compact_strings(
            [
                _scalar("path", member_block)
                for member_block in _extract_repeated_blocks("members", build_block)
            ]
        )

    requires_block = _extract_block("requires", manifest_block)
    if requires_block is not None:
        manifest.required_files = _compact_strings(_string_list("files", requires_block))

    artifacts_block = _extract_block("artifacts", manifest_block)
    if artifacts_block is not None:
        manifest.artifact_binary = _scalar("binary", artifacts_block)
        manifest.artifact_primary = _scalar("primary", artifacts_block)

    manifest.skills = [
        ResolvedSkill(
            name=_scalar("name", skill_block),
            description=_scalar("description", skill_block),
            when=_scalar("when", skill_block),
            steps=_trimmed_strings(_string_list("steps", skill_block)),
        )
        for skill_block in _extract_repeated_blocks("skills", manifest_block)
    ]

    manifest.sequences = [
        ResolvedSequence(
            name=_scalar("name", sequence_block),
            description=_scalar("description", sequence_block),
            params=[
                ResolvedSequenceParam(
                    name=_scalar("name", param_block),
                    description=_scalar("description", param_block),
                    required=_bool("required", param_block),
                    default=_scalar("default", param_block),
                )
                for param_block in _extract_repeated_blocks("params", sequence_block)
            ],
            steps=_trimmed_strings(_string_list("steps", sequence_block)),
        )
        for sequence_block in _extract_repeated_blocks("sequences", manifest_block)
    ]

    return manifest


def find_holon_proto(root: str | Path) -> Path | None:
    """Locate a holon.proto file from a root directory or file path."""
    root = Path(root).expanduser()
    if root.is_file():
        return root.resolve() if root.name == PROTO_MANIFEST_FILE_NAME else None
    if not root.exists() or not root.is_dir():
        return None

    direct = root / PROTO_MANIFEST_FILE_NAME
    if direct.exists():
        return direct.resolve()

    api_v1 = root / "api" / "v1" / PROTO_MANIFEST_FILE_NAME
    if api_v1.exists():
        return api_v1.resolve()

    candidates = _collect_proto_files(root)
    return candidates[0] if candidates else None


def resolve_manifest_path(root: str | Path) -> Path:
    """Locate the nearest holon.proto relative to a proto directory or holon root."""
    root = Path(root).expanduser()
    search_roots = [root]
    if root.name == "protos":
        search_roots.append(root.parent)
    elif root.parent not in search_roots:
        search_roots.append(root.parent)

    for candidate_root in search_roots:
        candidate = find_holon_proto(candidate_root)
        if candidate is not None:
            return candidate

    raise FileNotFoundError(f"no {PROTO_MANIFEST_FILE_NAME} found near {root}")


def _collect_proto_files(root: Path) -> list[Path]:
    root = root.expanduser()
    if not root.exists() or not root.is_dir():
        return []

    files: list[Path] = []
    for path in sorted(root.rglob(PROTO_MANIFEST_FILE_NAME)):
        rel_parts = path.relative_to(root).parts
        if any(_should_skip_part(part) for part in rel_parts[:-1]):
            continue
        files.append(path.resolve())
    return files


def _should_skip_part(name: str) -> bool:
    return name in {".git", ".op", "node_modules", "vendor", "build", "gen", "testdata"} or (
        name.startswith(".") and not name.endswith(".holon")
    )


def _extract_manifest_block(source: str) -> str | None:
    match = _MANIFEST_OPTION_RE.search(source)
    if match is None:
        return None
    brace_index = source.find("{", match.start())
    if brace_index < 0:
        return None
    return _balanced_block_contents(source, brace_index)


def _extract_block(name: str, source: str) -> str | None:
    match = re.search(rf"\b{re.escape(name)}\s*:\s*\{{", source, re.DOTALL)
    if match is None:
        return None
    brace_index = source.find("{", match.start())
    if brace_index < 0:
        return None
    return _balanced_block_contents(source, brace_index)


def _extract_repeated_blocks(name: str, source: str) -> list[str]:
    blocks: list[str] = []
    for match in re.finditer(rf"\b{re.escape(name)}\s*:\s*\{{", source, re.DOTALL):
        brace_index = source.find("{", match.start())
        if brace_index < 0:
            continue
        block = _balanced_block_contents(source, brace_index)
        if block is not None:
            blocks.append(block)
    return blocks


def _scalar(name: str, source: str) -> str:
    quoted = re.search(rf'\b{re.escape(name)}\s*:\s*"((?:[^"\\]|\\.)*)"', source, re.DOTALL)
    if quoted is not None:
        return _unescape_proto_string(quoted.group(1))

    bare = re.search(rf"\b{re.escape(name)}\s*:\s*([^\s,\]}}]+)", source, re.DOTALL)
    if bare is not None:
        return bare.group(1)
    return ""


def _bool(name: str, source: str) -> bool:
    return _scalar(name, source).strip().lower() == "true"


def _string_list(name: str, source: str) -> list[str]:
    match = re.search(rf"\b{re.escape(name)}\s*:\s*\[(.*?)\]", source, re.DOTALL)
    if match is None:
        return []

    body = match.group(1)
    values: list[str] = []
    for token in re.finditer(r'"((?:[^"\\]|\\.)*)"|([^\s,\]]+)', body):
        quoted, bare = token.groups()
        if quoted is not None:
            values.append(_unescape_proto_string(quoted))
        elif bare is not None:
            values.append(bare)
    return values


def _balanced_block_contents(source: str, opening_brace: int) -> str | None:
    depth = 0
    inside_string = False
    escaped = False
    content_start = opening_brace + 1

    for index in range(opening_brace, len(source)):
        char = source[index]
        if inside_string:
            if escaped:
                escaped = False
            elif char == "\\":
                escaped = True
            elif char == '"':
                inside_string = False
            continue

        if char == '"':
            inside_string = True
        elif char == "{":
            depth += 1
        elif char == "}":
            depth -= 1
            if depth == 0:
                return source[content_start:index]

    return None


def _compact_strings(values: list[str]) -> list[str]:
    out: list[str] = []
    seen: set[str] = set()
    for value in values:
        trimmed = value.strip()
        if not trimmed or trimmed in seen:
            continue
        seen.add(trimmed)
        out.append(trimmed)
    return out


def _trimmed_strings(values: list[str]) -> list[str]:
    return [value.strip() for value in values if value.strip()]


def _unescape_proto_string(value: str) -> str:
    return value.replace(r"\"", '"').replace(r"\\", "\\")
