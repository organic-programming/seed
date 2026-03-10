# OP v0.3 Design Tasks — Core Maturity

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

Design documents referenced by these tasks are listed in [INDEX.md](../INDEX.md).
