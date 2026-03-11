# TASK03 — Extract DRY Flutter/Dart HostUI (Proof of Concept)

## Summary

Extract the Flutter/Dart HostUI from `go-dart-holons` into a
standalone `recipes/hostui/greeting-hostui-flutter/` holon.
Paired with TASK02 to form the complete Go + Dart proof of concept.

> [!IMPORTANT]
> **Always use the language SDK as much as possible.**
> The Dart HostUI must use `dart-holons` SDK `connect(slug)` — not
> raw gRPC channel creation or hardcoded addresses.

> [!IMPORTANT]
> **Connect approach only.** Every UI assembly must use the SDK
> `connect(slug)` primitive for daemon resolution and lifecycle.
> No raw `GrpcChannel(host, port)` or equivalent.

## Source

| From | To |
|---|---|
| `recipes/go-dart-holons/examples/greeting/greeting-godart/` | `recipes/hostui/greeting-hostui-flutter/` |

## Acceptance Criteria

- [ ] `greeting-hostui-flutter` has its own `holon.yaml` + Dart/Flutter source
- [ ] Uses `recipes/protos/greeting/v1/greeting.proto` (shared)
- [ ] Uses `dart-holons` SDK `connect(slug)` to reach the daemon
- [ ] Builds standalone with `op build`
- [ ] Can connect to any GreetingService daemon (configurable endpoint)
- [ ] Supports macOS, Windows, Linux (no platform regression)
- [ ] Existing `go-dart-holons` is NOT modified (don't clean up yet)

## Dependencies

TASK02 (the Go daemon must exist to test the HostUI against it).
