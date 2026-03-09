# RUST_TASK005 — WebSocket-to-gRPC Bridge in serve.rs

## Context

Depends on RUST_TASK002 (grpcclient WsStream adapter) and RUST_TASK004 (server).

Currently `serve::run()` line 70–74 rejects `ws://` URIs:
```rust
Listener::Ws(_) => {
    return Err(boxed_err("serve::run() does not support ws://..."));
}
```

After this task, `serve::run("ws://...")` works — a Rust holon can
serve standard gRPC over WebSocket, matching go-holons.

## Approach

Reuse the `WsStream` adapter from RUST_TASK002. For each accepted
TCP connection:
1. Perform HTTP upgrade to WebSocket
2. Wrap in `WsStream` (implements `AsyncRead + AsyncWrite + Connected`)
3. Feed to `tonic::transport::Server::serve_with_incoming()`

## Files to modify

### `src/serve.rs`

Replace the `Listener::Ws(_) => Err(...)` arm:

```rust
Listener::Ws(listener) => {
    let actual_uri = bound_ws_uri(listen_uri, &listener)?;
    announce_bound_uri(&actual_uri, options);
    serve_ws(router, listener).await?;
}
```

### `src/grpcclient.rs` — export `WsStream` for reuse

The `WsStream` adapter created in RUST_TASK002 for client-side dial
is generic enough to reuse server-side.

## Checklist

- [ ] Implement `serve_ws()` (accept loop + HTTP upgrade + WsStream)
- [ ] Implement `bound_ws_uri()` (like `bound_tcp_uri` but ws:// scheme)
- [ ] Replace `Listener::Ws` error arm in serve macro
- [ ] Also lift `Listener::Mem` if feasible (lower priority)
- [ ] Add integration test: Rust gRPC server over ws:// ↔ Go client
- [ ] Add integration test: Rust gRPC server over ws:// ↔ Rust client
- [ ] Verify `serve::run("ws://...")` end-to-end

## Dependencies

- RUST_TASK002 (WsStream adapter)
- RUST_TASK004 (Holon-RPC for end-to-end testing)

## After this task

rust-holons reaches **100% parity with go-holons**:

| Module | Status |
|---|---|
| serve (runner + auto HolonMeta + ws://) | ✅ |
| transport | ✅ |
| identity | ✅ |
| discover | ✅ |
| connect | ✅ |
| describe | ✅ |
| grpcclient (tcp + ws + mem dial) | ✅ |
| holonrpc (client + server + routing) | ✅ |
