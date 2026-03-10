# OP v0.4 Design Tasks — Recipe Ecosystem

## Tasks

| # | File | Summary | Depends on |
|---|---|---|---|
| 01 | [TASK01](./DESIGN_recipe_ecosystem.md) | Restructure recipes into `ui/` + `composition/` | — |
| 01.01 | [TASK01.01](./op_v0.4_TASK01.01_dry_daemons.md) | Extract 8 DRY daemons | — |
| 01.02 | [TASK01.02](./op_v0.4_TASK01.02_dry_hostui.md) | Extract 6 DRY HostUIs | — |
| 01.03 | [TASK01.03](./op_v0.4_TASK01.03_assembly_manifests.md) | Create 48 assembly manifests | 01.01, 01.02 |
| 01.04 | [TASK01.04](./op_v0.4_TASK01.04_remove_submodules.md) | Remove 12 submodules, archive repos | 01.03 |
| 01.05 | [TASK01.05](./op_v0.4_TASK01.05_testmatrix.md) | Combinatorial testing (Go testmatrix) | 01.03, 01.06 |
| 01.06 | [TASK01.06](./op_v0.4_TASK01.06_composition_recipes.md) | 3 patterns × 11 orchestrator languages | — |

Design documents:
- [DESIGN_recipe_ecosystem.md](./DESIGN_recipe_ecosystem.md) — architecture, patterns, rationale
- [DESIGN_recipe_monorepo.md](./DESIGN_recipe_monorepo.md) — proto contracts, assembly matrix
