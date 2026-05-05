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
| `sdk-prebuilt-cpp` | hosted/macOS | SDK paths or workflow changed | blocks tier 2/3 if required and failed | 10-30 min |
| `sdk-prebuilt-c` | hosted/macOS | SDK paths or workflow changed | blocks tier 2/3 if required and failed | 10-30 min |
| `sdk-prebuilt-ruby` | hosted/macOS | SDK paths or workflow changed | blocks tier 2/3 if required and failed | 10-30 min |
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

## Publish Semantics

Tier 1 uploads artifacts only for targets that build successfully. Matrix entries marked `continue-on-error` can fail without failing the workflow; the post-merge promote workflow downloads the merged PR's successful `pipeline.yml` run and publishes only `*-holons-v*.tar.gz` archives that are present. If no SDK archive exists, promotion exits cleanly.

## Maintenance Contract

Any PR that adds, removes, renames, or reorders jobs in `.github/workflows/pipeline.yml` must update this file in the same commit.
