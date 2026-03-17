# SDK Fleet

**Connect anything with anything.** A Go holon calls a Rust holon.
A Swift UI talks to a Python daemon. A C++ engine composes with a
Kotlin frontend. The language doesn't matter — the protocol does.

Every SDK implements the same 5 modules so that a holon written in
any language can discover, start, and talk to a holon written in any
other language, on any transport, without knowing or caring which
language the other side uses:

- **`serve`** — host a gRPC server with standard lifecycle
- **`transport`** — listen on `tcp://`, `unix://`, `stdio://`, `mem://`, `ws://`, `wss://`, `rest+sse://`
- **`identity`** — read `holon.proto` (name, slug, artifacts)
- **`discover`** — scan `OPPATH` to find holons by slug
- **`connect`** — resolve a slug, start the daemon if needed, return a ready gRPC channel

One line is all it takes: `connect("any-holon")` — the SDK finds the
binary, spawns it, wires gRPC over stdin/stdout, and hands back a
ready channel. No configuration, no port management, no glue code.

The remaining differences across SDKs are in `serve` depth and
Holon-RPC support.

---

## SDK Classification

SDKs are classified by **role** — what they _do_ in an assembly,
not what they _could_ theoretically support.

### Daemon SDKs — serve holons as background processes

These SDKs host the full `serve` lifecycle. Holons written with
them run as daemons: they listen for gRPC (and eventually
REST+SSE), process requests, and are managed by `op`.

| SDK | Language | Serve | Describe | Version |
|-----|----------|-------|----------|:-------:|
| [go-holons](https://github.com/organic-programming/go-holons) | Go | runner | ✅ auto | 0.3.0 |
| [rust-holons](https://github.com/organic-programming/rust-holons) | Rust | flags only | — | 0.1.0 |
| [node-holons](https://github.com/organic-programming/js-holons) | Node.js | runner | ✅ auto | 0.1.0 |
| [python-holons](https://github.com/organic-programming/python-holons) | Python | runner | ✅ auto | 0.1.0 |
| [c-holons](https://github.com/organic-programming/c-holons) | C | runner | — | 0.1.0 |

### Frontend SDKs — drive native UIs, connect to daemons

These SDKs are embedded in **UI applications** (mobile, desktop).
They use `connect(slug)` to reach daemon holons and do **not**
serve RPCs themselves (though some _can_ if needed).

| SDK | Language | UI Framework | Connect | Version |
|-----|----------|--------------|---------|:-------:|
| [swift-holons](https://github.com/organic-programming/swift-holons) | Swift | **SwiftUI** | ✅ stdio | 0.1.0 |
| [dart-holons](https://github.com/organic-programming/dart-holons) | Dart | **Flutter** | ✅ stdio | 0.1.0 |
| [kotlin-holons](https://github.com/organic-programming/kotlin-holons) | Kotlin | **Compose** | ✅ | 0.1.0 |
| [csharp-holons](https://github.com/organic-programming/csharp-holons) | C# | **MAUI** | ✅ | 0.1.0 |
| [cpp-holons](https://github.com/organic-programming/cpp-holons) | C++ | **Qt** | ✅ | 0.1.0 |

### Full-Stack SDKs — both daemon and frontend

Some SDKs work on both sides. They can serve holons _and_
drive UIs with first-class framework support.

| SDK | Language | Daemon? | UI? | Note |
|-----|----------|---------|-----|------|
| **Swift** | Swift | ⚠️ possible (Vapor/NIO) | ✅ SwiftUI | macOS apps can serve; iOS cannot |
| **Dart** | Dart | ⚠️ possible (shelf) | ✅ Flutter | Primarily frontend |
| **Kotlin** | Kotlin | ⚠️ possible (Ktor) | ✅ Compose | Primarily frontend |
| **C#** | C# | ✅ Kestrel | ✅ MAUI | Both sides supported |
| **C++** | C++ | ✅ custom | ✅ Qt | Both sides supported |

> The Full-Stack table shows **capability**, not current usage.
> Today, Swift/Dart/Kotlin are used as frontends in recipes.

### Browser SDK — web client only

| SDK | Language | Serve | Connect |
|-----|----------|-------|---------|
| [js-web-holons](https://github.com/organic-programming/js-web-holons) | JS (Browser) | ❌ | dial only |

The browser cannot scan filesystems, spawn processes, or serve
gRPC. `js-web-holons` provides `connect(uri)` (direct dial)
and native `EventSource` for SSE streaming.

### Utility SDKs

| SDK | Language | Role | Version |
|-----|----------|------|:-------:|
| [java-holons](https://github.com/organic-programming/java-holons) | Java | Server-side alternative to Kotlin | 0.1.0 |
| [ruby-holons](https://github.com/organic-programming/ruby-holons) | Ruby | Scripting + automation | 0.1.0 |

---

## Holon-RPC (Describe)

`HolonMeta.Describe` is the self-documentation RPC auto-registered
by the SDK's `serve` runner. It is **not** a transport — it is a
service that runs _over_ any transport.

- **Server** (auto): daemon SDKs with a `serve` runner register
  `Describe` automatically by parsing `.proto` files at startup.
- **Client** (all): any SDK that calls `connect()` can invoke
  `Describe()` on the remote holon. This is how readiness
  verification works and how `op inspect` queries holons.

See [PROTO.md §5](../PROTO.md) and [PROTOCOL.md §3.5](../PROTOCOL.md).

---

## Transport Surface

All SDKs support the 7 transport schemes defined in PROTOCOL.md:
`tcp://`, `unix://`, `stdio://`, `mem://`, `ws://`, `wss://`,
`rest+sse://` (v0.6+).

| SDK | tcp | unix | stdio | mem | ws/wss | rest+sse |
|-----|:---:|:----:|:-----:|:---:|:------:|:--------:|
| `go-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | v0.6 |
| `js-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | v0.6 |
| `python-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | v0.6 |
| `rust-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | v0.6 |
| `swift-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | v0.6 client |
| `dart-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | v0.6 client |
| `kotlin-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | v0.6 client |
| `java-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | v0.6 |
| `csharp-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | v0.6 |
| `cpp-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | v0.6 |
| `c-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | v0.6 |
| `ruby-holons` | ✅ | ✅ | ✅ | ✅ | ✅ | v0.6 |
| `js-web-holons` | — | — | — | — | ✅ | v0.6 client |

See [SDK_GUIDE.md](./SDK_GUIDE.md) and each SDK README for exact API
surfaces and limitations.

---

## Recipes

Cross-language matrices now live directly in [`recipes/`](../recipes/)
instead of per-recipe git submodules.

- `recipes/assemblies/` contains the 48 Gudule greeting assemblies.
- `recipes/composition/` contains the 33 Charon compositions plus the 2
  shared worker holons.
- `recipes/testmatrix/gudule-greeting-testmatrix/` provides the reusable
  build-and-run matrix CLI.

Use [`../design/grace-op/v0.4/recipes.yaml`](../design/grace-op/v0.4/recipes.yaml)
for inventory and naming, and [`../recipes/README.md`](../recipes/README.md)
for current matrix notes and runnable examples.

| Recipe | Backend | Frontend | Frontend uses `connect` |
|--------|---------|----------|:----------------------:|
| [go-dart-holons](https://github.com/organic-programming/go-dart-holons) | Go | Flutter/Dart | ✅ |
| [go-swift-holons](https://github.com/organic-programming/go-swift-holons) | Go | SwiftUI | ✅ |
| [go-kotlin-holons](https://github.com/organic-programming/go-kotlin-holons) | Go | Kotlin Desktop | ✅ |
| [go-web-holons](https://github.com/organic-programming/go-web-holons) | Go | Web | ✅ |
| [go-qt-holons](https://github.com/organic-programming/go-qt-holons) | Go | Qt/C++ | ✅ |
| [go-dotnet-holons](https://github.com/organic-programming/go-dotnet-holons) | Go | .NET MAUI | ✅ |
| [rust-dart-holons](https://github.com/organic-programming/rust-dart-holons) | Rust | Flutter/Dart | ✅ |
| [rust-swift-holons](https://github.com/organic-programming/rust-swift-holons) | Rust | SwiftUI | ✅ |
| [rust-kotlin-holons](https://github.com/organic-programming/rust-kotlin-holons) | Rust | Kotlin Desktop | ✅ |
| [rust-web-holons](https://github.com/organic-programming/rust-web-holons) | Rust | Web | ✅ |
| [rust-dotnet-holons](https://github.com/organic-programming/rust-dotnet-holons) | Rust | .NET MAUI | ✅ |
| [rust-qt-holons](https://github.com/organic-programming/rust-qt-holons) | Rust | Qt/C++ | ✅ |

---

## Reference Implementation

**go-holons** remains the reference SDK. It currently provides the most
complete `serve` / `transport` / `identity` / `discover` / `connect`
surface and the canonical Holon-RPC implementation used by the rest of
the fleet for interop validation.
