# ader CI acceleration — Specification

Status: spec — pending composer approval  
Date: 2026-04-26  
References: [`docs/specs/sdk-prebuilts.md`](sdk-prebuilts.md), [`docs/audit/sdk-toolchain-audit.md`](../audit/sdk-toolchain-audit.md), [`holons/clem-ader/README.md`](../../holons/clem-ader/README.md), [`.github/workflows/ader.yml`](../../.github/workflows/ader.yml), Issue #25

This spec defines the next-after-prebuilts chantier: making ader bouquets practical at full coverage. Today the bouquet has a 240-minute job timeout and the full cross-platform profile takes 15+ hours; this spec targets bringing the standard bouquet wall-clock from ~4 hours to ~1 hour while preserving coverage.

The prebuilts chantier (in flight) addresses native cross-compile time. This chantier is complementary: it addresses ader's own architectural costs (workspace mirroring, sequential recipe builds, missing toolchain caches).

---

## 1. Context — what makes ader slow today

Per the toolchain audit and operational observations, the dominant ader costs in order of magnitude:

| Cost | Magnitude | Touched by prebuilts? |
|---|---|---|
| `xcodebuild` for SwiftUI composite (per build) | 15-30 min | ❌ No |
| `flutter build` hardened (per build) | 10-20 min | ❌ No |
| COAX recipe sequential build of 13 holons | 30-60 min | ❌ No |
| ader workspace mirror copy (`prepareWorkspaceMirror`) | 30 sec to 2 min × N tests | ❌ No |
| Workspace mirror bit-rot (Issue #25) | indeterminate, sometimes hours of debug | ❌ No |
| Ruby grpc gem cold install | 5-10 min | ✅ Yes |
| gRPC system install / linking on cold runner | 5-15 min | ✅ Yes |
| Cross-compile vendored gRPC (when in scope) | 10-15 min × N targets | ✅ Yes |
| `op build` of canonical grace-op + clem-ader at bouquet start | 1-2 min each | ❌ No |

The 240-minute `ader.yml` timeout is set by the multiplicative cost of xcodebuild + Flutter + COAX recipe + workspace mirroring across the bouquet matrix. **Prebuilts alone do not make ader fast.**

## 2. Goals

- **Standard bouquet (`local-dev` profile) wall-clock**: ≤ 60 min on warm popok cache (currently ~3-4h).
- **`cross-platform` profile (full coverage)**: ≤ 6 h (currently 15+ h).
- **Per-test workspace mirror cost**: ≤ 30 sec amortised (currently 30 sec to 2 min, with bit-rot risk).
- **Cross-compile bouquet additions**: become viable to add (today they would push timeouts unmaintainable).
- **Coverage parity**: zero loss vs. the current bouquet matrix.

## 3. Non-goals

- Not replacing `clem-ader` or its locking model.
- Not changing the bouquet/catalogue/suite/check schema (`checks.yaml`, `suites/`, `bouquets/`).
- Not eliminating xcodebuild as the macOS composite build driver.
- Not implementing distributed ader (multi-machine bouquet split).
- Not changing the per-language SDK runners.

---

## 4. Scope decomposition

Five focused improvements, each independently mergeable. Prioritised by gain/effort.

### 4.1 Fix Issue #25 — workspace mirror cleanup (Tier A — high gain, low effort)

**Problem**: `prepareWorkspaceMirror` in `ader/catalogues/grace-op/integration/runtime.go:635` calls `copyTree` which does not delete destination files absent from source. Stale state from previous runs accumulates and occasionally produces hard-to-diagnose errors (e.g., the SwiftUI duplicate `CoaxControlsView.swift` that blocked Zig P12 smoke).

**Fix**: prepend `os.RemoveAll(root)` before `MkdirAll(root)` in `prepareWorkspaceMirror`. The runtime cost of full re-copy (~30 sec) is acceptable; the elimination of bit-rot debug time is large.

**Estimate**: 1 PR, ~10 lines + a test, 1 day.

### 4.2 GitHub Actions cache for xcodebuild DerivedData (Tier A)

**Problem**: every CI run of the SwiftUI composite triggers a full xcodebuild pipeline including SPM resolution, Swift compilation, signing. No cross-run cache.

**Fix**: add an `actions/cache@v4` block in `ader.yml` keyed on:
- `Package.resolved` for SPM dependency resolution
- `Package.swift` files in the SwiftUI composite + sdk/swift-holons + organism_kits/swiftui

Cache paths:
- `~/Library/Developer/Xcode/DerivedData/`
- `~/Library/Caches/org.swift.swiftpm/`
- `${{ runner.workspace }}/**/.build/`

Expected wall-clock reduction on warm cache: 50-70% on SwiftUI composite builds (15-30 min → 5-10 min).

**Estimate**: 1 PR, ~30 lines in `ader.yml` + cache validation, 1 day.

### 4.3 Parallel COAX recipe member builds (Tier B — medium gain, medium effort)

**Problem**: `recipe` runner in `holons/grace-op/internal/holons/runner_registry.go` (and lifecycle.go) executes `build_member` steps sequentially. For COAX composites with 5-13 members and no inter-member dependencies, this is a wall-clock multiplier.

**Fix**: introduce a `parallel: true` flag in `build_member` step config (per OP_BUILD.md §recipe). When set, the recipe runner schedules independent member builds concurrently up to `OP_BUILD_PARALLELISM` (default = `runtime.NumCPU() / 2`). Members declaring `depends_on:` other members run after their dependencies.

Composite manifests opt in by adding `parallel: true` to the relevant steps. Default behaviour unchanged for backwards-compat.

Expected wall-clock reduction on COAX composites: 40-60% (30-60 min → 12-25 min).

Risks: race conditions in shared build outputs; mitigation = each member build writes only to its own `.op/` dir (already enforced by `op build` design).

**Estimate**: 1 PR, ~150 lines + tests + sample COAX composites updated, 2-3 days.

### 4.4 Source-content hash skip for build_member (Tier B)

**Problem**: every COAX recipe build rebuilds every member from scratch. Even if the member's source hasn't changed, `op build <member>` re-runs the runner.

**Fix**: extend the recipe runner to compute a content hash of the member's source tree (excluding `.op/`, generated dirs) plus the runner's manifest options. If the hash matches the last successful build's hash (stored in `.op/build-cache/<member>-<hash>.holon`), skip rebuild and reuse the cached `.holon` package.

Cache invalidation: any change to source, manifest, or transitively depended-on artifacts triggers rebuild.

Expected wall-clock reduction on incremental composites: 60-80% on no-source-change runs (which dominate CI for unrelated PRs).

Risks: incorrect cache invalidation produces stale builds; mitigation = include the `op` binary's hash in the cache key (so a `op` upgrade invalidates everything).

**Estimate**: 1 PR, ~200 lines + extensive tests, 3-4 days.

### 4.5 SPM and Gradle cache key tightening (Tier A — medium gain, low effort)

**Problem**: `actions/cache@v4` in `ader.yml:85-91` keys only on Ruby `Gemfile.lock`. Other ecosystems (SPM, Gradle, npm, cargo, dart pub) silently rely on filesystem cache persistence on popok. This works on the self-hosted runner but does not produce GitHub-cache-portable artifacts.

**Fix**: add per-ecosystem cache blocks:

```yaml
- uses: actions/cache@v4
  with:
    path: ${{ env.GRADLE_USER_HOME }}/caches
    key: gradle-${{ runner.os }}-${{ hashFiles('**/*.gradle*', '**/gradle-wrapper.properties') }}
- uses: actions/cache@v4
  with:
    path: ~/Library/Caches/org.swift.swiftpm
    key: spm-${{ runner.os }}-${{ hashFiles('**/Package.resolved') }}
- uses: actions/cache@v4
  with:
    path: ${{ env.PUB_CACHE }}
    key: dart-${{ runner.os }}-${{ hashFiles('**/pubspec.lock') }}
- uses: actions/cache@v4
  with:
    path: ${{ env.npm_config_cache }}
    key: npm-${{ runner.os }}-${{ hashFiles('**/package-lock.json') }}
```

Side benefit: when popok is migrated or replaced (or eventually augmented with ephemeral runners), these caches are portable.

Expected wall-clock reduction on cold-popok or runner-migration scenarios: 30-50%. Marginal on already-warm popok.

**Estimate**: 1 PR, ~50 lines, 1 day.

---

## 5. Phasing

Sequential, one PR per phase. Same loop policy as the prebuilts chantier.

| Phase | Scope | Estimated effort |
|---|---|---|
| **A1** | Workspace mirror cleanup (Issue #25) | 1 day |
| **A2** | xcodebuild DerivedData cache | 1 day |
| **A3** | SPM/Gradle/Dart/npm cache key tightening | 1 day |
| **B1** | Parallel COAX recipe member builds | 2-3 days |
| **B2** | Source-content hash skip for build_member | 3-4 days |
| **C** | Documentation + benchmark report | 1 day |

**Total**: ~10 days of agent time. Tier A first (cumulative gain ~30-40%), then Tier B (additional ~30-40%).

---

## 6. Verification

For each phase:

1. Run the standard `local-dev` bouquet on popok before merge, capture wall-clock.
2. Apply the phase change.
3. Re-run the same bouquet, capture wall-clock.
4. Report the delta in the PR description.
5. Repeat for `cross-platform` profile when relevant.

End-of-chantier acceptance:

- `local-dev` bouquet wall-clock ≤ 60 min on warm popok.
- `cross-platform` bouquet wall-clock ≤ 6 h on warm popok.
- No regression in test coverage (all suites still selected, all checks still asserted).
- Issue #25 closed.

A manual benchmarking workflow (`.github/workflows/ader-bench.yml`) runs a selected bouquet and uploads an objective diagnostic report. It separates runner wait from executed time and reports functional success/failure without enforcing performance thresholds.

---

## 7. Decisions to resolve

These need composer arbitration before implementation:

1. **Default parallelism for recipe builds**: `runtime.NumCPU() / 2` is conservative. Should it be `runtime.NumCPU()` or env-overridable? Recommend env-overridable with sane default.
2. **Source-content hash invalidation**: include `op` binary hash, OS version, runner labels? Recommend: `op` hash + go version + manifest hash. Exclude OS minor version (too fragile).
3. **Cache eviction policy**: GitHub Actions cache has a 10 GB per-repo budget. With xcodebuild DerivedData easily reaching 5 GB, we may need to evict aggressively. Recommend: prune caches older than 7 days via a scheduled workflow.
4. **Compatibility with prebuilts**: when prebuilts ship c/cpp/zig/ruby native libs, the recipe build_member of those holons becomes near-instant. Coordination with prebuilts chantier needed for the Phase B2 hash key (include prebuilts version pins in the hash). Recommend: explicit dependency on `op sdk path <lang>` output in the hash.
5. **Issue #25 fix scope**: the simple `os.RemoveAll` is one option; a true rsync-with-delete is another. Recommend: `os.RemoveAll` for v1, full rsync for v2 if I/O cost on popok HFS+/APFS is observed to matter.

---

## 8. Risks

- **Parallel build races**: COAX recipes that incorrectly assume sequential build order will silently produce different artifacts. Mitigation: explicit `depends_on:` declarations + tests with intentional dependency chains.
- **Hash skip false negatives**: source-content hash misses some modification (e.g., a generated file outside source dirs). Mitigation: extensive integration tests + a `--no-cache` flag for forced rebuild.
- **Cache poisoning**: a stale GitHub Actions cache produces incorrect builds. Mitigation: hash key includes all relevant inputs; emergency `actions/cache/restore` skip via `cache-from: ''` env.
- **xcodebuild cache invalidation surprises**: Apple's tooling sometimes invalidates DerivedData internally on Xcode version change. Mitigation: include `xcodebuild -version` in cache key.
- **Composite holon ordering changes break cached builds**: if the order of members changes, the hash should still be stable (sort members by slug for hashing). Mitigation: documented in B2 implementation.

---

## 9. Acceptance

This spec originally scoped an acceleration chantier with performance targets.
After the C-only pivot, the mergeable acceptance for this branch is diagnostic
coverage only:

- `ader-bench.yml` workflow in place as a manual measurement workflow.
- Report separates runner wait from executed wall-clock.
- Report lists infrastructure block status, bouquet entry status, and failed
  internal steps with log paths.
- Report is uploaded on functional pass and functional fail.
- No performance threshold is enforced by the benchmark workflow.
- `INDEX.md` reflects the diagnostic benchmark protocol.

The original acceleration targets (`local-dev` under 60 min and
`cross-platform` under 6 h) are not validated by this C-only branch. They remain
future optimization criteria that require separate before/after measurements
before any optimization phase is merged.
