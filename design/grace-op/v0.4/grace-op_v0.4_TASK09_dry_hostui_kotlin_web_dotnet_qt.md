# TASK09 — Extract DRY HostUIs: Kotlin Compose, Web, .NET MAUI, Qt

## Summary

Extract the four remaining HostUIs into `recipes/hostui/`. Each
follows the connect-approach pattern validated by TASK10.

> [!IMPORTANT]
> **Connect approach only.** Every HostUI must use its language's
> SDK `connect(slug)` primitive — no raw gRPC dial.

> [!IMPORTANT]
> **Always use the language SDK as much as possible.**
> - Kotlin → `kotlin-holons` SDK
> - Web → `@connectrpc/connect-web` (Connect protocol)
> - .NET → `csharp-holons` SDK / `Grpc.Net.Client`
> - Qt/C++ → `cpp-holons` SDK or raw gRPC-C++ if SDK unavailable

## HostUIs

| HostUI | Source repo | SDK |
|---|---|---|
| `greeting-hostui-kotlin` | `go-kotlin-holons` | `kotlin-holons` |
| `greeting-hostui-web` | `go-web-holons` | `@connectrpc/connect-web` |
| `greeting-hostui-dotnet` | `go-dotnet-holons` | `csharp-holons` |
| `greeting-hostui-qt` | `go-qt-holons` | `cpp-holons` (or raw gRPC-C++) |

## Acceptance Criteria

- [ ] Each HostUI has its own `holon.yaml` + source
- [ ] All use `recipes/protos/greeting/v1/greeting.proto` (shared)
- [ ] Each uses SDK `connect(slug)` for daemon resolution
- [ ] Each builds standalone with `op build`
- [ ] Each can connect to any GreetingService daemon
- [ ] Existing submodule repos NOT modified

## Dependencies

TASK08.
