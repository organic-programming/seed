# RUST_TASK002 — grpcclient Module (WebSocket + mem dial)

Status: Next implementation milestone

## Context

go-holons has `pkg/grpcclient/` with 3 dialing strategies:
- `client.go` — standard TCP dial
- `ws.go` — gRPC over WebSocket (HTTP upgrade → binary frames → tonic)
- `mem.go` — in-process gRPC channel (for testing)

rust-holons has no equivalent. The `connect` module dials TCP directly
via `tonic::transport::Endpoint`. There is no WebSocket or in-memory dial path.

TASK01 is already complete, so this is the next planned build phase.
Do not mix HolonRPC or server-side WebSocket serve work into this task.

## Goal

Create `src/grpcclient.rs` with `dial_uri(uri)` that dispatches to
the right dialer based on scheme:

```rust
pub async fn dial_uri(uri: &str) -> Result<Channel>   // tcp://, unix://, ws://, mem://
pub async fn dial_websocket(uri: &str) -> Result<Channel>
pub async fn dial_mem(name: &str) -> Result<Channel>
```

Add the new module publicly from `src/lib.rs`, then route direct dialing
through it from `src/connect.rs`.

## How WebSocket dial works (from Go)

1. Open a WebSocket connection to `ws://host:port/grpc`
2. Each gRPC frame becomes a WebSocket binary message
3. The remote end does the reverse (ws → gRPC)
4. Return a `tonic::transport::Channel` to the caller

Implementation: create a `WsStream` adapter (`AsyncRead + AsyncWrite`)
backed by `tokio-tungstenite`, then pass it to `Endpoint::connect_with_connector`.

## Files to create

### `src/grpcclient.rs` (new)

### `src/lib.rs` — add `pub mod grpcclient;`

### `src/connect.rs` — wire `grpcclient::dial_uri` into slug resolution

Currently `connect_direct()` builds a `Channel` via `Endpoint`. Replace with
`grpcclient::dial_uri()` to support all transport schemes.

## Checklist

- [ ] Add `tokio-tungstenite` to `Cargo.toml`
- [ ] Implement `dial_uri` (scheme dispatch: tcp, unix, ws, wss, mem)
- [ ] Implement `dial_websocket` (WS adapter)
- [ ] Implement `dial_mem` (in-process channel)
- [ ] Export `pub mod grpcclient;` from `src/lib.rs`
- [ ] Wire `grpcclient::dial_uri` into `connect.rs`
- [ ] Add test: TCP dial round-trip
- [ ] Add test: Unix dial round-trip
- [ ] Add test: WebSocket dial against Go echo-server
- [ ] Add test: mem dial in-process
- [ ] Add regression: `connect(slug)` routes non-TCP schemes through `grpcclient::dial_uri`

## Dependencies

- None — can run in parallel with RUST_TASK001

## Out of scope

- HolonRPC client or server work
- Server-side `ws://` serving in `serve.rs`
- Rust daemon migration
