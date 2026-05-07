# GitHub Workflows

The pre-merge workflow is intentionally ordered: prove the organic-programming tools first, rebuild changed SDK prebuilts, then run domain coverage.

```text
pull_request / push on master
        |
        v
  gate-1-go-build-op (popok, fail-fast)
        |
  gate-2-op-self-build (popok, fail-fast)
        |
  gate-3-op-core-tests (popok, fail-fast)
        |
  gate-4-op-build-ader (popok, fail-fast)
        |
  gate-5-ader-core-tests (popok, fail-fast)
        |
  gate-6-smoke-suite (popok, fail-fast)
        |
        v
  detect-sdk-changes (ubuntu-hosted)
        |
        v
  tier 1: sdk-prebuilt-{zig,cpp,c,ruby} (hosted/macOS, conditional)
        |
  sdk-prebuilts-fresh (ubuntu-hosted pass-through gate)
        |
        +--------------------------+
        |                          |
        v                          v
  tier 2 composites          tier 3 sdk-deep-tests
  swiftui / flutter /        zig host + cross-smoke
  ader bouquet (popok)       matrix (popok)

post-merge:
  sdk-prebuilts.yml promote downloads successful tier-1 artifacts from the
  merged PR's pipeline run and publishes only archives that actually exist.

benchmark/maintenance:
  ader-bench.yml is a manual diagnostic workflow that runs an ader bouquet,
  separates runner wait from executed time, and uploads functional success /
  failure reports. It does not enforce performance targets.
```

## Jobs

| Job | Runner | Trigger | Downstream effect | Warm target |
|---|---|---|---|---|
| `gate-1-go-build-op` | popok | same-repo PR, push, dispatch | blocks all later jobs | <1 min |
| `gate-2-op-self-build` | popok | `needs: gate-1` | blocks all later jobs | <1 min |
| `gate-3-op-core-tests` | popok | `needs: gate-2` | blocks all later jobs | 1-3 min |
| `gate-4-op-build-ader` | popok | `needs: gate-3` | blocks all later jobs | <1 min |
| `gate-5-ader-core-tests` | popok | `needs: gate-4` | blocks all later jobs | 1-2 min |
| `gate-6-smoke-suite` | popok | `needs: gate-5` | blocks all later jobs | 2-3 min |
| `detect-sdk-changes` | ubuntu-latest | `needs: gate-6` | decides tier-1 SDK jobs | <1 min |
| `sdk-prebuilt-zig` | hosted/macOS | SDK paths or workflow changed | blocks tier 2/3 if required and failed | 10-30 min |
| `sdk-prebuilt-cpp` | hosted/macOS | SDK paths or workflow changed; uploads `cpp-prefix-<target>` artifact for adapter SDKs | blocks tier 2/3 if required and failed | 10-30 min |
| `sdk-prebuilt-c` | hosted/macOS | SDK paths or workflow changed | blocks tier 2/3 if required and failed | 10-30 min |
| `sdk-prebuilt-ruby` | hosted/macOS | SDK paths or workflow changed; `needs: sdk-prebuilt-cpp`, downloads `cpp-prefix-<target>` | blocks tier 2/3 if required and failed | 10-30 min |
| `sdk-prebuilts-fresh` | ubuntu-latest | after tier 1 | pass-through/no-op when no SDK changed | <1 min |
| `composite-swiftui-full` | popok | `needs: sdk-prebuilts-fresh` | fail-isolated PR check | 1-2 h |
| `composite-flutter-full` | popok | `needs: sdk-prebuilts-fresh` | fail-isolated PR check | 1-2 h |
| `ader-bouquet-full` | popok | `needs: sdk-prebuilts-fresh` | fail-isolated PR check | 1-3 h |
| `sdk-deep-tests` | popok | `needs: sdk-prebuilts-fresh` | fail-isolated matrix | 1-3 h |
| `sdk-prebuilts.yml/promote` | ubuntu-latest | merged PR only | publishes releases | <10 min |
| `ader-bench.yml/bench` | popok | dispatch | uploads objective bouquet timing and failure diagnostics | no performance target |

## Failed Check

1. Find the tier. Gate failures mean `op` or `clem-ader` is broken and later checks are intentionally skipped. Tier-1 failures mean changed SDK prebuilts did not build. Tier-2 or tier-3 failures are domain coverage failures.
2. Check whether the failing matrix entry is `continue-on-error`. Those entries do not block the workflow conclusion and are documented below.
3. Fix in the owning layer: Go/tooling for gate jobs, SDK source or build scripts for tier 1, catalogues/examples for tier 2, SDK test tooling for tier 3.

## Suspended Targets

| Area | Target | State | Reason |
|---|---|---|---|
| zig prebuilt | `x86_64-apple-darwin` | omitted | target needs end-to-end validation |
| cpp prebuilt | `x86_64-apple-darwin` | omitted | target needs end-to-end validation |
| c prebuilt | `x86_64-apple-darwin` | omitted | target needs end-to-end validation |
| ruby prebuilt | `x86_64-apple-darwin` | omitted | grpc native gem build fails on Intel mac with incompatible function pointer types |
| zig/cpp/c/ruby prebuilts | linux and windows matrix entries | `continue-on-error` | target needs validation before gating |
| zig cross-smoke | `aarch64-linux-android` | omitted | NDK/linker setup is not stable on popok |

## Inter-SDK dependency: cpp prefix

Several adapter SDKs need a gRPC plugin binary that lives next to the cpp toolchain (`grpc_<lang>_plugin`):

- `ruby` needs `grpc_ruby_plugin` because the `grpc-tools arm64-darwin` gem upstream does not ship it.
- `python`, `csharp`, `java`, `kotlin` adapter SDKs follow the same pattern when their native ecosystem (pip / NuGet / Maven) does not provide the plugin on a given target.

The cpp build configures `gRPC_BUILD_GRPC_<LANG>_PLUGIN=ON` so each plugin lands in `sdk/cpp-holons/.cpp-prebuilt/<target>/prefix/bin/`.

To make these binaries available cross-job in CI:

1. The `sdk-prebuilt-cpp` job uploads the prefix as a dedicated artifact `cpp-prefix-<target>` (retention 1 day, just enough to feed downstream jobs in the same workflow run).
2. Each dependent adapter job (today: `sdk-prebuilt-ruby`) declares `needs: sdk-prebuilt-cpp` and passes `needs-cpp-prefix: true` to `_sdk-prebuilt-target.yml`.
3. The reusable workflow downloads `cpp-prefix-<target>` and extracts it under `sdk/cpp-holons/.cpp-prebuilt/<target>/prefix/` before the build script runs, so the existing sibling-lookup logic in `lib-codegen-prebuilt.sh` finds the plugin natively.

When adding a new adapter SDK that needs a cpp-side plugin, mirror the ruby pattern: `needs: sdk-prebuilt-cpp` + `needs-cpp-prefix: true`, and ensure the cpp build enables the matching `gRPC_BUILD_GRPC_<LANG>_PLUGIN` flag.

## Publish Semantics

Tier 1 uploads artifacts only for targets that build successfully. Matrix entries marked `continue-on-error` can fail without failing the workflow; the post-merge promote workflow downloads the merged PR's successful `pipeline.yml` run and publishes only `*-holons-v*.tar.gz` archives that are present. If no SDK archive exists, promotion exits cleanly.

## Maintenance Contract

Any PR that adds, removes, renames, or reorders jobs in `.github/workflows/pipeline.yml` must update this file in the same commit.
