# TASK02 — Hermetic Environment Construction

## Context

Rob-Go must isolate its Go operations from the host system's
Go configuration. All exec-mode commands and library-mode
`packages.Load` calls must use a controlled environment.

## Objective

Implement a reusable environment builder that constructs
hermetic `Env` slices from managed variables + caller overrides.

## Changes

### `internal/toolchain/env.go` [NEW]

```go
// HermeticEnv builds an isolated environment for subprocess calls.
// It does NOT inherit os.Environ(). It constructs from scratch,
// carrying only necessary system variables (HOME, USER, TMPDIR)
// plus Rob-managed Go variables. Caller overrides are applied last.
func (t *Toolchain) HermeticEnv(overrides []string) []string
```

Managed variables:

| Variable | Source |
|---|---|
| `GOROOT` | `t.Root` |
| `GOPATH` | `$OPPATH/toolchains/go/gopath` |
| `GOMODCACHE` | `$OPPATH/toolchains/go/modcache` |
| `GOCACHE` | `$OPPATH/toolchains/go/cache` |
| `GOBIN` | `$OPBIN` |
| `PATH` | `<GOROOT>/bin:$OPBIN:<system PATH>` |

System passthrough: `HOME`, `USER`, `TMPDIR`, `TEMP`, `TMP`,
`SystemRoot` (Windows), `USERPROFILE` (Windows).

Caller overrides (from gRPC `env` fields) take precedence.

### `internal/toolchain/env_test.go` [NEW]

- `TestHermeticEnvContainsGOROOT` — verify GOROOT is set
- `TestHermeticEnvNoSystemGOPATH` — verify system GOPATH is NOT inherited
- `TestHermeticEnvOverrides` — caller override wins

## Acceptance Criteria

- [ ] `HermeticEnv` sets all managed variables
- [ ] System `GOPATH`, `GOROOT`, `GOMODCACHE` are NOT inherited
- [ ] Caller overrides in `env` field take final precedence
- [ ] `PATH` includes Rob's go binary directory
- [ ] Works on macOS, Linux, Windows

## Dependencies

TASK01 (needs `Toolchain.Root` to determine `GOROOT`).
