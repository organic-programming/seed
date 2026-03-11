# TASK01 — Shared Greeting Proto

## Summary

Create the single canonical `greeting.proto` in a shared location
before any daemon or HostUI extraction.

> [!IMPORTANT]
> **One greeting proto governs all daemons and HostUIs.** No
> per-language copies. Language-specific codegen is each holon's
> responsibility (using `protoc` + the language plugin), but the
> `.proto` source is shared.

## Layout

```
recipes/protos/greeting/v1/
└── greeting.proto
```

## Acceptance Criteria

- [ ] `greeting.proto` extracted from current `go-dart-holons`
- [ ] Placed in `recipes/protos/greeting/v1/`
- [ ] Proto package: `greeting.v1`
- [ ] Service: `GreetingService` with `SayHello` RPC
- [ ] No per-daemon copies — all daemons/HostUIs reference this path

## Dependencies

None (first task in the chain).
