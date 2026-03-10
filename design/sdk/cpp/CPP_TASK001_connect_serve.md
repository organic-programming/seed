# CPP_TASK001 — Complete C++ SDK connect + serve

## Current State

`connect(slug)` is **partially implemented** (header-only in
`holons.hpp`). Codex audit results:

**Working (POSIX):**
- Discovery via `find_by_slug()`
- Port-file reuse/probe via `usable_port_file()`
- Process spawn via `start_tcp_holon()` / `start_stdio_holon()`
- Dial via `grpc::CreateChannel()` + readiness via `WaitForConnected()`

**Missing:**
- Windows slug startup — explicitly unimplemented
- Graceful degradation without grpc++ — falls back to dummy shims
- No gRPC serve/server support (`grpc::Server`, `ServerBuilder`)
- No generated `*.grpc.pb.h` service stubs
- Stdio transport uses loopback proxy, not dedicated stdio gRPC

**Tests:** 137 passed, 0 failed. Connect tests skipped without
grpc++ headers.

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

- [ ] `connect(slug)` works on macOS and Linux (POSIX) — already done
- [ ] `connect(slug)` works on Windows
- [ ] `serve(listeners, register_fn)` starts a gRPC server
- [ ] Stdio transport without loopback proxy
- [ ] `make test` passes including connect tests (with grpc++)
- [ ] A sample C++ holon can serve and be connected to

## Dependencies

None. Independent of OP tasks.
