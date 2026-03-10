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

Built-in templates sourced from TASK08 recipe daemons:

| Template | Lang | Runner | Notes |
|---|---|---|---|
| `go-daemon` | Go | go-module | |
| `rust-daemon` | Rust | cargo | |
| `python-daemon` | Python | script | |
| `swift-daemon` | Swift | swift-package | |
| `kotlin-daemon` | Kotlin | gradle | |
| `dart-daemon` | Dart | flutter | |
| `csharp-daemon` | C# | dotnet | |
| `node-daemon` | Node.js | npm | |
| `cpp-daemon` | C++ | cmake | |
| `composite-*` | — | recipe | Assembly templates |

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
