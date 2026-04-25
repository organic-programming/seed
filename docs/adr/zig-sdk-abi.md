# ADR: Zig SDK C ABI Policy

Status: Proposed

Date: 2026-04-25

## Context

The Zig SDK must expose a stable public C ABI in
`sdk/zig-holons/include/holons_sdk.h`. This ABI is the strategic reason for
adding `zig-holons` alongside `rust-holons`: downstream C consumers should be
able to link against the Zig SDK without depending on `sdk/c-holons` or any
gRPC-C++ bridge.

The SDK may use C ABI dependencies internally, including `libprotobuf-c` and
gRPC Core, but those implementation types must not appear in the public ABI.

## Decision

The public ABI uses opaque handles, explicit result structs, and SDK-owned
memory-management functions.

Required ABI coverage:

- SDK lifecycle and version queries;
- connect and disconnect;
- describe;
- serve-and-block and shutdown;
- handle lifecycle;
- discovery;
- hub client.

The header is emitted by `zig build headers` to:

```text
sdk/zig-holons/include/holons_sdk.h
```

The generated header is committed. It is reviewed as public API, not treated as
an unimportant build artifact.

## Versioning

The C ABI has its own semver version. The SDK exports:

- compile-time ABI version macros in `holons_sdk.h`;
- a runtime ABI version function;
- a runtime SDK version function.

Breaking ABI changes require:

- a major ABI version bump;
- a soname bump for shared artifacts;
- a changelog entry;
- migration notes for removed or changed functions.

Non-breaking additions require a minor version bump. Documentation-only changes
or implementation fixes that preserve binary compatibility require a patch bump.

## Handle and Memory Rules

All public handles are opaque pointer types. Callers cannot allocate, inspect,
or embed SDK-owned structs.

Every function that returns allocated memory documents the matching release
function. The C ABI never requires callers to free SDK memory with `free(3)`.

The ABI does not expose:

- `ProtobufCMessage`;
- gRPC Core structs;
- Zig allocator internals;
- generated protobuf-c message structs;
- thread or completion-queue internals.

## Error Rules

Every fallible ABI function returns either a status code or a result struct that
contains a status code plus optional SDK-owned error text. Error text is valid
until released with the documented SDK release function.

The ABI must be safe to call from plain C with no Zig runtime knowledge.

## Test Requirement

`sdk/zig-holons/tests/c_abi/main.c` must compile with `cc`, link
`libholons_zig.a`, dial a live holon, and print a valid describe payload:

```text
cc sdk/zig-holons/tests/c_abi/main.c \
  -I sdk/zig-holons/include \
  -L sdk/zig-holons/zig-out/lib \
  -lholons_zig \
  -o /tmp/c_smoke
/tmp/c_smoke
```

The C smoke test is required before the ABI is considered shippable.
