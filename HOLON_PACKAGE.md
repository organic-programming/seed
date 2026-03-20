# PACKAGE — The Holon Package Format v1

Status: draft

Audience:
- `grace-op` implementers
- SDK implementers (Go, Swift, Rust, …)
- holon authors
- composite-app recipe authors

---

## Why This Spec Exists

The holon package is the center of the Organic Programming world.

Every `op` operation — build, install, discover, run, inspect, depend, compose
— works on the same unit: a `.holon` package. This spec defines that unit
once and for all.

The current system scatters holon concerns across disconnected formats:

| Before | Problem |
|--------|---------|
| Source manifest outside the proto/package boundary | No single source of truth for identity, contract, and runtime metadata |
| Build output as bare binary under `.op/build/bin/` | No metadata, no architecture support |
| Installed binary as bare file in `$OPBIN` | Opaque — cannot inspect, version, or uninstall cleanly |
| Dependency cache as raw source trees | No binary caching, no multi-arch, no integrity verification |

This spec replaces all of that with one universal container: `<slug>.holon/`.

---

## Core Position

**The proto is the single source of truth. The package is the universal unit.**

The `.proto` file carrying `option (holons.v1.manifest)` is what humans
author. Everything else is derived — including the user-facing guide.
The `.holon` package wraps source, binaries, documentation, and metadata
into one structure that `op` understands at every stage: development,
build, cache, install, runtime, and registry.

`op` reads proto files directly using `github.com/bufbuild/protocompile` —
a pure Go proto compiler. No `protoc` binary required. `.holon.json` is a
generated cache, never hand-edited.

---

## Relationship to Existing Documents

| Document | Role after this spec |
|----------|---------------------|
| `OP.md` | CLI spec. Discovery, lifecycle, `op mod`, environment remain authoritative. This spec defines the package format those commands operate on. |
| `HOLON_BUILD.md` | Build orchestration. Recipe runner, step types, CLI contract remain authoritative. This spec extends its artifact model. |
| `HOLON_PROTO.md` | Proto authoring spec. Canonical format. This spec defines how the package is produced from it. |

---

## Package Structure

A `.holon` package is a directory. It supports **source, distributable,
and binary distribution** — in any combination.

```
<slug>.holon/
  .holon.json              # generated JSON cache (always present)
  bin/                     # compiled binary (optional, platform-specific)
    <arch>/                # one dir per target arch — <os>_<cpu> (e.g. darwin_arm64)
      <slug>               # native executable for that arch
  dist/                    # built/bundled artifact (optional, platform-independent)
    <entrypoint>           # transpiled or interpreted entrypoint
    node_modules/          # bundled runtime dependencies
    requirements.txt       # or frozen deps manifest
    ...
  git/                     # development source (optional, for build-from-source)
    api/v1/holon.proto     # source tree — full repo content, no .git/
    src/app.ts             # raw source (TypeScript, Python, etc.)
    cmd/main.go
    go.mod
    ...
```

### Three Artifact Layers

| Layer | What it is | Produced by | Example |
|-------|-----------|-------------|--------|
| **`bin/<arch>/`** | Compiled native binary, platform-specific. No runtime needed. | `go build`, `cargo build`, `cmake`, `swiftc` | Go, Rust, C++, Swift |
| **`dist/`** | Built/bundled artifact, platform-independent. Runs with an interpreter. Includes transpiled code and/or bundled dependencies. | `tsc`, `pip install`, `npm install`, `bundle` | TypeScript→JS, Python, Node, Ruby |
| **`git/`** | Development source. What the developer writes. May need compilation, transpilation, or dependency resolution before it can run. | `git clone`, `op mod pull` | Any language |

The build step transforms **source → dist or binary**:

```
git/ (development source)
  │
  ├── go-module ─────→ bin/<arch>/<slug>     (compiled)
  ├── cargo ─────────→ bin/<arch>/<slug>     (compiled)
  ├── cmake ─────────→ bin/<arch>/<slug>     (compiled)
  ├── swift-package ─→ bin/<arch>/<slug>     (compiled)
  │
  ├── node ──────────→ dist/                 (JS + bundled node_modules)
  ├── python ────────→ dist/                 (Python + frozen deps)
  ├── ruby ──────────→ dist/                 (Ruby + bundled gems)
  └── typescript ────→ dist/                 (transpiled JS — source was TS)
```

### Contents

| Entry | Required | Description |
|-------|----------|-------------|
| `.holon.json` | always | Generated JSON cache for fast discovery. Derived from the proto, never hand‑edited. |
| `bin/<arch>/<slug>` | optional | Pre-built native binary. `<arch>` follows Go convention: `<os>_<cpu>` — e.g. `darwin_arm64`, `linux_amd64`, `windows_amd64`. |
| `dist/` | optional | Built/bundled distributable. Contains transpiled or interpreted entrypoint with bundled runtime dependencies. Platform-independent. No proto files — the running holon answers `Describe` via the SDK. |
| `git/` | optional | Development source tree. Read-only. No `.git/` directory — just the working tree content at a specific version. |

### Distribution Modes

A package may contain any combination of layers:

| Mode | `bin/` | `dist/` | `git/` | Use case |
|------|--------|---------|--------|----------|
| **Binary-only** | ✓ | — | — | Pre-built Go/Rust holon from registry. |
| **Dist-only** | — | ✓ | — | Bundled Python/Node holon with frozen deps. |
| **Source-only** | — | — | ✓ | Unbuilt source. `op build` compiles on demand. |
| **Binary + source** | ✓ | — | ✓ | Pre-built binary + source for rebuild. |
| **Dist + source** | — | ✓ | ✓ | Bundled dist + source for rebuild. |

A package must have at least one of `bin/`, `dist/`, or `git/`.

### Examples

```
# Compiled binary-only (Go, from registry)
gabriel-greeting-go.holon/
  .holon.json
  bin/
    darwin_arm64/gabriel-greeting-go
    linux_amd64/gabriel-greeting-go

# Interpreted dist-only (Python, bundled)
my-transcriber-py.holon/
  .holon.json
  dist/
    main.py
    requirements.txt
    venv/

# Transpiled dist-only (TypeScript → JS)
my-analyzer-ts.holon/
  .holon.json
  dist/
    index.js
    node_modules/

# Source-only (from op mod pull, build locally)
gabriel-greeting-go.holon/
  .holon.json
  git/
    api/v1/holon.proto
    cmd/main.go
    go.mod
    internal/

# Full compiled (binary + source for rebuild)
gabriel-greeting-go.holon/
  .holon.json
  bin/
    darwin_arm64/gabriel-greeting-go
  git/
    api/v1/holon.proto
    cmd/main.go
    go.mod

# Full interpreted (dist + source for rebuild)
my-analyzer-ts.holon/
  .holon.json
  dist/
    index.js
    node_modules/
  git/
    src/app.ts
    package.json
    tsconfig.json

# The op CLI itself
grace-op.holon/
  .holon.json
  bin/
    darwin_arm64/op              # exception: binary name is "op", not "grace-op"
```

---

## Proto Compilation: `protocompile`

`op` embeds `github.com/bufbuild/protocompile` — a pure Go protobuf
compiler. `op` can parse any `.proto` file, resolve imports, and extract the
`HolonManifest` extension from `FileOptions`.

**No `protoc` binary required.** The developer only needs `protoc` (or `buf`)
for language-specific stub generation (Go, Swift, Dart). `op` handles all
manifest-related proto work internally.

### What `op` does with `protocompile`

| Operation | Input | Output |
|-----------|-------|--------|
| `op build` | source proto + `_protos/` | `.holon.json` + binary or dist in `.holon` package |
| `op check` | source proto + `_protos/` | validation report |
| `op discover` (source) | source proto + `_protos/` | identity fields for listing |
| `op discover` (package) | `.holon.json` | identity fields (fast path, no proto parsing) |
| `op inspect` (source) | source proto + `_protos/` | full API documentation |
| `op inspect` (live) | `Describe` RPC | full API documentation (holon must be running) |

> Detailed semantics of `op list`, `op discover`, and `op inspect` are
> defined in `OP.md`.

---

## `.holon.json` — Generated Accelerator Cache

`.holon.json` is a **generated accelerator cache** derived from the proto.
It is **never hand-edited**. If the proto changes, `op build` regenerates
it.

It exists because:

- `op discover` scans many packages — parsing proto files for each is
  slower than reading JSON.
- Shell completion needs slug/uuid lookups in microseconds.
- `op uninstall` needs identity verification without proto infrastructure.
- Collision detection needs slug + uuid from many packages.

`.holon.json` is **not a hard requirement**. When `op` finds a `.holon`
package directory without `.holon.json`, it falls back to launching the
packaged binary via stdio and calling `HolonMeta.Describe` to obtain
metadata. If successful, `op` writes `.holon.json` as a cache so
subsequent scans use the fast path. If the binary is absent or the
`Describe` call fails, the entry is silently skipped.

For deeper queries (services, RPCs, messages, skills, sequences), `op`
uses `protocompile` on source protos or calls `Describe` RPC on a running
holon.

### Schema

```json
{
  "schema": "holon-package/v1",
  "slug": "gabriel-greeting-go",
  "uuid": "3f08b5c3-8931-46d0-847a-a64d8b9ba57e",
  "identity": {
    "given_name": "Gabriel",
    "family_name": "Greeting-Go",
    "motto": "Greets users in 56 languages — a Go daemon recipe example."
  },
  "lang": "go",
  "runner": "go-module",
  "status": "draft",
  "kind": "native",
  "transport": "stdio",
  "entrypoint": "gabriel-greeting-go",
  "architectures": ["darwin_arm64", "linux_amd64"],
  "has_dist": false,
  "has_source": true
}
```

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `schema` | string | Always `"holon-package/v1"`. |
| `slug` | string | Canonical holon slug. |
| `uuid` | string | UUID v4. Never changes. |
| `identity.*` | object | Given name, family name, motto. |
| `lang` | string | Primary implementation language. |
| `runner` | string | Build runner (`go-module`, `python`, `node`, `cargo`, etc.). Determines how `op` launches the holon. |
| `status` | string | Lifecycle stage (`draft`, `stable`, `deprecated`, `dead`). |
| `kind` | string | `native`, `wrapper`, or `composite`. |
| `transport` | string | Default transport hint (`stdio`, `tcp`, or empty). |
| `entrypoint` | string | Entrypoint name. For compiled: binary in `bin/<arch>/`. For interpreted: script in `dist/`. |
| `architectures` | list | Available pre-built architectures in `bin/` (empty for interpreted or source-only). |
| `has_dist` | bool | Whether `dist/` is present. |
| `has_source` | bool | Whether `git/` is present. |

Rules:

- `.holon.json` is **derived from the proto** — a projection, not a source.
- Readers must ignore unknown fields (forward compatibility).
- `slug` and `uuid` are the main resolution keys.
- For full manifest data (build config, skills, sequences, contract), start the holon and call `Describe` RPC, or use `protocompile` on source.

---

## Truth Boundaries

**The proto is the source of truth at every stage.** Other formats
(`.holon.json`, `Describe` RPC) are derived caches.

```
SOURCE → BUILD → CACHE → INSTALL → RUNTIME
```

### 1. Source Truth (devtime — human edits)

The canonical format is a `.proto` file carrying
`option (holons.v1.manifest)`.

```
api/v1/holon.proto         # holon-local manifest (humans edit this)
../../_protos/             # platform protos (holons/v1/manifest.proto)
../_protos/                # domain protos (shared contract)
```

`op` reads `.proto` files directly via `protocompile`. It resolves imports
via the `_protos/` include paths.

The proto is the only source format.

### 2. Build Truth (build output → package)

`op build` reads the source proto, runs the build step, and produces a
`.holon` package:

1. Runner compiles/builds (e.g. `go build`, `tsc`, `pip install`).
2. `op` uses `protocompile` to parse the source proto → generates `.holon.json`.
3. `op` places result under `bin/<arch>/` (compiled) or `dist/` (interpreted).
4. `op` assembles the `.holon` package under `.op/build/`.

Build output (compiled):

```
.op/build/<slug>.holon/
  .holon.json
  bin/
    <current_arch>/<slug>
```

Build output (interpreted):

```
.op/build/<slug>.holon/
  .holon.json
  dist/
    <entrypoint>
    <bundled deps>
```

### 3. Cache Truth (fetched dependency)

`op mod pull` fetches dependencies into `$OPPATH/cache/`. Cached packages
are `.holon` packages — the same format.

```
$OPPATH/cache/<module>@<version>.holon/
  .holon.json
  bin/                     # if binary distribution is available
    darwin_arm64/<slug>
  git/                     # if source distribution
    api/v1/holon.proto
    ...
```

### 4. Install Truth (installed → ready to run)

`op install` copies the package into `$OPBIN/`. For compiled holons, only
the current architecture binary is needed. For interpreted holons, `dist/`
is copied as-is.

```
# Compiled
$OPBIN/<slug>.holon/
  .holon.json
  bin/
    <current_arch>/<slug>

# Interpreted
$OPBIN/<slug>.holon/
  .holon.json
  dist/
    <entrypoint>
    <bundled deps>
```

### 5. Runtime Truth (running process)

`op inspect`, `op tools`, and `op mcp` obtain rich metadata from
`holons.v1.HolonMeta/Describe`. The SDK embeds the manifest into the
binary at build time.

| Consumer | Source | Purpose |
|----------|--------|---------|
| `op discover` | `.holon.json` | Fast identity lookup |
| `op inspect` (source) | source `.proto` via `protocompile` | Static contract introspection |
| `op inspect` (live) | `Describe` RPC | Live state + dynamic contract |
| `op build` (source) | source `.proto` | Build-time manifest extraction |

All three are projections of the same truth: the `.proto` file.

---

## Bundle Integration (`.app`)

A composite holon can produce a macOS `.app` (or iOS `.framework`, etc.)
that embeds child holon packages. The `.app` is the delivery vehicle;
the `.holon` packages inside are the functional units.

### Bundle Layout

```
MyApp.app/
  Contents/
    MacOS/
      MyApp                                      # host binary (Flutter, SwiftUI, etc.)
    Resources/
      Holons/                                    # embedded holon packages
        gabriel-greeting-go.holon/
          .holon.json
          bin/darwin_arm64/gabriel-greeting-go
        gabriel-greeting-c.holon/
          .holon.json
          bin/darwin_arm64/gabriel-greeting-c
    Info.plist
```

Rules:

- Embedded holons go in `Contents/Resources/Holons/`.
- Each embedded holon is a complete `.holon` package — same format as
  installed or cached packages.
- Only the target architecture binary is embedded (no multi-arch inside
  a signed bundle).
- The host binary and embedded holons are part of the same code-signed
  bundle — no external sidecars needed.

### Discovery Inside a Bundle

When running inside a `.app` bundle, the host app (via the SDK) discovers
embedded holons by scanning `Contents/Resources/Holons/*.holon/`. The SDK
reads `.holon.json` for fast identity resolution, same as `op` does
externally.

The SDK's `connect(slug)` function resolves in this order:

```
1. Bundle-local holons   (Contents/Resources/Holons/<slug>.holon/)
2. Installed packages    ($OPBIN/<slug>.holon/)
3. System PATH           (fallback)
```

Bundle-local takes priority — the app is self-contained by default.

### Execution Inside a Bundle

The host app launches embedded holons the same way `op run` does:

```
Contents/Resources/Holons/<slug>.holon/bin/<arch>/<slug> serve --listen <uri>
```

The SDK manages lifecycle: start on `connect(slug)`, readiness probe,
gRPC calls, and cleanup on app exit.

Transport inside a bundle is typically `stdio://` (direct pipe, no
network) or `tcp://localhost:<port>` for holons that need concurrent
clients.

### Dependencies Inside a Bundle

If an embedded holon has its own holon dependencies, they must be
bundled too. The composite recipe is responsible for pulling the full
transitive dependency tree:

```yaml
steps:
  - build_member: daemon                         # builds gabriel-greeting-go
  - build_member: greeting-c                     # builds gabriel-greeting-c
  - copy_artifact:
      from: daemon
      to: MyApp.app/Contents/Resources/Holons/gabriel-greeting-go.holon
  - copy_artifact:
      from: greeting-c
      to: MyApp.app/Contents/Resources/Holons/gabriel-greeting-c.holon
  - use_cached:
      ref: github.com/organic-programming/some-dep
      version: v0.3.0
      as: dep
  - copy_artifact:
      from: dep
      to: MyApp.app/Contents/Resources/Holons/some-dep.holon
```

`op build` for a composite holon resolves the full dependency graph from
`holon.mod`, builds or fetches each dependency as a `.holon` package,
and embeds them all under `Contents/Resources/Holons/`.

### Code Signing

- Embedded `.holon` packages contain executables — they are included in
  the bundle's code signature.
- `.holon.json` is a data file — it does not affect signing.
- The recipe places holons before the signing step. Any post-sign
  modification to `bin/<arch>/<slug>` invalidates the signature.

---

## OPPATH / OPBIN Layout

```
~/.op/                                          # OPPATH
  bin/                                          # OPBIN
    grace-op.holon/                             # installed packages
      .holon.json
      bin/darwin_arm64/op
    gabriel-greeting-go.holon/
      .holon.json
      bin/darwin_arm64/gabriel-greeting-go
    my-transcriber-py.holon/                    # interpreted holon
      .holon.json
      dist/
        main.py
        requirements.txt
        venv/
    rob-go.holon/
      .holon.json
      bin/darwin_arm64/rob-go
    OldLegacyTool                               # legacy bare binary (fallback)
  cache/                                        # fetched dependencies
    github.com/organic-programming/
      gabriel-greeting-c@v0.1.0.holon/            # source-only cached package
        .holon.json
        git/
          api/v1/holon.proto
          ...
      gabriel-greeting-go@v1.0.0.holon/         # full cached package
        .holon.json
        bin/darwin_arm64/gabriel-greeting-go
        git/
          api/v1/holon.proto
          ...
```

### PATH Resolution

When running a holon by slug, `op` reads `.holon.json` to determine the
runner, then dispatches:

- **Compiled** (`go-module`, `cargo`, etc.): `$OPBIN/<slug>.holon/bin/<arch>/<slug> serve --listen <uri>`
- **Interpreted** (`python`, `node`, etc.): `<interpreter> $OPBIN/<slug>.holon/dist/<entrypoint> serve --listen <uri>`

Direct shell invocation without `op` is not the primary use case — `op <slug>` is.

The one exception remains `op` itself: `grace-op.holon/bin/<arch>/op` is
symlinked as `$OPBIN/op` so that `op` is directly callable.

---

## Dependency Management (`op mod`)

Dependency management is built directly into `op`, following the Go modules
pattern. The `.holon` package is the unit `op mod` fetches, caches, and
resolves.

### Files

| File | Role | Analogy |
|------|------|---------|
| `holon.mod` | Declares dependencies with minimum versions | `go.mod` |
| `holon.sum` | Integrity hashes for fetched packages | `go.sum` |

### Resolution

`op` uses **Minimum Version Selection (MVS)** — the same deterministic
algorithm as Go modules. No solver, no SAT.

### Commands

| Command | Description |
|---------|-------------|
| `op mod init` | Create `holon.mod` |
| `op mod add <module> [version]` | Add a dependency |
| `op mod remove <module>` | Remove a dependency |
| `op mod tidy` | Clean `holon.mod`, regenerate `holon.sum` |
| `op mod pull` | Fetch all deps into `$OPPATH/cache/` as `.holon` packages |
| `op mod update [<module>]` | Update one or all deps to latest allowed version |
| `op mod list` | List declared dependencies |
| `op mod graph` | Print dependency graph |

### Fetch Strategy

When `op mod pull` fetches a dependency:

1. **Check cache** — if `$OPPATH/cache/<module>@<version>.holon/` exists
   and `holon.sum` matches, done.
2. **Fetch** — download the `.holon` package from the registry or git.
   - If the registry provides a binary-only or dist-only package → cache it as-is.
   - If fetching from git → clone into `git/`, generate
     `.holon.json` from the proto.
3. **Verify** — check integrity against `holon.sum`.
4. **Build on demand** — when `op` needs to run the holon and neither
   `bin/<arch>/` nor `dist/` is present, build from `git/` and cache the result.

### `holon.mod` Format

```
module github.com/organic-programming/my-holon

require (
    github.com/organic-programming/gabriel-greeting-c v0.1.0
    github.com/organic-programming/rob-go v0.2.0
)
```

---

## Lifecycle Integration

### `op build`

Reads source truth, produces a `.holon` package:

```
.op/build/<slug>.holon/
  .holon.json
  bin/<current_arch>/<slug>
```

Reports the `.holon` directory as the primary artifact.

For bundle holons:

```
.op/build/<App>.app
.op/build/<App>.app.holon.json
```

### `op install`

Copies the `.holon` package into `$OPBIN/`. If the package was source-only
and no binary exists for the current arch, `op` builds from `git/` first.

### `op uninstall`

Removes the entire `.holon` directory from `$OPBIN/`.

### `op discover`

Discovery order:

```
1. Source holons in known roots   (holon.proto)
2. Built packages                (.op/build/*.holon/)
3. Installed packages            ($OPBIN/*.holon/)
4. Cached packages               ($OPPATH/cache/*.holon/)
5. Legacy bare binaries           ($OPBIN, fallback)
6. $PATH                          (system-wide fallback)
```

For packages (layers 2–4), `op` reads `.holon.json` for fast identity
resolution. If `.holon.json` is missing, `op` probes the binary via
stdio `Describe` and caches the result as `.holon.json` for subsequent
scans. For deeper queries, `op` launches the holon and calls `Describe`.

### `op run`

Resolves the package, reads `.holon.json` for runner and entrypoint, launches:

- **Compiled**: `<package>/bin/<arch>/<slug> serve --listen <uri>`
- **Interpreted**: `<interpreter> <package>/dist/<entrypoint> serve --listen <uri>`

### `op inspect`, `op tools`, `op mcp`

For source holons:
1. Parse source proto via `protocompile`.

For packages (built/installed/cached):
1. Resolve via `.holon.json`.
2. Launch and call `HolonMeta.Describe` for full contract and live state.

No pre-compiled proto snapshot needed — the running holon carries its own
contract via the `Describe` RPC.

### `op man`

Display a holon's user guide:

```bash
op man gabriel-greeting-go
op man rob-go
op man gabriel-greeting-go --export-path ./docs/
op man gabriel-greeting-go --export-path ./gabriel-guide.md
```

The guide is authored as **markdown** inside `(holons.v1.manifest)` in the
proto. `op man` extracts it from source proto or via `Describe` RPC and renders
it for the terminal.

`--export-path` writes the guide as a `.md` file — either to a directory
(uses `<slug>.md` as filename) or to an explicit file path. This allows
generating standalone documentation from any package without source trees.

Every holon carries its own documentation. No external README, no wiki,
no separate doc site required. The proto is the guide.

---

## Composite Embedding

When a composite holon builds a `.app` or other bundle, it embeds child
holon packages — not bare binaries. The recipe steps handle the wiring.

### Recipe Step: `copy_artifact`

Copies a freshly built member's `.holon` package into the bundle:

```yaml
steps:
  - build_member: daemon
  - copy_artifact:
      from: daemon
      to: MyApp.app/Contents/Resources/Holons/gabriel-greeting-go.holon
```

### Recipe Step: `use_installed`

References an already-installed package from `$OPBIN`:

```yaml
steps:
  - use_installed:
      ref: gabriel-greeting-go
      as: greetingd
  - copy_artifact:
      from: greetingd
      to: MyApp.app/Contents/Resources/Holons/gabriel-greeting-go.holon
```

### Recipe Step: `use_cached`

References a cached dependency from `$OPPATH/cache/`:

```yaml
steps:
  - use_cached:
      ref: github.com/organic-programming/gabriel-greeting-go
      version: v1.0.0
      as: greetingd
  - copy_artifact:
      from: greetingd
      to: MyApp.app/Contents/Resources/Holons/gabriel-greeting-go.holon
```

### Architecture Selection

Embedded packages carry only the target arch binary. The recipe's
`--target` flag determines which `bin/<arch>/` to select:

```bash
op build --target macos     # embeds bin/darwin_arm64/
op build --target linux     # embeds bin/linux_amd64/
```

### Standalone vs Shared

| Pattern | Embedded | Use case |
|---------|----------|----------|
| `copy_artifact` | yes | Portable standalone app. All dependencies inside the bundle. |
| `use_installed` | no | Dev setup. Host app connects to installed holons via `op`. |
| `use_cached` | yes | CI/CD. Pull from cache, embed into bundle. |

---

## Source vs Dist vs Binary Distribution

The three layers serve different purposes:

### Source Distribution (`git/`)

The development source tree — what the developer works with:

```
gabriel-greeting-go/           # developer's working directory
  api/v1/holon.proto           # source truth (humans edit this)
  api/public.go                # Code API
  api/cli.go                   # CLI facet
  cmd/main.go                  # entry point
  internal/server.go           # RPC server
  internal/greetings.go        # domain data
  gen/                         # generated proto code
  go.mod
  go.sum
  holon.mod                    # holon dependencies
  holon.sum                    # integrity hashes
```

Source distribution is handled by Git. The proto manifest travels with the
source. When fetched as a dependency, the source tree goes into
`<slug>.holon/git/`.

### Binary Distribution (`bin/<arch>/`)

Compiled native executables — no runtime needed:

```
gabriel-greeting-go.holon/
  .holon.json
  bin/
    darwin_arm64/gabriel-greeting-go
    linux_amd64/gabriel-greeting-go
```

Binary distribution is platform-specific. Handled by `op install`,
`op publish` (future), and the holon registry (future).

### Dist Distribution (`dist/`)

Built/bundled interpreted or transpiled artifacts — platform-independent,
but requires an interpreter:

```
my-analyzer-ts.holon/
  .holon.json
  dist/
    index.js                   # transpiled from TypeScript
    node_modules/              # bundled runtime deps
```

Dist distribution is the output of a build step that doesn't produce a
native binary: `tsc` (TypeScript→JS), `pip install` (Python deps),
`npm install` (Node deps), `bundle install` (Ruby gems).

The source (TypeScript `.ts` files) cannot run. The dist (transpiled `.js`
+ `node_modules/`) can run with `node`. They are different things.

### The Package Unifies All Three

The `.holon` package is the **same format** everywhere — the only
difference is which optional subdirectories (`bin/`, `dist/`, `git/`) are
populated:

| Context | `bin/` | `dist/` | `git/` | Created by |
|---------|--------|---------|--------|------------|
| Build output (compiled) | current arch | — | — | `op build` |
| Build output (interpreted) | — | ✓ | — | `op build` |
| Installed (compiled) | current arch | — | — | `op install` |
| Installed (interpreted) | — | ✓ | — | `op install` |
| Cache (source fetch) | — | — | ✓ | `op mod pull` (git) |
| Cache (binary fetch) | multi-arch | — | — | `op mod pull` (registry) |
| Cache (dist fetch) | — | ✓ | — | `op mod pull` (registry) |
| Cache (after local build) | current arch or ✓ | ✔ or — | ✓ | `op build` on cached source |
| Registry package | multi-arch | optional | optional | `op publish` |

### Runner → Launch Command

`op` reads `runner` from `.holon.json` to determine how to launch:

| Runner | Looks for | Launch command |
|--------|-----------|----------------|
| `go-module` | `bin/<arch>/<slug>` | `<slug> serve --listen <uri>` |
| `cargo` | `bin/<arch>/<slug>` | `<slug> serve --listen <uri>` |
| `cmake` | `bin/<arch>/<slug>` | `<slug> serve --listen <uri>` |
| `swift-package` | `bin/<arch>/<slug>` | `<slug> serve --listen <uri>` |
| `python` | `dist/<entrypoint>` | `python3 dist/<entrypoint> serve --listen <uri>` |
| `node` | `dist/<entrypoint>` | `node dist/<entrypoint> serve --listen <uri>` |
| `ruby` | `dist/<entrypoint>` | `ruby dist/<entrypoint> serve --listen <uri>` |

---

## SDK Contract

For the `.holon` package to be fully self-describing at runtime, the SDK
must embed metadata into the process.

### Build-Time Embedding

`op build` generates a `DescribeResponse` snapshot from the source proto.
For compiled holons, this is embedded into the binary at link time.
For interpreted holons, the SDK parses the co-located source proto via
its own `protocompile` equivalent, or receives the snapshot from `op build`
as a generated source file.

### Runtime Behavior

- `serve` auto-registers `HolonMeta.Describe` from the embedded snapshot.
- Source-tree parsing is a development fallback only.
- A built holon must answer `Describe` with no source files nearby.

This applies to all SDKs: Go, Swift, Rust, Python, Node, and future
languages.




## Glossary

| Term | Definition |
|------|-----------|
| **Package** | A `.holon` directory. The universal unit of holon distribution. Contains `bin/`, `dist/`, `git/`, or any combination. |
| **Entrypoint** | The compiled binary inside `bin/<arch>/` or the script inside `dist/`. Named after the slug. |
| **Cache** | `.holon.json` — generated JSON for fast discovery. Never hand-edited. |
| **Source truth** | The `.proto` file carrying `(holons.v1.manifest)`. The only format humans edit. |
| **`protocompile`** | `github.com/bufbuild/protocompile` — pure Go proto compiler embedded in `op`. |
| **Architecture** | Target identifier following Go convention: `<os>_<cpu>` — e.g. `darwin_arm64`, `linux_amd64`, `windows_amd64`. |
| **Bundle** | Platform-native deliverable (`.app`, `.framework`) with `.holon.json` for `op` discovery. |
| **Slug** | `<given_name>-<family_name>`, lowercased, hyphenated. The universal holon identifier. |
