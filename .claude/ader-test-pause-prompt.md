# Claude prompt — implement the ader test pause mechanism

> Hand this prompt to a fresh Claude Code session. Self-contained: the session has filesystem access to the `seed` repo and must read the referenced files to learn conventions. Do not assume any prior conversation context.

---

## Mission

Implement the per-check pause mechanism specified in [`docs/specs/ader-test-pause.md`](../docs/specs/ader-test-pause.md). The mechanism lets individual ader checks be flagged as "non-blocking under analysis" via two new fields in `checks.yaml`: `paused: true` and `paused_reason: <text>`. Paused checks still run, still report their actual result, but their failures are ignored when the suite/profile/bouquet aggregates pass/fail.

This is a coding task across ~5 files in `holons/clem-ader/internal/engine/`. ~3 days of agent time per the spec, but achievable in one focused session because the changes compose cleanly.

All written artifacts in English. PRs target `master` directly per `docs/adr/git-workflow-trunk-based.md`.

---

## Required reading

1. [`docs/specs/ader-test-pause.md`](../docs/specs/ader-test-pause.md) — the spec, all 7 sections.
2. [`holons/clem-ader/README.md`](../holons/clem-ader/README.md) — ader concepts: catalogues, suites, lanes, profiles, bouquets.
3. [`holons/clem-ader/internal/engine/config.go`](../holons/clem-ader/internal/engine/config.go) — schema definitions (`checkConfig`, `suiteStepConfig`, `rawSuiteStepConfig`, `materializeSuiteStep`).
4. [`holons/clem-ader/internal/engine/types.go`](../holons/clem-ader/internal/engine/types.go) — `StepSpec`, `StepResult`, `HistoryRecord`, `BouquetEntryResult`.
5. [`holons/clem-ader/internal/engine/engine.go`](../holons/clem-ader/internal/engine/engine.go) — execution loop (~lines 180-265), aggregate logic (~lines 260-264), markdown rendering (~lines 760-770).
6. [`holons/clem-ader/internal/engine/stepmatrix.go`](../holons/clem-ader/internal/engine/stepmatrix.go) — `StepSpec` construction (line ~152).
7. [`holons/clem-ader/internal/engine/bouquet.go`](../holons/clem-ader/internal/engine/bouquet.go) — bouquet aggregate (uses HistoryRecord status).
8. [`CLAUDE.md`](../CLAUDE.md) — repo invariants, "doubt is the method".
9. [`WORKFLOW.md`](../WORKFLOW.md) — branching and PR conventions (trunk-based on master).

---

## Implementation plan

Work in this order. Each step is a logical unit; commit each one separately for clarity.

### Step 1 — Schema extension (`config.go`)

Add to `checkConfig`:
```go
Paused       bool   `yaml:"paused,omitempty"`
PausedReason string `yaml:"paused_reason,omitempty"`
```

Add the same two fields to `suiteStepConfig` and `rawSuiteStepConfig` (so a suite can override a check's pause state if needed).

In `readChecksConfig`, after the existing validation loop, validate: if `check.Paused` is true and `strings.TrimSpace(check.PausedReason) == ""`, return an error: `"check catalog %s entry %q is paused but has no paused_reason"`.

In `materializeSuiteStep`:
- When the step references a `check`, propagate `Paused` and `PausedReason` from `check` to the resolved `suiteStepConfig`.
- When the step is inline, copy from `step.Paused` / `step.PausedReason`.
- Validation: same as above — if Paused, PausedReason required.

### Step 2 — StepSpec wiring (`types.go` + `stepmatrix.go`)

Add `Paused bool` and `PausedReason string` to `StepSpec`.

In `stepmatrix.go` ~line 152, populate the new fields when constructing the StepSpec from `stepEntry`.

### Step 3 — StepResult + HistoryRecord (`types.go`)

Add to `StepResult`:
```go
Paused       bool   `json:"paused,omitempty"`
PausedReason string `json:"paused_reason,omitempty"`
```

Add to `HistoryRecord`:
```go
PausedFailCount int `json:"paused_fail_count"`
PausedPassCount int `json:"paused_pass_count"`
```

(Two counters: paused checks that passed, paused checks that failed. Both are visible in the report but neither blocks.)

### Step 4 — Execution loop modification (`engine.go` ~lines 247-256)

Replace the current pass/fail branching:

```go
if code == 0 {
    result.Status = "PASS"
    manifest.PassCount++
    printProgress(...)
} else {
    result.Status = "FAIL"
    manifest.FailCount++
    printProgress(...)
}
```

with logic that honors pause:

```go
result.Paused = step.Paused
result.PausedReason = step.PausedReason

if code == 0 {
    result.Status = "PASS"
    if step.Paused {
        manifest.PausedPassCount++
        printProgress(reporter, "[%02d/%02d] PASS %s (paused, %ds)\n", ...)
    } else {
        manifest.PassCount++
        printProgress(reporter, "[%02d/%02d] PASS %s (%ds)\n", ...)
    }
} else {
    result.Status = "FAIL"
    if step.Paused {
        manifest.PausedFailCount++
        printProgress(reporter, "[%02d/%02d] FAIL %s (paused, %ds)\n", ...)
    } else {
        manifest.FailCount++
        printProgress(reporter, "[%02d/%02d] FAIL %s (%ds)\n", ...)
    }
}
```

The aggregate logic at ~line 260 (`if manifest.FailCount == 0 ...`) is unchanged: paused failures don't increment FailCount, so they don't flip status to FAIL.

### Step 5 — Markdown report ("Paused checks" section)

In `buildSummaryMarkdown` (around line 760-770), after the existing summary lines:

```
- Pass: N
- Fail: N
- Skip: N
```

Add:
- `- Paused (pass): N` if `manifest.PausedPassCount > 0`
- `- Paused (fail): N` if `manifest.PausedFailCount > 0`

Then, after the per-step results table, add a new section:

```markdown
## Paused checks

The following checks are flagged `paused: true` in checks.yaml and are
ungated in the aggregate. Their actual result is reported here for
visibility.

| Step | Result | Reason |
|---|---|---|
| <step-id> | PASS / FAIL | <paused_reason> |
```

Only render this section if at least one paused check ran.

### Step 6 — `--no-paused` flag

Plumb a new boolean through `RunOptions` (in `types.go`) and the CLI/RPC entry points:

- Find where `RunOptions` is populated from CLI flags (likely in `holons/clem-ader/api/cli.go` or `internal/server/server.go`).
- Add `--no-paused` flag (default false) that sets `RunOptions.NoPaused = true`.
- In the engine, when `NoPaused` is true, **ignore** the `step.Paused` flag entirely: paused failures count as real failures (`manifest.FailCount++`).
- Document the flag in `holons/clem-ader/README.md` under a "Test pause" section.

### Step 7 — Tests

Add unit tests in `holons/clem-ader/internal/engine/engine_test.go` (or an equivalent file):

1. **Schema validation**: a check with `paused: true` and empty `paused_reason` is rejected.
2. **Pause propagation**: a check with `paused: true, paused_reason: "..."` propagates correctly to the StepSpec.
3. **Aggregate ignores paused failures**: a suite with one normal step (PASS) and one paused step (FAIL) finishes with `FinalStatus = "PASS"`, `FailCount = 0`, `PausedFailCount = 1`.
4. **`--no-paused` enforces**: same suite with `NoPaused = true` finishes with `FinalStatus = "FAIL"`, `FailCount = 1`, `PausedFailCount = 0`.
5. **Markdown report**: paused checks section appears when present, omitted when absent.

### Step 8 — Documentation

- Update [`holons/clem-ader/README.md`](../holons/clem-ader/README.md) with a "Test pause" section linking to the spec.
- Update [`TDD.md`](../TDD.md) with a brief mention: "Individual checks may be flagged `paused: true` for triage; see `docs/specs/ader-test-pause.md`".
- One **worked example**: pick an existing check in `ader/catalogues/grace-op/checks.yaml` that's known to be flaky, flag it `paused: true` with a clear reason and a tracking issue. (Use the SwiftUI composite check that surfaced Issue #25 if it's still in the catalogue and applicable.)

### Step 9 — OP_NEW.md preservation

The composer has a doc at `holons/grace-op/OP_NEW.md` in their local working tree that has never been committed. Since this PR is a feature branch they want to embark with their unstaged modifications, and since OP_NEW.md is the composer's prior work on the `op new --lang` spec:

- Copy `holons/grace-op/OP_NEW.md` from the local working tree into the branch as a separate commit at the start of the work: `docs(op): commit op-new spec from working tree`.
- If the file is not present in the local working tree (user has cleaned it), skip this step — do not invent content. Mention in the PR description that OP_NEW.md was not found locally.

---

## PR conventions

- Branch name: `bpds/ader-test-pause-impl` or similar; the user is composer for this work.
- Base: `master`.
- Title: `feat(ader): per-check pause mechanism`.
- PR description includes: spec link, summary of the 9 steps, the worked example used.
- Commit-by-commit: one commit per logical step (8 commits for the technical work + 1 for OP_NEW.md = 9 total). Squash if the reviewer prefers, but keep the implementation history clean during review.

---

## Operating mode

- Halt at any real doubt: spec ambiguity, an existing repo invariant in tension with the change, a test passing for the wrong reason, ambiguity in how `--no-paused` should interact with bouquets.
- Do not relitigate the spec — it's accepted. If you find a spec contradiction during implementation, halt and report.
- Do not extend the scope (e.g., don't add automatic pause expiry — the spec says no expiry).

## Verification before opening the PR

- `go test ./holons/clem-ader/...` passes.
- A manual ader run with one paused check that fails completes with `FinalStatus: PASS` (status visible in the JSON manifest).
- `ader test ... --no-paused` with the same check completes with `FinalStatus: FAIL`.
- The bouquet report markdown includes the "Paused checks" section.

## Definition of done

- All 9 implementation steps committed.
- Tests green.
- PR opened against `master` with title above.
- Worked example pause documented and tracked.
- The composer admin-merges; chantier closes.

Go.
