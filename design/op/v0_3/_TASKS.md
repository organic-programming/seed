# OP v0.3 Design Tasks

Design documents and implementation tasks for Grace OP (the `op` CLI)

## Tasks

| # | File | Summary | Depends on |
|---|---|---|---|
| 00A | [TASK00A](./TASK00A_install_no_build.md) | `op install --no-build` flag | — |
| 00B | [TASK00B](./TASK00B_composite_kind.md) | `kind: composite` + `artifacts.primary` | — |
| 01 | [TASK01](./TASK01_tier1_runners.md) | `cargo`, `swift-package`, `flutter` runners | — |
| 02 | [TASK02](./TASK02_tier2_runners.md) | `npm`, `gradle` runners | TASK01 |
| 03 | [TASK03](./TASK03_tier3_runners.md) | `dotnet`, `qt-cmake` runners | TASK01 |
| 04 | [TASK04](./TASK04_install_bundles.md) | Install .app/.exe bundles to `$OPBIN` | TASK00B |
| 05 | [TASK05](./TASK05_package_distribution.md) | Package manager distribution (Homebrew, WinGet, NPM) | — |
| 06 | [TASK06](./TASK06_mesh_documentation.md) | Document `op mesh` + transport security | — |
| 07 | [TASK07](./TASK07_setup_documentation.md) | Document `op setup` + `setup.yaml` spec | TASK06 |
| 08 | [TASK08](./TASK08_recipe_restructuring.md) | Restructure recipes into `ui/` + `composition/` | — |
| 08.01 | [TASK08.01](./TASK08.01_dry_daemons.md) | Extract 8 DRY daemons | — |
| 08.02 | [TASK08.02](./TASK08.02_dry_hostui.md) | Extract 6 DRY HostUIs | — |
| 08.03 | [TASK08.03](./TASK08.03_assembly_manifests.md) | Create 48 assembly manifests | 08.01, 08.02 |
| 08.04 | [TASK08.04](./TASK08.04_remove_submodules.md) | Remove 12 submodules, archive repos | 08.03 |
| 08.05 | [TASK08.05](./TASK08.05_testmatrix.md) | Combinatorial testing (Go testmatrix) | 08.03, 08.06 |
| 08.06 | [TASK08.06](./TASK08.06_composition_recipes.md) | 3 patterns × 11 orchestrator languages | — |
| 09 | [TASK09](./TASK09_build_configs.md) | Build configs (`--config` + `OP_CONFIG`) | — |
| 10 | [TASK10](./TASK10_mvs_resolution.md) | MVS transitive dependency resolution | — |
| 11 | [TASK11](./TASK11_sequences.md) | Sequences (`op do`) + MCP tool exposure | — |

Design documents referenced by these tasks are listed in [INDEX.md](../INDEX.md).
