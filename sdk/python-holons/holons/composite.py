"""Helpers for composite holons."""

from __future__ import annotations

import os
import stat
import sys
from pathlib import Path


def member(member_id: str) -> str:
    """Resolve a declared member's binary relative to this composite."""

    executable = os.environ.get("OP_HOLON_EXECUTABLE") or sys.executable
    return member_from_executable(executable, member_id)


def member_from_executable(executable: str | os.PathLike[str], member_id: str) -> str:
    member_id = member_id.strip()
    if not member_id:
        raise ValueError("member id is required")
    member_dir = Path(executable).resolve().parent / "holons" / member_id
    for entry in sorted(member_dir.iterdir()):
        if entry.is_file() and _is_executable(entry):
            return str(entry)
    raise FileNotFoundError(f"no executable found in {member_dir}")


def _is_executable(path: Path) -> bool:
    if os.name == "nt":
        return path.suffix.lower() == ".exe"
    return bool(path.stat().st_mode & (stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH))
