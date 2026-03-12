# TASK04 — Wire Library Mode to Hermetic Environment

## Context

Library-mode functions (`TypeCheck`, `Analyze`, `LoadPackages`,
`Doc`) use `packages.Load`, which shells out via the Go driver.
This driver currently inherits `os.Environ()`.

## Objective

Pass the hermetic environment to `packages.Config.Env`.

## Changes

### `internal/analyzer/analyzer.go`

Modify `loadWithConfig` to accept the hermetic environment
as a parameter instead of appending to `os.Environ()`:

```go
// Before
cfg.Env = append(os.Environ(), env...)

// After
cfg.Env = env // already hermetic + caller overrides
```

The service layer (TASK03) is responsible for merging hermetic
env + caller overrides before calling analyzer functions.

### `internal/analyzer/analyzer.go`

Update function signatures to accept the pre-merged environment:

```go
func TypeCheck(patterns []string, workdir string, env []string) ...
func Analyze(patterns []string, workdir string, env []string, ...) ...
func LoadPackages(patterns []string, workdir string, env []string, ...) ...
func Doc(pattern, workdir string, env []string) ...
```

`Doc` gains an `env` parameter (currently hardcoded to `nil`).

### `internal/service/service.go`

Service methods merge hermetic env + caller overrides before
calling analyzer functions.

## Acceptance Criteria

- [ ] `loadWithConfig` does not call `os.Environ()`
- [ ] `Doc` accepts `env` parameter
- [ ] Library mode uses same hermetic environment as exec mode
- [ ] Existing tests pass
- [ ] `go test ./... -race` — zero failures

## Dependencies

TASK02 (hermetic env).
