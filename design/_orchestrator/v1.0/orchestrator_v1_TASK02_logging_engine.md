# TASK02 — Timestamped Tee Logging

## Context

DESIGN.md §3.4 requires three separate log files per task (`.jsonl`,
`.stderr.log`, `.result.md`), a human-readable timestamp on every line, and
the tee principle (nothing masked — the operator sees everything in real time).

## Objective

Implement `internal/logging` and the JSONL parsing portion of `internal/codex`.

## Changes

### [NEW] `internal/logging/tee.go`

A `TeeWriter` wraps a terminal `io.Writer` and a log `*os.File`. Every line
is prefixed with `YYYY_MM_DD_HH_MM_SS_mmm` before being written to both.

```go
package logging

// TeeWriter writes each line to both the terminal and a log file,
// prefixing every line with a human-readable timestamp.
type TeeWriter struct {
    Terminal io.Writer
    LogFile  *os.File
}

func (tw *TeeWriter) WriteLine(line string) { ... }
```

Expose a `TimestampNow() string` helper using `time.Now().Format("2006_01_02_15_04_05")` + milliseconds.

### [NEW] `internal/codex/jsonl.go`

Parse timestamped JSONL lines. Key functions:

- `ParseEvent(line string) (timestamp string, event map[string]any, err error)` — split on first `{`.
- `ExtractThreadID(events []Event) string` — find `thread.started` and return `thread_id`.
- `ExtractTokenUsage(events []Event) TokenUsage` — sum `turn.completed` usage fields.

### [NEW] `internal/codex/exec.go` (partial — logging setup only)

Set up the three log files per task and wire `TeeWriter` instances to the codex
child process stdout/stderr pipes. Pass `-o <task>.result.md` to codex.

## Acceptance Criteria

- [ ] Every line in `.jsonl` and `.stderr.log` starts with a timestamp
- [ ] Codex stdout appears on orchestrator stdout in real time
- [ ] Codex stderr appears on orchestrator stderr in real time
- [ ] `-o` flag produces a `.result.md` file
- [ ] `go build ./...` — zero errors
- [ ] `go vet ./...` — zero warnings

## Dependencies

TASK01.
