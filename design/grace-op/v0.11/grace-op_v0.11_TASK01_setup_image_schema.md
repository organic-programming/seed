# TASK01 — `setup.yaml` Image Schema + Parser

## Objective

Define and parse the `setup.yaml` image file format: name,
toolchains, holons, platform overrides, include, mesh.join.

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`

## Reference

- [DESIGN_setup.md](./DESIGN_setup.md) — §Image File, §Multi-Image

## Scope

### Schema

```yaml
name: developer
include: [base.yaml]
toolchains:
  go: "1.22"
  rust: "1.80"
holons:
  - rob-go
  - phill-files
platform:
  darwin:
    holons: [al-brew]
  windows:
    holons: [marvin-winget]
mesh:
  join: paris.example.com
```

### Parser

- Parse all fields into Go structs
- Resolve `include` (recursive, merge toolchains/holons)
- File resolution: `./setup.yaml` → `~/.op/setup.yaml`
- `op check` validates image files

### Merge rules for `include`

- `toolchains`: union, last wins on version conflict
- `holons`: union
- `platform`: merge per-OS lists

## Acceptance Criteria

- [ ] All fields parsed
- [ ] `include` composition works
- [ ] File resolution chain works
- [ ] `op check` validates image files
- [ ] `go test ./...` — zero failures

## Dependencies

None — foundation.
