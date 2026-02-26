---
# Cartouche v1
title: "Identity — Holon Civil Status Specification"
author:
  name: "B. ALTER & Claude"
  copyright: "© 2026 Benoit Pereira da Silva"
created: 2026-02-12
revised: 2026-02-12
lang: en-US
origin_lang: en-US
translation_of: null
translator: null
access:
  humans: true
  agents: true
status: review
---

> [!WARNING]
> **This document is under active review.** The naming conventions (family name, given name, full name format) are still being evaluated and may change significantly.

# Identity — Holon Civil Status Specification

Every holon is born with an identity. This document specifies the
**civil status** that every holon must carry throughout its lifecycle.

The identity answers a single question: **Who is this holon?**

---

## Rationale

In an ecosystem of many holons, a filename is not enough. A UUID alone
is inhuman. We need both — a unique identifier for machines, and a
name for humans. We also need lineage (who are the parents?), intent
(what is the motto?), and constraints (which binary version? which OS?).

The civil status is recorded in a `HOLON.md` file at the root of each
holon directory. It is generated at birth — by
[Sophia Who?](#sophia-who), by an agent, or manually — and maintained
throughout the holon's life.

---

## Fields

### Required fields

| Field | Type | Description |
|---|---|---|
| `uuid` | UUID v4 | Unique identifier. Generated once at birth. Never changes. |
| `family_name` | string | The function — what the holon does. Like trade surnames: `Transcriber`, `Prober`, `Sorter`. |
| `given_name` | string | The character — what distinguishes this holon from others of the same family. Chosen by the Composer or proposed by the agent. |
| `motto` | string | The *dessein* in one sentence. What is this holon's purpose? |
| `composer` | string | Who made the design decisions — the human or agent who used Sophia Who? or the holonizer. The godparent. This is the author of intent, not the tool. |
| `clade` | enum | The computational nature (see [Clade](#clade)). |
| `status` | enum | Lifecycle status (see [Lifecycle](#lifecycle)). |
| `born` | date | ISO 8601 date of creation. |

### Lineage fields

| Field | Type | Description |
|---|---|---|
| `parents` | list of UUID | Holons from which this one descends. Empty for primordial holons. |
| `reproduction` | enum | How this holon was created: `manual`, `assisted`, `automatic`, `autopoietic`, `bred`. |

### Optional fields

| Field | Type | Description |
|---|---|---|
| `aliases` | list of string | Human-friendly names and shortcuts. A holon can be known by many names — its UUID is the truth, aliases are convenience. Like DNS for IP addresses. |

### Metadata fields

| Field | Type | Description |
|---|---|---|
| `generated_by` | string | What created this holon: `holonizer-prompt`, `sophia-who`, `manual`, etc. |
| `lang` | string | Implementation language: `go`, `python`, `rust`, etc. |
| `proto_status` | enum | Contract maturity: `draft`, `stable`. Only a human can promote to `stable`. |

---

## Clade

The clade classifies the holon by its **computational nature**, not by
its domain. This is the fundamental split.

### Values

```
deterministic/pure          — same input → same output, no state, no side effects
deterministic/stateful      — same input + same state → same output
deterministic/io_bound      — deterministic logic, external dependencies

probabilistic/generative    — output sampled from a distribution (LLM, diffusion)
probabilistic/perceptual    — approximation of ground truth (ASR, OCR, detection)
probabilistic/adaptive      — behavior changes over time (RL, online learning)
```

### Why it matters

- **Composition safety.** A pipeline of deterministic holons is
  deterministic. One probabilistic holon makes the chain probabilistic.
- **Testing strategy.** Deterministic: `assert(output == expected)`.
  Probabilistic: `assert(score(output) > threshold)`.
- **Auditability.** Regulated domains require knowing which holons
  introduce uncertainty.

---

## Lifecycle

A holon moves through four lifecycle stages:

```
draft ──► stable ──► deprecated ──► dead
```

| Status | Meaning |
|---|---|
| `draft` | The contract is provisional. May change without notice. |
| `stable` | The contract is frozen. Changes require a new version. Only a human can promote to stable. |
| `deprecated` | Superseded by another holon. Still functional, but should not be used in new compositions. |
| `dead` | No longer functional. Kept for genealogical record only. |

---

## Naming Conventions

### Family name

The family name describes **what the holon does**. It is a noun or
noun phrase, capitalized.

Good: `Transcriber`, `Prober`, `Sorter`, `Segmenter`, `Aligner`
Bad: `DoTranscription`, `run_ffprobe`, `my-tool`

### Given name

The given name distinguishes **character**. It may express speed,
precision, reliability, or personality.

Examples:
- `Swift` Transcriber — optimized for speed
- `Deep` Transcriber — optimized for accuracy
- `Sophia` Who? — wisdom, the primordial identity-maker

The given name is optional for simple holons. It becomes essential when
multiple holons share the same family name.

### Full name format

```
<given_name> <family_name>
```

Example: `Sophia Who?`, `Swift Transcriber`, `Deep Prober`

---

## Reproduction

Holons can be created through several modes:

| Mode | Description | Parents |
|---|---|---|
| `manual` | Human writes the contract and implementation from scratch. | 0 |
| `assisted` | Human and agent co-design the contract. | 0 |
| `automatic` | The holonizer introspects a binary and generates everything. | 0 (adoption) |
| `autopoietic` | An agent discovers and holonizes a tool autonomously. | 0 (adoption) |
| `bred` | A new holon is created by recombining traits from N parent holons. | N ≥ 2 |

### Bred holons (polygamous reproduction)

When a holon is bred, it inherits patterns from multiple parents:
- Contract structures (message shapes, field patterns)
- Error handling strategies
- Streaming patterns
- Testing approaches

The parent UUIDs are recorded in the `parents` field. This creates a
traceable genealogy — the phylogeny of holons is not metaphorical, it
is recorded.

---

## HOLON.md Template

Every holon directory contains a `HOLON.md` file with the following
YAML frontmatter:

```yaml
---
# Holon Identity v1
uuid: "xxxxxxxx-xxxx-4xxx-xxxx-xxxxxxxxxxxx"
given_name: "<given name>"
family_name: "<family name>"
motto: "<one sentence — the dessein>"
composer: "<who made the decisions — human name or agent id>"
clade: "<clade/sub-clade>"
status: draft
born: "YYYY-MM-DD"

# Lineage
parents: []
reproduction: "<manual|assisted|automatic|autopoietic|bred>"

# Optional
aliases: []

# Metadata
generated_by: "<holonizer-prompt|sophia-who|manual>"
lang: "<go|python|rust|typescript>"
proto_status: draft
---

# <Given Name> <Family Name>

> *"<motto>"*

## Description

<One paragraph describing what this holon does, how it wraps the
underlying tool, and any important behavioral notes.>

## Contract

- Proto file: `<file>.proto`
- Service: `<ServiceName>`
- RPCs: <list>

## Introspection Notes

<Any contradictions, ambiguities, or assumptions made during
introspection. Critical for the human reviewer.>
```

---

## Sophia Who?

The first holon. Sophia Who? is a Go CLI that interactively creates
identity cards for other holons. She is the midwife of the ecosystem.

```
who new         — create a new holon identity (interactive)
who show <uuid> — display a holon's identity
who list        — list all known holons
who pin <uuid>  — capture version/commit/arch for a holon's binary
```

Sophia's own identity:

```yaml
uuid: "00000000-0000-4000-0000-000000000001"
given_name: "Sophia"
family_name: "Who?"
motto: "Know thyself."
composer: "B. ALTER"
clade: "deterministic/pure"
status: draft
born: "2026-02-12"
parents: []
reproduction: "manual"
generated_by: "manual"
lang: "go"
proto_status: draft
```

She is the primordial holon — she has no parents. Every other holon's
identity passes through her.
