# TASK005 — Implement HolonMeta `Describe` across SDK fleet

## Context

Depends on: TASK004 (reference `go-holons` implementation).

Once `go-holons` has the reference `HolonMeta` implementation, each SDK
must provide the same capability: parse the holon's `.proto` files at
startup and auto-register a `Describe` RPC handler.

See `PROTOCOL.md` §3.5 for the proto definition.

## SDKs to implement

The proto file (`holonmeta/v1/holonmeta.proto`) must be generated for
each language. Each SDK's `serve` runner must auto-register `HolonMeta`.

### Tier 1 — SDKs with full serve runners

| SDK | Serve entry point | Proto parser strategy |
|---|---|---|
| `js-holons` | `src/server.mjs` | Use `protobufjs` to parse `.proto` files |
| `python-holons` | `holons/serve.py` | Use `grpcio-tools` or `protobuf` compiler API |
| `c-holons` | `src/holons.c` | Simple text parser for comments (no full proto compiler in C) |

### Tier 2 — SDKs with partial serve (flag parsing)

These SDKs don't have a full `serve` runner yet. Implement `describe` as
a standalone module that can be manually registered:

| SDK | Module to create | Proto parser strategy |
|---|---|---|
| `rust-holons` | `src/describe.rs` | Use `protobuf-parse` crate |
| `swift-holons` | `Sources/Holons/Describe.swift` | Use `SwiftProtobuf` reflection or text parsing |
| `dart-holons` | `lib/src/describe.dart` | Use `protoc_plugin` or text parsing |
| `kotlin-holons` | `src/main/kotlin/holons/Describe.kt` | Use `com.google.protobuf:protobuf-java` descriptor API |
| `java-holons` | `src/main/java/holons/Describe.java` | Use `com.google.protobuf` descriptor API |
| `csharp-holons` | `Holons/Describe.cs` | Use `Google.Protobuf.Reflection` |
| `cpp-holons` | `include/holons/describe.hpp` | Use `google::protobuf::compiler` |
| `ruby-holons` | `lib/holons/describe.rb` | Use `google-protobuf` gem or text parsing |
| `objc-holons` | `src/HolonMeta.m` | Text parsing of `.proto` files |
| `js-web-holons` | `src/describe.mjs` | Use `protobufjs` (same as `js-holons`) |

### Fallback: simple text parser

For SDKs where a full proto compiler library is too heavy (C, Obj-C, Ruby),
a simple text-based parser is acceptable. It only needs to extract:
- `service Name { ... }` blocks and their leading comments
- `rpc Method(...)` declarations and their leading comments
- `message Name { ... }` blocks with field declarations and comments
- `@required` and `@example` tags from comment lines

This is a subset of proto syntax — no need for a full compiler.

## Testing per SDK

For each:
1. Parse the echo-server `echo.proto` and verify correct extraction.
2. Verify `HolonMeta` registration produces a working `Describe` RPC.
3. Verify graceful degradation when no `.proto` files exist.

## Rules

- One SDK at a time. Commit each independently.
- The `holonmeta.v1` proto is the **same** across all SDKs — do not diverge.
- Generated code for `holonmeta.v1` must be committed (per CONVENTIONS.md §5).
