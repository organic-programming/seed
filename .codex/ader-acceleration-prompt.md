# Codex prompt — implement ader CI acceleration chantier

> Hand this prompt to Codex. Self-contained: Codex has filesystem access to the `seed` repo and must read the referenced files to learn the conventions. Do not assume Codex has seen the planning conversation that produced this prompt.

---

## Mission

Implement the ader CI acceleration chantier specified in [`docs/specs/ader-acceleration.md`](../docs/specs/ader-acceleration.md). The chantier brings the standard `local-dev` ader bouquet wall-clock from ~3-4 hours to ≤ 60 minutes, and the `cross-platform` bouquet from 15+ hours to ≤ 6 hours, on warm popok cache.

The chantier is **complementary to the prebuilts chantier** (which addresses native compile time). This chantier addresses ader's own architectural costs:

- workspace mirror cleanup (Issue #25)
- xcodebuild / SPM / Gradle / Dart / npm cross-run caching
- parallel COAX recipe member builds
- source-content hash skip for unchanged members

5 phases, ~10 days agent time.

All written artifacts in English.

---

## Kickoff gate — do not start until prebuilts chantier is done

This chantier touches `holons/grace-op/internal/holons/lifecycle.go` (Phase B1) and `runner_registry.go` (Phase B1). The prebuilts chantier also touches both files. To avoid rebase conflicts, **wait until the prebuilts chantier has shipped its Phase 8 PR and merged onto `dev`**.

Verify before starting:

```bash
git fetch origin dev
gh release list | grep -E 'zig-holons-v|cpp-holons-v|c-holons-v|ruby-holons-v'
```

You should see the four prebuilts releases (zig, cpp, c, ruby). If not, halt and wait — the prebuilts chantier is still in flight.

---

## Required reading (do not skip)

1. [`docs/specs/ader-acceleration.md`](../docs/specs/ader-acceleration.md) — the spec, all 9 sections.
2. [`docs/specs/sdk-prebuilts.md`](../docs/specs/sdk-prebuilts.md) — the prebuilts spec (Phase B2 hash key depends on prebuilts version pins).
3. [`CLAUDE.md`](../CLAUDE.md) — repo invariants, "doubt is the method", PRs target `dev`.
4. [`holons/clem-ader/README.md`](../holons/clem-ader/README.md) — ader concepts, locks, bouquets.
5. [`ader/catalogues/grace-op/integration/runtime.go`](../ader/catalogues/grace-op/integration/runtime.go) — `prepareWorkspaceMirror` (line ~635), the source of Issue #25.
6. [`holons/clem-ader/internal/engine/engine.go`](../holons/clem-ader/internal/engine/engine.go) — `copyTree` (line ~1395), `snapshotWorkspace` (line ~510).
7. [`holons/grace-op/OP_BUILD.md`](../holons/grace-op/OP_BUILD.md) — runner semantics, recipe runner spec.
8. [`holons/grace-op/internal/holons/runner_registry.go`](../holons/grace-op/internal/holons/runner_registry.go) — the `recipe` runner (Phase B1).
9. [`.github/workflows/ader.yml`](../.github/workflows/ader.yml) — current bouquet workflow, cache structure.
10. Issue #25 — workspace mirror bit-rot tracking.

---

## Composer decisions (resolved, treat as immutable)

The 5 questions in spec §7 have been arbitrated. Treat each as binding constraint, not recommendation:

1. **Default recipe parallelism**: `runtime.NumCPU() / 2`, env-overridable via `OP_BUILD_PARALLELISM`. Document the env var in `OP_BUILD.md`.
2. **Source-content hash inputs (Phase B2)**: manifest hash + `op` binary hash (sha256 of installed `op` binary) + Go toolchain version + `op sdk path <lang>` output for any sdk_prebuilts dependency. **Exclude** OS minor version (too fragile, makes cache thrash).
3. **GitHub Actions cache eviction**: a scheduled workflow `.github/workflows/cache-prune.yml` runs nightly, deleting cache entries older than 7 days using `gh actions-cache delete --all`. Cache budget per repo is 10 GB; we want to stay under 8 GB.
4. **Coordination with prebuilts**: Phase B2 hash explicitly includes `op sdk path <lang>` output. When prebuilts version bumps, all consumer holons cache invalidates correctly. If `op sdk path <lang>` errors (no prebuilt installed), exclude that lang from the hash and emit a warning.
5. **Issue #25 fix scope**: `os.RemoveAll(root)` before `MkdirAll(root)` in `prepareWorkspaceMirror`. Simple, correct, low-risk. Full rsync-with-delete is v2 if I/O cost on popok APFS becomes observable (it shouldn't — APFS handles `RemoveAll` of GB-scale trees in seconds).

Do not relitigate these. If implementation surfaces a contradiction, halt and report.

---

## Phasing

Sequential PRs against `dev`. One PR per phase. Same operating mode as the prebuilts chantier: continuous progression (composer admin-merges fast), halt only at real doubts.

### Phase A1 — Issue #25 workspace mirror cleanup
- Edit `prepareWorkspaceMirror` in `ader/catalogues/grace-op/integration/runtime.go` to `os.RemoveAll(root)` before `MkdirAll(root)`.
- Add a regression test in `runtime_test.go` (or wherever fits) that creates a fake stale file in the mirror, runs prepareWorkspaceMirror, and asserts the stale file is gone.
- Verify by running `go test ./ader/catalogues/grace-op/integration/...` and the full ader smoke profile end-to-end (should pass cleanly without the SwiftUI duplicate that blocked Zig P12).
- Close Issue #25 in the PR description.
- **Exit:** mirror is provably clean per run; smoke green.

### Phase A2 — xcodebuild DerivedData cache
- Add a cache block in `.github/workflows/ader.yml` for `~/Library/Developer/Xcode/DerivedData/`, `~/Library/Caches/org.swift.swiftpm/`, `${{ runner.workspace }}/**/.build/`.
- Cache key includes `Package.resolved`, `Package.swift` files, and `xcodebuild -version` output.
- Capture wall-clock for SwiftUI composite build before/after; report in PR.
- **Exit:** SwiftUI composite build wall-clock cut by ≥ 40% on warm cache.

### Phase A3 — SPM/Gradle/Dart/npm cache key tightening
- Add explicit `actions/cache@v4` blocks for Gradle, SPM, Dart pub, npm, alongside the existing Ruby gemfile cache. Per-ecosystem keys hashing the relevant lockfiles.
- Each cache restored before the bouquet runs; populated after.
- **Exit:** cold-runner bouquet wall-clock cut by ≥ 30%; warm-popok unchanged or marginally better.

### Phase B1 — parallel COAX recipe member builds
- Add `parallel: true` field to the `build_member` recipe step (proto + bindings + manifest validation).
- Implement the parallel scheduler in the recipe runner (`runner_registry.go`): when `parallel: true`, schedule independent member builds concurrently up to `OP_BUILD_PARALLELISM` (default `runtime.NumCPU() / 2`). Members declaring `depends_on:` other members run after their dependencies.
- Update existing COAX recipe manifests (gabriel-greeting-app-flutter, gabriel-greeting-app-swiftui) to use `parallel: true` for independent members.
- Document the new field in `OP_BUILD.md`.
- Tests: add a recipe with mixed dependencies + intentional ordering checks.
- **Exit:** COAX composite wall-clock cut by ≥ 40%.

### Phase B2 — source-content hash skip for build_member
- Implement the hash logic in the recipe runner: compute SHA-256 of (member source tree minus `.op/` and gen dirs) + manifest hash + `op` binary hash + Go toolchain version + `op sdk path <lang>` output for sdk_prebuilts deps.
- Cache the artifact at `.op/build-cache/<member>-<hash>.holon` after a successful build.
- On subsequent recipe runs, if the hash matches an existing cache entry, skip the rebuild and reuse the cached `.holon`.
- Add `--no-cache` flag to `op build` for forced rebuild.
- Tests: cache hit, cache miss on source change, cache miss on prebuilt version bump, `--no-cache` flag.
- Document in `OP_BUILD.md`.
- **Exit:** repeat ader run on unchanged sources cut by ≥ 60%.

### Phase C — documentation + benchmark report
- Add `.github/workflows/ader-bench.yml`: runs the standard `local-dev` bouquet, captures wall-clock per check, uploads timing report as workflow artifact.
- Update `INDEX.md` with the new spec and the chantier close-out.
- Update `OP_BUILD.md` with `parallel:`, `--no-cache`, and `OP_BUILD_PARALLELISM`.
- Generate a final benchmark report comparing bouquet wall-clocks before vs after this chantier; commit under `docs/benchmarks/ader-acceleration-2026-04.md`.
- **Exit:** all docs reflect the new state; benchmark report shows the targeted reductions.

---

## Operating mode

Same as the prebuilts chantier: continuous progression with composer admin-merging each phase PR within minutes. Do not halt at merge gates when confidence is high.

Halt and report at the first real doubt per [`CLAUDE.md`](../CLAUDE.md) "doubt is the method":
- Spec ambiguity that materially changes implementation.
- Existing repo invariant in tension with the phase's needs.
- Test passes but you are not sure it covers the spec's intent.
- Doc-vs-code-vs-proto drift on a point this phase touches.
- Library/version pin choice that benefits from explicit composer arbitration.
- Merge conflict resolution requiring a non-trivial judgment call.

---

## Constraints in force throughout

- PRs target `dev`, never `master` directly.
- Same loop policy as prebuilts (10 iterations max on a single root-cause blocker).
- Do not break existing tests. Every phase keeps the bouquet green; coverage parity is non-negotiable.
- Do not change the bouquet/catalogue/suite/check schemas (`checks.yaml`, `suites/`, `bouquets/`).
- Do not change per-language SDK runners (those land in their own chantiers).
- Do not eliminate xcodebuild as the macOS composite driver.
- Do not implement distributed ader (multi-machine bouquet split) — out of scope.
- Do not commit `.codex/observability-impl.md` deletion that may show up in worktrees.

---

## Reporting cadence

After each phase merges:
- Reply with: PR URL, commit SHA range, `git diff --stat`, exit-criteria checklist (each ✅ or ⚠️), wall-clock measurement before/after, non-trivial decisions with 2-3 line rationale.
- Then immediately start the next phase. Do not wait for acknowledgement.

After Phase C merges (final delivery):
- Reply with: list of all 6 PR URLs, the final benchmark report URL, the closed Issue #25 link, updated `INDEX.md` reference.
- Halt — chantier closed.

If you halt mid-chantier:
- Reply with: phase, blocker classification, everything tried, reproduction steps, recommendation for resolution (do not act on it without composer approval).

---

## Definition of done

- All 6 phase PRs merged.
- `local-dev` bouquet wall-clock ≤ 60 min on warm popok (vs ~3-4 h baseline).
- `cross-platform` bouquet wall-clock ≤ 6 h on warm popok (vs 15+ h baseline).
- Issue #25 closed.
- `ader-bench.yml` workflow in place reporting wall-clock per run.
- `OP_BUILD.md` documents `parallel:`, `--no-cache`, and `OP_BUILD_PARALLELISM`.
- `INDEX.md` reflects the close-out.
- A benchmark report committed showing measured improvements.

When this chantier closes, combined with the prebuilts chantier (already done), the cross-platform bouquet should be in the 3-4 h range — practical for full coverage.

Go (after the kickoff gate per §1).
