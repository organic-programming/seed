# TASK01 — Auto Ad-Hoc Signing in Recipe Runner

## Objective

After assembling a bundle artifact, the recipe runner
automatically runs `codesign --force --deep --sign -` on it.

## Changes

### `internal/holons/runner_recipe.go`

After the final build step completes:

1. Read `artifacts.primary` from manifest
2. If path ends with `.app` or `.framework`:
   - Run `codesign --force --deep --sign - <path>`
   - Log: `signed (ad-hoc): <path>`
3. If `--no-sign` flag is set, skip

### `cmd/op/build.go`

Add `--no-sign` flag to `op build` command.

## Acceptance Criteria

- [ ] `.app` bundles auto-signed after assembly
- [ ] `_CodeSignature/CodeResources` created
- [ ] `--no-sign` skips the step
- [ ] Existing assemblies pass without modification

## Dependencies

v0.4.3 (assemblies must exist).
