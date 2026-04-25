# ADR: Zig SDK Scope and M0 Gate

Status: Proposed, M0 native spike passed, cross-target gate blocked

Date: 2026-04-25

## Context

`sdk/zig-holons` is intended to join the existing SDKs at the
`sdk/rust-holons` parity level. The target transport row is:

| `tcp://` | `unix://` | `stdio://` | `ws://` | `wss://` | `rest+sse` | `hub api` |
|---|---|---|---|---|---|---|
| both | both | both | dial | dial | dial | client |

The SDK is Zig-owned at the public API boundary, but deliberately uses mature C
ABI dependencies internally for protobuf and gRPC. It must not route through any
existing SDK, must not use gRPC-C++ shims, and must emit its own public C ABI.

## Decisions

### Zig Toolchain

Pin Zig `0.16.0` after M0. The official download page exposes:

```text
<h2 id="release-0.16.0">0.16.0</h2>
<li>2026-04-13</li>
https://ziglang.org/download/0.16.0/zig-aarch64-macos-0.16.0.tar.xz
```

The M0 local toolchain was downloaded from the official tarball and reported:

```text
$ /tmp/seed-zig-m0/zig-aarch64-macos-0.16.0/zig version
0.16.0
```

### Protobuf Runtime

Use `libprotobuf-c`, pinned at `v1.5.2`.

Reasons:

- It is a mature C ABI runtime and code generator for protobuf messages.
- It is directly consumable from Zig with `@cImport("protobuf-c/protobuf-c.h")`.
- It keeps wire-format code internal while allowing the Zig SDK to expose
  idiomatic Zig wrappers and its own stable C ABI.

M0 evidence:

```text
$ protoc-c --version
protobuf-c 1.5.2
libprotoc 34.1

$ /tmp/seed-zig-m0/spike/zig-out/bin/m0-spike 127.0.0.1:9090
protobuf-c imported; version=1.5.2 number=1005002
grpc_posix fd APIs imported
grpc channel created for 127.0.0.1:9090
```

Generated `.pb-c.h` and `.pb-c.c` files must be committed under
`sdk/zig-holons/gen/` and example `gen/` directories. `ProtobufCMessage` and
other protobuf-c types are private implementation details and must not leak into
the public Zig API or `include/holons_sdk.h`.

### gRPC Transport

Use gRPC Core C API via Zig `@cImport`, pinned at `v1.80.0`.

Reasons:

- The current stable gRPC release list reports `v1.80.0` as a non-prerelease
  release published on 2026-03-26.
- The gRPC FAQ says the project does not do LTS releases and supports the
  current release and the previous release under its rolling model.
- gRPC Core exposes the C ABI used by multiple language bindings and avoids a
  Zig dependency on C++ symbols or hand-written `extern "C"` shims.

M0 evidence:

```text
$ brew info grpc
grpc: stable 1.80.0 (bottled), HEAD

$ curl -fsSL 'https://api.github.com/repos/grpc/grpc/releases?per_page=10' \
  | jq -r '.[] | [.tag_name, (.prerelease|tostring), .published_at, .name] | @tsv' | head -1
v1.80.0	false	2026-03-26T17:26:33Z	Release v1.80.0

$ pkg-config --modversion grpc
53.0.0
```

The SDK implementation must wrap these C headers:

- `grpc/grpc.h`
- `grpc/credentials.h`
- `grpc/byte_buffer.h`
- `grpc/byte_buffer_reader.h`
- `grpc/grpc_posix.h`

The POSIX fd APIs required for `stdio://` were present:

```text
GRPCAPI grpc_channel* grpc_channel_create_from_fd(...)
GRPCAPI void grpc_server_add_channel_from_fd(...)
```

### Distribution and Cross-Compilation

The production SDK must vendor gRPC and protobuf-c source trees under
`sdk/zig-holons/third_party/` and build them per target. System packages are
acceptable only for local M0 experiments.

M0 native linking against Homebrew packages succeeded:

```text
Build Summary: 3/3 steps succeeded
install success
+- install m0-spike success
   +- compile exe m0-spike Debug native success
```

M0 non-host linking against Homebrew packages failed:

```text
$ zig build -Dtarget=x86_64-macos ...
error: unable to find dynamic system library 'z' using strategy 'paths_first'
searched paths:
  /opt/homebrew/Cellar/grpc/1.80.0/lib/libz.dylib
  /opt/homebrew/Cellar/abseil/20260107.1/lib/libz.dylib
  ...
```

This is expected: `/opt/homebrew` packages are host-arch artifacts on this
arm64 macOS machine. The SDK skeleton must not start until a vendored
gRPC/protobuf-c build proves at least one non-host target. The next M0 task is
to build pinned `grpc@v1.80.0` and `protobuf-c@v1.5.2` from source with Zig or
CMake toolchain files for a non-host target, then rerun the spike against those
artifacts.

### TLS

Use gRPC Core's vendored BoringSSL path for gRPC transports. For Holon-RPC
`wss://` dial support, prefer Zig `std.crypto.tls`; if the pinned Zig version
is insufficient, use the same vendored BoringSSL dependency already required by
gRPC Core.

### Observability

Match the actual Tier 2 Rust SDK behavior:

- structured logs;
- counters, gauges, histograms;
- event bus and lifecycle events;
- chain propagation;
- auto-registered `HolonObservability`;
- `.op/run/` JSONL writers and `meta.json`.

Per `OBSERVABILITY.md` and `sdk/rust-holons/src/observability.rs`, `otel` is
reserved for v2 and is a startup error. Implementing an OTLP exporter would
exceed Rust parity and is out of this chantier.

## M0 Verdict

Native `@cImport` and linking are proven for Zig `0.16.0`,
`libprotobuf-c 1.5.2`, and gRPC Core `v1.80.0`. The cross-target gate is not
yet satisfied because the current spike used host-arch Homebrew packages.

Per the integration plan, do not open the SDK skeleton PR until the vendored
source build proves at least one non-host target.

## Deferred Follow-Up

`op new --lang zig` is intentionally out of scope until the cross-language
`op new --lang <lang>` specification lands. Track a Zig template follow-up when
that spec is published; do not add a Zig-only template first.
