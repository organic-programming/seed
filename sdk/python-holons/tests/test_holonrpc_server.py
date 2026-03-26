"""Tests for Holon-RPC server behavior in python-holons."""

from __future__ import annotations

import asyncio
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
import textwrap

import pytest
import websockets

from holons.holonrpc import HolonRPCServer


def _resolve_node_binary() -> str:
    found = shutil.which("node")
    if not found:
        raise RuntimeError("node binary not found")
    return found


def test_holonrpc_server_jsonrpc_roundtrip():
    async def _run() -> None:
        server = HolonRPCServer("ws://127.0.0.1:0/rpc")
        server.register("echo.v1.Echo/Ping", lambda params: params)
        try:
            url = await server.start()
        except (PermissionError, OSError) as exc:
            if "operation not permitted" in str(exc).lower():
                pytest.skip(f"local bind denied in this environment: {exc}")
            raise

        try:
            async with websockets.connect(url, subprotocols=["holon-rpc"]) as ws:
                await ws.send(
                    json.dumps(
                        {
                            "jsonrpc": "2.0",
                            "id": "c1",
                            "method": "echo.v1.Echo/Ping",
                            "params": {"message": "hello"},
                        }
                    )
                )
                raw = await ws.recv()
                payload = json.loads(raw)
                assert payload["jsonrpc"] == "2.0"
                assert payload["id"] == "c1"
                assert payload["result"]["message"] == "hello"
        finally:
            await server.close()

    asyncio.run(_run())


def test_holonrpc_server_js_web_bidirectional_roundtrip():
    async def _run() -> None:
        server = HolonRPCServer("ws://127.0.0.1:0/rpc")
        server.register("echo.v1.Echo/Ping", lambda params: params)
        try:
            url = await server.start()
        except (PermissionError, OSError) as exc:
            if "operation not permitted" in str(exc).lower():
                pytest.skip(f"local bind denied in this environment: {exc}")
            raise

        sdk_dir = Path(__file__).resolve().parents[2]
        node_bin = _resolve_node_binary()

        script = textwrap.dedent(
            """
            import { HolonClient } from "./js-web-holons/src/index.mjs";
            import WebSocket from "./js-web-holons/node_modules/ws/index.js";

            const url = process.argv[2];
            const client = new HolonClient(url, {
              WebSocket,
              reconnect: false,
              heartbeat: false,
            });

            client.register("client.v1.Client/Hello", (payload) => ({
              message: `hello ${payload.name}`,
            }));

            await client.connect();
            const out = await client.invoke("echo.v1.Echo/Ping", { message: "from-js-web" });
            process.stdout.write(JSON.stringify({ ready: true, ping: out.message }) + "\\n");

            process.stdin.resume();
            process.stdin.once("data", () => {
              try {
                client.close();
              } catch {
                // no-op
              }
              process.exit(0);
            });
            """
        ).strip()

        with tempfile.NamedTemporaryFile("w", suffix=".mjs", dir=sdk_dir, delete=False) as f:
            f.write(script)
            script_path = Path(f.name)

        proc = subprocess.Popen(
            [node_bin, str(script_path), url],
            cwd=sdk_dir,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )

        try:
            assert proc.stdout is not None
            ready_line = (await asyncio.to_thread(proc.stdout.readline)).strip()
            if not ready_line:
                stderr = proc.stderr.read() if proc.stderr is not None else ""
                raise RuntimeError(f"js-web-holons client did not become ready: {stderr}")
            ready = json.loads(ready_line)
            assert ready.get("ready") is True
            assert ready.get("ping") == "from-js-web"

            client_id = await server.wait_for_client(timeout=5.0)
            out = await server.invoke(
                client_id,
                "client.v1.Client/Hello",
                {"name": "browser"},
                timeout=5.0,
            )
            assert out["message"] == "hello browser"
        finally:
            if proc.stdin is not None and proc.poll() is None:
                try:
                    await asyncio.to_thread(proc.stdin.write, "\n")
                    await asyncio.to_thread(proc.stdin.flush)
                except Exception:
                    pass
            try:
                await asyncio.to_thread(proc.wait, 5)
            except subprocess.TimeoutExpired:
                proc.kill()
                await asyncio.to_thread(proc.wait, 5)

            script_path.unlink(missing_ok=True)
            await server.close()

    asyncio.run(_run())
