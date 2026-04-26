# Codex prompt — implement `op sdk` prebuilts subsystem

> Hand this prompt to Codex. Self-contained: Codex has filesystem access to the `seed` repo and must read the referenced files to learn conventions. Do not assume Codex has seen the planning conversation that produced this prompt.

---

## Mission

Implement the SDK prebuilts subsystem specified in [`docs/specs/sdk-prebuilts.md`](../docs/specs/sdk-prebuilts.md), grounded in the audit at [`docs/audit/sdk-toolchain-audit.md`](../docs/audit/sdk-toolchain-audit.md). Both documents are authoritative — read them in full before any implementation.

Scope: ship prebuilt native runtime libraries / vendored gem bundles for the 4 SDKs whose audit identified substantive cold-build pain — `ruby`, `c`, `cpp`, `zig`. Other SDKs (go, rust, dart, js, js-web, swift, java, kotlin, csharp) are out of scope: they are pure-language or already covered by upstream package managers per the audit §2.

Non-goal: do not modify the Zig SDK chantier in flight (PRs against `dev` from `codex/zig-sdk-p*` branches). The Zig prebuilts work begins after that chantier reaches P12 and merges to `dev`. Coordinate timing with the composer.

All written artifacts in English.

---

## Required reading (do not skip)

1. [`docs/audit/sdk-toolchain-audit.md`](../docs/audit/sdk-toolchain-audit.md) — establishes the why and the per-SDK pain.
2. [`docs/specs/sdk-prebuilts.md`](../docs/specs/sdk-prebuilts.md) — establishes the design. Read all 12 sections; §11 lists open questions that need composer arbitration before they are settled in your ADRs.
3. [`CLAUDE.md`](../CLAUDE.md) — repo invariants, "doubt is the method", PRs target `dev`.
4. [`holons/grace-op/OP_BUILD.md`](../holons/grace-op/OP_BUILD.md) — proto stage, runner registry, preflight.
5. [`holons/grace-op/internal/holons/runner_registry.go`](../holons/grace-op/internal/holons/runner_registry.go) — registry of 13 runners.
6. [`holons/grace-op/internal/holons/lifecycle.go:814-846`](../holons/grace-op/internal/holons/lifecycle.go) — `preflight()` check, where SDK prebuilt verification will hook in.
7. [`holons/grace-op/api/v1/holon.proto`](../holons/grace-op/api/v1/holon.proto) — `Requires` message, where `sdk_prebuilts` field lands.
8. [`.github/workflows/ader.yml`](../.github/workflows/ader.yml) — current CI strategy. Read in full, especially `ader.yml:37-50` (cache vars), `:85-91` (cache key).
9. [`sdk/README.md`](../sdk/README.md) — SDK contract.
10. [`sdk/zig-holons/build.zig`](../sdk/zig-holons/build.zig) — current Zig vendoring orchestration; the prebuilt detection branch will live here.
11. [`sdk/cpp-holons/CMakeLists.txt`](../sdk/cpp-holons/CMakeLists.txt) — current C++ system-deps logic (lines 13, 30, 39, 87 hard-coded paths).
12. [`sdk/c-holons/Makefile`](../sdk/c-holons/Makefile) — current C build.
13. [`sdk/ruby-holons/Gemfile`](../sdk/ruby-holons/Gemfile) and [`sdk/ruby-holons/README.md`](../sdk/ruby-holons/README.md) — current Ruby state.

---

## Composer decisions (resolved, treat as immutable)

The 8 questions in spec §11 have been arbitrated. Treat each as a binding constraint, not a recommendation. Your M0 ADR (`docs/adr/sdk-prebuilts-scope.md`) records them verbatim.

1. **Python OUT of v1.** Add as v1.1 only if Alpine demand emerges. Do not include `sdk/python-holons` in the prebuilts pipeline this chantier.
2. **popok-first runner strategy, transitional GitHub-hosted at kickoff.** Target state per spec §6.2: popok (self-hosted Apple Silicon Mac) hosts most targets via Docker / containers, and `winwok` (a separate Windows mini-PC) hosts the Windows target. Both runners live at Saint-Émilion on residential fibre — see [`docs/st_emilions_runners.md`](../docs/st_emilions_runners.md) for the deployment plan.

   **Important transitional reality**: at the time this chantier kicks off (right after the in-flight Zig P12 merges), the Saint-Émilion physical deployment has NOT yet happened (scheduled Thursday 2026-04-30). Therefore:

   - **All workflows ship initially with GitHub-hosted runners** (`runs-on: macos-14`, `ubuntu-latest`, `ubuntu-24.04-arm`, `windows-latest`). This is the explicit transitional default per `docs/st_emilions_runners.md` §1.
   - The first complete release cycle on GitHub-hosted validates the workflow logic itself.
   - **A separate follow-up PR** (small, workflow-YAML-only) swaps `runs-on:` to the self-hosted labels (`[self-hosted, popok, macos]`, `[self-hosted, popok, linux-via-docker]`, `[self-hosted, winwok, windows]`) once both runners are operational at Saint-Émilion and the composer signals readiness.
   - Do not delay any of the 10 chantier PRs waiting for Saint-Émilion; the transition is orthogonal.
3. **cosign keyless** signing at v1.0. v0.x ships SHA-256 only.
4. **Manifest field name `sdk_prebuilts`** as `Requires.sdk_prebuilts: repeated string`. Add to `holons/grace-op/_protos/holons/v1/manifest.proto`.
5. **T0 + T1 only in v1** (7 targets). T2 mobile (iOS, Android) deferred to v1.5. Do not add iOS/Android workflow stubs in this chantier.
6. **Stripped main archive + sidecar debug archive** per platform: `.dSYM/` (macOS), `.pdb` (Windows), `-debug.tar.gz` (Linux).
7. **Cache + prebuilts are complementary.** Keep the existing `actions/cache@v4` block in `ader.yml`; add the prebuilts-output cache key separately.
8. **Hardened mobile composites are an implementation detail** of the composite recipe runner, lands with T2 in v1.5. Out of scope for this chantier.

Do not relitigate these in code or in the ADR. If implementation surfaces a contradiction with one of them, halt and report per the loop policy below.

---

## Phasing

Sequential PRs against `dev`. Each phase is one PR. Same loop policy as the Zig chantier (10 iterations max per blocker, halt on truly blocking events).

### Phase 0 — Dead-code cleanup (do this first)

Before any prebuilts work, land a small cleanup PR addressing the 8 vestigial items the audit identified at [`docs/audit/sdk-toolchain-audit.md`](../docs/audit/sdk-toolchain-audit.md) §7:

**6 redundant proto-validation blocks** (each writes a descriptor file then either deletes it or leaves it unread):
- `examples/hello-world/gabriel-greeting-rust/build.rs:20-37` — delete the `Command::new("protoc")` block; tonic-build at lines 13-18 handles real codegen.
- `examples/hello-world/gabriel-greeting-dart/scripts/generate_proto.sh:22-29` — delete the descriptor block; line 11 handles real codegen.
- `examples/hello-world/gabriel-greeting-c/scripts/generate_proto.sh:30-38` — delete.
- `examples/hello-world/gabriel-greeting-cpp/scripts/generate_proto.sh:25-33` — delete.
- `examples/hello-world/gabriel-greeting-python/scripts/generate_proto.sh:13-20` — delete.
- `examples/hello-world/gabriel-greeting-kotlin/scripts/generate_proto.sh:17-24` — delete.

**2 phantom Gradle deps** (declared but never imported):
- `examples/hello-world/gabriel-greeting-java/build.gradle:21` — remove `compileOnly 'org.apache.tomcat:annotations-api:6.0.53'`.
- `examples/hello-world/gabriel-greeting-kotlin/build.gradle.kts:33` — remove `implementation("javax.annotation:javax.annotation-api:1.3.2")`.

Verify each deletion with the corresponding grep that returns zero hits (audit §7 cites the queries). Run `op build` for each affected example after each deletion to confirm nothing breaks.

**Exit:** all 8 deletions land in a single PR titled `chore(examples): remove redundant proto sanity checks and phantom annotation deps`.

### M0 — ADR and spike

- Open `docs/adr/sdk-prebuilts-scope.md` recording all 8 resolved decisions verbatim from the prompt above, plus the Windows path A/B choice determined by this spike.
- Open `docs/adr/sdk-prebuilts-abi.md` recording artifact ABI policy (semver, soname, stripping per spec §11.6).
- Build a throwaway spike workflow on popok that:
  1. Builds gRPC + libprotobuf-c for `aarch64-apple-darwin` from the `cpp-holons` source layout, native popok (no container).
  2. Builds the same for `x86_64-unknown-linux-gnu` via Docker `linux/amd64` on popok (QEMU emulation).
  3. Builds the same for `aarch64-unknown-linux-gnu` via Docker `linux/arm64` on popok (no emulation, native ARM).
  4. **Tests the Windows decision:** attempt to build for `x86_64-pc-windows-msvc` on popok via UTM/Tart Windows VM. Time it. If wall time is within 2× the macOS native build time, choose Path A. Otherwise document Path B (GitHub `windows-latest` fallback) in the ADR. Do not actually run a GitHub-hosted job in M0; only document the decision.
  5. Packages each output as a tarball, computes SHA-256, generates a stub SBOM (full SBOM lands in Phase 1).
- The spike does NOT touch any SDK source. It runs in `/tmp/seed-prebuilts-m0-spike/` (or equivalent worktree), commits the ADRs only.
- **Exit:** ADRs committed (with Windows path decision), spike workflow run green for the 3 (or 4) target slots, timing data captured in the ADR for capacity planning.

### Phase 1 — Verb skeleton (`op sdk`)
- Add `Requires.sdk_prebuilts` repeated string field to `holons/v1/manifest.proto` (in `holons/grace-op/_protos/`). Regenerate Go bindings.
- Add `op sdk` subcommand at `holons/grace-op/internal/cli/`:
  - `op sdk install <lang> [--target <triplet>] [--version <v>] [--source <url>]`
  - `op sdk list [--installed | --available]`
  - `op sdk uninstall <lang> [--target <triplet>] [--version <v>]`
  - `op sdk verify <lang> [--target <triplet>] [--version <v>]`
  - `op sdk path <lang> [--target <triplet>]`
- New RPCs `InstallSdkPrebuilt`, `ListSdkPrebuilts`, `UninstallSdkPrebuilt`, `VerifySdkPrebuilt`, `LocateSdkPrebuilt` in `holons/grace-op/api/v1/holon.proto`.
- Storage at `$OPPATH/sdk/<lang>/<version>/<target>/` per spec §5.2.
- For now, install path stops at SHA-256 verification (no cosign yet).
- Tests: unit tests for argv parsing, install path resolution, list-installed iteration. Integration tests deferred to phases that ship real artifacts.
- **Exit:** `op sdk` verbs work end-to-end against a hand-crafted local tarball (no real release backend yet).

### Phase 2 — `op build` preflight integration
- Extend `holons/grace-op/internal/holons/lifecycle.go:preflight()` to:
  - Read `Requires.sdk_prebuilts` from the manifest.
  - For each entry, call `LocateSdkPrebuilt` for the host triplet.
  - On miss: emit a single actionable error pointing at `op sdk install <lang>`.
  - On hit: set `OP_SDK_<LANG>_PATH` env var when invoking the runner.
- Tests: `lifecycle_test.go` with synthetic manifests declaring `sdk_prebuilts`.
- **Exit:** A holon manifest declaring `sdk_prebuilts: ["cpp"]` fails with the actionable error when no prebuilt is installed, and succeeds when one is.

### Phase 3 — Zig prebuilts (first SDK shipped)
- Add `.github/workflows/sdk-prebuilts.yml` (top-level orchestration) and `.github/workflows/_sdk-prebuilt-target.yml` (reusable per (sdk, target)). Triggers per spec §6.3 — PR to master only + workflow_dispatch.
- **Runner config (transitional, per the kickoff reality)**: ship the workflow with GitHub-hosted runners — `runs-on: macos-14` for macOS targets, `runs-on: ubuntu-latest` for Linux x86_64 (gnu and musl), `runs-on: ubuntu-24.04-arm` for Linux arm64 (gnu and musl), `runs-on: windows-latest` for Windows. The reusable workflow's `runs-on:` is a parameterised input so a follow-up PR can swap to self-hosted labels (`[self-hosted, popok, macos]`, `[self-hosted, popok, linux-via-docker]`, `[self-hosted, winwok, windows]`) without rewriting the SDK build scripts. Plan the workflow with that swap in mind from the start.
- Add `.github/scripts/build-prebuilt-zig.sh` parameterised by `$SDK_TARGET` and `$SDK_VERSION`. Builds gRPC + libprotobuf-c + the SDK static lib for the target, packages a tarball, computes SHA-256, generates SBOM via `syft`.
- Modify `sdk/zig-holons/build.zig` to detect `OP_SDK_ZIG_PATH` first, fall back to `third_party/.zig-vendor/native/`, error otherwise (per spec §5.3).
- Cover targets: T0 (4) + T1 (3) = 7 targets per resolved §11.5. **T2 mobile is OUT of v1**, do not add iOS/Android workflow stubs.
- Add `op sdk list --available` query to `https://github.com/organic-programming/seed/releases` to find the latest `zig-holons-v*` tag.
- Test by triggering the workflow on a draft PR; verify all 7 artifacts produced; verify `op sdk install zig` resolves and installs them; verify `op build` of a Zig holon uses the prebuilt and skips CMake/Ninja.
- Update `sdk/zig-holons/README.md` with the new install flow.
- **Exit:** A fresh checkout can build a Zig holon end-to-end without CMake / Ninja / submodule init, using `op sdk install zig` only.

### Phase 4 — C++ prebuilts (largest surface)
- Add `.github/scripts/build-prebuilt-cpp.sh`. Compiles gRPC-C++ + transitive (abseil, BoringSSL, c-ares, re2, zlib, upb, address_sorting, protobuf, grpc_cpp_plugin) for the target.
- Modify `sdk/cpp-holons/CMakeLists.txt` to detect `$OP_SDK_CPP_PATH` first, fall back to existing system-deps path with the hard-coded macOS paths, error otherwise.
- Repeat the verification pattern from Phase 3 against `examples/hello-world/gabriel-greeting-cpp/`.
- **Exit:** A fresh macOS / Linux / Windows machine can build a C++ holon end-to-end without `brew install grpc`, using `op sdk install cpp`.

### Phase 5 — C prebuilts (subset of C++)
- Add `.github/scripts/build-prebuilt-c.sh`. Subset of cpp (gRPC C, libprotobuf-c, no gRPC-C++).
- Modify `sdk/c-holons/Makefile` to detect `$OP_SDK_C_PATH`. Fall back to existing system-deps path. Error otherwise.
- Verify against `examples/hello-world/gabriel-greeting-c/`.
- **Exit:** Same as Phase 4, for C.

### Phase 6 — Ruby prebuilts (different model)
- Add `.github/scripts/build-prebuilt-ruby.sh`. Per target, run `bundle install` against the SDK Gemfile in a clean Ruby environment, compile the grpc + google-protobuf C extensions, package the resulting `vendor/bundle/` tree as a tarball.
- Modify `sdk/ruby-holons/` (and the example holon) to detect `$OP_SDK_RUBY_PATH` and configure bundle to reuse the pre-installed gems via `bundle config local.<gem> <path>`.
- Tests: verify Apple Silicon arm Ruby cold install drops from ~10-15 min to <30 sec.
- **Exit:** A fresh checkout with arm64 Ruby on Apple Silicon installs the Ruby SDK in <30 sec via `op sdk install ruby`, no native compile.

### Phase 7 — Promotion workflow + manifest publication
- Add the `promote` job in `sdk-prebuilts.yml` that on merge-to-master downloads the PR artifacts, computes the release manifest (`release-manifest.json` listing all artifacts + hashes + SBOM links), and creates the GitHub Release per `<sdk>-holons-v<version>` tag.
- Implement `op sdk list --available` to query `release-manifest.json` from the latest release per SDK.
- **Exit:** A merged PR produces a GitHub Release with all artifacts, no rebuild.

### Phase 8 — Documentation + INDEX promotion
- Update `INDEX.md` with `op sdk` row, status ✅.
- Update `OP_BUILD.md` with the prebuilt detection in §preflight.
- Update `sdk/README.md` to mention the prebuilts strategy under §5 ("Every SDK implements") with a note that c, cpp, ruby, zig also support `op sdk install`.
- Update each affected SDK's `README.md` with the install instructions.
- **Exit:** All registry edits in.

### Phase 9 (deferred to v1.0+) — Reproducibility, signing, T2 mobile
Not part of this chantier. Tracking-note only.

---

## Constraints in force

- **In-scope SDKs are exactly 4**: `ruby`, `c`, `cpp`, `zig`. Python is OUT of v1 (resolved §11.1). Do not add Python prebuilts in any phase, even speculatively.
- **Do not modify SDKs not listed here**: go, rust, dart, js, js-web, swift, java, kotlin, csharp, python stay untouched.
- **Do not break existing builds**: the prebuilt detection in each SDK's build script must fall back to the existing path; backwards-compatible.
- **PRs target `dev`**, never `master` directly.
- **No `--no-verify`**, no skipping hooks.
- **Do not delete `.codex/observability-impl.md`** (still applies, leave the file alone if you encounter it).
- **Runner policy (transitional)**: at chantier kickoff the Saint-Émilion self-hosted infra is not yet deployed. Workflows ship with GitHub-hosted runners (`macos-14`, `ubuntu-latest`, `ubuntu-24.04-arm`, `windows-latest`) initially. A small follow-up PR (workflow-YAML-only) swaps to self-hosted labels (`[self-hosted, popok, macos]`, `[self-hosted, popok, linux-via-docker]`, `[self-hosted, winwok, windows]`) once both runners are online at Saint-Émilion. See [`docs/st_emilions_runners.md`](../docs/st_emilions_runners.md) §5 for the transition plan.
- **Reuse existing infrastructure**: the `actions/cache@v4` already in `ader.yml`, the `OPPATH` / `OPBIN` env vars, the runner registry. Do not introduce parallel mechanisms.
- **No premature abstraction**: 4 SDKs in scope. Do not generalize the verb to all 14 unless an SDK joins the prebuilt list later.
- **No T2 mobile in v1**: do not add iOS / Android targets, workflow stubs, or runner labels for mobile. Resolved §11.5.
- **Coordinate with the Zig chantier**: if the Zig P12 chantier hasn't merged when Phase 3 starts, halt and flag to the composer. Phase 3 cannot land while `sdk/zig-holons/` is in flux.

---

## Loop policy (autonomous)

- One PR per phase. Each PR rebases on `dev` and keeps CI green.
- 10 iterations max on a single root-cause blocker before halting.
- Per phase: aim for 1-3 working days of agent time. Halt if a phase exceeds 5 days of stuck progress.
- Across phases: do not skip ahead. If Phase 4 fails, do not start Phase 5.

## Halt conditions

- Constraint contradiction with `CLAUDE.md` or other repo specs.
- Cross-compile dead end on a target after exhausting toolchain config.
- Architectural conflict with `op` runtime that needs composer input.
- Discovery of an earlier wrong decision (e.g., spec §5.3 prebuilt-detection branch turns out to be impossible in one of the SDKs' build systems).
- Security or data-integrity issue.

## Reporting cadence

After each phase merges:
- Reply with: PR URL, commit SHA range, `git diff --stat`, exit-criteria checklist (each ✅ or ⚠️), non-trivial decisions with 2-3 line rationale, latency / size numbers if relevant.
- Then immediately start the next phase.

After Phase 8 merges (final delivery):
- Reply with: list of all 8 PR URLs, the GitHub Releases URLs for each of the 4 SDKs (zig, cpp, c, ruby), `op sdk install` smoke output for each on a fresh runner, perf comparison table (cold install: before vs after), updated docs index.
- Then halt — chantier closed.

If you halt mid-chantier:
- Reply with: phase you halted in, blocker classification, everything tried, reproduction steps, recommendation for resolution (do not act on it without composer approval).

---

## Definition of done

- All 10 PRs merged: Phase 0 cleanup + M0 ADR + Phases 1–8.
- 4 GitHub Releases tagged: `zig-holons-v0.1.0`, `cpp-holons-v1.80.0`, `c-holons-v1.80.0`, `ruby-holons-v1.58.3`. Each with T0 + T1 = 7 artifacts + SHA-256 + SBOM.
- `op sdk install <lang>` works end-to-end for all 4 SDKs on macOS arm64, macOS x86_64, Linux x86_64, Linux arm64, Linux musl x86_64, Linux musl arm64, Windows x86_64.
- `op build` for a holon declaring `requires.sdk_prebuilts: ["cpp"]` (or c/ruby/zig) errors actionably when prebuilt missing, succeeds when present.
- Cold install times (measured on fresh runner): `<30s` for ruby (down from ~10-15m), `<30s` for cpp/c (down from ~5-10m brew install), `<30s` for zig (down from ~10-15m vendored compile).
- ADRs `sdk-prebuilts-scope.md` and `sdk-prebuilts-abi.md` reflect all decisions made.
- `INDEX.md`, `OP_BUILD.md`, `sdk/README.md`, and each affected SDK's README updated.

Go. Start M0 now.
