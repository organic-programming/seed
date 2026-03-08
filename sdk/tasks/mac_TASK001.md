# mac_TASK001 — Build Go-Backend Recipes on macOS

## Context

Depends on: TASK013 (Go recipe migration to SDK `connect`).

Build and verify all Go-backend recipes on macOS. Use
`IMPLEMENTATION_ON_MAC_OS.md` as the full reference.

## Recipes

| # | Recipe | Frontend | Reference |
|---|--------|----------|-----------|
| 1 | `go-dart-holons` | Flutter | ✅ exists — verify build |
| 2 | `go-swift-holons` | SwiftUI | ✅ exists — verify build |
| 3 | `go-kotlin-holons` | Compose Desktop | ✅ exists — verify build |
| 4 | `go-dotnet-holons` | .NET MAUI (Mac Catalyst) | ✅ exists — verify build |
| 5 | `go-web-holons` | TypeScript (Vite) | implement from scratch |
| 6 | `go-qt-holons` | Qt/C++ (CMake) | implement from scratch |

## For each recipe

1. Ensure daemon uses `go-holons` SDK `serve.Run()`.
2. Ensure frontend uses its SDK's `connect("slug")`.
3. Run `op check <recipe>/examples/greeting`.
4. Run `op build <recipe>/examples/greeting`.
5. Verify the built artifact launches and shows the greeting UI.

## Required tools

`go`, `flutter`, `xcodebuild`, `gradle` (JDK 17+), `dotnet`,
`node`/`npm`, `cmake`, Qt6 (Homebrew).

## Rules

- One recipe at a time: build, verify, commit.
- If blocked after 3 attempts, write `BLOCKED.md` and move on.
- Use `--dry-run` first to verify the build plan.
