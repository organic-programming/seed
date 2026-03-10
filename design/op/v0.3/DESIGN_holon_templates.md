# Holon Templates (`op new`)

## Problem

Creating a new holon requires manually setting up directory
structure, `holon.yaml`, proto stubs, build files, and serve
boilerplate — all following conventions from `OP.md`, `PROTO.md`,
and `CONVENTIONS.md`. This is error-prone for humans and slow for
agents.

## Solution

`op new --template <name> <holon-name>` generates a correct,
buildable holon scaffold from a template. One command, zero
convention risk.

---

## CLI

```bash
op new --template go-daemon wisupaa-whisper
op new --template rust-daemon my-service
op new --template composite-go-swift my-app
op new --list                              # list available templates
```

## Template Structure

Templates live inside `grace-op` (shipped with `op`):

```
holons/grace-op/templates/
├── go-daemon/
│   ├── template.yaml
│   ├── holon.yaml.tmpl
│   ├── cmd/main.go.tmpl
│   ├── protos/service.proto.tmpl
│   └── go.mod.tmpl
├── rust-daemon/
│   ├── template.yaml
│   ├── holon.yaml.tmpl
│   ├── src/main.rs.tmpl
│   ├── protos/service.proto.tmpl
│   └── Cargo.toml.tmpl
├── python-daemon/
├── composite-go-swift/
└── ...
```

### `template.yaml` (metadata)

```yaml
name: go-daemon
description: Native Go gRPC daemon holon
lang: go
params:
  - name: name
    description: Holon name (given-family)
    required: true
  - name: service
    description: gRPC service name
    default: "{{ .FamilyName }}Service"
```

### Template variables

| Variable | Source | Example |
|---|---|---|
| `{{ .UUID }}` | Auto-generated UUID v4 | `a1b2c3...` |
| `{{ .Slug }}` | From holon name arg | `wisupaa-whisper` |
| `{{ .GivenName }}` | Parsed from name | `wisupaa` |
| `{{ .FamilyName }}` | Parsed from name | `whisper` |
| `{{ .Service }}` | Derived or param | `WhisperService` |
| `{{ .Module }}` | Go module path | `wisupaa-whisper` |
| `{{ .Date }}` | Current date (ISO 8601) | `2026-03-10` |

## What `op new` Does

1. Copy template directory → `./<holon-name>/`
2. Generate UUID v4
3. Parse `<holon-name>` → given + family name
4. Fill all `.tmpl` files with Go `text/template`
5. Rename `.tmpl` → real extensions
6. Run post-init hooks (e.g., `go mod tidy`)

Result: `op build` works immediately on the generated scaffold.

---

## Template Catalog

### Daemon Templates (native gRPC servers)

| Template | Lang | Runner | Source |
|---|---|---|---|
| `go-daemon` | Go | go-module | TASK01 |
| `rust-daemon` | Rust | cargo | TASK01 |
| `python-daemon` | Python | script | TASK01 |
| `swift-daemon` | Swift | swift-package | TASK01 |
| `kotlin-daemon` | Kotlin | gradle | TASK01 |
| `dart-daemon` | Dart | flutter | TASK01 |
| `csharp-daemon` | C# | dotnet | TASK01 |
| `node-daemon` | Node.js | npm | TASK01 |
| `cpp-daemon` | C++ | cmake | CPP_TASK001 |

### HostUI Templates (frontend clients)

| Template | Tech | Runner | Source |
|---|---|---|---|
| `hostui-swiftui` | SwiftUI | swift-package | TASK02 |
| `hostui-flutter` | Flutter | flutter | TASK02 |
| `hostui-kotlin` | Kotlin Compose | gradle | TASK02 |
| `hostui-web` | HTML/JS | script | TASK02 |
| `hostui-dotnet` | .NET MAUI | dotnet | TASK02 |
| `hostui-qt` | Qt/C++ | qt-cmake | TASK02 |

### Composition Templates (backend-to-backend)

| Template | Pattern | Source |
|---|---|---|
| `composition-direct-call` | A → B | TASK06 |
| `composition-pipeline` | A → B → C | TASK06 |
| `composition-fan-out` | A → {B, C} parallel | TASK06 |

### Composite Templates (assemblies)

| Template | Generates | Source |
|---|---|---|
| `composite-<daemon>-<hostui>` | manifest-only assembly | TASK03 |

Example: `op new --template composite-go-swift my-app` generates
a `holon.yaml` referencing a Go daemon and SwiftUI HostUI.

### Wrapper Templates (external CLI delegation)

| Template | Wraps | Source |
|---|---|---|
| `wrapper-cli` | Any CLI tool | Based on rob-go, jess-npm |

### Toolchain Templates (development tooling)

| Template | Purpose | Source |
|---|---|---|
| `toolchain-lang` | Language toolchain holon | Based on rob-go |

---

## Agent Integration

Templates accelerate agent-driven holon creation:

```
# Without templates: 5-10 tool calls, convention risk
Agent reads OP.md → generates each file manually

# With templates: 1 tool call + customization
Agent runs: op new --template go-daemon wisupaa-whisper
Agent opens main.go → adds actual RPC implementations
```

Templates eliminate the agent's most error-prone work (structure,
imports, serve boilerplate) and let it focus on domain logic.

---

## Spec File for Agents (optional companion)

For custom holons beyond templates, a `holon-spec.yaml` describes
intent for agents to interpret:

```yaml
# holon-spec.yaml — read by agents, not by op
given_name: wisupaa
family_name: whisper
lang: cpp
contract:
  service: WhisperService
  rpcs:
    - name: Transcribe
      input: {audio_path: string, model: string}
      output: {text: string, segments: repeated Segment}
build:
  runner: cmake
  requires: [cmake, grpc, whisper.cpp]
```

The agent reads this + runs `op new --template cpp-daemon` + fills
in the details. The spec file is a **structured prompt**, not code.

---

## Template Maintenance Workflow

Templates are living artifacts. When conventions or APIs change,
templates must stay in sync.

### Triggers

| Event | Action |
|---|---|
| New runner added (e.g. `zig`) | Add daemon template |
| New HostUI tech added | Add hostui template |
| `serve.Run` API changes | Update all daemon `main.*` templates |
| New `holon.yaml` field | Update all `holon.yaml.tmpl` files |
| Proto convention change | Update all `service.proto.tmpl` files |
| New composition pattern (e.g. sidecar) | Add composition template |
| Platform matrix change | Update template `platforms` lists |

### Maintenance Checklist

1. **Edit the template** — modify `.tmpl` files
2. **Regenerate test holon** — `op new --template <name> test-holon`
3. **Verify it builds** — `op build test-holon`
4. **Run testmatrix** — ensure existing recipes still pass
5. **Update catalog** — if adding/removing templates
6. **Update recipe source** — if the change applies to TASK01 recipes too

### Source of Truth

Recipe implementations are the canonical reference. Templates are
scaffolds that produce the same structure:

```
recipes/daemons/greeting-daemon-go/   ← canonical (TASK01)
templates/go-daemon/                  ← generates same structure
```

When both diverge, the **recipe is the source of truth**. Update
the template to match.
