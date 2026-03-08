# TASK003 — Add `@required`/`@example` Tags and `skills` Across All Holons

## Context

Depends on: TASK001 (`op inspect` which displays tags and skills).

The spec now supports two conventions that holons should adopt:

1. **Proto comment tags** — `@required` and `@example` in `.proto`
   file comments, parsed by `op inspect` and `Describe`.
2. **Skills** — composed workflows in `holon.yaml`, displayed by
   `op inspect` and exposed as MCP prompts by `op mcp`.

No holons currently use either. This task adds them across the ecosystem.

## Scope

### Tier 1 — Core holons (proto tags + skills)

These holons have real proto contracts and meaningful workflows.
Add both `@required`/`@example` tags to their protos AND `skills`
to their manifests.

| Holon | Proto | Skills to define |
|-------|-------|-----------------|
| `grace-op` | `protos/op/v1/op.proto` | discover-and-build, full-lifecycle |
| `rob-go` | `protos/go/v1/go.proto` | prepare-release, lint-and-fix |
| `jess-npm` | `protos/npm/v1/npm.proto` | setup-project, audit-and-fix |

### Tier 2 — VideoSteno holons

| Holon | Status | Has proto | Has source | Action |
|-------|--------|-----------|------------|--------|
| `wisupaa-whisper` | **implemented** | ✅ `protos/whisper/v1/` | ✅ C++ (CMake) | tags + skills |
| `vs-cli` | **partial** | ❌ | ✅ `main.go` | skills only (when proto exists) |
| `vs-aligner` | **placeholder** | ✅ `aligner.proto` | ❌ | tags only |
| `vs-dte` | **placeholder** | ✅ `dte.proto` | ❌ | tags only |
| `vs-hub` | **placeholder** | ✅ `hub.proto` | ❌ | tags only |
| `vs-ingest` | **placeholder** | ✅ `ingest.proto` | ❌ | tags only |
| `vs-packager` | **placeholder** | ✅ `packager.proto` | ❌ | tags only |
| `vs-repository` | **placeholder** | ✅ `repository.proto` | ❌ | tags only |
| `vs-revision` | **placeholder** | ✅ `revision.proto` | ❌ | tags only |
| `vs-transcriber` | **placeholder** | ✅ `transcriber.proto` | ❌ | tags only |
| `vs-app` | **placeholder** | ❌ | ❌ | skip (manifest only) |

> [!NOTE]
> Placeholder holons (proto but no source) get `@required`/`@example`
> tags in their proto files now — this documents the API intent before
> implementation. Skills are deferred until the holon has working RPCs.

### Tier 3 — B-ALTER holons

| Holon | Status | Has proto | Has source | Action |
|-------|--------|-----------|------------|--------|
| `constantin-sculptor-godart` | **implemented** | ✅ `protos/godart/v1/` | ✅ Dart | tags + skills |
| `abel-fishel-translator` | **placeholder** | ✅ `protos/` (dir only) | ❌ (go.mod stub) | tags only |
| `constantin-sculptor` | **placeholder** | ✅ `protos/sculptor/v1/` | ❌ (go.mod stub) | tags only |

### Tier 4 — Hello-world examples (proto tags only)

These are minimal examples — add `@required`/`@example` tags to
their proto files as reference patterns. No skills needed (they
have a single RPC).

All 13 hello-world examples under `organic-programming/examples/`.

### Tier 5 — Recipe daemons (proto tags only)

All recipe greeting daemons share the same `greeting/v1/greeting.proto`
pattern. Add `@required`/`@example` tags to one, propagate to all 12.

## How to add proto tags

In each `.proto` file, add `@required` and `@example` tags to
field and RPC comments:

```protobuf
// Transcribe processes an audio file and returns timed text.
// @example {"file": "interview.wav", "language": "fr"}
rpc Transcribe(TranscribeRequest) returns (TranscribeResponse);

message TranscribeRequest {
  // Path to the audio file.
  // @required
  // @example "interview.wav"
  string file = 1;

  // BCP-47 language code.
  // @example "fr"
  string language = 2;
}
```

## How to add skills

In each `holon.yaml`, add a `skills` section:

```yaml
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

## Execution order

1. **Tier 1** first — core holons serve as the reference.
2. **Tier 4** next — hello-world examples show the simplest pattern.
3. **Tier 5** — recipe daemons (one change, propagate to all).
4. **Tier 2** — VideoSteno holons.
5. **Tier 3** — B-ALTER holons.

## Rules

- Study each holon's proto file to write meaningful `@required`,
  `@example`, and `skills`. Do not use placeholders.
- Skills must match the holon's actual capabilities (its RPCs).
- One commit per tier.
- Run `op list` after each tier to verify manifests still parse.
