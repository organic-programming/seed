# ader Acceleration Benchmark

Status: implementation complete, warm popok measurements pending on the stacked
phase PRs.

## Targets

| Bouquet | Warm-cache target | Baseline |
|---|---:|---:|
| `local-dev` | <= 60 min | ~3-4 h |
| `cross-platform` | <= 6 h | 15+ h |

Measurements must use instrumented wall-clock inside the workflow run, not the
GitHub job duration, because popok queue time is expected until the second
runner lands.

## Measurement Workflow

`.github/workflows/ader-bench.yml` runs on `ader-bench/**` tracking branches
and by manual dispatch. It records:

- Zig submodule checkout seconds.
- `op` / `ader` / SDK prebuilt bootstrap seconds.
- `ader test-bouquet ader --name <bouquet>` seconds.

The workflow uploads `ader/reports/bench/<run-id>/timings.tsv` plus the normal
ader report directories. It fails the run when the selected bouquet exceeds the
target threshold (`local-dev` > 3600 s, `cross-platform` > 21600 s).

## Cache Budget

`.github/workflows/cache-prune.yml` prunes GitHub Actions caches older than
7 days nightly with `gh actions-cache delete --all --older-than 7d --confirm`.
This keeps the repository under the 10 GB Actions cache budget while leaving a
warm-cache window for active PRs.

## Phase Summary

| Phase | Change |
|---|---|
| A2 | SwiftUI xcodebuild DerivedData and source-package output cache. |
| A3 | Portable SwiftPM, Dart pub, Gradle, and npm cache restore/save pairs. |
| B1 | Opt-in `parallel: true` recipe `build_member` batches capped by `OP_BUILD_PARALLELISM`. |
| B2 | Content-hash `.holon` package reuse for recipe members with standard local package materialization. |
| C | Benchmark and cache-prune workflows plus close-out documentation. |

Before merge, run both benchmark bouquets on warm popok cache and paste the
`timings.tsv` deltas into the relevant PR descriptions.
