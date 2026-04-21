from __future__ import annotations

"""HolonMeta Describe implementation for Python holons."""

import tempfile
import threading
from dataclasses import dataclass
from importlib import resources
from pathlib import Path

import grpc
from google.protobuf import descriptor_pb2

from holons.identity import parse_manifest, resolve_manifest_path
from holons.v1 import describe_pb2, describe_pb2_grpc, manifest_pb2

HOLON_META_SERVICE_NAME = "holons.v1.HolonMeta"
NO_INCODE_DESCRIPTION_MESSAGE = "no Incode Description registered — run op build"

_SCALAR_TYPE_NAMES = {
    descriptor_pb2.FieldDescriptorProto.TYPE_DOUBLE: "double",
    descriptor_pb2.FieldDescriptorProto.TYPE_FLOAT: "float",
    descriptor_pb2.FieldDescriptorProto.TYPE_INT64: "int64",
    descriptor_pb2.FieldDescriptorProto.TYPE_UINT64: "uint64",
    descriptor_pb2.FieldDescriptorProto.TYPE_INT32: "int32",
    descriptor_pb2.FieldDescriptorProto.TYPE_FIXED64: "fixed64",
    descriptor_pb2.FieldDescriptorProto.TYPE_FIXED32: "fixed32",
    descriptor_pb2.FieldDescriptorProto.TYPE_BOOL: "bool",
    descriptor_pb2.FieldDescriptorProto.TYPE_STRING: "string",
    descriptor_pb2.FieldDescriptorProto.TYPE_GROUP: "group",
    descriptor_pb2.FieldDescriptorProto.TYPE_BYTES: "bytes",
    descriptor_pb2.FieldDescriptorProto.TYPE_UINT32: "uint32",
    descriptor_pb2.FieldDescriptorProto.TYPE_SFIXED32: "sfixed32",
    descriptor_pb2.FieldDescriptorProto.TYPE_SFIXED64: "sfixed64",
    descriptor_pb2.FieldDescriptorProto.TYPE_SINT32: "sint32",
    descriptor_pb2.FieldDescriptorProto.TYPE_SINT64: "sint64",
}

__all__ = [
    "HOLON_META_SERVICE_NAME",
    "NO_INCODE_DESCRIPTION_MESSAGE",
    "IncodeDescriptionError",
    "build_response",
    "register",
    "static_response",
    "use_static_response",
    "describe_pb2",
    "describe_pb2_grpc",
]


class IncodeDescriptionError(RuntimeError):
    """Raised when runtime Describe registration is unavailable."""


_STATIC_RESPONSE_LOCK = threading.RLock()
_STATIC_RESPONSE: describe_pb2.DescribeResponse | None = None


class _HolonMetaServicer(describe_pb2_grpc.HolonMetaServicer):
    def __init__(self, response: describe_pb2.DescribeResponse):
        self._response = response

    def Describe(
        self,
        _request: describe_pb2.DescribeRequest,
        _context: grpc.ServicerContext,
    ) -> describe_pb2.DescribeResponse:
        return self._response


@dataclass(frozen=True)
class _CommentMeta:
    description: str = ""
    required: bool = False
    example: str = ""


@dataclass(frozen=True)
class _MessageRef:
    file_name: str
    full_name: str
    path: tuple[int, ...]
    proto: descriptor_pb2.DescriptorProto


@dataclass(frozen=True)
class _EnumRef:
    file_name: str
    full_name: str
    path: tuple[int, ...]
    proto: descriptor_pb2.EnumDescriptorProto


def build_response(
    proto_dir: str | Path,
) -> describe_pb2.DescribeResponse:
    """Build-time utility for op build to derive a Describe response."""
    resolved = parse_manifest(resolve_manifest_path(proto_dir))
    services = _parse_services(proto_dir)
    return describe_pb2.DescribeResponse(
        manifest=_proto_manifest_from_resolved(resolved),
        services=services,
    )


def use_static_response(response: describe_pb2.DescribeResponse | None) -> None:
    """Register the build-generated Describe response for runtime use."""
    global _STATIC_RESPONSE
    with _STATIC_RESPONSE_LOCK:
        _STATIC_RESPONSE = _clone_response(response)


def static_response() -> describe_pb2.DescribeResponse | None:
    """Return the registered static Describe response, if any."""
    with _STATIC_RESPONSE_LOCK:
        return _clone_response(_STATIC_RESPONSE)


def register(server: grpc.Server) -> None:
    """Register the static HolonMeta gRPC service on a server."""
    if server is None:
        raise ValueError("grpc server is required")

    response = static_response()
    if response is None:
        raise IncodeDescriptionError(NO_INCODE_DESCRIPTION_MESSAGE)
    describe_pb2_grpc.add_HolonMetaServicer_to_server(_HolonMetaServicer(response), server)


def _clone_response(
    response: describe_pb2.DescribeResponse | None,
) -> describe_pb2.DescribeResponse | None:
    if response is None:
        return None
    clone = describe_pb2.DescribeResponse()
    clone.CopyFrom(response)
    return clone


def _proto_manifest_from_resolved(resolved) -> manifest_pb2.HolonManifest:
    return manifest_pb2.HolonManifest(
        identity=manifest_pb2.HolonManifest.Identity(
            uuid=resolved.identity.uuid,
            given_name=resolved.identity.given_name,
            family_name=resolved.identity.family_name,
            motto=resolved.identity.motto,
            composer=resolved.identity.composer,
            status=resolved.identity.status,
            born=resolved.identity.born,
            aliases=resolved.identity.aliases,
        ),
        lang=resolved.identity.lang,
        kind=resolved.kind,
        build=manifest_pb2.HolonManifest.Build(
            runner=resolved.build_runner,
            main=resolved.build_main,
        ),
        artifacts=manifest_pb2.HolonManifest.Artifacts(
            binary=resolved.artifact_binary,
            primary=resolved.artifact_primary,
        ),
    )


def _parse_services(proto_dir: str | Path) -> list[describe_pb2.ServiceDoc]:
    root = Path(proto_dir).expanduser()
    if not root.exists():
        return []
    if not root.is_dir():
        raise ValueError(f"{root} is not a directory")

    rel_files = _collect_proto_files(root)
    if not rel_files:
        return []

    descriptor_set = _compile_descriptor_set(root.resolve(), rel_files)
    return _ResponseBuilder(descriptor_set.file, set(rel_files)).build_services(rel_files)


def _collect_proto_files(root: Path) -> list[str]:
    files: list[str] = []
    for path in sorted(root.rglob("*.proto")):
        if any(part.startswith(".") for part in path.relative_to(root).parts):
            continue
        files.append(path.relative_to(root).as_posix())
    return files


def _compile_descriptor_set(
    proto_dir: Path,
    rel_files: list[str],
) -> descriptor_pb2.FileDescriptorSet:
    try:
        from grpc_tools import protoc
    except ImportError as exc:  # pragma: no cover - dependency guard
        raise RuntimeError("grpcio-tools is required for holons.describe") from exc

    descriptor_set = descriptor_pb2.FileDescriptorSet()
    with resources.as_file(resources.files("grpc_tools").joinpath("_proto")) as builtin_include:
        with tempfile.NamedTemporaryFile(suffix=".pb", delete=False) as tmp:
            descriptor_path = Path(tmp.name)

        try:
            result = protoc.main(
                [
                    "grpc_tools.protoc",
                    f"-I{proto_dir}",
                    f"-I{builtin_include}",
                    f"--descriptor_set_out={descriptor_path}",
                    "--include_imports",
                    "--include_source_info",
                    *rel_files,
                ]
            )
            if result != 0:
                raise RuntimeError(f"grpc_tools.protoc exited with status {result}")
            descriptor_set.ParseFromString(descriptor_path.read_bytes())
        finally:
            descriptor_path.unlink(missing_ok=True)

    return descriptor_set


class _ResponseBuilder:
    def __init__(
        self,
        files: list[descriptor_pb2.FileDescriptorProto],
        input_files: set[str],
    ):
        self._input_files = input_files
        self._files_by_name = {file_proto.name: file_proto for file_proto in files}
        self._comment_lookups = {
            file_proto.name: _build_comment_lookup(file_proto) for file_proto in files
        }
        self._messages: dict[str, _MessageRef] = {}
        self._enums: dict[str, _EnumRef] = {}

        for file_proto in files:
            if file_proto.name not in self._input_files:
                continue
            for index, message in enumerate(file_proto.message_type):
                self._index_message(file_proto, message, (4, index), file_proto.package)
            for index, enum in enumerate(file_proto.enum_type):
                self._index_enum(file_proto, enum, (5, index), file_proto.package)

    def build_services(self, rel_files: list[str]) -> list[describe_pb2.ServiceDoc]:
        services: list[describe_pb2.ServiceDoc] = []
        for file_name in rel_files:
            file_proto = self._files_by_name.get(file_name)
            if file_proto is None:
                continue
            for service_index, service in enumerate(file_proto.service):
                service_name = _qualify(file_proto.package, service.name)
                if service_name == HOLON_META_SERVICE_NAME:
                    continue
                services.append(self._build_service(file_proto, service_index, service))
        return services

    def _build_service(
        self,
        file_proto: descriptor_pb2.FileDescriptorProto,
        service_index: int,
        service: descriptor_pb2.ServiceDescriptorProto,
    ) -> describe_pb2.ServiceDoc:
        meta = _parse_comment_block(self._comment(file_proto.name, (6, service_index)))
        return describe_pb2.ServiceDoc(
            name=_qualify(file_proto.package, service.name),
            description=meta.description,
            methods=[
                self._build_method(file_proto, service_index, method_index, method)
                for method_index, method in enumerate(service.method)
            ],
        )

    def _build_method(
        self,
        file_proto: descriptor_pb2.FileDescriptorProto,
        service_index: int,
        method_index: int,
        method: descriptor_pb2.MethodDescriptorProto,
    ) -> describe_pb2.MethodDoc:
        meta = _parse_comment_block(
            self._comment(file_proto.name, (6, service_index, 2, method_index))
        )
        input_name = method.input_type.lstrip(".")
        output_name = method.output_type.lstrip(".")
        return describe_pb2.MethodDoc(
            name=method.name,
            description=meta.description,
            input_type=input_name,
            output_type=output_name,
            input_fields=self._build_fields(input_name, set()),
            output_fields=self._build_fields(output_name, set()),
            client_streaming=method.client_streaming,
            server_streaming=method.server_streaming,
            example_input=meta.example,
        )

    def _build_fields(
        self,
        message_name: str,
        seen: set[str],
    ) -> list[describe_pb2.FieldDoc]:
        message = self._messages.get(message_name)
        if message is None or message.full_name in seen or message.proto.options.map_entry:
            return []

        next_seen = set(seen)
        next_seen.add(message.full_name)

        return [
            self._build_field(message, field_index, field, next_seen)
            for field_index, field in enumerate(message.proto.field)
        ]

    def _build_field(
        self,
        message: _MessageRef,
        field_index: int,
        field: descriptor_pb2.FieldDescriptorProto,
        seen: set[str],
    ) -> describe_pb2.FieldDoc:
        meta = _parse_comment_block(self._comment(message.file_name, message.path + (2, field_index)))
        doc = describe_pb2.FieldDoc(
            name=field.name,
            type=self._field_type_name(field),
            number=field.number,
            description=meta.description,
            label=self._field_label(field),
            required=meta.required,
            example=meta.example,
        )

        if self._is_map_field(field):
            entry = self._messages[field.type_name.lstrip(".")]
            key_field = entry.proto.field[0]
            value_field = entry.proto.field[1]
            doc.map_key_type = self._field_type_name(key_field)
            doc.map_value_type = self._field_type_name(value_field)
            self._attach_field_expansions(doc, value_field, seen)
            return doc

        self._attach_field_expansions(doc, field, seen)
        return doc

    def _attach_field_expansions(
        self,
        doc: describe_pb2.FieldDoc,
        field: descriptor_pb2.FieldDescriptorProto,
        seen: set[str],
    ) -> None:
        enum_type = self._enums.get(field.type_name.lstrip("."))
        if enum_type is not None and enum_type.file_name in self._input_files:
            doc.enum_values.extend(self._build_enum_values(enum_type))

        message_type = self._messages.get(field.type_name.lstrip("."))
        if (
            message_type is not None
            and message_type.file_name in self._input_files
            and not message_type.proto.options.map_entry
        ):
            doc.nested_fields.extend(self._build_fields(message_type.full_name, seen))

    def _build_enum_values(self, enum_type: _EnumRef) -> list[describe_pb2.EnumValueDoc]:
        values: list[describe_pb2.EnumValueDoc] = []
        for value_index, value in enumerate(enum_type.proto.value):
            meta = _parse_comment_block(self._comment(enum_type.file_name, enum_type.path + (2, value_index)))
            values.append(
                describe_pb2.EnumValueDoc(
                    name=value.name,
                    number=value.number,
                    description=meta.description,
                )
            )
        return values

    def _field_label(self, field: descriptor_pb2.FieldDescriptorProto) -> int:
        if self._is_map_field(field):
            return describe_pb2.FIELD_LABEL_MAP
        if field.label == descriptor_pb2.FieldDescriptorProto.LABEL_REPEATED:
            return describe_pb2.FIELD_LABEL_REPEATED
        return describe_pb2.FIELD_LABEL_OPTIONAL

    def _field_type_name(self, field: descriptor_pb2.FieldDescriptorProto) -> str:
        if self._is_map_field(field):
            entry = self._messages[field.type_name.lstrip(".")]
            key_type = self._field_type_name(entry.proto.field[0])
            value_type = self._field_type_name(entry.proto.field[1])
            return f"map<{key_type}, {value_type}>"
        if field.type_name:
            return field.type_name.lstrip(".")
        return _SCALAR_TYPE_NAMES.get(
            field.type,
            descriptor_pb2.FieldDescriptorProto.Type.Name(field.type).removeprefix("TYPE_").lower(),
        )

    def _is_map_field(self, field: descriptor_pb2.FieldDescriptorProto) -> bool:
        if field.type != descriptor_pb2.FieldDescriptorProto.TYPE_MESSAGE or not field.type_name:
            return False
        message = self._messages.get(field.type_name.lstrip("."))
        return message is not None and message.proto.options.map_entry

    def _comment(self, file_name: str, path: tuple[int, ...]) -> str:
        return self._comment_lookups.get(file_name, {}).get(path, "")

    def _index_message(
        self,
        file_proto: descriptor_pb2.FileDescriptorProto,
        message: descriptor_pb2.DescriptorProto,
        path: tuple[int, ...],
        scope: str,
    ) -> None:
        full_name = _qualify(scope, message.name)
        self._messages[full_name] = _MessageRef(file_proto.name, full_name, path, message)
        for index, nested_message in enumerate(message.nested_type):
            self._index_message(file_proto, nested_message, path + (3, index), full_name)
        for index, enum in enumerate(message.enum_type):
            self._index_enum(file_proto, enum, path + (4, index), full_name)

    def _index_enum(
        self,
        file_proto: descriptor_pb2.FileDescriptorProto,
        enum: descriptor_pb2.EnumDescriptorProto,
        path: tuple[int, ...],
        scope: str,
    ) -> None:
        full_name = _qualify(scope, enum.name)
        self._enums[full_name] = _EnumRef(file_proto.name, full_name, path, enum)


def _build_comment_lookup(
    file_proto: descriptor_pb2.FileDescriptorProto,
) -> dict[tuple[int, ...], str]:
    lookup: dict[tuple[int, ...], str] = {}
    for location in file_proto.source_code_info.location:
        comment = _source_comments(location)
        if comment:
            lookup[tuple(location.path)] = comment
    return lookup


def _parse_comment_block(raw: str) -> _CommentMeta:
    lines = [line.strip() for line in raw.strip().splitlines()]
    description: list[str] = []
    examples: list[str] = []
    required = False

    for line in lines:
        if not line:
            continue
        if line == "@required":
            required = True
            continue
        if line.startswith("@example"):
            example = line.removeprefix("@example").strip()
            if example:
                examples.append(example)
            continue
        description.append(line)

    return _CommentMeta(
        description=" ".join(description),
        required=required,
        example="\n".join(examples),
    )


def _source_comments(location: descriptor_pb2.SourceCodeInfo.Location) -> str:
    leading = location.leading_comments.strip()
    if leading:
        return leading
    return location.trailing_comments.strip()


def _qualify(scope: str, name: str) -> str:
    return f"{scope}.{name}" if scope else name
