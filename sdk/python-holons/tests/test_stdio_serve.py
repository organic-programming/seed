"""Test: gRPC server over stdio (cert check 2.6).

The stdio bridge involves subprocess stdin/stdout pipes, which can conflict with
pytest's stdout capture. This test uses subprocess isolation: pytest spawns an
outer script that in turn spawns the server child, bypassing capture entirely.
"""
from __future__ import annotations

import os
import subprocess
import sys
import textwrap

import pytest


# Self-contained script that both serves and dials via stdio
_TEST_SCRIPT = textwrap.dedent("""\
    import os, sys, time
    sys.path.insert(0, os.environ["HOLONS_SDK_DIR"])

    if "--serve" in sys.argv:
        import grpc
        from concurrent import futures
        from google.protobuf import struct_pb2
        from holons import describe
        from holons.serve import run_with_options
        from holons.v1 import describe_pb2, manifest_pb2

        class EchoHandler(grpc.GenericRpcHandler):
            def service(self, handler_call_details):
                if handler_call_details.method == "/echo.v1.Echo/Ping":
                    return grpc.unary_unary_rpc_method_handler(
                        lambda req, ctx: req,
                        request_deserializer=struct_pb2.Struct.FromString,
                        response_serializer=struct_pb2.Struct.SerializeToString,
                    )
                return None

        def register(server):
            describe.use_static_response(
                describe_pb2.DescribeResponse(
                    manifest=manifest_pb2.HolonManifest(
                        identity=manifest_pb2.HolonManifest.Identity(
                            uuid="stdio-serve-0000",
                            given_name="Stdio",
                            family_name="Serve",
                        ),
                        lang="python",
                    )
                )
            )
            server.add_generic_rpc_handlers([EchoHandler()])

        run_with_options("stdio://", register, reflect=False, max_workers=2)
        sys.exit(0)

    # -- Client mode --
    import signal
    signal.alarm(20)

    from holons.grpcclient import dial_stdio
    from google.protobuf import struct_pb2

    ch = dial_stdio(
        sys.executable, __file__, "--serve",
        env=dict(os.environ),
        cwd=os.environ["HOLONS_SDK_DIR"],
    )

    time.sleep(1.0)

    req = struct_pb2.Struct()
    req.fields["message"].string_value = "stdio-roundtrip"

    stub = ch.unary_unary(
        "/echo.v1.Echo/Ping",
        request_serializer=struct_pb2.Struct.SerializeToString,
        response_deserializer=struct_pb2.Struct.FromString,
    )
    resp = stub(req, timeout=10)
    msg = resp.fields["message"].string_value
    assert msg == "stdio-roundtrip", f"expected 'stdio-roundtrip', got {msg!r}"
    print(f"OK: message={msg!r}")
    ch.close()
""")


def test_stdio_serve_roundtrip():
    """Spawn a subprocess that runs the full stdio client→server roundtrip."""
    import tempfile

    sdk_dir = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

    with tempfile.NamedTemporaryFile("w", suffix=".py", delete=False) as f:
        f.write(_TEST_SCRIPT)
        script_path = f.name

    try:
        env = dict(os.environ)
        env["HOLONS_SDK_DIR"] = sdk_dir

        result = subprocess.run(
            [sys.executable, script_path],
            cwd=sdk_dir,
            env=env,
            capture_output=True,
            text=True,
            timeout=25,
        )

        assert result.returncode == 0, (
            f"stdio roundtrip failed (exit {result.returncode}):\n"
            f"stdout: {result.stdout}\n"
            f"stderr: {result.stderr}"
        )
        assert "OK:" in result.stdout, f"expected OK in output, got: {result.stdout}"
    finally:
        os.unlink(script_path)
