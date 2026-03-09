# SDK Fleet

The SDK fleet provides language-native Organic Programming runtime
surfaces. The shared architecture is:

- `serve`
- `transport`
- `identity`
- `discover`
- `connect`

All SDKs now parse `holon.yaml` identity data and transport URIs. The
remaining differences are in `serve` depth, `connect` availability, and
Holon-RPC support.

---

## Fleet Overview

| SDK | Language | Serve | Discover | Connect | Holon-RPC | Version |
|-----|----------|-------|----------|---------|-----------|:-------:|
| [go-holons](https://github.com/organic-programming/go-holons) | Go | runner | ✅ | ✅ | client + server | 0.3.0 |
| [js-holons](https://github.com/organic-programming/js-holons) | JavaScript (Node) | runner | ✅ | ✅ | client + server | 0.1.0 |
| [python-holons](https://github.com/organic-programming/python-holons) | Python | runner | ✅ | ✅ | client + server | 0.1.0 |
| [rust-holons](https://github.com/organic-programming/rust-holons) | Rust | flags only | ✅ | — | — | 0.1.0 |
| [swift-holons](https://github.com/organic-programming/swift-holons) | Swift | flags only | ✅ | — | client | 0.1.0 |
| [dart-holons](https://github.com/organic-programming/dart-holons) | Dart | flags only | ✅ | ✅ tcp-only | client + server | 0.1.0 |
| [kotlin-holons](https://github.com/organic-programming/kotlin-holons) | Kotlin | flags only | ✅ | ✅ tcp-only | client | 0.1.0 |
| [java-holons](https://github.com/organic-programming/java-holons) | Java | flags only | ✅ | ✅ tcp-only | client | 0.1.0 |
| [csharp-holons](https://github.com/organic-programming/csharp-holons) | C# | flags only | ✅ | ✅ tcp-only | client | 0.1.0 |
| [cpp-holons](https://github.com/organic-programming/cpp-holons) | C++ | flags only | ✅ | — | client | 0.1.0 |
| [c-holons](https://github.com/organic-programming/c-holons) | C | runner | ✅ | — | wrapper binaries only | 0.1.0 |
| [objc-holons](https://github.com/organic-programming/objc-holons) | Objective-C | flags only | ✅ | — | client | 0.1.0 |
| [ruby-holons](https://github.com/organic-programming/ruby-holons) | Ruby | flags only | ✅ | — | client | 0.1.0 |
| [js-web-holons](https://github.com/organic-programming/js-web-holons) | JavaScript (Browser) | browser | remote manifest only | — | browser client + node harness | 0.1.0 |

`runner` means the SDK can host the standard `serve` lifecycle itself.
`flags only` means the SDK currently stops at CLI parsing plus transport
primitives.

---

## Transport Surface

All SDKs support the 6 transport schemes defined in PROTOCOL.md:
`tcp://`, `unix://`, `stdio://`, `mem://`, `ws://`, `wss://`.

| SDK | tcp | unix | stdio | mem | ws/wss | Holon-RPC |
|-----|:---:|:----:|:-----:|:---:|:------:|:---------:|
| `go-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | server + client |
| `js-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | server + client |
| `python-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | server + client |
| `rust-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | — |
| `swift-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | server + client |
| `dart-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | server + client |
| `kotlin-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | server + client |
| `java-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | server + client |
| `csharp-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | server + client |
| `cpp-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | server + client |
| `c-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | server + client |
| `objc-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | server + client |
| `ruby-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | server + client |
| `js-web-holons` | — | — | — | — | ✅ | browser client |

See [SDK_GUIDE.md](./SDK_GUIDE.md) and each SDK README for exact API
surfaces and limitations.

---

## Recipes

Cross-language assembly patterns live in [`recipes/`](../recipes/).

| Recipe | Backend | Frontend | SDK adoption |
|--------|---------|----------|--------------|
| [go-dart-holons](https://github.com/organic-programming/go-dart-holons) | Go | Flutter/Dart | daemon + frontend both on SDKs |
| [go-swift-holons](https://github.com/organic-programming/go-swift-holons) | Go | SwiftUI | daemon on SDK, frontend still raw `grpc-swift` |
| [go-kotlin-holons](https://github.com/organic-programming/go-kotlin-holons) | Go | Compose Desktop | daemon + frontend both on SDKs |
| [go-web-holons](https://github.com/organic-programming/go-web-holons) | Go | Web | daemon on SDK, frontend still raw web client |
| [go-qt-holons](https://github.com/organic-programming/go-qt-holons) | Go | Qt/C++ | daemon on SDK, frontend still raw client |
| [go-dotnet-holons](https://github.com/organic-programming/go-dotnet-holons) | Go | .NET MAUI | daemon + frontend both on SDKs |
| [rust-dart-holons](https://github.com/organic-programming/rust-dart-holons) | Rust | Flutter/Dart | frontend uses SDK, daemon still raw Rust |
| [rust-swift-holons](https://github.com/organic-programming/rust-swift-holons) | Rust | SwiftUI | both halves still raw |
| [rust-kotlin-holons](https://github.com/organic-programming/rust-kotlin-holons) | Rust | Compose Desktop | both halves still raw |
| [rust-web-holons](https://github.com/organic-programming/rust-web-holons) | Rust | Web | both halves still raw |
| [rust-dotnet-holons](https://github.com/organic-programming/rust-dotnet-holons) | Rust | .NET MAUI | both halves still raw |
| [rust-qt-holons](https://github.com/organic-programming/rust-qt-holons) | Rust | Qt/C++ | both halves still raw |

---

## Reference Implementation

**go-holons** remains the reference SDK. It currently provides the most
complete `serve` / `transport` / `identity` / `discover` / `connect`
surface and the canonical Holon-RPC implementation used by the rest of
the fleet for interop validation.
