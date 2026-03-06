# SDK Fleet

The SDK fleet provides language-native implementations of the Organic
Programming transport and protocol stack. Each SDK enables holons written
in its target language to **serve**, **dial**, and **compose** with any
other holon in the ecosystem — regardless of its implementation language.

All SDKs implement the same contract surface defined by
[PROTOCOL.md](../PROTOCOL.md) and are validated against the Go reference
implementation.

---

## Language SDKs

### Fleet Overview

| SDK | Language | Role | Valence | Version |
|-----|----------|:----:|:-------:|:-------:|
| [go-holons](https://github.com/organic-programming/go-holons) | Go | hub | multi | 0.3.0 |
| [js-holons](https://github.com/organic-programming/js-holons) | JavaScript (Node) | hub | multi | 0.1.0 |
| [python-holons](https://github.com/organic-programming/python-holons) | Python | hub | multi | 0.1.0 |
| [rust-holons](https://github.com/organic-programming/rust-holons) | Rust | hub | multi | 0.1.0 |
| [swift-holons](https://github.com/organic-programming/swift-holons) | Swift | hub | multi | 0.1.0 |
| [dart-holons](https://github.com/organic-programming/dart-holons) | Dart | hub | multi | 0.1.0 |
| [kotlin-holons](https://github.com/organic-programming/kotlin-holons) | Kotlin | hub | multi | 0.1.0 |
| [java-holons](https://github.com/organic-programming/java-holons) | Java | client | multi | 0.1.0 |
| [csharp-holons](https://github.com/organic-programming/csharp-holons) | C# | client | multi | 0.1.0 |
| [cpp-holons](https://github.com/organic-programming/cpp-holons) | C++ | client | multi | 0.1.0 |
| [c-holons](https://github.com/organic-programming/c-holons) | C | client | multi | 0.1.0 |
| [objc-holons](https://github.com/organic-programming/objc-holons) | Objective-C | client | multi | 0.1.0 |
| [ruby-holons](https://github.com/organic-programming/ruby-holons) | Ruby | client | multi | 0.1.0 |
| [js-web-holons](https://github.com/organic-programming/js-web-holons) | JavaScript (Browser) | client | mono | 0.1.0 |

**Role** — `hub` SDKs support all four dispatch modes (unicast, fanout,
broadcast-response, full-broadcast). `client` SDKs support unicast only.

**Valence** — `multi` = handles N concurrent connections. `mono` = one
connection per lifetime (browser sandbox constraint).

---

### gRPC Transport Matrix

Each SDK can **Listen** (act as a server) and **Dial** (act as a client)
on multiple transports. The table below shows current capability per SDK.

| SDK | Listen tcp | Listen stdio | Listen unix | Listen mem | Dial tcp | Dial stdio | Dial unix | Dial ws |
|-----|:----------:|:------------:|:-----------:|:----------:|:--------:|:----------:|:---------:|:-------:|
| go-holons | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| js-holons | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| python-holons | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| rust-holons | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| swift-holons | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| kotlin-holons | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| java-holons | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| csharp-holons | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| cpp-holons | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| c-holons | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | — | ✅ |
| objc-holons | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| ruby-holons | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| dart-holons | ✅ | ✅ | — | — | ✅ | ✅ | — | ✅ |
| js-web-holons | — | — | — | — | ✅ | ✅ | ✅ | — |

**Transport legend** — `tcp://` = TCP socket, `stdio://` = stdin/stdout
pipes, `unix://` = Unix domain socket, `mem://` = in-process bufconn,
`ws://` = gRPC-over-WebSocket.

---

### Holon-RPC Support

Holon-RPC is the JSON-RPC 2.0 over WebSocket protocol that enables
browser-facing and lightweight interoperability.

| SDK | Client | Server | Bidirectional |
|-----|:------:|:------:|:-------------:|
| go-holons | ✅ | ✅ | ✅ |
| js-holons | ✅ | ✅ | ✅ |
| python-holons | ✅ | ✅ | ✅ |
| rust-holons | ✅ | ✅ | — |
| swift-holons | ✅ | ✅ | ✅ |
| dart-holons | ✅ | ✅ | ✅ |
| kotlin-holons | ✅ | ✅ | ✅ |
| java-holons | ✅ | ✅ | ✅ |
| csharp-holons | ✅ | ✅ | ✅ |
| cpp-holons | ✅ | ✅ | ✅ |
| c-holons | ✅ | ✅ | ✅ |
| objc-holons | ✅ | ✅ | ✅ |
| ruby-holons | ✅ | ✅ | ✅ |
| js-web-holons | ✅ | ✅ | ✅ |

---

## Specialized SDKs

These SDKs address specific composition scenarios rather than a single
target language.

| SDK | Purpose | Status |
|-----|---------|:------:|
| [go-dart-holons](https://github.com/organic-programming/go-dart-holons) | Cross-stack composition — demonstrates a Go holon and a Dart holon composing into a single unit via serve/dial | draft |
| [go-cli-holonization](https://github.com/organic-programming/go-cli-holonization) | Patterns, documentation, and examples for adopting existing CLI tools as holons in Go (built on `go-holons`) | draft |

---

## Reference Implementation

**go-holons** is the reference SDK. It provides:

- The canonical echo server and client used for interoperability testing.
- The WebBridge — the embeddable Holon-RPC gateway for browser-facing interop.
- The transport implementations that all other SDKs are validated against.

All SDKs are tested for interoperability **against Go** — not against each
other. If every SDK talks correctly to Go, transitive interoperability
across the fleet is guaranteed.
