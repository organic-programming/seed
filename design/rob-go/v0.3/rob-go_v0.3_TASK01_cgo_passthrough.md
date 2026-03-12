# TASK01 — CGO Passthrough Allowlist

## Context

Rob-Go's hermetic environment (v0.1 TASK02) strips all host
variables. CGO-enabled builds need access to the host C
compiler via `CC`, `CXX`, `AR`.

## Objective

Extend `HermeticEnv` with a passthrough mode that inherits
host C toolchain variables when CGO is active.

## Changes

### `internal/toolchain/env.go`

Extend `HermeticEnv` signature:

```go
func (t *Toolchain) HermeticEnv(overrides []string, cgoPassthrough bool) []string
```

When `cgoPassthrough` is true, the following host variables
are read from `os.Getenv` and added to the hermetic env
(before caller overrides):

| Variable | Purpose |
|---|---|
| `CC` | C compiler path |
| `CXX` | C++ compiler path |
| `AR` | Archiver |
| `PKG_CONFIG_PATH` | pkg-config search paths |

### `internal/toolchain/env_test.go`

- `TestHermeticEnvCGOPassthrough` — verify CC/CXX inherited
- `TestHermeticEnvCGODisabled` — verify CC/CXX NOT inherited
- `TestHermeticEnvCGOOverride` — caller override wins over host

## Acceptance Criteria

- [ ] `HermeticEnv(_, true)` inherits `CC`, `CXX`, `AR`, `PKG_CONFIG_PATH`
- [ ] `HermeticEnv(_, false)` does NOT inherit them
- [ ] Caller override `CC=/custom/gcc` takes precedence over host
- [ ] Existing v0.1 tests still pass (they use `cgoPassthrough: false`)

## Dependencies

v0.1 TASK02 (hermetic env).
