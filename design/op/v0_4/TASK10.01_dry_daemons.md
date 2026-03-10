# TASK10.01 — Extract DRY Daemon Holons

## Summary

Extract one canonical daemon per language from the 12 copies into
`recipes/daemons/`. Currently each recipe repo has its own copy
of the daemon source. After extraction, daemon code exists once.

## Daemon Languages (8)

| Daemon | Source | Greeting proto |
|---|---|---|
| `greeting-daemon-go` | Go | `greeting.proto` |
| `greeting-daemon-rust` | Rust | `greeting.proto` |
| `greeting-daemon-python` | Python | `greeting.proto` |
| `greeting-daemon-swift` | Swift | `greeting.proto` |
| `greeting-daemon-kotlin` | Kotlin | `greeting.proto` |
| `greeting-daemon-dart` | Dart | `greeting.proto` |
| `greeting-daemon-csharp` | C# | `greeting.proto` |
| `greeting-daemon-node` | Node.js | `greeting.proto` |

## Acceptance Criteria

- [ ] Each daemon has its own `holon.yaml` + source
- [ ] Each daemon builds standalone with `op build`
- [ ] Each daemon runs standalone with `op run`
- [ ] Proto contract is identical across all daemons
- [ ] Go and Rust extracted from existing recipes; others new

## Dependencies

None (can start immediately).
