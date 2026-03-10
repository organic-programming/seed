# TASK10 — Token Accounting + Post-Run Summary

## Context

DESIGN.md §3.12 requires token usage tracking from JSONL events and §3.15
defines a structured post-run summary.

## Objective

Extend `internal/state` with token tracking and add `internal/summary`.

## Changes

### [MODIFY] `internal/state/state.go`

Extend the state struct with per-task token usage:

```go
type TaskState struct {
    Completed bool       `json:"completed"`
    ThreadID  string     `json:"thread_id,omitempty"`
    Tokens    TokenUsage `json:"tokens"`
    Phase     string     `json:"phase,omitempty"`    // for resume
    Attempts  int        `json:"attempts,omitempty"` // for resume
}

type TokenUsage struct {
    InputTokens       int `json:"input_tokens"`
    CachedInputTokens int `json:"cached_input_tokens"`
    OutputTokens      int `json:"output_tokens"`
}
```

### [NEW] `internal/summary/summary.go`

```go
package summary

// Print writes the post-run summary table to stdout and to
// .codex_orchestrator_summary.md.
func Print(st *state.State, sets []SetResult, elapsed time.Duration) { ... }

// BuildSetResult constructs a SetResult from the ordered entries and state.
func BuildSetResult(setName string, entries []tasks.Entry, st *state.State) SetResult { ... }

type SetResult struct {
    Name    string
    Tasks   int
    Passed  int
    Failed  int
    Tokens  state.TokenUsage
    Status  string // ✅, ⚠️, 💭
    Failures []string
}
```

## Acceptance Criteria

- [ ] Token usage extracted from JSONL events
- [ ] Per-task counts persisted in state file
- [ ] Summary table printed to stdout
- [ ] Summary written to `.codex_orchestrator_summary.md`
- [ ] Elapsed time displayed
- [ ] `go build ./...` — zero errors
- [ ] `go vet ./...` — zero warnings

## Dependencies

TASK02 (JSONL parsing for token extraction).
