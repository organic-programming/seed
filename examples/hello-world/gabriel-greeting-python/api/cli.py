from __future__ import annotations

import sys
from dataclasses import dataclass
from typing import TextIO

from google.protobuf.json_format import MessageToJson

from support import ensure_import_paths

ensure_import_paths()

from holons.serve import parse_flags
from v1 import greeting_pb2

from _internal import server as server_impl
from api import public

VERSION = "gabriel-greeting-python {{ .Version }}"


def main(argv: list[str] | None = None) -> int:
    args = list(sys.argv[1:] if argv is None else argv)
    return run_cli(args)


def run_cli(
    args: list[str],
    stdout: TextIO | None = None,
    stderr: TextIO | None = None,
) -> int:
    stdout = sys.stdout if stdout is None else stdout
    stderr = sys.stderr if stderr is None else stderr

    if not args:
        print_usage(stderr)
        return 1

    command = canonical_command(args[0])
    if command == "serve":
        listen_uri = parse_flags(args[1:])
        try:
            server_impl.listen_and_serve(listen_uri)
        except Exception as exc:
            print(f"serve: {exc}", file=stderr)
            return 1
        return 0
    if command == "version":
        print(VERSION, file=stdout)
        return 0
    if command == "help":
        print_usage(stdout)
        return 0
    if command == "listlanguages":
        return run_list_languages(args[1:], stdout, stderr)
    if command == "sayhello":
        return run_say_hello(args[1:], stdout, stderr)

    print(f'unknown command "{args[0]}"', file=stderr)
    print_usage(stderr)
    return 1


def run_list_languages(args: list[str], stdout: TextIO, stderr: TextIO) -> int:
    try:
        options, positional = parse_command_options(args)
    except ValueError as exc:
        print(f"listLanguages: {exc}", file=stderr)
        return 1

    if positional:
        print("listLanguages: accepts no positional arguments", file=stderr)
        return 1

    response = public.list_languages(greeting_pb2.ListLanguagesRequest())
    try:
        write_response(stdout, response, options.format)
    except ValueError as exc:
        print(f"listLanguages: {exc}", file=stderr)
        return 1
    return 0


def run_say_hello(args: list[str], stdout: TextIO, stderr: TextIO) -> int:
    try:
        options, positional = parse_command_options(args)
    except ValueError as exc:
        print(f"sayHello: {exc}", file=stderr)
        return 1

    if len(positional) > 2:
        print("sayHello: accepts at most <name> [lang_code]", file=stderr)
        return 1

    request = greeting_pb2.SayHelloRequest(lang_code="en")
    if positional:
        request.name = positional[0]
    if len(positional) >= 2:
        if options.lang:
            print(
                "sayHello: use either a positional lang_code or --lang, not both",
                file=stderr,
            )
            return 1
        request.lang_code = positional[1]
    if options.lang:
        request.lang_code = options.lang

    response = public.say_hello(request)
    try:
        write_response(stdout, response, options.format)
    except ValueError as exc:
        print(f"sayHello: {exc}", file=stderr)
        return 1
    return 0


def parse_command_options(args: list[str]) -> tuple["CommandOptions", list[str]]:
    options = CommandOptions()
    positional: list[str] = []
    index = 0

    while index < len(args):
        arg = args[index]
        if arg == "--json":
            options.format = "json"
        elif arg == "--format":
            index += 1
            if index >= len(args):
                raise ValueError("--format requires a value")
            options.format = parse_output_format(args[index])
        elif arg.startswith("--format="):
            options.format = parse_output_format(arg.split("=", 1)[1])
        elif arg == "--lang":
            index += 1
            if index >= len(args):
                raise ValueError("--lang requires a value")
            options.lang = args[index].strip()
        elif arg.startswith("--lang="):
            options.lang = arg.split("=", 1)[1].strip()
        elif arg.startswith("--"):
            raise ValueError(f'unknown flag "{arg}"')
        else:
            positional.append(arg)
        index += 1

    return options, positional


def parse_output_format(raw: str) -> str:
    normalized = raw.strip().lower()
    if normalized in ("", "text", "txt"):
        return "text"
    if normalized == "json":
        return "json"
    raise ValueError(f'unsupported format "{raw}"')


def write_response(stdout: TextIO, message: object, output_format: str) -> None:
    if output_format == "json":
        stdout.write(MessageToJson(message, indent=2))
        stdout.write("\n")
        return
    if output_format == "text":
        write_text(stdout, message)
        return
    raise ValueError(f'unsupported format "{output_format}"')


def write_text(stdout: TextIO, message: object) -> None:
    if isinstance(message, greeting_pb2.SayHelloResponse):
        stdout.write(f"{message.greeting}\n")
        return
    if isinstance(message, greeting_pb2.ListLanguagesResponse):
        for language in message.languages:
            stdout.write(f"{language.code}\t{language.name}\t{language.native}\n")
        return
    raise ValueError(f"unsupported text output for {type(message)!r}")


def canonical_command(raw: str) -> str:
    return raw.strip().lower().replace("-", "").replace("_", "").replace(" ", "")


def print_usage(output: TextIO) -> None:
    print("usage: gabriel-greeting-python <command> [args] [flags]", file=output)
    print("", file=output)
    print("commands:", file=output)
    print("  serve [--listen <uri>]                    Start the gRPC server", file=output)
    print("  version                                  Print version and exit", file=output)
    print("  help                                     Print usage", file=output)
    print("  listLanguages [--format text|json]       List supported languages", file=output)
    print(
        "  sayHello [name] [lang_code] [--format text|json] [--lang <code>]",
        file=output,
    )
    print("", file=output)
    print("examples:", file=output)
    print("  gabriel-greeting-python serve --listen stdio", file=output)
    print("  gabriel-greeting-python listLanguages --format json", file=output)
    print("  gabriel-greeting-python sayHello Alice fr", file=output)
    print(
        "  gabriel-greeting-python sayHello Alice --lang fr --format json",
        file=output,
    )


@dataclass
class CommandOptions:
    format: str = "text"
    lang: str = ""
