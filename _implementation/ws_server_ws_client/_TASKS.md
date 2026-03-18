# OP v0.5.1 Design Tasks — SDK Transport Full Parity

> [!IMPORTANT]
> **This milestone runs after v0.5 (REST+SSE delivery).**
> It uses the SDK source code as its only source of truth — not README.md.
> Goal: every SDK reaches its *actual achievable maximum* transport support,
> then all documentation is rewritten to accurately reflect that state.

> [!CAUTION]
> Do NOT rewrite SDK architecture. Extend transport/serve layers only.

## Execution Strategy

TASK01–05 are SDK-level and can run in parallel.
TASK06 (docs) gates on all others being complete.

## Tasks

| # | File | Summary | Depends on |
|---|---|---|---|
| 01 | [TASK01](./grace-op_v0.5.1_TASK01_swift_transport.md) | Swift: add `unix`, `mem`, ws client to serve | v0.5 |
| 02 | [TASK02](./grace-op_v0.5.1_TASK02_rust_mem.md) | Rust: add `mem` to `serve_router!` | v0.5 |
| 03 | [TASK03](./grace-op_v0.5.1_TASK03_cpp_mem.md) | C++: add `mem`; document stdio POSIX-only | v0.5 |
| 04 | [TASK04](./grace-op_v0.5.1_TASK04_kotlin_java_serve.md) | Kotlin + Java: wire stdio/unix/mem dispatch in `Serve` | v0.5 |
| 05 | [TASK05](./grace-op_v0.5.1_TASK05_go_ws_client.md) | Go: add grpc-ws dial in `connect()`; validate SSE+REST from v0.5 | v0.5 |
| 06 | [TASK06](./grace-op_v0.5.1_TASK06_docs.md) | Rewrite sdk/README.md, SDK_GUIDE.md, all per-SDK READMEs | TASK01–05 |

## Dependency Graph

```
v0.5 → TASK01 ─┐
     → TASK02 ─┤
     → TASK03 ─┼→ TASK06
     → TASK04 ─┤
     → TASK05 ─┘
```
