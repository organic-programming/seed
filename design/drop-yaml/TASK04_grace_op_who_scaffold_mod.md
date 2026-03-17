# TASK04 — grace-op: who, scaffold, mod → proto-only

## Scope

- `holons/grace-op/internal/who/`
- `holons/grace-op/internal/scaffold/`
- `holons/grace-op/internal/mod/`

## Changes

### `who/who.go`

- `Create()`: write `holon.proto` instead of `holon.yaml`.
- Update error messages referencing `holon.yaml`.
- Replace `identity.WriteHolonYAML()` calls with the proto-based equivalent from TASK01.

### `scaffold/`

- Scaffold must generate `holon.proto` instead of `holon.yaml`.
- **DO NOT** remove `template.yaml` support or its `yaml.v3` import — this is the scaffold template system, unrelated to holon manifests.

### `mod/mod.go`

- Remove `"/holon.yaml"` suffix handling in version strings (L385).
- Remove `holonYAMLHash` computation and hash entries in `holon.sum` (L551-553).
- Replace `identity.ReadHolonYAML(manifestPath)` call (L499) with proto-based equivalent.

## Depends on

TASK01.

## Verification

```bash
cd holons/grace-op && go build ./...
cd holons/grace-op && go test ./internal/who/... ./internal/scaffold/... ./internal/mod/...
```
