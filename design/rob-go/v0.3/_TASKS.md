# Rob-Go v0.3 Design Tasks — CGO Toolchain

## Tasks

### Stage 1 — Passthrough

| # | File | Summary | Depends on | Status |
|---|---|---|---|---|
| 01 | [TASK01](./rob-go_v0.3_TASK01_cgo_passthrough.md) | CGO passthrough allowlist in `HermeticEnv` | v0.1 TASK02 | — |
| 02 | [TASK02](./rob-go_v0.3_TASK02_manifest_cgo.md) | Manifest `cgo` declaration parsing | TASK01 | — |

### Stage 2 — Embedded C Toolchain

| # | File | Summary | Depends on | Status |
|---|---|---|---|---|
| 03 | [TASK03](./rob-go_v0.3_TASK03_embedded_cc.md) | Provision embedded C compiler (Zig) | — | — |
| 04 | [TASK04](./rob-go_v0.3_TASK04_wire_embedded_cc.md) | Wire embedded CC into hermetic env | TASK01, TASK03 | — |

Design document: [DESIGN_cgo_toolchain.md](./DESIGN_cgo_toolchain.md)
