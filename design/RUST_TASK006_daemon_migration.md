# RUST_TASK006 — Migrate Rust Daemons to SDK

## Context

Depends on RUST_TASK001–005 (SDK must reach Go parity first).

All 6 Rust recipe daemons use raw `tonic`/`prost` boilerplate
(~30 lines of duplicated flag parsing, URI handling, server setup).
`sdk/rust-holons` already provides `serve::parse_flags`,
`transport::listen`, and `identity` — but the daemons don't use them.

## Goal

Replace hand-rolled boilerplate in all 6 Rust daemons with SDK calls.

## Phase 1 — Prepare the SDK

- [ ] Verify `cargo build && cargo test` passes on `sdk/rust-holons`
- [ ] Add `tonic-reflection` and `tonic-web` re-exports to SDK
- [ ] Run tests again — zero regressions
- [ ] Migrate `examples/rust-hello-world` to use SDK (if not already)

## Phase 2 — Migrate each daemon

For each daemon:
1. Update `Cargo.toml`: remove direct tonic deps, add `holons` path dep
2. Replace hand-rolled `parse_listen_addr()` with `holons::serve::parse_flags()`
3. Replace manual server setup with `holons::transport::listen()`
4. Use SDK re-exports for reflection and tonic-web
5. Remove the local `parse_listen_addr()` function
6. Verify: `cargo build && cargo test`

### Order

- [ ] `rust-dart-holons` (reference — most tested)
- [ ] `rust-swift-holons`
- [ ] `rust-web-holons`
- [ ] `rust-kotlin-holons`
- [ ] `rust-qt-holons`
- [ ] `rust-dotnet-holons`

## Rules

- One daemon at a time: migrate, test, commit, push
- Do not change proto files or gRPC contracts
- Do not modify Go daemons (already on SDK)
- Do not migrate frontends (out of scope)
- Never break `sdk/rust-holons` tests or `examples/rust-hello-world`

## Dependencies

- RUST_TASK001–005 (SDK parity — holonrpc, grpcclient, serve ws://)
