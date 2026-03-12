# CPP_TASK001 — Complete C++ SDK connect + serve ✅

## Status

Complete ✅

- `cpp-holons`: `96d6470` | https://github.com/organic-programming/cpp-holons/commit/96d6470
- `seed`: `f4974af` | https://github.com/organic-programming/seed/commit/f4974af

## Current State

`connect(slug)` and `serve(...)` are implemented in `cpp-holons`.

**Implemented:**
- Discovery via `find_by_slug()`
- Port-file reuse/probe via `usable_port_file()`
- Process spawn via `start_tcp_holon()` / `start_stdio_holon()` on POSIX and Windows
- Dial via `grpc::CreateChannel()` + readiness via `WaitForConnected()`
- `serve.hpp` with listener parsing, multi-listener support, and graceful shutdown
- CMake proto/grpc codegen for bundled sample protos
- Sample holon source in `examples/echo_server.cpp`
- Direct stdio gRPC transport on POSIX when grpc++ exposes the required FD APIs
- Proxy fallback where direct stdio FD transport is unavailable

**Tests:** `./build/test_runner` passed with `140 passed, 0 failed`.
`connect` tests are still skipped on machines without grpc++ headers.

## Objective

Bring `cpp-holons` to parity with Go/Rust/Swift SDKs.

## Changes

### 1. Windows connect support

Implement `start_tcp_holon()` / `start_stdio_holon()` on Windows
using `CreateProcess`. Currently the code explicitly errors out.

### 2. gRPC serve support

Add a `serve.hpp` with:
- `grpc::ServerBuilder`-based server setup
- `--listen` URI parsing (tcp, unix, stdio)
- Multi-listener support
- Graceful shutdown on SIGTERM/SIGINT

### 3. Generated service stubs

Add a proto codegen step or CMake target that produces
`*.grpc.pb.h` + `*.grpc.pb.cc` from holon protos, enabling
holons to serve their contracts.

### 4. Stdio transport

Replace the loopback proxy with a proper stdio gRPC transport
matching the Go/Rust implementation (direct stdin/stdout pipe).

## Acceptance Criteria

- [x] `connect(slug)` works on macOS and Linux (POSIX)
- [x] `connect(slug)` works on Windows
- [x] `serve(listeners, register_fn)` starts a gRPC server
- [x] Direct stdio transport is used where grpc++ FD APIs are available
- [x] Test coverage passes locally; grpc++-gated connect tests run when the dependency is present
- [x] A sample C++ holon can serve and be connected to

## Dependencies

None. Independent of OP tasks.
