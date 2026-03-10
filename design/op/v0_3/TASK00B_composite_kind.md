# TASK00B — Composite Kind Formalization

## Context

OP.md §4, OP_BUILD_SPEC §1 and §3 define `kind: composite` and
`artifacts.primary` for holons whose deliverable is not a single
binary (e.g., `.app` bundles). The `recipe` runner already works
but the manifest parser does not formally support these fields.

## Relationship to other tasks

- **TASK04** (install bundles) depends on this: it needs
  `artifacts.primary` to locate the bundle path for install.
- **TASK09** (build configs) touches the manifest parser but
  different fields — no conflict.
- TASK01–08: no interaction.

## Objective

Formalize `kind: composite` and `artifacts.primary` in the manifest
parser so that composite holons are first-class citizens.

## Changes

### 1. Manifest parser (`internal/holons/manifest.go`)

Accept `composite` as a valid `kind` value alongside `native` and
`wrapper`.

Add to the `Artifacts` struct:

```go
Primary string `yaml:"primary,omitempty"`
```

Validation in `op check`:
- `kind: composite` may omit `artifacts.binary`
- `kind: composite` should declare `artifacts.primary`
- `kind: native` / `kind: wrapper` must declare `artifacts.binary`
- A holon cannot declare both `artifacts.binary` and
  `artifacts.primary` simultaneously

### 2. Success contract (`internal/holons/lifecycle.go`)

After `op build`, verify the primary artifact exists:

```
if manifest.Artifacts.Primary != "" {
    check artifacts.primary exists
} else {
    check artifacts.binary exists (current behavior)
}
```

### 3. Report struct

Populate `Report.Artifact` with `artifacts.primary` when set,
`artifacts.binary` otherwise.

## Acceptance Criteria

- [ ] `kind: composite` accepted by manifest parser
- [ ] `artifacts.primary` parsed and stored
- [ ] `op check` validates kind ↔ artifact consistency
- [ ] `op build` success contract checks `artifacts.primary` when set
- [ ] Report shows correct artifact path for composite holons
- [ ] Existing `native`/`wrapper` holons unaffected
- [ ] `go test ./...` — zero failures

## Dependencies

None. TASK04 depends on this task.
