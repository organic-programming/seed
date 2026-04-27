# SDK Prebuilts — Specification

Status: spec — pending composer approval  
Date: 2026-04-26  
References: [`docs/audit/sdk-toolchain-audit.md`](../audit/sdk-toolchain-audit.md), [`OP_BUILD.md`](../../holons/grace-op/OP_BUILD.md), [`sdk/README.md`](../../sdk/README.md)

---

## 1. Context

Per the [toolchain audit](../audit/sdk-toolchain-audit.md), there are two distinct moments where toolchain matters and they're commonly conflated (audit §2):

- **At build time** (`op build my-holon`): generated code is committed per CLAUDE.md rule #7, and `op` parses protos in-process via pure-Go protoparse. **No SDK requires `protoc` at build time, ever.** What hurts at build time is the runtime native libraries the linker needs.
- **At proto-edit time** (developer modifies `.proto`): `protoc` (or substitute) is needed. 7 SDKs already vendor it via their package manager (java/kotlin via Gradle, csharp via NuGet, python via wheel, ruby via gem, rust via `protoc-bin-vendored` crate, zig via SDK vendored). The residual gap is go/dart/swift, where a diagnostic suffices.

The audit identifies 4 SDKs with substantive build-time pain prebuilts can eliminate:

- `ruby` — `grpc` C extension source build on Apple Silicon w/ x86_64 Ruby, Alpine/musl, niche ARMs (`ader.yml:37`, `sdk/ruby-holons/README.md:5`).
- `c` and `cpp` — system-deps-required CMake builds with hard-coded macOS paths, broken on Windows (`sdk/cpp-holons/CMakeLists.txt:13-87`).
- `zig` — vendored-from-source cold first build ~10-15 min.

The remaining 10 SDKs are either pure-language at runtime or already covered by upstream ecosystems for both build-time and proto-edit-time.

This spec defines an `op sdk` subsystem that ships prebuilts for the 4 affected SDKs uniformly. Each prebuilt archive carries both the runtime libraries (lib/, include/) and the codegen tooling for that language (bin/protoc, bin/protoc-c, bin/grpc_cpp_plugin where relevant), so a single `op sdk install <lang>` covers both moments.

The audit also identifies 8 dead-code items (audit §7) that should be cleaned up before or in parallel with this work — they reinforce the false belief that system protoc is required at build time.

---

## 2. Goals

- One mechanism (`op sdk install <lang>`) that handles all 4 (or 5 with Python) SDKs with native pain.
- Reproducible artifacts pinned to deterministic source SHAs (gRPC, libprotobuf-c, abseil, BoringSSL, etc.).
- Triggered on PR-to-master only (and manual dispatch). Never on feature-branch push.
- Promoted on merge without rebuild (artifact promotion, not double-build).
- Cross-compile coverage for production targets: macOS arm64/x86_64, Linux x86_64/arm64 (gnu + musl), Windows x86_64, iOS arm64 + simulator, Android arm64 + x86_64.
- Signed and SBOM-attached at v1.0; un-signed acceptable for v0.x internal use.

## 3. Non-goals

- **Not** a generic package manager. Scope is the 4-5 SDKs identified.
- **Not** a replacement for upstream package managers. Maven Central, NuGet, RubyGems, PyPI continue to serve where they already work.
- **Not** an Xcode replacement for Swift. Swift SPM slowness is dependency-resolution, not native compilation; out of scope.
- **Not** Zig toolchain shipping. We ship the SDK runtime libs, not the Zig compiler itself.
- **Not** ABI-stable across major versions; semver applies per artifact.

---

## 4. SDK selection — what's in, what's out, why

| SDK | In/Out | Reason | Artifact contents |
|---|---|---|---|
| `ruby` | **IN** | Native gem source build documented as slow; Apple Silicon x86_64 Ruby + Alpine/musl + niche ARMs lack binary gems | Pre-installed `vendor/bundle/` with grpc + google-protobuf compiled per target |
| `c` | **IN** | System-deps-required, no portable fallback; Windows broken | Static libs (`libgrpc.a`, `libprotobuf-c.a`, transitive), headers, `protoc-c` plugin |
| `cpp` | **IN** | Same as C plus larger surface (gRPC-C++, abseil, BoringSSL, c-ares, re2, zlib, upb) | Static libs, headers, `grpc_cpp_plugin` codegen tool |
| `zig` | **IN** | Vendored-from-source cold build ~10-15 min on every fresh checkout | Static libs from `sdk/zig-holons/.zig-vendor/native/` baked into a portable archive, plus `protoc-c` |
| `python` | **OPT** | Wheels cover most platforms; Alpine/musl needs source build. Add only if Alpine usage is real. | Pre-installed `vendor/site-packages/` with grpcio + grpcio-tools + grpcio-reflection compiled per target |
| `go` | OUT | Pure Go (CGO off by default); no native runtime |
| `rust` | OUT | Pure Rust with rustls; no native runtime |
| `dart` | OUT | Pure Dart, `grpc^5.1.0` is pure Dart |
| `js`, `js-web` | OUT | Pure JS since `@grpc/grpc-js` deprecated native addon |
| `swift` | OUT | Pure Swift; slowness is SPM resolution, addressed by Xcode-side caching |
| `java`, `kotlin` | OUT | JVM bytecode; `grpc-netty-shaded` bundles all transitive code |
| `csharp` | OUT | Managed .NET; gRPC implementation is in stdlib HTTP/2 |

V1 ships **`ruby + c + cpp + zig`** (4 SDKs). Python is a follow-up if Alpine demand materializes.

---

## 5. Architecture

### 5.1 Verb surface

New top-level verb `op sdk` with subcommands:

```
op sdk install <lang> [--target <triplet>] [--version <v>] [--source <url>]
op sdk list [--installed | --available]
op sdk uninstall <lang> [--target <triplet>] [--version <v>]
op sdk verify <lang> [--target <triplet>] [--version <v>]
op sdk path <lang> [--target <triplet>]                       # print install path for build scripts
```

Behaviour:

- **`install`** without `--target`: detects host triplet, installs that one. With `--target`: installs prebuilt for that target (used in cross-compile workflows).
- **`install`** with `--source`: takes a local tarball or URL instead of the default release URL (for offline / mirror / dev).
- **`install`** is idempotent. Re-running with the same version is a no-op; with a different version it adds the new version alongside.
- **`list --installed`**: tree of `$OPPATH/sdk/<lang>/<version>/<target>/`.
- **`list --available`**: query the release manifest at `https://github.com/organic-programming/seed/releases` (or override URL) for known versions and targets per SDK.
- **`verify`**: re-hash installed files and compare to manifest hashes. Detects tampering or partial install.
- **`path`**: emits the install path; used by SDK build scripts (`build.zig`, `Makefile`, `CMakeLists.txt`, `Gemfile`-driven bundle config) to locate the prebuilt.

### 5.2 Local layout

```
$OPPATH/sdk/
├── ruby/
│   └── 1.58.3/
│       └── arm64-darwin/
│           ├── vendor/bundle/   # bundled gems compiled for arm64-darwin
│           ├── manifest.json    # versions, hashes, build metadata
│           └── SBOM.spdx.json   # software bill of materials
├── c/
│   └── 1.80.0/
│       └── aarch64-apple-darwin/
│           ├── lib/             # libgrpc.a, libprotobuf-c.a, libabsl_*.a, ...
│           ├── include/         # grpc/, protobuf-c/, absl/, ...
│           ├── bin/             # protoc-c
│           ├── manifest.json
│           └── SBOM.spdx.json
├── cpp/
│   └── 1.80.0/
│       └── aarch64-apple-darwin/
│           ├── lib/
│           ├── include/         # grpc/, google/protobuf/, absl/, nlohmann/, ...
│           ├── bin/             # grpc_cpp_plugin, protoc
│           ├── manifest.json
│           └── SBOM.spdx.json
└── zig/
    └── 0.1.0/                   # version of sdk/zig-holons itself
        └── aarch64-apple-darwin/
            ├── lib/             # libholons_zig.a + transitive gRPC + protobuf-c
            ├── include/
            ├── bin/             # protoc-c
            ├── manifest.json
            └── SBOM.spdx.json
```

Versions are pinned to the **upstream library version** (gRPC `1.80.0`, libprotobuf-c `1.5.2`, grpc gem `1.58.3`) for c/cpp/ruby, and to the **SDK source version** for zig (0.1.0, 0.2.0, …).

### 5.3 SDK build-script integration

Each native SDK gets a **prebuilt-detection branch** at the head of its build script. Triple fallback: prebuilt > local vendored > error.

**`sdk/zig-holons/build.zig`** (pseudo-code):
```zig
const prebuilt = std.process.getEnvVarOwned(b.allocator, "OP_SDK_ZIG_PATH") catch null;
if (prebuilt) |p| {
    // OP_SDK_ZIG_PATH set by `op build` after `op sdk install zig`
    use_prebuilt(p);
} else if (path_exists("third_party/.zig-vendor/native/lib/libgrpc.a")) {
    use_local_vendored();
} else {
    @panic("Run `op sdk install zig` or `cd sdk/zig-holons && zig build vendor` first");
}
```

**`sdk/cpp-holons/CMakeLists.txt`**:
```cmake
if(DEFINED ENV{OP_SDK_CPP_PATH})
    set(CMAKE_PREFIX_PATH "$ENV{OP_SDK_CPP_PATH}" ${CMAKE_PREFIX_PATH})
    find_package(gRPC CONFIG REQUIRED)
    find_package(Protobuf CONFIG REQUIRED)
elseif(EXISTS "/opt/homebrew/lib/libgrpc.dylib")
    # existing system-deps path
else()
    message(FATAL_ERROR "Run `op sdk install cpp` or install gRPC system-wide")
endif()
```

**`sdk/c-holons/Makefile`**: similar conditional on `OP_SDK_C_PATH`.

**`sdk/ruby-holons/Gemfile`** + bundle config: when `OP_SDK_RUBY_PATH` is set, `bundle config local.grpc <path>` and reuse the pre-installed gem path.

### 5.4 `op build` integration

Before invoking the runner, `op build` checks the manifest's `build.runner` and if the runner is `cmake`, `cargo` (with native deps), `ruby`, `python`, or a custom runner declared as needing native libs, it:

1. Determines the host target triplet.
2. Calls `op sdk path <lang>` to find the prebuilt root.
3. If no prebuilt is installed, emits a single actionable error: *"Run `op sdk install <lang>` to install prebuilt native libraries (~30 sec download, no source compilation)"*.
4. Sets `OP_SDK_<LANG>_PATH` env var when invoking the runner.

This is implemented in `holons/grace-op/internal/holons/lifecycle.go` `preflight()` — extends the existing `requires.commands` check.

### 5.5 Manifest declaration

Holons opt-in via their `holon.proto` manifest:

```protobuf
requires: {
  commands: ["cmake"]
  files: ["CMakeLists.txt"]
  sdk_prebuilts: ["cpp"]   // new field; consumed by `op build`
}
```

`sdk_prebuilts` is a repeated string field in `requires` (proto change in `holons/v1/manifest.proto`). When set, `op build` enforces the prebuilt presence check before invoking the runner.

---

## 6. Distribution

### 6.1 Backend: GitHub Releases

Each SDK has its own release tag stream:

```
ruby-holons-v1.58.3
c-holons-v1.80.0
cpp-holons-v1.80.0
zig-holons-v0.1.0
```

Per release tag, assets per target:

```
<sdk>-<version>-<target>.tar.gz
<sdk>-<version>-<target>.tar.gz.sha256
<sdk>-<version>-<target>.tar.gz.sig                # cosign sig (v1.0+)
<sdk>-<version>-<target>.spdx.json                  # SBOM
release-manifest.json                               # lists all assets, hashes, sigs
```

`release-manifest.json` is the source of truth queried by `op sdk list --available`.

### 6.2 Target matrix and runner strategy

**Runner policy** (resolved §11.2): popok (self-hosted Apple Silicon Mac) is the primary runner via containerization (Docker, Lima/Colima/Rancher Desktop) and virtualization (UTM/Tart for non-Linux/non-macOS targets). GitHub-hosted runners are the **last resort**, used only when popok cannot reasonably host a target.

| Tier | Target triplet | Runner / mechanism | Scope |
|---|---|---|---|
| T0 | `aarch64-apple-darwin` | popok (native macOS arm64) | dev primary (Apple Silicon) |
| T0 | `x86_64-apple-darwin` | popok (Rosetta x86_64 native build) | dev legacy (Intel Mac) |
| T0 | `x86_64-unknown-linux-gnu` | popok + Docker `linux/amd64` (qemu emulation on Apple Silicon) | server primary |
| T0 | `aarch64-unknown-linux-gnu` | popok + Docker `linux/arm64` (native, no emulation) | server ARM |
| T1 | `x86_64-unknown-linux-musl` | popok + Docker `linux/amd64` + Alpine container | container builds |
| T1 | `aarch64-unknown-linux-musl` | popok + Docker `linux/arm64` + Alpine container | container ARM |
| T1 | `x86_64-windows-gnu` | GitHub `ubuntu-latest` + Zig bundled MinGW (transitional v1) | dev Windows |
| T1 follow-up | `x86_64-pc-windows-msvc` | winwok native Windows runner after Saint-Émilion deployment | dev Windows |
| T2 | `aarch64-apple-ios` | popok (native macOS + Xcode) | iOS device builds |
| T2 | `aarch64-apple-ios-sim` | popok (native macOS + Xcode iOS simulator SDK) | iOS Apple Silicon simulator |
| T2 | `aarch64-linux-android` | popok + Docker + Android NDK | Android arm64 |
| T2 | `x86_64-linux-android` | popok + Docker + Android NDK | Android emulator x86_64 |

12 targets in the full matrix while Windows is transitional. Per spec §11.5 resolution and Phase 3 arbitration: **v1 ships T0 + T1 only (7 targets)** with `x86_64-windows-gnu` as the Windows target. `x86_64-pc-windows-msvc` is kept visible as a v1.x follow-up once winwok is deployed at Saint-Émilion. T2 (mobile) deferred to v1.5 once composite hardened builds prove the demand.

V1 artifacts: 7 targets × 4 SDKs = **28 artifacts per release**.

Current v0.x workflow rows:

| SDK | Release stream | Workflow version input | Packaged payload |
|---|---|---|---|
| `zig` | `zig-holons-v0.1.0` | `sdk-version` | Zig SDK static library plus gRPC/protobuf-c headers and static libs |
| `cpp` | `cpp-holons-v1.80.0` | `cpp-version` | gRPC-C++ static libs, transitive headers/libs, `protoc`, and `grpc_cpp_plugin` |
| `c` | `c-holons-v1.80.0` | `c-version` | C SDK native bundle with gRPC/upb/protobuf-c headers/libs, `protoc`, upb generators, and `protoc-c` |
| `ruby` | `ruby-holons-v1.58.3` | `ruby-version` | Vendored Bundler `vendor/bundle/` tree for pinned `grpc` and transitive native gems |

Per-target build cost: ~30-60 min wall time on popok. With matrix parallelism (popok has limited CPU vs GitHub matrix, but no minute quota), total wall time per release is ~2-3 hours sequential or ~60 min if popok hosts ≥4 concurrent builds.

#### Windows fallback decision

Windows is the one target where popok-via-VM has a real cost: UTM Windows VM on Apple Silicon is slower than a native Windows runner, and gRPC-C++ on MSVC is the trickiest target. Phase 3 ships `x86_64-windows-gnu` because P12 validated Zig's bundled MinGW target and winwok is not yet online. The MSVC target is tracked separately for the post-Saint-Émilion native Windows runner. Two acceptable paths remain for that follow-up:

- **Path A (preferred)**: popok + UTM Windows ARM (Apple Silicon → Windows ARM via UTM is fast); cross-compile x86_64-windows from there if MSVC supports it natively, else fall back to Path B.
- **Path B (fallback)**: GitHub `windows-latest` runner only for `x86_64-pc-windows-msvc`. All other targets stay on popok.

The M0 spike validates which path works. Decision recorded in `docs/adr/sdk-prebuilts-scope.md`.

### 6.3 Triggers

```yaml
# .github/workflows/sdk-prebuilts.yml
on:
  pull_request:
    branches: [master]
    paths:
      - 'sdk/zig-holons/build.zig'
      - 'sdk/zig-holons/build.zig.zon'
      - 'sdk/zig-holons/.gitmodules'
      - 'sdk/cpp-holons/CMakeLists.txt'
      - 'sdk/c-holons/Makefile'
      - 'sdk/ruby-holons/Gemfile'
      - 'sdk/ruby-holons/Gemfile.lock'
      - 'sdk/python-holons/pyproject.toml'  # if python included
      - '.gitmodules'
      - '.github/workflows/sdk-prebuilts.yml'
      - '.github/workflows/_sdk-prebuilt-target.yml'  # reusable
  workflow_dispatch:
    inputs:
      sdk-version:
        description: Zig SDK prebuilt version
        default: '0.1.0'
      cpp-version:
        description: C++ SDK prebuilt version
        default: '1.80.0'
```

PR to `master` triggers the current full matrix. Manual dispatch reruns the current SDK rows with explicit version inputs; Phase 7 promotes this into release-manifest publication.

For v0.x, the workflow ships with both triggers: `pull_request` to `master` for the release validation path and `workflow_dispatch` for composer-initiated full-matrix validation after the workflow exists on the default branch. GitHub Actions only exposes `workflow_dispatch` for workflows present on the default branch, so a chantier PR that introduces or changes this workflow relies on PR checks plus local cross-smoke evidence until manual dispatch after merge.

Push to feature branches: **no trigger**. Saves CI compute.

### 6.4 Promotion on merge

```yaml
# Same workflow, second job
on:
  push:
    branches: [master]

jobs:
  promote:
    if: <merged from a PR>
    steps:
      - name: Download PR artifacts
        uses: actions/download-artifact@v4
        with: { run-id: <PR head run> }
      - name: Compute hashes & manifest
        run: ./.github/scripts/build-release-manifest.sh
      - name: Sign with cosign (v1.0+)
        uses: sigstore/cosign-installer@v3
      - name: Create GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          tag_name: <sdk>-<version>
          files: |
            dist/*.tar.gz
            dist/*.sha256
            dist/*.spdx.json
            dist/release-manifest.json
```

No rebuild on merge. Artifacts produced during PR are promoted.

---

## 7. Release engineering

### 7.1 Reusable workflow per target

`.github/workflows/_sdk-prebuilt-target.yml`:

```yaml
on:
  workflow_call:
    inputs:
      sdk: { required: true, type: string }
      target: { required: true, type: string }
      runner: { required: true, type: string }
      container: { required: false, type: string }
      continue-on-error: { required: false, type: boolean, default: false }
```

Called by `sdk-prebuilts.yml` once per (sdk × target) pair.

`continue-on-error` is plumbed onto the inner `build` job and lets `sdk-prebuilts.yml` mark targets as paused without blocking the overall workflow. The flag must live on the inner job: GitHub Actions rejects `continue-on-error` at the job level on jobs that call a reusable workflow with `uses:` (only `name`, `uses`, `with`, `secrets`, `needs`, `if`, `permissions`, `strategy` are accepted there), which would fail the workflow at load time before any job runs.

Per-SDK build script lives at `.github/scripts/build-prebuilt-<sdk>.sh` and is parameterised by `$SDK_TARGET`. Each script:

1. Initialises the SDK's source tree (submodules, vendored deps).
2. Configures and compiles the relevant native libs for the target.
3. Stages the output into `dist/<sdk>-<version>-<target>/`.
4. Computes SHA-256.
5. Generates SBOM via `syft` (or `cdxgen`).
6. Tarballs the staged tree.

### 7.2 Reproducibility

For T0 + T1 (server / dev), reproducibility is mandatory at v1.0:

- `SOURCE_DATE_EPOCH` exported per build.
- `CFLAGS=-ffile-prefix-map=...`, deterministic linker flags.
- Strip mtime from tarballs (`tar --mtime=@$SOURCE_DATE_EPOCH --sort=name`).
- Pin compiler versions per runner (e.g., `clang-17`, `gcc-13`, `msvc-2022`).
- Pin source SHAs via submodules.

For T2 (mobile), reproducibility is best-effort due to Apple/Android SDK version churn.

### 7.3 Signing (v1.0+)

Each artifact signed with **cosign keyless** (sigstore) tied to the GitHub Actions OIDC identity. Verification via:

```
cosign verify --certificate-identity ... --certificate-oidc-issuer https://token.actions.githubusercontent.com <artifact>
```

`op sdk install` calls `cosign verify` before extracting. Failure aborts install with a clear error.

V0.x ships with SHA-256 hashes only (no signing). The signing layer is added at v1.0.

### 7.4 SBOM

Each artifact ships an SPDX 2.3 SBOM in JSON format. Generated by `syft` invoked on the build output. Per `OBSERVABILITY.md` lifecycle and EU CRA 2027 compliance.

---

## 8. Versioning & lifecycle

### 8.1 Versioning scheme

- **For c, cpp**: pinned to gRPC version. Tag = `<sdk>-holons-v<grpc-version>` (e.g., `cpp-holons-v1.80.0`).
- **For ruby**: pinned to grpc gem version. Tag = `ruby-holons-v<gem-version>` (e.g., `ruby-holons-v1.58.3`).
- **For zig**: pinned to SDK source version (independent of upstream gRPC). Tag = `zig-holons-v<sdk-version>` (e.g., `zig-holons-v0.1.0`). The `manifest.json` per artifact records the underlying gRPC + libprotobuf-c versions.
- **For python (if shipped)**: pinned to grpcio version. Tag = `python-holons-v<grpcio-version>`.

### 8.2 Bumping policy

- **Major (`X.y.z` → `X+1.0.0`)**: ABI-breaking change in the prebuilt. SDK build scripts must conditionalize.
- **Minor (`x.Y.z` → `x.Y+1.0`)**: new optional functionality. Backward-compatible.
- **Patch (`x.y.Z` → `x.y.Z+1`)**: bug fix or transitive dep refresh. Drop-in.

### 8.3 Retention

The latest 3 minor versions per SDK are kept indefinitely (no GC). Older versions retained for 12 months then archived to a "frozen" branch of releases.

### 8.4 Deprecation of targets

Targets follow upstream support. When a runner OS image is deprecated by GitHub (e.g., `macos-13` removal), targets relying on it are marked deprecated for 6 months, then dropped.

---

## 9. Migration

Phase the rollout to minimise disruption:

1. **Phase 1 — `zig`**: ship Zig prebuilts first. The SDK is in flight, less legacy state. Validates the full pipeline.
2. **Phase 2 — `cpp`**: largest surface (gRPC-C++ + abseil + BoringSSL). Most strain on the build infra. Validates target matrix at scale.
3. **Phase 3 — `c`**: subset of cpp (gRPC C + libprotobuf-c). Reuses much of phase 2.
4. **Phase 4 — `ruby`**: different model (vendored gem bundle, not static libs). Validates the SDK-agnostic vendor-pack abstraction.
5. **Phase 5 — `python`** (optional): same model as ruby (vendored site-packages).

Each phase is one PR introducing the workflow, the SDK build-script changes, and `op` runtime changes. Sequential, not parallel — each phase validates the spec further.

---

## 10. Security

- Public repo, so all artifacts are public. No secret material in builds.
- **Self-hosted runner threat model** (popok): per resolved §11.2, popok hosts most targets via containers/VMs. Each container/VM gets a fresh state per build (no persistent cross-build mutation). Network isolation: containers join an ephemeral docker network with no access to popok's secrets or to other catalogues' caches except those explicitly mounted. The runner itself runs as a non-privileged user (`popok`) without sudo for the workflow steps.
- Cosign keyless signing ties artifacts to the workflow's OIDC identity. Even though popok is self-hosted, the OIDC issuer is GitHub Actions for the workflow run; signatures still bind to the repo + workflow + ref.
- SBOMs allow downstream auditing of transitive deps.
- `op sdk install` verifies SHA-256 (always) and cosign signature (v1.0+) before extracting. Refuses to install otherwise.
- No credentials needed for download (public assets); install does not require auth.
- Windows fallback path (if Path B is chosen, see §6.2): GitHub `windows-latest` runner is ephemeral and follows GitHub's standard hardening; same trust model as the OIDC-bound cosign signature.

---

## 11. Decisions resolved

All 8 questions arbitrated by the composer 2026-04-26. The spec is now decision-complete.

| # | Question | Resolution |
|---|---|---|
| 1 | Python in v1? | **OUT.** Skip v1, add as v1.1 if Alpine demand emerges. |
| 2 | Runner strategy? | **popok-first via containers/VMs.** GitHub-hosted runners as last resort. See §6.2 for the per-target mapping and §10 for the self-hosted threat model. |
| 3 | Signing? | **cosign keyless** at v1.0. v0.x ships SHA-256 only. |
| 4 | Manifest field name? | **`sdk_prebuilts`** (`Requires.sdk_prebuilts: repeated string`). |
| 5 | Mobile targets in v1? | **OUT of v1.** T2 deferred to v1.5; demand validated when composite hardened builds need it. V1 = T0 + T1 = 7 targets. |
| 6 | Symbol stripping? | **One stripped main archive + sidecar debug archive per platform**: `.dSYM/` on macOS, `.pdb` on Windows, `-debug.tar.gz` on Linux. |
| 7 | Cache vs prebuilt? | **Complementary, no conflict.** GitHub Actions cache = within-run. Prebuilts = cross-run. |
| 8 | Hardened mobile composites? | **Composite recipe runner calls `op sdk install <lang> --target <mobile-triplet>`** when needed. Implementation detail of the composite, lands when T2 lands (v1.5). |

The Codex M0 phase opens `docs/adr/sdk-prebuilts-scope.md` and copies these resolutions verbatim, plus the path A/B Windows decision determined by the M0 spike.

---

## 12. Acceptance

Spec is accepted when:

- The 4 v1 SDKs (`ruby`, `c`, `cpp`, `zig`) each have a green PR producing prebuilts for T0 + T1 (7 targets each = 28 artifacts).
- `op sdk install`/`list`/`uninstall`/`verify`/`path` are implemented and tested.
- `op build` integration (env var injection + prereq check) is implemented and tested.
- `holon.proto` `requires.sdk_prebuilts` field exists and is honored.
- Each SDK's build script has the prebuilt-detection branch with triple fallback.
- A release-manifest verification script under `.github/scripts/` validates artifact integrity.
- `OP_BUILD.md` and `sdk/README.md` updated to document the new flow.
- `INDEX.md` lists `op sdk` under "op CLI" with status ✅.

T2 mobile targets and full signing (cosign + reproducibility audit) are v1.5 acceptance, not v1.
