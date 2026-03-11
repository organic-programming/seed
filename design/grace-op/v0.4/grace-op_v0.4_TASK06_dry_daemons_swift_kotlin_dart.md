# TASK06 — Extract DRY Daemons: Swift, Kotlin, Dart

## Summary

Extract three daemons that share a mobile/cross-platform profile.
Each implements GreetingService against the shared proto.

> [!IMPORTANT]
> **Always use the language SDK as much as possible.**
> - Swift daemon → `swift-holons` SDK
> - Kotlin daemon → `kotlin-holons` SDK
> - Dart daemon → `dart-holons` SDK (server-side)

## Daemons

| Daemon | Source repo | SDK |
|---|---|---|
| `greeting-daemon-swift` | `go-swift-holons` or new | `swift-holons` |
| `greeting-daemon-kotlin` | `go-kotlin-holons` or new | `kotlin-holons` |
| `greeting-daemon-dart` | new | `dart-holons` |

## Acceptance Criteria

- [ ] Each daemon has its own `holon.yaml` + source
- [ ] All use `recipes/protos/greeting/v1/greeting.proto` (shared)
- [ ] Each builds standalone with `op build`
- [ ] Each runs standalone with `op run`
- [ ] Uses respective language SDK for server bootstrap

## Dependencies

TASK05.
