# Recipes

The recipe workspace now lives directly in this repository. v0.4.3
replaces the older `*-holons` submodules with shared dry daemons,
shared HostUIs, generated assemblies, Charon compositions, and a
reusable matrix runner.

## Try It Yourself

From the repository root:

```sh
op build recipes/composition/direct-call/charon-direct-go-go
op run --no-build recipes/composition/direct-call/charon-direct-go-go

op build recipes/assemblies/gudule-greeting-go-web
op run --no-build recipes/assemblies/gudule-greeting-go-web

go test ./recipes/testmatrix/gudule-greeting-testmatrix/...
go run ./recipes/testmatrix/gudule-greeting-testmatrix --dry-run --format json
go run ./recipes/testmatrix/gudule-greeting-testmatrix --filter 'charon-' --format json
```

## Current Matrix

- Inventory:
  - 48 Gudule greeting assemblies in `recipes/assemblies/`
  - 33 Charon composition recipes in `recipes/composition/`
  - 2 shared Go workers in `recipes/composition/workers/`
- Latest committed runtime snapshot:
  - `recipes/testmatrix/gudule-greeting-testmatrix/snapshots/current-macos-composition.json`
  - generated on March 12, 2026 with `--filter 'charon-'`
  - result: 33 selected, 33 passed, 0 skipped, 0 build failures, 0 run failures, 0 timeouts
- Full discovery snapshot:
  - `recipes/testmatrix/gudule-greeting-testmatrix/snapshots/current-macos-dry-run.json`
  - result: 81 selected, 48 assemblies, 33 compositions
- The committed runtime baseline above is intentionally focused on the
  composition recipes because UI assemblies are only validated as launch
  smoke and still need manual interaction checks.

## Layout

```text
recipes/
├── protos/         # shared canonical contracts
├── daemons/        # dry greeting daemons
├── hostui/         # dry HostUI projects
├── assemblies/     # 48 greeting composites
├── composition/    # 33 charon compositions + 2 workers
└── testmatrix/     # reusable Go matrix CLI + snapshots
```

### Shared Contracts

- `recipes/protos/greeting/v1/greeting.proto`
- `recipes/protos/compute/v1/compute.proto`
- `recipes/protos/transform/v1/transform.proto`

### Transport Rules

- All desktop HostUI assemblies use `transport: stdio`.
- Web assemblies use `transport: tcp`.
- The web row remains daemon + shared web HostUI composites in v0.4.3.

## Notes

- Inventory comes from `design/grace-op/v0.4/recipes.yaml`.
- Canonical naming comes from
  `design/grace-op/v0.4/DESIGN_recipe_monorepo.md`.
- Old recipe submodules were removed in v0.4.3; new work should target
  the shared dry holons and the generated assembly/composition trees.
