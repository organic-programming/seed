# TASK03 — 6-Phase Execution Engine

## Objective

Implement the `op setup` command with its 6-phase execution:
resolve → toolchains → system deps → holons → environment → mesh.

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`

## Reference

- [DESIGN_setup.md](./DESIGN_setup.md) — §Execution Flow, §Idempotency

## Scope

### Phase 1: Resolve

- Parse image file (TASK01)
- Build dependency graph (TASK02)
- Display plan summary

### Phase 2: Toolchains

- Install or verify Go, Rust, Node, etc.
- Download from official sources (golang.org, rustup.rs, etc.)
- Version check: skip if already installed at correct version

### Phase 3: System dependencies

- Install via platform package manager
- Bootstrap driver for bare machines
- Skip already-present packages

### Phase 4: Holons

- Install each holon using best method:
  prebuilt binary → `go install` / `cargo install` → source build
- `op build` + `op install` for source builds
- Handle `requires.sources` (clone + build)

### Phase 5: Environment

- Verify `OPPATH`, `OPBIN`, `PATH`
- Output shell config if needed

### Phase 6: Mesh (optional)

- If `mesh.join` present: `op mesh add --deploy`

### Idempotency

- Skip already-installed items (version check)
- Upgrade outdated versions
- Additive only (no automatic removal)

### CLI

```bash
op setup dev.yaml           # specific image
op setup                    # ./setup.yaml or ~/.op/setup.yaml
op setup --dry-run          # show plan, don't execute
```

## Acceptance Criteria

- [ ] All 6 phases execute in order
- [ ] Idempotent: safe to run multiple times
- [ ] `--dry-run` shows plan without acting
- [ ] Source compilation works (clone + build)
- [ ] Mesh join triggered when configured
- [ ] `go test ./...` — zero failures

## Dependencies

TASK01, TASK02, v0.8 (registry for prebuilt binaries), v0.9 (mesh join).
