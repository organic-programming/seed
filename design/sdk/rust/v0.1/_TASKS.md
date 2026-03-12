# rust-holons SDK v0.1 — Tasks

> Bring rust-holons to full parity with go-holons: auto HolonMeta,
> grpcclient (ws+mem), HolonRPC client+server, ws serve, daemon migration.

## Execution Strategy

TASK01 and TASK02 are independent. TASK03–05 are linear.
TASK06 depends on all prior tasks.

## Tasks

| # | File | Summary | Depends on |
|---|------|---------|------------|
| | | **— SDK Core —** | |
| 01 | [TASK01](./RUST_TASK001_auto_holonmeta.md) | Auto HolonMeta.Describe in serve runner | — |
| 02 | [TASK02](./RUST_TASK002_grpcclient.md) | grpcclient: ws + mem dial | — |
| 03 | [TASK03](./RUST_TASK003_holonrpc_client.md) | HolonRPC client (JSON-RPC over WS) | TASK02 |
| 04 | [TASK04](./RUST_TASK004_holonrpc_server.md) | HolonRPC server + routing + fanout | TASK03 |
| 05 | [TASK05](./RUST_TASK005_ws_grpc_bridge.md) | WebSocket-to-gRPC bridge in serve.rs | TASK02, TASK04 |
| | | **— Migration —** | |
| 06 | [TASK06](./RUST_TASK006_daemon_migration.md) | Migrate 6 Rust daemons to SDK | TASK01–05 |

## Dependency Graph

```
TASK01 ──────────────────────────→ TASK06
TASK02 → TASK03 → TASK04 → TASK05 → TASK06
```
