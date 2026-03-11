# TASK03 — Extract DRY Daemons: Python, C#, Node.js

## Summary

Extract one daemon from an existing repo (C#) and create two new
daemons (Python, Node.js — no existing source). Each implements
GreetingService against the shared proto.

> [!WARNING]
> **Python and Node.js are new implementations.** There is no
> existing source to extract.

> [!IMPORTANT]
> **Always use the language SDK.**
> - Python daemon → `python-holons` SDK
> - C# daemon → `csharp-holons` SDK
> - Node.js daemon → `node-holons` SDK

## Daemons

| Daemon | Source repo | SDK | `family_name` |
|---|---|---|---|
| `gudule-daemon-greeting-python` | new (no existing source) | `python-holons` | `Greeting-Daemon-Python` |
| `gudule-daemon-greeting-csharp` | `go-dotnet-holons` | `csharp-holons` | `Greeting-Daemon-Csharp` |
| `gudule-daemon-greeting-node` | new (no existing source) | `node-holons` | `Greeting-Daemon-Node` |

## Acceptance Criteria

- [ ] Each daemon has its own `holon.yaml` + source
- [ ] All use `recipes/protos/greeting/v1/greeting.proto` (shared, via `import public`)
- [ ] Each builds standalone with `op build`
- [ ] Each runs standalone with `op run`
- [ ] Uses respective language SDK for server bootstrap

## Dependencies

v0.4.2/TASK02 (Swift, Kotlin, Dart daemons extracted).
