# GitHub Workflows

`pipeline.yml` is the single pre-merge and master pipeline. It always proves the
core tools first, then gate 7 decides whether the run needs fresh SDK prebuilts.

```text
pull_request / push on master / workflow_dispatch
        |
        v
  gates 1-6: op, ader, bootstrap, smoke
        |
        v
  gate-7-classify-changes
        |
        +-- sdk_source=false
        |     skip sdk-build-* and sdk-deep-tests
        |     op sdk install all
        |     composites + local-dev bouquet
        |
        +-- sdk_source=true
              sdk-build-* matrix
              run-local artifacts
              composites + local-dev bouquet
              sdk-deep-tests (zig host only)
              master push only: auto-bump seed_release
```

## Classification

Gate 7 uses `.github/scripts/sdk_ci_paths.go`:

- Markdown, images, `README*`, `LICENSE*`, `CHANGELOG*`, and `docs/**` under
  `sdk/**` are not SDK source.
- SDK directories, `seed-toolchain.yaml`, `.gitmodules`, build-prebuilt scripts,
  shared prebuilt helpers, SDK prebuilt runtime code, and protoc adapter/noop
  generators are SDK source.
- `seed-toolchain.yaml` or transverse prebuilt tooling republishes all 14 SDKs.
- `sdk/cpp-holons/**` republishes `cpp,c,ruby,python,csharp,kotlin,java,js`.
- `sdk/zig-holons/third_party/grpc/**` republishes
  `zig,cpp,c,ruby,python,csharp,kotlin,java,js`.

## Job Shape

| Area | `sdk_source=false` | `sdk_source=true` |
|---|---|---|
| Gates 1-6 | yes | yes |
| SDK build matrix | no | yes |
| SDK source for downstream jobs | published releases via `op sdk install all` | artifacts from the same run |
| Composites | SwiftUI, Flutter | SwiftUI, Flutter |
| Bouquet | `local-dev` | `local-dev` |
| SDK deep tests | no | `zig, host` |

`local-dev` includes the `op_invoke` smoke suite. `sdk-deep-tests` keeps only
Zig host checks: `zig fmt --check`, `zig build vendor`, `zig build test`,
`zig build test-c-abi`, and the `op build/test/clean/build gabriel-greeting-zig`
lifecycle. Cross-compilation remains covered by the `sdk-build-zig` target
matrix.

## SDK Versions

All SDK prebuilt scripts default `SDK_VERSION` from `seed_release` in
`seed-toolchain.yaml`. Explicit `SDK_VERSION` values still override this for
ad hoc local builds. The reusable `_sdk-prebuilt-target.yml` workflow therefore
passes an empty version by default.

## Auto-Bump Policy

On successful `push` runs to `master` where gate 7 reports `sdk_source=true`,
`auto-bump-seed-release` runs after SDK builds, composites, the bouquet, and
Zig deep tests all pass.

The job:

- uses `SEED_AUTOBUMP_TOKEN`, which must have `contents:write`, `actions:write`,
  and permission to bypass protection on `master`;
- serializes through concurrency group `auto-bump-seed-release`;
- fetches and resets to latest `origin/master` immediately before reading
  `seed_release`;
- bumps the patch segment, for example `0.7.0` to `0.7.1`;
- commits `ci: bump seed_release to <version> [skip ci]`;
- retries stale-base push rejection up to 5 times.

If the merged PR already changed `seed_release`, the job does not create another
bump. In that case `sdk-prebuilts.yml` promotes the matching PR artifacts.

Run `25671995484` used the previous minor-bump policy and may move
`seed_release` from `0.7.0` to `0.8.0`. Treat that as a one-off legacy release;
do not roll it back. After this patch policy lands, the next SDK-source merge
should bump `0.8.0` to `0.8.1`.

Because `[skip ci]` prevents a normal push-triggered build for the bump commit,
the auto-bump job dispatches `pipeline.yml` on `master` with release inputs.

## Release Intent

Dispatched release builds upload a `release-intent` artifact containing
`release-intent.json`:

```json
{
  "version": "0.8.0",
  "publish_set": ["c", "cpp", "csharp", "dart", "go", "java", "js", "js-web", "kotlin", "python", "ruby", "rust", "swift", "zig"],
  "source_run_id": "123456789",
  "source_commit": "abcdef..."
}
```

`sdk-prebuilts.yml` listens for successful completed `pipeline` workflow runs.
If a completed workflow-dispatch run has `release-intent.json`, it downloads
that run's `sdk-*` artifacts and promotes only `publish_set`.

`sdk-prebuilts.yml` also keeps the `pull_request: closed` path for manual
`seed_release` PRs. Non-manual SDK-source merges do not publish their PR
artifacts directly; the auto-bump dispatch owns the new versioned release.

## Maintenance Contract

Any PR that changes workflow names, job dependencies, path-classification rules,
SDK publish-set rules, auto-bump behavior, or SDK deep-test coverage must update
this file and the relevant Go tests under `.github/scripts/` in the same commit.
