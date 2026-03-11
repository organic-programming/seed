# TASK02 — Extract DRY Go Daemon (Proof of Concept)

## Summary

Extract the Go daemon from `go-dart-holons` into a standalone
`recipes/daemons/gudule-daemon-greeting-go/` holon. This is the first
DRY extraction and serves as the proof-of-concept for all
subsequent daemon extractions.

> [!IMPORTANT]
> **Always use the language SDK as much as possible.**
> The Go daemon must use `go-holons` SDK primitives (server bootstrap,
> transport negotiation, readiness probes) — not raw gRPC boilerplate.

> [!IMPORTANT]
> **Single greeting proto.** All daemons (current and future) share
> one canonical `greeting/v1/greeting.proto` placed in
> `recipes/protos/greeting/v1/`. Each daemon imports this shared
> proto; no per-daemon copies.

## Source

| From | To |
|---|---|
| `recipes/go-dart-holons/examples/greeting/greeting-daemon/` | `recipes/daemons/gudule-daemon-greeting-go/` |

## Acceptance Criteria

- [ ] `gudule-daemon-greeting-go` has its own `holon.yaml` + Go source
- [ ] `family_name: Greeting-Daemon-Go` / binary: `gudule-daemon-greeting-go`
- [ ] Uses `recipes/protos/greeting/v1/greeting.proto` (shared, via `import public`)
- [ ] Builds standalone with `op build`
- [ ] Runs standalone with `op run` (serves GreetingService on gRPC)
- [ ] Supports macOS, Windows, Linux (no platform regression)
- [ ] Uses `go-holons` SDK for server bootstrap
- [ ] Existing `go-dart-holons` is NOT modified (don't clean up yet)

## What This Proves

1. The shared-proto layout works
2. A daemon can build/run independently of any HostUI
3. The extraction pattern is repeatable for all 7 remaining daemons

## Dependencies

TASK01 (shared proto must exist first).
