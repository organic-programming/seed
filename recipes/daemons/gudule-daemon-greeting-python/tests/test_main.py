from __future__ import annotations

import subprocess
import unittest
from pathlib import Path


class GreetingDaemonPythonMainTest(unittest.TestCase):
    def test_entrypoint_version_invocation_works(self) -> None:
        root = Path(__file__).resolve().parents[1]
        if not self._python_has_grpc():
            self.skipTest("python3 in this test environment does not provide grpc")
        command = (
            f"python3 {self._shell_quote(str(root / 'bin' / 'main.py'))} version"
        )

        result = subprocess.run(
            ["/bin/zsh", "-lc", command],
            capture_output=True,
            text=True,
            check=False,
            cwd=root,
        )

        self.assertEqual(result.returncode, 0, msg=result.stderr)
        self.assertIn("gudule-daemon-greeting-python v0.4.2", result.stdout)

    def _python_has_grpc(self) -> bool:
        probe = subprocess.run(
            ["/bin/zsh", "-lc", "python3 -c 'import grpc'"],
            capture_output=True,
            text=True,
            check=False,
        )
        return probe.returncode == 0

    def _shell_quote(self, value: str) -> str:
        return "'" + value.replace("'", "'\"'\"'") + "'"


if __name__ == "__main__":
    unittest.main()
