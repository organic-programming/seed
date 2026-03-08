# Proto Design Guide

> *"The .proto file is the single source of truth for a holon's public surface."*
> — Constitution, Article 2

This document explains how to write `.proto` files in the Organic
Programming ecosystem. For directory layout see
[CONVENTIONS.md §1](./CONVENTIONS.md). For the wire protocol see
[PROTOCOL.md](./PROTOCOL.md). For the holon manifest see
[HOLON_YAML.md](./HOLON_YAML.md).

---

## 1. File Structure

```
protos/
└── <package>/
    └── v1/
        └── <service>.proto
```

One `.proto` file per service. The package follows `<name>.v1`
(or `v2`, `v3` for breaking changes). Versioning is in the package
name, not in the filename.

```protobuf
syntax = "proto3";
package echo.v1;

service Echo {
  rpc Ping(PingRequest) returns (PingResponse);
}
```

---

## 2. Comments Are Functional

Proto comments are not decorative — they are **data**. The SDK parses
them at startup to build `HolonMeta.Describe` responses, and `op`
reads them for `op inspect`, `op mcp`, and `op tools`.

Every `service`, `rpc`, `message`, `enum`, and field **must** carry
a leading comment explaining its purpose (Article 2).

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

## 3. Meta-Annotations

Standard proto comments become `description` fields in `Describe`
responses. Organic Programming extends this with **meta-annotations**
— special tags in comments that the SDK and `op` extract:

### `@required`

Marks a field as semantically required. Proto3 has no wire-level
required, but RPCs have semantic requirements.

```protobuf
message CreateIdentityRequest {
  // @required
  string given_name = 1;
  // @required
  string family_name = 2;
  string lang = 7;           // optional — no @required
}
```

The SDK sets `FieldDoc.required = true` for these fields.
`op tools` marks them as required in JSON Schema and MCP tool
definitions. LLMs reading tool definitions know they must provide
these fields.

### `@example`

Provides a concrete example value. Appears in two contexts:

**On an RPC** — a complete JSON request example:

```protobuf
// Invoke dispatches a command to a holon by name.
// @example {"holon":"rob-go","args":["build","./cmd/rob-go"]}
rpc Invoke (InvokeRequest) returns (InvokeResponse);
```

**On a field** — a single value example:

```protobuf
// Holon name or alias to dispatch to.
// @required
// @example "rob-go"
string holon = 1;
```

The SDK populates `MethodDoc.example_input` and `FieldDoc.example`
from these tags.

### `@skill`

Documents a higher-level capability that spans multiple RPCs.
Used by `op inspect` to group related methods under a human-readable
label.

```protobuf
// @skill Identity Management
// CreateIdentity creates a new holon identity.
rpc CreateIdentity (CreateIdentityRequest) returns (CreateIdentityResponse);

// @skill Identity Management
// ListIdentities lists all known holon identities.
rpc ListIdentities (ListIdentitiesRequest) returns (ListIdentitiesResponse);
```

---

## 4. Design Rules

### One service per proto file

Keep services focused. If your holon exposes multiple concerns,
use separate services in separate files:

```
protos/
└── myholon/v1/
    ├── transcribe.proto    ← TranscriptionService
    └── status.proto        ← StatusService
```

### Every public function is an `rpc`

If it's not in the proto, it's not public. Even functions that may
be called locally are declared as `rpc` methods (Article 2).

### Request/Response pairs

Every RPC uses dedicated request and response messages, even if
they're empty. Avoid reusing message types across RPCs — each
method gets its own pair.

```protobuf
// Good
rpc Ping(PingRequest) returns (PingResponse);
rpc Status(StatusRequest) returns (StatusResponse);

// Bad — reusing Empty across methods
rpc Ping(Empty) returns (PingResponse);
rpc Status(Empty) returns (StatusResponse);
```

### Naming

Follow the Protocol Buffers
[style guide](https://protobuf.dev/programming-guides/style/):

- Services: `PascalCase` (`Echo`, `TranscriptionService`)
- RPCs: `PascalCase` (`Ping`, `GetTime`)
- Messages: `PascalCase` (`PingRequest`, `PingResponse`)
- Fields: `snake_case` (`given_name`, `family_name`)
- Enums: `SCREAMING_SNAKE_CASE` (`CLADE_UNSPECIFIED`)

---

## 5. HolonMeta — Self-Documentation

Every holon auto-exposes a `HolonMeta.Describe` RPC via the SDK's
`serve` runner. The holon developer does not implement this — the
SDK parses the holon's `.proto` files and populates the response.

The canonical proto lives in:

```
sdk/go-holons/protos/holonmeta/v1/holonmeta.proto
```

Other SDKs generate language-specific stubs from this canonical
source — they do not maintain their own copies (see
[TASK005](./sdk/tasks/TASK005.md)).

For the full `HolonMeta` schema and behavior, see
[PROTOCOL.md §3.5](./PROTOCOL.md).

---

## 6. Generated Code

Generated code lives in `gen/` and is **committed** — consumers
should be able to build without `protoc` (CONVENTIONS.md §5).

Each holon should document its `protoc` invocation in a `Makefile`
or script. The `gen/` directory mirrors the proto package structure.

Never edit generated code. If the output needs changes, modify the
`.proto` and regenerate.

---

## Cross-References

| Topic | Document |
|-------|----------|
| Directory layout (`protos/`, `gen/`) | [CONVENTIONS.md §1, §5](./CONVENTIONS.md) |
| Contract as primary facet | [AGENT.md Article 2](./AGENT.md) |
| HolonMeta schema and behavior | [PROTOCOL.md §3.5](./PROTOCOL.md) |
| Manifest contract section | [HOLON_YAML.md](./HOLON_YAML.md) |
| Proto canonical location | [TASK005](./sdk/tasks/TASK005.md) |
