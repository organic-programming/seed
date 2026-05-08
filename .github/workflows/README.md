# GitHub Workflows

The pre-merge workflow is intentionally ordered: prove the organic-programming tools first, build fresh run-local SDK prebuilts, then run domain coverage from artifacts produced by the same run.

```text
pull_request / push on master
        |
        v
  gate-1-go-build-op (self-hosted macOS, fail-fast)
        |
  gate-2-op-self-build (self-hosted macOS, fail-fast)
        |
  gate-3-op-core-tests (self-hosted macOS, fail-fast)
        |
  gate-3a-go-codegen-sdk (self-hosted macOS, bootstrap artifact)
        |
  gate-4-op-build-ader (self-hosted macOS, fail-fast)
        |
  gate-5-ader-core-tests (self-hosted macOS, fail-fast)
        |
  gate-6-smoke-suite (self-hosted macOS, fail-fast)
        |
        v
  tier 1 wave A: sdk-build-{zig,cpp} (hosted/macOS)
        |
  tier 1 wave B: sdk-build-{c,ruby,go,python,csharp,java,kotlin,dart,swift,js}
        |
  fresh-sdk-delivery (ubuntu-hosted pass-through gate)
        |
        +--------------------------+
        |                          |
        v                          v
  tier 2 composites          tier 3 sdk-deep-tests
  swiftui / flutter /        zig host + cross-smoke
  ader bouquet (macOS)       matrix (macOS)

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
| `gate-1-go-build-op` | self-hosted macOS | same-repo PR, push, dispatch | blocks all later jobs | <1 min |
| `gate-2-op-self-build` | self-hosted macOS | `needs: gate-1` | blocks all later jobs | <1 min |
| `gate-3-op-core-tests` | self-hosted macOS | `needs: gate-2` | blocks all later jobs | 1-3 min |
| `gate-3a-go-codegen-sdk` | self-hosted macOS | `needs: gate-3` | uploads `sdk-go-bootstrap-<target>` for the ader build manifest | <1 min |
| `gate-4-op-build-ader` | self-hosted macOS | `needs: gate-3a-go-codegen-sdk` | blocks all later jobs | <1 min |
| `gate-5-ader-core-tests` | self-hosted macOS | `needs: gate-4` | blocks all later jobs | 1-2 min |
| `gate-6-smoke-suite` | self-hosted macOS | `needs: gate-5` | blocks all later jobs | 2-3 min |
| `sdk-build-zig` | hosted/macOS | `needs: gate-6` | uploads `sdk-zig-<target>` | 10-30 min |
| `sdk-build-cpp` | hosted/macOS | `needs: gate-6` | uploads `sdk-cpp-<target>` and `cpp-prefix-<target>` | 10-30 min |
| `sdk-build-c` | hosted/macOS | `needs: sdk-build-cpp` | downloads `cpp-prefix-<target>`, uploads `sdk-c-<target>` | 10-30 min |
| `sdk-build-ruby` | hosted/macOS | `needs: sdk-build-cpp` | downloads `cpp-prefix-<target>`, uploads `sdk-ruby-<target>` | 10-30 min |
| `sdk-build-{go,python,csharp,java,kotlin,dart,swift,js}` | hosted macOS | `needs: sdk-build-cpp` | uploads `sdk-<lang>-<target>` | 5-30 min |
| `fresh-sdk-delivery` | ubuntu-latest | after tier 1 | pass-through once run-local SDK artifacts exist | <1 min |
| `composite-swiftui-full` | self-hosted macOS | `needs: fresh-sdk-delivery` | fail-isolated PR check | 1-2 h |
| `composite-flutter-full` | self-hosted macOS | `needs: fresh-sdk-delivery` | fail-isolated PR check | 1-2 h |
| `ader-bouquet-full` | self-hosted macOS | `needs: fresh-sdk-delivery` | fail-isolated PR check | 1-3 h |
| `sdk-deep-tests` | self-hosted macOS | `needs: fresh-sdk-delivery` | fail-isolated matrix | 1-3 h |
| `sdk-prebuilts.yml/promote` | ubuntu-latest | merged PR only | publishes releases | <10 min |
| `ader-bench.yml/bench` | self-hosted macOS | dispatch | uploads objective bouquet timing and failure diagnostics | no performance target |

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
| zig cross-smoke | `aarch64-linux-android` | omitted | NDK/linker setup is not stable on the self-hosted macOS runner |

## Inter-SDK dependency: cpp prefix

Only SDKs that need native gRPC artifacts consume files built by the cpp prebuilt job:

- `c` needs the cpp prefix libraries, `protoc`/upb generators, and the matching CMake toolchain file.
- `ruby` needs `grpc_ruby_plugin` because the `grpc-tools arm64-darwin` gem upstream does not ship it.
- `zig` is independent: its prebuilt script builds and consumes its own prefix under `sdk/zig-holons/.zig-prebuilt/<target>/prefix/`.
- `go`, `python`, `csharp`, `java`, `kotlin`, `dart`, `swift`, and `js` are codegen-light SDKs. They fetch `protoc` and language plugins directly and do not read the cpp prefix.

The cpp build configures `gRPC_BUILD_GRPC_<LANG>_PLUGIN=ON` so each plugin lands in `sdk/cpp-holons/.cpp-prebuilt/<target>/prefix/bin/`, and it writes the target CMake toolchain under `sdk/cpp-holons/.cpp-prebuilt/<target>/toolchain/`.

To make these files available cross-job in CI:

1. The `sdk-build-cpp` job uploads `prefix/` and `toolchain/` as a dedicated artifact `cpp-prefix-<target>` (retention 1 day, just enough to feed downstream jobs in the same workflow run).
2. Each dependent job declares `needs: sdk-build-cpp` and passes `needs-cpp-prefix: true` to `_sdk-prebuilt-target.yml`.
3. The reusable workflow downloads `cpp-prefix-<target>` and extracts it under `sdk/cpp-holons/.cpp-prebuilt/<target>/` before the build script runs.

When adding a new adapter SDK that needs a cpp-side plugin, mirror the ruby pattern: `needs: sdk-build-cpp` + `needs-cpp-prefix: true`, and ensure the cpp build enables the matching `gRPC_BUILD_GRPC_<LANG>_PLUGIN` flag.

## CI cache configuration

The pipeline does not hardcode any cache path. Cache locations are pulled from `~/.op-mac.env` on each Mac runner (self-hosted), via the composite action `.github/actions/load-mac-runner-env`. Cloud runners ignore the file and rely on `actions/cache@v4` where persistence matters.

- **Self-hosted Mac runner provisioning**: install tooling via `scripts/setup-op-mac.sh`, then `cp scripts/op-mac.env.example ~/.op-mac.env`. **Nothing else to do** — every Mac job in the pipeline begins with `uses: ./.github/actions/load-mac-runner-env`, which sources the file and propagates the documented variables to `$GITHUB_ENV` for the rest of the job. The runner does not need to source the file from its shell rc.
- **Cloud runners**: no `~/.op-mac.env`, the composite action no-ops. `actions/cache@v4` covers cross-run cache where it matters (Go modules, Go build cache).
- **Adding a new Mac runner**: only step is `cp scripts/op-mac.env.example ~/.op-mac.env`. Adding a new variable to the contract: edit `scripts/op-mac.env.example` and the `for var in …` list inside `.github/actions/load-mac-runner-env/action.yml` in the same commit.

## Publish Semantics

Tier 1 uploads artifacts only for targets that build successfully. Matrix entries marked `continue-on-error` can fail without failing the workflow; the post-merge promote workflow downloads the merged PR's successful `pipeline.yml` run and publishes only `*-holons-v*.tar.gz` archives that are present. If no SDK archive exists, promotion exits cleanly.

## Maintenance Contract

Any PR that adds, removes, renames, or reorders jobs in `.github/workflows/pipeline.yml` must update this file in the same commit.
