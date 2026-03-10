# OP v0.3 Design Tasks — Core Maturity

## Tasks

| # | File | Summary | Depends on |
|---|---|---|---|
| 01 | [TASK01](./grace-op_v0.3_TASK01_install_no_build.md) | `op install --no-build` flag | — |
| 02 | [TASK02](./grace-op_v0.3_TASK02_composite_kind.md) | `kind: composite` + `artifacts.primary` | — |
| 03 | [TASK03](./grace-op_v0.3_TASK03_tier1_runners.md) | `cargo`, `swift-package`, `flutter` runners | — |
| 04 | [TASK04](./grace-op_v0.3_TASK04_tier2_runners.md) | `npm`, `gradle` runners | TASK03 |
| 05 | [TASK05](./grace-op_v0.3_TASK05_tier3_runners.md) | `dotnet`, `qt-cmake` runners | TASK03 |
| 06 | [TASK06](./grace-op_v0.3_TASK06_install_bundles.md) | Install .app/.exe bundles to `$OPBIN` | TASK02 |
| 07 | [TASK07](./grace-op_v0.3_TASK07_package_distribution.md) | Package manager distribution (Homebrew, WinGet, NPM) | — |
| 08 | [TASK08](./grace-op_v0.3_TASK08_mesh_documentation.md) | Document `op mesh` + transport security | — |
| 09 | [TASK09](./grace-op_v0.3_TASK09_setup_documentation.md) | Document `op setup` + `setup.yaml` spec | TASK08 |

Design documents: [DESIGN_holon_templates.md](./DESIGN_holon_templates.md)
