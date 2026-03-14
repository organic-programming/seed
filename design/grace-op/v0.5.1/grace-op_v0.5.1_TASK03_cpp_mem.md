# TASK03 — C++: Add `mem://` + Document stdio POSIX-only

## Summary

`sdk/cpp-holons/include/holons/serve.hpp` currently throws for anything beyond
`tcp://`, `unix://`, and `stdio://`. `mem://` has no gRPC++ standard in-process
channel — but gRPC++ provides `grpc::experimental::ChannelFromFd` and
`grpc::CreateCustomChannel` with in-process credentials.

Implement `mem://` if the `grpc::experimental` API is available (via a
`HOLONS_HAS_GRPC_IP` compile-time guard), and formally document the
stdio POSIX-only constraint.

## Target

| Transport | Before | After |
|-----------|:------:|:-----:|
| `tcp://` | ✅ | ✅ |
| `unix://` | ✅ | ✅ |
| `stdio://` | ✅ (POSIX) | ✅ (POSIX) |
| `mem://` | ❌ | ✅ if `HOLONS_HAS_GRPC_IP` |
| ws server | ❌ | 🚫 library |

## Implementation

### `mem://` — in-process channel

gRPC++ exposes `grpc::experimental::CreateLocalChannel` / `CreateLocalServer`
since gRPC 1.57. Guard behind `HOLONS_HAS_GRPC_IP`:

```cpp
#if HOLONS_HAS_GRPCPP && __has_include(<grpcpp/create_channel_binder.h>)
#define HOLONS_HAS_GRPC_IP 1
#endif
```

In `serve::start()`, when `parsed.scheme == "mem"`:
1. Create a `grpc::LocalServerCredentials(LOCAL_TCP)` listener on loopback:0.
2. Register the bound address in a process-local registry (thread-safe map `mem-name → "127.0.0.1:port"`) so `transport::dial("mem://name")` can resolve it.
3. Expose `transport::dial_mem(uri)` → `grpc::CreateCustomChannel(...)`.

### stdio POSIX-only

In `serve.hpp`, add a clear compile-time diagnostic:

```cpp
#if !HOLONS_HAS_GRPC_FD
static_assert(false,
    "stdio:// serve requires POSIX file-descriptor support "
    "(HOLONS_HAS_GRPC_FD). Not supported on Windows.");
#endif
```

Add a `#pragma message` on POSIX to document the limitation without failing.

## Acceptance Criteria

- [ ] `cmake --build` + `ctest` pass on macOS arm64
- [ ] `mem_round_trip` test (gtest): serve on `mem://test` then dial and call Describe
- [ ] `stdio` test unchanged — still passes on macOS, compile-time guard documented
- [ ] On a Windows cross-compile (CI or documented manual step): clear `static_assert` fires for stdio

## Dependencies

`sdk/cpp-holons` only. Requires gRPC ≥ 1.57 for `LocalServerCredentials`.
