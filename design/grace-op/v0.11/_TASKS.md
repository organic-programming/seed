# Grace-OP v0.11 Design Tasks — Setup

## Tasks

| # | File | Summary | Depends on | Status |
|---|---|---|---|---|
| 01 | [TASK01](./grace-op_v0.11_TASK01_setup_image_schema.md) | `setup.yaml` image schema + parser + `include` composition | — | — |
| 02 | [TASK02](./grace-op_v0.11_TASK02_dependency_resolution.md) | Dependency resolution engine (graph from image + holon manifests) | TASK01, v0.8 | — |
| 03 | [TASK03](./grace-op_v0.11_TASK03_execution_engine.md) | 6-phase `op setup` execution engine | TASK01, TASK02, v0.8, v0.9 | — |
| 04 | [TASK04](./grace-op_v0.11_TASK04_requires_sources.md) | `requires.sources` in `holon.yaml` (clone + build) | TASK01 | — |

Spec drafts: [output/](./output/) (📝 files for review)

Design document: [DESIGN_setup.md](./DESIGN_setup.md)
