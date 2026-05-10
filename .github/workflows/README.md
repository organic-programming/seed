# GitHub Workflows

The pre-merge CI is split by intent:

- `sdk-source-pipeline` runs when SDK source, SDK prebuilt tooling, the central
  toolchain pin, `.gitmodules`, or SDK-prebuilt runtime code changes.
- `consumer-pipeline` runs when those source surfaces are untouched. It validates
  consumers against published SDK prebuilts installed with `op sdk install`.

Both workflows contain a `classify-changes` guard backed by
`.github/scripts/sdk_ci_paths.py`. The native GitHub `paths:` filter keeps the
source workflow quiet for obvious non-source changes; the guard is the final
mutual-exclusion check when filters overlap or a workflow is dispatched.

```text
SDK source PR
  sdk-source-pipeline
    gates 1-6
    sdk-build-* prebuilt matrix
    composites + local-dev bouquet from run-local artifacts
    sdk-deep-tests host matrix

Consumer/doc PR
  consumer-pipeline
    gates 1-6
    op sdk install <all official SDKs>
    composites + local-dev bouquet from published prebuilts

Merged SDK source PR
  sdk-prebuilts
    pull_request closed context
    matching successful sdk-source-pipeline run
    publish-set expansion from the merged PR diff
    GitHub Release promotion
```

## Path Split

`sdk-source-pipeline` is selected for:

- `sdk/**`, excluding Markdown, images, `README*`, `LICENSE*`, `CHANGELOG*`, and
  `docs/**`.
- `seed-toolchain.yaml`, `.gitmodules`, `.github/scripts/build-prebuilt-*.sh`,
  `.github/scripts/lib-codegen-prebuilt.sh`, `.github/scripts/seed_toolchain.py`,
  and `.github/scripts/sdk_ci_paths.py`.
- `holons/grace-op/internal/sdkprebuilts/**`, excluding Markdown.
- `holons/grace-op/cmd/protoc-gen-op-adapter/**`, excluding Markdown.

Everything else is consumer-facing and belongs to `consumer-pipeline`.

## Job Shape

| Area | `sdk-source-pipeline` | `consumer-pipeline` |
|---|---|---|
| Gates 1-6 | yes | yes |
| SDK prebuilt build matrix | yes | no |
| SDK installation for downstream jobs | run-local artifacts | `op sdk install <lang>` |
| Composites | SwiftUI, Flutter | SwiftUI, Flutter |
| `ader-bouquet-full` | `local-dev` | `local-dev` |
| `sdk-deep-tests` | host-only SDK matrix | no |

`local-dev` includes the invoke smoke suite, so that coverage runs once in
`ader-bouquet-full`. It must not be duplicated inside `sdk-deep-tests`.

## SDK Deep Tests

The deep-test matrix is host-only. Cross-compile validation lives in the
`sdk-build-*` prebuilt matrix.

Full host coverage runs SDK-native tests plus `op build`, `op test`,
`op clean`, and a final `op build` for the matching example holon:

`zig`, `go`, `python`, `dart`, `java`, `kotlin`, `csharp`, `js`, `ruby`,
`rust`, `swift`, `c`, and `cpp`.

`js-web` is SDK-suite only (`node --test`) because the browser-only SDK has no
`gabriel-greeting-js-web` holon on `master`.

## Release Coordination

`sdk-prebuilts.yml` stays on `pull_request: closed` so it retains PR number,
merge status, and head SHA context. On merge, it:

1. Computes the changed-file list with `gh pr diff <number> --name-only`.
2. Uses `.github/scripts/sdk_ci_paths.py publish-set` to choose SDKs:
   - central pin or transverse tooling change: all 14 SDKs;
   - `sdk/cpp-holons/**`: `cpp` plus `c`, `ruby`, `python`, `csharp`,
     `kotlin`, `java`, and `js`;
   - `sdk/zig-holons/third_party/grpc/**`: `zig`, `cpp`, and `c`;
   - otherwise, the directly touched SDKs.
3. Looks for a successful `sdk-source-pipeline` pull-request run whose head SHA
   matches the merged PR head.
4. Exits cleanly with nothing to promote when no matching source run exists.
5. Downloads that run's artifacts and promotes only the selected SDK archives.

All `build-prebuilt-<lang>.sh` scripts default `SDK_VERSION` from
`seed_release` in `seed-toolchain.yaml`, so promoted tags are mechanically
aligned as `<lang>-holons-v<seed_release>` across all 14 SDKs. Maintainers may
still set `SDK_VERSION` explicitly for ad hoc builds.

## Maintenance Contract

Any PR that changes workflow names, job dependencies, path-classification rules,
SDK publish-set rules, or SDK deep-test coverage must update this file and the
unit tests for `.github/scripts/sdk_ci_paths.py` in the same commit.
