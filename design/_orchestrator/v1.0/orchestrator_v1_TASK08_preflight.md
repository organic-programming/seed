# TASK08 — Pre-Flight Checks + Concurrency Lock

## Context

DESIGN.md §3.10 defines pre-flight validation and §3.13 defines a PID-based
concurrency lock.

## Objective

Implement pre-flight checks in `cmd/orchestrator` and the lock in
`internal/state/lock.go`.

## Changes

### [NEW] `internal/state/lock.go`

```go
package state

// Lock represents a PID-based file lock.
type Lock struct { path string }

// Acquire creates .codex_orchestrator.lock with the current PID.
// If a live process holds the lock, returns an error.
// Reclaims stale locks from dead processes.
func Acquire(root string) (*Lock, error) { ... }

// Release removes the lock file.
func (l *Lock) Release() error { ... }
```

### [NEW] `internal/preflight/checks.go`

```go
package preflight

// Run executes all pre-flight checks in order. Returns the first error.
func Run(cfg cli.Config) error { ... }
```

Checks (in order):
1. `exec.LookPath("codex")`
2. `codex login status` exit code
3. `codex exec --ephemeral -m <MODEL> 'Reply OK'`
4. `git status --porcelain` is empty
5. `git submodule status --recursive` (no `-` prefix)
6. `os.Stat()` for each set directory
7. `os.Stat()` for each `_TASKS.md`

## Acceptance Criteria

- [ ] Missing codex binary → clear error
- [ ] Unauthenticated codex → clear error
- [ ] Dirty git repo → clear error
- [ ] Lock prevents concurrent runs
- [ ] Stale lock (dead PID) reclaimed
- [ ] Lock released on exit
- [ ] `go build ./...` — zero errors
- [ ] `go vet ./...` — zero warnings

## Dependencies

TASK01.
