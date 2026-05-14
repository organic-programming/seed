from __future__ import annotations

import sys
from dataclasses import dataclass
from typing import TextIO

from support import ensure_import_paths

ensure_import_paths()

from holons.serve import MemberRef, parse_options

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
        options = parse_options(args[1:])
        try:
            members = parse_member_refs(args[1:])
            server_impl.listen_and_serve(
                options.listen_uri,
                reflect=options.reflect,
                members=members,
            )
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

    print(f'unknown command "{args[0]}"', file=stderr)
    print_usage(stderr)
    return 1


def parse_member_refs(args: list[str]) -> list[MemberRef]:
    members: list[MemberRef] = []
    index = 0
    while index < len(args):
        arg = args[index]
        if arg == "--member":
            index += 1
            if index >= len(args):
                raise ValueError("--member requires <slug>=<address>")
            members.append(parse_member_ref(args[index]))
        elif arg.startswith("--member="):
            members.append(parse_member_ref(arg.removeprefix("--member=")))
        index += 1
    return members


def parse_member_ref(raw: str) -> MemberRef:
    if "=" not in raw:
        raise ValueError("--member requires <slug>=<address>")
    slug, address = raw.split("=", 1)
    slug = slug.strip()
    address = address.strip()
    if not slug or not address:
        raise ValueError("--member requires non-empty slug and address")
    return MemberRef(slug=slug, address=address)


def canonical_command(raw: str) -> str:
    return raw.strip().lower().replace("-", "").replace("_", "").replace(" ", "")


def print_usage(output: TextIO) -> None:
    print("usage: observability-cascade-python-node <command> [args] [flags]", file=output)
    print("", file=output)
    print("commands:", file=output)
    print("  serve [--listen <uri>] [--member <slug>=<address>]  Start the gRPC server", file=output)
    print("  version                                             Print version and exit", file=output)
    print("  help                                                Print this help", file=output)


@dataclass
class CommandOptions:
    format: str = "text"
