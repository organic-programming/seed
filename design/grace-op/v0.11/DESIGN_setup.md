# `op setup` — Declarative Host Provisioning

## Overview

`op setup` provisions a host from a declarative image file. It installs toolchains, resolves holon dependencies, builds holons that require compilation, and configures the environment — everything needed to go from a bare machine to a functioning OP host.

**Key principle**: like Docker, `op setup` is **declarative and automatic**. You describe the desired state, it makes it happen. No confirmation prompts, no manual steps.

```bash
op setup dev.yaml       # apply a specific image
op setup                # apply ./setup.yaml or ~/.op/setup.yaml
```

## Scope

`op setup` targets the same small, intentional networks as `op mesh` — a team's machines, a few build servers, a developer's laptop. It is not a general-purpose configuration management tool (Ansible, Puppet).

---

## Image File

An image file declares **what should be present on this host**:

```yaml
# dev.yaml — developer workstation
name: developer

# Toolchains to install or verify
toolchains:
  go: "1.22"
  rust: "1.80"         # optional

# Holons to install in OPBIN
holons:
  - rob-go
  - phill-files
  - line-git
  - megg-ffmpeg

# Per-platform overrides
platform:
  darwin:
    holons: [al-brew]
  windows:
    holons: [marvin-winget]

# Optional: join a mesh
mesh:
  join: paris.example.com
```

---

## Dependency Resolution

This is where `op setup` goes deeper than a simple installer. Each holon already declares its dependencies in `holon.yaml`:

```yaml
# rob-go/holon.yaml
requires:
  commands: [go]       # → needs the Go toolchain

# megg-ffmpeg/holon.yaml
kind: wrapper
delegates:
  commands: [ffmpeg]   # → needs ffmpeg binary on PATH

# wisupaa-whisper/holon.yaml
requires:
  commands: [cmake, make]  # → needs build tools
```

`op setup` reads these declarations and **resolves the full dependency tree** before installing anything.

### Dependency Categories

| Category | Source | Example | Resolution |
|---|---|---|---|
| **Toolchain** | Image file `toolchains:` | `go: "1.22"` | Download official binary from golang.org |
| **System command** | `holon.yaml` `requires.commands` | `cmake`, `make` | Install via platform package manager |
| **Delegated command** | `holon.yaml` `delegates.commands` | `ffmpeg` | Install via platform package manager or compile from source |
| **Source build** | `holon.yaml` `build.runner` | `cmake` holon (wisupaa) | Clone repo + build locally |
| **Holon dependency** | Future `holon.yaml` `requires.holons` | `rob-go` needs nothing, but a compositor needs its members | Recursive `op setup` |

### Resolution Strategy per Platform

System dependencies are installed through the platform's native package manager:

| Platform | Package Manager | Agent Holon |
|---|---|---|
| macOS | Homebrew (`brew`) | Al Brew |
| Windows | WinGet (`winget`) | Marvin WinGet |
| Linux (Debian) | APT (`apt`) | TBD |
| Linux (Nix) | Nix (`nix-env`) | TBD |

This creates a **bootstrapping dependency**: `op setup` needs a package manager holon (Al, Marvin) to install system dependencies, but those holons might themselves be in the install list.

**Bootstrap resolution:**
1. `op setup` has a built-in, minimal package manager driver (hardcoded `brew install`, `apt install`, `winget install` calls)
2. This driver installs the bare essentials — including the package manager holon if needed
3. Once the package manager holon exists, all further system installs go through it

### The Dependency Graph

```
op setup dev.yaml
│
├── toolchains
│   └── go 1.22 ─────────── download from golang.org/dl
│
├── holons
│   ├── rob-go
│   │   └── requires: go ── (satisfied by toolchain above)
│   │
│   ├── phill-files
│   │   └── requires: go ── (satisfied)
│   │
│   ├── line-git
│   │   └── requires: go, git ── go (satisfied), git (install via brew/apt)
│   │
│   ├── megg-ffmpeg
│   │   └── delegates: ffmpeg ── install via brew/apt OR compile from source
│   │
│   └── al-brew (darwin only)
│       └── requires: brew ── verify present or install Homebrew
│
└── mesh
    └── join paris.example.com ── op mesh add --deploy
```

### Source Compilation

Some holons require compiling a dependency from source instead of installing a prebuilt binary. This is declared in an extended `requires` section:

```yaml
# wisupaa-whisper/holon.yaml
requires:
  commands: [cmake, make]
  sources:
    - name: whisper.cpp
      repo: https://github.com/ggerganov/whisper.cpp
      ref: v1.5.4                # git tag — pinned to a release
      build: cmake

# megg-ffmpeg/holon.yaml (if compiling from source)
requires:
  commands: [make, nasm]
  sources:
    - name: ffmpeg
      repo: https://github.com/FFmpeg/FFmpeg
      ref: a1b2c3d4e5f6           # commit SHA — pinned to exact revision
      build: configure-make
      configure_args: [--enable-gpl, --enable-libx264]
```

**Pinning rule**: `ref` accepts a git tag (`v1.5.4`), a full or abbreviated commit SHA (`a1b2c3d4e5f6`), or a branch name (`master`). For **reproducibility**, tags or commit SHAs are required — floating branches are rejected by `op check` with a warning.

`op setup` handles this:
1. Clone the repo to `~/.op/cache/sources/<name>`
2. Check out the specified ref
3. Build using the declared build system
4. Install the binary to `OPBIN` or system PATH

---

## Execution Flow

```bash
op setup dev.yaml
```

```
── op setup dev.yaml ───────────────────────────────────

Phase 1: Resolve
  Reading dev.yaml...
  Fetching holon manifests...
  Resolving dependency graph: 4 holons, 2 toolchains, 3 system packages

Phase 2: Toolchains
  ✅ go 1.22.1     (already installed)
  📦 rust 1.80.0   → ~/.rustup/toolchains/stable

Phase 3: System dependencies
  📦 git           (via brew)
  📦 cmake         (via brew)
  ✅ make          (already present)

Phase 4: Holons
  📦 rob-go        → go install → ~/.op/bin/rob-go
  📦 phill-files   → go install → ~/.op/bin/phill-files
  📦 line-git      → go install → ~/.op/bin/line-git
  📦 megg-ffmpeg   → brew install ffmpeg → wrapper → ~/.op/bin/megg-ffmpeg
  📦 al-brew       → go install → ~/.op/bin/al-brew

Phase 5: Environment
  ✅ OPPATH=~/.op
  ✅ OPBIN=~/.op/bin
  ✅ PATH includes OPBIN

Phase 6: Mesh (optional)
  🔐 Joining mesh at paris.example.com...
  ✅ Certificates deployed

── done (12s) ──────────────────────────────────────────
```

---

## Holon Installation Methods

`op setup` determines how to install each holon based on its `kind` and available information:

| Kind | Method | How |
|---|---|---|
| `native` (Go) | `go install` | `go install github.com/organic-programming/<holon>/cmd/<binary>@latest` |
| `native` (Go) | Prebuilt binary | Download from GitHub release (if available) |
| `native` (C/C++) | Source build | Clone + build using declared runner (`cmake`, `make`) |
| `native` (Rust) | `cargo install` | `cargo install <holon>` |
| `wrapper` | System install | Install delegated command via package manager, place wrapper in OPBIN |

**Priority**: prebuilt binary > `go install` / `cargo install` > source build. Fastest method that works.

---

## Idempotency

Running `op setup dev.yaml` multiple times is safe:
- Already-installed toolchains and holons are skipped (version check)
- Outdated versions are upgraded
- Removed items (not in the image anymore) are **not** uninstalled — `op setup` is additive, not subtractive
- To remove: explicit `op uninstall <holon>`

---

## Multi-Image Composition

Images can include other images for layering:

```yaml
# builder.yaml
name: builder
include: [dev.yaml]      # everything in dev, plus:

holons:
  - wisupaa-whisper       # adds a C++ holon with source deps
```

Resolution: merge toolchains, holons, and platform overrides. Conflicts: last wins.

---

## Relationship to Existing Commands

| Command | Role |
|---|---|
| `op setup` | Provision the host (install toolchains, holons, system deps) |
| `op build` | Compile a single holon from source (assumes deps are present) |
| `op check` | Verify a single holon's prerequisites (read-only) |
| `op mesh` | Configure distributed topology (certificates, registry) |
| `op install` | Install a single holon binary to OPBIN (post-build) |

`op setup` calls `op build` and `op install` internally for each holon that needs compilation.

---

## Open Questions

1. **`requires.sources` in `holon.yaml`** — is this the right place to declare source dependencies, or should it live in a separate file to keep `holon.yaml` minimal?
2. **Toolchain installation** — should `op setup` install Go by downloading from golang.org directly, or delegate to the platform package manager (`brew install go`)?
3. **Wrapper holons** — `megg-ffmpeg` wraps `ffmpeg`. Should `op setup` install `ffmpeg` (the system binary) AND the wrapper holon, or just the system binary (since the wrapper may not exist yet)?
4. **Image file location** — should images live in a central registry (like Docker Hub) so teams can share them, or always local files?
5. **Rollback** — if phase 4 fails halfway, should `op setup` roll back the completed installs, or leave them in place?
