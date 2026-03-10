# TASK07 — Lifecycle Engine

## Context

DESIGN.md §3.5 defines the full lifecycle doctrine: start marking,
completion status in `_TASKS.md`, `## Status` blocks in task files,
failure reports, version folder status, and release tagging.

Task files are **never renamed** — all status is tracked via the
`_TASKS.md` Status column and the task file's `## Status` block.

## Objective

Implement `internal/lifecycle` and `internal/git/ops.go`.

## Changes

### [NEW] `internal/lifecycle/start.go`

```go
package lifecycle

// StartTask sets the task's Status column in _TASKS.md to 💭.
// Commits and pushes.
func StartTask(task tasks.Entry, setDir string, git *git.Ops) error { ... }
```

### [NEW] `internal/lifecycle/complete.go`

```go
// CompleteTask handles ✅ or ❌ outcomes:
// - Injects ## Status block in the task file (commit SHAs, URLs)
// - Updates _TASKS.md Status column to ✅ or ❌
// - On ❌: generates .failure.md with attempt history
// - Commits and pushes
func CompleteTask(task tasks.Entry, result codex.Result, setDir string, git *git.Ops) error { ... }
```

### [NEW] `internal/lifecycle/status.go`

```go
// UpdateVersionStatus evaluates all tasks in _TASKS.md Status column
// and renames the version folder once, after all tasks complete:
// - all ✅ → ✅ v0.X
// - any ❌ → ⚠️ v0.X
func UpdateVersionStatus(setDir string, entries []tasks.Entry, git *git.Ops) error { ... }
```

### [NEW] `internal/lifecycle/reset.go`

```go
// Reset strips emoji prefix from the version folder, clears _TASKS.md
// Status column, removes ## Status blocks and .failure.md files.
// Used when re-running a previously completed version.
func Reset(setDir string, git *git.Ops) error { ... }
```

### [NEW] `internal/lifecycle/release.go`

```go
// Release bumps holon.yaml version and creates a git tag.
func Release(setDir, version string, git *git.Ops) error { ... }
```

### [NEW] `internal/git/ops.go`

```go
package git

// Ops wraps git commands needed by the lifecycle engine.
type Ops struct { Root string }

func (o *Ops) Rename(from, to string) error { ... }
func (o *Ops) AddCommitPush(msg string, files ...string) error { ... }
func (o *Ops) Tag(name, msg string) error { ... }

// EnsureConsistency verifies branch consistency across root and submodules
// for the given project/set. Creates -dev branches if missing (§3.2).
func EnsureConsistency(root, project, setName string) error { ... }
```

## Acceptance Criteria

- [ ] `_TASKS.md` Status column updated with 💭, ✅, or ❌
- [ ] `## Status` block injected in task files (never renamed)
- [ ] Failure reports contain attempt history
- [ ] Version folder renamed only once on completion
- [ ] Reset clears all status for re-runs
- [ ] Release tag created on full ✅ completion
- [ ] `go build ./...` — zero errors
- [ ] `go vet ./...` — zero warnings

## Dependencies

TASK02 (logging), TASK03 (tasks.Entry type and parser).
