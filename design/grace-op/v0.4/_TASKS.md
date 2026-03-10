# OP v0.4 Design Tasks — Recipe Ecosystem

## Tasks

| # | File | Summary | Depends on | Status |
|---|---|---|---|---|
| 01 | [TASK01](./grace-op_v0.4_TASK01_dry_daemons.md) | Extract 8 DRY daemons | — | — |
| 02 | [TASK02](./grace-op_v0.4_TASK02_dry_hostui.md) | Extract 6 DRY HostUIs | — | — |
| 03 | [TASK03](./grace-op_v0.4_TASK03_assembly_manifests.md) | Create 48 assembly manifests | TASK01, TASK02 | — |
| 04 | [TASK04](./grace-op_v0.4_TASK04_remove_submodules.md) | Remove 12 submodules, archive repos | TASK03 | — |
| 05 | [TASK05](./grace-op_v0.4_TASK05_testmatrix.md) | Combinatorial testing (Go testmatrix) | TASK03, TASK06 | — |
| 06 | [TASK06](./grace-op_v0.4_TASK06_composition_recipes.md) | 3 patterns × 11 orchestrator languages | — | — |

Design documents:
- [DESIGN_recipe_ecosystem.md](./DESIGN_recipe_ecosystem.md) — architecture, patterns, rationale
- [DESIGN_recipe_monorepo.md](./DESIGN_recipe_monorepo.md) — proto contracts, assembly matrix
