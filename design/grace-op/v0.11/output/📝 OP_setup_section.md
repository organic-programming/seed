# OP.md §12 — Setup (Draft)

> This section is designed to be inserted into `OP.md` as section 12,
> after "Mesh" (§11). Existing §12–§16 become §13–§17.

---

## 12. Setup

`op setup` provisions a host from a declarative image file. It
installs toolchains, resolves holon dependencies, builds holons
that require compilation, and configures the environment —
everything needed to go from a bare machine to a functioning OP host.

**Key principle**: like Docker, `op setup` is **declarative and
automatic**. You describe the desired state, it makes it happen.

### 12.1 Usage

```bash
op setup dev.yaml       # apply a specific image
op setup                # apply ./setup.yaml or ~/.op/setup.yaml
op setup --dry-run      # show plan, don't execute
```

File resolution: `./setup.yaml` → `~/.op/setup.yaml`.

### 12.2 Image File

An image file declares what should be present on this host:

```yaml
name: developer

toolchains:
  go: "1.22"
  rust: "1.80"

holons:
  - rob-go
  - phill-files
  - line-git
  - megg-ffmpeg

platform:
  darwin:
    holons: [al-brew]
  windows:
    holons: [marvin-winget]

mesh:
  join: paris.example.com
```

See [SETUP_YAML.md](./SETUP_YAML.md) for the full schema.

### 12.3 Execution Phases

`op setup` runs 6 phases in order:

| Phase | Action | Example |
|---|---|---|
| 1. Resolve | Build dependency graph from image + holon manifests | 4 holons, 2 toolchains, 3 system packages |
| 2. Toolchains | Install or verify Go, Rust, Node | `go 1.22` → download from golang.org |
| 3. System deps | Install via platform package manager | `cmake` → `brew install cmake` |
| 4. Holons | Build + install each holon | `rob-go` → `go install` → `$OPBIN` |
| 5. Environment | Verify `OPPATH`, `OPBIN`, `PATH` | Output shell config if needed |
| 6. Mesh | Join mesh if `mesh.join` configured | `op mesh add --deploy` |

### 12.4 Holon Installation Methods

| Kind | Method | Priority |
|---|---|---|
| Native (Go) | Prebuilt binary → `go install` → source build | Fastest first |
| Native (Rust) | Prebuilt binary → `cargo install` → source build | Fastest first |
| Native (C/C++) | Source build (clone + cmake/make) | Only option |
| Wrapper | Install delegated command via package manager | Via brew/apt/winget |

### 12.5 Idempotency

Running `op setup` multiple times is safe:
- Already-installed items are skipped (version check)
- Outdated versions are upgraded
- `op setup` is additive — it does not uninstall removed items
- To remove: explicit `op uninstall <holon>`

### 12.6 Multi-Image Composition

Images can include other images:

```yaml
name: builder
include: [dev.yaml]
holons:
  - wisupaa-whisper
```

Merge: toolchains union, holons union, platform merge. Conflicts:
last wins.

### 12.7 Relationship to Other Commands

| Command | Role |
|---|---|
| `op setup` | Provision the host (full dependency resolution) |
| `op build` | Compile a single holon (assumes deps present) |
| `op install` | Install a single holon binary to OPBIN |
| `op mesh` | Configure distributed topology |
| `op check` | Verify a single holon's prerequisites |
