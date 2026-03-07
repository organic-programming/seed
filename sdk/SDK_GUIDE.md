# SDK Guide — Building and Composing Holons

Organic Programming SDKs are converging on the same five-module runtime
architecture:

| Module | Purpose |
|---|---|
| `serve` | Standard `serve` CLI surface. In the most complete SDKs this is a real runner; in the lighter SDKs it is currently flag parsing only. |
| `transport` | Parse transport URIs and expose runtime listeners/dialers. |
| `identity` | Parse `holon.yaml` identity and manifest fields. |
| `discover` | Scan local roots for visible holons. |
| `connect` | Resolve a slug or `host:port` target to a ready gRPC channel. |

`js-web-holons` is the browser-side exception: it provides Holon-RPC over
WebSocket plus manifest-fetch discovery helpers, but it cannot scan the
local filesystem or spawn processes.

---

## SDKs and `op`

| Capability | SDK | `op` |
|---|---|---|
| Serve a holon | `serve` | `op serve`, `op run` |
| Listen and dial transports | `transport` | `op grpc://...` dispatch |
| Read identity | `identity` | `op show`, `op list` |
| Find nearby holons | `discover` | `op discover` |
| Resolve and start another holon | `connect` | `op grpc://<slug>` |
| Build, install, test, clean | — | `op build`, `op install`, `op test`, `op clean` |

The SDK is the runtime substrate. `op` is the lifecycle and operator
tooling layered on top.

---

## Current Fleet Status

Identity parsing now exists across the fleet. The remaining differences
are mostly in how much `serve`, `connect`, and Holon-RPC each SDK exposes.

| SDK | Serve surface | Transport surface | Discover | Connect | Holon-RPC |
|---|---|---|---|---|---|
| `go-holons` | runner | full gRPC + ws bridge | ✅ | ✅ | client + server |
| `js-holons` | runner | full gRPC + ws bridge | ✅ | ✅ | client + server |
| `python-holons` | runner | TCP/Unix + mem + ws tunnel | ✅ | ✅ | client + server |
| `rust-holons` | flags only | tcp/unix/mem runtime, ws/wss metadata | ✅ | — | — |
| `swift-holons` | flags only | tcp/unix/stdio/mem runtime, ws/wss metadata | ✅ | — | client |
| `dart-holons` | flags only | tcp/unix/stdio/mem runtime, ws/wss metadata | ✅ | ✅ tcp-only | client + server |
| `kotlin-holons` | flags only | tcp/unix/mem runtime, stdio/ws metadata | ✅ | ✅ tcp-only | client |
| `java-holons` | flags only | tcp/unix/mem runtime, stdio/ws metadata | ✅ | ✅ tcp-only | client |
| `csharp-holons` | flags only | tcp/unix/mem runtime, stdio/ws metadata | ✅ | ✅ tcp-only | client |
| `cpp-holons` | flags only | tcp/unix/stdio/mem runtime, ws/wss metadata | ✅ | — | client |
| `c-holons` | runner | tcp/unix/stdio/mem runtime, ws/wss URI layer | ✅ | — | wrapper binaries only |
| `objc-holons` | flags only | tcp/unix/stdio/mem runtime, ws/wss metadata | ✅ | — | client |
| `ruby-holons` | flags only | tcp/unix/stdio/mem runtime, ws/wss metadata | ✅ | — | client |
| `js-web-holons` | browser | ws/wss Holon-RPC only | remote manifest only | — | browser client + node harness |

`runner` means the SDK can host the standard `serve` lifecycle itself.
`flags only` means it currently stops at `--listen` / `--port` parsing and
transport primitives.

---

## Hello-World Audit

The table below reflects the current workspace, not the intended end
state. Several hello-worlds are still raw gRPC baselines.

| Example | Matching SDK imported? | Note |
|---|---|---|
| [`go-hello-world`](../examples/go-hello-world) | ✅ | Uses `go-holons/pkg/serve`. |
| [`rust-hello-world`](../examples/rust-hello-world) | ❌ | Raw `tonic`; migration still pending. |
| [`dart-hello-world`](../examples/dart-hello-world) | ❌ | Raw gRPC baseline. |
| [`swift-hello-world`](../examples/swift-hello-world) | ✅ | Depends on `swift-holons` via SPM path dependency. |
| [`js-hello-world`](../examples/js-hello-world) | ✅ | Depends on `@organic-programming/holons`. |
| [`web-hello-world`](../examples/web-hello-world) | ✅ | Browser uses a synced copy of `js-web-holons`; backend uses `go-holons`. |
| [`kotlin-hello-world`](../examples/kotlin-hello-world) | ❌ | Raw gRPC baseline. |
| [`java-hello-world`](../examples/java-hello-world) | ❌ | Raw gRPC baseline. |
| [`csharp-hello-world`](../examples/csharp-hello-world) | ❌ | Raw gRPC baseline. |
| [`cpp-hello-world`](../examples/cpp-hello-world) | ❌ | Raw gRPC baseline. |
| [`c-hello-world`](../examples/c-hello-world) | ✅ | Uses `c-holons` transport and serve helpers. |
| [`python-hello-world`](../examples/python-hello-world) | ❌ | Raw `grpcio` baseline. |
| [`ruby-hello-world`](../examples/ruby-hello-world) | ❌ | Raw gRPC baseline. |
| [`objc-hello-world`](../examples/objc-hello-world) | ❌ | Raw gRPC baseline. |

---

## Recipe Audit

This table tracks whether each side of a recipe is actually using its
language SDK today.

| Recipe | Daemon SDK | Frontend SDK | Current launch path |
|---|---|---|---|
| [`go-dart-holons`](../recipes/go-dart-holons) | ✅ `go-holons` | ✅ `dart-holons` | Desktop uses `connect("greeting-daemon-greeting-godart")`; mobile uses `unix://`. |
| [`go-swift-holons`](../recipes/go-swift-holons) | ✅ `go-holons` | ❌ raw `grpc-swift` | Fixed localhost TCP. |
| [`go-kotlin-holons`](../recipes/go-kotlin-holons) | ✅ `go-holons` | ✅ `kotlin-holons` | Desktop uses `Connect.connect("greeting-daemon-greeting-gokotlin")`. |
| [`go-web-holons`](../recipes/go-web-holons) | ✅ `go-holons` | ❌ raw Connect-Web client | Browser reaches the daemon over HTTP/gRPC-Web style transport. |
| [`go-qt-holons`](../recipes/go-qt-holons) | ✅ `go-holons` | ❌ raw Qt/gRPC client | Fixed localhost TCP. |
| [`go-dotnet-holons`](../recipes/go-dotnet-holons) | ✅ `go-holons` | ✅ `csharp-holons` | Desktop uses `Holons.ConnectTarget("greeting-daemon-greeting-godotnet")`. |
| [`rust-dart-holons`](../recipes/rust-dart-holons) | ❌ raw Rust daemon | ✅ `dart-holons` | Fixed localhost TCP with Dart-managed daemon lifecycle. |
| [`rust-swift-holons`](../recipes/rust-swift-holons) | ❌ raw Rust daemon | ❌ raw `grpc-swift` | Fixed localhost TCP. |
| [`rust-kotlin-holons`](../recipes/rust-kotlin-holons) | ❌ raw Rust daemon | ❌ raw JVM gRPC client | Fixed localhost TCP. |
| [`rust-web-holons`](../recipes/rust-web-holons) | ❌ raw Rust daemon | ❌ raw web client | Browser reaches the daemon over localhost HTTP. |
| [`rust-qt-holons`](../recipes/rust-qt-holons) | ❌ raw Rust daemon | ❌ raw Qt/gRPC client | Fixed localhost TCP. |
| [`rust-dotnet-holons`](../recipes/rust-dotnet-holons) | ❌ raw Rust daemon | ❌ raw .NET gRPC client | Fixed localhost TCP. |

---

## Getting Started

### Go

```go
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

### C#

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
