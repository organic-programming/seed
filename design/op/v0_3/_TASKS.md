# OP v0.3 Design Tasks

Design documents and implementation tasks for Grace OP (the `op` CLI)

## Tasks

| # | File | Summary | Depends on |
|---|---|---|---|
| 00 | [TASK00](./TASK00_install_no_build.md) | `op install --no-build` flag | — |
| 01 | [TASK01](./TASK01_tier1_runners.md) | `cargo`, `swift-package`, `flutter` runners | — |
| 02 | [TASK02](./TASK02_tier2_runners.md) | `npm`, `gradle` runners | TASK01 |
| 03 | [TASK03](./TASK03_tier3_runners.md) | `dotnet`, `qt-cmake` runners | TASK01 |
| 04 | [TASK04](./TASK04_install_bundles.md) | Install .app/.exe bundles to `$OPBIN` | TASK10 |
| 05 | [TASK05](./TASK05_package_distribution.md) | Package manager distribution (Homebrew, WinGet, NPM) | — |
| 06 | [TASK06](./TASK06_mesh_documentation.md) | Document `op mesh` + transport security | — |
| 07 | [TASK07](./TASK07_setup_documentation.md) | Document `op setup` + `setup.yaml` spec | TASK06 |
| 08 | [TASK08](./TASK08_recipe_restructuring.md) | Restructure recipes into `ui/` + `composition/` | — |
| 09 | [TASK09](./TASK09_build_configs.md) | Build configs (`--config` + `OP_CONFIG`) | — |
| 10 | [TASK10](./TASK10_composite_kind.md) | `kind: composite` + `artifacts.primary` | — |
| 11 | [TASK11](./TASK11_mvs_resolution.md) | MVS transitive dependency resolution | — |
| 12 | [TASK12](./TASK12_sequences.md) | Sequences (`op do`) + MCP tool exposure | — |

Design documents referenced by these tasks are listed in [INDEX.md](../INDEX.md).
