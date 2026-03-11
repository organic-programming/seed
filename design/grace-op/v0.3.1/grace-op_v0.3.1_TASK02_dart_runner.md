# TASK02 — Tier 2 Runner (dart)

## Context

v0.3 introduced a runner registry with native build tools (`cargo`, `npm`, `dotnet`, etc.), including `flutter` for Dart UIs. However, pure Dart daemons (like `dart-package`) were left relying on the verbose `recipe` runner (which requires manual `exec` YAML blocks for every step).

Pure Dart backend components are a known requirement in the OP ecosystem (e.g., Godart-only services). To provide a declarative experience, `grace-op` needs a dedicated `dart` runner.

**Repository**: `organic-programming/holons/grace-op`

---

### `dart` runner

For `build.runner: dart`. 
(Distinct from the `flutter` runner, designed for pure Dart backend packages/daemons).

| Op | Command |
|---|---|
| Check | Verify `dart` on PATH. |
| Build | `dart pub get && dart compile exe bin/main.dart -o build/main` (or compile destination based on manifest). |
| Test | `dart test` |
| Clean | Remove `build/`, `.dart_tool/`. |

## Checklist

- [ ] Implement `DartRunner` (distinct from `FlutterRunner`)
- [ ] Unit tests for the runner's Check operation
- [ ] Integration: `op check` and `op build --dry-run`
- [ ] Update `runners` registry in `internal/holons/runner.go`
- [ ] `go test ./...` — zero failures

## Dependencies

- v0.3 TASK03 (runner registry)
