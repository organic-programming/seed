# Grace-OP v0.9 Design Tasks — Mesh

## Tasks

| # | File | Summary | Depends on | Status |
|---|---|---|---|---|
| 01 | [TASK01](./grace-op_v0.9_TASK01_mesh_foundation.md) | CA generation, cert signing, `mesh.yaml`, `op mesh list` | — | — |
| 02 | [TASK02](./grace-op_v0.9_TASK02_mesh_deployment.md) | SSH deploy (`--deploy`), `op mesh remove`, cert renewal | TASK01 | — |
| 03 | [TASK03](./grace-op_v0.9_TASK03_mesh_introspection.md) | `op mesh status` + `op mesh describe` (mTLS health check) | TASK01, TASK02 | — |
| 04 | [TASK04](./grace-op_v0.9_TASK04_sdk_mesh_integration.md) | SDK discover/connect/serve with mTLS auto-detection | TASK01, TASK02, v0.6 | — |

Spec drafts: [output/](./output/) (📝 files for review)

Design document: [DESIGN_mesh.md](./DESIGN_mesh.md)
