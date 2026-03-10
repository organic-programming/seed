# OP v0.4 Design Tasks — Recipe Ecosystem

## Tasks

| # | File | Summary | Depends on |
|---|---|---|---|
| 10 | [TASK10](./TASK10_recipe_restructuring.md) | Restructure recipes into `ui/` + `composition/` | — |
| 10.01 | [TASK10.01](./TASK10.01_dry_daemons.md) | Extract 8 DRY daemons | — |
| 10.02 | [TASK10.02](./TASK10.02_dry_hostui.md) | Extract 6 DRY HostUIs | — |
| 10.03 | [TASK10.03](./TASK10.03_assembly_manifests.md) | Create 48 assembly manifests | 10.01, 10.02 |
| 10.04 | [TASK10.04](./TASK10.04_remove_submodules.md) | Remove 12 submodules, archive repos | 10.03 |
| 10.05 | [TASK10.05](./TASK10.05_testmatrix.md) | Combinatorial testing (Go testmatrix) | 10.03, 10.06 |
| 10.06 | [TASK10.06](./TASK10.06_composition_recipes.md) | 3 patterns × 11 orchestrator languages | — |

Reference material: [TASK10_NOTES.md](./TASK10_NOTES.md)

Design documents referenced by these tasks are listed in [INDEX.md](../INDEX.md).
