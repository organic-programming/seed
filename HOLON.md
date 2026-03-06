---
# Cartouche v1
title: "holon.yaml — Unified Holon Specification"
author:
  name: "B. ALTER & Claude"
  copyright: "© 2026 Benoit Pereira da Silva"
created: 2026-03-06
revised: 2026-03-06
lang: en-US
origin_lang: en-US
translation_of: null
translator: null
access:
  humans: true
  agents: true
status: review
---

# holon.yaml — Unified Holon Specification

Every holon carries a single `holon.yaml` at its root. This file is
the complete declaration of a holon — identity, description, contract,
and operational metadata in one place.

---

## Scope

`holon.yaml` answers three questions:

1. **Who is this holon?** — identity, lineage, motto
2. **What does it do?** — description, contract
3. **How does `op` build/test/clean it?** — kind, runner, requires, artifacts

Out of scope: runtime transport, `serve --listen`, SDK internals,
protocol semantics, shell recipes, dependency graphs, install hooks.

---

## Schema

```yaml
# ── Identity ──────────────────────────────────────────
schema: holon/v0
uuid: "b00932e5-49d4-4724-ab4b-e2fc9e22e108"
given_name: Sophia
family_name: "Who?"
motto: "Know thyself."
composer: "B. ALTER"
clade: deterministic/pure
status: draft
born: "2026-02-12"

# ── Lineage ───────────────────────────────────────────
parents: []
reproduction: manual
generated_by: manual

# ── Description ───────────────────────────────────────
description: |
  The first holon — the primordial identity-maker.
  A Go CLI that guides a composer through holon identity creation.

# ── Contract ──────────────────────────────────────────
contract:
  proto: protos/sophia_who/v1/sophia_who.proto
  service: SophiaWhoService
  rpcs: [CreateIdentity, ShowIdentity, ListIdentities]

# ── Operational ───────────────────────────────────────
kind: native
build:
  runner: go-module
  main: ./cmd/who
requires:
  commands: [go]
  files: [go.mod]
artifacts:
  binary: sophia-who
```

---

## Fields

### Identity

| Field | Type | Required | Description |
|---|---|---|---|
| `schema` | string | yes | Always `holon/v0`. |
| `uuid` | UUID v4 | yes | Unique identifier. Generated once at birth. Never changes. |
| `given_name` | string | yes | The character — what distinguishes this holon from others of the same family. |
| `family_name` | string | yes | The function — what the holon does. Like trade surnames: `Transcriber`, `OP`, `Who?`. |
| `motto` | string | yes | The *dessein* in one sentence. |
| `composer` | string | yes | Who made the design decisions — the human or agent who designed the holon. Author of intent, not the tool. |
| `clade` | enum | yes | Computational nature (see [Clade](#clade)). |
| `status` | enum | yes | Lifecycle stage (see [Lifecycle](#lifecycle)). |
| `born` | date | yes | ISO 8601 date of creation. |

### Lineage

| Field | Type | Required | Description |
|---|---|---|---|
| `parents` | list of UUID | yes | Holons from which this one descends. Empty for primordial holons. |
| `reproduction` | enum | yes | How this holon was created (see [Reproduction](#reproduction)). |
| `generated_by` | string | yes | What created this file: `sophia-who`, `manual`, `codex`, etc. |

### Description

| Field | Type | Required | Description |
|---|---|---|---|
| `description` | multiline string | yes | What this holon does, in plain language. YAML `|` block scalar. |

### Contract

| Field | Type | Required | Description |
|---|---|---|---|
| `contract.proto` | string | no | Path to the `.proto` file, relative to holon root. |
| `contract.service` | string | no | gRPC service name. |
| `contract.rpcs` | list of string | no | RPC method names. |

Omit `contract` entirely if the proto is not yet defined.

### Operational

| Field | Type | Required | Description |
|---|---|---|---|
| `kind` | enum | yes | `native` or `wrapper`. Semantic — not inferred (see [Kind](#kind)). |
| `platforms` | list of string | no | Supported operating systems. Omit if cross-platform. |
| `build.runner` | string | yes | Selects the `op` Go runner (see [Runners](#v0-runners)). |
| `build.main` | string | no | Go package path. Only for `go-module` when `./cmd/<holon-dir>` does not apply. |
| `requires.commands` | list of string | yes | CLI tools that must exist on `PATH` before build. |
| `requires.files` | list of string | yes | Files that must exist relative to `holon.yaml`. |
| `delegates.commands` | list of string | no | Wrapper-only. External commands the holon delegates to. |
| `artifacts.binary` | string | yes | Path to the primary binary, relative to `holon.yaml`. |

All paths are relative to the directory containing `holon.yaml`.

---

## Kind

`kind` is a semantic axis describing the holon's relationship to its
domain capability:

- **`native`** — the holon implements the capability from source.
  Example: `wisupaa-whisper` embeds whisper.cpp.
- **`wrapper`** — the holon delegates to an external CLI.
  Example: `rob-go` wraps the `go` command.

`kind` cannot be inferred from language, build runner, or SDK.
A Go wrapper and a Go native holon may share the same runner
but differ fundamentally in kind.

Wrappers must declare `delegates.commands`.

---

## Clade

The clade classifies the holon by its **computational nature**, not by
its domain.

```
deterministic/pure          — same input → same output, no state, no side effects
deterministic/stateful      — same input + same state → same output
deterministic/io_bound      — deterministic logic, external dependencies

probabilistic/generative    — output sampled from a distribution (LLM, diffusion)
probabilistic/perceptual    — approximation of ground truth (ASR, OCR, detection)
probabilistic/adaptive      — behavior changes over time (RL, online learning)
```

**Why it matters:**

- **Composition safety.** A pipeline of deterministic holons is
  deterministic. One probabilistic holon makes the chain probabilistic.
- **Testing strategy.** Deterministic: `assert(output == expected)`.
  Probabilistic: `assert(score(output) > threshold)`.
- **Auditability.** Regulated domains require knowing which holons
  introduce uncertainty.

---

## Lifecycle

```
draft ──► stable ──► deprecated ──► dead
```

| Status | Meaning |
|---|---|
| `draft` | The contract is provisional. May change without notice. |
| `stable` | The contract is frozen. Changes require a new version. Only a human can promote to stable. |
| `deprecated` | Superseded. Still functional, not for new compositions. |
| `dead` | No longer functional. Kept for genealogical record. |

---

## Reproduction

| Mode | Description | Parents |
|---|---|---|
| `manual` | Human writes contract and implementation from scratch. | 0 |
| `assisted` | Human and agent co-design the contract. | 0 |
| `automatic` | The holonizer introspects a binary and generates everything. | 0 (adoption) |
| `autopoietic` | An agent discovers and holonizes a tool autonomously. | 0 (adoption) |
| `bred` | New holon created by recombining traits from N parents. | N ≥ 2 |

Bred holons record parent UUIDs in the `parents` field. Lineage
is not metaphorical — it is tracked.

---

## Naming Conventions

### Family name

Describes **what the holon does**. Noun or noun phrase, capitalized.

Good: `Transcriber`, `OP`, `Who?`, `Sculptor`
Bad: `DoTranscription`, `run_ffprobe`, `my-tool`

### Given name

Distinguishes **character**. May express speed, personality, or origin.

Examples: `Sophia` Who?, `Grace` OP, `Wisupaa` Whisper

### Slug

The holon slug is `<given_name>-<family_name>`, lowercased, hyphenated.
This is the directory name and the binary name.

Examples: `sophia-who`, `grace-op`, `wisupaa-whisper`, `rob-go`

Exception: the `op` binary keeps `op` as its primary name.

---

## Binary Naming

The primary binary uses the holon slug: `sophia-who`, `rob-go`,
`wisupaa-whisper`.

The single exception is `op`, the root entrypoint.

---

## v0 Runners

### `go-module`

- Build: `go build -o .op/build/bin/<artifacts.binary> <build.main or ./cmd/<dir>>`
- Test: `go test ./...`
- Clean: remove `.op/`

### `cmake`

- Configure: `cmake -S . -B .op/build/cmake -DCMAKE_RUNTIME_OUTPUT_DIRECTORY=.op/build/bin`
- Build: `cmake --build .op/build/cmake`
- Test: `ctest --test-dir .op/build/cmake --output-on-failure`
- Clean: remove `.op/`

`cmake` holons must register tests with CTest. If no tests are
registered, `op test` fails with an actionable error.

For holons using a runner not yet implemented in `op`, use the intended
runner name and add a YAML comment: `# runner not yet implemented in op`.

---

## `op check`

`op check` validates the manifest and preflights the build contract
without compiling.

It verifies:

- schema validity
- platform support
- required files
- required commands
- delegated commands for wrappers
- runner-specific entrypoint expectations

---

## Design Constraints

- Go is the scripting language, not shell.
- Runners own the build directory under `.op/`.
- `serve` stays Article 11: `op` launches `<binary> serve --listen <uri>`.
- If a field does not help `op` check, build, test, clean, or locate
  the primary binary, it does not belong in `holon.yaml`.
- `holon.yaml` is the single source of truth for each holon.
