# TASK01 — Project Skeleton

## Context

The `_orchestrator/` directory currently contains only the `v1.0/` spec
folder. There is no Go code, no `go.mod`, and no `cmd/` or `internal/` tree.
This task creates the complete project structure from scratch.

## Objective

Bootstrap the Effective Go structure defined in DESIGN.md §4.1: `go.mod`,
`cmd/orchestrator/main.go`, and stub files for all `internal/` packages.

## Changes

### [NEW] `go.mod`

```
module github.com/organic-programming/codex-orchestrator

go 1.25.1
```

### [NEW] `cmd/orchestrator/main.go`

Minimal entry point with flag parsing:

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
    Sets      []string
    Model     string
    Root      string
    StateFile string // default: <root>/.codex_orchestrator_state.json
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
- `internal/lifecycle/reset.go` — `package lifecycle`
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

## Acceptance Criteria

- [ ] `go build ./...` — zero errors
- [ ] `go vet ./...` — zero warnings
- [ ] `go run ./cmd/orchestrator` runs and prints config
- [ ] Every `internal/` package has at least one `.go` file
- [ ] No code outside `cmd/` and `internal/`
- [ ] Module path is `github.com/organic-programming/codex-orchestrator`

## Dependencies

None — this is the foundation task.
