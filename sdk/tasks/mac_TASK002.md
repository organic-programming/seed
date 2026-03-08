# mac_TASK002 — Build Rust-Backend Recipes on macOS

## Context

Depends on: TASK014 (Rust recipe migration), mac_TASK001 (Go recipes
serve as reference for frontend patterns).

Build and verify all Rust-backend recipes on macOS. Use
`IMPLEMENTATION_ON_MAC_OS.md` as the full reference.

## Recipes

| # | Recipe | Frontend | Notes |
|---|--------|----------|-------|
| 1 | `rust-dart-holons` | Flutter | first Rust recipe — proves cross-backend |
| 2 | `rust-swift-holons` | SwiftUI | clone go-swift + Rust daemon |
| 3 | `rust-kotlin-holons` | Compose Desktop | clone go-kotlin + Rust daemon |
| 4 | `rust-dotnet-holons` | .NET MAUI (Mac Catalyst) | clone go-dotnet + Rust daemon |
| 5 | `rust-web-holons` | TypeScript (Vite) | clone go-web + Rust daemon + `tonic-web` |
| 6 | `rust-qt-holons` | Qt/C++ (CMake) | clone go-qt + Rust daemon |

## Rust daemon pattern

All Rust daemons share the same structure:

```
greeting-daemon/
├── Cargo.toml         # tonic, prost, tokio
├── build.rs           # tonic-build
├── holon.yaml         # kind: native
├── proto/greeting.proto
└── src/
    ├── main.rs        # tonic server
    ├── greetings.rs   # 56 languages (port from Go)
    └── service.rs     # GreetingService impl
```

Build the first Rust daemon for `rust-dart-holons`, then reuse it
(with binary name changes) for all others.

## Required tools

`cargo`, `rustc`, plus same frontend tools as mac_TASK001.

## Rules

- One recipe at a time: build, verify, commit.
- If blocked after 3 attempts, write `BLOCKED.md` and move on.
- Share the Rust daemon code — only the binary name differs.
