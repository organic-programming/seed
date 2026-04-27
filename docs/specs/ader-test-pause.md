# ader test pause — Specification

Status: spec — pending composer approval  
Date: 2026-04-27  
References: [`docs/specs/ader-acceleration.md`](ader-acceleration.md), [`holons/clem-ader/README.md`](../../holons/clem-ader/README.md), [`TDD.md`](../../TDD.md)

This spec adds a "pause" mechanism to ader checks so that individual checks can be flagged as "non-blocking under analysis" without losing visibility. The principle:

> **In R&D, a single failing test should not block a release if the test itself is suspected to be poorly designed, premature, or testing the wrong thing.**

This is orthogonal to:
- The lane mechanism (progression / regression) which is about test maturity over time.
- The profile mechanism (smoke / full) which is about coverage breadth.
- The `continue_on_error` matrix flag in GitHub Actions which is about platform/runner gating.

Test pause is at the **check level**, expressing "this individual check is currently under analysis; surface its result but don't block on it".

---

## 1. Motivation

The seed project is in deep R&D elaboration (per `CLAUDE.md` and `WORKFLOW.md`). Tests are expected to evolve, drift, occasionally regress without indicating real defects. A few patterns where pause is the right tool:

- A check fails because the assertion was correct yesterday but the underlying behaviour deliberately changed (the check needs an update, not the implementation).
- A check is timing-sensitive and flakes on popok under load.
- A check was added prematurely; the feature it covers is still being designed.
- A check exposes a latent bug that the composer wants to triage at their pace, not block CI.

In all these cases, marking the check `paused` for some duration:
- Keeps the check running (so we keep collecting signal).
- Reports its result in the bouquet report (visible, not hidden).
- Does NOT cause the suite/profile/bouquet to fail because of it.
- Has a `reason` field so future readers (and the composer) understand why.

This is a **temporary** state. A paused check is a TODO with explicit visibility. The expectation is that the composer or an agent revisits it.

## 2. Decision

Extend the `checks.yaml` schema with two optional fields per check:

```yaml
checks:
  some-flaky-check:
    workdir: ...
    command: ...
    prereqs: [...]
    description: ...
    timeout: 5m

    # New optional fields:
    paused: true                # boolean, default false
    paused_reason: |            # required when paused=true
      Test is timing-sensitive on popok under heavy CI load.
      Re-evaluate after ader-acceleration Phase B1 lands.
      Tracking issue: #N
```

Behaviour when `paused: true`:

- The check is still selected by the suite if the lane/profile would normally select it.
- The check still runs and emits its output to the bouquet report.
- The check's pass/fail status is recorded in the report.
- The suite/profile/bouquet aggregate result **ignores** paused checks for failure decision-making.
- The bouquet report includes a "Paused" section listing all paused checks, their reason, and their actual result (pass/fail/skipped).

Implementation:

- ader engine reads the new fields from `checks.yaml`.
- The aggregate pass/fail logic in `holons/clem-ader/internal/engine/engine.go` filters out paused checks before deciding suite outcome.
- The report formatter adds a "Paused checks" section.
- `ader test ... --no-paused` is a new flag that tells ader to enforce paused checks too (for when the composer wants to see what would happen if pause were lifted).

## 3. Operational rules

- **A check enters pause** by editing `checks.yaml` and setting `paused: true` with a `paused_reason`. This is a normal PR.
- **A check exits pause** by removing the two fields. Also a PR.
- **Pause has no expiry** (no automatic timeout). The composer/agents are responsible for revisiting paused checks. A scheduled workflow may report "paused checks > 30 days" as a soft alert.
- **Paused checks must keep running** — never use pause to silence a check's output, only to ungate the suite.

## 4. Interaction with existing mechanisms

| Mechanism | Scope | Effect on suite outcome |
|---|---|---|
| **Lane** (`progression` / `regression`) | All checks | Profile-driven selection |
| **Profile** (`smoke` / `full`) | Bouquet-driven | Selection only |
| **`paused: true`** (new) | Per-check | Runs + reports, ignored in aggregate |
| GitHub Actions `continue-on-error` | Per-job | Ungates the workflow run |

These compose cleanly. A check can be `lane: regression, paused: true` — it would normally be required, but is currently ungated. When pause lifts, it returns to required.

## 5. Phasing

A small chantier, ~3 days agent time:

| Phase | Scope | Effort |
|---|---|---|
| **TP-1** | Extend `checks.yaml` schema in `holons/clem-ader/internal/engine/`. Add fields, parsing, validation. | 1 day |
| **TP-2** | Update aggregate pass/fail logic to ignore paused checks. Add "Paused checks" section to bouquet report. Add `--no-paused` flag. | 1 day |
| **TP-3** | Documentation (`TDD.md`, `holons/clem-ader/README.md`, `OP_BUILD.md` if relevant). One example pause in an existing check to demonstrate. | 1 day |

Could be folded into the ader-acceleration chantier as Phase A0 (before the workspace mirror cleanup), or shipped independently as its own short chantier.

## 6. Acceptance

The spec is accepted when:

- `checks.yaml` accepts `paused: true` + `paused_reason: <text>` per check.
- A paused check's failure does NOT cause its suite/profile/bouquet to fail.
- The bouquet report has a clear "Paused checks" section listing reason + actual result.
- `ader test ... --no-paused` enforces paused checks for one-off audits.
- Documentation updated in `TDD.md` and `holons/clem-ader/README.md`.
- At least one real check is paused as a worked example, with a tracking issue.

## 7. Why not just delete or comment out failing checks?

Three reasons:

1. **Visibility.** Deleted checks vanish from the bouquet report; paused checks stay visible. The composer should always know what's been deferred.
2. **Reversibility.** Pause is one line to flip; deletion needs a re-add. R&D iteration favours quick toggles.
3. **Continuous signal.** Even a paused check may pass intermittently. That signal is useful — it tells you whether the underlying issue is fixable, regressing, or stable. Deleted checks tell you nothing.

Pause is the cheapest mechanism that preserves observability.
