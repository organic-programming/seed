# TASK09 — Submodule Write Access

## Context

DESIGN.md §3.11 specifies auto-detection of submodule references in task
files and passing `--add-dir` flags to codex exec.

## Objective

Implement `internal/git/submodule.go`.

## Changes

### [NEW] `internal/git/submodule.go`

```go
package git

// ListSubmodulePaths returns local paths for all submodules (recursive).
func ListSubmodulePaths(root string) ([]string, error) { ... }

// DetectRefs scans a task file for repository references and returns
// the corresponding local submodule paths for --add-dir flags.
func DetectRefs(taskContent string, submodules []string) []string { ... }
```

Detection patterns:
- `github.com/organic-programming/<repo>`
- `## Repository` sections with repo names

### [MODIFY] `internal/codex/exec.go`

Append `--add-dir <path>` for each detected submodule.

## Acceptance Criteria

- [ ] Task referencing `go-holons` → `--add-dir` for its path
- [ ] No references → no `--add-dir` flags
- [ ] Multiple references handled
- [ ] Non-existent submodule → warning (not fatal)
- [ ] `go build ./...` — zero errors
- [ ] `go vet ./...` — zero warnings

## Dependencies

TASK01.
