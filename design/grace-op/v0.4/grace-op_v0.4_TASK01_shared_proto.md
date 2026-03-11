# TASK01 — Shared Greeting Proto

## Summary

Create the single canonical `greeting.proto` in a shared location.
Each daemon and HostUI then has a thin local `.proto` that imports
the shared definition via `import public` and adds holon-specific
meta-annotations (`@required`, `@example`, `@skill` — see
[PROTO.md](../../PROTO.md)).

> [!IMPORTANT]
> **One canonical service definition — per-holon annotation wrappers.**
> The shared proto defines the `GreetingService` and its messages.
> Each holon's local `.proto` re-exports it via `import public` and
> adds its own comments and meta-annotations. This follows PROTO.md
> §2–§3: comments are functional data, not decoration.

## Layout

```
recipes/protos/greeting/v1/
└── greeting.proto              ← canonical service + messages

recipes/daemons/gudule-daemon-greeting-go/protos/greeting/v1/
└── greeting.proto              ← import public + Go-specific annotations

recipes/hostui/gudule-greeting-hostui-flutter/protos/greeting/v1/
└── greeting.proto              ← import public + Dart-specific annotations
```

## Per-Holon Wrapper Pattern

```protobuf
// gudule-daemon-greeting-go/protos/greeting/v1/greeting.proto
syntax = "proto3";
package greeting.v1;

// Re-export the canonical service definition.
import public "greeting/v1/greeting.proto";

// Go daemon-specific annotations can be added here as needed.
```

Each holon's `protoc` invocation uses `-I ../../protos` (or
equivalent) to resolve the shared import. The wrapper allows each
holon to carry its own meta-annotations while the service definition
stays DRY.

## Acceptance Criteria

- [ ] Canonical `greeting.proto` extracted from current `go-dart-holons`
- [ ] Placed in `recipes/protos/greeting/v1/`
- [ ] Proto package: `greeting.v1`
- [ ] Service: `GreetingService` with `SayHello` RPC
- [ ] Comments and meta-annotations follow [PROTO.md](../../PROTO.md)
- [ ] Wrapper pattern documented and one example wrapper created

## Dependencies

None (first task in the chain).
