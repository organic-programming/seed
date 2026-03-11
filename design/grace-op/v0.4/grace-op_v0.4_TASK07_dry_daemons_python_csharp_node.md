# TASK07 — Extract DRY Daemons: Python, C#, Node.js

## Summary

Extract three daemons from server-scripting ecosystems.
Each implements GreetingService against the shared proto.

> [!IMPORTANT]
> **Always use the language SDK as much as possible.**
> - Python daemon → `python-holons` SDK (or raw gRPC-Python if SDK not yet available)
> - C# daemon → `csharp-holons` SDK / `Grpc.AspNetCore`
> - Node.js daemon → `node-holons` SDK / `@grpc/grpc-js`

## Daemons

| Daemon | Source repo | SDK | `family_name` |
|---|---|---|---|
| `gudule-daemon-greeting-python` | new | `python-holons` | `Greeting-Daemon-Python` |
| `gudule-daemon-greeting-csharp` | `go-dotnet-holons` or new | `csharp-holons` | `Greeting-Daemon-Csharp` |
| `gudule-daemon-greeting-node` | new | `node-holons` | `Greeting-Daemon-Node` |

## Acceptance Criteria

- [ ] Each daemon has its own `holon.yaml` + source
- [ ] All use `recipes/protos/greeting/v1/greeting.proto` (shared, via `import public`)
- [ ] Each builds standalone with `op build`
- [ ] Each runs standalone with `op run`
- [ ] Uses respective language SDK for server bootstrap

## Dependencies

TASK06.
