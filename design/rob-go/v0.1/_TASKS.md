# Rob-Go v0.1 Design Tasks — Toolchain Holon

## Tasks

| # | File | Summary | Depends on | Status |
|---|---|---|---|---|
| 01 | [TASK01](./rob-go_v0.1_TASK01_toolchain_provisioning.md) | Embed Go toolchain (download, verify, cache) | — | — |
| 02 | [TASK02](./rob-go_v0.1_TASK02_hermetic_env.md) | Hermetic environment construction | TASK01 | — |
| 03 | [TASK03](./rob-go_v0.1_TASK03_exec_integration.md) | Wire exec mode to embedded toolchain + hermetic env | TASK02 | — |
| 04 | [TASK04](./rob-go_v0.1_TASK04_library_env.md) | Wire library mode (`packages.Load`) to hermetic env | TASK02 | — |
| 05 | [TASK05](./rob-go_v0.1_TASK05_manifest_kind.md) | Update `holon.yaml` to `kind: toolchain` | — | — |
| 06 | [TASK06](./rob-go_v0.1_TASK06_grpc_reflection.md) | Enable gRPC reflection | — | — |

Design document: [DESIGN_hybrid_bootstrap.md](./DESIGN_hybrid_bootstrap.md)
