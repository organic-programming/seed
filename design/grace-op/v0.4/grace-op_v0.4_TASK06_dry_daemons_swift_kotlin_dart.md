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

| Daemon | Source repo | SDK | `family_name` |
|---|---|---|---|
| `gudule-daemon-greeting-swift` | `go-swift-holons` | `swift-holons` | `Greeting-Daemon-Swift` |
| `gudule-daemon-greeting-kotlin` | `go-kotlin-holons` | `kotlin-holons` | `Greeting-Daemon-Kotlin` |
| `gudule-daemon-greeting-dart` | new (no existing source) | `dart-holons` | `Greeting-Daemon-Dart` |

## Acceptance Criteria

- [ ] Each daemon has its own `holon.yaml` + source
- [ ] All use `recipes/protos/greeting/v1/greeting.proto` (shared, via `import public`)
- [ ] Each builds standalone with `op build`
- [ ] Each runs standalone with `op run`
- [ ] Uses respective language SDK for server bootstrap

## Dependencies

TASK05.
