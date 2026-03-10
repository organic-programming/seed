# TASK01 — `op install --no-build` flag

## Context

OP.md §7 specifies `--no-build` for `op install`. Today `op install`
always triggers a build when the artifact is missing. This flag
lets the user fail fast instead.

## Objective

Add `--no-build` flag to `op install`.

## Changes

### `internal/cli/install.go`

Parse `--no-build` flag. When set, skip the auto-build step and
fail if the artifact does not exist:

```
op install: artifact not found at .op/build/bin/<binary>; run op build first
```

## Acceptance Criteria

- [ ] `op install --no-build` fails if artifact missing
- [ ] `op install --no-build` succeeds if artifact exists
- [ ] `op install` without flag keeps current behavior (auto-build)
- [ ] `go test ./...` — zero failures

## Dependencies

None.
