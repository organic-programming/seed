#!/usr/bin/env python3
from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path


SEED_RELEASE_RE = re.compile(r'^(seed_release:\s*)"?([^"#\s]+)"?(.*)$')


def bump_minor(version: str) -> str:
    parts = version.strip().split(".")
    if len(parts) != 3 or not all(part.isdigit() for part in parts):
        raise ValueError(f"seed_release is not a major.minor.patch version: {version}")
    major, minor, _patch = (int(part) for part in parts)
    return f"{major}.{minor + 1}.0"


def read_seed_release(path: Path) -> str:
    for line in path.read_text(encoding="utf-8").splitlines():
        match = SEED_RELEASE_RE.match(line)
        if match:
            return match.group(2)
    raise ValueError(f"{path} does not contain seed_release")


def bump_file(path: Path) -> tuple[str, str]:
    lines = path.read_text(encoding="utf-8").splitlines(keepends=True)
    current = ""
    next_version = ""
    out: list[str] = []
    replaced = False
    for line in lines:
        newline = "\n" if line.endswith("\n") else ""
        body = line[:-1] if newline else line
        match = SEED_RELEASE_RE.match(body)
        if match and not replaced:
            current = match.group(2)
            next_version = bump_minor(current)
            out.append(f'{match.group(1)}"{next_version}"{match.group(3)}{newline}')
            replaced = True
        else:
            out.append(line)
    if not replaced:
        raise ValueError(f"{path} does not contain seed_release")
    path.write_text("".join(out), encoding="utf-8")
    return current, next_version


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="command", required=True)

    next_minor = sub.add_parser("next-minor")
    next_minor.add_argument("version")

    read = sub.add_parser("read")
    read.add_argument("path", type=Path)

    bump = sub.add_parser("bump-file")
    bump.add_argument("path", type=Path)

    args = parser.parse_args(argv[1:])
    try:
        if args.command == "next-minor":
            print(bump_minor(args.version))
        elif args.command == "read":
            print(read_seed_release(args.path))
        elif args.command == "bump-file":
            current, next_version = bump_file(args.path)
            print(f"current={current}")
            print(f"next={next_version}")
        else:
            raise AssertionError(args.command)
    except ValueError as exc:
        print(str(exc), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
