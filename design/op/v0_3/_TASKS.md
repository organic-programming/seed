# OP v0.3 Design Tasks

Design documents and implementation tasks for Grace OP (the `op` CLI)

## Tasks

| # | File | Summary | Depends on |
|---|---|---|---|
| 01 | [TASK01](./TASK01_install_no_build.md) | `op install --no-build` flag | — |
| 02 | [TASK02](./TASK02_composite_kind.md) | `kind: composite` + `artifacts.primary` | — |
| 03 | [TASK03](./TASK03_tier1_runners.md) | `cargo`, `swift-package`, `flutter` runners | — |
| 04 | [TASK04](./TASK04_tier2_runners.md) | `npm`, `gradle` runners | TASK03 |
| 05 | [TASK05](./TASK05_tier3_runners.md) | `dotnet`, `qt-cmake` runners | TASK03 |
| 06 | [TASK06](./TASK06_install_bundles.md) | Install .app/.exe bundles to `$OPBIN` | TASK02 |
| 07 | [TASK07](./TASK07_package_distribution.md) | Package manager distribution (Homebrew, WinGet, NPM) | — |
| 08 | [TASK08](./TASK08_mesh_documentation.md) | Document `op mesh` + transport security | — |
| 09 | [TASK09](./TASK09_setup_documentation.md) | Document `op setup` + `setup.yaml` spec | TASK08 |
| 10 | [TASK10](./TASK10_recipe_restructuring.md) | Restructure recipes into `ui/` + `composition/` | — |
| 10.01 | [TASK10.01](./TASK10.01_dry_daemons.md) | Extract 8 DRY daemons | — |
| 10.02 | [TASK10.02](./TASK10.02_dry_hostui.md) | Extract 6 DRY HostUIs | — |
| 10.03 | [TASK10.03](./TASK10.03_assembly_manifests.md) | Create 48 assembly manifests | 10.01, 10.02 |
| 10.04 | [TASK10.04](./TASK10.04_remove_submodules.md) | Remove 12 submodules, archive repos | 10.03 |
| 10.05 | [TASK10.05](./TASK10.05_testmatrix.md) | Combinatorial testing (Go testmatrix) | 10.03, 10.06 |
| 10.06 | [TASK10.06](./TASK10.06_composition_recipes.md) | 3 patterns × 11 orchestrator languages | — |
| 11 | [TASK11](./TASK11_build_configs.md) | Build configs (`--config` + `OP_CONFIG`) | — |
| 12 | [TASK12](./TASK12_mvs_resolution.md) | MVS transitive dependency resolution | — |
| 13 | [TASK13](./TASK13_sequences.md) | Sequences (`op do`) + MCP tool exposure | — |

Design documents referenced by these tasks are listed in [INDEX.md](../INDEX.md).
