# Recipes

A recipe is a **cross-language assembly pattern** — it shows how to
combine two or more language SDKs into a single application.

Unlike a language SDK (which you `import`), a recipe provides
architecture docs, build scripts, templates, and a working example.

## Available Recipes

### Go Backend

| Recipe | Frontend | Platforms | Status |
|--------|----------|-----------|--------|
| [go-dart-holons](https://github.com/organic-programming/go-dart-holons) | Flutter/Dart | macOS, Linux, Windows, iOS, Android | ✅ |
| [go-swift-holons](https://github.com/organic-programming/go-swift-holons) | SwiftUI | macOS, iOS | ✅ |
| [go-kotlin-holons](https://github.com/organic-programming/go-kotlin-holons) | Jetpack Compose | Android, desktop | planned |
| [go-web-holons](https://github.com/organic-programming/go-web-holons) | TypeScript (web) | browser | planned |
| [go-dotnet-holons](https://github.com/organic-programming/go-dotnet-holons) | WinUI 3 / .NET MAUI | Windows | planned |
| [go-qt-holons](https://github.com/organic-programming/go-qt-holons) | Qt / C++ | desktop, embedded | planned |

### Rust Backend

| Recipe | Frontend | Platforms | Status |
|--------|----------|-----------|--------|
| [rust-dart-holons](https://github.com/organic-programming/rust-dart-holons) | Flutter/Dart | macOS, Linux, Windows, iOS, Android | planned |
| [rust-swift-holons](https://github.com/organic-programming/rust-swift-holons) | SwiftUI | macOS, iOS | planned |
| [rust-kotlin-holons](https://github.com/organic-programming/rust-kotlin-holons) | Jetpack Compose | Android, desktop | planned |
| [rust-web-holons](https://github.com/organic-programming/rust-web-holons) | TypeScript (web) | browser | planned |
| [rust-dotnet-holons](https://github.com/organic-programming/rust-dotnet-holons) | WinUI 3 / .NET MAUI | Windows | planned |
| [rust-qt-holons](https://github.com/organic-programming/rust-qt-holons) | Qt / C++ | desktop, embedded | planned |

## Pattern

Every recipe follows the same structure:

```text
<backend>-<frontend>-holons/
└── examples/greeting/
    ├── holon.yaml           # composite recipe manifest
    ├── greeting-daemon/     # backend (Go or Rust gRPC daemon)
    └── greeting-<name>/     # frontend (UI framework)
```

All recipes share the same `greeting.proto` contract — a
`GreetingService` that greets users in 56 languages. Only the daemon
language and the UI framework change.

## Implementation Guides

- [IMPLEMENTATION_ON_MAC_OS.md](IMPLEMENTATION_ON_MAC_OS.md) — build all recipes on macOS
- [IMPLEMENTATION_ON_WINDOWS.md](IMPLEMENTATION_ON_WINDOWS.md) — build all recipes on Windows
