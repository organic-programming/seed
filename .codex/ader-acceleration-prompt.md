# Codex prompt — ader CI acceleration chantier (phases A2 → C)

> Fresh Codex session. Self-contained. Phase A1 shipped in PR #59
> (workspace mirror cleanup, Issue #25 closed). Five phases remain.

---

## Mission

Ship phases A2, A3, B1, B2, C of [`docs/specs/ader-acceleration.md`](../docs/specs/ader-acceleration.md).

**Wall-clock targets on warm popok cache:**
- `local-dev` bouquet ≤ 60 min (vs ~3-4 h baseline)
- `cross-platform` bouquet ≤ 6 h (vs 15+ h baseline)

Sequential PRs against `master`. Composer admin-merges fast on green CI **and** measured target hit. All written artifacts in English.

---

## Context discipline — non-negotiable

The previous session hit `Codex ran out of room in the model's context window`. Do not repeat that failure mode.

- Do **not** read whole files when grep / focused read suffices. Use offset+limit on Read.
- Do **not** paste large logs (build output, test verbose output, full diffs) back into your context — extract the few lines that matter, discard the rest.
- Do **not** re-read a file you already have in context.
- If a file is > 500 lines and you need part of it, locate the relevant region with grep first, then read only that region.
- Keep your scratchpad / planning text terse. Implementation details belong in code, not in prose to yourself.
- Between phases, drop everything you no longer need (file contents, intermediate analysis). Carry forward only the spec + invariants + the next phase's exit criterion.

---

## Read first (focused, not full)

1. [`docs/specs/ader-acceleration.md`](../docs/specs/ader-acceleration.md) — phases A2 onward; skip the A1 section (shipped).
2. [`CLAUDE.md`](../CLAUDE.md) — repo invariants, "doubt is the method", PRs target `master`.
3. [`holons/grace-op/OP_BUILD.md`](../holons/grace-op/OP_BUILD.md) — recipe runner spec, `copy_all_holons` step, `OP_HOLON_<SLUG>_PATH` injection.
4. [`.github/workflows/ader.yml`](../.github/workflows/ader.yml) — bouquet workflow, current cache layout.
5. [`holons/grace-op/internal/holons/runner_registry.go`](../holons/grace-op/internal/holons/runner_registry.go) — recipe runner (Phase B1 lives here).
6. [`holons/grace-op/internal/holons/lifecycle.go`](../holons/grace-op/internal/holons/lifecycle.go) — `stepCopyAllHolons`, composite loop, `dependencyIsFresh` (Phase B2 cache reuse must respect this contract).

PR #58 sets the hard precedent for the "materialize the .holon at the standard local path" contract referenced in B2.

---

## Composer decisions (immutable)

Resolved arbitrations from spec §7. Do not relitigate.

1. **Default recipe parallelism**: `runtime.NumCPU() / 2`, env override `OP_BUILD_PARALLELISM`. Document in `OP_BUILD.md`.
2. **B2 source-content hash inputs**: manifest hash + `op` binary sha256 + Go toolchain version + `op sdk path <lang>` output for each `sdk_prebuilts` dep. **Exclude** OS minor version.
3. **Cache eviction**: nightly `.github/workflows/cache-prune.yml` deletes entries older than 7 d via `gh actions-cache delete --all`. Budget 10 GB; stay under 8.
4. **Prebuilts coordination**: B2 hash includes `op sdk path <lang>`. If that errors (no prebuilt installed), exclude the lang from the hash and emit a warning — don't fail the build.

---

## Iterate until target met

Each phase has a quantitative exit criterion. **Do not advance if missed.** Loop: re-measure precisely → profile what's slow → identify the next bottleneck → adjust → re-measure → repeat.

**Zero-regression contract**: every commit keeps the ader smoke profile green. If your phase change breaks an unrelated test, back off and find a non-breaking path.

NOT blockers (keep iterating): target missed by < 20% on first attempt; a test you wrote fails; CI flake; two equally-defensible designs (pick the simpler).

True blockers (halt + report): spec ambiguity with no defensible default; existing invariant in fundamental tension with phase needs; ≥ 10 iterations missing the target with distinct hypotheses each measured.

---

## Phases

### A2 — xcodebuild DerivedData cache

Add `actions/cache@v4` block in `ader.yml` covering `~/Library/Developer/Xcode/DerivedData/`, `~/Library/Caches/org.swift.swiftpm/`, `${{ runner.workspace }}/**/.build/`. Key: hash of `Package.resolved` + `Package.swift` files + `xcodebuild -version` output.

**Exit (measured)**: SwiftUI composite build ≥ **40% faster** on warm cache. Capture before/after numbers in PR body. If missed, expand cache scope (ModuleCache, signed framework caches) and re-measure.

### A3 — SPM/Gradle/Dart/npm cache key tightening

Add per-ecosystem `actions/cache@v4` blocks: Gradle (`~/.gradle/caches/`), SPM (verify A2 covers), Dart pub (`~/.pub-cache/`), npm (`~/.npm/`), alongside the existing Ruby gemfile cache. Per-ecosystem key hashes the relevant lockfile.

**Exit (measured)**: cold-runner bouquet ≥ **30% faster**; warm popok unchanged or marginally better.

### B1 — parallel COAX recipe member builds

Add `parallel: true` field to the `build_member` recipe step (proto + bindings + manifest validation). Implement the parallel scheduler in `runner_registry.go`: when set, schedule independent members concurrently up to `OP_BUILD_PARALLELISM`. Members declaring `depends_on:` run after their deps.

Update `gabriel-greeting-app-flutter` and `gabriel-greeting-app-swiftui` recipe manifests to use `parallel: true` for independent members. Document the field in `OP_BUILD.md`. `go test -race` clean. Add a recipe test with mixed deps + ordering checks.

**Exit (measured)**: COAX composite ≥ **40% faster**. If missed, profile which members serialize unexpectedly (suspects: shared GOCACHE lock, ruby bundle path, disk).

### B2 — source-content hash skip for build_member

Compute SHA-256 of (member source tree minus `.op/` and gen dirs) + the inputs from composer decision #2. Cache the artifact at `.op/build-cache/<member>-<hash>.holon` after a successful build.

**On cache hit, materialize the `.holon` at the standard local path** (`<member>/.op/build/<member>.holon/`) by symlink or copy. **Hard contract** — `copy_all_holons` and `OP_HOLON_<SLUG>_PATH` consumers depend on it. PR #58 sets the precedent; don't break it.

Add `--no-cache` flag to `op build`. Tests: cache hit; cache miss on source change; cache miss on prebuilt version bump; `--no-cache` flag; **end-to-end that a cache-hit `build_member` produces a workable `.holon` at the standard local path that `copy_all_holons` finds**. Document in `OP_BUILD.md`.

**Exit (measured)**: repeat ader run on unchanged sources ≥ **60% faster**. If missed, profile (hash compute time, copy/symlink overhead).

### C — documentation + benchmark report

- Add `.github/workflows/ader-bench.yml`: runs `local-dev` bouquet, captures wall-clock per check, uploads timing artifact.
- Update `INDEX.md` with the new spec entry and chantier close-out.
- Update `OP_BUILD.md` with `parallel:`, `--no-cache`, `OP_BUILD_PARALLELISM`.
- Commit `docs/benchmarks/ader-acceleration-2026-04.md` comparing baseline vs final wall-clocks for both bouquets.

**Exit (measured)**: all five prior phase targets met and reflected in the report. If any was missed, go back and finish that phase first — do not declare the chantier done.

---

## Operating mode — overnight marathon

Run continuously, including overnight. Do not pause for "is this OK ?". Do not soft-fail on phase N then move to N+1; each phase blocks the next. Composer is asleep — push through on green CI + measured target hit.

---

## Hard constraints

- PRs target `master` directly.
- 10 iterations max on a single root-cause blocker before halting.
- Do not change bouquet/catalogue/suite/check schemas (`checks.yaml`, `suites/`, `bouquets/`).
- Do not change per-language SDK runners (separate chantier).
- Do not eliminate xcodebuild as the macOS composite driver.
- Do not implement distributed ader (multi-machine bouquet split) — out of scope.
- Do not commit transient files in `.codex/` or `.bpds/`.
- B2 must materialize the cache-hit `.holon` at the standard local path. Non-negotiable.

---

## Reporting cadence

After each phase merges, reply with: PR URL, commit range, `git diff --stat`, exit-criteria checklist (each ✅ or ⚠️ with measured number), wall-clock before/after, iteration log if you needed > 1 attempt. Then start the next phase immediately.

After Phase C merges (final): list of all 5 PR URLs, benchmark report URL, updated `INDEX.md` reference, **table comparing baseline vs final wall-clocks for both bouquets with absolute numbers**. Halt — chantier closed.

If you halt mid-chantier: phase, blocker classification, every iteration tried with measured outcome, reproduction steps, recommendation (do not act on it without composer approval).

---

## Definition of done

- 5 PRs merged (A2, A3, B1, B2, C).
- `local-dev` bouquet ≤ 60 min on warm popok — **measured**.
- `cross-platform` bouquet ≤ 6 h on warm popok — **measured**.
- `ader-bench.yml` in place.
- `OP_BUILD.md` documents `parallel:`, `--no-cache`, `OP_BUILD_PARALLELISM`.
- `INDEX.md` reflects close-out.
- Benchmark report committed.

Go.
