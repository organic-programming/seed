# TASK05 — grace-op: update all CLI + server tests

## Scope

- `holons/grace-op/internal/cli/*_test.go`
- `holons/grace-op/internal/server/server_test.go`
- `holons/grace-op/internal/suggest/suggest_test.go`
- `holons/grace-op/internal/holons/recipe_test.go`

## Changes

- `commands_test.go`: ~15 test functions write `holon.yaml` inline — update all to write `holon.proto` equivalents.
- `inspect_test.go`, `mcp_tools_test.go`, `transport_chain_test.go`: same pattern.
- `suggest_test.go`: same.
- `server_test.go`: update `seedHolon` helper to create `holon.proto` instead of `holon.yaml`.
- `recipe_test.go`: update YAML manifest comparisons to proto equivalents. Note the test at L310 (`TestRecipeManifestParity`) explicitly compares YAML and proto results — this test should be simplified to proto-only.

## Depends on

TASK01–04 (all grace-op source changes must land first).

## Verification

```bash
cd holons/grace-op && go test ./...
```

All tests must pass. Zero references to `holon.yaml` should remain in test files.
