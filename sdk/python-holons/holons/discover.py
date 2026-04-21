from __future__ import annotations

"""Uniform holon discovery for python-holons."""

from dataclasses import dataclass
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess
import sys
from typing import Callable
from urllib.parse import urlparse, unquote

import grpc

from . import grpcclient
from .discovery_types import (
    ALL,
    BUILT,
    CACHED,
    CWD,
    INSTALLED,
    LOCAL,
    NO_LIMIT,
    SIBLINGS,
    SOURCE,
    DiscoverResult,
    HolonInfo,
    HolonRef,
    IdentityInfo,
    ResolveResult,
)
from holons.v1 import describe_pb2, describe_pb2_grpc

_EXCLUDED_DIRS = {".git", ".op", "node_modules", "vendor", "build", "testdata"}
_DEFAULT_DESCRIBE_TIMEOUT_SECONDS = 5.0


@dataclass
class _DiscoveredEntry:
    ref: HolonRef
    dir_path: str
    relative_path: str


def Discover(
    scope: int,
    expression: str | None,
    root: str | None,
    specifiers: int,
    limit: int,
    timeout: int,
) -> DiscoverResult:
    if scope != LOCAL:
        return DiscoverResult(found=[], error=f"scope {scope} not supported")
    if specifiers < 0 or specifiers & ~ALL:
        return DiscoverResult(
            found=[],
            error=f"invalid specifiers 0x{specifiers:02X}: valid range is 0x00-0x3F",
        )
    if specifiers == 0:
        specifiers = ALL
    if limit < 0:
        return DiscoverResult(found=[], error=None)

    normalized_expression = _normalized_expression(expression)
    search_root: str | None = None

    def resolve_root() -> str:
        nonlocal search_root
        if search_root is None:
            search_root = _resolve_discover_root(root)
        return search_root

    try:
        if normalized_expression is not None:
            refs, handled = _discover_path_expression(normalized_expression, resolve_root, timeout)
            if handled:
                return DiscoverResult(found=_apply_ref_limit(refs, limit), error=None)

        entries = _discover_entries(resolve_root(), specifiers, timeout)
    except ValueError as exc:
        return DiscoverResult(found=[], error=str(exc))

    found: list[HolonRef] = []
    for entry in entries:
        if not _matches_expression(entry, normalized_expression):
            continue
        found.append(entry.ref)
        if limit > 0 and len(found) >= limit:
            break

    return DiscoverResult(found=found, error=None)


def resolve(
    scope: int,
    expression: str,
    root: str | None,
    specifiers: int,
    timeout: int,
) -> ResolveResult:
    result = Discover(scope, expression, root, specifiers, 1, timeout)
    if result.error is not None:
        return ResolveResult(ref=None, error=result.error)
    if not result.found:
        return ResolveResult(ref=None, error=f'holon "{expression}" not found')

    ref = result.found[0]
    if ref.error is not None:
        return ResolveResult(ref=ref, error=ref.error)
    return ResolveResult(ref=ref, error=None)


def _discover_entries(root: str, specifiers: int, timeout: int) -> list[_DiscoveredEntry]:
    layers: list[tuple[int, str, Callable[[str], list[_DiscoveredEntry]]]] = [
        (
            SIBLINGS,
            "siblings",
            lambda _root: _discover_packages_direct(_bundle_holons_root(), "siblings", timeout),
        ),
        (CWD, "cwd", lambda current_root: _discover_packages_recursive(current_root, "cwd", timeout)),
        (
            SOURCE,
            "source",
            lambda current_root: _entries_from_refs(
                current_root,
                _require_source_discovery_result(
                    _discover_source_with_local_op(
                        LOCAL,
                        None,
                        current_root,
                        SOURCE,
                        NO_LIMIT,
                        timeout,
                    )
                ).found,
            ),
        ),
        (
            BUILT,
            "built",
            lambda current_root: _discover_packages_direct(
                os.path.join(current_root, ".op", "build"),
                "built",
                timeout,
            ),
        ),
        (
            INSTALLED,
            "installed",
            lambda _root: _discover_packages_direct(_opbin(), "installed", timeout),
        ),
        (
            CACHED,
            "cached",
            lambda _root: _discover_packages_recursive(_cache_dir(), "cached", timeout),
        ),
    ]

    seen: set[str] = set()
    found: list[_DiscoveredEntry] = []

    for flag, _name, scan in layers:
        if specifiers & flag == 0:
            continue
        for entry in scan(root):
            key = _entry_key(entry)
            if key in seen:
                continue
            seen.add(key)
            found.append(entry)

    return found


def _discover_packages_direct(root: str, origin: str, timeout: int) -> list[_DiscoveredEntry]:
    return _discover_packages_from_dirs(root, origin, _package_dirs_direct(root), timeout)


def _discover_packages_recursive(root: str, origin: str, timeout: int) -> list[_DiscoveredEntry]:
    return _discover_packages_from_dirs(root, origin, _package_dirs_recursive(root), timeout)


def _discover_packages_from_dirs(
    root: str,
    origin: str,
    dirs: list[str],
    timeout: int,
) -> list[_DiscoveredEntry]:
    abs_root = _normalize_search_root(root)
    entries_by_key: dict[str, _DiscoveredEntry] = {}
    ordered_keys: list[str] = []

    for package_dir in dirs:
        try:
            entry = _load_package_entry(abs_root, package_dir)
        except Exception:
            try:
                entry = _probe_package_entry(abs_root, package_dir, timeout)
            except Exception:
                continue

        key = _entry_key(entry)
        existing = entries_by_key.get(key)
        if existing is not None:
            if _should_replace_entry(existing, entry):
                entries_by_key[key] = entry
            continue

        entries_by_key[key] = entry
        ordered_keys.append(key)

    entries = [entries_by_key[key] for key in ordered_keys if key in entries_by_key]
    entries.sort(key=lambda entry: (entry.relative_path, _entry_sort_key(entry)))
    return entries


def _load_package_entry(root: str, package_dir: str) -> _DiscoveredEntry:
    manifest_path = Path(package_dir).joinpath(".holon.json")
    payload = json.loads(manifest_path.read_text(encoding="utf-8"))
    schema = str(payload.get("schema", "")).strip()
    if schema and schema != "holon-package/v1":
        raise ValueError(f"unsupported package schema {schema!r}")

    identity_payload = payload.get("identity", {})
    aliases = _string_list(identity_payload.get("aliases"))
    identity = IdentityInfo(
        given_name=_trimmed(identity_payload.get("given_name")),
        family_name=_trimmed(identity_payload.get("family_name")),
        motto=_trimmed(identity_payload.get("motto")),
        aliases=aliases,
    )
    slug = _trimmed(payload.get("slug")) or _slug_for(identity.given_name, identity.family_name)
    info = HolonInfo(
        slug=slug,
        uuid=_trimmed(payload.get("uuid")),
        identity=identity,
        lang=_trimmed(payload.get("lang")),
        runner=_trimmed(payload.get("runner")),
        status=_trimmed(payload.get("status")),
        kind=_trimmed(payload.get("kind")),
        transport=_trimmed(payload.get("transport")),
        entrypoint=_trimmed(payload.get("entrypoint")),
        architectures=_string_list(payload.get("architectures")),
        has_dist=bool(payload.get("has_dist")),
        has_source=bool(payload.get("has_source")),
    )
    abs_dir = str(Path(package_dir).expanduser().resolve())
    return _DiscoveredEntry(
        ref=HolonRef(url=_file_url(abs_dir), info=info, error=None),
        dir_path=abs_dir,
        relative_path=_relative_path(root, abs_dir),
    )


def _probe_package_entry(root: str, package_dir: str, timeout: int) -> _DiscoveredEntry:
    info = _describe_package_directory(package_dir, timeout)
    package_path = Path(package_dir).expanduser().resolve()
    info.has_dist = package_path.joinpath("dist").is_dir() or info.has_dist
    info.has_source = package_path.joinpath("git").is_dir() or info.has_source
    if not info.architectures:
        info.architectures = _package_architectures(package_path)

    abs_dir = str(package_path)
    return _DiscoveredEntry(
        ref=HolonRef(url=_file_url(abs_dir), info=info, error=None),
        dir_path=abs_dir,
        relative_path=_relative_path(root, abs_dir),
    )


def _describe_package_directory(package_dir: str, timeout: int) -> HolonInfo:
    return _describe_binary_target(_package_binary_path(package_dir), timeout)


def _describe_binary_target(binary_path: str, timeout: int) -> HolonInfo:
    channel = grpcclient.dial_stdio(binary_path)
    describe_timeout = _describe_timeout_seconds(timeout)
    try:
        grpc.channel_ready_future(channel).result(timeout=describe_timeout)
        return _describe_channel(channel, describe_timeout)
    finally:
        channel.close()


def _describe_channel(channel: grpc.Channel, timeout_seconds: float) -> HolonInfo:
    stub = describe_pb2_grpc.HolonMetaStub(channel)
    response = stub.Describe(describe_pb2.DescribeRequest(), timeout=timeout_seconds)
    manifest = response.manifest if response is not None else None
    identity_payload = manifest.identity if manifest is not None else None
    if manifest is None or identity_payload is None:
        raise ValueError("Describe returned no manifest")

    identity = IdentityInfo(
        given_name=str(identity_payload.given_name),
        family_name=str(identity_payload.family_name),
        motto=str(identity_payload.motto),
        aliases=list(identity_payload.aliases),
    )
    return HolonInfo(
        slug=_slug_for(identity.given_name, identity.family_name),
        uuid=str(identity_payload.uuid),
        identity=identity,
        lang=str(manifest.lang),
        runner=str(manifest.build.runner) if manifest.HasField("build") else "",
        status=str(identity_payload.status),
        kind=str(manifest.kind),
        transport=str(manifest.transport),
        entrypoint=str(manifest.artifacts.binary) if manifest.HasField("artifacts") else "",
        architectures=list(manifest.platforms),
        has_dist=False,
        has_source=False,
    )


def _discover_path_expression(
    expression: str,
    resolve_root: Callable[[], str],
    timeout: int,
) -> tuple[list[HolonRef], bool]:
    candidate = _path_expression_candidate(expression, resolve_root)
    if candidate is None:
        return [], False
    ref = _discover_ref_at_path(candidate, timeout)
    if ref is None:
        return [], True
    return [ref], True


def _path_expression_candidate(expression: str, resolve_root: Callable[[], str]) -> str | None:
    trimmed = expression.strip()
    lowered = trimmed.lower()
    if not trimmed:
        return None
    if lowered.startswith("file://"):
        return _path_from_file_url(trimmed)
    if "://" in trimmed:
        return None
    if not (
        os.path.isabs(trimmed)
        or trimmed.startswith(".")
        or os.path.sep in trimmed
        or "/" in trimmed
        or "\\" in trimmed
        or lowered.endswith(".holon")
    ):
        return None
    if os.path.isabs(trimmed):
        return trimmed
    return os.path.join(resolve_root(), trimmed)


def _discover_ref_at_path(path: str, timeout: int) -> HolonRef | None:
    abs_path = str(Path(path).expanduser().resolve())
    candidate = Path(abs_path)
    if not candidate.exists():
        return None

    if candidate.is_dir():
        if candidate.name.endswith(".holon") or candidate.joinpath(".holon.json").is_file():
            try:
                return _load_package_entry(str(candidate.parent), abs_path).ref
            except Exception:
                try:
                    return _probe_package_entry(str(candidate.parent), abs_path, timeout).ref
                except Exception as exc:
                    return HolonRef(url=_file_url(abs_path), info=None, error=str(exc))

        result = _discover_source_with_local_op(LOCAL, None, abs_path, SOURCE, NO_LIMIT, timeout)
        if result.error is not None:
            raise ValueError(result.error)
        if len(result.found) == 1:
            return result.found[0]
        for ref in result.found:
            if _path_from_ref_url(ref.url) == abs_path:
                return ref
        return None

    if candidate.name == "holon.proto":
        result = _discover_source_with_local_op(
            LOCAL,
            None,
            str(candidate.parent),
            SOURCE,
            NO_LIMIT,
            timeout,
        )
        if result.error is not None:
            raise ValueError(result.error)
        if len(result.found) == 1:
            return result.found[0]
        for ref in result.found:
            if _path_from_ref_url(ref.url) == str(candidate.parent.resolve()):
                return ref
        return None

    try:
        info = _describe_binary_target(abs_path, timeout)
    except Exception as exc:
        return HolonRef(url=_file_url(abs_path), info=None, error=str(exc))
    return HolonRef(url=_file_url(abs_path), info=info, error=None)


def _require_source_discovery_result(result: DiscoverResult) -> DiscoverResult:
    if result.error is not None:
        raise ValueError(result.error)
    return result


def _entries_from_refs(root: str, refs: list[HolonRef]) -> list[_DiscoveredEntry]:
    entries: list[_DiscoveredEntry] = []
    for ref in refs:
        path = _path_from_ref_url(ref.url)
        if path:
            dir_path = path
        else:
            dir_path = ref.url
        entries.append(
            _DiscoveredEntry(
                ref=ref,
                dir_path=dir_path,
                relative_path=_relative_path(root, dir_path),
            )
        )
    return entries


def _matches_expression(entry: _DiscoveredEntry, expression: str | None) -> bool:
    if expression is None:
        return True

    needle = expression.strip()
    if not needle:
        return False

    info = entry.ref.info
    if info is not None:
        if info.slug == needle:
            return True
        if info.uuid.startswith(needle):
            return True
        if needle in info.identity.aliases:
            return True

    base = os.path.basename(entry.dir_path.rstrip(os.sep))
    if base.endswith(".holon"):
        base = base[: -len(".holon")]
    return base == needle


def _normalized_expression(expression: str | None) -> str | None:
    if expression is None:
        return None
    return expression.strip()


def _entry_key(entry: _DiscoveredEntry) -> str:
    if entry.ref.info is not None and entry.ref.info.uuid.strip():
        return entry.ref.info.uuid.strip()
    if entry.dir_path.strip():
        return os.path.normpath(entry.dir_path)
    return entry.ref.url.strip()


def _entry_sort_key(entry: _DiscoveredEntry) -> str:
    if entry.ref.info is not None and entry.ref.info.uuid.strip():
        return entry.ref.info.uuid.strip()
    return entry.ref.url


def _should_replace_entry(current: _DiscoveredEntry, next_entry: _DiscoveredEntry) -> bool:
    return _path_depth(next_entry.relative_path) < _path_depth(current.relative_path)


def _package_dirs_direct(root: str) -> list[str]:
    abs_root = _normalize_search_root(root)
    if not os.path.isdir(abs_root):
        return []

    dirs: list[str] = []
    for entry in os.scandir(abs_root):
        if entry.is_dir() and entry.name.endswith(".holon"):
            dirs.append(os.path.join(abs_root, entry.name))
    dirs.sort()
    return dirs


def _package_dirs_recursive(root: str) -> list[str]:
    abs_root = _normalize_search_root(root)
    if not os.path.isdir(abs_root):
        return []

    dirs: list[str] = []
    for current_root, dirnames, _filenames in os.walk(abs_root):
        if os.path.normpath(current_root) == os.path.normpath(abs_root):
            dirnames.sort()
        filtered_dirnames: list[str] = []
        for dirname in sorted(dirnames):
            full_path = os.path.join(current_root, dirname)
            if dirname.endswith(".holon"):
                dirs.append(full_path)
                continue
            if _should_skip_dir(abs_root, full_path, dirname):
                continue
            filtered_dirnames.append(dirname)
        dirnames[:] = filtered_dirnames
    dirs.sort()
    return dirs


def _should_skip_dir(root: str, path: str, name: str) -> bool:
    if os.path.normpath(path) == os.path.normpath(root):
        return False
    if name.endswith(".holon"):
        return False
    if name in _EXCLUDED_DIRS:
        return True
    return name.startswith(".")


def _resolve_discover_root(root: str | None) -> str:
    if root is None:
        cwd = os.getcwd()
        return str(Path(cwd).resolve())

    trimmed = root.strip()
    if not trimmed:
        raise ValueError("root cannot be empty")

    resolved = str(Path(trimmed).expanduser().resolve())
    if not os.path.isdir(resolved):
        raise ValueError(f'root "{trimmed}" is not a directory')
    return resolved


def _normalize_search_root(root: str) -> str:
    trimmed = root.strip()
    if not trimmed:
        return str(Path.cwd().resolve())
    return str(Path(trimmed).expanduser().resolve())


def _relative_path(root: str, path: str) -> str:
    try:
        return Path(os.path.relpath(path, root)).as_posix()
    except ValueError:
        return Path(path).as_posix()


def _path_depth(relative_path: str) -> int:
    trimmed = relative_path.strip().strip("/")
    if not trimmed or trimmed == ".":
        return 0
    return len(trimmed.split("/"))


def _bundle_holons_root() -> str:
    executable = sys.executable
    if not executable:
        return ""

    current = Path(executable).resolve().parent
    while True:
        if current.name.endswith(".app"):
            candidate = current.joinpath("Contents", "Resources", "Holons")
            if candidate.is_dir():
                return str(candidate)
        if current.parent == current:
            break
        current = current.parent
    return ""


def _oppath() -> str:
    env_root = os.environ.get("OPPATH", "").strip()
    if env_root:
        return str(Path(env_root).expanduser().resolve())

    home = Path.home()
    return str(home.joinpath(".op"))


def _opbin() -> str:
    env_root = os.environ.get("OPBIN", "").strip()
    if env_root:
        return str(Path(env_root).expanduser().resolve())
    return str(Path(_oppath()).joinpath("bin"))


def _cache_dir() -> str:
    return str(Path(_oppath()).joinpath("cache"))


def _describe_timeout_seconds(timeout: int) -> float:
    if timeout <= 0:
        return _DEFAULT_DESCRIBE_TIMEOUT_SECONDS
    return max(timeout / 1000.0, 0.001)


def _package_binary_path(package_dir: str) -> str:
    arch_root = Path(package_dir).expanduser().resolve().joinpath("bin", _package_arch_dir())
    candidates = sorted(path for path in arch_root.iterdir() if path.is_file()) if arch_root.is_dir() else []
    if not candidates:
        raise FileNotFoundError(f"no package binary for arch {_package_arch_dir()}")
    return str(candidates[0])


def _package_architectures(package_dir: Path) -> list[str]:
    bin_root = package_dir.joinpath("bin")
    if not bin_root.is_dir():
        return []
    return sorted(path.name for path in bin_root.iterdir() if path.is_dir())


def _package_arch_dir() -> str:
    system = platform.system().strip().lower() or sys.platform.lower()
    machine = platform.machine().strip().lower()
    arch_aliases = {
        "x86_64": "amd64",
        "amd64": "amd64",
        "aarch64": "arm64",
        "arm64": "arm64",
    }
    return f"{system}_{arch_aliases.get(machine, machine or 'unknown')}"


def _file_url(path: str) -> str:
    return Path(path).expanduser().resolve().as_uri()


def _path_from_ref_url(raw_url: str) -> str:
    if not raw_url:
        return ""
    if raw_url.lower().startswith("file://"):
        return _path_from_file_url(raw_url)
    return ""


def _path_from_file_url(raw_url: str) -> str:
    parsed = urlparse(raw_url.strip())
    if parsed.scheme != "file":
        raise ValueError(f'holon URL "{raw_url}" is not a local file target')

    path = unquote(parsed.path or "")
    if parsed.netloc and parsed.netloc != "localhost":
        path = f"//{parsed.netloc}{path}"
    if not path:
        raise ValueError(f'holon URL "{raw_url}" has no path')
    return str(Path(path).resolve())


def _slug_for(given_name: str, family_name: str) -> str:
    given = given_name.strip()
    family = family_name.strip().removesuffix("?")
    if not given and not family:
        return ""
    return f"{given}-{family}".strip().lower().replace(" ", "-").strip("-")


def _string_list(value: object) -> list[str]:
    if not isinstance(value, list):
        return []
    return [str(item).strip() for item in value if str(item).strip()]


def _trimmed(value: object) -> str:
    return str(value).strip() if value is not None else ""


def _apply_ref_limit(refs: list[HolonRef], limit: int) -> list[HolonRef]:
    if limit <= 0 or len(refs) <= limit:
        return refs
    return refs[:limit]


def _discover_source_with_local_op(
    scope: int,
    expression: str | None,
    root: str,
    specifiers: int,
    limit: int,
    timeout: int,
) -> DiscoverResult:
    if scope != LOCAL:
        return DiscoverResult(found=[], error=f"scope {scope} not supported")
    if specifiers != SOURCE:
        return DiscoverResult(found=[], error=f"invalid source bridge specifiers 0x{specifiers:02X}")

    op_binary = shutil.which("op")
    if not op_binary:
        return DiscoverResult(found=[], error=None)

    cmd = [op_binary, "discover", "--json"]
    try:
        completed = subprocess.run(
            cmd,
            cwd=root,
            capture_output=True,
            text=True,
            timeout=(timeout / 1000.0) if timeout > 0 else None,
            check=False,
        )
    except (OSError, subprocess.TimeoutExpired):
        return DiscoverResult(found=[], error=None)

    if completed.returncode != 0:
        return DiscoverResult(found=[], error=None)

    try:
        payload = json.loads(completed.stdout or "{}")
    except json.JSONDecodeError:
        return DiscoverResult(found=[], error=None)

    refs: list[HolonRef] = []
    for entry in payload.get("entries", []):
        if not isinstance(entry, dict):
            continue
        identity_payload = entry.get("identity", {}) if isinstance(entry.get("identity"), dict) else {}
        given_name = _trimmed(
            identity_payload.get("givenName", identity_payload.get("given_name")),
        )
        family_name = _trimmed(
            identity_payload.get("familyName", identity_payload.get("family_name")),
        )
        identity = IdentityInfo(
            given_name=given_name,
            family_name=family_name,
            motto=_trimmed(identity_payload.get("motto")),
            aliases=_string_list(identity_payload.get("aliases")),
        )
        relative_path = _trimmed(entry.get("relativePath", entry.get("relative_path")))
        url = _file_url(os.path.join(root, relative_path)) if relative_path else _file_url(root)
        refs.append(
            HolonRef(
                url=url,
                info=HolonInfo(
                    slug=_slug_for(given_name, family_name),
                    uuid=_trimmed(identity_payload.get("uuid")),
                    identity=identity,
                    lang=_trimmed(identity_payload.get("lang")),
                    runner="",
                    status=_trimmed(identity_payload.get("status")),
                    kind="",
                    transport="",
                    entrypoint="",
                    architectures=[],
                    has_dist=False,
                    has_source=True,
                ),
                error=None,
            )
        )

    if expression is not None:
        refs = [ref for ref in refs if _matches_expression(_entries_from_refs(root, [ref])[0], expression)]

    return DiscoverResult(found=_apply_ref_limit(refs, limit), error=None)
