# Grace-OP v0.6 Design Tasks — REST + SSE Transport

Go SDK (TASK01) is the **reference implementation** — it establishes
the wire format, URL routing, SSE event structure, and `protojson`
encoding. All other SDK tasks port this reference and verify
interop with `op` (TASK02).

## Tasks

### Reference + CLI

| # | File | Summary | Depends on | Status |
|---|---|---|---|---|
| 01 | [TASK01](./grace-op_v0.6_TASK01_rest_sse_go.md) | REST + SSE in `go-holons` (reference, **client + server**) | — | — |
| 02 | [TASK02](./grace-op_v0.6_TASK02_rest_sse_grace_op.md) | REST + SSE in `op serve` / `op dial` | TASK01 | — |

### Daemon SDKs — client + server

| # | File | Summary | Depends on | Status |
|---|---|---|---|---|
| 03 | [TASK03](./grace-op_v0.6_TASK03_rest_sse_rust.md) | REST + SSE in `rust-holons` | TASK01, TASK02 | — |
| 07 | [TASK07](./grace-op_v0.6_TASK07_rest_sse_dotnet.md) | REST + SSE in `dotnet-holons` | TASK01, TASK02 | — |
| 08 | [TASK08](./grace-op_v0.6_TASK08_rest_sse_node.md) | REST + SSE in `node-holons` | TASK01, TASK02 | — |
| 09 | [TASK09](./grace-op_v0.6_TASK09_rest_sse_cpp.md) | REST + SSE in `cpp-holons` | TASK01, TASK02 | — |
| 10 | [TASK10](./grace-op_v0.6_TASK10_rest_sse_python.md) | REST + SSE in `python-holons` | TASK01, TASK02 | — |

### Frontend SDKs — client only

| # | File | Summary | Depends on | Status |
|---|---|---|---|---|
| 04 | [TASK04](./grace-op_v0.6_TASK04_rest_sse_dart.md) | REST + SSE **client** in `dart-holons` | TASK01, TASK02 | — |
| 05 | [TASK05](./grace-op_v0.6_TASK05_rest_sse_swift.md) | REST + SSE **client** in `swift-holons` | TASK01, TASK02 | — |
| 06 | [TASK06](./grace-op_v0.6_TASK06_rest_sse_kotlin.md) | REST + SSE **client** in `kotlin-holons` | TASK01, TASK02 | — |

Design document: [DESIGN_transport_rest_sse.md](./DESIGN_transport_rest_sse.md)

