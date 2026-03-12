# RUST_TASK003 — Holon-RPC Client

## Context

Depends on RUST_TASK002 (grpcclient provides WebSocket dial infrastructure).

Every SDK except rust-holons has a `HolonRPCClient` — JSON-RPC 2.0
over WebSocket for lightweight scripting and browser interop.

## Goal

Implement `HolonRPCClient` in rust-holons.

## API

```rust
pub struct HolonRPCClient { ... }

impl HolonRPCClient {
    pub async fn connect(url: &str) -> Result<Self>
    pub async fn call(&self, method: &str, params: serde_json::Value) -> Result<serde_json::Value>
    pub fn register(&self, method: &str, handler: Handler)
    pub async fn close(&self) -> Result<()>
}

pub type Handler = Box<dyn Fn(serde_json::Value) -> BoxFuture<serde_json::Value> + Send + Sync>;
```

**Protocol**: JSON-RPC 2.0 over WebSocket, matching PROTOCOL.md §3.

## Reference

- `go-holons/pkg/holonrpc/client.go`
- `kotlin-holons/.../HolonRPCClient.kt`
- `python-holons/holons/holonrpc.py`

## Files

### `src/holonrpc.rs` (new)

### `src/lib.rs` — add `pub mod holonrpc;`

## Checklist

- [ ] Implement `connect` (WebSocket dial via `tokio-tungstenite`)
- [ ] Implement `call` (JSON-RPC request/response with ID matching)
- [ ] Implement `register` (bidirectional server-initiated call handling)
- [ ] Implement reconnect + heartbeat (match Go/Python behavior)
- [ ] Add test: echo round-trip against Go Holon-RPC server
- [ ] Add test: register handler, verify server-initiated calls
- [ ] Add test: reconnect after connection drop

## Dependencies

- RUST_TASK002 (WebSocket infrastructure in grpcclient)
