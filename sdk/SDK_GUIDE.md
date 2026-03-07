# SDK Guide — Building and Composing Holons

The SDK fleet is the **native runtime** for holons. Each SDK gives
a holon everything it needs to exist, communicate, and compose with
other holons — in any language, on any platform, with or without
`op`.

`op` is a facility. The SDK is the foundation.

---

## What an SDK provides

An SDK is not a protocol adapter. It is a complete toolkit for
building autonomous holons. Every SDK exposes five modules:

| Module | Purpose | Analogy |
|---|---|---|
| **serve** | Run a gRPC server with standard transport parsing | The holon's **mouth** — how it speaks |
| **transport** | Listen and dial on TCP, Unix, stdio, mem, WebSocket | The holon's **ears and voice** — how it connects |
| **identity** | Read and write `holon.yaml` — the holon's civil status | The holon's **passport** |
| **discover** | Scan the filesystem for other holons | The holon's **eyes** — how it finds others |
| **connect** | Resolve a holon name, start it if needed, dial gRPC | The holon's **hands** — how it reaches out |

The first three (serve, transport, identity) are already
implemented in most SDKs. The last two (discover, connect) are
being added — see `TODO_DISCOVER.md` and `TODO_CONNECT.md`.

---

## 1. Serve — running a holon

Every holon is a gRPC server. The SDK's `serve` module provides the
standard `serve` command handler with flag parsing, transport
binding, and graceful shutdown.

### Pattern (all languages)

```
binary serve --listen <transport-URI>
```

### Go

```go
import "github.com/organic-programming/go-holons/pkg/serve"

case "serve":
    listenURI := serve.ParseFlags(os.Args[2:])
    serve.Run(listenURI, func(s *grpc.Server) {
        pb.RegisterMyServiceServer(s, &myServer{})
    })
```

### Rust

```rust
use holons::serve;
use holons::transport;

let uri = serve::parse_flags(&args);
let listener = transport::listen(&uri).await?;
```

### Swift

```swift
import Holons
let uri = Holons.parseFlags(args)
try Holons.serve(uri: uri) { server in
    server.addService(MyServiceProvider())
}
```

### Python

```python
from holons.serve import parse_flags, run
uri = parse_flags(sys.argv[1:])
run(uri, [MyServiceServicer()])
```

The SDK handles:
- `--listen` and `--port` flag parsing
- Transport URI resolution (`tcp://`, `unix://`, `stdio://`, `mem://`)
- Signal handling (SIGINT/SIGTERM)
- Graceful shutdown with timeout

Without the SDK, each holon would reimplement this — and they did,
until now. See the 6 Rust daemons with identical 30-line
boilerplate.

---

## 2. Transport — the universal wire

Every SDK can listen and dial on multiple transports. A holon built
with `rust-holons` can serve over Unix sockets and a holon built
with `dart-holons` can dial it — the transport is transparent.

### Supported transports

| URI scheme | What it is | When to use |
|---|---|---|
| `tcp://:9090` | TCP socket | Default, cross-machine |
| `unix:///tmp/holon.sock` | Unix domain socket | Same-machine, faster than TCP |
| `stdio://` | stdin/stdout pipes | Parent-child processes (recipes) |
| `mem://` | In-process buffer | Unit testing, embedded holons |
| `ws://host:8080/grpc` | gRPC-over-WebSocket | Browser clients |
| `wss://host:443/grpc` | gRPC-over-WebSocket (TLS) | Secure browser clients |

### Listen (server side)

```go
// Go
listener, _ := transport.Listen("unix:///tmp/my.sock")
```

```rust
// Rust
let listener = transport::listen("tcp://:9090").await?;
```

### Dial (client side)

```go
// Go
conn, _ := transport.Dial("tcp://localhost:9090")
```

```python
# Python
channel = transport.dial("unix:///tmp/my.sock")
```

The same holon binary can serve on TCP in production and stdio in a
recipe — without code changes. Only the `--listen` argument differs.

---

## 3. Identity — the holon's civil status

Every holon has a `holon.yaml` manifest. The SDK reads and writes it.

```yaml
# holon.yaml
schema: 1
uuid: c7f3a1b2-...
given_name: rob
family_name: go
motto: "Go toolchain wrapper"
composer: "B. ALTER"
clade: system
status: living
born: "2026-03-02"
kind: wrapper
```

### Reading identity

```go
// Go
id, _ := identity.ReadFile("holon.yaml")
fmt.Println(id.Slug())  // "rob-go"
```

```rust
// Rust
let id = holons::identity::read("holon.yaml")?;
println!("{}", id.slug());  // "rob-go"
```

```python
# Python
from holons.identity import read
id = read("holon.yaml")
print(id.slug)  # "rob-go"
```

The SDK enforces naming conventions: the slug is
`<given_name>-<family_name>`, lowercased, hyphenated. It must match
the directory name and (for native/wrapper holons) `artifacts.binary`.

---

## 4. Discover — finding other holons

A holon can scan the filesystem to find other holons. No `op`
needed. This is the holon's own awareness of its environment.

```go
// Go — find all holons visible from the current directory
entries, _ := discover.DiscoverLocal()
for _, h := range entries {
    fmt.Printf("%s (%s) at %s\n", h.Slug, h.UUID, h.Dir)
}
```

```rust
// Rust — find a specific holon by slug
if let Some(entry) = holons::discover::find_by_slug("marco-atlas")? {
    println!("Found at {}", entry.dir.display());
}
```

```python
# Python — discover all holons from a root
from holons.discover import discover
for h in discover("/path/to/project"):
    print(f"{h.slug} → {h.dir}")
```

### Scan rules

- Recursive walk from the given root
- Skips `.git`, `.op`, `node_modules`, `vendor`, `build`, dotfiles
- Parses each `holon.yaml` found
- UUID dedup: closest to root wins
- Search order: local root → `$OPBIN` → `$OPPATH/cache/`

---

## 5. Connect — reaching other holons

The most powerful primitive. A holon can connect to any other holon
by name — the SDK handles resolution, process management, and gRPC
channel creation.

```go
// Go holon calling a Rust holon — no op, no hardcoded port
conn, _ := connect.Connect("marco-atlas")
defer connect.Disconnect(conn)
client := atlaspb.NewAtlasServiceClient(conn)
resp, _ := client.Resolve(ctx, &atlaspb.ResolveRequest{...})
```

```rust
// Rust holon calling a Go holon
let channel = holons::connect("rob-go").await?;
let mut client = GoServiceClient::new(channel);
```

```swift
// Swift UI connecting to its daemon
let channel = try Holons.connect("gudule-daemon-greeting-goswift")
let client = GreetingServiceClient(channel: channel)
```

```dart
// Flutter app connecting to its daemon
final channel = await connect('gudule-daemon-greeting-godart');
final client = GreetingServiceClient(channel);
```

### Resolution logic

`connect("rob-go")` does:

1. If target contains `:` → it's `host:port`, dial directly
2. Otherwise it's a holon slug →
   a. Check port file (`.op/run/rob-go.port`) — is it already
      running?
   b. If running → dial the existing server
   c. If not → discover the holon, find its binary, start it
      with `serve --listen tcp://:0`, read the allocated port,
      dial
3. Return a ready-to-use gRPC channel

### Why this matters

Without `connect`, a Flutter app knows its daemon is at
`localhost:9091` because someone hardcoded it. With `connect`,
the app says "I need `gudule-daemon`" and the SDK finds it,
starts it if needed, and hands back a channel. The app doesn't
know or care where the daemon runs.

This is **autonomous composition**. Holons find and connect to
each other like organisms in an ecosystem — not like components
wired together by a central orchestrator.

---

## How SDKs relate to `op`

| Capability | SDK | `op` |
|---|---|---|
| Serve gRPC | `serve` module | `op serve`, `op run` |
| Listen/Dial transports | `transport` module | `op grpc://` dispatch |
| Read identity | `identity` module | `op show`, `op list` |
| Find holons | `discover` module | `op discover` |
| Connect to holons | `connect` module | `op grpc://holon` |
| Build holons | ❌ (not SDK's job) | `op build` |
| Install binaries | ❌ (not SDK's job) | `op install` |
| Manage dependencies | ❌ (not SDK's job) | `op mod` |

**The SDK provides runtime primitives.** `op` provides lifecycle
management (build, test, install, deploy). A holon needs the SDK
to run. It needs `op` to be built and managed — but once running,
it is self-sufficient.

---

## SDK maturity

| SDK | serve | transport | identity | discover | connect | Holon-RPC |
|---|---|---|---|---|---|---|
| go-holons | ✅ | ✅ | ❌ | 🔜 | 🔜 | ✅ |
| rust-holons | ✅ | ✅ | ✅ | 🔜 | 🔜 | ✅ |
| swift-holons | ✅ | ✅ | ✅ | 🔜 | 🔜 | ✅ |
| dart-holons | ✅ | ✅ | ❓ | 🔜 | 🔜 | ✅ |
| python-holons | ✅ | ✅ | ✅ | 🔜 | 🔜 | ✅ |
| js-holons | ✅ | ✅ | ❓ | 🔜 | 🔜 | ✅ |
| js-web-holons | ✅ | ✅ | ❓ | ⚠️ | ⚠️ | ✅ |
| kotlin-holons | ✅ | ✅ | ❓ | 🔜 | 🔜 | ✅ |
| java-holons | ✅ | ✅ | ❓ | 🔜 | 🔜 | ✅ |
| csharp-holons | ✅ | ✅ | ✅ | 🔜 | 🔜 | ✅ |
| cpp-holons | ✅ | ✅ | ❓ | 🔜 | 🔜 | ✅ |
| c-holons | ✅ | ✅ | ❓ | 🔜 | 🔜 | ✅ |
| ruby-holons | ✅ | ✅ | ✅ | 🔜 | 🔜 | ✅ |
| objc-holons | ✅ | ✅ | ❓ | 🔜 | 🔜 | ✅ |

✅ implemented, ❓ needs verification, 🔜 planned, ⚠️ limited
(browser cannot spawn processes or scan filesystems).

---

## Hello-world examples

Each SDK has a matching hello-world in [`examples/`](../examples/).
These are the simplest working holons — one per language.

| Example | SDK | What it does |
|---|---|---|
| [`go-hello-world`](../examples/go-hello-world) | `sdk/go-holons` | Go gRPC server, uses `pkg/serve` |
| [`rust-hello-world`](../examples/rust-hello-world) | `sdk/rust-holons` | Rust gRPC server (raw tonic — migration pending) |
| [`dart-hello-world`](../examples/dart-hello-world) | `sdk/dart-holons` | Dart gRPC server |
| [`swift-hello-world`](../examples/swift-hello-world) | `sdk/swift-holons` | Swift gRPC server via SPM |
| [`js-hello-world`](../examples/js-hello-world) | `sdk/js-holons` | Node.js gRPC server |
| [`web-hello-world`](../examples/web-hello-world) | `sdk/js-web-holons` | Browser-based gRPC-Web client + Go backend |
| [`kotlin-hello-world`](../examples/kotlin-hello-world) | `sdk/kotlin-holons` | Kotlin/JVM gRPC server |
| [`java-hello-world`](../examples/java-hello-world) | `sdk/java-holons` | Java gRPC server |
| [`csharp-hello-world`](../examples/csharp-hello-world) | `sdk/csharp-holons` | C# gRPC server (.NET) |
| [`cpp-hello-world`](../examples/cpp-hello-world) | `sdk/cpp-holons` | C++ gRPC server (CMake) |
| [`c-hello-world`](../examples/c-hello-world) | `sdk/c-holons` | C gRPC server (Makefile) |
| [`python-hello-world`](../examples/python-hello-world) | `sdk/python-holons` | Python gRPC server |
| [`ruby-hello-world`](../examples/ruby-hello-world) | `sdk/ruby-holons` | Ruby gRPC server |
| [`objc-hello-world`](../examples/objc-hello-world) | `sdk/objc-holons` | Objective-C gRPC server |

---

## Recipes — cross-language composition

Recipes are composite holons that combine a backend daemon and a
frontend UI, communicating over gRPC. Each recipe is a working
example of cross-language holon composition.

See [`recipes/`](../recipes/) for the full list.

| Recipe | Backend | Backend SDK | Frontend | Frontend SDK | Transport |
|---|---|---|---|---|---|
| [`go-dart-holons`](../recipes/go-dart-holons) | Go | `go-holons` ✅ | Flutter (Dart) | `dart-holons` ✅ | stdio |
| [`go-swift-holons`](../recipes/go-swift-holons) | Go | `go-holons` ✅ | SwiftUI | `swift-holons` ❌ | stdio |
| [`go-kotlin-holons`](../recipes/go-kotlin-holons) | Go | `go-holons` ✅ | Kotlin/JVM | `kotlin-holons` ❌ | TCP |
| [`go-web-holons`](../recipes/go-web-holons) | Go | `go-holons` ✅ | Web (JS) | `js-web-holons` ❌ | gRPC-Web |
| [`go-qt-holons`](../recipes/go-qt-holons) | Go | `go-holons` ✅ | Qt/C++ | `cpp-holons` ❌ | TCP |
| [`go-dotnet-holons`](../recipes/go-dotnet-holons) | Go | `go-holons` ✅ | .NET MAUI | `csharp-holons` ❌ | TCP |
| [`rust-dart-holons`](../recipes/rust-dart-holons) | Rust | `rust-holons` ❌ | Flutter (Dart) | `dart-holons` ✅ | stdio |
| [`rust-swift-holons`](../recipes/rust-swift-holons) | Rust | `rust-holons` ❌ | SwiftUI | `swift-holons` ❌ | stdio |
| [`rust-kotlin-holons`](../recipes/rust-kotlin-holons) | Rust | `rust-holons` ❌ | Kotlin/JVM | `kotlin-holons` ❌ | TCP |
| [`rust-web-holons`](../recipes/rust-web-holons) | Rust | `rust-holons` ❌ | Web (JS) | `js-web-holons` ❌ | gRPC-Web |
| [`rust-qt-holons`](../recipes/rust-qt-holons) | Rust | `rust-holons` ❌ | Qt/C++ | `cpp-holons` ❌ | TCP |
| [`rust-dotnet-holons`](../recipes/rust-dotnet-holons) | Rust | `rust-holons` ❌ | .NET MAUI | `csharp-holons` ❌ | TCP |

✅ = uses its SDK. ❌ = uses raw gRPC, SDK adoption pending.

Each recipe lives in `recipes/<name>/examples/greeting/` and
follows the Gudule Greeting pattern: the daemon exposes a
`GreetingService` and the frontend calls it to display a greeting
in 56 languages.

---

## Getting started

### Build a holon in Go

```go
package main

import (
    "os"
    "github.com/organic-programming/go-holons/pkg/serve"
    pb "your/holon/gen/go/v1"
    "google.golang.org/grpc"
)

func main() {
    switch os.Args[1] {
    case "serve":
        uri := serve.ParseFlags(os.Args[2:])
        serve.Run(uri, func(s *grpc.Server) {
            pb.RegisterYourServiceServer(s, &yourImpl{})
        })
    }
}
```

### Build a holon in Rust

```rust
use holons::serve;
use holons::transport;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let args: Vec<String> = std::env::args().skip(1).collect();
    match args.first().map(|s| s.as_str()) {
        Some("serve") => {
            let uri = serve::parse_flags(&args[1..]);
            let listener = transport::listen(&uri).await?;
            // setup tonic server with listener
        }
        _ => eprintln!("usage: holon <serve> [flags]"),
    }
    Ok(())
}
```

### Connect holons together

```go
// A Go holon discovering and calling a Rust holon
import "github.com/organic-programming/go-holons/pkg/connect"

conn, _ := connect.Connect("rob-go")
defer connect.Disconnect(conn)

client := robpb.NewGoServiceClient(conn)
result, _ := client.Build(ctx, &robpb.BuildRequest{...})
```

No `op`. No hardcoded ports. Just holons talking to holons.
