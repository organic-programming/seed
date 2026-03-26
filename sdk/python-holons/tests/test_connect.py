from __future__ import annotations

import json
import os
from pathlib import Path
import re
import shlex
import signal
import socket
import subprocess
import sys
import time

import grpc
import pytest

from holons import connect as connect_module
from holons.connect import ConnectOptions


def _sdk_dir() -> Path:
    return Path(__file__).resolve().parents[1]


def _invoke_ping(channel: grpc.Channel, message: str, timeout: float = 2.0) -> dict:
    stub = channel.unary_unary(
        "/echo.v1.Echo/Ping",
        request_serializer=lambda value: json.dumps(value).encode("utf-8"),
        response_deserializer=lambda raw: json.loads(raw.decode("utf-8")),
    )
    return stub({"message": message}, timeout=timeout)


def _start_echo_server(*args: str) -> tuple[subprocess.Popen[str], str]:
    env = dict(os.environ)
    env["PYTHONPATH"] = str(_sdk_dir())
    proc = subprocess.Popen(
        [sys.executable, "-m", "holons.echo_server", *args],
        cwd=_sdk_dir(),
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        env=env,
    )

    assert proc.stdout is not None
    uri = proc.stdout.readline().strip()
    if uri:
        return proc, uri

    stderr = proc.stderr.read() if proc.stderr is not None else ""
    _stop_process(proc)
    if _is_bind_denied(stderr):
        pytest.skip("local bind denied in this environment")
    raise RuntimeError(f"echo_server failed to start: {stderr}")


def _stop_process(proc: subprocess.Popen[str]) -> int:
    if proc.poll() is not None:
        return int(proc.returncode)

    proc.terminate()
    try:
        proc.wait(timeout=5)
    except subprocess.TimeoutExpired:
        proc.kill()
        proc.wait(timeout=5)
    return int(proc.returncode)


def _is_bind_denied(text: str) -> bool:
    lowered = text.lower()
    return "bind" in lowered and "operation not permitted" in lowered


def _create_holon_fixture(tmp_path: Path, given_name: str, family_name: str) -> dict[str, Path | str]:
    slug = f"{given_name}-{family_name}".lower()
    holon_dir = tmp_path / "holons" / slug
    binary_dir = holon_dir / ".op" / "build" / "bin"
    binary_dir.mkdir(parents=True, exist_ok=True)

    pid_file = tmp_path / f"{slug}.pid"
    args_file = tmp_path / f"{slug}.args"
    wrapper = binary_dir / "echo-wrapper"
    wrapper.write_text(
        "\n".join(
            [
                "#!/bin/sh",
                f"printf '%s\\n' \"$$\" > {shlex.quote(str(pid_file))}",
                f": > {shlex.quote(str(args_file))}",
                f"for arg in \"$@\"; do printf '%s\\n' \"$arg\" >> {shlex.quote(str(args_file))}; done",
                f"export PYTHONPATH={shlex.quote(str(_sdk_dir()))}",
                f"exec {shlex.quote(sys.executable)} -m holons.echo_server \"$@\"",
                "",
            ]
        ),
        encoding="utf-8",
    )
    wrapper.chmod(0o755)

    (holon_dir / "holon.proto").write_text(
        "\n".join(
            [
                'syntax = "proto3";',
                "",
                "package test.v1;",
                "",
                "option (holons.v1.manifest) = {",
                "  identity: {",
                f'    uuid: "{slug}-uuid"',
                f'    given_name: "{given_name}"',
                f'    family_name: "{family_name}"',
                '    composer: "connect-test"',
                "  }",
                '  kind: "service"',
                "  build: {",
                '    runner: "python"',
                '    main: "holons/echo_server.py"',
                "  }",
                "  artifacts: {",
                '    binary: "echo-wrapper"',
                "  }",
                "};",
                "",
            ]
        ),
        encoding="utf-8",
    )

    return {
        "slug": slug,
        "pid_file": pid_file,
        "args_file": args_file,
        "binary_path": wrapper,
        "port_file": tmp_path / ".op" / "run" / f"{slug}.port",
    }


def _wait_for_pid_file(path: Path, timeout: float = 5.0) -> int:
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            raw = path.read_text(encoding="utf-8").strip()
            pid = int(raw)
            if pid > 0:
                return pid
        except (OSError, ValueError):
            pass
        time.sleep(0.025)
    raise AssertionError(f"timed out waiting for pid file {path}")


def _wait_for_args_file(path: Path, timeout: float = 5.0) -> list[str]:
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            lines = [line for line in path.read_text(encoding="utf-8").splitlines() if line]
            if lines:
                return lines
        except OSError:
            pass
        time.sleep(0.025)
    raise AssertionError(f"timed out waiting for args file {path}")


def _pid_exists(pid: int) -> bool:
    try:
        os.kill(pid, 0)
        return True
    except OSError:
        return False


def _wait_for_pid_exit(pid: int, timeout: float = 2.0) -> None:
    deadline = time.time() + timeout
    while time.time() < deadline:
        if not _pid_exists(pid):
            return
        time.sleep(0.025)
    raise AssertionError(f"process {pid} did not exit")


def _terminate_pid(pid: int) -> None:
    if not _pid_exists(pid):
        return
    os.kill(pid, signal.SIGTERM)
    try:
        _wait_for_pid_exit(pid, timeout=2.0)
        return
    except AssertionError:
        pass

    if _pid_exists(pid):
        os.kill(pid, signal.SIGKILL)
        _wait_for_pid_exit(pid, timeout=2.0)


def _reserve_loopback_port() -> int:
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.bind(("127.0.0.1", 0))
    _, port = sock.getsockname()
    sock.close()
    return int(port)


def test_connect_dials_direct_tcp_target():
    proc, uri = _start_echo_server("--listen", "tcp://127.0.0.1:0")
    try:
        channel = connect_module.connect(uri)
        try:
            out = _invoke_ping(channel, "direct-python")
            assert out["message"] == "direct-python"
            assert out["sdk"] == "python-holons"
        finally:
            connect_module.disconnect(channel)
    finally:
        _stop_process(proc)


def test_connect_starts_slug_ephemerally_and_stops_on_disconnect(tmp_path: Path, monkeypatch: pytest.MonkeyPatch):
    fixture = _create_holon_fixture(tmp_path, "Connect", "Ephemeral")
    monkeypatch.chdir(tmp_path)
    monkeypatch.setenv("OPPATH", str(tmp_path / ".op-home"))
    monkeypatch.setenv("OPBIN", str(tmp_path / ".op-bin"))

    channel = connect_module.connect(str(fixture["slug"]))
    pid = _wait_for_pid_file(Path(fixture["pid_file"]))
    args = _wait_for_args_file(Path(fixture["args_file"]))
    try:
        out = _invoke_ping(channel, "ephemeral-python")
        assert out["message"] == "ephemeral-python"
        assert args == ["serve", "--listen", "stdio://"]
    finally:
        connect_module.disconnect(channel)

    _wait_for_pid_exit(pid)
    assert not Path(fixture["port_file"]).exists()


def test_connect_writes_port_file_in_persistent_mode(tmp_path: Path, monkeypatch: pytest.MonkeyPatch):
    fixture = _create_holon_fixture(tmp_path, "Connect", "Persistent")
    monkeypatch.chdir(tmp_path)
    monkeypatch.setenv("OPPATH", str(tmp_path / ".op-home"))
    monkeypatch.setenv("OPBIN", str(tmp_path / ".op-bin"))

    channel = connect_module.connect(
        str(fixture["slug"]),
        ConnectOptions(timeout=5.0, transport="tcp", start=True),
    )
    pid = _wait_for_pid_file(Path(fixture["pid_file"]))
    try:
        out = _invoke_ping(channel, "persistent-python")
        assert out["message"] == "persistent-python"
    finally:
        connect_module.disconnect(channel)

    port_target = Path(fixture["port_file"]).read_text(encoding="utf-8").strip()
    assert re.match(r"^tcp://127\.0\.0\.1:\d+$", port_target)
    assert _pid_exists(pid)

    reused = connect_module.connect(str(fixture["slug"]))
    try:
        out = _invoke_ping(reused, "persistent-reuse-python")
        assert out["message"] == "persistent-reuse-python"
    finally:
        connect_module.disconnect(reused)
        _terminate_pid(pid)


def test_connect_writes_unix_port_file_in_persistent_mode(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
):
    fixture = _create_holon_fixture(tmp_path, "Connect", "Unix")
    monkeypatch.chdir(tmp_path)
    monkeypatch.setenv("OPPATH", str(tmp_path / ".op-home"))
    monkeypatch.setenv("OPBIN", str(tmp_path / ".op-bin"))

    channel = connect_module.connect(
        str(fixture["slug"]),
        ConnectOptions(timeout=5.0, transport="unix", start=True),
    )
    pid = _wait_for_pid_file(Path(fixture["pid_file"]))
    try:
        out = _invoke_ping(channel, "unix-python")
        assert out["message"] == "unix-python"
    finally:
        connect_module.disconnect(channel)

    port_target = Path(fixture["port_file"]).read_text(encoding="utf-8").strip()
    assert re.match(r"^unix:///tmp/holons-", port_target)
    assert _pid_exists(pid)

    reused = connect_module.connect(str(fixture["slug"]))
    try:
        out = _invoke_ping(reused, "unix-reuse-python")
        assert out["message"] == "unix-reuse-python"
    finally:
        connect_module.disconnect(reused)
        _terminate_pid(pid)


def test_connect_reuses_existing_port_file(tmp_path: Path, monkeypatch: pytest.MonkeyPatch):
    fixture = _create_holon_fixture(tmp_path, "Connect", "Reuse")
    monkeypatch.chdir(tmp_path)
    monkeypatch.setenv("OPPATH", str(tmp_path / ".op-home"))
    monkeypatch.setenv("OPBIN", str(tmp_path / ".op-bin"))

    proc = subprocess.Popen(
        [str(fixture["binary_path"]), "serve", "--listen", "tcp://127.0.0.1:0"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    try:
        assert proc.stdout is not None
        uri = proc.stdout.readline().strip()
        if not uri:
            stderr = proc.stderr.read() if proc.stderr is not None else ""
            if _is_bind_denied(stderr):
                pytest.skip("local bind denied in this environment")
            raise RuntimeError(f"wrapper server failed to start: {stderr}")

        pid = _wait_for_pid_file(Path(fixture["pid_file"]))
        Path(fixture["port_file"]).parent.mkdir(parents=True, exist_ok=True)
        Path(fixture["port_file"]).write_text(f"{uri}\n", encoding="utf-8")

        channel = connect_module.connect(str(fixture["slug"]))
        try:
            out = _invoke_ping(channel, "reuse-python")
            assert out["message"] == "reuse-python"
        finally:
            connect_module.disconnect(channel)

        assert _pid_exists(pid)
    finally:
        _stop_process(proc)


def test_connect_removes_stale_port_file_and_starts_fresh(tmp_path: Path, monkeypatch: pytest.MonkeyPatch):
    fixture = _create_holon_fixture(tmp_path, "Connect", "Stale")
    monkeypatch.chdir(tmp_path)
    monkeypatch.setenv("OPPATH", str(tmp_path / ".op-home"))
    monkeypatch.setenv("OPBIN", str(tmp_path / ".op-bin"))

    stale_port = _reserve_loopback_port()
    Path(fixture["port_file"]).parent.mkdir(parents=True, exist_ok=True)
    Path(fixture["port_file"]).write_text(f"tcp://127.0.0.1:{stale_port}\n", encoding="utf-8")

    channel = connect_module.connect(str(fixture["slug"]))
    pid = _wait_for_pid_file(Path(fixture["pid_file"]))
    try:
        out = _invoke_ping(channel, "stale-python")
        assert out["message"] == "stale-python"
        assert not Path(fixture["port_file"]).exists()
    finally:
        connect_module.disconnect(channel)

    _wait_for_pid_exit(pid)
