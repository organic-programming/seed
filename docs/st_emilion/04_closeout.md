# Post Saint-Émilion close-out plan

Status: plan  
Date: 2026-04-26 (Monday)  
Target activation: after Thursday 2026-04-30 deployment per [`docs/st_emilion/01_runners.md`](01_runners.md)

This document captures the work to close both the Zig SDK chantier and the SDK prebuilts chantier cleanly once popok + winwok are operational at Saint-Émilion.

---

## 1. Pre-flight validation (Thursday end-of-day or Friday morning)

Once `winwok-runner-setup.ps1` has finished and the GitHub Actions runner is registered, verify both runners are healthy before any chantier work depends on them.

```bash
# From the laptop (via WireGuard or local LAN)
gh api repos/organic-programming/seed/actions/runners --jq '.runners[] | {name, status, labels: [.labels[].name]}'
```

Expected output:
```json
{"name": "popok",  "status": "online", "labels": ["self-hosted", "popok",  "macos", "linux-via-docker"]}
{"name": "winwok", "status": "online", "labels": ["self-hosted", "winwok", "windows"]}
```

Smoke each runner with a no-op workflow_dispatch run:

```bash
# Trigger a manual run on each label set; verify they pick up
gh workflow run zig-sdk.yml --ref dev
```

If both runners stay `online` and pick up jobs, proceed.

If either is `offline`, debug per [`docs/st_emilion/01_runners.md`](01_runners.md) §6 (CGNAT, NDK driver, runner service health).

---

## 2. Self-hosted transition for the prebuilts chantier

By Thursday, Codex will likely be mid-chantier (somewhere between Phase 3 and Phase 6). All workflows currently use GitHub-hosted runners per the transitional reality at kickoff. The transition is a small follow-up PR.

### 2.1 Locate the workflows to update

The prebuilts chantier introduced (as of Phase 3+):

- `.github/workflows/sdk-prebuilts.yml` — top-level orchestration
- `.github/workflows/_sdk-prebuilt-target.yml` — reusable per-target

Both should have a parameterised `runs-on:` per the prompt's instruction. Verify by reading the YAMLs.

### 2.2 Swap `runs-on:`

Edit `_sdk-prebuilt-target.yml` (or wherever the per-target runner is selected). Replace:

```yaml
runs-on: ${{ inputs.runner }}
```

with explicit per-target self-hosted labels matching the spec §6.2 mapping. Concretely, the matrix in `sdk-prebuilts.yml` becomes:

```yaml
matrix:
  include:
    - { target: aarch64-apple-darwin,    runner: '[self-hosted, popok, macos]' }
    - { target: x86_64-apple-darwin,     runner: '[self-hosted, popok, macos]' }
    - { target: x86_64-unknown-linux-gnu,  runner: '[self-hosted, popok, linux-via-docker]', container: 'ubuntu:24.04' }
    - { target: aarch64-unknown-linux-gnu, runner: '[self-hosted, popok, linux-via-docker]', container: 'ubuntu:24.04' }
    - { target: x86_64-unknown-linux-musl, runner: '[self-hosted, popok, linux-via-docker]', container: 'alpine:3.19' }
    - { target: aarch64-unknown-linux-musl, runner: '[self-hosted, popok, linux-via-docker]', container: 'alpine:3.19' }
    - { target: x86_64-pc-windows-msvc,  runner: '[self-hosted, winwok, windows]' }
```

7 targets, all on Saint-Émilion infra. Zero GitHub-hosted runner used.

### 2.3 Open the transition PR

```bash
git checkout dev && git pull origin dev
git checkout -b bpds/prebuilts-self-hosted-transition
# Edit the workflow YAMLs
git add .github/workflows/sdk-prebuilts.yml .github/workflows/_sdk-prebuilt-target.yml
git commit -m "ci(prebuilts): swap GitHub-hosted runners for self-hosted popok + winwok

Saint-Émilion infra deployed per docs/st_emilion/01_runners.md. Transition
takes effect from this commit forward; release artifacts retain whatever
runners produced them (no re-build needed)."
git push -u origin bpds/prebuilts-self-hosted-transition
gh pr create --base dev --title "ci(prebuilts): self-hosted runners (popok + winwok)" --body "..."
```

Trigger a workflow_dispatch on the prebuilts workflow against the PR branch as a smoke test before merge. If all 7 targets pass, admin-merge.

### 2.4 First real prebuilts release on Saint-Émilion infra

Once transition is merged, the next merge to `master` of an SDK build script or `Gemfile.lock` will trigger the prebuilts workflow. Watch the first run:

- Total wall time should be ~60-90 min (popok hosts up to 3-4 concurrent matrix jobs; winwok handles its 1 job in parallel).
- Each artifact appears in the run's downloaded artifacts.
- On merge to master, the promotion job creates the GitHub Release with the artifacts.

Verify the release: assets present per spec §6.1 (`<sdk>-<version>-<target>.tar.gz` + `.sha256` + `.spdx.json` + `release-manifest.json`).

If the first release goes through cleanly, the transition is complete.

---

## 3. Zig SDK chantier — close-out

The Zig SDK chantier merged P2 through P12 on `dev` between 2026-04-25 and 2026-04-26. Several items deserve formal close-out.

### 3.1 Issue #25 (ader workspace mirror cleanup)

Root cause: `prepareWorkspaceMirror` in `ader/catalogues/grace-op/integration/runtime.go` uses `copyTree` (in `holons/clem-ader/internal/engine/engine.go`) which does not delete destination files absent from source. Stale state from older runs persists across re-runs.

Two acceptable resolutions:

- **(A) Fix in place, small chantier**: change `copyTree` to delete destination entries not present in source (rsync `--delete` semantics). Side effects: must verify nothing in the mirror downstream depends on stale state being preserved. Estimated: 1 PR, ~50-100 lines, ~1 day.
- **(B) Allocate a fresh mirror per run via `os.RemoveAll`**: simpler, avoids changing copyTree semantics. Side effect: slower first run if the mirror is large.

Recommend (B): single line change in `prepareWorkspaceMirror` to `os.RemoveAll(root)` before `MkdirAll(root)`. Trivial PR, cannot break other consumers.

### 3.2 Cross-smoke target gap (5 of 10 covered)

P12 covered: `aarch64-linux-musl`, `x86_64-linux-musl`, `x86_64-windows-gnu`, `aarch64-ios`, `aarch64-linux-android`.

Missing from the original P12 prompt: `x86_64-unknown-linux-gnu`, `aarch64-unknown-linux-gnu`, `x86_64-pc-windows-msvc`, `aarch64-apple-ios-sim`, `x86_64-linux-android`.

These will be covered naturally by the prebuilts chantier (which has them in its target matrix per spec §6.2) once popok + winwok host the prebuilds. **No standalone follow-up needed for the Zig SDK chantier.** When the prebuilts chantier ships zig-holons-v0.1.0 with all 7 T0+T1 targets, the gap is closed by side-effect.

### 3.3 Out-of-phase `.github/workflows/ader.yml` edits

Two edits landed in `ader.yml` during the Zig chantier (Phase 2 cache-key glob fix, Phase 9 toolchain bootstrap), per spec §6.2 these were transitional and should consolidate into `.github/workflows/zig-sdk.yml`. The Zig P12 PR partially did this. Cleanup PR optional.

If a polish PR is desired:
- Move the toolchain install (Zig + Ninja + submodule init) from inline in `ader.yml` to a reusable composite action under `.github/actions/setup-zig-sdk/`.
- `ader.yml` references the composite action when the bouquet needs the Zig holon.
- `zig-sdk.yml` references the same composite action for its own jobs.
- Eliminates duplication, reduces maintenance.

Estimate: 1 PR, ~100 lines.

### 3.4 `op new --lang zig` template

Per the prompt, `holon-zig` template was deferred until the cross-language `op new --lang <lang>` spec lands. That spec is partially in `holons/grace-op/OP_NEW.md` — composer-owned, not yet finalised.

**No action this week.** Track in a "follow-up after `op new` spec" note.

### 3.5 Documentation polish

After the prebuilts chantier closes:

- Update `INDEX.md` to mark `sdk/zig-holons` as ✅ (currently unmarked or ?).
- Add a "Zig SDK delivery summary" section to `sdk/zig-holons/README.md` listing the 11 PRs that delivered it (P2 through P12).
- Resolve any lingering doc-vs-code divergences flagged in [`docs/audit/sdk-toolchain-audit.md`](audit/sdk-toolchain-audit.md) §5.4 (Swift platforms, runner list, C SDK transport row).

Estimate: 1 PR, doc only, < 50 lines.

---

## 4. Prebuilts chantier — finalization

The prebuilts chantier ships in 8 phases beyond M0. By Thursday Codex should be roughly at Phase 3-5. After Thursday + transition, the remaining phases land naturally.

### 4.1 Verify all 4 SDKs deliver releases

Once Phases 3-6 (zig, cpp, c, ruby) merge, each will trigger a release tag. Confirm:

```bash
gh release list --repo organic-programming/seed | grep -E '(zig|cpp|c|ruby)-holons-v'
```

Expected: 4 releases, each with 7 target artifacts + manifests + SBOMs.

For each release, sanity-check:

```bash
# Verify the release manifest is valid
gh release view zig-holons-v0.1.0 --json assets --jq '.assets[].name'
# Should show: zig-holons-v0.1.0-<target>.tar.gz + .sha256 + .spdx.json for each of 7 targets, plus release-manifest.json

# Sample install on a fresh machine (or VM)
op sdk install zig --version 0.1.0
op sdk verify zig
```

### 4.2 First real holon build using prebuilts

Add `requires.sdk_prebuilts: ["zig"]` to a holon's manifest (e.g., `examples/hello-world/gabriel-greeting-zig/api/v1/holon.proto`) and run:

```bash
op build gabriel-greeting-zig
```

Expected: `op build` detects the requirement, calls `op sdk path zig`, sets `OP_SDK_ZIG_PATH`, and the build links against the prebuilt instead of compiling vendored sources. Should complete in seconds vs. the original 10-15 min cold build.

If the prebuilts subsystem is functioning, this is the user-visible win.

### 4.3 Audit the dead-code Phase 0 verification

Phase 0 of the prebuilts chantier removed 8 dead-code items per the audit. Spot-check that no regression happened:

```bash
op build ./examples/hello-world/gabriel-greeting-{rust,dart,c,cpp,python,kotlin,java}
```

All should pass. If any fail because a Phase 0 deletion was over-aggressive, file a fix-up PR.

### 4.4 Final docs and INDEX promotion

Phase 8 of the prebuilts chantier handles this, but verify:

- `INDEX.md` has an `op sdk` row under `op CLI` with status ✅
- `OP_BUILD.md` references the prebuilts preflight check
- `sdk/README.md` mentions the prebuilts strategy
- Each affected SDK's README documents `op sdk install <lang>` as the primary install path on Apple Silicon + Alpine

If anything is missing, a small doc PR closes it out.

---

## 5. Optional follow-up chantiers (post-close-out)

These are not required to declare both chantiers done, but track them in a backlog:

| Chantier | Trigger | Effort |
|---|---|---|
| **P13 ader workspace cleanup** (Issue #25) | Whenever the SwiftUI composite test starts failing again | 1 PR, ~1 day |
| **Reusable composite action for Zig setup** | When the next CI cleanup pass happens | 1 PR, ~1 day |
| **Python prebuilts (v1.1)** | If Alpine grpcio-from-source pain materialises | ~3 days, full phase |
| **`holon-zig` template** | After `op new --lang <lang>` spec finalises | ~1 day |
| **T2 mobile prebuilts (v1.5)** | When SwiftUI + Flutter hardened builds need them on real devices | ~1 week |
| **Cosign keyless signing (v1.0)** | When external consumers start using prebuilts publicly | ~3 days |
| **Reproducible builds audit** | At v1.0 milestone | ~3 days |
| **Mirror prebuilts to OCI registry** | If GitHub Releases asset size becomes a problem | ~2 days |

---

## 6. Both chantiers declared done — checklist

Both chantiers can be marked ✅ when:

- [ ] popok + winwok online with correct labels per `docs/st_emilion/01_runners.md` §6.
- [ ] Self-hosted transition PR for prebuilts merged.
- [ ] First successful prebuilts release on Saint-Émilion infra (4 SDKs × 7 targets = 28 artifacts).
- [ ] `op sdk install zig` verified end-to-end on a fresh machine.
- [ ] At least one holon (e.g., `gabriel-greeting-zig`) builds via `op sdk install` instead of vendored from source.
- [ ] `INDEX.md` reflects the new state: Zig SDK ✅, op sdk ✅.
- [ ] Issue #25 either resolved or formally deferred with a tracking note.
- [ ] No open PR from either chantier remaining stale > 1 week.

When all 8 boxes are ticked, the two chantiers are closed. Time-boxing: aim for end-of-week of 2026-05-09 (≈ 2 weeks after Saint-Émilion deployment).
