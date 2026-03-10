# SDK Guide — Building and Composing Holons

## What an SDK Gives You

An SDK is the **native runtime** for holons in a given language. It
replaces hand-rolled gRPC boilerplate with standardized modules:

```
sdk/<lang>-holons
├── serve        Start a gRPC server, handle signals, shut down gracefully
├── transport    Parse transport URIs (tcp, unix, stdio, mem, ws, wss)
├── identity     Read holon.yaml — name, UUID, artifacts, build metadata
├── discover     Scan the local filesystem for nearby holons
├── connect      Resolve a holon by name → find it → start it → dial it
└── describe     Self-documentation — auto-registered HolonMeta.Describe RPC
```

`describe` is auto-registered by the `serve` runner — holon developers
do not implement it. Any caller can invoke `Describe()` to get a
human-readable API catalog, even when gRPC reflection is disabled.
See [PROTOCOL.md §3.5](../PROTOCOL.md) for the full proto definition.

### The key primitive: `connect`

Without the SDK, calling another holon requires knowing its address,
starting it manually, and wiring up a raw gRPC channel. With the SDK:

```
connect("rob-go")          →  discover → start → dial → ready channel
connect("localhost:9090")  →  dial directly (address bypass)
```

One call. The SDK handles discovery, process lifecycle, port allocation,
and cleanup on disconnect. See
[Constitution, Article 11](../AGENT.md#connect--name-based-resolution).

`js-web-holons` is the browser exception: it provides Holon-RPC over
WebSocket plus manifest-fetch discovery, but cannot scan filesystems or
spawn processes. Its `connect` is direct-dial only.

---

## Getting Started

### Go

```go
// Serve — standard holon entry point
import (
    "os"
    "github.com/organic-programming/go-holons/pkg/serve"
    "google.golang.org/grpc"
)

func main() {
    uri := serve.ParseFlags(os.Args[1:])
    _ = serve.Run(uri, func(server *grpc.Server) {
        // register generated protobuf services here
    })
}
```

```go
// Connect — call another holon by name
import "github.com/organic-programming/go-holons/pkg/connect"

conn, err := connect.Connect("rob-go")
defer connect.Disconnect(conn)
// use conn as a *grpc.ClientConn
```

### Rust

```rust
use holons::{discover, serve, transport};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let uri = serve::parse_flags(&std::env::args().skip(1).collect::<Vec<_>>());
    let _listener = transport::listen(&uri).await?;
    let nearby = discover::discover_local()?;
    println!("nearby holons: {}", nearby.len());
    Ok(())
}
```

### Dart

```dart
import 'package:holons/holons.dart';

// Discover nearby holons and connect to one
final entries = await discoverLocal();
final channel = await connect('greeting-daemon-greeting-godart');
try {
  print(entries.map((e) => e.slug).toList());
} finally {
  await disconnect(channel);
}
```

### Swift

```swift
import Holons

let uri = Serve.parseFlags(Array(CommandLine.arguments.dropFirst()))
let entries = try discoverLocal()
print(uri)
print(entries.map(\.slug))
```

### Python

```python
from holons.connect import connect, disconnect
from holons.discover import discover_local

entries = discover_local()
channel = connect("greeting-daemon-greeting-godotnet")
try:
    print([entry.slug for entry in entries])
finally:
    disconnect(channel)
```

### Kotlin

```kotlin
import org.organicprogramming.holons.Connect
import org.organicprogramming.holons.Discover
import kotlinx.coroutines.runBlocking

runBlocking {
    val entries = Discover.discoverLocal()
    val channel = Connect.connect("greeting-daemon-greeting-gokotlin")
    try {
        println(entries.map { it.slug })
    } finally {
        Connect.disconnect(channel)
    }
}
```

### C# (.NET)

```csharp
using Holons;

var entries = Discover.DiscoverLocal();
var channel = Connect.ConnectTarget("greeting-daemon-greeting-godotnet");
try
{
    Console.WriteLine(string.Join(", ", entries.ConvertAll(entry => entry.Slug)));
}
finally
{
    Connect.Disconnect(channel);
}
```

### C

```c
#include <holons/holons.h>

// Parse flags and start serving
const char *uri = holons_parse_flags(argc - 1, argv + 1);
holons_listener lis = holons_listen(uri);
holons_serve(lis, my_handler, NULL);
```

### Ruby

```ruby
require 'holons'

entries = Holons.discover_local
entries.each { |e| puts e.slug }
```

---

## SDK vs `op`

The SDK is the holon's own runtime. `op` is the operator tooling layered
on top — it orchestrates, but is never required at runtime.

| Capability | SDK | `op` |
|---|---|---|
| Serve a holon | `serve` | `op serve`, `op run` |
| Listen and dial transports | `transport` | `op grpc://...` dispatch |
| Read identity | `identity` | `op show`, `op list` |
| Find nearby holons | `discover` | `op discover` |
| Resolve and start another holon | `connect` | `op grpc://<slug>` |
| Build, install, test, clean | — | `op build`, `op install`, `op test`, `op clean` |

---

## Fleet Status

| SDK | Serve | Discover | Connect | Holon-RPC |
|---|---|---|---|---|
| `go-holons` | runner | ✅ | ✅ | client + server |
| `js-holons` | runner | ✅ | ✅ | client + server |
| `python-holons` | runner | ✅ | ✅ | client + server |
| `rust-holons` | flags only | ✅ | — | — |
| `swift-holons` | flags only | ✅ | — | client |
| `dart-holons` | flags only | ✅ | ✅ tcp-only | client + server |
| `kotlin-holons` | flags only | ✅ | ✅ tcp-only | client |
| `java-holons` | flags only | ✅ | ✅ tcp-only | client |
| `csharp-holons` | flags only | ✅ | ✅ tcp-only | client |
| `cpp-holons` | flags only | ✅ | — | client |
| `c-holons` | runner | ✅ | — | wrapper binaries only |
| `ruby-holons` | flags only | ✅ | — | client |
| `js-web-holons` | browser | remote manifest only | — | browser client + node harness |

`runner` = SDK hosts the full serve lifecycle.
`flags only` = SDK parses `--listen` / `--port` and provides transport primitives.

---

## Hello-World Audit

| Example | Uses its SDK? | Note |
|---|---|---|
| `go-hello-world` | ✅ | Uses `go-holons/pkg/serve`. |
| `rust-hello-world` | ❌ | Raw `tonic`; migration pending. |
| `dart-hello-world` | ❌ | Raw gRPC baseline. |
| `swift-hello-world` | ✅ | Depends on `swift-holons` via SPM. |
| `js-hello-world` | ✅ | Depends on `@organic-programming/holons`. |
| `web-hello-world` | ✅ | Browser uses `js-web-holons`; backend uses `go-holons`. |
| `kotlin-hello-world` | ❌ | Raw gRPC baseline. |
| `java-hello-world` | ❌ | Raw gRPC baseline. |
| `csharp-hello-world` | ❌ | Raw gRPC baseline. |
| `cpp-hello-world` | ❌ | Raw gRPC baseline. |
| `c-hello-world` | ✅ | Uses `c-holons` transport and serve helpers. |
| `python-hello-world` | ❌ | Raw `grpcio` baseline. |
| `ruby-hello-world` | ❌ | Raw gRPC baseline. |

---

## Recipe Audit

| Recipe | Daemon SDK | Frontend SDK | Current state |
|---|---|---|---|
| `go-dart-holons` | ✅ `go-holons` | ✅ `dart-holons` | Desktop uses `connect(slug)`; mobile uses `unix://`. |
| `go-swift-holons` | ✅ `go-holons` | ❌ raw `grpc-swift` | Fixed localhost TCP. |
| `go-kotlin-holons` | ✅ `go-holons` | ✅ `kotlin-holons` | Desktop uses `Connect.connect(slug)`. |
| `go-web-holons` | ✅ `go-holons` | ❌ raw web client | Browser over HTTP/gRPC-Web. |
| `go-qt-holons` | ✅ `go-holons` | ❌ raw Qt client | Fixed localhost TCP. |
| `go-dotnet-holons` | ✅ `go-holons` | ✅ `csharp-holons` | Desktop uses `Holons.ConnectTarget(slug)`. |
| `rust-dart-holons` | ❌ raw Rust | ✅ `dart-holons` | Fixed localhost TCP. |
| `rust-swift-holons` | ❌ raw Rust | ❌ raw `grpc-swift` | Fixed localhost TCP. |
| `rust-kotlin-holons` | ❌ raw Rust | ❌ raw JVM gRPC | Fixed localhost TCP. |
| `rust-web-holons` | ❌ raw Rust | ❌ raw web client | Localhost HTTP. |
| `rust-qt-holons` | ❌ raw Rust | ❌ raw Qt client | Fixed localhost TCP. |
| `rust-dotnet-holons` | ❌ raw Rust | ❌ raw .NET gRPC | Fixed localhost TCP. |
