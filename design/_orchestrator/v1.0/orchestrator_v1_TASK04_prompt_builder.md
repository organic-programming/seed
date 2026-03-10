# TASK04 — Prompt Builder + Context Compression

## Context

DESIGN.md §3.8 defines a 4-layer context architecture (System, Version,
History, Task) with token-budget-based compression.

## Objective

Implement `internal/prompt` — the prompt assembly and compression module.

## Changes

### [NEW] `internal/prompt/builder.go`

```go
package prompt

// Build assembles the 4-layer prompt for a task invocation.
func Build(cfg Config, setDir, taskFile string, priorResults []string) (string, error) { ... }
```

1. **System layer** — read `CONVENTIONS.md` and `.agent/AGENT.md`.
2. **Version layer** — read all `DESIGN_*.md` in `setDir`.
3. **History layer** — read `.result.md` from prior completed tasks.
4. **Task layer** — read the current task file.
5. Concatenate using the template from §3.8.
6. Append LLM guardrails.

### [NEW] `internal/prompt/compress.go`

```go
// EstimateTokens returns a rough token count (len/4).
func EstimateTokens(text string) int { ... }

// CompressHistory calls codex exec --ephemeral with a summarization prompt.
// Caches the result as <setDir>/_HISTORY_SUMMARY.md.
func CompressHistory(results []string, setDir string) (string, error) { ... }
```

Budget: `model_max_context × 0.40`. When over, compress via
`codex exec --ephemeral -m gpt-5.1-codex-mini`.

## Acceptance Criteria

- [ ] Prompt includes all four layers in correct order
- [ ] Missing files handled gracefully (no panic)
- [ ] Token estimation triggers compression above threshold
- [ ] Compressed summary is cached and reused
- [ ] `go build ./...` — zero errors
- [ ] `go vet ./...` — zero warnings

## Dependencies

TASK01.
