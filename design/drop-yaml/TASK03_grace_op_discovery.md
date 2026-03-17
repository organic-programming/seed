# TASK03 — grace-op: discovery + lifecycle → proto-only

## Scope

- `holons/grace-op/internal/holons/discovery.go`
- `holons/grace-op/internal/holons/lifecycle.go`

## Changes

### `discovery.go`

- Remove any code path that scans for or reads `holon.yaml` files.
- Discovery must find holons by `holon.proto` only.
- Remove `addYAMLEntry()` or equivalent YAML-based discovery.

### `lifecycle.go`

- Update comment at L497: "holon.yaml must exist" → "holon.proto must exist".

## Depends on

TASK01, TASK02.

## Verification

```bash
cd holons/grace-op && go build ./...
cd holons/grace-op && go test ./internal/holons/...
```
