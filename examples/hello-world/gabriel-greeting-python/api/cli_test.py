from __future__ import annotations

import io
import json
import unittest

from api import cli


class GreetingCLITest(unittest.TestCase):
    def test_run_cli_version(self) -> None:
        stdout = io.StringIO()
        stderr = io.StringIO()

        code = cli.run_cli(["version"], stdout=stdout, stderr=stderr)

        self.assertEqual(code, 0)
        self.assertEqual(stdout.getvalue().strip(), cli.VERSION)
        self.assertEqual(stderr.getvalue(), "")

    def test_run_cli_help(self) -> None:
        stdout = io.StringIO()
        stderr = io.StringIO()

        code = cli.run_cli(["help"], stdout=stdout, stderr=stderr)

        self.assertEqual(code, 0)
        self.assertIn("usage: gabriel-greeting-python", stdout.getvalue())
        self.assertIn("listLanguages", stdout.getvalue())
        self.assertEqual(stderr.getvalue(), "")

    def test_run_cli_list_languages_json(self) -> None:
        stdout = io.StringIO()
        stderr = io.StringIO()

        code = cli.run_cli(["listLanguages", "--format", "json"], stdout=stdout, stderr=stderr)

        self.assertEqual(code, 0)
        payload = json.loads(stdout.getvalue())
        self.assertEqual(len(payload["languages"]), 56)
        self.assertEqual(payload["languages"][0]["code"], "en")
        self.assertEqual(payload["languages"][0]["name"], "English")
        self.assertEqual(stderr.getvalue(), "")

    def test_run_cli_say_hello_text(self) -> None:
        stdout = io.StringIO()
        stderr = io.StringIO()

        code = cli.run_cli(["sayHello", "Bob", "fr"], stdout=stdout, stderr=stderr)

        self.assertEqual(code, 0)
        self.assertEqual(stdout.getvalue().strip(), "Bonjour Bob")
        self.assertEqual(stderr.getvalue(), "")

    def test_run_cli_say_hello_defaults_to_english_json(self) -> None:
        stdout = io.StringIO()
        stderr = io.StringIO()

        code = cli.run_cli(["sayHello", "--json"], stdout=stdout, stderr=stderr)

        self.assertEqual(code, 0)
        payload = json.loads(stdout.getvalue())
        self.assertEqual(payload["greeting"], "Hello Mary")
        self.assertEqual(payload["language"], "English")
        self.assertEqual(payload["langCode"], "en")
        self.assertEqual(stderr.getvalue(), "")


if __name__ == "__main__":
    unittest.main()
