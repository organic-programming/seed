from __future__ import annotations

from support import ensure_import_paths

ensure_import_paths()

from v1 import greeting_pb2

from _internal.greetings import GREETINGS, lookup


def list_languages(request: greeting_pb2.ListLanguagesRequest) -> greeting_pb2.ListLanguagesResponse:
    del request
    response = greeting_pb2.ListLanguagesResponse()
    for greeting in GREETINGS:
        response.languages.add(
            code=greeting.lang_code,
            name=greeting.lang_english,
            native=greeting.lang_native,
        )
    return response


def say_hello(request: greeting_pb2.SayHelloRequest) -> greeting_pb2.SayHelloResponse:
    greeting = lookup(request.lang_code)
    subject = request.name.strip() or greeting.default_name
    return greeting_pb2.SayHelloResponse(
        greeting=greeting.template % subject,
        language=greeting.lang_english,
        lang_code=greeting.lang_code,
    )
