# TASK10.02 — Extract DRY HostUI Holons

## Summary

Extract one canonical HostUI per technology into `recipes/hostui/`.
Each HostUI connects to any Greeting daemon at a configurable
endpoint. Currently duplicated across 12 recipe repos.

## HostUI Technologies (6)

| HostUI | Tech | Platforms |
|---|---|---|
| `greeting-hostui-swiftui` | SwiftUI | macOS, iOS |
| `greeting-hostui-flutter` | Flutter (Dart) | macOS, iOS, Android, Linux, Windows |
| `greeting-hostui-kotlin` | Kotlin Compose | Android, macOS, Linux, Windows |
| `greeting-hostui-web` | Web (HTML/JS) | Browser |
| `greeting-hostui-dotnet` | .NET MAUI | Windows, macOS, iOS, Android |
| `greeting-hostui-qt` | Qt (C++) | macOS, Linux, Windows |

## Acceptance Criteria

- [ ] Each HostUI has its own `holon.yaml` + source
- [ ] Each HostUI builds standalone with `op build`
- [ ] Each HostUI can connect to any daemon (configurable endpoint)
- [ ] All 6 extracted from existing recipes

## Dependencies

None (can start in parallel with TASK10.01).
