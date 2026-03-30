# `op` — The Organic Programming CLI

> One command, every holon.

`op` is the unified entry point to the Organic Programming ecosystem.
It discovers holons, builds them, manages their identities and
dependencies, and dispatches commands to them through a single
interface. The actant installs one binary and gets access to every
holon.

```bash
op help
op — the Organic Programming CLI

Global flags (must come before <holon> or URI):
  -f, --format <text|json>              output format for RPC responses (default: text)
  -q, --quiet                           suppress progress and suggestions
  --root <path>                         override discovery root (default: cwd)
  --bin <slug>                          print the resolved binary path and exit

Holon dispatch (transport chain):
  op <holon> <command> [args]            dispatch via the SDK auto-connect chain
  op <holon> --clean <method> [--no-build] [json]
  op <holon> <method> [--no-build] [json]
                                         call a holon RPC; auto-build compiled slugs if needed
  op <binary-path> <method> [json]       call an executable directly (no discovery)

Direct gRPC URI dispatch:
  op grpc://<slug|host:port> <method>    gRPC auto-connect for slugs, direct TCP for host:port
  op tcp://<slug|host:port> <method>     force gRPC over TCP
  op stdio://<holon> <method>            force gRPC over stdio pipe (ephemeral)
  op unix://<path> <method>              gRPC over Unix socket
  op ws://<host:port> <method>           gRPC over WebSocket
  op wss://<host:port> <method>          gRPC over secure WebSocket
  op http://<host:port> <method>         gRPC over HTTP REST + SSE
  op https://<host:port> <method>        gRPC over secure HTTP REST + SSE
  op run <holon> [flags]                 build if needed, then launch in foreground
  op run <holon>:<port>                  shorthand for --listen tcp://:<port>

OP commands:
  op list [root]                         list local + cached holons natively
  op show <uuid-or-prefix>               display a holon identity natively
  op new [--json <payload>]              create a holon identity natively
  op new --list                          list shipped holon templates
  op new --template <name> <holon-name>  generate a holon scaffold from a template
  op inspect <slug|host:port> [--json]   inspect a holon's API offline or via Describe
  op do <holon> <sequence> [--param=value ...]
                                         run a declared manifest sequence
  op mcp <slug> [slug2...]               start an MCP server for one or more holons
  op mcp <tcp://host:port>               start an MCP server for a running gRPC server
  op tools <slug> [--format <fmt>]       output tool definitions (openai, anthropic, mcp)
  op check [<holon-or-path>]             validate the holon manifest and prerequisites
  op build [<holon-or-path>] [flags]     build a holon artifact via its runner
  op test [<holon-or-path>]              run a holon's test contract
  op clean [<holon-or-path>]             remove .op/ build outputs
  op install [<holon-or-path>] [flags]   install a pre-built artifact into $OPBIN
  op uninstall <holon>                   remove an installed artifact from $OPBIN
  op mod <command>                       manage holon.mod and holon.sum
  op env [--init] [--shell]              print resolved OPPATH / OPBIN / ROOT

Build flags:
  --clean                                      clean before building (cannot be combined with --dry-run)
  --target <macos|linux|windows|ios|ios-simulator|tvos|tvos-simulator|watchos|watchos-simulator|visionos|visionos-simulator|android|all>   platform target (default: current OS)
  --mode <debug|release|profile>               build mode (default: debug)
  --dry-run                                    print resolved plan, do not execute
  --no-sign                                    skip automatic ad-hoc signing for bundle artifacts

Install flags:
  --build                                      build before installing (default: install pre-built artifact as-is)
  --link-applications                          symlink installed .app bundles into /Applications (macOS only)

Run flags:
  --clean                                      clean before building and running (cannot be combined with --no-build)
  --listen <URI>                               listen address for service holons (default: stdio://)
  --no-build                                   fail if the artifact is missing instead of building
  --target <...>                               pass build target through if a build is needed
  --mode <debug|release|profile>               pass build mode through if a build is needed

Dispatch flag:
  --clean                                      clean the slug target before auto-building and calling
  --no-build                                   fail if a slug-based RPC target is missing its built binary

  op discover                            list available holons
  op serve [--listen tcp://:9090]        start OP's own gRPC server
  op version                             show op version
  op help [command]                      this message or topic help (build, run)
```


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
4. [Holon Proto Manifest](#4-holon-proto-manifest)
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

See [INSTALL.md](INSTALL.md) for all installation methods, environment
setup, shell completion, and uninstall instructions.

---

## 2. Philosophy

### One command, every holon

`op` hides the complexity of holon discovery, transport selection,
and binary location. The actant (human or agent) only needs to know
the holon's name and the command to invoke. `op` resolves the rest.

### Orchestrator, not compiler

`op` does not compile source code. It reads `api/v1/holon.proto`, selects
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
  `holon.proto` via the manifest `platforms` field. `op check` verifies that
  the current OS is in that list before building.

---

## 3. Environment

### Environment variables

| Variable | Default | Purpose |
|---|---|---|
| `OPPATH` | `~/.op` | User-local runtime home for all OP data |
| `OPBIN` | `$OPPATH/bin` | Canonical install directory for holon binaries |
| `OPROOT` | cwd | Override the discovery root (set automatically by `--root`) |

Resolution: explicit env var if set, otherwise the default.

### `op env`

Print the resolved environment:

```
$ op env
OPPATH=/Users/Bob/.op
OPBIN=/Users/Bob/.op/bin
ROOT=/Users/Bob/organic-programming
```

### `op env --init`

Create the runtime directory structure:

```
$ op env --init
created /Users/Bob/.op/
created /Users/Bob/.op/bin/
created /Users/Bob/.op/cache/
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

## 4. Holon Proto Manifest

Every holon carries a single human-authored manifest at
`api/v1/holon.proto`. `op` reads the `option (holons.v1.manifest)`
block in that file and treats it as the source of truth for identity,
contract, operational metadata, skills, sequences, and guide text.

It answers three questions:

1. **Who is this holon?** — identity, version, motto
2. **What does it do?** — description, contract
3. **How does `op` operate it?** — kind, runner, requires, artifacts

### Example manifest

```protobuf
syntax = "proto3";

package rob_go.v1;

import "holons/v1/manifest.proto";
import "rob_go/v1/rob_go.proto";

option go_package = "github.com/organic-programming/rob-go/gen/go/rob_go/v1;robgov1";

option (holons.v1.manifest) = {
  identity: {
    schema: "holon/v1"
    uuid: "c7f3a1b2-8d4e-4f5a-b6c7-d8e9f0a1b2c3"
    given_name: "Rob"
    family_name: "Go"
    motto: "Build what you mean."
    composer: "B. ALTER"
    status: "draft"
    born: "2026-02-20"
    version: "0.1.0"
  }
  description: "Rob Go wraps the go command, exposing build, test, run, fmt, vet, mod, and env as gRPC RPCs."
  lang: "go"
  kind: "wrapper"
  contract: {
    proto: "api/v1/holon.proto"
    service: "rob_go.v1.RobGoService"
    rpcs: "Build"
    rpcs: "Test"
    rpcs: "Run"
    rpcs: "Fmt"
    rpcs: "Vet"
  }
  platforms: "macos"
  platforms: "linux"
  platforms: "windows"
  build: {
    runner: "go-module"
    main: "./cmd/rob"
  }
  requires: {
    commands: "go"
    files: "go.mod"
  }
  artifacts: {
    binary: "rob-go"
  }
  skills: [{
    name: "prepare-release"
    description: "Prepare a Go package for production release."
    when: "User wants clean, tested, optimized code ready to ship."
    steps: "Fmt — format all source files"
    steps: "Vet — run static analysis"
    steps: "Test — run the full test suite"
    steps: "Build with mode=release"
  }]
};
```

For the authoritative field definitions, see
[PROTO.md](../../PROTO.md) and
[`_protos/holons/v1/manifest.proto`](./_protos/holons/v1/manifest.proto).

### Identity fields

| Field | Type | Required | Description |
|---|---|---|---|
| `identity.schema` | string | yes | Always `holon/v1`. |
| `identity.uuid` | UUID v4 | yes | Generated once at birth. Never changes. |
| `identity.given_name` | string | yes | The character — what distinguishes this holon. |
| `identity.family_name` | string | yes | The function — what the holon does. |
| `identity.motto` | string | yes | The *dessein* in one sentence. |
| `identity.composer` | string | yes | Who designed the holon. |
| `identity.status` | enum | yes | Lifecycle stage: `draft`, `stable`, `deprecated`, `dead`. |
| `identity.born` | date | yes | ISO 8601 date of creation. |
| `identity.version` | semver | yes | Release version without `v` prefix. Patch is auto-incremented by `op build`; major/minor set by humans. |

### Contract fields

| Field | Type | Required | Description |
|---|---|---|---|
| `contract.proto` | string | no | Path to the proto file that carries the exposed service definition. |
| `contract.service` | string | no | Fully qualified gRPC service name. |
| `contract.rpcs` | list | no | Exhaustive public RPC method names for the holon surface. |

Omit `contract` entirely if the proto is not yet defined.

### Operational fields

| Field | Type | Required | Description |
|---|---|---|---|
| `description` | string | yes | What the holon does, in plain language. |
| `lang` | string | yes | Primary implementation language. |
| `kind` | enum | yes | `native`, `wrapper`, or `composite`. |
| `transport` | string | no | Default transport hint such as `stdio` or `tcp`. |
| `platforms` | list | no | Supported OSes. Omit if cross-platform. |
| `build.runner` | string | yes | Selects the runner (see [Runners](#8-runners)). |
| `build.main` | string | no | Runner entry point (for example `./cmd/rob` for `go-module`). |
| `build.configs` | map | no | Named build configurations (see [Build configs](#build-configs)). |
| `build.default_config` | string | no | Default config name. Required when `build.configs` is set. |
| `requires.commands` | list | yes | CLI tools that must exist on `PATH`. |
| `requires.files` | list | yes | Files that must exist relative to the holon root. |
| `artifacts.binary` | string | yes for `native`/`wrapper`, no for `composite` | Primary binary name (not a path). Must equal the slug for `native`/`wrapper`. Build output is `.op/build/bin/<artifacts.binary>`, install destination is `$OPBIN/<artifacts.binary>`. |
| `artifacts.primary` | string | no for `native`/`wrapper`, yes for `composite` | Non-binary primary artifact path (e.g. `.app` bundle), relative to holon root. Used as the success contract for `op build` when set. |
| `guide` | string | no | User-facing markdown rendered by `op man`. |

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
  Example: `gabriel-greeting-c` implements the greeting service
  in C and compiles it directly.
- **`wrapper`** — delegates to an external CLI without implementing
  the domain logic itself.
  Example: `rob-go` wraps the `go` command.
- **`composite`** — assembled from multiple buildable parts into a
  single deliverable.
  Examples:
  - `gudule-greeting-godart` — Go daemon + Flutter (Dart) UI
  - `gudule-greeting-swift` — Go daemon + SwiftUI frontend
  - `gudule-greeting-rust-dart` — Rust daemon + Flutter (Dart) UI

### Wrapper delegation

A wrapper holon does not implement domain logic — it delegates to
an external command. Wrappers **must** declare those binaries in
`requires.commands` so that `op check` can verify the external tool
is available before build.

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

- Examples: `rob-go`, `gabriel-greeting-go`, `gabriel-greeting-c`,
  `gabriel-greeting-node`, `megg-ffmpeg`, `gudule-greeting-godart`.
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
  File: holons/megg-prober/api/v1/holon.proto
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
f1a2b3c4-5d6e-7f8a-9b0c-1d2e3f4a5b6c   Gabriel Greeting Go               local    deterministic/pure        draft    examples/hello-world/gabriel-greeting-go
d1e2f3a4-5b6c-7d8e-9f0a-1b2c3d4e5f6a   Gabriel Greeting C                local    deterministic/pure        draft    examples/hello-world/gabriel-greeting-c
```

Scans:
1. `<root>` recursively — every `api/v1/holon.proto` under the designated root
2. `$OPPATH/cache/` — cached dependencies

---

## 6. Discovery

### `op discover`

List all reachable holons with their origin:

```
$ op discover
NAME              LANG  CLADE                    STATUS  ORIGIN  REL_PATH              UUID
Rob Go              go    deterministic/io_bound   draft   local   rob-go                c7f3a1b2-...
Gabriel Greeting Go go    deterministic/pure       draft   local   gabriel-greeting-go   f1a2b3c4-...
Gabriel Greeting C  c     deterministic/pure       draft   local   gabriel-greeting-c    d1e2f3a4-...

In $PATH:
  op -> /Users/Bob/.op/bin/op
  rob-go -> /Users/Bob/.op/bin/rob-go
```

### Search order

When resolving a holon by name, `op` searches in this order:

```
1. Effective local root    recursive scan for `api/v1/holon.proto` + `*.holon/` package dirs
2. $OPBIN                  ~/.op/bin/ (installed holons)
3. $PATH                   system PATH
4. $OPPATH/cache/          cached dependencies (populated by op mod pull)
```

For `.holon` package directories, `op` reads `.holon.json` for fast
identity resolution. If `.holon.json` is missing, `op` probes the
binary via stdio `Describe` and caches the result as `.holon.json`
for subsequent scans.

### Effective root

The effective local root is:

1. the value of `--root <path>` (sets `OPROOT` env var)
2. the `OPROOT` env var if set directly
3. the explicit root argument when a command accepts one
4. otherwise the current working directory

```bash
# Override discovery root for a single call:
op --root ~/Desktop/isolated gabriel-greeting-go SayHello '{"name":"","lang_code":"fr"}'

# Print the resolved binary path (--bin):
op --bin gabriel-greeting-go
# → /path/to/.op/build/grace-op.holon/bin/darwin_arm64/gabriel-greeting-go
```

Discovery then scans recursively for every `api/v1/holon.proto` under that
root. There is no requirement that holons live only under fixed
folders such as `holons/` or `recipes/`.

### Exclusion rules

Recursive discovery skips the following directories:

- `.git`
- `.op`
- `node_modules`
- `vendor`
- `build`
- Any directory starting with `.` (except `*.holon` package directories)

Deduplication: if two `holon.proto` files have the same UUID, the
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
$ op build --clean rob-go           # clean first, then rebuild
$ op build --target macos --mode release
$ op build --dry-run                # print plan, don't execute
```

Flags:

| Flag | Values | Default | Description |
|---|---|---|---|
| `--clean` | | | Run `op clean` before building. For composites, clean recurses through holon members first. Cannot be combined with `--dry-run`. |
| `--target` | `macos`, `linux`, `windows`, `ios`, `ios-simulator`, `tvos`, `tvos-simulator`, `watchos`, `watchos-simulator`, `visionos`, `visionos-simulator`, `android`, `all` | current OS | Platform target |
| `--mode` | `debug`, `release`, `profile` | `debug` | Build mode |
| `--dry-run` | | | Print resolved plan without executing |
| `--no-sign` | | | Skip automatic ad-hoc signing for bundle artifacts |

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

For composite recipe holons, `op clean` is recursive by default: it
cleans all holon members first, then removes the composite's own
`.op/`.

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
  "manifest": "api/v1/holon.proto",
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
1. stdio://   ephemeral subprocess via stdin/stdout pipes
2. tcp://     gRPC over TCP (ephemeral or existing server)
```

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
op gabriel-greeting-go greet '{"name":"World"}'
op gabriel-greeting-c greet '{"name":"World"}'
```

### `op <binary-path> <method> [json]`

Call an executable directly — no discovery, no slug resolution.
The first argument must be an **existing executable file** (contains
`/` or `.`, and has the executable bit set). `op` launches it via
`stdio://` and invokes the RPC:

```bash
op ./.op/build/grace-op.holon/bin/darwin_arm64/gabriel-greeting-go SayHello '{"name":"World"}'
```

If the file does not exist or is not executable, `op` falls through
to slug resolution.

The command is mapped to an RPC method name:

| Command | RPC Method |
|---|---|
| `new` | `CreateIdentity` |
| `list` | `ListIdentities` |
| `show` | `ShowIdentity` |
| *other* | used as-is (e.g. `build` → `Build`, `install` → `Install`) |

### Auto-build

When the target holon has no built binary, `op` builds it before calling.
Build progress displays on stderr with elapsed time:

```text
00:00:01 building gabriel-greeting-swift…
00:00:08 building gabriel-greeting-swift… linking
```

On success, progress is erased and only the call result appears on stdout.
On failure, progress stays and the build error is shown.

Composite holons build dependency members in topological order first. Each
finished dependency freezes its own line with `✓`, then the next dependency
starts on a new line.

Interpreted holons (Go, Node, Python, Ruby, Dart) run from source without
building. Like `go run`, the interpreter is launched directly.

| Runner | Behavior |
|--------|----------|
| `go`, `go-module` | `go run <main>` — no build needed |
| `node`, `typescript` | `node <main>` — no build needed |
| `python` | `python3 <main>` — no build needed |
| `ruby` | `ruby <main>` — no build needed |
| `dart` | `dart run <main>` — no build needed |
| `swift-package`, `gradle`, `cargo`, `cmake`, `dotnet` | Auto-builds via `op build` |

To skip auto-build and fail immediately, use `--no-build`.

### Direct gRPC URI dispatch

For fine-grained transport control, use URI syntax:

```bash
# TCP — connect to existing server (authority contains ':')
op grpc://localhost:9090 Build

# TCP — ephemeral by holon name (authority has no ':')
op grpc://rob-go Build

# Stdio pipe — launch binary, pipe gRPC, done
op stdio://rob-go Build

# Unix domain socket
op unix:///tmp/rob.sock Build

# WebSocket
op ws://localhost:8080 Build
op wss://secure.example.com Build
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
op stdio://gabriel-greeting-go Greet '{"name":"World"}'
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

### `op run <holon> [flags]`

Launch a holon in the foreground. If the artifact is missing, `op run`
builds it first.

```bash
op run rob-go
op run --clean rob-go              # clean, rebuild, then run
op run rob-go:9091                 # TCP shorthand
op run gabriel-greeting-go --listen tcp://:9092
op run gabriel-greeting-c --listen unix:///tmp/greeting.sock
```

Flags:

| Flag | Values | Default | Description |
|---|---|---|---|
| `--clean` | | | Run `op clean` before building and running. Cannot be combined with `--no-build`. |
| `--listen` | any listen URI | `stdio://` | Listen address for service holons |
| `--no-build` | | | Fail if the artifact is missing instead of building |
| `--target` | same as `op build` | current OS | Build target to use if a build is needed |
| `--mode` | `debug`, `release`, `profile` | `debug` | Build mode to use if a build is needed |

When `--clean` is set, `op run` always performs `clean → build → run`.

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
- Otherwise, if `./api/v1/holon.proto` exists, derive the holon path from the
  identity slug (`<given_name>-<family_name>`, lowercased,
  hyphenated).
- Otherwise, fall back to the current directory name.
- If none of these produce a usable value, fail with an actionable
  error.

### `op mod add <module> [version]`

Add a dependency:

```bash
op mod add github.com/organic-programming/gabriel-greeting-go v0.1.0
```

If `version` is omitted, `op` resolves the latest published tag
automatically.

### `op mod remove <module>`

Remove a dependency:

```bash
op mod remove github.com/organic-programming/gabriel-greeting-go
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
| Invalid manifest | `op build: holon.proto: missing required field "identity.uuid"` |
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
op mcp rob-go gabriel-greeting-go     # multiple holons
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

If holons declare `skills` in `holon.proto`, `op mcp` exposes them
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
| `op build [<holon>] [--clean] [--target T] [--mode M] [--dry-run] [--no-sign]` | Build primary artifact |
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
| `op run <holon> [--clean] [--listen <URI>]` | Build if needed, then launch in the foreground |
| `op run <holon>:<port>` | Shorthand for `--listen tcp://:<port>` |
| `op serve [--listen <URI>]` | Start op's own gRPC server |

### Dispatch

| Command | Description |
|---|---|
| `op <holon> <command> [args]` | Transport-chain dispatch |
| `op <holon> --clean <method> [--no-build] [json]` | Clean the slug target, auto-build it, then call the RPC |
| `op <holon> <method> [--no-build] [json]` | Call a holon RPC; auto-build compiled slug targets if needed |
| `op grpc://<host:port> [method]` | gRPC over TCP |
| `op grpc://<slug> <method> [--no-build] [json]` | gRPC auto-connect for slug targets |
| `op stdio://<holon> <method>` | gRPC over stdio pipe |
| `op unix://<path> <method>` | gRPC over Unix socket |
| `op ws://<host:port> <method>` | gRPC over WebSocket |
| `op wss://<host:port> <method>` | gRPC over secure WebSocket |

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
