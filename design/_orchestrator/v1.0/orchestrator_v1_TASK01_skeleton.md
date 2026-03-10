# TASK01 — Project Skeleton

## Context

The worktree already contains a stub `main.go` (exits immediately) and a
`go.mod` with an old module path. The empty `pkg/` directory is a leftover.
This task bootstraps the proper Effective Go structure on top of that baseline.

## Objective

Create the complete `cmd/` + `internal/` directory tree with stub files so that
subsequent tasks can implement each package independently. Fix the module path.

## Changes

### [MODIFY] `go.mod`

Update the module path:

```
module github.com/organic-programming/codex-orchestrator

go 1.25.1
```

### [MODIFY] `main.go` → [MOVE] `cmd/orchestrator/main.go`

Move the existing stub into the canonical location and add basic flag parsing:

```go
package main

import (
    "fmt"
    "os"

    "github.com/organic-programming/codex-orchestrator/internal/cli"
)

func main() {
    cfg, err := cli.Parse()
    if err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
    fmt.Printf("orchestrator: sets=%v model=%s root=%s\n", cfg.Sets, cfg.Model, cfg.Root)
}
```

### [NEW] `internal/cli/cli.go`

Flag parsing with `--set` (repeatable), `--model`, `--root`:

```go
package cli

type Config struct {
    Sets  []string
    Model string
    Root  string
}

func Parse() (*Config, error) { ... }
```

### [NEW] Stub files for all internal packages

Create files with the correct `package` declaration so that
`go build ./...` succeeds from the start:

- `internal/codex/exec.go` — `package codex`
- `internal/codex/jsonl.go` — `package codex`
- `internal/codex/retry.go` — `package codex`
- `internal/git/branch.go` — `package git`
- `internal/git/submodule.go` — `package git`
- `internal/git/ops.go` — `package git`
- `internal/lifecycle/start.go` — `package lifecycle`
- `internal/lifecycle/complete.go` — `package lifecycle`
- `internal/lifecycle/status.go` — `package lifecycle`
- `internal/lifecycle/release.go` — `package lifecycle`
- `internal/logging/tee.go` — `package logging`
- `internal/prompt/builder.go` — `package prompt`
- `internal/prompt/compress.go` — `package prompt`
- `internal/state/state.go` — `package state`
- `internal/state/lock.go` — `package state`
- `internal/tasks/parser.go` — `package tasks`
- `internal/tasks/dag.go` — `package tasks`
- `internal/verify/extract.go` — `package verify`
- `internal/verify/runner.go` — `package verify`
- `internal/preflight/checks.go` — `package preflight`
- `internal/summary/summary.go` — `package summary`

### [DELETE] `pkg/`

Remove the empty placeholder directory. All packages live under `internal/`.

## Acceptance Criteria

- [ ] `go build ./...` — zero errors
- [ ] `go vet ./...` — zero warnings
- [ ] `go run ./cmd/orchestrator` runs and prints config
- [ ] Every `internal/` package has at least one `.go` file
- [ ] No code outside `cmd/` and `internal/`
- [ ] `pkg/` directory removed
- [ ] Module path is `github.com/organic-programming/codex-orchestrator`

## Dependencies

None — this is the foundation task.
