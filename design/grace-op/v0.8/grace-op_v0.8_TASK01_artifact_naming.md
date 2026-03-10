# TASK01 — Artifact Naming Convention & `build.publish` Schema

## Objective

Define the platform-tagged artifact naming convention and extend
`holon.yaml` with a `build.publish` schema declaring which
platforms to produce binaries for.

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`

## Reference

- [DESIGN_release_pipeline.md](./DESIGN_release_pipeline.md) — §Artifact Naming, §CI Build Matrix

## Scope

### Naming convention

```
<holon>_<version>_<os>-<arch>[.<ext>]
```

Standardize OS/arch strings: `darwin-arm64`, `linux-amd64`,
`windows-amd64`, `ios-arm64`, `android-arm64`, `wasm`.

### Manifest schema

```yaml
build:
  publish:
    platforms:
      - darwin-arm64
      - linux-amd64
      - windows-amd64
      - wasm
```

### `op check` validation

- Validate platform strings against known values
- Warn if declared platforms don't match any `build.targets` entry

## Acceptance Criteria

- [ ] `build.publish` parsed in manifest struct
- [ ] `op check` validates platform list
- [ ] Naming convention documented
- [ ] `go test ./...` — zero failures

## Dependencies

v0.7 TASK01 (`build.targets` schema must exist).
