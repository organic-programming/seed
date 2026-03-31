from __future__ import annotations

from dataclasses import dataclass, field
import json
from pathlib import Path
import platform
import sys
import textwrap

import grpc

from holons.discovery_types import DiscoverResult


@dataclass
class PackageSeed:
    slug: str
    uuid: str
    given_name: str
    family_name: str
    runner: str = "python"
    entrypoint: str = ""
    kind: str = "native"
    transport: str = "stdio"
    architectures: list[str] = field(default_factory=list)
    has_dist: bool = False
    has_source: bool = False
    aliases: list[str] = field(default_factory=list)


def sdk_dir() -> Path:
    return Path(__file__).resolve().parents[1]


def file_url(path: Path) -> str:
    return path.resolve().as_uri()


def package_arch_dir() -> str:
    system = platform.system().strip().lower() or sys.platform.lower()
    machine = platform.machine().strip().lower()
    arch_aliases = {
        "x86_64": "amd64",
        "amd64": "amd64",
        "aarch64": "arm64",
        "arm64": "arm64",
    }
    return f"{system}_{arch_aliases.get(machine, machine or 'unknown')}"


def sorted_slugs(result: DiscoverResult) -> list[str]:
    slugs = [ref.info.slug for ref in result.found if ref.info is not None]
    return sorted(slugs)


def invoke_ping(channel: grpc.Channel, message: str, timeout: float = 2.0) -> dict:
    stub = channel.unary_unary(
        "/echo.v1.Echo/Ping",
        request_serializer=lambda value: json.dumps(value).encode("utf-8"),
        response_deserializer=lambda raw: json.loads(raw.decode("utf-8")),
    )
    return stub({"message": message}, timeout=timeout)


def write_package_holon(
    directory: Path,
    seed: PackageSeed,
    *,
    with_holon_json: bool = True,
    with_binary: bool = False,
) -> None:
    directory.mkdir(parents=True, exist_ok=True)

    entrypoint = seed.entrypoint or seed.slug
    architectures = seed.architectures or ([package_arch_dir()] if with_binary else [])

    if with_binary:
        binary_path = directory / "bin" / package_arch_dir() / entrypoint
        binary_path.parent.mkdir(parents=True, exist_ok=True)
        binary_path.write_text(_echo_binary_script(), encoding="utf-8")
        binary_path.chmod(0o755)

    if with_holon_json:
        payload = {
            "schema": "holon-package/v1",
            "slug": seed.slug,
            "uuid": seed.uuid,
            "identity": {
                "given_name": seed.given_name,
                "family_name": seed.family_name,
                "aliases": list(seed.aliases),
            },
            "lang": "python",
            "runner": seed.runner,
            "status": "draft",
            "kind": seed.kind,
            "transport": seed.transport,
            "entrypoint": entrypoint,
            "architectures": architectures,
            "has_dist": seed.has_dist,
            "has_source": seed.has_source,
        }
        directory.joinpath(".holon.json").write_text(
            json.dumps(payload, indent=2) + "\n",
            encoding="utf-8",
        )


def _echo_binary_script() -> str:
    return textwrap.dedent(
        f"""\
        #!{sys.executable}
        import sys

        sys.path.insert(0, {str(sdk_dir())!r})

        from holons.echo_server import run


        def main():
            argv = sys.argv[1:]
            if not argv:
                argv = ["serve", "--listen", "stdio://"]
            run(argv)


        if __name__ == "__main__":
            main()
        """
    )
