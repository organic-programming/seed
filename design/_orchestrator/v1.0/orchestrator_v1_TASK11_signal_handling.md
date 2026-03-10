# TASK11 — Signal Handling

## Context

DESIGN.md §3.14 requires graceful shutdown on SIGINT/SIGTERM with proper
state persistence and lock release.

## Objective

Add signal handling in `cmd/orchestrator/main.go`.

## Changes

### [MODIFY] `cmd/orchestrator/main.go`

Set up `signal.Notify` for `SIGINT` and `SIGTERM`. On first signal:

1. Log: `"Received <signal>, shutting down gracefully..."`
2. If a codex child process is running, forward the signal and wait for exit.
3. Save current orchestrator state (loop phase, attempt count).
4. Release the concurrency lock.
5. Exit with code 130 (128 + SIGINT).

A second signal within 3 seconds forces `os.Exit(1)`.

### [MODIFY] `internal/codex/exec.go`

Expose a `SetCurrentCmd` / `CurrentCmd` accessor pair protected by
`sync.Mutex` so the signal handler can reach the child process.

## Acceptance Criteria

- [ ] SIGINT triggers graceful shutdown with state save
- [ ] Child process receives forwarded signal
- [ ] Lock released on signal
- [ ] Double SIGINT forces exit
- [ ] Next run resumes from interrupted state
- [ ] `go build ./...` — zero errors
- [ ] `go vet ./...` — zero warnings

## Dependencies

TASK02 (logging), TASK08 (lock to release).
