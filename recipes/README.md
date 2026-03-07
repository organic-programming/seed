# Recipes

A recipe is a cross-language assembly pattern: one daemon holon and one
frontend holon sharing the same protobuf contract.

Unlike a language SDK, a recipe is not imported as a library. It ships
architecture docs, build rules, manifests, and a working example under
`examples/greeting/`.

## Current Workspace Status

| Recipe | Backend | Frontend | Current status |
|--------|---------|----------|----------------|
| [go-dart-holons](./go-dart-holons/) | Go | Flutter/Dart | working; desktop frontend uses `dart-holons.connect()` |
| [go-swift-holons](./go-swift-holons/) | Go | SwiftUI | working; frontend still uses raw `grpc-swift` over localhost TCP |
| [go-kotlin-holons](./go-kotlin-holons/) | Go | Compose Desktop | working; frontend uses `kotlin-holons.Connect.connect()` |
| [go-web-holons](./go-web-holons/) | Go | Web | scaffolded; frontend still uses raw web client wiring |
| [go-qt-holons](./go-qt-holons/) | Go | Qt/C++ | scaffolded; fixed localhost TCP |
| [go-dotnet-holons](./go-dotnet-holons/) | Go | .NET MAUI | working on macOS; frontend uses `Holons.ConnectTarget(...)` |
| [rust-dart-holons](./rust-dart-holons/) | Rust | Flutter/Dart | working scaffold; frontend uses `dart-holons`, daemon still raw Rust |
| [rust-swift-holons](./rust-swift-holons/) | Rust | SwiftUI | scaffolded; raw gRPC on both sides |
| [rust-kotlin-holons](./rust-kotlin-holons/) | Rust | Compose Desktop | scaffolded; raw gRPC on both sides |
| [rust-web-holons](./rust-web-holons/) | Rust | Web | scaffolded; raw web client wiring |
| [rust-dotnet-holons](./rust-dotnet-holons/) | Rust | .NET MAUI | scaffolded; raw gRPC on both sides |
| [rust-qt-holons](./rust-qt-holons/) | Rust | Qt/C++ | scaffolded; raw gRPC on both sides |

No `BLOCKED.md` files are present in the current workspace snapshot.

## Shared Structure

```text
<backend>-<frontend>-holons/
└── examples/greeting/
    ├── holon.yaml           # composite recipe manifest
    ├── greeting-daemon/     # backend daemon
    └── greeting-<name>/     # frontend app
```

Every recipe carries:

- a composite `holon.yaml` at `examples/greeting/`
- a daemon `holon.yaml`
- a frontend app or component

All recipes share the same `greeting.proto` contract: `ListLanguages`
and `SayHello`.

## Guides

- [IMPLEMENTATION_ON_MAC_OS.md](./IMPLEMENTATION_ON_MAC_OS.md)
- [IMPLEMENTATION_ON_WINDOWS.md](./IMPLEMENTATION_ON_WINDOWS.md)
- [`../sdk/SDK_GUIDE.md`](../sdk/SDK_GUIDE.md) for the audited SDK usage table
