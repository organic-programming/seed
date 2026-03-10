# TASK03 — Dependency Graph

## Context

DESIGN.md §3.6 defines a strict `_TASKS.md` table format with a `Depends on`
column. The orchestrator must parse this table, build a DAG, and produce a
topological execution order.

## Objective

Implement `internal/tasks` — the `_TASKS.md` parser and dependency resolver.

## Changes

### [NEW] `internal/tasks/parser.go`

Parse the markdown table from `_TASKS.md`. Extract columns: task number, file
path (from the markdown link), summary, and depends-on list.

```go
package tasks

type Entry struct {
    Number    string
    FilePath  string   // resolved to absolute path
    Summary   string
    DependsOn []string // e.g. ["TASK01", "TASK03"]
}

// Parse reads a _TASKS.md file and returns all task entries.
func Parse(tasksFile string) ([]Entry, error) { ... }

// FindSetDir locates the version folder for a given set name (e.g. "v0.4")
// by scanning <root>/design/<project>/ for a matching directory.
// Returns the absolute path and the project name.
func FindSetDir(root, setName string) (setDir, project string, err error) { ... }
```

Handle edge cases: `—` means no dependencies; `TASK01, TASK03` means multiple;
whitespace around values.

### [NEW] `internal/tasks/dag.go`

Build an adjacency list from `Entry.DependsOn`, run Kahn's algorithm (BFS
topological sort), return ordered `[]Entry`.

```go
// Sort returns entries in a valid execution order.
// Returns an error if a cycle is detected.
func Sort(entries []Entry) ([]Entry, error) { ... }
```

## Acceptance Criteria

- [ ] Parses multi-dep entries like `TASK01, TASK03`
- [ ] Handles `—` as no dependency
- [ ] Cross-version references (e.g. `v0.6`, `v0.7 TASK01`) are preserved in `Entry.DependsOn` but not enforced by the DAG (§6 future work)
- [ ] Topological order is correct for intra-version deps
- [ ] Cycle detection returns a clear error
- [ ] `go build ./...` — zero errors
- [ ] `go vet ./...` — zero warnings

## Dependencies

TASK01.
