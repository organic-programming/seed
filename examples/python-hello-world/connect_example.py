"""Slug-based connect example using python-holons."""

from __future__ import annotations

import os
from pathlib import Path
import sys
import tempfile
import textwrap

SDK_ROOT = Path(__file__).resolve().parents[2] / "sdk" / "python-holons"
if str(SDK_ROOT) not in sys.path:
    sys.path.insert(0, str(SDK_ROOT))

from holons import connect as connect_module


class CurrentDirGuard:
    def __init__(self, target: Path) -> None:
        self._previous = Path.cwd()
        os.chdir(target)

    def close(self) -> None:
        os.chdir(self._previous)

    def __enter__(self) -> "CurrentDirGuard":
        return self

    def __exit__(self, exc_type, exc, tb) -> None:
        self.close()


def _wrapper_script() -> str:
    return textwrap.dedent(
        f"""\
        #!/bin/sh
        export PYTHONPATH="{SDK_ROOT}"
        exec "{sys.executable}" -m holons.echo_server "$@"
        """
    )


def _write_echo_holon(root: Path) -> None:
    holon_dir = root / "holons" / "echo-server"
    binary_dir = holon_dir / ".op" / "build" / "bin"
    binary_dir.mkdir(parents=True, exist_ok=True)

    wrapper = binary_dir / "echo-wrapper"
    wrapper.write_text(_wrapper_script(), encoding="utf-8")
    wrapper.chmod(0o755)

    (holon_dir / "holon.yaml").write_text(
        textwrap.dedent(
            f"""\
            uuid: "echo-server-connect-example"
            given_name: Echo
            family_name: Server
            motto: Reply precisely.
            composer: "connect-example"
            kind: service
            build:
              runner: python
              main: holons/echo_server.py
            artifacts:
              binary: "{wrapper}"
            """
        ),
        encoding="utf-8",
    )


def main() -> int:
    with tempfile.TemporaryDirectory(prefix="python-holons-connect-") as tmp:
        root = Path(tmp)
        _write_echo_holon(root)

        with CurrentDirGuard(root):
            channel = connect_module.connect("echo-server")
            try:
                ping = channel.unary_unary(
                    "/echo.v1.Echo/Ping",
                    request_serializer=lambda value: value.encode("utf-8"),
                    response_deserializer=lambda raw: raw.decode("utf-8"),
                )
                response = ping('{"message":"hello-from-python"}', timeout=5.0)
                print(response)
            finally:
                connect_module.disconnect(channel)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
