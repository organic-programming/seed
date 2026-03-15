# TASK02 — Rust: Add `mem://` to `serve_router!`

## Summary

`sdk/rust-holons/src/serve.rs` currently rejects `mem://` in `serve_router!`:

```rust
Listener::Mem(_) => {
    return Err(boxed_err(
        "serve::run() does not support mem://; use a custom server loop",
    ));
}
```

Implement `mem://` using a tokio in-process connection pair — the same pattern
as Go's `google.golang.org/grpc/test/bufconn`.

## Target

| Transport | Before | After |
|-----------|:------:|:-----:|
| `tcp://` | ✅ | ✅ |
| `unix://` | ✅ | ✅ |
| `stdio://` | ✅ | ✅ |
| `mem://` | ❌ | ✅ |
| ws server | ❌ | 🚫 library |

## Implementation

### `sdk/rust-holons/src/transport.rs`

The `Listener::Mem` variant already exists. Add a `MemListener` struct that holds
a `tokio::io::DuplexStream` pair created with `tokio::io::duplex(capacity)`.
Expose a `dial()` method returning the client half, matching Go's `bufconn` API.

```rust
pub struct MemListener {
    capacity: usize,
    tx: tokio::sync::mpsc::Sender<DuplexStream>,
    rx: tokio::sync::Mutex<tokio::sync::mpsc::Receiver<DuplexStream>>,
}
```

### `sdk/rust-holons/src/serve.rs`

In `serve_router!`, replace the `Listener::Mem` arm:

```rust
Listener::Mem(listener) => {
    announce_bound_uri("mem://", options);
    let mut mem_listener = listener;
    loop {
        let stream = mem_listener.accept().await?;
        let svc = router.clone();
        tokio::spawn(async move {
            svc.serve_with_incoming(
                tokio_stream::once(Ok::<_, std::io::Error>(stream))
            ).await
        });
    }
}
```

## Acceptance Criteria

- [ ] `cargo test` passes, including:
  - `mem_round_trip` — in-process server + client via `MemListener::dial()`
  - `mem_describe` — `HolonMeta/Describe` reachable over `mem://`
- [ ] `cargo test --all` still passes (no regression in tcp/unix/stdio)
- [ ] `MemListener::dial()` is exported in `holons::transport`

## Dependencies

`sdk/rust-holons` only.
