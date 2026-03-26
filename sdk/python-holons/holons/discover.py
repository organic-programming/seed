from __future__ import annotations

"""Discover holons by scanning for nearby holon.proto manifests."""

from dataclasses import dataclass, field
from pathlib import Path
import os
import re

from . import identity
from .identity import HolonIdentity, ResolvedManifest

_PROTO_VERSION_DIR_RE = re.compile(r"^v[0-9]+(?:[A-Za-z0-9._-]*)?$")


@dataclass
class HolonBuild:
    runner: str = ""
    main: str = ""


@dataclass
class HolonArtifacts:
    binary: str = ""
    primary: str = ""


@dataclass
class HolonManifest:
    kind: str = ""
    build: HolonBuild = field(default_factory=HolonBuild)
    artifacts: HolonArtifacts = field(default_factory=HolonArtifacts)


@dataclass
class HolonEntry:
    slug: str
    uuid: str
    dir: str
    relative_path: str
    origin: str
    identity: HolonIdentity
    manifest: HolonManifest | None
    source_kind: str = "source"
    package_root: str = ""
    runner: str = ""
    entrypoint: str = ""
    architectures: list[str] = field(default_factory=list)
    has_dist: bool = False
    has_source: bool = False


def discover(root: str | Path) -> list[HolonEntry]:
    return _discover_source_in_root(Path(root), "local")


def discover_local() -> list[HolonEntry]:
    return discover(Path.cwd())


def discover_all() -> list[HolonEntry]:
    entries: list[HolonEntry] = []
    seen: set[str] = set()
    for root, origin in (
        (Path.cwd(), "local"),
        (_opbin(), "$OPBIN"),
        (_cache_dir(), "cache"),
    ):
        for entry in _discover_source_in_root(root, origin):
            key = entry.uuid.strip() or entry.dir
            if key in seen:
                continue
            seen.add(key)
            entries.append(entry)
    return entries


def find_by_slug(slug: str) -> HolonEntry | None:
    needle = slug.strip()
    if not needle:
        return None

    match: HolonEntry | None = None
    for entry in discover_all():
        if entry.slug != needle:
            continue
        if match is not None and match.uuid != entry.uuid:
            raise ValueError(f'ambiguous holon "{needle}"')
        match = entry
    return match


def find_by_uuid(prefix: str) -> HolonEntry | None:
    needle = prefix.strip()
    if not needle:
        return None

    match: HolonEntry | None = None
    for entry in discover_all():
        if not entry.uuid.startswith(needle):
            continue
        if match is not None and match.uuid != entry.uuid:
            raise ValueError(f'ambiguous UUID prefix "{needle}"')
        match = entry
    return match


def _discover_source_in_root(root: Path, origin: str) -> list[HolonEntry]:
    root = (root if str(root).strip() else Path.cwd()).expanduser().resolve()
    if not root.exists() or not root.is_dir():
        return []

    proto_files = sorted(
        path
        for path in root.rglob(identity.PROTO_MANIFEST_FILE_NAME)
        if not any(_should_skip_dir_name(part) for part in path.relative_to(root).parts[:-1])
    )

    entries_by_key: dict[str, HolonEntry] = {}
    ordered_keys: list[str] = []
    for proto_path in proto_files:
        try:
            resolved = identity.resolve_proto_file(proto_path)
        except Exception:
            continue
        if resolved.identity.given_name == "" and resolved.identity.family_name == "":
            continue

        holon_dir = _proto_holon_dir(root, proto_path, resolved)
        entry = HolonEntry(
            slug=_slug_for(resolved.identity),
            uuid=resolved.identity.uuid,
            dir=str(holon_dir),
            relative_path=_relative_path(root, holon_dir),
            origin=origin,
            identity=resolved.identity,
            manifest=_manifest_from_resolved(resolved),
            runner=resolved.build_runner,
            entrypoint=resolved.artifact_binary,
        )
        key = entry.uuid.strip() or entry.dir
        existing = entries_by_key.get(key)
        if existing is not None:
            if _should_replace_entry(existing, entry):
                entries_by_key[key] = entry
            continue

        entries_by_key[key] = entry
        ordered_keys.append(key)

    entries = [entries_by_key[key] for key in ordered_keys if key in entries_by_key]
    entries.sort(key=lambda entry: (entry.relative_path, entry.uuid))
    return entries


def _manifest_from_resolved(resolved: ResolvedManifest) -> HolonManifest:
    return HolonManifest(
        kind=resolved.kind,
        build=HolonBuild(
            runner=resolved.build_runner,
            main=resolved.build_main,
        ),
        artifacts=HolonArtifacts(
            binary=resolved.artifact_binary,
            primary=resolved.artifact_primary,
        ),
    )


def _proto_holon_dir(root: Path, proto_path: Path, resolved: ResolvedManifest) -> Path:
    proto_dir = proto_path.parent.resolve()
    best_dir = proto_dir
    best_score = 0
    best_depth = _path_depth(_relative_path(root, proto_dir))

    candidate = proto_dir
    while _is_within_root(root, candidate):
        score = _proto_candidate_score(candidate, resolved)
        depth = _path_depth(_relative_path(root, candidate))
        if score > best_score or (score == best_score and score > 0 and depth > best_depth):
            best_dir = candidate
            best_score = score
            best_depth = depth

        if candidate == root:
            break
        parent = candidate.parent
        if parent == candidate:
            break
        candidate = parent

    if best_score > 0:
        return best_dir

    if _PROTO_VERSION_DIR_RE.match(proto_dir.name):
        parent = proto_dir.parent
        if _is_within_root(root, parent):
            return parent.resolve()

    return proto_dir


def _proto_candidate_score(base: Path, resolved: ResolvedManifest) -> int:
    score = 0
    for required_file in resolved.required_files:
        if _proto_candidate_path_exists(base, required_file):
            score += 1
    if _proto_candidate_path_exists(base, resolved.build_main):
        score += 1
    for member_path in resolved.member_paths:
        if _proto_candidate_path_exists(base, member_path):
            score += 1
    if _proto_candidate_path_exists(base, resolved.artifact_primary):
        score += 1
    return score


def _proto_candidate_path_exists(base: Path, relative_path: str) -> bool:
    candidate = _resolve_proto_candidate_path(base, relative_path)
    return candidate is not None and candidate.exists()


def _resolve_proto_candidate_path(base: Path, relative_path: str) -> Path | None:
    trimmed = relative_path.strip()
    if not trimmed:
        return None

    candidate = (base / Path(trimmed)).resolve()
    return candidate if _is_within_root(base, candidate) else None


def _slug_for(identity_value: HolonIdentity) -> str:
    return identity_value.slug()


def _should_skip_dir_name(name: str) -> bool:
    return name in {".git", ".op", "node_modules", "vendor", "build", "testdata"} or (
        name.startswith(".") and not name.endswith(".holon")
    )


def _relative_path(root: Path, holon_dir: Path) -> str:
    try:
        rel = holon_dir.resolve().relative_to(root.resolve())
        return "." if str(rel) == "." else rel.as_posix()
    except ValueError:
        return holon_dir.resolve().as_posix()


def _path_depth(relative_path: str) -> int:
    trimmed = relative_path.strip().strip("/")
    if not trimmed or trimmed == ".":
        return 0
    return len(trimmed.split("/"))


def _should_replace_entry(current: HolonEntry, next_entry: HolonEntry) -> bool:
    return _path_depth(next_entry.relative_path) < _path_depth(current.relative_path)


def _is_within_root(root: Path, candidate: Path) -> bool:
    try:
        candidate.resolve().relative_to(root.resolve())
        return True
    except ValueError:
        return False


def _op_path() -> Path:
    configured = os.environ.get("OPPATH", "").strip()
    if configured:
        return Path(configured).expanduser().resolve()
    return Path.home().joinpath(".op")


def _opbin() -> Path:
    configured = os.environ.get("OPBIN", "").strip()
    if configured:
        return Path(configured).expanduser().resolve()
    return _op_path().joinpath("bin")


def _cache_dir() -> Path:
    return _op_path().joinpath("cache")
