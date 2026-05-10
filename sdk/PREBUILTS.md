# SDK Native Prebuilts

This document covers SDK distributions installed under `$OPPATH/sdk`. Heavy
SDKs (`c`, `cpp`, `ruby`, `zig`) carry compiled native runtime dependencies
(gRPC, protobuf-c, abseil, c-ares, ...). The other official SDKs
(`csharp`, `dart`, `go`, `java`, `js`, `js-web`, `kotlin`, `python`,
`rust`, `swift`) carry the proto codegen plugin binaries used by
`build.codegen`.

The `protoc` binary itself is **not** carried by individual SDKs. It lives
once per machine under `$OPPATH/sdk/shared/protoc/<version>/` and is
referenced from each SDK's `manifest.json`. See [Shared toolchain](#shared-toolchain)
below. Host-PATH `protoc` is never consulted by `op` — codegen always
resolves through the shared pool.

## Two ways to land a prebuilt

`op sdk` exposes two explicit verbs — no silent fallback between them.

| Verb | What it does | When to use | Cost |
|---|---|---|---|
| `op sdk install <lang>` | Downloads a published release tarball from the GitHub Release manifest and unpacks it under `$OPPATH/sdk/<lang>/<version>/<target>/`. | A release exists for your `(lang, target)` pair. | ~30 s download. |
| `op sdk build <lang>` | Invokes the per-SDK script under `.github/scripts/build-prebuilt-<lang>.sh` to compile the prebuilt locally from the gRPC + per-SDK sources, then installs the resulting tarball via the same code path as `install`. | No release exists yet (e.g. you bumped the SDK version, or you're on a target that CI doesn't publish). | ~30-60 min cold; seconds with `--force` skipped on cache hit. |

Both paths produce identical on-disk layout, manifest, and tree-hash. `op sdk verify <lang>`
recomputes the tree hash and fails if the install no longer matches its recorded metadata.

## Per-SDK prerequisites

`op sdk build` invokes a shell script per SDK; each script has its own toolchain
expectations. `op sdk list --compilable` reports which SDKs are buildable RIGHT NOW
on this checkout, and surfaces per-entry `blockers` when something is missing.

| SDK | Required commands on PATH | Required submodules |
|---|---|---|
| `c` | `zig`, `cmake`, `ninja`, `xcrun` (darwin only) | `sdk/zig-holons/third_party/grpc`, `sdk/zig-holons/third_party/protobuf-c`, `sdk/cpp-holons/third_party/nlohmann-json` |
| `cpp` | `zig`, `cmake`, `ninja`, `xcrun` (darwin only) | `sdk/zig-holons/third_party/grpc`, `sdk/cpp-holons/third_party/nlohmann-json` |
| `go`, `js-web` | `go` | none |
| `csharp`, `java`, `js`, `kotlin`, `python` | `go`, `curl`, `unzip` | none |
| `dart` | `dart` | none |
| `ruby` | `ruby` (3.1.x), `bundle` | none |
| `rust` | `cargo` | none |
| `swift` | `git`, `swift` | none |
| `zig` | `zig`, `cmake`, `ninja`, `xcrun` (darwin only) | `sdk/zig-holons/third_party/grpc`, `sdk/zig-holons/third_party/protobuf-c` |

Initialise missing submodules with `git submodule update --init --recursive`.

## Storage layout

Installed prebuilts live under:

```text
$OPPATH/sdk/
  shared/                                # cross-SDK toolchain pool
    protoc/<version>/
      bin/protoc
      include/google/protobuf/...        # well-known types
      manifest.json                      # version, sha256, install source
  <lang>/<version>/<target>/
    manifest.json        # archive_sha256, tree_sha256, source URL,
                         # codegen.plugins manifest, and the protoc
                         # version this SDK requires (key: `protoc.version`)
    …                    # extracted SDK contents (lib/, include/, bin/plugins…)
```

`op sdk path <lang>` prints the install path. `op sdk uninstall <lang>` removes it.

`op build <holon>` consumes installed prebuilts via the holon manifest's
`requires.sdk_prebuilts` field. Preflight injects an env var per matched SDK
(`OP_SDK_C_PATH`, `OP_SDK_CPP_PATH`, `OP_SDK_RUBY_PATH`, `OP_SDK_ZIG_PATH`) so
runners pick them up without per-runner glue. Hyphenated SDK names use an
underscore in the environment variable, e.g. `OP_SDK_JS_WEB_PATH`. The shared
protoc resolved for the current SDK is exposed as `OP_SDK_PROTOC` (absolute
path to the `bin/protoc` inside `$OPPATH/sdk/shared/protoc/<version>/`).

## Shared toolchain

`protoc` is the only tool whose semantics are coupled across SDKs (the binary
codegens for cpp/java/kotlin/python/ruby/csharp live inside `libprotoc`, so the
protoc binary version determines stub layout and runtime ABI compatibility).
For that reason it is materialized once per machine and shared by every SDK
that needs it.

**Layout.** A version-pinned protoc lives at:

```text
$OPPATH/sdk/shared/protoc/<version>/bin/protoc
$OPPATH/sdk/shared/protoc/<version>/include/google/protobuf/*.proto
$OPPATH/sdk/shared/protoc/<version>/manifest.json   # sha256, install source
```

Multiple versions can cohabit (e.g. during a transition) but a single seed
release pins exactly one.

**Per-SDK declaration.** Each SDK's `manifest.json` declares which protoc
version it requires:

```json
{
  "lang": "cpp",
  "version": "1.80.0",
  "target": "aarch64-apple-darwin",
  "protoc": { "version": "32.0", "sha256": "..." },
  "codegen": { "plugins": [ ... ] }
}
```

For pure-plugin SDKs that don't invoke protoc (`go`, `dart`, `rust`,
`swift`, `js-web`), the `protoc` field is `null` or absent — `op` skips
resolution.

**Self-healing on install.** `op sdk install <lang>` reads the SDK manifest,
resolves the required `protoc.version`, then:

- if `$OPPATH/sdk/shared/protoc/<version>/` is absent → downloads and installs;
- if present but `manifest.json` sha256 mismatches the recorded value →
  replaces;
- if present and coherent → no-op.

This makes every `op sdk install` an opportunity to repair `shared/` for all
sibling SDKs. The first SDK installed pays the protoc download; subsequent
installs that reference the same version are zero-cost.

**`op` core never resolves protoc.** Consumer-tier paths — `op <slug> <rpc>`,
`op run`, `op mcp`, `op proxy`, `op inspect` — neither read nor invoke
`shared/`. The shared pool is touched only by `op sdk install` /
`op sdk build` / `op sdk verify`, and by holon-build recipes that opt in via
`requires.sdk_prebuilts`. A non-developer install of `op` populates nothing
under `$OPPATH/sdk/shared/`.

**CI parity.** `.github/scripts/build-prebuilt-<lang>.sh` consume the same
shared pool — they call `op sdk install` (or its primitive) ahead of the
codegen step rather than exporting a per-script `PROTOC_VERSION`. The
historical `PROTOC_VERSION=32.0` exports (in 5 scripts) and the `34.1`
default in `lib-codegen-prebuilt.sh` are removed in favor of the SDK
manifest's declaration.

## Supported targets

System-wide allowed targets (in `allowedTargets`):

| Target | Status |
|---|---|
| `aarch64-apple-darwin` | ✅ supported |
| `x86_64-apple-darwin` | ✅ supported (with per-SDK exceptions, see below) |
| `x86_64-unknown-linux-gnu` | ✅ supported |
| `aarch64-unknown-linux-gnu` | ✅ supported |
| `x86_64-unknown-linux-musl` | ✅ supported |
| `aarch64-unknown-linux-musl` | ✅ supported |
| `x86_64-windows-gnu` | ✅ supported |
| `x86_64-pc-windows-msvc` | deferred (no self-hosted Windows runner yet) |

`--target` accepts any of the supported triplets. Default is the host triplet
(detected via `runtime.GOOS` / `runtime.GOARCH`).

### Per-SDK suspended pairs

Some `(lang, target)` pairs are temporarily unsupported even when the target
itself is allowed. They live in `suspendedPrebuilts` in
[`sdkprebuilts.go`](../holons/grace-op/internal/sdkprebuilts/sdkprebuilts.go).
`op sdk build` rejects them with a clear message; `op sdk list --compilable`
reports them as `blockers`.

| Lang | Target | Reason |
|---|---|---|
| `ruby` | `x86_64-apple-darwin` | macOS Intel ruby toolchain build path is broken pending fix |

### Re-enabling a suspended (lang, target) pair

Remove the entry from `suspendedPrebuilts` in
[`sdkprebuilts.go`](../holons/grace-op/internal/sdkprebuilts/sdkprebuilts.go),
verify the per-SDK build script handles that target, then validate with
`op sdk build <lang> --target <target>`. Re-test downstream consumers
(gabriel composites, etc.) before claiming "supported".

## macOS toolchain workarounds (zigcxx + zig build)

Zig's clang wrapper does NOT auto-propagate the macOS SDK paths that real
clang infers from `-isysroot`. Two layers of fix are needed.

**1. zigcc / zigcxx wrappers** (`.zig-prebuilt/<target>/toolchain/`) — used by
cmake when compiling gRPC + abseil + c-ares + boringssl:

| Failure | Root cause | Fix in [`build-prebuilt-zig.sh`](../.github/scripts/build-prebuilt-zig.sh) |
|---|---|---|
| `'CoreFoundation/CFTimeZone.h' file not found` | Framework search path missing | `-F ${SDK}/System/Library/Frameworks` |
| `'libDER/DERItem.h' file not found` (chained from `<Security/oids.h>`) | SDK's `usr/include/` not on the include path | `-isystem ${SDK}/usr/include` |

The script computes these into `macos_framework_flag` once and folds them into
`extra_cflags` for every darwin target. If you hit a similar "header X not found"
in a future SDK / abseil bump, extend that variable rather than patching
individual call sites.

**2. `zig build` direct invocation** (the holons_zig library link step,
not via the cc wrapper) — needs the SDK paths exposed via the `SDKROOT` env
var that [`build.zig`](../sdk/zig-holons/build.zig) reads:

| Failure | Root cause | Fix |
|---|---|---|
| `unable to find dynamic system library 'resolv'` | Zig has no system lib path for macOS without explicit flags | `SDKROOT=$(xcrun --show-sdk-path)` in build script + `mod.addLibraryPath(${SDKROOT}/usr/lib)` in build.zig |
| `unable to find framework 'CoreFoundation'` | Same root cause for frameworks | Same env var + `mod.addFrameworkPath(${SDKROOT}/System/Library/Frameworks)` in build.zig |

`build.zig` only adds the paths when `SDKROOT` is set, so non-darwin targets
remain unaffected. The build script sets `SDKROOT` automatically for any
`*-apple-darwin` target.

## Build script invocation contract

`op sdk build` invokes each script with the following environment:

| Variable | Value |
|---|---|
| `SDK_TARGET` | The normalised target triplet (host if `--target` not given) |
| `SDK_VERSION` | The pinned default for this lang, or `--version` if provided |
| `OP_SDK_PROTOC` | Absolute path to `$OPPATH/sdk/shared/protoc/<version>/bin/protoc`, materialised by `op sdk install` before the script runs (empty for pure-plugin SDKs that declare `protoc: null` in their manifest) |
| `OP_SDK_PROTOC_INCLUDE` | Companion include directory for well-known types (`$OPPATH/sdk/shared/protoc/<version>/include`) |
| `<LANG>_HOLONS_JOBS` | `--jobs N` if non-zero (e.g. `ZIG_HOLONS_JOBS=8`) |
| `MACOSX_DEPLOYMENT_TARGET` | Defaults to `11.0` if unset (darwin only) |
| `ZIG`, `RUBY`, `BUNDLE` | Forwarded from the parent process if set |

Scripts MUST consume `$OP_SDK_PROTOC` rather than calling `protoc` from
PATH. Hardcoded `PROTOC_VERSION=...` exports inside build scripts are
deprecated — the version is the SDK manifest's responsibility.

Output tarball lands at:

```text
dist/sdk-prebuilts/<lang>/<target>/<lang>-holons-v<version>-<target>.tar.gz
dist/sdk-prebuilts/<lang>/<target>/<lang>-holons-v<version>-<target>.tar.gz.sha256
dist/sdk-prebuilts/<lang>/<target>/<lang>-holons-v<version>-<target>.spdx.json
```

`op sdk build` then installs the tarball via the existing `Install()` path
unless `--no-install` is passed (useful for iterating on the build script
without polluting `$OPPATH/sdk/`).

## Maintainer flows

```bash
# First-time check from a fresh clone:
git submodule update --init --recursive
op sdk list --compilable                   # which SDKs build right now? what blocks each?

# Build a SDK from local sources:
op sdk build zig                           # ~30-60 min cold
op sdk path zig                            # confirm install
op sdk verify zig                          # tree hash check

# Re-build after editing SDK sources:
op sdk build zig --force                   # bypass the cached tarball

# Bump version and ship a release tarball without installing locally:
op sdk build zig --version 0.2.0 --no-install
ls dist/sdk-prebuilts/zig/aarch64-apple-darwin/
```

## Cross-references

- [`OP_SDK.md`](../holons/grace-op/OP_SDK.md) — `op sdk` CLI reference
- [`OP_BUILD.md`](../holons/grace-op/OP_BUILD.md) — how `op build` integrates `requires.sdk_prebuilts`
- [`scripts/generate-protos.sh`](./scripts/generate-protos.sh) — canonical core proto regen (not for prebuilts)
- [`README.md`](./README.md) — high-level overview of all 14 SDKs
