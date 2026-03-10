# TASK06 — Error Classification + Retry

## Context

DESIGN.md §3.7 defines error classification (network, quota, task failure,
sandbox violation) with different retry strategies per category.

## Objective

Implement `internal/codex/retry.go` — error classifier and retry executor.

## Changes

### [NEW] `internal/codex/retry.go`

```go
package codex

type ErrorCategory int

const (
    ErrNetwork ErrorCategory = iota
    ErrQuota
    ErrTaskFailure
    ErrSandboxViolation
)

// Classify inspects the exit code and stderr to determine the error category.
func Classify(exitCode int, stderr string) ErrorCategory { ... }

// RetryWithBackoff re-invokes the given function according to the retry
// policy for the error category. Logs heartbeat every 60s during waits.
func RetryWithBackoff(category ErrorCategory, fn func() error, logger *logging.TeeWriter) error { ... }
```

Pattern matching on stderr:
- Network: `connection`, `timeout`, `DNS`, `ECONNREFUSED`
- Quota: `429`, `rate limit`, `quota`, `capacity`
- Sandbox: `sandbox`, `permission denied`

Network: exponential backoff (5s→5min, 5 attempts).
Quota: long waits (15m→1h, 3 attempts).

### [MODIFY] `internal/codex/exec.go`

Wire `Classify` and `RetryWithBackoff` into `ExecuteLoop`. Network/quota
errors retry the same invocation. Task failures enter the FIX phase.

## Acceptance Criteria

- [ ] Network errors trigger exponential backoff
- [ ] Quota errors trigger long-wait retry
- [ ] Task failures pass to FIX loop (not retried blindly)
- [ ] Sandbox violations mark ❌ immediately
- [ ] Heartbeat logged during waits
- [ ] `go build ./...` — zero errors
- [ ] `go vet ./...` — zero warnings

## Dependencies

TASK02 (logging), TASK05 (execution loop).
