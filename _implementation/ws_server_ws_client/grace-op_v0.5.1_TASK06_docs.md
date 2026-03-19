# TASK06 — Rewrite SDK Transport Documentation

## Summary

After TASK01–05 close the achievable transport gaps, this task rewrites all
SDK documentation to accurately reflect what every SDK actually supports —
replacing the aspirational blanket ✅ table in `sdk/README.md` with
a source-verified matrix that distinguishes serve, ws client, ws server, and
SSE+REST.

This is the only task that touches documentation. No SDK source changes.

## Files to Update

### `sdk/README.md` — Transport Surface section (lines 51–73)

Replace the current table (all ✅) with the accurate post-v0.5.1 state:

| SDK | tcp | unix | stdio | ws server | ws client | SSE+REST |
|-----|:---:|:----:|:-----:|:---------:|:---------:|:--------:|
| `go-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `js-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | — |
| `kotlin-holons` | ✅ | ✅ | ✅ | 🚫 | ❌ | ❌ |
| `java-holons` | ✅ | ✅ | ✅ | 🚫 | ❌ | ❌ |
| `ruby-holons` | ✅ | ✅ | ✅ | 🚫 | ❌ | ❌ |
| `csharp-holons` | ✅ | ✅ | ✅ | 🚫 | ❌ | ❌ |
| `python-holons` | ✅ | ✅ | ✅¹ | 🚫 | ❌ | ❌ |
| `dart-holons` | ✅ | ✅ | ✅ | 🚫 | ❌ | ❌ |
| `rust-holons` | ✅ | ✅ | ✅ | 🚫 | ❌ | ❌ |
| `cpp-holons` | ✅ | ✅ | ✅² | 🚫 | ❌ | ❌ |
| `swift-holons` | ✅ | ✅ | ✅ | 🚫 | ✅ | ❌ |
| `js-web-holons` | — | — | — | — | ✅ | ✅ client |
| `c-holons` | ✅ | — | — | — | — | — |

¹ Python stdio via Unix bridge (`_StdioServeBridge`).
² C++ stdio is POSIX-only (`HOLONS_HAS_GRPC_FD`).

Legend:
- ✅ = fully implemented in serve layer
- 🚫 = gRPC library constraint (ws server not exposed)
- ❌ = not implemented
- — = not applicable

Also add a callout box:
> **ws client vs ws server**: A ✅ in *ws client* means the SDK's `connect()` can dial a `ws://` daemon. A ✅ in *ws server* means the SDK itself can serve gRPC over WebSocket. Most SDKs need only be ws clients. ws server requires gRPC library support — only Go and Node provide this.

### `sdk/SDK_GUIDE.md` — new "Transport Matrix" section

Add after the architecture overview, before API examples:

```markdown
## Transport Matrix

[copy of the table above]

### How to use

- Pass `--listen unix:///tmp/holons.sock` to a daemon to use a Unix socket.
- Pass `--listen stdio://` for pipe-based embedding (SwiftUI, Qt subprocess).
- Pass `--listen ws://:9091` (Go/Node daemons only) for browser-accessible daemons.
```

### Per-SDK READMEs

Each SDK's own `README.md` must include a "Supported transports" section with its own row from the table, and a short note explaining any constraints (e.g., "stdio requires POSIX" for C++, "ws server not supported by grpcio" for Python).

Files to update:
- `sdk/swift-holons/README.md`
- `sdk/rust-holons/README.md`
- `sdk/cpp-holons/README.md`
- `sdk/kotlin-holons/README.md`
- `sdk/java-holons/README.md`
- `sdk/dart-holons/README.md`
- `sdk/python-holons/README.md`
- `sdk/ruby-holons/README.md`
- `sdk/csharp-holons/README.md`
- `sdk/js-holons/README.md`
- `sdk/js-web-holons/README.md`
- `sdk/c-holons/README.md`

## Acceptance Criteria

- [ ] `sdk/README.md` transport table matches the post-v0.5.1 source-verified state
- [ ] `sdk/SDK_GUIDE.md` has a "Transport Matrix" section
- [ ] Every per-SDK README has a "Supported transports" section
- [ ] No SDK README claims a transport that the code does not deliver
- [ ] All `🚫 library` entries include a one-line explanation of which library is the constraint

## Dependencies

TASK01 through TASK05 must be complete before this task is finalized, so the table reflects the actual post-v0.5.1 state — not the pre-task state.
