# TASK02 — `op publish` Command

## Objective

Implement `op publish` — builds holons for all declared platforms
and uploads artifacts to the holon registry.

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`

## Reference

- [DESIGN_release_pipeline.md](./DESIGN_release_pipeline.md) — §`op publish` Command

## Scope

### CLI

```bash
op publish                         # all platforms
op publish --platform darwin-arm64 # single platform
op publish --dry-run               # build only, no upload
```

### Execution

1. Read `build.publish.platforms` from `holon.yaml`
2. For each platform: invoke `op build --target <platform>`
3. Name artifacts per naming convention (TASK01)
4. Compute SHA256 checksums
5. Upload to registry (TASK04)
6. Update registry index

### Error handling

- Platform build failure → skip, report at end
- Partial success → upload what succeeded, list failures

## Acceptance Criteria

- [ ] `op publish` builds for all declared platforms
- [ ] `op publish --platform` builds single platform
- [ ] `op publish --dry-run` builds without uploading
- [ ] Checksums computed and included in upload
- [ ] Partial failure handling works
- [ ] `go test ./...` — zero failures

## Dependencies

TASK01, v0.7 TASK02 (`op build --target`).
