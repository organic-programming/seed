from __future__ import annotations

import os
import sys
import unittest
from concurrent import futures
from pathlib import Path
from unittest.mock import patch

import grpc

from support import ensure_import_paths

ensure_import_paths()

from holons import describe, observability
from holons.grpcclient import dial_stdio
from holons.v1 import observability_pb2, observability_pb2_grpc
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

    def test_say_hello_emits_observability_signals(self) -> None:
        self.addCleanup(observability.reset)
        observability.reset()
        with patch.dict("os.environ", {"OP_OBS": "logs,metrics"}):
            obs = observability.configure(
                observability.Config(slug="gabriel-greeting-python")
            )
            response = self.stub.SayHello(
                greeting_pb2.SayHelloRequest(name=" Bob ", lang_code="en"),
                timeout=5,
            )

        self.assertEqual(response.greeting, "Hello Bob")
        self.assertIsNotNone(obs.registry)
        snapshot = obs.registry.snapshot()
        counters = [
            counter
            for counter in snapshot["counters"]
            if counter[0] == "greeting_emitted_total"
        ]
        self.assertEqual(len(counters), 1)
        _name, _help, labels, value = counters[0]
        self.assertEqual(
            labels,
            {"lang_code": "en", "language": "English", "transport": "unknown"},
        )
        self.assertEqual(value, 1)

        self.assertIsNotNone(obs.log_ring)
        entries = [
            entry
            for entry in obs.log_ring.drain()
            if observability.body_string(entry.record) == "Greeted Bob in English (en)"
        ]
        self.assertEqual(len(entries), 1)
        record = entries[0].record
        attrs = {attr.key: attr.value for attr in record.attributes}
        fields = observability.attributes_map(record.attributes)
        self.assertEqual(
            set(fields),
            {"lang_code", "language", "name", "greeting", "transport", "duration_ns"},
        )
        self.assertEqual(fields["lang_code"], "en")
        self.assertEqual(fields["language"], "English")
        self.assertEqual(fields["name"], "Bob")
        self.assertEqual(fields["greeting"], "Hello Bob")
        self.assertEqual(fields["transport"], "unknown")
        self.assertGreaterEqual(fields["duration_ns"], 0)
        self.assertEqual(record.body.string_value, "Greeted Bob in English (en)")
        self.assertEqual(attrs["holons.slug"].string_value, "gabriel-greeting-python")
        self.assertEqual(attrs["service.name"].string_value, "gabriel-greeting-python")
        self.assertTrue(attrs["holons.session_id"].string_value)
        self.assertEqual(attrs["lang_code"].WhichOneof("value"), "string_value")
        self.assertEqual(attrs["duration_ns"].WhichOneof("value"), "int_value")
        self.assertEqual(attrs["transport"].string_value, "unknown")

    def test_say_hello_emits_stdio_transport_under_stdio_serve(self) -> None:
        root = Path(__file__).resolve().parents[1]
        env = dict(os.environ)
        env["OP_OBS"] = "logs,metrics"
        channel = dial_stdio(
            sys.executable,
            str(root / "cmd" / "main.py"),
            "serve",
            "--listen",
            "stdio://",
            env=env,
            cwd=str(root),
        )
        try:
            stub = greeting_pb2_grpc.GreetingServiceStub(channel)
            response = stub.SayHello(
                greeting_pb2.SayHelloRequest(name="Ana", lang_code="es"),
                timeout=5,
            )
            self.assertEqual(response.greeting, "Hola Ana")

            obs_stub = observability_pb2_grpc.HolonObservabilityStub(channel)
            records = list(
                obs_stub.Logs(observability_pb2.LogsRequest(follow=False), timeout=5)
            )
        finally:
            channel.close()

        matches = [
            record
            for record in records
            if observability.body_string(record) == "Greeted Ana in Spanish (es)"
        ]
        self.assertEqual(len(matches), 1)
        attrs = {attr.key: attr.value for attr in matches[0].attributes}
        self.assertEqual(attrs["transport"].string_value, "stdio")
        self.assertEqual(attrs["duration_ns"].WhichOneof("value"), "int_value")

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
