# TASK07 — Lifecycle Engine

## Context

DESIGN.md §3.5 defines the full lifecycle doctrine: start marking,
completion with ✅/❌ rename, `_TASKS.md` updates, failure reports,
version folder status, and release tagging.

## Objective

Implement `internal/lifecycle` and `internal/git/ops.go`.

## Changes

### [NEW] `internal/lifecycle/start.go`

```go
package lifecycle

// StartTask marks the task as in-progress in _TASKS.md and renames the
// version folder to 💭 if this is the first task. Commits and pushes.
func StartTask(task tasks.Entry, setDir string, git *git.Ops) error { ... }
```

### [NEW] `internal/lifecycle/complete.go`

```go
// CompleteTask handles ✅ or ❌ outcomes:
// - Renames task file with status suffix
// - Injects ## Status block
// - Updates _TASKS.md row
// - On ❌: generates .failure.md with attempt history
// - Commits and pushes
func CompleteTask(task tasks.Entry, result codex.Result, setDir string, git *git.Ops) error { ... }
```

### [NEW] `internal/lifecycle/status.go`

```go
// UpdateVersionStatus evaluates all tasks and renames the version folder:
// any ❌ → ⚠️, all ✅ → ✅, otherwise stays 💭.
func UpdateVersionStatus(setDir string, entries []tasks.Entry, git *git.Ops) error { ... }
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

func (o *Ops) MvFolder(from, to string) error { ... }
func (o *Ops) AddCommitPush(msg string, files ...string) error { ... }
func (o *Ops) Tag(name, msg string) error { ... }
```

## Acceptance Criteria

- [ ] `_TASKS.md` updated with 🔨, ✅, or ❌ markers
- [ ] Task files renamed with status suffix
- [ ] Failure reports contain attempt history
- [ ] Version folder renamed via `git mv`
- [ ] Release tag created on full completion
- [ ] All git ops commit and push
- [ ] `go build ./...` — zero errors
- [ ] `go vet ./...` — zero warnings

## Dependencies

TASK02 (logging), TASK03 (tasks.Entry type and parser).
