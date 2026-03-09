# SDK Roadmap — Autonomous Holon Connectivity

`op` is a facility, not a dependency. Every SDK must provide the
primitives for holons to discover and connect to each other
directly — without `op` as an intermediary.

## Principle

A holon built with `sdk/rust-holons` can serve gRPC, discover
other holons, and connect to a holon built with `sdk/go-holons` —
all by itself. The SDK is the native runtime. `op` orchestrates,
but is never required.

## SDK Module Architecture

Every SDK must expose these 5 modules:

```
sdk/<lang>-holons
├── serve        SERVER: parse flags, listen, run gRPC server, shutdown
├── transport    SHARED: URI parsing (tcp, unix, stdio, mem, ws, wss), listen + dial
├── identity     SHARED: read/write holon.yaml, parse manifest
├── discover     CLIENT: scan filesystem for holon.yaml, resolve by slug/UUID/path
└── connect      CLIENT: resolve holon → find/start binary → dial gRPC channel
```

### Current state per SDK

Statuses below reflect current implementation plus verification in this
checkout. For `js-web-holons`, `discover` and `connect` are the
browser-adapted variants (`discoverFromManifest(...)` and direct
`host:port` connect only).

| SDK | serve | transport | identity | discover | connect |
|---|---|---|---|---|---|
| `go-holons` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `rust-holons` | ✅ | ✅ | ✅ | ✅ | ❓ |
| `swift-holons` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `dart-holons` | ✅ | ✅ | ✅ | ✅ | ❓ |
| `js-holons` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `js-web-holons` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `kotlin-holons` | ✅ | ✅ | ✅ | ✅ | ❓ |
| `java-holons` | ✅ | ✅ | ✅ | ✅ | ❓ |
| `csharp-holons` | ✅ | ✅ | ✅ | ✅ | ❓ |
| `cpp-holons` | ✅ | ✅ | ✅ | ✅ | ❓ |
| `c-holons` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `python-holons` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `ruby-holons` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `objc-holons` | ✅ | ✅ | ✅ | ✅ | ❓ |

Detailed plans: see `TODO_DISCOVER.md` and `TODO_CONNECT.md`.

## Execution order

See detailed per-file plans:

1. `TODO_DISCOVER.md` — implement `discover` module in each SDK
2. `TODO_CONNECT.md` — implement `connect` module in each SDK
   (depends on discover)
3. `TODO_MIGRATE_RECIPES.md` — migrate recipes+examples to use
   SDK client primitives

## Testing strategy

Each SDK has existing tests (Go `go test`, Rust `cargo test`,
Python `pytest`, Ruby `rake test`, etc.). New modules must follow
the same test patterns. Cross-language integration tests use the
Go `cmd/echo-server-go` and `cmd/echo-client-go` test fixtures
already present in most SDKs.
