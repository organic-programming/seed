# Recipe Implementation Guide — Windows

This document tracks the Windows port posture after the v0.4.3 recipe
restructure. It replaces the older submodule-era notes that referenced
separate `*-holons` repositories.

## Source Of Truth

- Inventory: `design/grace-op/v0.4/recipes.yaml`
- Canonical names: `design/grace-op/v0.4/DESIGN_recipe_monorepo.md`

## Current Repository Model

The workspace now keeps reusable pieces in shared directories:

```text
recipes/
├── daemons/
├── hostui/
├── assemblies/
├── composition/
└── testmatrix/
```

Windows enablement should extend these dry holons and assemblies
directly. Do not recreate the removed `recipes/*-holons` submodules.

## Windows Expectations

- HostUI rows expected to be viable on Windows:
  - Flutter
  - Compose
  - Web
  - Dotnet
  - Qt
- HostUI row not expected on Windows:
  - SwiftUI
- Daemon languages expected to port most cleanly:
  - Go
  - Rust
  - Kotlin
  - Dart
  - Python
  - Csharp
  - Node
- The Swift daemon remains a macOS/Linux target in the current inventory.

## Porting Rules

- Preserve canonical daemon naming:
  - binary: `gudule-daemon-greeting-<lang>`
  - slug: `gudule-greeting-daemon-<lang>`
  - family name: `Greeting-Daemon-<Lang>`
- Keep transport rules aligned with the current matrix:
  - SwiftUI: `stdio` on Apple platforms only
  - Flutter, Compose, Dotnet, Qt, Web: `tcp`
- Keep the v0.4.3 web row as daemon + shared web HostUI composites.
- Treat UI assemblies as launch smoke in automation; full UI/RPC
  interaction remains a manual pass.

## Suggested Windows Enablement Order

1. Extend the dry daemon holons that already support Windows in
   `recipes/daemons/`.
2. Add Windows targets to the Windows-capable HostUIs in
   `recipes/hostui/`.
3. Update the relevant assembly manifests in `recipes/assemblies/` so
   `platforms` and `requires.commands` reflect the new targets.
4. Run the matrix tool with Windows filters to classify skips before
   attempting the full build:

```sh
go run ./recipes/testmatrix/gudule-greeting-testmatrix --dry-run --format json
go run ./recipes/testmatrix/gudule-greeting-testmatrix --filter 'charon-|gudule-greeting-' --format json
```

## Migration Notes

- The old recipe submodules were removed in v0.4.3.
- Future Windows work should update the shared dry holons, not revive
  the retired recipe repos.
