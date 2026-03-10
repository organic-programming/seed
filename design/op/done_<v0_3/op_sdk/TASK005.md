# TASK005 — Implement HolonMeta `Describe` across SDK fleet

## Context

Depends on: TASK004 (reference `go-holons` implementation, done).

Every SDK must provide HolonMeta self-documentation: parse the holon's
`.proto` files at startup and auto-register a `Describe` RPC handler.
See `PROTOCOL.md` §3.5 for the proto definition.

## Critical rule: no proto copy

The canonical `holonmeta.proto` lives in **one place**:

```
sdk/go-holons/protos/holonmeta/v1/holonmeta.proto
```

**Do NOT copy this file into each SDK.** Each SDK generates
language-specific stubs by pointing `protoc -I` (or equivalent) at the
canonical source via a relative path:

```
../../go-holons/protos
```

Only the **generated stubs** are committed per SDK, under the
language-idiomatic `gen/` location defined in CONVENTIONS.md §3.

### Example: python-holons

```bash
# Generate from canonical source — no local proto copy
protoc \
  --proto_path=../../go-holons/protos \
  --python_out=gen/python \
  --grpc_python_out=gen/python \
  holonmeta/v1/holonmeta.proto
```

Result:

```
sdk/python-holons/
├── gen/python/holonmeta/v1/holonmeta_pb2.py       ← committed
├── gen/python/holonmeta/v1/holonmeta_pb2_grpc.py  ← committed
├── holons/describe.py                              ← implementation
└── (NO protos/holonmeta/ — forbidden)
```

## Per-SDK checklist

Every SDK below must be implemented. Check off each one after commit.

### Tier 1 — Full serve runners

| SDK | Serve entry | Proto parser strategy | Status |
|-----|-------------|----------------------|--------|
| `python-holons` | `holons/serve.py` | `grpcio-tools` / `protobuf` compiler API | ⚠️ has local proto copy — fix |
| `js-holons` | `src/serve.js` | `protobufjs` to parse `.proto` files | ❌ |
| `c-holons` | `src/holons.c` | Simple text parser (no full proto compiler in C) | ❌ |

### Tier 2 — Standalone describe module

| SDK | Module to create | Proto parser strategy | Status |
|-----|-----------------|----------------------|--------|
| `rust-holons` | `src/describe.rs` | `protobuf-parse` crate | ❌ |
| `swift-holons` | `Sources/Holons/Describe.swift` | `SwiftProtobuf` reflection or text parsing | ❌ |
| `dart-holons` | `lib/src/describe.dart` | `protoc_plugin` or text parsing | ❌ |
| `kotlin-holons` | `src/main/kotlin/holons/Describe.kt` | `com.google.protobuf:protobuf-java` descriptor API | ❌ |
| `java-holons` | `src/main/java/holons/Describe.java` | `com.google.protobuf` descriptor API | ❌ |
| `csharp-holons` | `Holons/Describe.cs` | `Google.Protobuf.Reflection` | ❌ |
| `cpp-holons` | `include/holons/describe.hpp` | `google::protobuf::compiler` | ❌ |
| `ruby-holons` | `lib/holons/describe.rb` | `google-protobuf` gem or text parsing | ❌ |
| `objc-holons` | `src/HolonMeta.m` | Text parsing of `.proto` files | ❌ |
| `js-web-holons` | `src/describe.mjs` | `protobufjs` (same as `js-holons`) | ❌ |

### Fallback: simple text parser

For SDKs where a full proto compiler library is too heavy (C, Obj-C, Ruby),
a simple text-based parser is acceptable. It only needs to extract:
- `service Name { ... }` blocks and their leading comments
- `rpc Method(...)` declarations and their leading comments
- `message Name { ... }` blocks with field declarations and comments
- `@required` and `@example` tags from comment lines

## Implementation per SDK

For each SDK:

1. **Generate stubs** from `../../go-holons/protos` — commit to `gen/`.
2. **Implement `describe` module** — parse protos, build `DescribeResponse`.
3. **Auto-register** in the `serve` runner (Tier 1) or expose as standalone (Tier 2).
4. **Write tests**:
   - Parse the echo-server `echo.proto` → verify correct extraction.
   - Verify `Describe` RPC produces a working response.
   - Verify graceful degradation when no `.proto` files exist.
5. **Delete any local proto copy** if one was committed.
6. **Commit independently** — one SDK per commit.

## Rules

- **Process ALL SDKs in this task.** Do not stop after the first one.
  Iterate through every SDK in the checklist above. Commit each SDK
  independently, then move to the next. The task is complete only when
  every row in the checklist is ✅.
- The `holonmeta.v1` proto is **canonical in `go-holons`** — do not copy or diverge.
- Generated code for `holonmeta.v1` must be committed (per CONVENTIONS.md §5).
- Reference the proto via relative path `../../go-holons/protos`, never via a local copy.
