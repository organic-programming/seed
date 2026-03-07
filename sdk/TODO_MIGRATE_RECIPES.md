# TODO: Migrate Recipes & Examples to SDK Client Primitives

Depends on: `TODO_DISCOVER.md` and `TODO_CONNECT.md`.

## Goal

Once `discover` and `connect` are implemented, migrate all recipe
frontends and hello-world examples to use them instead of hardcoded
`localhost:PORT` addresses.

## What changes

### Before (hardcoded address)

```dart
// Flutter frontend — tight coupling to daemon address
final channel = ClientChannel('localhost', port: 9091);
```

```swift
// SwiftUI frontend — hardcoded
let channel = try GRPCChannelPool.with(target: .host("localhost", port: 9091))
```

### After (SDK connect)

```dart
// Flutter frontend — autonomous resolution
import 'package:dart_holons/connect.dart';
final channel = await connect('gudule-daemon-greeting-godart');
```

```swift
// SwiftUI frontend — autonomous resolution
import Holons
let channel = try Holons.connect("gudule-daemon-greeting-goswift")
```

## Migration plan per recipe

### Go-backend recipes (daemon already uses `sdk/go-holons`)

| Recipe | Frontend language | Frontend SDK | Migration |
|---|---|---|---|
| `go-dart-holons` | Dart/Flutter | `dart-holons` | Replace `ClientChannel('localhost', port:)` with `connect(slug)` |
| `go-swift-holons` | Swift/SwiftUI | `swift-holons` | Replace `GRPCChannelPool.with(target:)` with `Holons.connect(slug)` |
| `go-kotlin-holons` | Kotlin/JVM | `kotlin-holons` | Replace `ManagedChannelBuilder.forAddress(...)` with `connect(slug)` |
| `go-web-holons` | Web (JS) | `js-web-holons` | Replace hardcoded URL with `connect(hostPort)` |
| `go-qt-holons` | Qt/C++ | `cpp-holons` | Replace `grpc::CreateChannel(...)` with `holons::connect(slug)` |
| `go-dotnet-holons` | .NET MAUI | `csharp-holons` | Replace `GrpcChannel.ForAddress(...)` with `Holons.Connect(slug)` |

### Rust-backend recipes (daemon needs `sdk/rust-holons` first)

Same frontends as above, plus the daemon itself:

| Recipe | Daemon migration | Frontend migration |
|---|---|---|
| `rust-dart-holons` | Use `holons::serve` | Use `dart_holons.connect` |
| `rust-swift-holons` | Use `holons::serve` | Use `Holons.connect` |
| `rust-kotlin-holons` | Use `holons::serve` | Use `connect(slug)` |
| `rust-web-holons` | Use `holons::serve` | Use `connect(hostPort)` |
| `rust-qt-holons` | Use `holons::serve` | Use `holons::connect` |
| `rust-dotnet-holons` | Use `holons::serve` | Use `Holons.Connect` |

### Hello-world examples

| Example | SDK | Migration |
|---|---|---|
| `examples/go-hello-world` | `go-holons` | Already uses SDK serve. Add connect example. |
| `examples/rust-hello-world` | `rust-holons` | Migrate to SDK serve + add connect example. |
| `examples/dart-hello-world` | `dart-holons` | Verify SDK usage. Add connect example. |
| `examples/swift-hello-world` | `swift-holons` | Verify SDK usage. Add connect example. |
| `examples/js-hello-world` | `js-holons` | Verify SDK usage. Add connect example. |
| `examples/web-hello-world` | `js-web-holons` | Verify SDK usage. Add connect example. |
| `examples/kotlin-hello-world` | `kotlin-holons` | Verify. Add connect example. |
| `examples/java-hello-world` | `java-holons` | Verify. Add connect example. |
| `examples/csharp-hello-world` | `csharp-holons` | Verify. Add connect example. |
| `examples/cpp-hello-world` | `cpp-holons` | Verify. Add connect example. |
| `examples/c-hello-world` | `c-holons` | Verify. Add connect example. |
| `examples/python-hello-world` | `python-holons` | Verify. Add connect example. |
| `examples/ruby-hello-world` | `ruby-holons` | Verify. Add connect example. |
| `examples/objc-hello-world` | `objc-holons` | Verify. Add connect example. |

## Execution order

1. **Rust daemons → `sdk/rust-holons`** (see `op_sdk_adoption.md`
   in `b-alter/todo_videosteno/`)
2. **Implement `discover` in Tier 1 SDKs** (go, rust, dart, swift)
3. **Implement `connect` in Tier 1 SDKs** (go, rust, dart, swift)
4. **Migrate `go-dart-holons`** — the reference recipe, most tested
5. **Migrate `go-swift-holons`** — validates Swift SDK client
6. **Migrate remaining Go-backend recipes**
7. **Migrate Rust-backend recipes** (depends on step 1)
8. **Implement `discover` + `connect` in Tier 2 SDKs**
9. **Implement `discover` + `connect` in Tier 3 SDKs**
10. **Add connect examples to each hello-world**

## Rules

1. **Non-regression**: every recipe and example that builds today
   must still build after migration.
2. **One recipe at a time**: migrate, test, commit.
3. **SDK first**: never migrate a recipe before its SDK has
   `discover` + `connect` implemented and tested.
4. **Backward compatible**: `connect("localhost:9091")` must still
   work (direct dial). Only slug resolution is new.
5. **No proto changes**: gRPC contracts stay as-is.
