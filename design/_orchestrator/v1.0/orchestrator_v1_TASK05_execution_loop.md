# TASK05 — Execution Loop (Create → Verify → Fix)

## Context

DESIGN.md §3.9 defines an iterative loop per task: CREATE → VERIFY → FIX,
using `codex exec resume` for fix phases to preserve session context.

## Objective

Implement `internal/verify` and the execution loop in `internal/codex`.

## Changes

### [NEW] `internal/verify/extract.go`

Extract verifiable shell commands from task file sections
(`## Acceptance Criteria`, `## Checklist`):

```go
package verify

// ExtractCommands parses a task file and returns shell commands to verify.
func ExtractCommands(taskFile string) ([]string, error) { ... }
```

Matches: backtick-quoted commands, lines starting with `go test`, `op build`,
`cargo test`, `swift test`, `flutter test`.

### [NEW] `internal/verify/runner.go`

```go
// Result holds the outcome of a single verification command.
type Result struct {
    Command  string
    Passed   bool
    Output   string
    Duration time.Duration
}

// Run executes each command via os/exec with a timeout.
func Run(commands []string, workDir string, timeout time.Duration) []Result { ... }
```

### [MODIFY] `internal/codex/exec.go`

Add `ExecuteLoop` — the main create/verify/fix loop:

```go
// ExecuteLoop runs the §3.9 loop: CREATE → VERIFY → FIX (up to maxAttempts).
func ExecuteLoop(cfg Config, task Task, prompt string, addDirs []string, st *state.State) Result { ... }
```

- CREATE: invoke `codex exec` with the full prompt.
- VERIFY: call `verify.Run()` on extracted commands.
- FIX: invoke `codex exec resume <thread_id>` with verification error output.
- Track `thread_id` from JSONL `thread.started` event.

### Constants

```go
const MaxFixAttempts = 3
const VerifyTimeout  = 5 * time.Minute
```

## Acceptance Criteria

- [ ] CREATE phase invokes codex exec with full prompt
- [ ] VERIFY phase runs extracted commands with timeout
- [ ] FIX phase uses `codex exec resume <thread_id>`
- [ ] Loop terminates on success or after max attempts
- [ ] `go build ./...` — zero errors
- [ ] `go vet ./...` — zero warnings

## Dependencies

TASK02 (logging for JSONL capture and `thread_id` extraction).
