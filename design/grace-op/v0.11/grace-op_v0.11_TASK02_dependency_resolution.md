# TASK02 — Dependency Resolution Engine

## Objective

Implement the dependency graph resolver that reads `holon.yaml`
manifests from all declared holons and builds a full installation
plan (toolchains, system commands, sources, holons).

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`

## Reference

- [DESIGN_setup.md](./DESIGN_setup.md) — §Dependency Resolution, §Dependency Graph

## Scope

### Resolution input

- `setup.yaml` → list of holons + toolchains
- Each `holon.yaml` → `requires.commands`, `delegates.commands`,
  `requires.sources`, `build.runner`

### Resolution output

Ordered installation plan:
1. Toolchains (go, rust, node, etc.)
2. System commands (cmake, make, git, etc.)
3. Source builds (whisper.cpp, ffmpeg, etc.)
4. Holons (in dependency order)

### Bootstrapping

- Built-in minimal package manager driver (hardcoded
  `brew install`, `apt install`, `winget install`)
- Installs package manager holon first if needed
- Further installs go through the holon

## Acceptance Criteria

- [ ] Graph built from image + holon manifests
- [ ] Circular dependencies detected and reported
- [ ] Installation order respects dependencies
- [ ] Platform-specific holons filtered correctly
- [ ] `go test ./...` — zero failures

## Dependencies

TASK01 (image parser), v0.8 (registry for fetching manifests).
