# ader Benchmark Diagnostic Report

Status: reporting workflow implemented; performance targets are intentionally
out of scope for this workflow.

## Purpose

`.github/workflows/ader-bench.yml` is a manual measurement and diagnostic tool
for ader bouquets. It answers:

- what was launched;
- how long GitHub waited before a runner started executing;
- how long the workflow actually executed on the runner;
- which infrastructure blocks passed, failed, or were skipped;
- which bouquet entries passed, failed, or were skipped;
- which internal ader steps failed, with reasons when available and log paths.

It does **not** decide whether a run is fast enough. There are no 5 h, 6 h, or
8 h performance thresholds in the workflow.

## Inputs

| Input | Meaning |
|---|---|
| `bouquet` | `local-dev` or `cross-platform`. |
| `cache_note` | Free-form cache context, for example `warm-popok-cache` or `unspecified`. |
| `run_note` | Free-form operator context, for example `popok queue high`. |

The existing popok cache environment remains in place because it is part of the
normal CI environment. The benchmark records that context but does not force,
clear, prune, or optimize caches.

## Artifacts

Each run uploads `ader/reports/bench/<run-id>/` with:

| File | Contents |
|---|---|
| `summary.md` | Human-readable run identity, wall-clock, outcome, blocks, entries, and failures. |
| `summary.json` | Structured form of the same report. |
| `blocks.tsv` | Workflow blocks with status, exit code, timestamps, seconds, and reason. |
| `bouquet-entries.tsv` | One row per catalogue/suite entry with status, duration, child report, and reason. |
| `failed-steps.tsv` | Internal ader failed steps with catalogue, suite, step id, reason, and log path. |

The workflow also uploads the raw ader bouquet and catalogue reports so failures
can be inspected without re-running the job.

`ader` reports do not always include a machine-readable failure reason. When a
failed bouquet entry or internal step has no `reason`, the benchmark report uses
`see child_report_dir` or `see log_path` so the diagnostic path is still visible.
Bouquet reports are selected only when their `started_at` is at or after the
current workflow's runner-acquired timestamp; this avoids accidentally reporting
entries from an older run when the current workflow fails before creating a new
bouquet report.

## Variance Guidance

One run is an observation, not a stable performance conclusion. popok can vary
under load and GitHub queue time is external to the build. For timing analysis,
compare multiple reports from equivalent refs and cache context, and inspect the
block and bouquet-entry tables rather than relying on a single total duration.

## Outcome Semantics

The report uses `functional PASS` / `functional FAIL` only:

- `PASS`: all infrastructure blocks passed and the bouquet report is passing.
- `FAIL`: an infrastructure block failed, the bouquet command failed, a
  catalogue/suite entry failed, or an internal ader step failed.

Skipped blocks are diagnostic context. They usually mean a prior infrastructure
block failed.

## Comparison Policy

Comparisons are manual and report-to-report. Run the same workflow on two refs,
then compare:

- runner wait;
- executed wall-clock;
- block timings;
- bouquet entry timings;
- failure tables.

The workflow does not compare popok and winwok, does not normalize for hardware,
and does not declare performance pass/fail.
