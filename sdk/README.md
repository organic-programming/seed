# SDK Fleet

**Connect anything with anything.** A Go holon calls a Rust holon.
A Swift UI talks to a Python daemon. A C++ engine composes with a
Kotlin frontend. The language doesn't matter — the protocol does.

Every SDK implements the same 5 modules so that a holon written in
any language can discover, start, and talk to a holon written in any
other language, on any transport, without knowing or caring which
language the other side uses:

- **`serve`** — host a gRPC server with standard lifecycle
- **`transport`** — listen on `tcp://`, `unix://`, `stdio://`, `mem://`, `ws://`, `wss://`
- **`identity`** — read `holon.yaml` (name, slug, artifacts)
- **`discover`** — scan `OPPATH` to find holons by slug
- **`connect`** — resolve a slug, start the daemon if needed, return a ready gRPC channel

One line is all it takes: `connect("any-holon")` — the SDK finds the
binary, spawns it, wires gRPC over stdin/stdout, and hands back a
ready channel. No configuration, no port management, no glue code.

The remaining differences across SDKs are in `serve` depth and
Holon-RPC support.

---

## Fleet Overview

| SDK | Language | Serve | Discover | Connect | Holon-RPC | Version |
|-----|----------|-------|----------|---------|-----------|:-------:|
| [go-holons](https://github.com/organic-programming/go-holons) | Go | runner | ✅ | ✅ stdio | client + server | 0.3.0 |
| [js-holons](https://github.com/organic-programming/js-holons) | JavaScript (Node) | runner | ✅ | ✅ | client + server | 0.1.0 |
| [python-holons](https://github.com/organic-programming/python-holons) | Python | runner | ✅ | ✅ | client + server | 0.1.0 |
| [rust-holons](https://github.com/organic-programming/rust-holons) | Rust | flags only | ✅ | ✅ stdio | — | 0.1.0 |
| [swift-holons](https://github.com/organic-programming/swift-holons) | Swift | flags only | ✅ | ✅ stdio | client | 0.1.0 |
| [dart-holons](https://github.com/organic-programming/dart-holons) | Dart | flags only | ✅ | ✅ stdio | client + server | 0.1.0 |
| [kotlin-holons](https://github.com/organic-programming/kotlin-holons) | Kotlin | flags only | ✅ | ✅ | client | 0.1.0 |
| [java-holons](https://github.com/organic-programming/java-holons) | Java | flags only | ✅ | ✅ | client | 0.1.0 |
| [csharp-holons](https://github.com/organic-programming/csharp-holons) | C# | flags only | ✅ | ✅ | client | 0.1.0 |
| [cpp-holons](https://github.com/organic-programming/cpp-holons) | C++ | flags only | ✅ | ✅ | client | 0.1.0 |
| [c-holons](https://github.com/organic-programming/c-holons) | C | runner | ✅ | ✅ | wrapper binaries only | 0.1.0 |
| [ruby-holons](https://github.com/organic-programming/ruby-holons) | Ruby | flags only | ✅ | ✅ stdio | client | 0.1.0 |
| [js-web-holons](https://github.com/organic-programming/js-web-holons) | JavaScript (Browser) | browser | remote only | dial only | browser client | 0.1.0 |

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
| `ruby-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | server + client |
| `js-web-holons` | — | — | — | — | ✅ | browser client |

See [SDK_GUIDE.md](./SDK_GUIDE.md) and each SDK README for exact API
surfaces and limitations.

---

## Recipes

Cross-language assembly patterns live in [`recipes/`](../recipes/).

| Recipe | Backend | Frontend | Frontend uses `connect` |
|--------|---------|----------|:----------------------:|
| [go-dart-holons](https://github.com/organic-programming/go-dart-holons) | Go | Flutter/Dart | ✅ |
| [go-swift-holons](https://github.com/organic-programming/go-swift-holons) | Go | SwiftUI | ❌ raw grpc-swift |
| [go-kotlin-holons](https://github.com/organic-programming/go-kotlin-holons) | Go | Compose Desktop | ✅ |
| [go-web-holons](https://github.com/organic-programming/go-web-holons) | Go | Web | ✅ |
| [go-qt-holons](https://github.com/organic-programming/go-qt-holons) | Go | Qt/C++ | ✅ |
| [go-dotnet-holons](https://github.com/organic-programming/go-dotnet-holons) | Go | .NET MAUI | ✅ |
| [rust-dart-holons](https://github.com/organic-programming/rust-dart-holons) | Rust | Flutter/Dart | ✅ |
| [rust-swift-holons](https://github.com/organic-programming/rust-swift-holons) | Rust | SwiftUI | ❌ raw grpc-swift |
| [rust-kotlin-holons](https://github.com/organic-programming/rust-kotlin-holons) | Rust | Compose Desktop | ✅ |
| [rust-web-holons](https://github.com/organic-programming/rust-web-holons) | Rust | Web | ✅ |
| [rust-dotnet-holons](https://github.com/organic-programming/rust-dotnet-holons) | Rust | .NET MAUI | ✅ |
| [rust-qt-holons](https://github.com/organic-programming/rust-qt-holons) | Rust | Qt/C++ | ✅ |

---

## Reference Implementation

**go-holons** remains the reference SDK. It currently provides the most
complete `serve` / `transport` / `identity` / `discover` / `connect`
surface and the canonical Holon-RPC implementation used by the rest of
the fleet for interop validation.
