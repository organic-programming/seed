# rust-holons SDK v0.1 — Tasks

> Bring rust-holons to full parity with go-holons: auto HolonMeta,
> grpcclient (ws+mem), HolonRPC client+server, ws serve, daemon migration.

## Execution Strategy

TASK01 is complete. TASK02 is the next implementation milestone.
TASK03–05 remain future work gated on TASK02. TASK06 stays deferred
until the transport and HolonRPC surface is intentionally accepted.

## Tasks

| # | File | Summary | Status | Depends on |
|---|------|---------|--------|------------|
| | | **— SDK Core —** | | |
| 01 | [TASK01](./RUST_TASK001_auto_holonmeta.md) | Auto HolonMeta.Describe in serve runner | Complete | — |
| 02 | [TASK02](./RUST_TASK002_grpcclient.md) | grpcclient: ws + mem dial | Next | — |
| 03 | [TASK03](./RUST_TASK003_holonrpc_client.md) | HolonRPC client (JSON-RPC over WS) | Deferred | TASK02 |
| 04 | [TASK04](./RUST_TASK004_holonrpc_server.md) | HolonRPC server + routing + fanout | Deferred | TASK03 |
| 05 | [TASK05](./RUST_TASK005_ws_grpc_bridge.md) | WebSocket-to-gRPC bridge in serve.rs | Deferred | TASK02, TASK04 |
| | | **— Migration —** | | |
| 06 | [TASK06](./RUST_TASK006_daemon_migration.md) | Migrate 6 Rust daemons to SDK | Deferred | TASK01–05 |

## Dependency Graph

```
TASK01 ──────────────────────────→ TASK06
TASK02 → TASK03 → TASK04 → TASK05 → TASK06
```

## Current Rebaseline

- TASK01 is closed out by the shipped Rust SDK:
  `serve::run_single_with_options` and `serve::run_with_options`
  auto-register `HolonMeta.Describe` from the current holon root.
- The closeout is backed by the Rust SDK regression that exercises
  `HolonMeta.Describe` through the serve runner.
- The change also resolved the real SwiftUI + Rust assembly startup
  path, which now passes manual verification.
