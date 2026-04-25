# ADR: Zig SDK Scope and M0 Gate

Status: Proposed, M0 gate passed

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

This was expected: `/opt/homebrew` packages are host-arch artifacts on this
arm64 macOS machine. M0 therefore switched to the production path: vendored
source builds driven by Zig `build.zig` and CMake.

Vendored M0 source pins:

```text
grpc v1.80.0       f5e2d6e856176c2f6b7691032adfefe21e5f64c1
protobuf-c v1.5.2  4719fdd7760624388c2c5b9d6759eb6a47490626
```

The scratch `build.zig` M0 step orchestrated these CMake builds:

- host macOS arm64 gRPC Core `install`, static libraries, all providers set to
  `module`;
- host macOS arm64 protobuf-c runtime plus host `protoc-gen-c`;
- cross aarch64-linux-musl gRPC Core `install` with a Zig cc/c++ toolchain
  file;
- cross aarch64-linux-musl protobuf-c runtime;
- `protoc --c_out` for `examples/_protos/v1/greeting.proto`;
- host Zig TCP and stdio spike binaries;
- cross Zig TCP spike binary.

Clean reproducibility command:

```sh
cd /tmp/seed-zig-m0-vendored
rm -rf build/zig-grpc-host build/zig-protobuf-c-host \
  build/zig-grpc-aarch64-linux-musl build/zig-protobuf-c-aarch64-linux-musl \
  out/zig-host out/zig-aarch64-linux-musl \
  spike/zigbuild-gen spike/zigbuild-zig-grpc-client \
  spike/zigbuild-zig-stdio-spike \
  spike/zigbuild-zig-grpc-client-aarch64-linux-musl
/tmp/seed-zig-m0/zig-aarch64-macos-0.16.0/zig build m0 --summary all
```

Result:

```text
Build Summary: 10/10 steps succeeded
m0 success
+- m0-host success
|  +- run bash success 3s
|     +- run bash success 216ms
|        +- run bash success 7s
|           +- run bash success 11m
+- m0-cross success
   +- run bash success 9s
      +- run bash success 1s
      |  +- run bash success 13m
      +- run bash (+1 more reused dependencies)
```

The CMake toolchain used these wrappers:

```sh
/tmp/seed-zig-m0-vendored/toolchain/zigcc-aarch64-linux-musl \
  # execs zig cc -target aarch64-linux-musl -Wno-nullability-completeness
/tmp/seed-zig-m0-vendored/toolchain/zigcxx-aarch64-linux-musl \
  # execs zig c++ -target aarch64-linux-musl -Wno-nullability-completeness
```

The equivalent cross commands driven by `build.zig` were:

```sh
cmake -S third_party/grpc -B build/zig-grpc-aarch64-linux-musl -G Ninja \
  -DCMAKE_TOOLCHAIN_FILE=/tmp/seed-zig-m0-vendored/toolchain/aarch64-linux-musl.cmake \
  -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_INSTALL_PREFIX=/tmp/seed-zig-m0-vendored/out/zig-aarch64-linux-musl \
  -DBUILD_SHARED_LIBS=OFF \
  -DgRPC_INSTALL=ON \
  -DgRPC_BUILD_TESTS=OFF \
  -DgRPC_BUILD_CODEGEN=OFF \
  -DgRPC_BUILD_GRPC_CPP_PLUGIN=OFF \
  -DgRPC_BUILD_GRPC_CSHARP_PLUGIN=OFF \
  -DgRPC_BUILD_GRPC_NODE_PLUGIN=OFF \
  -DgRPC_BUILD_GRPC_OBJECTIVE_C_PLUGIN=OFF \
  -DgRPC_BUILD_GRPC_PHP_PLUGIN=OFF \
  -DgRPC_BUILD_GRPC_PYTHON_PLUGIN=OFF \
  -DgRPC_BUILD_GRPC_RUBY_PLUGIN=OFF \
  -DgRPC_ABSL_PROVIDER=module \
  -DgRPC_CARES_PROVIDER=module \
  -DgRPC_PROTOBUF_PROVIDER=module \
  -DgRPC_RE2_PROVIDER=module \
  -DgRPC_SSL_PROVIDER=module \
  -DgRPC_ZLIB_PROVIDER=module \
  -Dprotobuf_BUILD_TESTS=OFF \
  -Dprotobuf_BUILD_PROTOC_BINARIES=OFF \
  -Dprotobuf_BUILD_LIBPROTOC=OFF \
  -DRE2_BUILD_TESTING=OFF \
  -DCARES_BUILD_TOOLS=OFF \
  -DZLIB_BUILD_EXAMPLES=OFF
cmake --build build/zig-grpc-aarch64-linux-musl --target install --parallel 8

cmake -S third_party/protobuf-c/build-cmake \
  -B build/zig-protobuf-c-aarch64-linux-musl -G Ninja \
  -DCMAKE_TOOLCHAIN_FILE=/tmp/seed-zig-m0-vendored/toolchain/aarch64-linux-musl.cmake \
  -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_INSTALL_PREFIX=/tmp/seed-zig-m0-vendored/out/zig-aarch64-linux-musl \
  -DCMAKE_PREFIX_PATH=/tmp/seed-zig-m0-vendored/out/zig-aarch64-linux-musl \
  -DBUILD_SHARED_LIBS=OFF \
  -DBUILD_PROTOC=OFF \
  -DBUILD_TESTS=OFF
cmake --build build/zig-protobuf-c-aarch64-linux-musl --target install --parallel 8

LIBS="$(PKG_CONFIG_PATH=/tmp/seed-zig-m0-vendored/out/zig-aarch64-linux-musl/lib/pkgconfig \
  pkg-config --libs --static grpc_unsecure) -lprotobuf-c"
/tmp/seed-zig-m0/zig-aarch64-macos-0.16.0/zig build-exe \
  -target aarch64-linux-musl -O ReleaseFast \
  spike/main.zig spike/zigbuild-gen/v1/greeting.pb-c.c \
  -I/tmp/seed-zig-m0-vendored/out/zig-aarch64-linux-musl/include \
  -I/tmp/seed-zig-m0-vendored/spike/zigbuild-gen \
  -L/tmp/seed-zig-m0-vendored/out/zig-aarch64-linux-musl/lib \
  $LIBS -lc -lc++ \
  -femit-bin=/tmp/seed-zig-m0-vendored/spike/zigbuild-zig-grpc-client-aarch64-linux-musl
```

Artifact verification:

```text
spike/zigbuild-zig-grpc-client                         Mach-O 64-bit executable arm64
spike/zigbuild-zig-stdio-spike                         Mach-O 64-bit executable arm64
spike/zigbuild-zig-grpc-client-aarch64-linux-musl      ELF 64-bit LSB executable, ARM aarch64
build/zig-grpc-host/.../call_arena_allocator.cc.o      Mach-O 64-bit object arm64
build/zig-grpc-aarch64-linux-musl/.../call_arena_allocator.cc.o
                                                        ELF 64-bit LSB relocatable, ARM aarch64
```

Cross runtime verification used Docker Desktop's Linux arm64 VM as the QEMU or
real-Linux alternative. The Go holon and the Zig client both ran on the Linux
arm64 side:

```sh
docker image inspect 02f8efbefad6 --format '{{.Os}}/{{.Architecture}}'
docker run --rm --platform linux/arm64 \
  -v /tmp/seed-zig-m0-vendored/spike:/spike \
  -w /spike \
  02f8efbefad6 \
  sh -lc 'uname -m; ls -l /lib/ld-musl*; \
    ./gabriel-greeting-go-linux-arm64 serve --listen tcp://127.0.0.1:9090 >/tmp/go-server.log 2>&1 & pid=$!; \
    for i in 1 2 3 4 5; do \
      ./zigbuild-zig-grpc-client-aarch64-linux-musl && status=0 && break || status=$?; \
      sleep 1; \
    done; \
    kill $pid 2>/dev/null || true; cat /tmp/go-server.log; exit ${status:-1}'
```

Output excerpt:

```text
linux/arm64
aarch64
/lib/ld-musl-aarch64.so.1
greeting=Bonjour Bob language=French lang_code=fr rpc_ms=5.771
gRPC server listening on tcp://127.0.0.1:9090 (reflection OFF)
```

### Stdio gRPC Mechanism

The M0 stdio implementation uses a Unix `socketpair` bridge.

`grpc_channel_create_from_fd` and `grpc_server_add_channel_from_fd` expect a
connected socket fd, while the existing holon stdio contract exposes two
unidirectional pipes. The bridge pumps child stdout into one side of a Unix
socketpair and pumps the other side back into child stdin. gRPC Core then owns
the opposite socket fd, so the transport remains native gRPC with no JSON
fallback.

The server ordering that worked reliably was:

```text
grpc_server_create
grpc_server_register_completion_queue
grpc_server_start
grpc_server_add_channel_from_fd
grpc_server_request_call
```

Zig dialing the Go stdio server:

```text
$ /tmp/seed-zig-m0-vendored/spike/zigbuild-zig-stdio-spike
gRPC server listening on stdio:// (reflection OFF)
zig->go stdio greeting=Bonjour Bob language=French lang_code=fr rpc_ms=6.414
```

Go dialing the Zig stdio server:

```text
$ go run /tmp/seed-zig-m0-vendored/spike/go_stdio_client.go
zig stdio server: grpc init fd=3
zig stdio server: started
go->zig stdio served greeting for name=Bob lang_code=fr
go->zig stdio greeting=Bonjour Bob language=French lang_code=fr rpc_ms=1.228
```

### M0 Latency Notes

These are rough single-machine sanity checks, not benchmarks.

```text
tcp:// Zig -> Go, host macOS arm64 loopback:
1.620 ms, 1.350 ms, 1.544 ms, 1.787 ms, 1.080 ms; mean about 1.48 ms

stdio:// Zig -> Go, host macOS arm64 socketpair bridge:
32.600 ms first cold process, then 6.414 ms and 6.394 ms warm-ish; steady rough about 6.4 ms

stdio:// Go -> Zig, host macOS arm64 socketpair bridge:
1.374 ms, 1.103 ms, 1.228 ms; mean about 1.24 ms

tcp:// Zig -> Go, Linux arm64 container:
5.771 ms after one expected startup retry while the Go server bound the port
```

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

M0 is green. Zig `0.16.0`, vendored `libprotobuf-c v1.5.2`, vendored gRPC Core
`v1.80.0`, host TCP gRPC, Linux arm64 cross TCP gRPC, and stdio gRPC in both
directions are proven without JSON fallback and without gRPC-C++.

The SDK skeleton PR remains gated on explicit approval after this ADR update.

## Deferred Follow-Up

`op new --lang zig` is intentionally out of scope until the cross-language
`op new --lang <lang>` specification lands. Track a Zig template follow-up when
that spec is published; do not add a Zig-only template first.
