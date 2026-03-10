# SETUP_YAML.md — Image File Specification (Draft)

> New specification document. Final location:
> `organic-programming/SETUP_YAML.md` (alongside `HOLON_YAML.md`).

---

# setup.yaml — Host Provisioning Image

A setup image declares the **desired state of a host** — which
toolchains, system packages, and holons should be installed.

`op setup` reads the image and makes it happen.

---

## Scope

*"What should this host have installed?"*

| Question | Answered by |
|---|---|
| Which toolchains? | `toolchains` |
| Which holons? | `holons` |
| Platform-specific additions? | `platform.<os>.holons` |
| Compose from other images? | `include` |
| Join a mesh? | `mesh.join` |

---

## Location

```
./setup.yaml              # project-local (priority 1)
~/.op/setup.yaml          # user default (priority 2)
op setup <path>           # explicit override
```

---

## Schema

### Full example

```yaml
# dev.yaml — developer workstation
name: developer

include: [base.yaml]

toolchains:
  go: "1.22"
  rust: "1.80"
  node: "20"

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
  linux:
    holons: []

mesh:
  join: paris.example.com
```

### Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Image name (for display and composition) |
| `include` | list of string | no | Other images to compose (merge) |
| `toolchains` | map | no | Toolchain name → version string |
| `holons` | list of string | yes | Holon slugs to install in `$OPBIN` |
| `platform` | map | no | Per-OS overrides |
| `platform.<os>.holons` | list of string | no | Platform-specific holons |
| `mesh.join` | string | no | Host address to join mesh |

### Supported `toolchains` keys

| Key | Source | Install method |
|---|---|---|
| `go` | golang.org/dl | Download official archive |
| `rust` | rustup.rs | `rustup toolchain install` |
| `node` | nodejs.org | Download or via `nvm` |
| `python` | python.org | Download or via package manager |

### Supported `platform` keys

| Key | OS |
|---|---|
| `darwin` | macOS |
| `windows` | Windows |
| `linux` | Linux (any distro) |

---

## Multi-Image Composition

```yaml
name: builder
include: [dev.yaml]
holons:
  - wisupaa-whisper
```

Merge rules:
- `toolchains`: union of all images, last declaration wins on version
- `holons`: union of all images
- `platform`: merge per-OS lists
- `mesh.join`: last wins

---

## Examples

### Developer workstation

```yaml
name: developer
toolchains:
  go: "1.22"
holons:
  - rob-go
  - phill-files
  - line-git
platform:
  darwin:
    holons: [al-brew]
```

### CI builder

```yaml
name: ci-builder
include: [developer.yaml]
toolchains:
  go: "1.22"
  rust: "1.80"
holons:
  - wisupaa-whisper
  - megg-ffmpeg
```

### Minimal mesh node

```yaml
name: mesh-node
holons:
  - phill-files
mesh:
  join: paris.example.com
```

---

## Cross-Reference

| File | Where | Who writes it | What it answers |
|---|---|---|---|
| `holon.yaml` | Each holon repo | Holon author | *"What does this holon need?"* |
| `setup.yaml` | Project / `~/.op/` | Operator | *"What should this host have?"* |
| `mesh.yaml` | `~/.op/mesh/` | `op mesh` (auto) | *"Who are the other hosts?"* |

---

## Rules

- `setup.yaml` is **operator-written** — it reflects choices about
  what this class of host needs
- `op setup` is additive. It does not remove previously installed
  items. Use `op uninstall` for removal.
- Running `op setup` multiple times is idempotent and safe
- Toolchain versions are pinned. Floating versions are not supported.
