# TASK02 — UpdateToolchain RPC Implementation

## Context

The `UpdateToolchain` RPC lets callers replace the active Go
version. It downloads, verifies, and installs the target
version, then prunes the old one.

## Objective

Implement the server-side handler for `UpdateToolchain`.

## Changes

### `internal/toolchain/update.go` [NEW]

```go
// Update resolves the target version, provisions it, swaps the
// `current` symlink, and prunes the old version directory.
// Returns the previous and current version strings.
func (t *Toolchain) Update(ctx context.Context, target string) (prev, curr string, err error)

// ResolveLatest fetches https://go.dev/dl/?mode=json and returns
// the newest stable release version string.
func ResolveLatest(ctx context.Context) (string, error)
```

Key behaviors:
- If `target == "latest"`: call `ResolveLatest` first
- If `target == current`: return no-op (was_noop = true)
- Call `Ensure(target)` to download + verify
- Update `current` symlink to new version
- Remove old version directory
- Rewrite `delegates.toolchain.version` in `holon.yaml`

### `internal/toolchain/update_test.go` [NEW]

- `TestResolveLatest` — mock HTTP, verify version parsing
- `TestUpdateSwapsSymlink` — verify symlink points to new version
- `TestUpdatePrunesOld` — verify old directory removed
- `TestUpdateNoop` — same version returns was_noop

### `internal/service/service.go`

Add `UpdateToolchain` method to `GoServer`.

## Acceptance Criteria

- [ ] `UpdateToolchain("latest")` resolves and provisions newest Go
- [ ] `current` symlink updated after success
- [ ] Old version directory pruned
- [ ] `holon.yaml` version field rewritten
- [ ] No-op when target equals current version
- [ ] Fails cleanly on network errors (no partial state)

## Dependencies

TASK01 (proto stubs). Builds on v0.1 TASK01 (`Ensure`).
