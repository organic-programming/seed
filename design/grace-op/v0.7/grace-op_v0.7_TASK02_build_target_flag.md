# TASK02 — `op build --target` CLI Flag

## Objective

Add the `--target` flag to `op build` so it selects a declared
cross-compilation target from `build.targets`.

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`

## Reference

- [DESIGN_cross_compilation.md](./DESIGN_cross_compilation.md) — §CLI Interface

## Scope

### CLI dispatch (`cmd/build.go`)

- Add `--target <name>` flag (default: `default`)
- Resolve target entry from manifest `build.targets[name]`
- If target not declared: fail with clear error message
  `holon "foo" does not declare a build target for "ios"`
- Pass `mode`, `tool`, `flags`, `env` to the runner

### Runner interface

- Extend runner interface to accept target parameters
- Runners that don't support cross-compilation fail gracefully
- `OP_TARGET` environment variable set for runner scripts

### No `--target` (backward compat)

- `op build` (no flag) → uses `build.targets.default` if present
- If no `build.targets` at all → current behavior unchanged

## Acceptance Criteria

- [ ] `op build --target ios` resolves target from manifest
- [ ] Missing target → clear error message
- [ ] `op build` without flag → backward compatible
- [ ] `OP_TARGET` env var passed to runner
- [ ] `go test ./...` — zero failures

## Dependencies

TASK01 (manifest schema must exist).
