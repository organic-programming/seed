# Grace-OP v0.8 Design Tasks — Release Pipeline

## Tasks

| # | File | Summary | Depends on |
|---|---|---|---|
| 01 | [TASK01](./grace-op_v0.8_TASK01_artifact_naming.md) | Artifact naming convention + `build.publish` schema | v0.7 TASK01 |
| 02 | [TASK02](./grace-op_v0.8_TASK02_op_publish.md) | `op publish` command (build all platforms + upload) | TASK01, v0.7 |
| 03 | [TASK03](./grace-op_v0.8_TASK03_install_resolution.md) | `op install` platform resolution + source fallback | TASK01, TASK04 |
| 04 | [TASK04](./grace-op_v0.8_TASK04_holon_registry.md) | Holon registry (artifact storage + index service) | TASK01 |
| 05 | [TASK05](./grace-op_v0.8_TASK05_ci_templates.md) | CI build matrix templates (GitHub Actions) | TASK02, TASK04 |
| 06 | [TASK06](./grace-op_v0.8_TASK06_signing.md) | Artifact signing + verification | TASK02, TASK03, TASK04 |
| 07 | [TASK07](./grace-op_v0.8_TASK07_docs.md) | Documentation (spec updates → output/ for review) | TASK01–06 |

Design document: [DESIGN_release_pipeline.md](./DESIGN_release_pipeline.md)
