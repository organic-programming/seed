# TASK03 — `op install` Platform Resolution

## Objective

Extend `op install` to resolve and download pre-compiled
platform-specific binaries from the holon registry, with
fallback to source build.

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`

## Reference

- [DESIGN_release_pipeline.md](./DESIGN_release_pipeline.md) — §`op install` Platform Resolution, §Fallback to Source

## Scope

### Resolution chain

1. Query registry for `<holon>@<version>`
2. Detect current platform (`runtime.GOOS` + `runtime.GOARCH`)
3. Match against available artifacts
4. If binary exists → download, verify checksum, install to `$OPBIN`
5. If no binary → fallback: clone source + `op build`

### CLI additions

```bash
op install greeting-daemon-go           # latest, current platform
op install greeting-daemon-go@0.3.0     # specific version
op install greeting-daemon-go --source  # force build from source
```

### Caching

- Downloaded binaries cached in `~/.op/cache/artifacts/`
- Source builds cached in `~/.op/cache/builds/`

## Acceptance Criteria

- [ ] `op install` downloads correct platform binary
- [ ] Checksum verification on download
- [ ] Fallback to source when no binary available
- [ ] `--source` flag forces source build
- [ ] Version pinning works
- [ ] `go test ./...` — zero failures

## Dependencies

TASK01 (naming convention), TASK04 (registry).
