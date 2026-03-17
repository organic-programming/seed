# PROTO — The Holon Proto Specification v1

Status: draft

Audience:
- holon authors
- `grace-op` implementers
- SDK implementers (Go, Swift, Rust, …)

> *"The .proto file is the single source of truth for a holon's identity,
> contract, documentation, and operational metadata."*

---

## Why This Spec Exists

The proto file is the center of the holon universe:

- It carries the **identity** (who is this holon).
- It carries the **contract** (what RPCs it exposes).
- It carries the **manifest** (how to build, run, require).
- It carries the **guide** (user-facing documentation).
- It carries the **skills and sequences** (how agents use it).

Everything else — `.holon.json`, `Describe` RPC — is
derived from the proto. This spec defines how to author, structure, and
share proto files in the Organic Programming ecosystem.

---

## Relationship to Existing Documents

| Document | Role |
|----------|------|
| `HOLON_PACKAGE.md` | Defines the `.holon` package produced from the proto. |
| `OP.md` | CLI spec. `op` commands consume what the proto declares. |
| `HOLON_BUILD.md` | Build orchestration. Recipe fields originate in the proto manifest. |
| Existing `PROTO.md` | Precursor. Basic conventions. This spec absorbs and extends it. |

---

## 1. Two Proto Layers

Every holon has two proto layers. The separation is fundamental.

### Domain Contract (shared)

The domain contract is owned by one holon and shared by others. It
lives in a shared `_protos/` directory — never copied, never symlinked
(**No Copy, No Symlink** principle).

```
_protos/v1/greeting.proto          # domain: GreetingService + messages
```

- Language-neutral — no `go_package`, no language options.
- No manifest data — just the service, RPCs, and messages.
- Versioned by package name (`greeting.v1`, `greeting.v2`).
- Consumed by every holon implementing or calling this contract.

### Holon Manifest (local)

The holon manifest is owned by a single holon. It imports the domain
contract and layers identity, operational metadata, skills, sequences,
and documentation on top.

```
api/v1/holon.proto                 # local: manifest + language options
```

- Contains `option (holons.v1.manifest) = { ... }`.
- Contains language-specific options (e.g. `option go_package`).
- One per holon — this is the file `op` reads.

### Example — Gabriel Greeting Go

```protobuf
syntax = "proto3";

package greeting.v1;

import "holons/v1/manifest.proto";      // platform manifest extension
import "v1/greeting.proto";             // shared domain contract

option go_package = "...";              // language-specific

option (holons.v1.manifest) = {
  identity: {
    schema: "holon/v1"
    uuid: "3f08b5c3-..."
    given_name: "Gabriel"
    family_name: "Greeting-Go"
    motto: "Greets users in 56 languages."
    composer: "B. ALTER"
    status: "draft"
    born: "2026-02-20"
  }
  // ... build, skills, sequences, guide
};
```

---

## 2. Include Path Convention

Proto imports are resolved via include paths. `op` (and `protoc`) resolve
them using a three-level hierarchy:

| Level | Path | Content |
|-------|------|---------|
| **Platform** | `_protos/` (at repo root or organic-programming root) | System types: `holons/v1/manifest.proto`, `holons/v1/describe.proto` |
| **Domain** | `_protos/` (at example or project level) | Shared contracts: `v1/greeting.proto` |
| **Local** | `.` (holon root) | The holon's own `api/v1/holon.proto` |

### Compilation Command

```bash
protoc \
  --proto_path=. \
  --proto_path=../../_protos \
  --proto_path=../../../_protos \
  api/v1/holon.proto \
  --descriptor_set_out=/dev/null
```

`op` does this internally via `protocompile` — no manual `protoc`
invocation needed for manifest operations. Developers only call `protoc`
for language-specific stub generation.

### No Copy, No Symlink

The domain contract (`.proto` file) is owned by exactly one holon. Other
holons reference it through include paths at code-generation time.
Consumers **must not** copy or symlink proto files into their own source
tree.

Benefits:
- **Zero desync** — changes propagate at next generation step.
- **Git hygiene** — no duplicated contracts.
- **Cross-language consistency** — Go, Swift, Dart all reference the same source.

---

## 3. The `HolonManifest` Schema

The `HolonManifest` message is defined in
`_protos/holons/v1/manifest.proto`. It is registered as a
`google.protobuf.FileOptions` extension at field number `50000`.

```protobuf
extend google.protobuf.FileOptions {
  HolonManifest manifest = 50000;
}
```

Any `.proto` file can carry the full manifest by writing:

```protobuf
option (holons.v1.manifest) = { ... };
```

### Top-Level Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `identity` | `Identity` | yes | Who this holon is. |
| `description` | `string` | yes | What this holon does, plain language. |
| `lang` | `string` | yes | Primary implementation language. |
| `kind` | `string` | yes | `native`, `wrapper`, or `composite`. |
| `transport` | `string` | no | Default transport (`stdio`, `tcp`). |
| `build` | `Build` | yes | Runner, main entry, recipe steps. |
| `requires` | `Requires` | yes | Commands and files that must exist. |
| `artifacts` | `Artifacts` | yes | Binary or primary artifact path. |
| `skills` | `Skill[]` | no | Agent-facing capability descriptions. |
| `sequences` | `Sequence[]` | no | Deterministic RPC batches for `op do`. |
| `contract` | `Contract` | no | Pointer to gRPC service (for cross-ref). |
| `platforms` | `string[]` | no | Supported platforms. |
| `guide` | `string` | no | User-facing documentation in markdown. |

### Surface Symmetry

The manifest declares the holon's public surface, so it must enforce the
same golden rule as the constitution:

> **Code API surface = CLI surface = RPC surface = Test surface**

`contract.rpcs` must exhaustively match the service definition. Every RPC
listed there must exist in the Code API, CLI, RPC server, and tests. The
only CLI affordances outside `contract.rpcs` are `serve` and `help`.
`version` is also not listed there because the SDK derives it from
`identity.version` and surfaces it automatically across CLI, RPC, Code
API, and tests.

### Identity

```protobuf
message Identity {
  string schema = 1;       // "holon/v1"
  string uuid = 2;         // generated once, never changes
  string given_name = 3;   // the character (e.g. "Gabriel")
  string family_name = 4;  // the function (e.g. "Greeting-Go")
  string motto = 5;        // dessein in one sentence
  string composer = 6;     // who designed this holon
  string status = 8;       // draft | stable | deprecated | dead
  string born = 9;         // ISO 8601 date (YYYY-MM-DD)
  string version = 10;     // semver, e.g. "0.4.1" — no "v" prefix
}
```

### Version

`version` is the **single source of truth** for the holon's release version.
It is a [semver](https://semver.org) string stored without a `v` prefix
(e.g. `"0.4.1"`, not `"v0.4.1"`). The `v` is a display convention added
by CLIs; the datum stays clean.

| Facet | How version is surfaced |
|-------|------------------------|
| **CLI** | SDK-provided `version` subcommand — reads from the manifest at startup |
| **RPC** | `Describe` response includes `version` from identity |
| **Code API** | SDK exposes a `Version()` helper — no hand-written constant needed |
| **Tests** | Assert against the SDK helper |

Version lifecycle:

```
0.1.0  →  0.1.1 (patch: bug fix)
       →  0.2.0 (minor: new RPC, backward-compatible)
       →  1.0.0 (major: breaking contract change)
```

A new major version implies a new proto package (`greeting.v2`) — see §6.4.
Major bumps are rare; the contract is designed to be stable.

> **Author directive**: bump `version` in the proto manifest on every
> release. Never maintain a separate version constant in code.

### Lifecycle

```
draft ──► stable ──► deprecated ──► dead
```

| Status | Meaning |
|--------|--------|
| `draft` | The contract is provisional. May change without notice. |
| `stable` | The contract is frozen. Changes require a new version. Only a human can promote to stable. |
| `deprecated` | Superseded. Still functional, not for new compositions. |
| `dead` | No longer functional. Kept for genealogical record. |

### Naming Conventions

**Family name** — describes **what the holon does**. Noun or noun phrase,
capitalized.

- Good: `Transcriber`, `OP`, `Echo`, `Sculptor`
- Bad: `DoTranscription`, `run_ffprobe`, `my-tool`

**Given name** — distinguishes **character**. May express speed,
personality, or origin.

- Examples: `Grace` OP, `Wisupaa` Whisper, `Rob` Go, `Gabriel` Greeting-Go

**Slug** — `<given_name>-<family_name>`, lowercased, hyphenated. This is
the directory name, the binary name, and the universal identifier.

- Examples: `grace-op`, `wisupaa-whisper`, `rob-go`, `gabriel-greeting-go`
- Exception: `op` is the binary name for `grace-op`.

### Build

```protobuf
message Build {
  // Compiled:    go-module | cargo | cmake | swift-package
  // Interpreted: python | node | ruby
  // Transpiled:  typescript
  // Mobile:      dart | flutter
  // Composite:   recipe
  // None:        none
  string runner = 1;
  string main = 2;                 // entry point (go-module: "./cmd")

  // Recipe-mode fields
  Defaults defaults = 3;
  repeated Member members = 4;
  map<string, Target> targets = 5;
}
```

### Artifacts

```protobuf
message Artifacts {
  string binary = 1;                          // for native/wrapper holons
  string primary = 2;                         // for composites (.app path)
  map<string, TargetArtifacts> by_target = 3; // per-platform overrides
}
```

### Skills

```protobuf
message Skill {
  string name = 1;              // kebab-case identifier
  string description = 2;      // what it achieves
  string when = 3;              // trigger condition
  repeated string steps = 4;   // ordered human-readable steps
}
```

Skills describe **when and why** to use a holon. Agents discover them
via `op inspect` or MCP and use them to decide which holon to call.

### Sequences

```protobuf
message Sequence {
  string name = 1;
  string description = 2;
  repeated Param params = 3;
  repeated string steps = 4;     // op commands with Go text/template syntax
}
```

Sequences are **deterministic batches** — no reasoning between steps.
Executed by `op do` or exposed as MCP tools.

### Guide

```protobuf
// Inside HolonManifest:
string guide = 15;               // user-facing documentation, markdown
```

The guide is the holon's **user manual** — authored in markdown, embedded
directly in the proto manifest. It replaces external README files as the
canonical user documentation for a holon package.

Content:

- What the holon does (expanded beyond `description`).
- How to use it — CLI, RPC, and API facets.
- Configuration and prerequisites.
- Examples and common patterns.
- Troubleshooting.

Example:

```protobuf
option (holons.v1.manifest) = {
  identity: { ... }
  guide: "# Gabriel Greeting Go\n\nA multilingual greeting holon.\n\n## Usage\n\n```bash\nop gabriel-greeting-go SayHello '{\"name\":\"Alice\",\"lang_code\":\"fr\"}'\n```\n\n## Supported Languages\n\n56 languages with culturally appropriate default names.\n"
};
```

Consumers:

| Command | What it does with the guide |
|---------|-----------------------------|
| `op man <slug>` | Renders the guide for the terminal |
| `op man <slug> --export-path` | Exports the guide as a `.md` file |
| `Describe` RPC | Returns the guide as part of the response |
| `op mcp` | Exposes the guide as MCP context |

---

## 4. Comments Are Functional

Proto comments are **data**, not decoration. The SDK parses them to build
`HolonMeta.Describe` responses, and `op` reads them for `op inspect`,
`op mcp`, and `op tools`.

Every `service`, `rpc`, `message`, `enum`, and field **must** carry a
leading comment explaining its purpose.

```protobuf
// Echo is a minimal test service for SDK validation.
service Echo {
  // Ping returns the input message with SDK metadata.
  rpc Ping(PingRequest) returns (PingResponse);
}

// PingRequest carries the message to echo back.
message PingRequest {
  // The message to echo.
  string message = 1;
}
```

A contract without comments is incomplete.

---

## 5. Meta-Annotations

Standard comments become `description` fields in `Describe` responses.
The OP ecosystem extends this with **meta-annotations** — special tags
in comments that the SDK and `op` extract:

### `@required`

Marks a field as semantically required. Proto3 has no wire-level required,
but RPCs have semantic requirements.

```protobuf
message SayHelloRequest {
  // Name to greet. If empty, falls back to a localized default.
  // @example "Alice"
  string name = 1;

  // ISO 639-1 code chosen by the UI.
  // @required
  // @example "fr"
  string lang_code = 2;
}
```

The SDK sets `FieldDoc.required = true`. `op tools` marks them as required
in JSON Schema and MCP tool definitions. LLMs know they must provide them.

### `@example`

Provides a concrete example value.

**On an RPC** — a complete JSON request example:

```protobuf
// Greets the user in the chosen language.
// @example {"name":"Alice","lang_code":"fr"}
rpc SayHello(SayHelloRequest) returns (SayHelloResponse);
```

**On a field** — a single value example:

```protobuf
// ISO 639-1 code chosen by the UI.
// @required
// @example "fr"
string lang_code = 2;
```

### `@skill`

Groups related RPCs under a human-readable label for `op inspect`:

```protobuf
// @skill Identity Management
// CreateIdentity creates a new holon identity.
rpc CreateIdentity(CreateIdentityRequest) returns (CreateIdentityResponse);

// @skill Identity Management
// ListIdentities lists all known holon identities.
rpc ListIdentities(ListIdentitiesRequest) returns (ListIdentitiesResponse);
```

---

## 6. Design Rules

### One service per proto file

Keep services focused. Multiple concerns get separate services in
separate files.

### Every public function is an `rpc`

If it's not in the proto, it's not public. Even functions called locally
are declared as `rpc` methods.

### Dedicated request/response pairs

Every RPC uses its own request and response messages — no reuse.

```protobuf
// Good
rpc SayHello(SayHelloRequest) returns (SayHelloResponse);
rpc ListLanguages(ListLanguagesRequest) returns (ListLanguagesResponse);

// Bad — reusing Empty across methods
rpc SayHello(google.protobuf.Empty) returns (SayHelloResponse);
```

### Naming

Follow the Protocol Buffers [style guide](https://protobuf.dev/programming-guides/style/):

| Element | Style | Example |
|---------|-------|---------|
| Services | `PascalCase` | `GreetingService` |
| RPCs | `PascalCase` | `SayHello` |
| Messages | `PascalCase` | `SayHelloRequest` |
| Fields | `snake_case` | `lang_code` |
| Enums | `SCREAMING_SNAKE_CASE` | `CLADE_UNSPECIFIED` |

### Versioning

Version is in the **package name**, not the filename:

```protobuf
package greeting.v1;    // v1 contract
package greeting.v2;    // breaking change → new version
```

---

## 7. Generated Code

Generated code lives in `gen/` and is **committed** — consumers should
be able to build without `protoc`.

```
gen/
  go/greeting/v1/          # generated Go stubs
  swift/greeting/v1/       # generated Swift stubs
  dart/greeting/v1/        # generated Dart stubs
```

Never edit generated code. If the output needs changes, modify the `.proto`
and regenerate.

---

## 8. Proto → Package Pipeline

The proto is the input. Everything else is output.

```
holon.proto (human-authored)
     │
     ├── protocompile ──────→ .holon.json      (derived JSON cache for fast discovery)
     ├── protoc + plugins ──→ gen/             (language-specific stubs)
     └── SDK embed ─────────→ Describe RPC     (runtime metadata inside the binary)
```

`op` performs the first step internally using `protocompile` (pure Go).
The developer performs stub generation using `protoc` or `buf`.

---

## 9. Complete Holon Proto Example

```protobuf
syntax = "proto3";

package greeting.v1;

import "holons/v1/manifest.proto";
import "v1/greeting.proto";

option go_package = "github.com/organic-programming/seed/.../greetingv1";

option (holons.v1.manifest) = {
  identity: {
    schema: "holon/v1"
    uuid: "3f08b5c3-8931-46d0-847a-a64d8b9ba57e"
    given_name: "Gabriel"
    family_name: "Greeting-Go"
    motto: "Greets users in 56 languages — a Go daemon recipe example."
    composer: "B. ALTER"
    status: "draft"
    born: "2026-02-20"
    version: "0.4.1"
  }
  description: "A Go gRPC daemon that greets users in 56 languages."
  lang: "go"
  kind: "native"
  build: {
    runner: "go-module"
    main: "./cmd"
  }
  requires: {
    commands: ["go"]
    files: ["go.mod"]
  }
  artifacts: {
    binary: "gabriel-greeting-go"
  }
  skills: [{
    name: "multilingual-greeter"
    description: "Greet a person by name in any of the 56 supported languages."
    when: "The user wants to greet someone in a specific language."
    steps: [
      "Call ListLanguages to show available languages",
      "Ask the user for a name and language code",
      "Call SayHello with the chosen name and lang_code"
    ]
  }]
  sequences: [{
    name: "greeting-fr-ja-ru-en"
    description: "Greet in French, Japanese, Russian, and English."
    params: [{name: "name", description: "Person to greet", required: true}]
    steps: [
      "op gabriel-greeting-go ListLanguages",
      "op gabriel-greeting-go SayHello '{\"name\":\"{{ .name }}\",\"lang_code\":\"fr\"}'",
      "op gabriel-greeting-go SayHello '{\"name\":\"{{ .name }}\",\"lang_code\":\"ja\"}'",
      "op gabriel-greeting-go SayHello '{\"name\":\"{{ .name }}\",\"lang_code\":\"ru\"}'",
      "op gabriel-greeting-go SayHello '{\"name\":\"{{ .name }}\",\"lang_code\":\"en\"}'",
    ]
  }]
  guide: "# Gabriel Greeting Go\n\nA multilingual greeting service.\n\n## Quick Start\n\n```bash\nop gabriel-greeting-go SayHello '{\"name\":\"Alice\",\"lang_code\":\"fr\"}'\n```\n\n## 56 Languages\n\nGabriel covers 56 languages with localized templates and culturally\nappropriate default names (Marie in French, マリア in Japanese).\n"
};
```

This single file is the complete truth. From it, `op` derives:

- **`.holon.json`** — fast JSON cache for discovery
- **`op man` output** — user guide rendered from the `guide` field
- **MCP tools** — JSON Schema from messages and meta-annotations
- **`Describe` response** — embedded in the binary at build time
