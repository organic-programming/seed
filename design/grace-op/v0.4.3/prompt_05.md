# Codex Prompt 05 — Assembly Identity + Contextual Logs

## Problem

All 6 shared HostUIs display hardcoded window titles from the original Go-only
era. When reused inside different assemblies (8 daemon languages), the title
never reflects which assembly is actually running. Additionally, HostUI
DaemonProcess classes produce no startup identification in logs, and the
testmatrix text output lacks the assembly's `family_name`, making it hard to
correlate a test result with its combination.

### Hardcoded titles

| HostUI | Hardcoded | File:Line |
|--------|-----------|-----------|
| Compose | `Gudule Greeting Gokotlin` | `Main.kt:16` + `ContentView.kt:96` |
| Qt | `Gudule Greeting Goqt` | `MainWindow.cpp:16` |
| .NET | `Gudule Greeting Godotnet` | `MainPage.xaml:6` |
| SwiftUI | `Gudule Greeting` | `ContentView.swift:13` |
| Flutter | `Flutter Greeting` | `app.dart:72` |
| Web | `Gudule Greeting Goweb` | `index.html:6` + `ui.ts:8` |

---

## Part 1 — `OP_ASSEMBLY_FAMILY` Environment Variable

### 1a — grace-op: export the env var

In `holons/grace-op`, when `op run` launches a composite's primary artifact,
set `OP_ASSEMBLY_FAMILY` to the composite's `family_name` from `holon.yaml`.
The relevant code is in `holons/grace-op/internal/holons/` and
`holons/grace-op/internal/cli/`.

Find where `op run` reads the composite manifest and launches the primary
artifact (app bundle or binary). Before calling `exec.Command`, add:

```go
cmd.Env = append(os.Environ(), "OP_ASSEMBLY_FAMILY="+manifest.FamilyName)
```

Also set `OP_ASSEMBLY_TRANSPORT` to the composite's `transport` field.

### 1b — Each HostUI: read `OP_ASSEMBLY_FAMILY` at startup

Replace every hardcoded title with a runtime lookup. Fall back to the current
hardcoded default when the env var is not set (the HostUI must still work when
launched standalone).

#### Compose (Kotlin)

`recipes/hostui/gudule-greeting-hostui-kotlin/src/main/kotlin/greeting/gokotlin/Main.kt`
```kotlin
val assemblyFamily = System.getenv("OP_ASSEMBLY_FAMILY") ?: "Greeting-Kotlinui-Go"
// Use assemblyFamily in Window title (line 16)
```

`recipes/hostui/gudule-greeting-hostui-kotlin/src/main/kotlin/greeting/gokotlin/ui/ContentView.kt`
```kotlin
// Line 96: replace "Gudule Greeting Gokotlin" with "Gudule $assemblyFamily"
// Pass assemblyFamily as a parameter from Main.kt
```

Update subtitle (~line 97) to reflect the actual daemon language and transport.

#### SwiftUI

`recipes/hostui/gudule-greeting-hostui-swiftui/GreetingSwiftUI/ContentView.swift`
```swift
let assemblyFamily = ProcessInfo.processInfo.environment["OP_ASSEMBLY_FAMILY"] ?? "Greeting-SwiftUI-Go"
// Line 13: use "Gudule \(assemblyFamily)" instead of "Gudule Greeting"
```

#### Flutter (Dart)

`recipes/hostui/gudule-greeting-hostui-flutter/lib/src/app.dart`
```dart
final assemblyFamily = Platform.environment['OP_ASSEMBLY_FAMILY'] ?? 'Greeting-Flutter-Go';
// Line 72: use "Gudule $assemblyFamily" as MaterialApp title
```

#### .NET (C# / XAML)

`recipes/hostui/gudule-greeting-hostui-dotnet/MainPage.xaml` has a static
`Title=` attribute. Read the env var in `MainPage.xaml.cs` and set it
programmatically:

```csharp
var assemblyFamily = Environment.GetEnvironmentVariable("OP_ASSEMBLY_FAMILY") ?? "Greeting-Dotnet-Go";
Title = $"Gudule {assemblyFamily}";
```

Also update the `<Label>` text (line 9) programmatically.

#### Qt (C++)

`recipes/hostui/gudule-greeting-hostui-qt/src/MainWindow.cpp`
```cpp
auto family = qEnvironmentVariable("OP_ASSEMBLY_FAMILY", "Greeting-Qt-Go");
setWindowTitle(QStringLiteral("Gudule ") + family); // line 16
```

#### Web

The web HostUI is served by the daemon, so `OP_ASSEMBLY_FAMILY` is not
directly available in the browser. Since the web HostUI already knows its
daemon slug from `DaemonProcess`, derive the title from the slug:
- `index.html:6` (`<title>`) — set via `document.title` in TS
- `ui.ts:8` (`<h1>`) — set dynamically

---

## Part 2 — DaemonProcess Startup Logs

Each HostUI's `DaemonProcess` class should emit an identification line to
`stderr` when it starts and when it connects to the daemon. This helps
debugging when running assemblies — you can see which combination is active
in the terminal.

### Format

```
[HostUI] assembly=Greeting-Flutter-Go daemon=gudule-daemon-greeting-go transport=tcp
[HostUI] connected to gudule-daemon-greeting-go on localhost:50051
```

### Files to modify

| HostUI | DaemonProcess file |
|--------|-------------------|
| Compose | `recipes/hostui/gudule-greeting-hostui-kotlin/src/main/kotlin/greeting/gokotlin/grpc/DaemonProcess.kt` |
| SwiftUI | `recipes/hostui/gudule-greeting-hostui-swiftui/GreetingSwiftUI/DaemonProcess.swift` |
| Flutter | `recipes/hostui/gudule-greeting-hostui-flutter/lib/src/client/greeting_target.dart` |
| .NET | `recipes/hostui/gudule-greeting-hostui-dotnet/Services/DaemonProcess.cs` |
| Qt | `recipes/hostui/gudule-greeting-hostui-qt/src/DaemonProcess.cpp` |
| Web | `recipes/hostui/gudule-greeting-hostui-web/src/grpc-client.ts` |

Each DaemonProcess already reads the daemon slug. At the point where the
daemon is discovered/started and the gRPC channel is established, add a
`stderr` log line with the above format. Use `OP_ASSEMBLY_FAMILY` if set,
else a generic fallback.

---

## Part 3 — Testmatrix Contextual Enhancements

### 3a — Read `family_name` from manifests

In `recipes/testmatrix/gudule-greeting-testmatrix/matrix.go`, the
`manifestLite` struct (line 58) and `loadTarget` function (line 311) already
parse `holon.yaml`. Add `FamilyName` to `manifestLite` and carry it into
`Target` and `TargetResult`:

```go
// manifestLite addition
FamilyName string `yaml:"family_name,omitempty"`

// Target addition
FamilyName string `json:"family_name,omitempty"`

// TargetResult addition
FamilyName string `json:"family_name,omitempty"`
```

### 3b — Richer text output

In `renderReport` (line 551), the text format currently shows:
```
%-13s %-11s %s
```
Add `family_name` between status and kind:
```
%-13s %-25s %-11s %s
```
So the output reads:
```
passed        Greeting-Kotlinui-Go       composition recipes/composition/direct-call/charon-direct-go-go
smoke-passed  Greeting-Flutter-Rust      assembly    recipes/assemblies/gudule-greeting-flutter-rust
```

---

## Verification

```bash
# Grace-op tests
cd holons/grace-op
go test ./internal/holons ./internal/cli

# Testmatrix tests
cd recipes/testmatrix/gudule-greeting-testmatrix
go test ./...

# HostUI tests
cd recipes/hostui/gudule-greeting-hostui-flutter
flutter test

# Manual smoke: run a non-Go assembly and verify:
# 1. Window title shows "Gudule Greeting-Flutter-Rust"
# 2. stderr shows "[HostUI] assembly=Greeting-Flutter-Rust daemon=... transport=..."
# 3. testmatrix text output includes family_name column
op build recipes/assemblies/gudule-greeting-flutter-rust
op run --no-build recipes/assemblies/gudule-greeting-flutter-rust
```

## Scope

- Branch: `op-v0.4-dev`
- Do NOT modify assembly manifests — only grace-op runner + 6 HostUIs +
  testmatrix
- Do NOT rename packages, classes, or theme functions (e.g. `GokotlinTheme`
  stays)
- Keep existing tests passing; add tests for `OP_ASSEMBLY_FAMILY` propagation
  and `FamilyName` in testmatrix results
