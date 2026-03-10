# OP v1.0.0 Design Tasks

Design documents and implementation tasks for Grace OP (the `op` CLI)

## Tasks

| # | File | Summary | Depends on |
|---|---|---|---|
| 01 | [TASK01](./TASK01_tier1_runners.md) | `cargo`, `swift-package`, `flutter` runners | — |
| 02 | [TASK02](./TASK02_tier2_runners.md) | `npm`, `gradle` runners | TASK01 |
| 03 | [TASK03](./TASK03_tier3_runners.md) | `dotnet`, `qt-cmake` runners | TASK01 |
| 04 | [TASK04](./TASK04_install_bundles.md) | Install .app/.exe bundles to `$OPBIN` | — |
| 05 | [TASK05](./TASK05_package_distribution.md) | Package manager distribution (Homebrew, WinGet, NPM) | — |
| 06 | [TASK06](./TASK06_mesh_documentation.md) | Document `op mesh` + transport security | — |
| 07 | [TASK07](./TASK07_setup_documentation.md) | Document `op setup` + `setup.yaml` spec | TASK06 |
| 08 | [TASK08](./TASK08_recipe_restructuring.md) | Restructure recipes into `ui/` + `composition/` | — |

## Design Documents

Reference material used by the tasks above — not tasks themselves.

| Document | Referenced by |
|---|---|
| [DESIGN_mesh.md](./DESIGN_mesh.md) | TASK06 |
| [DESIGN_setup.md](./DESIGN_setup.md) | TASK07 |
| [DESIGN_public_holons.md](./DESIGN_public_holons.md) | TASK06 |
| [DESIGN_transport_rest_sse.md](./DESIGN_transport_rest_sse.md) | TASK06 |
