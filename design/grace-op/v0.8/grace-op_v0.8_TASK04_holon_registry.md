# TASK04 — Holon Registry

## Objective

Implement the artifact storage and index service that `op publish`
uploads to and `op install` downloads from.

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`
- New repo (if self-hosted): `github.com/organic-programming/holon-registry`

## Reference

- [DESIGN_release_pipeline.md](./DESIGN_release_pipeline.md) — §Holon Registry

## Scope

### Registry backends

Support at least one, design for pluggability:

| Backend | Storage | Index |
|---|---|---|
| GitHub Releases | Release assets | GitHub API |
| S3 / GCS | Object storage | JSON index file |
| OCI Registry | Container tags | OCI manifest |

### Index format

```json
{
  "name": "greeting-daemon-go",
  "version": "0.3.0",
  "artifacts": [
    {"platform": "darwin-arm64", "url": "...", "sha256": "..."},
    {"platform": "windows-amd64", "url": "...", "sha256": "..."}
  ]
}
```

### API (from grace-op perspective)

- `registry.Publish(artifact, metadata)` — upload
- `registry.Resolve(name, version, platform)` — download URL
- `registry.List(name)` — available versions

## Acceptance Criteria

- [ ] At least one backend implemented (GitHub Releases recommended)
- [ ] Publish: upload artifact + update index
- [ ] Resolve: return correct artifact URL for platform
- [ ] List: return available versions
- [ ] Index includes checksums
- [ ] `go test ./...` — zero failures

## Dependencies

TASK01 (naming convention).
