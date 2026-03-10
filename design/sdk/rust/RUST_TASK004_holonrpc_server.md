# RUST_TASK004 — Holon-RPC Server + Routing

## Context

Depends on RUST_TASK003 (client needed for testing).

go-holons has 3 dedicated files: `server.go`, `routing.go`, `types.go`.
The server accepts WebSocket connections, routes JSON-RPC calls to
registered handlers, supports fanout to multiple clients, and handles
server-initiated calls.

## Goal

Implement `HolonRPCServer` in rust-holons.

## API

```rust
pub struct HolonRPCServer { ... }

impl HolonRPCServer {
    pub fn new(bind_url: &str) -> Self
    pub fn register(&mut self, method: &str, handler: Handler)
    pub async fn serve(&self) -> Result<()>
    pub fn bound_url(&self) -> &str
}
```

## Features to match Go

- **Method routing**: dispatch incoming JSON-RPC by method name
- **Fanout**: broadcast a call to all connected clients, aggregate responses
- **Server-initiated calls**: server can push JSON-RPC notifications to clients
- **Concurrent clients**: multiple simultaneous WebSocket connections

## Files

### `src/holonrpc.rs` — extend with server alongside client

## Checklist

- [ ] Implement `HolonRPCServer::new` (bind WebSocket listener)
- [ ] Implement HTTP upgrade handshake per connection
- [ ] Implement JSON-RPC method routing
- [ ] Implement server → client calls
- [ ] Implement fanout routing (broadcast + aggregate)
- [ ] Add test: echo server round-trip
- [ ] Add test: multi-client fanout
- [ ] Add interop test: Rust server ↔ Go client

## Dependencies

- RUST_TASK003 (Rust HolonRPC client for testing)
