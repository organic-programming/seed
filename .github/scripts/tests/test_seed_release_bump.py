#!/usr/bin/env python3
from __future__ import annotations

import importlib.util
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).resolve().parents[1] / "seed_release_bump.py"
SPEC = importlib.util.spec_from_file_location("seed_release_bump", SCRIPT)
assert SPEC and SPEC.loader
seed_release_bump = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(seed_release_bump)


class SeedReleaseBumpTests(unittest.TestCase):
    def test_bump_patch(self) -> None:
        self.assertEqual(seed_release_bump.bump_patch("0.7.0"), "0.7.1")
        self.assertEqual(seed_release_bump.bump_patch("1.4.5"), "1.4.6")
        self.assertEqual(seed_release_bump.bump_patch("2.99.0"), "2.99.1")

    def test_bump_file_updates_once(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "seed-toolchain.yaml"
            path.write_text('seed_release: "0.7.0"\\nother: true\\n', encoding="utf-8")
            current, next_version = seed_release_bump.bump_file(path)

            self.assertEqual(current, "0.7.0")
            self.assertEqual(next_version, "0.7.1")
            self.assertEqual(path.read_text(encoding="utf-8"), 'seed_release: "0.7.1"\\nother: true\\n')


if __name__ == "__main__":
    unittest.main()
