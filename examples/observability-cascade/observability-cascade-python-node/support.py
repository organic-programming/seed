from __future__ import annotations

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent
GEN_ROOT = ROOT / "gen" / "python"


def find_repo_root(start: Path) -> Path:
    for candidate in [start, *start.parents]:
        if (candidate / "sdk" / "python-holons").is_dir():
            return candidate
    raise RuntimeError("could not locate repository root")


SDK_ROOT = find_repo_root(ROOT) / "sdk" / "python-holons"


def ensure_import_paths() -> None:
    for path in (ROOT, GEN_ROOT, SDK_ROOT):
        resolved = str(path)
        if resolved not in sys.path:
            sys.path.insert(0, resolved)
