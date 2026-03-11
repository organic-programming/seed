# TASK01 — Shared Greeting Proto

## Summary

Create the single canonical `greeting.proto` in a shared location.
Each daemon and HostUI then has a thin local `.proto` that imports
the shared definition via `import public` and adds holon-specific
meta-annotations (`@required`, `@example`, `@skill` — see
[PROTO.md](../../PROTO.md)).

> [!IMPORTANT]
> **One canonical service definition — per-holon annotation wrappers.**
> The shared proto defines the `GreetingService` and its messages.
> Each holon's local `.proto` re-exports it via `import public` and
> adds its own comments and meta-annotations. This follows PROTO.md
> §2–§3: comments are functional data, not decoration.

## Layout

```
recipes/protos/greeting/v1/
└── greeting.proto              ← canonical service + messages

recipes/daemons/gudule-daemon-greeting-go/protos/greeting/v1/
└── greeting.proto              ← import public + Go-specific annotations

recipes/hostui/gudule-greeting-hostui-flutter/protos/greeting/v1/
└── greeting.proto              ← import public + Dart-specific annotations
```

## Per-Holon Wrapper Pattern

```protobuf
// gudule-daemon-greeting-go/protos/greeting/v1/greeting.proto
syntax = "proto3";
package greeting.v1;

// Re-export the canonical service definition.
import public "greeting/v1/greeting.proto";

// Go daemon-specific annotations can be added here as needed.
```

Each holon's `protoc` invocation uses `-I ../../protos` (or
equivalent) to resolve the shared import. The wrapper allows each
holon to carry its own meta-annotations while the service definition
stays DRY.

## Toolchain Build Templates

TASK01 must also deliver a **proto build template** per toolchain,
showing the exact configuration to resolve `import public` from
`recipes/protos/`. Without these, each daemon/HostUI extraction will
stall on `protoc` path errors.

| Toolchain | Config file | Proto include path |
|---|---|---|
| Go | `buf.gen.yaml` or `Makefile` | `-I ../../protos` |
| Rust | `build.rs` | `.proto_path("../../protos")` |
| Swift | `Package.swift` + `protoc` script | `--proto_path=../../protos` |
| Kotlin | `build.gradle.kts` | `protobuf { protoc { path = ... } }` |
| Dart/Flutter | `build.yaml` or `protoc` script | `--proto_path=../../protos` |
| C# | `.csproj` + Grpc.Tools | `<Protobuf Include="..." ProtoRoot="..." />` |
| Node/Web | `buf.gen.yaml` or `npx protoc` | `--proto_path=../../protos` |
| Python | `buf.gen.yaml` or `grpc_tools.protoc` script | `--proto_path=../../protos` |
| C++/Qt | `CMakeLists.txt` | `protobuf_generate(IMPORT_DIRS ../../protos)` |

## Acceptance Criteria

- [ ] Canonical `greeting.proto` extracted from current `go-dart-holons`
- [ ] Placed in `recipes/protos/greeting/v1/`
- [ ] Proto package: `greeting.v1`
- [ ] Service: `GreetingService` with `ListLanguages` and `SayHello` RPCs
- [ ] Messages: `Language`, `ListLanguagesRequest/Response`, `SayHelloRequest/Response`
- [ ] Comments and meta-annotations follow [PROTO.md](../../PROTO.md)
- [ ] Wrapper pattern documented and one example wrapper created
- [ ] Build templates documented for all 9 toolchains (including Python)

## Dependencies

None (first task in the chain).
