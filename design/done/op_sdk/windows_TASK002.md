# windows_TASK002 — Add Windows Targets to Cross-Platform Recipes

## Context

Depends on: windows_TASK001 (WinUI recipes), mac_TASK001, mac_TASK002.

All cross-platform recipes already build on macOS. This task adds
`windows` targets to their composite `holon.yaml` files and verifies
they build on Windows.

See `IMPLEMENTATION_ON_WINDOWS.md` for full reference.

## Recipes to add Windows targets

| # | Recipe | Frontend | Windows build command |
|---|--------|----------|----------------------|
| 1 | `go-dart-holons` | Flutter | `flutter build windows` |
| 2 | `rust-dart-holons` | Flutter | `flutter build windows` |
| 3 | `go-kotlin-holons` | Compose | `gradlew.bat createDistributable` |
| 4 | `rust-kotlin-holons` | Compose | `gradlew.bat createDistributable` |
| 5 | `go-web-holons` | TypeScript | `npm run build` (identical to macOS) |
| 6 | `rust-web-holons` | TypeScript | `npm run build` (identical to macOS) |
| 7 | `go-qt-holons` | Qt/C++ | `cmake -G "Visual Studio 17 2022"` |
| 8 | `rust-qt-holons` | Qt/C++ | `cmake -G "Visual Studio 17 2022"` |

## What to do for each

1. Add a `windows` section under `targets` in the composite `holon.yaml`.
2. Set daemon binary extension to `.exe`.
3. Run `op build <recipe>/examples/greeting --target windows`.
4. Verify the artifact exists and launches.

## Windows-specific notes

- **Binary extension:** all binaries end in `.exe` on Windows.
- **Path separators:** `holon.yaml` uses `/` everywhere — `op`
  converts via `filepath.FromSlash`.
- **Named Pipes:** optional transport (`npipe://gudule-greeting`),
  not required for this task.
- **Long paths:** enable in Windows registry for `node_modules`
  and `.gradle` directories.

## Required tools

Visual Studio 2022, .NET 8, Go, Cargo, Flutter, JDK 17+,
Node.js/npm, Qt6 (MSVC build), CMake.

## Rules

- Web recipes (5, 6) build identically on both platforms — verify only.
- Qt recipes (7, 8) need MSVC or MinGW — test with Visual Studio generator.
- One recipe at a time: add target, build, verify, commit.
