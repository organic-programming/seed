from __future__ import annotations

import unittest
from concurrent import futures

import grpc

from support import ensure_import_paths

ensure_import_paths()

from holons import describe
from v1 import greeting_pb2, greeting_pb2_grpc

from _internal.server import GreetingService, normalize_listen_uri


class GreetingServerTest(unittest.TestCase):
    def setUp(self) -> None:
        self.server = grpc.server(futures.ThreadPoolExecutor(max_workers=2))
        greeting_pb2_grpc.add_GreetingServiceServicer_to_server(
            GreetingService(), self.server
        )
        port = self.server.add_insecure_port("127.0.0.1:0")
        self.server.start()
        self.channel = grpc.insecure_channel(f"127.0.0.1:{port}")
        grpc.channel_ready_future(self.channel).result(timeout=5)
        self.stub = greeting_pb2_grpc.GreetingServiceStub(self.channel)

    def tearDown(self) -> None:
        self.channel.close()
        self.server.stop(0).wait(timeout=5)

    def test_list_languages_returns_all_languages(self) -> None:
        response = self.stub.ListLanguages(greeting_pb2.ListLanguagesRequest(), timeout=5)
        self.assertEqual(len(response.languages), 56)

    def test_list_languages_populates_required_fields(self) -> None:
        response = self.stub.ListLanguages(greeting_pb2.ListLanguagesRequest(), timeout=5)
        for language in response.languages:
            self.assertTrue(language.code)
            self.assertTrue(language.name)
            self.assertTrue(language.native)

    def test_say_hello_uses_requested_language(self) -> None:
        response = self.stub.SayHello(
            greeting_pb2.SayHelloRequest(name="Bob", lang_code="fr"),
            timeout=5,
        )
        self.assertEqual(response.greeting, "Bonjour Bob")
        self.assertEqual(response.language, "French")
        self.assertEqual(response.lang_code, "fr")

    def test_say_hello_uses_localized_default_name(self) -> None:
        response = self.stub.SayHello(
            greeting_pb2.SayHelloRequest(lang_code="fr"),
            timeout=5,
        )
        self.assertEqual(response.greeting, "Bonjour Marie")
        self.assertEqual(response.lang_code, "fr")

    def test_say_hello_falls_back_to_english(self) -> None:
        response = self.stub.SayHello(
            greeting_pb2.SayHelloRequest(name="Bob", lang_code="xx"),
            timeout=5,
        )
        self.assertEqual(response.greeting, "Hello Bob")
        self.assertEqual(response.lang_code, "en")

    def test_normalize_listen_uri_expands_empty_tcp_host(self) -> None:
        self.assertEqual(normalize_listen_uri("tcp://:9090"), "tcp://0.0.0.0:9090")
        self.assertEqual(normalize_listen_uri("tcp://127.0.0.1:9090"), "tcp://127.0.0.1:9090")

    def test_static_describe_response_is_registered(self) -> None:
        response = describe.static_response()
        self.assertEqual(response.manifest.identity.given_name, "Gabriel")
        self.assertEqual(response.manifest.lang, "python")
        self.assertEqual(response.manifest.build.runner, "python")


if __name__ == "__main__":
    unittest.main()
