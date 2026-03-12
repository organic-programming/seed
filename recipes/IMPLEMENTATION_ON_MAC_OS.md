# Recipe Implementation Guide — macOS

This document tracks the v0.4.3 recipe layout on macOS. It replaces the
older submodule-era notes that referenced separate `*-holons` recipe
repositories.

## Source Of Truth

- Inventory: `design/grace-op/v0.4/recipes.yaml`
- Canonical names: `design/grace-op/v0.4/DESIGN_recipe_monorepo.md`

## Current Layout

```text
recipes/
├── protos/
├── daemons/
├── hostui/
├── assemblies/
├── composition/
└── testmatrix/
```

- `recipes/daemons/` holds the dry greeting daemons.
- `recipes/hostui/` holds the dry HostUI projects.
- `recipes/assemblies/` holds the 48 Gudule greeting composites.
- `recipes/composition/` holds the 33 Charon composition recipes and 2
  shared Go workers.
- `recipes/testmatrix/gudule-greeting-testmatrix/` is the reusable
  build-and-run audit CLI.

## macOS Status

- All 48 greeting assemblies currently declare `platforms: [macos]`.
- Transport is explicit across the matrix:
  - SwiftUI assemblies use `transport: stdio`.
  - Flutter, Compose, Dotnet, Qt, and Web assemblies use `transport: tcp`.
- The web row remains a daemon + shared web HostUI composite in v0.4.3.
  The shared web dist defaults to same-origin and can be overridden per
  assembly when needed.
- UI assemblies are validated as launch smoke only. Full UI interaction
  and RPC behavior remain manual checks.
- Composition recipes run as CLI demos and are validated by exit code
  plus fixed JSON output.

## Useful Commands

From the repository root:

```sh
op build recipes/assemblies/gudule-greeting-go-web
op run --no-build recipes/assemblies/gudule-greeting-go-web

op build recipes/composition/direct-call/charon-direct-go-go
op run --no-build recipes/composition/direct-call/charon-direct-go-go

go test ./recipes/testmatrix/gudule-greeting-testmatrix/...
go run ./recipes/testmatrix/gudule-greeting-testmatrix --filter 'charon-' --format json
```

## Migration Notes

- The old `recipes/*-holons` git submodules were removed in v0.4.3.
- New macOS work should land in the shared dry holons under
  `recipes/daemons/`, `recipes/hostui/`, `recipes/assemblies/`, and
  `recipes/composition/`.
- Canonical daemon runtime naming is now:
  - binary: `gudule-daemon-greeting-<lang>`
  - slug: `gudule-greeting-daemon-<lang>`
  - family name: `Greeting-Daemon-<Lang>`
