# QA_TASK001 — Visual and Functional Verification of All Recipes

## Context

Recipe migrations (TASK013, TASK014) rewrote daemon launch and gRPC
connection code. Some UI code may have been corrupted in the process.
This task performs end-to-end verification of every recipe: build,
launch, and confirm the UI renders correctly and the greeting RPC
works.

## What to verify per recipe

For each recipe below:

1. **Build** — must compile without errors.
2. **Launch** — the app must start and display its main window/screen.
3. **UI integrity** — all expected UI elements render:
   - A text input field for the user's name
   - A language selector (dropdown/picker) with at least English, French, Spanish
   - A "Greet" button
   - A response area showing the greeting
4. **RPC round-trip** — type a name, select a language, press Greet,
   confirm a greeting is returned and displayed.
5. **Daemon lifecycle** — the Go or Rust daemon starts automatically
   when the app launches and stops when the app closes.

If any check fails, **fix the issue in the recipe**, commit, and
note what was wrong.

## Checklist

### Go-backend recipes

| Recipe | Build | Launch | UI | RPC | Daemon | Status |
|--------|:-----:|:------:|:--:|:---:|:------:|--------|
| `go-swift-holons` | | | | | | ❌ |
| `go-web-holons` | | | | | | ❌ |
| `go-qt-holons` | | | | | | ❌ |
| `go-dart-holons` | | | | | | ❌ |
| `go-kotlin-holons` | | | | | | ❌ |
| `go-dotnet-holons` | | | | | | ❌ |

### Rust-backend recipes

| Recipe | Build | Launch | UI | RPC | Daemon | Status |
|--------|:-----:|:------:|:--:|:---:|:------:|--------|
| `rust-swift-holons` | | | | | | ❌ |
| `rust-web-holons` | | | | | | ❌ |
| `rust-qt-holons` | | | | | | ❌ |
| `rust-dart-holons` | | | | | | ❌ |
| `rust-kotlin-holons` | | | | | | ❌ |
| `rust-dotnet-holons` | | | | | | ❌ |

### Hello-world examples (`examples/`)

These are SDK echo-server demos. For each: build, run tests,
run the connect example if present.

| Example | Build | Tests | Connect example | Status |
|---------|:-----:|:-----:|:---------------:|--------|
| `go-hello-world` | | | | ❌ |
| `rust-hello-world` | | | | ❌ |
| `python-hello-world` | | | | ❌ |
| `js-hello-world` | | | | ❌ |
| `web-hello-world` | | | | ❌ |
| `swift-hello-world` | | | | ❌ |
| `dart-hello-world` | | | | ❌ |
| `java-hello-world` | | | | ❌ |
| `kotlin-hello-world` | | | | ❌ |
| `csharp-hello-world` | | | | ❌ |
| `c-hello-world` | | | | ❌ |
| `cpp-hello-world` | | | | ❌ |
| `ruby-hello-world` | | | | ❌ |

## Build commands per frontend

| Frontend | Build | Run |
|----------|-------|-----|
| SwiftUI | `swift build` or `xcodebuild` | Run from Xcode or `.build/debug/` |
| Web | `npm install && npm run dev` | Open browser at localhost |
| Qt | `cmake -S . -B build && cmake --build build` | `./build/<binary>` |
| Flutter (Dart) | `flutter build macos` | `flutter run -d macos` |
| Compose (Kotlin) | `./gradlew createDistributable` | `./gradlew run` |
| .NET MAUI | `dotnet build` | `dotnet run` |

## Daemon build (run from recipe root)

```bash
# Go daemon
cd examples/greeting/greeting-daemon && go build -o daemon . && cd -

# Rust daemon
cd examples/greeting/greeting-daemon && cargo build && cd -
```

## Rules

- **Process ALL recipes.** Do not stop after the first one.
- If a recipe fails any check, fix the issue, commit the fix, then
  continue to the next recipe.
- The task is complete only when every row is ✅.
- Report any systematic issues (e.g., all SwiftUI recipes missing
  the language picker) so they can be addressed at the SDK level.
