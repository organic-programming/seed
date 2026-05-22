from __future__ import annotations

import os
import stat
import sys
import tempfile
import unittest
import importlib.util
from pathlib import Path


_COMPOSITE_PATH = Path(__file__).resolve().parents[1] / "holons" / "composite.py"
_SPEC = importlib.util.spec_from_file_location("holons_composite_test", _COMPOSITE_PATH)
assert _SPEC is not None and _SPEC.loader is not None
_MODULE = importlib.util.module_from_spec(_SPEC)
sys.modules[_SPEC.name] = _MODULE
_SPEC.loader.exec_module(_MODULE)
member_from_executable = _MODULE.member_from_executable


class CompositeMemberTest(unittest.TestCase):
    def test_member_resolves_embedded_binary(self):
        with tempfile.TemporaryDirectory() as tmp:
            bin_dir = Path(tmp) / "composite.holon" / "bin" / "darwin_arm64"
            member_dir = bin_dir / "holons" / "node-a"
            member_dir.mkdir(parents=True)
            self_path = bin_dir / "composite"
            self_path.write_text("self")
            (member_dir / "README.txt").write_text("not executable")
            member = member_dir / "node-a-bin"
            member.write_text("node")
            if os.name != "nt":
                member.chmod(member.stat().st_mode | stat.S_IXUSR)

            self.assertEqual(member_from_executable(self_path, "node-a"), str(member.resolve()))

    def test_member_rejects_empty_id(self):
        with self.assertRaises(ValueError):
            member_from_executable("/tmp/composite", " ")


if __name__ == "__main__":
    unittest.main()
