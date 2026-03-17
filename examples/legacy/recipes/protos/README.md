# Shared Proto Contracts

`recipes/protos/` holds canonical recipe contracts that stay language-agnostic.
Per-holon wrappers re-export them with `import public` from local `protos/`
trees so each daemon or HostUI can keep its own contract surface.

## Canonical layout

```text
recipes/protos/greeting/v1/greeting.proto
```

Local wrappers use the same import path and rely on an extra include root:

```protobuf
syntax = "proto3";

package greeting.v1;

import public "greeting/v1/greeting.proto";
```

## Include roots

Every generator needs two inputs:

- the canonical include root: `recipes/protos`
- the local wrapper tree when compiling a wrapper for docs/introspection

For codegen in v0.4.1, generate from the canonical proto directly and point the
output at the target holon. That avoids local-wrapper import cycles.

## Toolchain templates

- Go:
  `protoc -I recipes/protos --go_out=paths=source_relative,Mgreeting/v1/greeting.proto=github.com/organic-programming/seed/recipes/daemons/gudule-daemon-greeting-go/gen/go/greeting/v1;greetingv1:recipes/daemons/gudule-daemon-greeting-go/gen/go --go-grpc_out=paths=source_relative,Mgreeting/v1/greeting.proto=github.com/organic-programming/seed/recipes/daemons/gudule-daemon-greeting-go/gen/go/greeting/v1;greetingv1:recipes/daemons/gudule-daemon-greeting-go/gen/go recipes/protos/greeting/v1/greeting.proto`
- Rust:
  use `tonic_build::configure().compile_protos(&["../../recipes/protos/greeting/v1/greeting.proto"], &["../../recipes/protos"])?;`
- Swift:
  `protoc --proto_path=../../recipes/protos --swift_out=Sources/Generated --grpc-swift_out=Sources/Generated ../../recipes/protos/greeting/v1/greeting.proto`
- Kotlin:
  set `srcDir("../../recipes/protos")` and compile `greeting/v1/greeting.proto` through the Gradle protobuf plugin.
- Dart / Flutter:
  `protoc -I recipes/protos --dart_out=grpc:recipes/hostui/gudule-greeting-hostui-flutter/lib/gen recipes/protos/greeting/v1/greeting.proto`
- C#:
  `<Protobuf Include="../../recipes/protos/greeting/v1/greeting.proto" ProtoRoot="../../recipes/protos" GrpcServices="Client" />`
- Node / Web:
  `protoc --proto_path=../../recipes/protos --js_out=import_style=commonjs,binary:src/gen --grpc-web_out=import_style=typescript,mode=grpcwebtext:src/gen ../../recipes/protos/greeting/v1/greeting.proto`
- Python:
  `python -m grpc_tools.protoc --proto_path=../../recipes/protos --python_out=gen --grpc_python_out=gen ../../recipes/protos/greeting/v1/greeting.proto`
- C++ / Qt:
  `protobuf_generate(TARGET greeting IMPORT_DIRS ../../recipes/protos PROTOS ../../recipes/protos/greeting/v1/greeting.proto)`

## Notes

- The canonical proto intentionally has no language-specific package options.
- Go requires the explicit `M...=import/path;package` mapping shown above.
- `go-holons Describe` in v0.4.1 understands wrapper protos that import this
  shared tree from sibling `recipes/protos` directories.
