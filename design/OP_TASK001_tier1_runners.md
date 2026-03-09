# OP_TASK001 — Tier 1 Runners (cargo, swift-package, flutter)

## Context

`grace-op` currently has 3 runners: `go-module`, `cmake`, `recipe`.
These 3 new runners unlock all currently working recipes.

**Repository**: `organic-programming/holons/grace-op`

## Runner interface

Every runner implements 4 lifecycle operations, selected by `build.runner`
in `holon.yaml`:

```go
type Runner interface {
    Check(ctx RunContext) error  // verify toolchain is available
    Build(ctx RunContext) error  // execute build command
    Test(ctx RunContext) error   // execute test command
    Clean(ctx RunContext) error  // remove build artifacts
}
```

Create `internal/holons/runner.go` as the registry if it doesn't exist.

---

### `cargo` runner

For `build.runner: cargo`.

| Op | Command |
|---|---|
| Check | verify `cargo` and `rustc` on PATH |
| Build | `cargo build --release` (or `--debug`) |
| Test | `cargo test` |
| Clean | `cargo clean` |

Artifact: `target/release/<artifacts.binary>`.
Handle CMake+cargo hybrid: if `CMakeLists.txt` exists AND runner is
`cargo`, fall back to cmake for build but use cargo for test/clean.

### `swift-package` runner

For `build.runner: swift-package`.

| Op | Command |
|---|---|
| Check | verify `swift` and `xcodebuild` on PATH |
| Build | `swift build` (SPM) or `xcodebuild` (Xcode project) |
| Test | `swift test` |
| Clean | `swift package clean` |

Detection: `Package.swift` → SPM. `.xcodeproj`/`.xcworkspace` → Xcode.

### `flutter` runner

For `build.runner: flutter`.

| Op | Command |
|---|---|
| Check | verify `flutter` and `dart` on PATH |
| Build | `flutter build <platform>` |
| Test | `flutter test` |
| Clean | `flutter clean` |

Platform mapping: `macos` → `macos --debug`, `linux` → `linux --debug`,
`windows` → `windows --debug`, `ios` → `ios --debug --no-codesign`,
`android` → `apk --debug`.

## Checklist

- [ ] Create `internal/holons/runner.go` registry (if missing)
- [ ] Implement `CargoRunner`
- [ ] Implement `SwiftPackageRunner`
- [ ] Implement `FlutterRunner`
- [ ] Unit tests for each runner's Check (mocked commands)
- [ ] Integration: `op check` and `op build --dry-run` with minimal holon.yaml
- [ ] `go test ./...` — zero failures

## Rules

- Do NOT edit `OP.md` — only modify `grace-op/` internals
- Do NOT modify existing runners (go-module, cmake, recipe)
- Runners must handle missing deps with actionable error messages

## Dependencies

- None — self-contained
