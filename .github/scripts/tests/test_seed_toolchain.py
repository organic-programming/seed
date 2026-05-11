#!/usr/bin/env python3
import importlib.util
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).resolve().parents[1] / "seed_toolchain.py"
SPEC = importlib.util.spec_from_file_location("seed_toolchain", SCRIPT)
seed_toolchain = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(seed_toolchain)


class SeedToolchainTest(unittest.TestCase):
    def test_seed_release_reads_canonical_toolchain_pin(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "seed-toolchain.yaml").write_text('seed_release: "2.3.4"\n', encoding="utf-8")
            seed = seed_toolchain.seed_toolchain(root)

        self.assertEqual(seed_toolchain.seed_release(seed), "2.3.4")


if __name__ == "__main__":
    unittest.main()
