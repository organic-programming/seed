from __future__ import annotations

import base64
import json
import os
from concurrent import futures
from pathlib import Path
import subprocess
import sys
import tempfile
import textwrap

import grpc
import pytest

from holons import describe
from holons.describe import build_response, describe_pb2, describe_pb2_grpc
from holons.grpcclient import dial_uri
from holons.v1 import manifest_pb2

_SDK_DIR = Path(__file__).resolve().parents[1]

_ECHO_PROTO = """\
syntax = "proto3";
package echo.v1;

// Echo echoes request payloads for documentation tests.
service Echo {
  // Ping echoes the inbound message.
  // @example {"message":"hello","sdk":"go-holons"}
  rpc Ping(PingRequest) returns (PingResponse);
}

message PingRequest {
  // Message to echo back.
  // @required
  // @example "hello"
  string message = 1;

  // SDK marker included in the response.
  // @example "go-holons"
  string sdk = 2;
}

message PingResponse {
  // Echoed message.
  string message = 1;

  // SDK marker from the server.
  string sdk = 2;
}
"""

_INVALID_ECHO_PROTO = """\
syntax = "proto3";
package echo.v1;

import public "echo/v1/echo.proto";
"""

_HOLON_PROTO = """\
syntax = "proto3";

package holons.test.v1;

option (holons.v1.manifest) = {
  identity: {
    uuid: "echo-server-0000"
    given_name: "Echo"
    family_name: "Server"
    motto: "Reply precisely."
    composer: "describe-test"
    status: "draft"
    born: "2026-03-17"
  }
  lang: "python"
};
"""

_HELPER_SCRIPT = textwrap.dedent(
    """\
    import json
    import os
    import sys

    import grpc

    sys.path.insert(0, os.environ["HOLONS_SDK_DIR"])

    from holons import describe
    from holons.serve import run_with_options
    from holons.v1 import describe_pb2, manifest_pb2


    def _static_response():
        services = []
        if os.environ.get("HOLONS_WITH_ECHO") == "1":
            services.append(
                describe_pb2.ServiceDoc(
                    name="echo.v1.Echo",
                    description="Echo echoes request payloads for documentation tests.",
                    methods=[
                        describe_pb2.MethodDoc(
                            name="Ping",
                            description="Ping echoes the inbound message.",
                            input_type="echo.v1.PingRequest",
                            output_type="echo.v1.PingResponse",
                            example_input='{\"message\":\"hello\",\"sdk\":\"go-holons\"}',
                            input_fields=[
                                describe_pb2.FieldDoc(
                                    name="message",
                                    type="string",
                                    number=1,
                                    description="Message to echo back.",
                                    label=describe_pb2.FIELD_LABEL_OPTIONAL,
                                    required=True,
                                    example='\"hello\"',
                                ),
                                describe_pb2.FieldDoc(
                                    name="sdk",
                                    type="string",
                                    number=2,
                                    description="SDK marker included in the response.",
                                    label=describe_pb2.FIELD_LABEL_OPTIONAL,
                                    example='\"go-holons\"',
                                ),
                            ],
                            output_fields=[
                                describe_pb2.FieldDoc(
                                    name="message",
                                    type="string",
                                    number=1,
                                    description="Echoed message.",
                                    label=describe_pb2.FIELD_LABEL_OPTIONAL,
                                ),
                                describe_pb2.FieldDoc(
                                    name="sdk",
                                    type="string",
                                    number=2,
                                    description="SDK marker from the server.",
                                    label=describe_pb2.FIELD_LABEL_OPTIONAL,
                                ),
                            ],
                        )
                    ],
                )
            )
        return describe_pb2.DescribeResponse(
            manifest=manifest_pb2.HolonManifest(
                identity=manifest_pb2.HolonManifest.Identity(
                    uuid="echo-server-0000",
                    given_name="Echo",
                    family_name="Server",
                    motto="Reply precisely.",
                    composer="describe-test",
                    status="draft",
                    born="2026-03-17",
                ),
                lang="python",
            ),
            services=services,
        )


    def _decode_request(raw):
        if not raw:
            return {}
        payload = json.loads(raw.decode("utf-8"))
        return payload if isinstance(payload, dict) else {}


    def _encode_response(payload):
        return json.dumps(payload, separators=(",", ":")).encode("utf-8")


    class EchoHandler(grpc.GenericRpcHandler):
        def service(self, handler_call_details):
            if handler_call_details.method != "/echo.v1.Echo/Ping":
                return None
            return grpc.unary_unary_rpc_method_handler(
                self._ping,
                request_deserializer=_decode_request,
                response_serializer=_encode_response,
            )

        def _ping(self, request, _context):
            return {
                "message": str(request.get("message", "")),
                "sdk": "python-holons",
            }


    def register(server):
        if os.environ.get("HOLONS_REGISTER_DESCRIBE") == "1":
            describe.use_static_response(_static_response())
        if os.environ.get("HOLONS_WITH_ECHO") == "1":
            server.add_generic_rpc_handlers([EchoHandler()])


    run_with_options(
        "tcp://127.0.0.1:0",
        register,
        reflect=False,
        on_listen=lambda uri: print(uri, flush=True),
    )
    """
)


@pytest.fixture(autouse=True)
def _reset_static_describe_response():
    describe.use_static_response(None)
    yield
    describe.use_static_response(None)


def _write_echo_holon(
    root: Path,
    *,
    include_proto: bool = True,
    proto_text: str = _ECHO_PROTO,
) -> None:
    (root / "holon.proto").write_text(_HOLON_PROTO, encoding="utf-8")
    if not include_proto:
        return
    proto_path = root / "protos" / "echo" / "v1"
    proto_path.mkdir(parents=True, exist_ok=True)
    (proto_path / "echo.proto").write_text(proto_text, encoding="utf-8")


def _find_field(fields, name: str):
    for field in fields:
        if field.name == name:
            return field
    raise AssertionError(f"field {name!r} not found")


def _is_bind_denied(stderr: str) -> bool:
    text = stderr.lower()
    return "bind" in text and "operation not permitted" in text


def _start_describe_helper(
    workdir: Path,
    *,
    include_proto: bool,
    with_echo: bool,
    proto_text: str = _ECHO_PROTO,
    register_describe: bool = True,
    expect_startup_failure: bool = False,
) -> tuple[subprocess.Popen[str], str, Path]:
    _write_echo_holon(workdir, include_proto=include_proto, proto_text=proto_text)

    with tempfile.NamedTemporaryFile("w", suffix=".py", delete=False) as handle:
        handle.write(_HELPER_SCRIPT)
        script_path = Path(handle.name)

    env = dict(os.environ)
    env["HOLONS_SDK_DIR"] = str(_SDK_DIR)
    env["HOLONS_WITH_ECHO"] = "1" if with_echo else "0"
    env["HOLONS_REGISTER_DESCRIBE"] = "1" if register_describe else "0"
    env["PYTHONPATH"] = str(_SDK_DIR) + os.pathsep + env.get("PYTHONPATH", "")

    proc = subprocess.Popen(
        [sys.executable, str(script_path)],
        cwd=workdir,
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )

    assert proc.stdout is not None
    uri = proc.stdout.readline().strip()
    if uri:
        return proc, uri, script_path

    if expect_startup_failure:
        return proc, "", script_path

    stderr = ""
    if proc.stderr is not None:
        stderr = proc.stderr.read()
    _stop_process(proc)
    script_path.unlink(missing_ok=True)

    if _is_bind_denied(stderr):
        pytest.skip("local bind denied in this environment")
    raise RuntimeError(f"describe helper failed to start: {stderr}")


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


def test_build_response_from_echo_proto(tmp_path: Path):
    _write_echo_holon(tmp_path, include_proto=True)

    response = build_response(tmp_path / "protos")

    assert response.manifest.identity.given_name == "Echo"
    assert response.manifest.identity.family_name == "Server"
    assert response.manifest.identity.motto == "Reply precisely."
    assert response.manifest.lang == "python"
    assert len(response.services) == 1

    service = response.services[0]
    assert service.name == "echo.v1.Echo"
    assert service.description == "Echo echoes request payloads for documentation tests."
    assert len(service.methods) == 1

    method = service.methods[0]
    assert method.name == "Ping"
    assert method.description == "Ping echoes the inbound message."
    assert method.input_type == "echo.v1.PingRequest"
    assert method.output_type == "echo.v1.PingResponse"
    assert method.example_input == '{"message":"hello","sdk":"go-holons"}'

    message_field = _find_field(method.input_fields, "message")
    assert message_field.type == "string"
    assert message_field.number == 1
    assert message_field.description == "Message to echo back."
    assert message_field.label == describe_pb2.FIELD_LABEL_OPTIONAL
    assert message_field.required is True
    assert message_field.example == '"hello"'


def _sample_static_response(with_echo: bool = True) -> describe_pb2.DescribeResponse:
    services = []
    if with_echo:
        services.append(
            describe_pb2.ServiceDoc(
                name="echo.v1.Echo",
                description="Echo echoes request payloads for documentation tests.",
                methods=[
                    describe_pb2.MethodDoc(
                        name="Ping",
                        description="Ping echoes the inbound message.",
                        input_type="echo.v1.PingRequest",
                        output_type="echo.v1.PingResponse",
                    )
                ],
            )
        )

    return describe_pb2.DescribeResponse(
        manifest=manifest_pb2.HolonManifest(
            identity=manifest_pb2.HolonManifest.Identity(
                uuid="echo-server-0000",
                given_name="Echo",
                family_name="Server",
                motto="Reply precisely.",
            ),
            lang="python",
        ),
        services=services,
    )


def test_register_requires_static_describe_response():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=1))
    with pytest.raises(describe.IncodeDescriptionError, match=describe.NO_INCODE_DESCRIPTION_MESSAGE):
        describe.register(server)


def test_register_serves_registered_static_describe_response():
    describe.use_static_response(_sample_static_response())
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=1))
    port = server.add_insecure_port("127.0.0.1:0")
    describe.register(server)
    server.start()
    try:
        channel = dial_uri(f"tcp://127.0.0.1:{port}")
        try:
            stub = describe_pb2_grpc.HolonMetaStub(channel)
            response = stub.Describe(describe_pb2.DescribeRequest(), timeout=5)
        finally:
            channel.close()
    finally:
        server.stop(None)

    assert response.manifest.identity.given_name == "Echo"
    assert response.manifest.identity.family_name == "Server"
    assert [service.name for service in response.services] == ["echo.v1.Echo"]


def test_serve_uses_registered_static_describe_response(tmp_path: Path):
    proc, uri, script_path = _start_describe_helper(
        tmp_path,
        include_proto=True,
        with_echo=True,
    )
    try:
        channel = dial_uri(uri)
        try:
            stub = describe_pb2_grpc.HolonMetaStub(channel)
            response = stub.Describe(describe_pb2.DescribeRequest(), timeout=5)
        finally:
            channel.close()
    finally:
        rc = _stop_process(proc)
        script_path.unlink(missing_ok=True)
        assert rc == 0

    assert response.manifest.identity.given_name == "Echo"
    assert response.manifest.identity.family_name == "Server"
    assert response.manifest.identity.motto == "Reply precisely."
    assert [service.name for service in response.services] == ["echo.v1.Echo"]
    assert response.services[0].methods[0].name == "Ping"


def test_serve_uses_registered_static_describe_without_local_protos(tmp_path: Path):
    proc, uri, script_path = _start_describe_helper(
        tmp_path,
        include_proto=False,
        with_echo=False,
    )
    try:
        channel = dial_uri(uri)
        try:
            stub = describe_pb2_grpc.HolonMetaStub(channel)
            response = stub.Describe(describe_pb2.DescribeRequest(), timeout=5)
        finally:
            channel.close()
    finally:
        rc = _stop_process(proc)
        script_path.unlink(missing_ok=True)
        assert rc == 0

    assert response.manifest.identity.given_name == "Echo"
    assert response.manifest.identity.family_name == "Server"
    assert response.manifest.identity.motto == "Reply precisely."
    assert list(response.services) == []


def test_built_server_without_proto_files_serves_static_describe_response(tmp_path: Path):
    generated = tmp_path / "gen"
    generated.mkdir(parents=True)
    response = _sample_static_response(with_echo=False)
    payload = base64.b64encode(response.SerializeToString()).decode("ascii")

    (generated / "describe_generated.py").write_text(
        "\n".join(
            [
                "from __future__ import annotations",
                "",
                "import base64",
                "",
                "from holons.v1 import describe_pb2",
                "",
                "",
                "def static_describe_response() -> describe_pb2.DescribeResponse:",
                "    response = describe_pb2.DescribeResponse()",
                f"    response.ParseFromString(base64.b64decode({payload!r}))",
                "    return response",
                "",
                "",
                "def StaticDescribeResponse() -> describe_pb2.DescribeResponse:",
                "    return static_describe_response()",
                "",
            ]
        ),
        encoding="utf-8",
    )

    built_server = tmp_path / "built_server.py"
    built_server.write_text(
        textwrap.dedent(
            """\
            from __future__ import annotations

            import os
            import sys
            from pathlib import Path

            ROOT = Path(__file__).resolve().parent
            sys.path.insert(0, os.environ["HOLONS_SDK_DIR"])
            sys.path.insert(0, str(ROOT))

            from holons import describe
            from holons.serve import run_with_options
            from gen import describe_generated


            def register(server):
                del server
                describe.use_static_response(describe_generated.static_describe_response())


            run_with_options(
                "tcp://127.0.0.1:0",
                register,
                reflect=False,
                on_listen=lambda uri: print(uri, flush=True),
            )
            """
        ),
        encoding="utf-8",
    )

    assert list(tmp_path.rglob("*.proto")) == []

    env = dict(os.environ)
    env["HOLONS_SDK_DIR"] = str(_SDK_DIR)
    env["PYTHONPATH"] = str(_SDK_DIR) + os.pathsep + env.get("PYTHONPATH", "")

    proc = subprocess.Popen(
        [sys.executable, str(built_server)],
        cwd=tmp_path,
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    try:
        assert proc.stdout is not None
        uri = proc.stdout.readline().strip()
        assert uri

        channel = dial_uri(uri)
        try:
            stub = describe_pb2_grpc.HolonMetaStub(channel)
            served = stub.Describe(describe_pb2.DescribeRequest(), timeout=5)
        finally:
            channel.close()
    finally:
        rc = _stop_process(proc)
        assert rc == 0

    assert served.manifest.identity.given_name == "Echo"
    assert served.manifest.identity.family_name == "Server"
    assert list(served.services) == []


def test_serve_fails_without_static_describe_response(tmp_path: Path):
    proc, uri, script_path = _start_describe_helper(
        tmp_path,
        include_proto=True,
        with_echo=True,
        proto_text=_INVALID_ECHO_PROTO,
        register_describe=False,
        expect_startup_failure=True,
    )
    assert uri == ""
    stderr = proc.stderr.read() if proc.stderr is not None else ""
    rc = _stop_process(proc)
    script_path.unlink(missing_ok=True)
    assert rc != 0
    assert describe.NO_INCODE_DESCRIPTION_MESSAGE in stderr
