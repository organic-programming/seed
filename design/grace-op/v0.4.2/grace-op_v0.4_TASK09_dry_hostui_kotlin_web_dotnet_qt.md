# TASK05 — Extract DRY HostUIs: Kotlin Compose, Web, .NET MAUI, Qt

## Summary

Extract the four remaining HostUIs into `recipes/hostui/`. Each
follows the connect-approach pattern validated by TASK04.

> [!IMPORTANT]
> **Connect approach only.** Every HostUI must use its language's
> SDK `connect(slug)` primitive — no raw gRPC dial.

> [!IMPORTANT]
> **Always use the language SDK.**
> - Kotlin → `kotlin-holons` SDK
> - Web → `@connectrpc/connect-web` (Connect protocol)
> - .NET → `csharp-holons` SDK
> - Qt/C++ → `cpp-holons` SDK

## HostUIs

| HostUI | Source repo | SDK | `family_name` |
|---|---|---|---|
| `gudule-greeting-hostui-compose` | `go-kotlin-holons` | `kotlin-holons` | `Greeting-Hostui-Compose` |
| `gudule-greeting-hostui-web` | `go-web-holons` | `@connectrpc/connect-web` | `Greeting-Hostui-Web` |
| `gudule-greeting-hostui-dotnet` | `go-dotnet-holons` | `csharp-holons` | `Greeting-Hostui-Dotnet` |
| `gudule-greeting-hostui-qt` | `go-qt-holons` | `cpp-holons` | `Greeting-Hostui-Qt` |

## Acceptance Criteria

- [ ] Each HostUI has its own `holon.yaml` + source
- [ ] All use `recipes/protos/greeting/v1/greeting.proto` (shared, via `import public`)
- [ ] Each uses SDK `connect(slug)` for daemon resolution
- [ ] Each builds standalone with `op build`
- [ ] Each can connect to any GreetingService daemon
- [ ] Existing submodule repos NOT modified

## Dependencies

TASK08.
