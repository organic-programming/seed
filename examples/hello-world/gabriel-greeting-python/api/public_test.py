from __future__ import annotations

import unittest

from support import ensure_import_paths

ensure_import_paths()

from v1 import greeting_pb2

from api import public


class GreetingPublicTest(unittest.TestCase):
    def test_list_languages_includes_english(self) -> None:
        response = public.list_languages(greeting_pb2.ListLanguagesRequest())

        self.assertTrue(response.languages)
        english = next(language for language in response.languages if language.code == "en")
        self.assertEqual(english.name, "English")
        self.assertEqual(english.native, "English")

    def test_say_hello_uses_requested_language(self) -> None:
        response = public.say_hello(
            greeting_pb2.SayHelloRequest(name="Bob", lang_code="fr")
        )

        self.assertEqual(response.greeting, "Bonjour Bob")
        self.assertEqual(response.language, "French")
        self.assertEqual(response.lang_code, "fr")

    def test_say_hello_uses_localized_default_name(self) -> None:
        response = public.say_hello(greeting_pb2.SayHelloRequest(lang_code="ja"))

        self.assertEqual(response.greeting, "こんにちは、マリアさん")
        self.assertEqual(response.language, "Japanese")
        self.assertEqual(response.lang_code, "ja")

    def test_say_hello_falls_back_to_english(self) -> None:
        response = public.say_hello(greeting_pb2.SayHelloRequest(lang_code="unknown"))

        self.assertEqual(response.greeting, "Hello Mary")
        self.assertEqual(response.language, "English")
        self.assertEqual(response.lang_code, "en")


if __name__ == "__main__":
    unittest.main()
