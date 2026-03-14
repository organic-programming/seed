# SDK Transport Parity

> Source of truth: SDK source code (not README.md).
> This document is the reference for v0.5.1 implementation and documentation.
> It was written on 2026-03-12 after reading every SDK transport/serve source file directly.

---

## Two distinct levels

| Level | Meaning |
|---|---|
| **Transport layer** (`transport.*`) | URI parsing + raw socket bind |
| **Serve layer** (`serve.*`) | Full gRPC lifecycle: bind → register → signal → stop |

README.md conflates both. A ✅ for `ws` in most SDKs means the URI *parses* — not that `serve.run(ws://)` works.

## ws client ≠ ws server

| Role | Meaning | Who needs it |
|---|---|---|
| **ws server** | Listen on `ws://`, accept WebSocket gRPC connections | Daemons exposed to browsers |
| **ws client** | Dial out to a `ws://` endpoint | HostUIs reaching a remote daemon |

**Policy:** Go must be a ws server (already is). Node already is. For all other SDKs, ws server is a gRPC library constraint — document it as `🚫 library`, not a code gap.

---

## Current State (source-verified 2026-03-12)

### Legend
- ✅ = fully implemented in serve layer
- ⚠️ = transport layer parses, serve layer rejects at runtime
- ❌ = absent
- 🚫 = gRPC library constraint, cannot be fixed in SDK code

**SSE+REST** = HTTP/1.1 REST + EventSource gateway alongside gRPC (no proxy needed for browsers).

| SDK | tcp | unix | stdio | mem | ws server | ws client | SSE+REST |
|-----|:---:|:----:|:-----:|:---:|:---------:|:---------:|:--------:|
| `go-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠️ todo | ❌ |
| `js-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ |
| `kotlin-holons` | ✅ | ✅ | ✅ | ✅ | ⚠️ parse | ❌ | ❌ |
| `java-holons` | ✅ | ✅ | ✅ | ✅ | ⚠️ parse | ❌ | ❌ |
| `ruby-holons` | ✅ | ✅ | ✅ | ✅ | ⚠️ parse | ❌ | ❌ |
| `csharp-holons` | ✅ | ✅ | ✅ | ✅ | ⚠️ parse | ❌ | ❌ |
| `python-holons` | ✅ | ✅ | ✅¹ | ✅ | ❌ | ❌ | ❌ |
| `dart-holons` | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
| `rust-holons` | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| `cpp-holons` | ✅ | ✅ | ✅² | ❌ | ❌ | ❌ | ❌ |
| `swift-holons` | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| `js-web-holons` | — | — | — | — | — | ✅ | ✅ client |
| `c-holons` | — | — | — | — | — | — | — |

¹ Python stdio via Unix socket bridge (`_StdioServeBridge`).  
² C++ stdio requires `HOLONS_HAS_GRPC_FD` (POSIX only, not Windows).

**Key gaps by SDK:**
- **`swift-holons`**: `Serve.startWithOptions` has an explicit `guard parsed.scheme == "tcp"` — all non-TCP transports throw `runtimeUnsupported`. SwiftNIO can handle unix/stdio/mem, just not wired.
- **`rust-holons`**: `serve_router!` returns explicit errors for `mem` and `ws`. Tonic has no ws server.
- **`cpp-holons`**: `serve::start()` throws for mem/ws.
- **`kotlin-holons` / `java-holons`**: `Transport.listen` builds a `WS` struct but `Serve` never dispatches it. All other transports parse fine, dispatch just needs to be wired.
- **`go-holons`**: ws dial not yet added to `connect()` (transport parses ws URLs, server-side ws works via `nhooyr.io/websocket`).

---

## Target State — end of v0.5.1

`v0.5` delivers SSE+REST for Go. `v0.5.1` adds remaining transport gaps and rewrites all docs.

| SDK | tcp | unix | stdio | mem | ws server | ws client | SSE+REST |
|-----|:---:|:----:|:-----:|:---:|:---------:|:---------:|:--------:|
| `go-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `js-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | — client |
| `kotlin-holons` | ✅ | ✅ | ✅ | ✅ | 🚫 | ❌ | ❌ |
| `java-holons` | ✅ | ✅ | ✅ | ✅ | 🚫 | ❌ | ❌ |
| `ruby-holons` | ✅ | ✅ | ✅ | ✅ | 🚫 | ❌ | ❌ |
| `csharp-holons` | ✅ | ✅ | ✅ | ✅ | 🚫 | ❌ | ❌ |
| `python-holons` | ✅ | ✅ | ✅¹ | ✅ | 🚫 | ❌ | ❌ |
| `dart-holons` | ✅ | ✅ | ✅ | ✅ | 🚫 | ❌ | ❌ |
| `rust-holons` | ✅ | ✅ | ✅ | ✅ | 🚫 | ❌ | ❌ |
| `cpp-holons` | ✅ | ✅ | ✅² | ✅ | 🚫 | ❌ | ❌ |
| `swift-holons` | ✅ | ✅ | ✅ | ✅ | 🚫 | ❌ | ❌ |
| `js-web-holons` | — | — | — | — | — | ✅ | ✅ client |
| `c-holons` | ✅ | — | — | — | — | — | — |

> [!NOTE]
> **🚫 library** = the gRPC library (tonic, grpc-java, grpc-dart, etc.) does not expose a WebSocket server transport. This is a library-level constraint — not a SDK code gap. Only Go and Node can be ws servers in this ecosystem.

---

## v0.5.1 Task Scope

| # | SDK(s) | Work |
|---|--------|------|
| TASK01 | `swift-holons` | Remove tcp-only guard in `Serve.startWithOptions`; add unix, stdio, mem dispatch via SwiftNIO |
| TASK02 | `rust-holons` | Add `mem` to `serve_router!` using tokio channel pair (same pattern as Go `bufconn`) |
| TASK03 | `cpp-holons` | Add `mem` via gRPC++ in-process channel if feasible; document stdio as POSIX-only |
| TASK04 | `kotlin-holons` `java-holons` | Wire `Serve` to dispatch stdio/unix/mem using the already-correct `Transport.listen` |
| TASK05 | `go-holons` | Add ws dial in `connect()` (transport parses ws:// already); validate v0.5 SSE+REST end-to-end |
| TASK06 | All SDKs | Rewrite every SDK README, `sdk/README.md`, `sdk/SDK_GUIDE.md` with accurate transport matrices |

**ws server and SSE+REST are Go-only targets.** All other SDKs cap at `🚫 library` for ws server and `❌` for SSE+REST — that is the correct documented state, not an open issue.
