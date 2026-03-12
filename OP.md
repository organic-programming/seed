# `op` — The Organic Programming CLI

> One command, every holon.

`op` is the unified entry point to the Organic Programming ecosystem.
It discovers holons, builds them, manages their identities and
dependencies, and dispatches commands to them through a single
interface. The actant installs one binary and gets access to every
holon.

---

## R&D Context

This project is under active research and development. There is no
external documentation, wiki, or design authority other than the
composer (B. ALTER).

**Rules for any implementer (human or agent):**

1. **The composer is the only source of truth.** If something is
   ambiguous, unclear, or contradicts another document — ask the
   composer and wait for a response. Do not guess, infer, or invent.
2. **Do not make design decisions autonomously.** Implementation
   details that go beyond what is written here require explicit
   validation before proceeding.
3. **Prefer asking a dumb question over shipping a wrong assumption.**
4. **When in doubt, stop and ask.**

---

## Table of Contents

1. [Installation](#1-installation)
2. [Philosophy](#2-philosophy)
3. [Environment](#3-environment)
4. [Holon Manifest (`holon.yaml`)](#4-holon-manifest)
5. [Identity Management](#5-identity-management)
6. [Discovery](#6-discovery)
7. [Lifecycle](#7-lifecycle)
8. [Runners](#8-runners)
9. [Transport & Dispatch](#9-transport--dispatch)
10. [Serve & Dial](#10-serve--dial)
11. [Dependency Management](#11-dependency-management)
12. [Output & Formatting](#12-output--formatting)
13. [Error Model](#13-error-model)
14. [Introspection](#14-introspection)
15. [Complete Command Reference](#15-complete-command-reference)

---

## 1. Installation

### Via Go (recommended)

> **Note:** `go install ...@latest` requires at least one published
> Git tag (e.g. `v0.1.0`) on the `grace-op` repository. Until then,
> use the "From source" method below.

```bash
export OPPATH="${OPPATH:-$HOME/.op}"
export OPBIN="${OPBIN:-$OPPATH/bin}"
mkdir -p "$OPBIN"
GOBIN="$OPBIN" go install github.com/organic-programming/grace-op/cmd/op@latest
export PATH="$OPBIN:$PATH"
```

### From source

```bash
cd organic-programming/holons/grace-op
go build -o op ./cmd/op
mv op "$OPBIN/"
```

### Bootstrap from zero

```bash
go install github.com/organic-programming/grace-op/cmd/op@latest
op env --init
eval "$(op env --shell)"
```

After bootstrap, `op` is in `~/.op/bin/` and all OP commands are
available.

### Shell completion

`op` supports tab-completion for zsh and bash. Completions use the
existing discovery mechanism: identity-derived slugs, OPBIN entries,
and PATH binaries are all suggested.

**zsh** (add to `~/.zshrc`):

```bash
eval "$(op completion zsh)"
```

**bash** (add to `~/.bashrc`):

```bash
eval "$(op completion bash)"
```

After sourcing, tab-completion works for all holon-accepting commands:

```
op run gudule-<TAB>       →  gudule-greeting-godart, gudule-greeting-goswift, ...
op build grace<TAB>       →  grace-op
op install rob<TAB>       →  rob-go
op uninstall <TAB>        →  only lists installed holons from $OPBIN
```

Subcommands (`build`, `run`, `install`, `check`, `test`, `clean`,
`inspect`, `show`, `uninstall`) are also completed at the verb level.

---

## 2. Philosophy

### One command, every holon

`op` hides the complexity of holon discovery, transport selection,
and binary location. The actant (human or agent) only needs to know
the holon's name and the command to invoke. `op` resolves the rest.

### Orchestrator, not compiler

`op` does not compile source code. It reads `holon.yaml`, selects
the declared runner, executes the minimum required sequence, and
reports the result. Language tools remain the actual builders.

### Go is the scripting language

No shell scripts, no Makefiles for core orchestration. All build
logic is expressed in Go and driven by structured manifests.

### Integrated toolchain

`op` integrates identity management and dependency management
directly. There are no separate binaries for these concerns —
everything is built into the `op` binary, just like `go` integrates
`go mod`, `go env`, and `go install`.

| Concern | Commands |
|---|---|
| Identity | `op new`, `op list`, `op show` |
| Dependencies | `op mod init`, `op mod add`, `op mod pull`, etc. |
| Lifecycle | `op check`, `op build`, `op test`, `op clean`, `op install` |
| Discovery | `op discover` |
| Runtime | `op run`, `op serve` |

### Cross-platform by default

`op` itself is a pure Go binary — it compiles and runs on any
platform Go supports (macOS, Linux, Windows, etc.). Platform
constraints come from two places, not from `op`:

- **Runners**: some runners depend on platform-specific toolchains
  (e.g. `swift-package` requires Xcode, `cmake` requires CMake).
- **Holons**: each holon declares its supported platforms in
  `holon.yaml` via the `platforms` field. `op check` verifies that
  the current OS is in that list before building.

---

## 3. Environment

### Environment variables

| Variable | Default | Purpose |
|---|---|---|
| `OPPATH` | `~/.op` | User-local runtime home for all OP data |
| `OPBIN` | `$OPPATH/bin` | Canonical install directory for holon binaries |

Resolution: explicit env var if set, otherwise the default.

### `op env`

Print the resolved environment:

```
$ op env
OPPATH=/Users/alice/.op
OPBIN=/Users/alice/.op/bin
ROOT=/Users/alice/organic-programming
```

### `op env --init`

Create the runtime directory structure:

```
$ op env --init
created /Users/alice/.op/
created /Users/alice/.op/bin/
created /Users/alice/.op/cache/
```

### `op env --shell`

Output the shell configuration snippet (paste into `.zshrc` or
`.bashrc`):

```bash
$ op env --shell
export OPPATH="${OPPATH:-$HOME/.op}"
export OPBIN="${OPBIN:-$OPPATH/bin}"
mkdir -p "$OPBIN"
export PATH="$OPBIN:$PATH"
```

---

## 4. Holon YAML Manifest (`holon.yaml`)

Every holon carries a single `holon.yaml` at its root. This file
answers three questions:

1. **Who is this holon?** — identity, lineage, motto
2. **What does it do?** — description, contract
3. **How does `op` operate it?** — kind, runner, requires, artifacts

### Full schema

```yaml
# ── Identity ──────────────────────────────────────────
schema: holon/v0
uuid: "c7f3a1b2-8d4e-4f5a-b6c7-d8e9f0a1b2c3"
given_name: Rob
family_name: Go
motto: "Build what you mean."
composer: "B. ALTER"
clade: deterministic/io_bound
status: draft
born: "2026-02-20"

# ── Lineage ───────────────────────────────────────────
parents: []
reproduction: manual
generated_by: op

# ── Description ───────────────────────────────────────
description: |
  Rob Go wraps the go command, exposing build, test, run,
  fmt, vet, mod, and env as gRPC RPCs.

# ── Contract ──────────────────────────────────────────
contract:
  proto: protos/rob_go/v1/rob_go.proto
  service: RobGoService
  rpcs: [Build, Test, Run, Fmt, Vet]

# ── Operational ────────────────────────────────────────────
kind: wrapper
platforms: [macos, linux, windows]
build:
  runner: go-module
  main: ./cmd/rob
  configs:
    standard: {}
    with-cgo:
      description: "Enable CGo for native bindings"
  default_config: standard
requires:
  commands: [go]
  files: [go.mod]
delegates:
  commands: [go]
artifacts:
  binary: rob-go

# ── Skills ───────────────────────────────────────────────
skills:
  - name: prepare-release
    description: Prepare a Go package for production release.
    when: User wants clean, tested, optimized code ready to ship.
    steps:
      - Fmt — format all source files
      - Vet — run static analysis
      - Test — run the full test suite
      - Build with mode=release
```

### Identity fields

| Field | Type | Required | Description |
|---|---|---|---|
| `schema` | string | yes | Always `holon/v0`. |
| `uuid` | UUID v4 | yes | Generated once at birth. Never changes. |
| `given_name` | string | yes | The character — what distinguishes this holon. |
| `family_name` | string | yes | The function — what the holon does. |
| `motto` | string | yes | The *dessein* in one sentence. |
| `composer` | string | yes | Who designed the holon. |
| `clade` | enum | yes | Computational nature (see below). |
| `status` | enum | yes | Lifecycle stage: `draft`, `stable`, `deprecated`, `dead`. |
| `born` | date | yes | ISO 8601 date of creation. |

### Lineage fields

| Field | Type | Required | Description |
|---|---|---|---|
| `parents` | list of UUID | yes | Holons from which this one descends. Empty for primordial holons. |
| `reproduction` | enum | yes | `manual`, `assisted`, `automatic`, `autopoietic`, `bred`. |
| `generated_by` | string | yes | What created this file: `op`, `manual`, `codex`, etc. |

### Contract fields

| Field | Type | Required | Description |
|---|---|---|---|
| `contract.proto` | string | no | Path to `.proto` file, relative to holon root. |
| `contract.service` | string | no | gRPC service name. |
| `contract.rpcs` | list | no | RPC method names. |

Omit `contract` entirely if the proto is not yet defined.

### Operational fields

| Field | Type | Required | Description |
|---|---|---|---|
| `kind` | enum | yes | `native`, `wrapper`, or `composite`. |
| `platforms` | list | no | Supported OSes. Omit if cross-platform. |
| `build.runner` | string | yes | Selects the runner (see [Runners](#8-runners)). |
| `build.main` | string | no | Go package path (go-module only). |
| `build.configs` | map | no | Named build configurations (see [Build configs](#build-configs)). |
| `build.default_config` | string | no | Default config name. Required when `build.configs` is set. |
| `requires.commands` | list | yes | CLI tools that must exist on `PATH`. |
| `requires.files` | list | yes | Files that must exist relative to `holon.yaml`. |
| `delegates.commands` | list | no | Wrapper-only. External commands the holon wraps. |
| `artifacts.binary` | string | yes for `native`/`wrapper`, no for `composite` | Primary binary name (not a path). Must equal the slug for `native`/`wrapper`. Build output is `.op/build/bin/<artifacts.binary>`, install destination is `$OPBIN/<artifacts.binary>`. |
| `artifacts.primary` | string | no for `native`/`wrapper`, yes for `composite` | Non-binary primary artifact path (e.g. `.app` bundle), relative to holon root. Used as the success contract for `op build` when set. |

### Build configs

A holon may declare multiple **named build configurations** to
express build-time variants — such as license modes, feature sets,
or linkage strategies — without exposing runner-specific flags in
the manifest.

The mechanism has two layers:

1. **Universal envelope** — `op` knows about config names, selection
   (`--config`), defaults (`default_config`), and propagation to
   child builds. It never interprets the config contents.
2. **Runner injection** — `op` passes the selected config name to
   the runner as a well-known variable `OP_CONFIG`. The holon's own
   build system (CMakeLists.txt, go build tags, Cargo.toml, etc.)
   decides what the config name means.

```yaml
# megg-ffmpeg: LGPL vs GPL build
build:
  runner: cmake
  configs:
    lgpl:
      description: "LGPL-safe build, no GPL codecs"
    gpl:
      description: "Full GPL build with x264/x265"
  default_config: lgpl
```

Config entries are maps. The only universal field is `description`
(optional, for documentation). Runners may define additional
runner-specific fields in the future; unknown fields are rejected
at `op check` time.

When no `build.configs` is declared, `OP_CONFIG` is not set and
the runner builds with its own defaults.

### Skills fields

| Field | Type | Required | Description |
|---|---|---|---|
| `skills` | list | no | Composed workflows that describe how to use the holon's RPCs together. |
| `skills[].name` | string | yes | Skill identifier, kebab-case (e.g. `prepare-release`). |
| `skills[].description` | string | yes | What this skill achieves. |
| `skills[].when` | string | no | When to use this skill — the trigger or context. |
| `skills[].steps` | list of string | yes | Ordered sequence of RPC calls or instructions. |

Skills are natural language workflows. They describe **when** to use
the holon and **how** to combine its RPCs — complementing the proto
file which describes **what** each RPC does individually.

### Kind

- **`native`** — implements the capability from source.
  Example: `wisupaa-whisper` embeds whisper.cpp and compiles it
  directly.
- **`wrapper`** — delegates to an external CLI without implementing
  the domain logic itself.
  Example: `jess-npm` wraps the `npm` command.
- **`composite`** — assembled from multiple buildable parts into a
  single deliverable.
  Examples:
  - `gudule-greeting-godart` — Go daemon + Flutter (Dart) UI
  - `gudule-greeting-swift` — Go daemon + SwiftUI frontend
  - `gudule-greeting-rust-dart` — Rust daemon + Flutter (Dart) UI

### Wrapper delegation

A wrapper holon does not implement domain logic — it delegates to
an external command. Wrappers **must** declare `delegates.commands`
in their manifest so that `op check` can verify the external tool
is available before build:

```yaml
kind: wrapper
delegates:
  commands: [npm]          # jess-npm delegates to npm
```

```yaml
kind: wrapper
delegates:
  commands: [go]           # rob-go delegates to go
```

`op check` verifies each delegated command exists on `PATH` and
reports an actionable install hint if missing.

### Clade

The clade classifies a holon by its computational nature:

```
deterministic/pure          same input → same output, no state
deterministic/stateful      same input + same state → same output
deterministic/io_bound      deterministic logic, external dependencies

probabilistic/generative    output sampled from a distribution (LLM)
probabilistic/perceptual    approximation of ground truth (ASR, OCR)
probabilistic/adaptive      behavior changes over time (RL)
```

Why it matters: composition safety, testing strategy, auditability.

### Lifecycle

```
draft ──► stable ──► deprecated ──► dead
```

Only a human can promote a holon to `stable`.

### Naming conventions

A holon's canonical name is its **slug**:
`<given_name>-<family_name>`, lowercased, hyphenated. The slug is
used as directory name. For `native` and `wrapper` holons,
`artifacts.binary` must equal the slug. Composite holons use
`artifacts.primary` instead and may omit `artifacts.binary`.
The slug is the default way to reference a holon on the command
line. Path and UUID prefix are also valid selectors for
disambiguation (see [Collision handling](#collision-handling)).

The two-part naming convention minimizes collisions by design: the
given name provides uniqueness (a character), the family name
provides meaning (a function). Collision requires both to overlap,
which is unlikely when the composer names things deliberately.

- Examples: `rob-go`, `wisupaa-whisper`, `jess-npm`, `megg-ffmpeg`,
  `megg-ffprobe`, `line-git`, `gudule-greeting-godart`.
- Exception: the `op` binary keeps `op` as its name.

### Collision handling

If two holons share the same slug, `op` checks their UUIDs:

- **Same UUID** — the same holon found in two places (e.g. local
  root and `$OPBIN`). `op` uses the search order and picks the
  first match. No error.
- **Different UUIDs** — two distinct holons with the same slug.
  `op` does not silently pick one. It fails with a list of matches
  and asks the user to disambiguate by path or UUID:

```
$ op build rob-go
op build: ambiguous holon "rob-go" — found 2 matches (different UUIDs):

  1. [local]  ./holons/rob-go       UUID c7f3a1b2-...
  2. [$OPBIN] ~/.op/bin/rob-go      UUID d8e0f1a2-...

Disambiguate with a path or UUID:
  op build ./holons/rob-go
  op build c7f3a1b2
```

UUID prefix matching uses the shortest unique prefix (like Git).

---

## 5. Identity Management

Identity commands are built directly into `op`.

### `op new`

Interactively create a new holon identity:

```
$ op new
─── op new — New Holon Identity ───
UUID: a1b2c3d4-... (generated)

Family name (the function): Prober
Given name (the character): Megg
Composer: B. ALTER
Motto: Know what you have.
Clade (1-6): 3
Reproduction mode (1-5): 1
Implementation language [go]: go
Output directory [holons/megg-prober]:

✓ Born: Megg Prober
  UUID: a1b2c3d4-...
  File: holons/megg-prober/holon.yaml
```

Non-interactive mode:

```bash
op new --json '{"given_name":"Megg", "family_name":"Prober", "composer":"B. ALTER", "motto":"Know what you have.", "clade":"deterministic/io_bound"}'
```

### `op show <uuid-or-prefix>`

Display a holon's identity by full UUID or prefix:

```bash
op show a1b2c3d4
op show b00932e5-49d4-4724-ab4b-e2fc9e22e108
```

### `op list [root]`

List all known holons — local and cached:

```
$ op list
UUID                                   NAME                              ORIGIN   CLADE                     STATUS   PATH
──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
c7f3a1b2-8d4e-4f5a-b6c7-d8e9f0a1b2c3   Rob Go                            local    deterministic/io_bound    draft    holons/rob-go
d9e0f1a2-3b4c-5d6e-7f8a-9b0c1d2e3f4a   Jess NPM                          local    deterministic/io_bound    draft    holons/jess-npm
a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d   Wisupaa Whisper                    local    probabilistic/perceptual  draft    holons/wisupaa-whisper
```

Scans:
1. `<root>` recursively — every `holon.yaml` under the designated root
2. `$OPPATH/cache/` — cached dependencies

---

## 6. Discovery

### `op discover`

List all reachable holons with their origin:

```
$ op discover
NAME              LANG  CLADE                    STATUS  ORIGIN  REL_PATH              UUID
Rob Go            go    deterministic/io_bound   draft   local   rob-go                c7f3a1b2-...
Jess NPM          go    deterministic/io_bound   draft   local   jess-npm              d9e0f1a2-...
Wisupaa Whisper   c++   probabilistic/perceptual draft   local   wisupaa-whisper        a1b2c3d4-...

In $PATH:
  op -> /Users/alice/.op/bin/op
  rob-go -> /Users/alice/.op/bin/rob-go
```

### Search order

When resolving a holon by name, `op` searches in this order:

```
1. Effective local root    recursive scan for every holon.yaml under the designated root
2. $OPBIN                  ~/.op/bin/ (installed holons)
3. $PATH                   system PATH
4. $OPPATH/cache/          cached dependencies (populated by op mod pull)
```

### Effective root

The effective local root is:

1. the explicit root argument when a command accepts one
2. otherwise the current working directory

Discovery then scans recursively for every `holon.yaml` under that
root. There is no requirement that holons live only under fixed
folders such as `holons/` or `recipes/`.

### Exclusion rules

Recursive discovery skips the following directories:

- `.git`
- `.op`
- `node_modules`
- `vendor`
- `build`
- Any directory starting with `.`

Deduplication: if two `holon.yaml` files have the same UUID, the
one closest to the effective root wins.

### Name resolution

The slug (directory name) is the default symbolic name. Path and
UUID prefix are also valid selectors. There are no aliases, no
partial-name matching, no fuzzy resolution.

```bash
op build rob-go                     # slug (default)
op build ./path/to/rob-go           # explicit path
op build c7f3a1b2                   # UUID prefix
```

---

## 7. Lifecycle

`op` manages the full lifecycle of a holon through six commands:

```
op check   → op build   → op install
                ↓
             op test
                ↓
             op clean   → op uninstall
```

### `op check [<holon-or-path>]`

Validate the manifest and preflight the build contract without
compiling. Verifies:

- Schema validity (`holon/v0`)
- Platform support (current OS vs `platforms` field)
- Required files exist (e.g. `go.mod`)
- Required commands exist on PATH (e.g. `go`, `cmake`)
- Delegated commands for wrappers
- Runner-specific entrypoint expectations (e.g. `build.main` for
  go-module)

```bash
$ op check
$ op check rob-go
$ op check ./path/to/holon
```

### `op build [<holon-or-path>] [flags]`

Build the primary artifact via the declared runner.

```bash
$ op build                          # build current directory
$ op build rob-go                   # build by name
$ op build --target macos --mode release
$ op build --config gpl             # select a named build config
$ op build --dry-run                # print plan, don't execute
```

Flags:

| Flag | Values | Default | Description |
|---|---|---|---|
| `--target` | `macos`, `linux`, `windows`, `ios`, `ios-simulator`, `tvos`, `tvos-simulator`, `watchos`, `watchos-simulator`, `visionos`, `visionos-simulator`, `android`, `all` | current OS | Platform target |
| `--mode` | `debug`, `release`, `profile` | `debug` | Build mode |
| `--config` | any key from `build.configs` | `build.default_config` | Named build configuration |
| `--dry-run` | | | Print resolved plan without executing |

**Success contract**: a successful `op build` guarantees:
1. The manifest was valid.
2. The target was supported.
3. All prerequisites existed.
4. The runner completed all build steps.
5. The primary artifact exists at the declared path.

If the runner exits zero but the artifact is missing, `op build`
fails.

### `op test [<holon-or-path>]`

Run the holon's test contract:

```bash
$ op test
$ op test rob-go
```

Runner-specific:
- `go-module`: `go test ./...`
- `cmake`: `ctest --test-dir .op/build/cmake --output-on-failure`

### `op clean [<holon-or-path>]`

Remove the `.op/` build directory:

```bash
$ op clean
$ op clean rob-go
```

### `op install [<holon-or-path>]`

Build (if needed) and copy the artifact to `$OPBIN`:

```bash
$ op install                        # install current holon
$ op install rob-go                 # install by name
$ op install --no-build             # fail if not already built
```

Steps:
1. Resolve the target holon.
2. If `artifacts.binary` is not declared, fail:
   `op install: holon "gudule-greeting-godart" has no installable binary (composite with artifacts.primary only)`
3. If no artifact at `.op/build/bin/<binary>`, run `op build`.
4. Create `$OPBIN` if needed.
5. Copy artifact to `$OPBIN/<artifacts.binary>`.
6. Report the installed path.

Scope:
- `op install` is **binary-only** in v0.
- It copies `artifacts.binary` into `$OPBIN`.
- If the primary artifact is not an executable binary (for example a
  `.app` bundle), `op install` fails with an actionable error.
- Non-binary deployment is out of scope for v0.

### `op uninstall <holon>`

Remove an installed holon from `$OPBIN`:

```bash
$ op uninstall rob-go
```

### Lifecycle report

All lifecycle commands produce a structured `Report`:

```json
{
  "operation": "build",
  "target": "rob-go",
  "holon": "Rob Go",
  "dir": "/path/to/rob-go",
  "manifest": "holon.yaml",
  "kind": "wrapper",
  "runner": "go-module",
  "build_target": "macos",
  "build_mode": "debug",
  "build_config": "standard",
  "artifact": ".op/build/bin/rob-go",
  "commands": ["go build -o .op/build/bin/rob-go ./cmd/rob"],
  "notes": [],
  "children": []
}
```

Use `--format json` to get the report as JSON instead of text.

---

## 8. Runners

A runner translates `op` lifecycle commands into language-specific
toolchain invocations. All build output is placed under `.op/`.

### `go-module` (leaf runner)

For holons written in Go.

| Operation | Command |
|---|---|
| check | verify `go.mod`, `build.main` or `./cmd/<dir>` exists |
| build | `go build -o .op/build/bin/<binary> <build.main>` |
| test | `go test ./...` |
| clean | remove `.op/` |

When `build.configs` is declared, `OP_CONFIG` is set as an
environment variable during `go build` and `go test`.

### `cmake` (leaf runner)

For holons written in C/C++.

| Operation | Command |
|---|---|
| check | verify `CMakeLists.txt` exists |
| build (configure) | `cmake -S . -B .op/build/cmake -DCMAKE_BUILD_TYPE=<mode> -DCMAKE_RUNTIME_OUTPUT_DIRECTORY=.op/build/bin [-DOP_CONFIG=<config>]` |
| build (compile) | `cmake --build .op/build/cmake --config <mode>` |
| test | `ctest --test-dir .op/build/cmake --output-on-failure -C <mode>` |
| clean | remove `.op/` |

Mode mapping: `debug` → `Debug`, `release` → `Release`,
`profile` → `RelWithDebInfo`.

When `build.configs` is declared, `OP_CONFIG` is passed as a
CMake define (`-DOP_CONFIG=<config>`) during the configure step.
The `CMakeLists.txt` is responsible for interpreting the value:

```cmake
# Example: megg-ffmpeg license selection
if(OP_CONFIG STREQUAL "gpl")
    list(APPEND FFMPEG_CONFIGURE_FLAGS --enable-gpl)
endif()
```

CMake holons must register tests with CTest. If no tests are
registered, `op test` fails with an actionable error.

### `recipe` (orchestration runner)

For composite holons assembled from multiple parts.

Recipe manifest:

```yaml
kind: composite
build:
  runner: recipe
  defaults:
    target: macos
    mode: debug
  members:
    - id: daemon
      path: greeting-daemon
      type: holon
    - id: app
      path: greeting-godart
      type: component
  targets:
    macos:
      steps:
        - build_member: daemon
        - copy:
            from: greeting-daemon/gudule-daemon
            to: build/gudule-daemon
        - exec:
            cwd: greeting-godart
            argv: ["flutter", "pub", "get"]
        - exec:
            cwd: greeting-godart
            argv: ["flutter", "build", "macos", "--debug"]
        - assert_file:
            path: greeting-godart/build/macos/.../app
artifacts:
  primary: greeting-godart/build/macos/.../app
```

#### Step types

| Step | Description |
|---|---|
| `build_member` | Recursively `op build` a member of type `holon`. Accepts optional `config:` to override the child's default build config. |
| `exec` | Run a command (argv array, explicit `cwd`, no shell) |
| `copy` | Copy a file from one manifest-relative path to another |
| `assert_file` | Verify a file exists (packaging validation) |

No shell interpolation. No loops or conditionals in v0.

### Future runners (not yet implemented)

None of these runners exist yet. They are reserved names for future
implementation:

- `dart-package` — Flutter/Dart builds
- `cargo` — Rust builds
- `swift-package` — Swift Package Manager builds
- `gradle` — Android/Kotlin/Java builds
- `dotnet` — C#/.NET builds

Until a runner is implemented, composite recipes can invoke these
toolchains through `exec` steps in the `recipe` runner.

---

## 9. Transport & Dispatch

`op` dispatches commands to holons through a **transport chain**.
The chain selects the best available transport automatically.

### Transport selection priority

When `op <holon> <command>` is invoked:

```
1. mem://     in-process composition (Go holons compiled into op)
2. stdio://   ephemeral subprocess via stdin/stdout pipes
3. tcp://     gRPC over TCP (ephemeral or existing server)
```

- **mem://** is used when the holon is registered in the in-process
  compose registry. Zero process spawn overhead. Only available for
  Go holons compiled into `op`.
- **stdio://** is used when a holon binary is found locally. `op`
  launches it with `serve --listen stdio://`, communicates via
  stdin/stdout gRPC pipe, then kills the process.
- **tcp://** is used as fallback. `op` starts the binary on an
  ephemeral TCP port, waits for readiness, calls the RPC, then kills
  the process.

### `op <holon> <command> [args]`

Dispatch a command to any holon through the transport chain:

```bash
op rob-go build '{"package":"./..."}'
op jess-npm install '{"packages":["express"]}'
op wisupaa-whisper transcribe '{"file":"audio.wav"}'
```

The command is mapped to an RPC method name:

| Command | RPC Method |
|---|---|
| `new` | `CreateIdentity` |
| `list` | `ListIdentities` |
| `show` | `ShowIdentity` |
| *other* | used as-is (e.g. `build` → `Build`, `install` → `Install`) |

### Direct gRPC URI dispatch

For fine-grained transport control, use URI syntax:

```bash
# TCP — connect to existing server (authority contains ':')
op grpc://localhost:9090 Build

# TCP — ephemeral by holon name (authority has no ':')
op grpc://rob-go Build

# Stdio pipe — launch binary, pipe gRPC, done
op grpc+stdio://rob-go Build

# Unix domain socket
op grpc+unix:///tmp/rob.sock Build

# WebSocket
op grpc+ws://localhost:8080 Build
op grpc+wss://secure.example.com Build
```

**Parsing rule**: if the authority part of `grpc://` contains a `:`
(port), it is treated as a host address. Otherwise it is treated as
a holon name and `op` resolves the binary, starts it ephemerally,
and dials it.

If no method is provided on a TCP URI, `op` lists available methods
via gRPC reflection:

```bash
$ op grpc://localhost:9090
Available methods at localhost:9090:
  /rob_go.v1.RobGoService/Build
  /rob_go.v1.RobGoService/Test
  /rob_go.v1.RobGoService/Run
```

### Input format

RPC input is JSON:

```bash
op grpc://rob-go Build '{"package":"./cmd/rob"}'
op grpc+stdio://jess-npm Install '{"packages":["express"]}'
```

If no JSON is provided, `{}` is used.

---

## 10. Serve & Dial

Every holon that exposes a gRPC contract can be started as a
long-running server. `op` provides two ways to do this.

### `op serve [--listen <URI>]`

Start **op's own gRPC server** (the OPService):

```bash
op serve                            # default: tcp://:9090
op serve --listen tcp://:8080
op serve --listen unix:///tmp/op.sock
op serve --listen stdio://
```

This exposes the `OPService` contract. The authoritative list of
RPCs is defined by the `.proto` file; current methods include:
`Discover`, `Invoke`, `CreateIdentity`, `ShowIdentity`,
`ListIdentities`.

### `op run <holon>:<port>`

Start **any holon's gRPC server** as a background process:

```bash
op run rob-go:9091                  # TCP shorthand
op run jess-npm --listen tcp://:9092
op run wisupaa-whisper --listen unix:///tmp/whisper.sock
```

Under the hood:
1. Resolve the holon binary.
2. Launch: `<binary> serve --listen <URI>`.
3. Detach the process.
4. Print PID and listen URI.

```
$ op run rob-go:9091
op run: started rob-go (pid 12345) on tcp://:9091
op run: stop the process by PID using your platform's process tool
```

### Transport URIs

All transport URIs follow a consistent scheme:

| URI | Transport | Direction |
|---|---|---|
| `tcp://:9090` | TCP on all interfaces | serve |
| `tcp://localhost:9090` | TCP on localhost | dial |
| `unix:///tmp/holon.sock` | Unix domain socket | serve/dial |
| `stdio://` | Standard I/O pipes | serve (child process) |
| `ws://host:port/grpc` | WebSocket | dial |
| `wss://host:port/grpc` | Secure WebSocket | dial |
| `mem://` | In-process listener | internal only |

### The `serve --listen` contract

Every holon that implements a gRPC service must support:

```bash
<binary> serve --listen <URI>
```

This is Article 11 of the holon conventions. `op` does not embed
server logic for other holons — it launches them with this contract.

---

## 11. Dependency Management

Dependency commands are built directly into `op`.
They manage `holon.mod` and `holon.sum` files, following the same
philosophy as Go modules.

### `op mod init [holon-path]`

Create a `holon.mod` file in the current directory:

```bash
$ op mod init
created holon.mod
```

Initialization behavior:
- If an explicit holon path is provided, use it as-is.
- Otherwise, if `./holon.yaml` exists, derive the holon path from the
  identity slug (`<given_name>-<family_name>`, lowercased,
  hyphenated).
- Otherwise, fall back to the current directory name.
- If none of these produce a usable value, fail with an actionable
  error.

### `op mod add <module> [version]`

Add a dependency:

```bash
op mod add github.com/organic-programming/wisupaa-whisper v0.1.0
```

If `version` is omitted, `op` resolves the latest published tag
automatically.

### `op mod remove <module>`

Remove a dependency:

```bash
op mod remove github.com/organic-programming/wisupaa-whisper
```

### `op mod tidy`

Clean up `holon.mod` / regenerate `holon.sum`:

```bash
op mod tidy
```

Scope:
- `op mod tidy` only operates on `holon.mod` and `holon.sum`.
- It does **not** touch `go.mod`, `package.json`, `Cargo.toml`, or any
  language-level dependency file.
- It cleans the holon dependency set and regenerates `holon.sum`.

### `op mod pull`

Fetch all declared dependencies into `$OPPATH/cache/`:

```bash
op mod pull
```

Cache structure: `$OPPATH/cache/<module>@<version>/`

### `op mod update [<module>]`

Update one or all dependencies to latest allowed version:

```bash
op mod update                       # update all
op mod update github.com/org/dep    # update one
```

### `op mod list`

List declared dependencies:

```bash
op mod list
```

### `op mod graph`

Print the dependency graph:

```bash
op mod graph
```

### Resolution strategy

`op` uses **Minimum Version Selection (MVS)**, the same algorithm
used by Go modules. For each dependency, `op` selects the maximum
of all minimum versions required across the dependency graph.
Deterministic, no solver, no SAT.

---

## 12. Output & Formatting

### Global format flag

```bash
op --format json <command>
op -f json <command>
```

Two formats:
- `text` (default) — human-readable tabular or line output.
- `json` — structured JSON on stdout.

The flag must come **before** the command name.

### RPC output

When dispatching to a holon via gRPC, the response is formatted as:
- `text`: key-value pairs or the raw response.
- `json`: pretty-printed protobuf JSON.

### Lifecycle report output

All lifecycle commands (`check`, `build`, `test`, `clean`, `install`)
produce a `Report` struct (see [Lifecycle](#7-lifecycle)) that can
be output as text or JSON.

---

## 13. Error Model

All commands follow the same pattern:

- **Exit 0** on success.
- **Exit 1** on failure with `op <cmd>: <message>` on stderr.
- `--format json` still produces structured output when possible.

### Failure categories

| Category | Example |
|---|---|
| Invalid manifest | `op build: holon.yaml: missing required field "uuid"` |
| Unsupported runner | `op build: unknown runner "cargo"` |
| Unsupported target | `op build: target "android" not supported by go-module runner` |
| Missing prerequisite command | `op check: missing required command "go" on PATH; install it with...` |
| Missing prerequisite file | `op check: missing required file "go.mod"` |
| Failed child build | `op build: child build "daemon" failed` |
| Failed command step | `op build: exec step failed: flutter build macos` |
| Missing primary artifact | `op build: artifact not found at .op/build/bin/rob-go` |
| Holon not found | `op: holon "foo" not found` |
| Transport failure | `op grpc: cannot start rob-go: exec: "rob-go": not found` |

Install hint: when a required command is missing, `op` provides a
platform-specific install suggestion (e.g. `brew install cmake` on
macOS, `sudo apt install cmake` on Linux).

---

## 14. Introspection

Every holon carries its `.proto` contract (Article 2, Article 13).
`op` reads these files directly to provide rich introspection —
the holon author/developer does not need to implement any introspection logic, as the SDK auto-registers the `HolonMeta` service at runtime, and `op inspect` statically parses `.proto` files.

### `op inspect <slug>`

Offline API documentation. Reads `protos/` from the holon's
directory — the holon does not need to be running.

```bash
op inspect rob-go              # human-readable API reference
op inspect rob-go --json       # structured JSON output
op inspect localhost:9090      # fallback: call Describe RPC
```

The human-readable output:

```
rob-go — Build what you mean.

  rob_go.v1.RobGoService
    Wraps the go command for gRPC access.

    Build(BuildRequest) → BuildResponse
      Compile Go packages.

      Request:
        package  string  [required]  The Go package to build.
                                     @example "./cmd/rob"

      Response:
        output   string  Compiler output.
        success  bool    Whether the build succeeded.
```

`op inspect` extracts `@required` and `@example` tags from proto
comments. These are plain comment conventions, not proto syntax:

```protobuf
message BuildRequest {
  // The Go package to build.
  // @required
  // @example "./cmd/rob"
  string package = 1;
}
```

When given a `host:port` address (an already-running holon), `op
inspect` falls back to calling the `HolonMeta.Describe` RPC.

If the holon declares `skills` in its manifest, `op inspect` shows
them after the API reference:

```
  Skills:
    prepare-release — Prepare a Go package for production release.
      When: User wants clean, tested, optimized code ready to ship.
      Steps:
        1. Fmt — format all source files
        2. Vet — run static analysis
        3. Test — run the full test suite
        4. Build with mode=release
```

### `op mcp <slug> [slug2...]`

Start an MCP server that exposes one or more holons as MCP tools.

```bash
op mcp rob-go                        # one holon
op mcp rob-go jess-npm echo-server   # multiple holons
```

This is the bridge between the organic ecosystem and any AI agent
that speaks MCP (Claude, Cursor, Windsurf, etc.).

**How it works:**

1. Read each holon's `protos/` → parse services, methods, types
2. Extract `@required`, `@example` tags from comments
3. Generate MCP tool definitions with JSON Schema (from the
   proto type tree) for each RPC method
4. Start an MCP server over stdio (per MCP specification)
5. When the LLM calls a tool → `op` translates the JSON request
   into a gRPC call → `connect(slug)` → invoke the RPC →
   return the response as JSON

The JSON Schema generation is internal to `op` — the holon never
sees it. The holon just serves its domain RPCs over gRPC as always.

If holons declare `skills` in `holon.yaml`, `op mcp` exposes them
as **MCP prompts** — composed workflows that guide the LLM through
multi-step operations.

**Every holon in the ecosystem is instantly MCP-compatible.**

### `op tools <slug>`

Output LLM tool definitions for a holon's RPCs.

```bash
op tools rob-go                      # default format
op tools rob-go --format openai      # OpenAI function calling
op tools rob-go --format anthropic   # Anthropic tool use
op tools rob-go --format mcp         # MCP tool list
```

Same proto parsing as `op inspect`, different output format.
Useful for embedding tool definitions in agent prompts or
configuration files.

---

## 15. Complete Command Reference

### Identity

| Command | Description |
|---|---|
| `op new` | Interactively create a holon identity |
| `op new --json '<json>'` | Non-interactive identity creation |
| `op show <uuid-or-prefix>` | Display a holon's identity |
| `op list [root]` | List holons (local + cached) |

### Lifecycle

| Command | Description |
|---|---|
| `op check [<holon>]` | Validate manifest and prerequisites |
| `op build [<holon>] [--target T] [--mode M] [--dry-run]` | Build primary artifact |
| `op test [<holon>]` | Run test contract |
| `op clean [<holon>]` | Remove `.op/` build outputs |
| `op install [<holon>] [--no-build]` | Copy artifact to `$OPBIN` |
| `op uninstall <holon>` | Remove artifact from `$OPBIN` |

### Dependencies

| Command | Description |
|---|---|
| `op mod init [holon-path]` | Create `holon.mod`; resolve path from argument, local slug, or directory name |
| `op mod add <module> [version]` | Add a dependency; resolve latest tag if version is omitted |
| `op mod remove <module>` | Remove a dependency |
| `op mod tidy` | Clean `holon.mod` and regenerate `holon.sum` only |
| `op mod pull` | Fetch all deps into `$OPPATH/cache/` |
| `op mod update [<module>]` | Update one or all deps |
| `op mod list` | List declared dependencies |
| `op mod graph` | Print dependency graph |

### Environment

| Command | Description |
|---|---|
| `op env` | Print resolved `OPPATH`, `OPBIN`, `ROOT` |
| `op env --init` | Create `~/.op/` directory structure |
| `op env --shell` | Output shell config snippet |

### Discovery

| Command | Description |
|---|---|
| `op discover` | List all reachable holons |

### Runtime

| Command | Description |
|---|---|
| `op run <holon>:<port>` | Start holon gRPC server (TCP) |
| `op run <holon> --listen <URI>` | Start holon server (any transport) |
| `op serve [--listen <URI>]` | Start op's own gRPC server |

### Dispatch

| Command | Description |
|---|---|
| `op <holon> <command> [args]` | Transport-chain dispatch |
| `op grpc://<host:port> [method]` | gRPC over TCP |
| `op grpc+stdio://<holon> <method>` | gRPC over stdio pipe |
| `op grpc+unix://<path> <method>` | gRPC over Unix socket |
| `op grpc+ws://<host:port> <method>` | gRPC over WebSocket |
| `op grpc+wss://<host:port> <method>` | gRPC over secure WebSocket |

### Meta

| Command | Description |
|---|---|
| `op version` | Print version |
| `op help` | Print usage |

### Introspection

| Command | Description |
|---|---|
| `op inspect <slug>` | Offline API documentation from protos/ |
| `op inspect <slug> --json` | Structured JSON output |
| `op inspect <host:port>` | Fallback: call Describe RPC on running holon |
| `op mcp <slug> [slug2...]` | Start MCP server exposing holon RPCs as tools |
| `op tools <slug>` | Output LLM tool definitions |
| `op tools <slug> --format <fmt>` | Specific format: openai, anthropic, mcp |
