from __future__ import annotations

import sys
from dataclasses import dataclass
from typing import TextIO

from support import ensure_import_paths

ensure_import_paths()

from holons import composite, observability
from holons.serve import parse_child_flags, parse_options

from _internal import server as server_impl

VERSION = "observability-cascade-python-node 0.1.0"


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
        children, remaining = parse_child_flags(args[1:])
        options = parse_options(remaining)
        transport = parse_transport(remaining)
        observability.from_env(observability.Config(slug="observability-cascade-python-node"))
        downstream = None
        try:
            if children:
                first = children[0]
                downstream = composite.SpawnMember(
                    composite.SpawnOptions(
                        slug=first.slug,
                        binary_path=first.binary,
                        transport=transport,
                        downstream_chain=tuple(
                            composite.ChildSpec(child.slug, child.binary)
                            for child in children[1:]
                        ),
                    )
                )
            server_impl.listen_and_serve(
                options.listen_uri,
                reflect=options.reflect,
                downstream_conn=downstream.conn if downstream is not None else None,
            )
        except Exception as exc:
            if downstream is not None:
                downstream.stop()
            print(f"serve: {exc}", file=stderr)
            return 1
        finally:
            if downstream is not None:
                downstream.stop()
        return 0
    if command == "version":
        print(VERSION, file=stdout)
        return 0
    if command == "help":
        print_usage(stdout)
        return 0

    print(f'unknown command "{args[0]}"', file=stderr)
    print_usage(stderr)
    return 1


def parse_transport(args: list[str]) -> str:
    for index, arg in enumerate(args):
        if arg == "--transport" and index + 1 < len(args):
            return args[index + 1]
        if arg.startswith("--transport="):
            return arg.removeprefix("--transport=")
    return "stdio"


def canonical_command(raw: str) -> str:
    return raw.strip().lower().replace("-", "").replace("_", "").replace(" ", "")


def print_usage(output: TextIO) -> None:
    print("usage: observability-cascade-python-node <command> [args] [flags]", file=output)
    print("", file=output)
    print("commands:", file=output)
    print("  serve [--listen <uri>] [--transport <name>] [--child <slug>=<binary>]  Start the gRPC server", file=output)
    print("  version                                             Print version and exit", file=output)
    print("  help                                                Print this help", file=output)


@dataclass
class CommandOptions:
    format: str = "text"
