# TASK07 — Document `op setup` and Host Provisioning

## Objective

Create specification documents for `op setup` (declarative host provisioning) and the `setup.yaml` image file format. Update `holon.yaml` to support source dependency declarations (`requires.sources`).

## Reference Documents

- [DESIGN_setup.md](./DESIGN_setup.md) — full design for `op setup`, dependency resolution, execution flow

---

## Changes

### 1. `OP.md` — Add `op setup` Command Reference

**Location**: new section after `op mesh` (§12 or adjacent).

- Overview: declarative provisioning, Docker-like approach
- `op setup <image.yaml>` — apply an image to the host
- `op setup` (no args) — apply `./setup.yaml` or `~/.op/setup.yaml`
- 6-phase execution: resolve → toolchains → system deps → holons → environment → mesh
- Idempotency: safe to run multiple times
- Examples: developer, builder, mesh node

---

### 2. `SETUP_YAML.md` — New Specification Document

**Location**: `organic-programming/SETUP_YAML.md` (alongside `HOLON_YAML.md`)

- Scope: *"What should this host have installed?"*
- Full schema with field table:
  - `name` (string, required) — image name
  - `include` (list of string, optional) — other images to compose
  - `toolchains` (map, optional) — `go: "1.22"`, `rust: "1.80"`, `node: "20"`
  - `holons` (list of string, required) — holons to install in OPBIN
  - `platform` (map, optional) — per-OS overrides (`darwin`, `windows`, `linux`)
  - `platform.<os>.holons` (list of string) — platform-specific holons
  - `mesh.join` (string, optional) — host to join in the mesh
- File location resolution: `./setup.yaml` → `~/.op/setup.yaml`
- Relationship to `holon.yaml` (setup lists holons, `op setup` reads their deps)
- Relationship to `mesh.yaml` (`mesh.join` triggers `op mesh add`)
- Examples: developer workstation, CI builder, minimal mesh node
- Include three-YAML cross-reference table

---

### 3. `HOLON_YAML.md` — Add `requires.sources` Schema

**Location**: within the `requires` field documentation.

New field:

| Field | Type | Required | Description |
|---|---|---|---|
| `requires.sources` | list | no | External source dependencies to clone and build |
| `requires.sources[].name` | string | yes | Human-readable name |
| `requires.sources[].repo` | string | yes | Git repository URL |
| `requires.sources[].ref` | string | yes | Git tag or commit SHA (branches rejected by `op check`) |
| `requires.sources[].build` | string | yes | Build system: `cmake`, `configure-make`, `cargo`, `go` |
| `requires.sources[].configure_args` | list of string | no | Arguments for `./configure` (configure-make only) |

**Pinning rule**: `ref` accepts tags (`v1.5.4`) or commit SHAs (`a1b2c3d4`). Floating branches (`master`) are rejected by `op check` for reproducibility.

**Cache**: `~/.op/cache/sources/<name>/` — cloned once, reused across `op setup` runs.

---

### 4. `CONVENTIONS.md` — Add `~/.op/cache/sources/` to Standard Directories

| Directory | Purpose |
|---|---|
| `~/.op/cache/sources/` | Cloned source dependencies for holons with `requires.sources` |

---

### Cross-Reference Table (include in all three YAML spec docs)

| File | Where | Who writes it | What it answers |
|---|---|---|---|
| `holon.yaml` | Each holon repo | Holon author | *"What does this holon need?"* |
| `setup.yaml` | Project / `~/.op/` | Operator | *"What should this host have?"* |
| `mesh.yaml` | `~/.op/mesh/` | `op mesh` (auto) | *"Who are the other hosts?"* |

---

## Checklist

- [ ] **OP.md**: Draft `op setup` section
- [ ] **SETUP_YAML.md**: Create new spec document (schema, examples, resolution rules)
- [ ] **HOLON_YAML.md**: Add `requires.sources` schema with pinning rule
- [ ] **CONVENTIONS.md**: Add `~/.op/cache/sources/` to standard directories
- [ ] Add three-YAML cross-reference table to `SETUP_YAML.md`, `MESH_YAML.md`, `HOLON_YAML.md`
- [ ] Review all changes for consistency with existing document tone and style

## Dependencies

- TASK06 must be completed first (creates `MESH_YAML.md` which is referenced here)
- Design reference: `DESIGN_setup.md`
