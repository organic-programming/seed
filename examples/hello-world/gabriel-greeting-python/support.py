from __future__ import annotations

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent
SDK_ROOT = ROOT.parent.parent.parent / "sdk" / "python-holons"
GEN_ROOT = ROOT / "gen" / "python" / "greeting"


def ensure_import_paths() -> None:
    for path in (ROOT, GEN_ROOT, SDK_ROOT):
        resolved = str(path)
        if resolved not in sys.path:
            sys.path.insert(0, resolved)
