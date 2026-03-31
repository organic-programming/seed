from __future__ import annotations

"""Resolve holons to ready-to-use gRPC channels."""

from dataclasses import dataclass
import os
from pathlib import Path
import platform
import sys
from urllib.parse import unquote, urlparse

import grpc

from . import grpcclient
from .discover import resolve as resolve_ref
from .discovery_types import ConnectResult, HolonInfo, HolonRef, LOCAL

_DEFAULT_CONNECT_TIMEOUT_SECONDS = 5.0


@dataclass(frozen=True)
class _LaunchTarget:
    command: tuple[str, ...]
    cwd: str | None = None


def connect(
    scope: int,
    expression: str,
    root: str | None,
    specifiers: int,
    timeout: int,
) -> ConnectResult:
    if scope != LOCAL:
        return ConnectResult(channel=None, uid="", origin=None, error=f"scope {scope} not supported")

    target = expression.strip()
    if not target:
        return ConnectResult(channel=None, uid="", origin=None, error="expression is required")

    resolved = resolve_ref(scope, target, root, specifiers, timeout)
    if resolved.error is not None:
        return ConnectResult(channel=None, uid="", origin=resolved.ref, error=resolved.error)
    if resolved.ref is None:
        return ConnectResult(channel=None, uid="", origin=None, error=f'holon "{target}" not found')

    ref = resolved.ref
    if ref.error is not None:
        return ConnectResult(channel=None, uid="", origin=ref, error=ref.error)

    try:
        return _connect_resolved(ref, timeout)
    except Exception as exc:
        return ConnectResult(channel=None, uid="", origin=ref, error=str(exc) or "target unreachable")


def disconnect(result: ConnectResult) -> None:
    if result.channel is None:
        return
    try:
        result.channel.close()
    except Exception:
        return


def _connect_resolved(ref: HolonRef, timeout: int) -> ConnectResult:
    scheme = _url_scheme(ref.url)
    if scheme in {"tcp", "unix", "ws", "wss"}:
        channel = _dial_ready_uri(ref.url, timeout)
        return ConnectResult(channel=channel, uid="", origin=ref, error=None)

    if scheme != "file":
        raise ValueError(f"unsupported target URL {ref.url!r}")

    target = _launch_target_from_ref(ref)
    channel = grpcclient.dial_stdio(target.command[0], *target.command[1:], cwd=target.cwd)
    _wait_ready(channel, timeout)
    return ConnectResult(channel=channel, uid="", origin=ref, error=None)


def _dial_ready_uri(uri: str, timeout: int) -> grpc.Channel:
    channel = grpcclient.dial_uri(uri)
    try:
        _wait_ready(channel, timeout)
        return channel
    except Exception:
        channel.close()
        raise


def _wait_ready(channel: grpc.Channel, timeout: int) -> None:
    future = grpc.channel_ready_future(channel)
    seconds = None if timeout <= 0 else max(timeout / 1000.0, 0.001)
    future.result(timeout=seconds)


def _launch_target_from_ref(ref: HolonRef) -> _LaunchTarget:
    path = _path_from_file_url(ref.url)
    info = ref.info
    if info is None:
        raise ValueError("holon metadata unavailable")

    if os.path.isfile(path):
        return _LaunchTarget(command=(path,), cwd=str(Path(path).resolve().parent))

    if not os.path.isdir(path):
        raise ValueError(f'target path "{path}" is not launchable')

    if path.endswith(".holon"):
        target = _package_launch_target(path, info)
        if target is not None:
            return target

    target = _source_launch_target(path, info)
    if target is not None:
        return target

    raise ValueError("target unreachable")


def _package_launch_target(package_dir: str, info: HolonInfo) -> _LaunchTarget | None:
    entrypoint = (info.entrypoint or info.slug).strip()
    if not entrypoint:
        return None

    binary_path = Path(package_dir).joinpath("bin", _package_arch_dir(), Path(entrypoint).name)
    if binary_path.is_file():
        return _LaunchTarget(command=(str(binary_path),), cwd=package_dir)

    dist_entry = Path(package_dir).joinpath("dist", Path(entrypoint))
    if dist_entry.is_file():
        return _launch_target_for_runner(info.runner, str(dist_entry), package_dir)

    git_root = Path(package_dir).joinpath("git")
    if git_root.is_dir():
        return _source_launch_target(str(git_root), info)

    return None


def _source_launch_target(source_dir: str, info: HolonInfo) -> _LaunchTarget | None:
    entrypoint = (info.entrypoint or info.slug).strip()
    if entrypoint:
        absolute_entry = Path(entrypoint)
        if absolute_entry.is_absolute() and absolute_entry.is_file():
            return _LaunchTarget(command=(str(absolute_entry),), cwd=source_dir)

        source_package_binary = (
            Path(source_dir)
            .joinpath(".op", "build", f"{info.slug}.holon", "bin", _package_arch_dir(), Path(entrypoint).name)
        )
        if source_package_binary.is_file():
            return _LaunchTarget(command=(str(source_package_binary),), cwd=source_dir)

        source_binary = Path(source_dir).joinpath(".op", "build", "bin", Path(entrypoint).name)
        if source_binary.is_file():
            return _LaunchTarget(command=(str(source_binary),), cwd=source_dir)

        direct_entry = Path(source_dir).joinpath(entrypoint)
        if direct_entry.is_file():
            target = _launch_target_for_runner(info.runner, str(direct_entry), source_dir)
            if target is not None:
                return target

    return None


def _launch_target_for_runner(runner: str, entrypoint: str, cwd: str) -> _LaunchTarget | None:
    runner_name = runner.strip().lower()
    if not runner_name or not entrypoint:
        return None

    if runner_name in {"go", "go-module"}:
        return _LaunchTarget(command=("go", "run", entrypoint), cwd=cwd)
    if runner_name == "python":
        return _LaunchTarget(command=(sys.executable, entrypoint), cwd=cwd)
    if runner_name in {"node", "typescript", "npm"}:
        return _LaunchTarget(command=("node", entrypoint), cwd=cwd)
    if runner_name == "ruby":
        return _LaunchTarget(command=("ruby", entrypoint), cwd=cwd)
    if runner_name == "dart":
        return _LaunchTarget(command=("dart", "run", entrypoint), cwd=cwd)
    return None


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


def _url_scheme(raw_url: str) -> str:
    parsed = urlparse(raw_url.strip())
    return parsed.scheme.lower()


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
