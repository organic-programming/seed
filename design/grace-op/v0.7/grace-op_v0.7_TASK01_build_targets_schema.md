# TASK01 — `build.targets` Manifest Schema

## Objective

Extend the `holon.yaml` manifest parser to support the
`build.targets` schema for cross-compilation target declarations.

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`

## Reference

- [DESIGN_cross_compilation.md](./DESIGN_cross_compilation.md) — §Manifest: `build.targets`

## Scope

### Manifest parser (`internal/holons/manifest.go`)

Add `build.targets` map to the manifest struct:

```yaml
build:
  runner: go-module
  targets:
    default:
      mode: binary
    ios:
      mode: framework
      tool: gomobile bind
      flags: [-target, ios]
    wasm:
      mode: wasm
      env:
        GOOS: js
        GOARCH: wasm
```

Each target entry has:
- `mode` (required): `binary`, `framework`, or `wasm`
- `tool` (optional): override build tool
- `flags` (optional): additional build flags
- `env` (optional): environment variables for the build

### `op check` validation

- Validate `mode` is one of: `binary`, `framework`, `wasm`
- Warn if no `default` target is declared
- Reject unknown fields in target entries

## Acceptance Criteria

- [ ] `build.targets` parsed and stored in manifest struct
- [ ] `op check` validates target entries
- [ ] Existing holons without `build.targets` unaffected
- [ ] `go test ./...` — zero failures

## Dependencies

None — schema only, no build behavior change.
