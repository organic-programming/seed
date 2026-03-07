# TODO: `connect` Module — All SDKs

Depends on: `TODO_DISCOVER.md` (discover must exist first).

## What `connect` does

Resolves a holon by name, finds or starts its binary, and returns
a ready-to-use gRPC client channel. This is what `op grpc://holon`
does — but embedded in the SDK so any holon can call any other
holon directly.

## Resolution logic (same as `op`)

```
connect("rob-go") →
  1. If argument contains ":" → treat as host:port, dial directly
  2. Else → it's a holon slug
     a. discover() → find holon by slug
     b. Check if already running (port file: .op/run/<slug>.port)
     c. If not running → find binary, start with `serve --listen tcp://:0`
     d. Read allocated port from stdout or port file
     e. Dial grpc://localhost:<port>
  3. Return gRPC channel/connection
```

## API (same shape in every language)

```
connect(target: string) → GRPCChannel
connect(target: string, opts: ConnectOptions) → GRPCChannel

ConnectOptions:
  timeout:    duration      # how long to wait for server ready
  transport:  string        # override: "tcp", "unix", "stdio"
  start:      bool          # true = start if not running (default true)
  port_file:  string        # override port file path

disconnect(channel: GRPCChannel) → void
  # if we started the holon, stop it
```

### Ephemeral vs persistent

- **Ephemeral**: `connect("rob-go")` starts the binary, uses it,
  `disconnect()` stops it. Default for one-shot calls.
- **Persistent**: `connect("rob-go", {start: true})` starts and
  writes a port file. Other holons can find it via the port file.
  `disconnect()` does NOT stop the server.

## Port file convention

When a holon is started by `connect`, write:

```
$ROOT/.op/run/<slug>.port
```

Contents: `tcp://localhost:<port>\n`

Before starting a holon, check if port file exists AND if the
process is alive. If port file exists but process is dead, remove
the stale file and start fresh.

## Implementation per SDK

### Tier 1 — SDKs with recipe consumers

#### `go-holons` → `pkg/connect/connect.go`

```go
func Connect(target string) (*grpc.ClientConn, error)
func ConnectWithOpts(target string, opts ConnectOptions) (*grpc.ClientConn, error)
func Disconnect(conn *grpc.ClientConn) error
```

- Use `os/exec.Command` to start holon binary with `serve --listen tcp://:0`.
- Parse port from binary's stderr (`"listening on tcp://:NNNN"`).
- Use `google.golang.org/grpc.Dial`.
- Track started processes in a package-level map for cleanup.

#### `rust-holons` → `src/connect.rs`

```rust
pub async fn connect(target: &str) -> Result<tonic::transport::Channel>
pub async fn connect_with_opts(target: &str, opts: ConnectOptions) -> Result<Channel>
pub async fn disconnect(channel: Channel) -> Result<()>
```

- Use `tokio::process::Command` to start holon binary.
- Use `tonic::transport::Channel::from_shared`.

#### `dart-holons` → `lib/src/connect.dart`

```dart
Future<ClientChannel> connect(String target);
Future<void> disconnect(ClientChannel channel);
```

- Use `dart:io Process.start` to launch binary.
- Use `package:grpc` `ClientChannel`.

#### `swift-holons` → `Sources/Holons/Connect.swift`

```swift
public func connect(_ target: String) throws -> GRPCChannel
public func disconnect(_ channel: GRPCChannel) throws
```

- Use `Foundation.Process` to launch binary.
- Use `grpc-swift` `ClientConnection`.

### Tier 2 — SDKs with hello-world examples

#### `js-holons` → `src/connect.js`

```javascript
export async function connect(target) → GrpcClient
export async function disconnect(client) → void
```

- Use `child_process.spawn` to launch binary.
- Use `@grpc/grpc-js` `credentials.createInsecure()`.

#### `js-web-holons` → `src/connect.mjs`

**Note**: browser JS cannot spawn processes. `connect()` in the
web SDK only supports `host:port` mode — it dials an already
running server. No ephemeral launch.

```javascript
export function connect(hostPort) → GrpcWebClient
```

#### `kotlin-holons` → `src/main/kotlin/holons/Connect.kt`

```kotlin
suspend fun connect(target: String): ManagedChannel
suspend fun disconnect(channel: ManagedChannel)
```

- Use `ProcessBuilder` to launch binary.
- Use `io.grpc.ManagedChannelBuilder`.

#### `csharp-holons` → `Holons/Connect.cs`

```csharp
public static GrpcChannel Connect(string target)
public static void Disconnect(GrpcChannel channel)
```

- Use `System.Diagnostics.Process` to launch binary.
- Use `Grpc.Net.Client.GrpcChannel.ForAddress`.

#### `python-holons` → `holons/connect.py`

```python
def connect(target: str) -> grpc.Channel
def disconnect(channel: grpc.Channel) -> None
```

- Use `subprocess.Popen` to launch binary.
- Use `grpcio` `grpc.insecure_channel`.

#### `ruby-holons` → `lib/holons/connect.rb`

```ruby
module Holons
  def self.connect(target) → GRPC::Core::Channel
  def self.disconnect(channel)
end
```

- Use `Process.spawn` to launch binary.
- Use `grpc` gem channel.

### Tier 3 — Native SDKs (C, C++, Obj-C)

#### `c-holons` → `src/connect.c`

```c
grpc_channel *holons_connect(const char *target);
void holons_disconnect(grpc_channel *channel);
```

- Use `fork`/`exec` to launch binary.
- Use core `grpc_insecure_channel_create`.

#### `cpp-holons` → `include/holons/connect.hpp`

```cpp
namespace holons {
  std::shared_ptr<grpc::Channel> connect(const std::string& target);
  void disconnect(std::shared_ptr<grpc::Channel> channel);
}
```

- Use `std::system` or `popen` for process launch.
- Use `grpc::CreateChannel`.

#### `objc-holons`

```objc
+ (GRPCChannel *)connect:(NSString *)target;
+ (void)disconnect:(GRPCChannel *)channel;
```

- Use `NSTask` to launch binary.

#### `java-holons` → `src/main/java/holons/Connect.java`

```java
public static ManagedChannel connect(String target)
public static void disconnect(ManagedChannel channel)
```

- Use `ProcessBuilder`.
- Use `io.grpc.ManagedChannelBuilder`.

## Testing pattern

For every SDK:

1. **Direct dial test**: start `cmd/echo-server-go` manually on a
   known port, call `connect("localhost:PORT")`, verify channel
   works, disconnect.

2. **Slug resolution test** (requires discover):
   - Create a temp tree with a holon that has a built binary.
   - Call `connect("slug")`.
   - Verify the binary was started (check PID).
   - Verify gRPC channel is functional.
   - Call `disconnect()`.
   - Verify the process was stopped (ephemeral mode).

3. **Port file test**:
   - Start a holon manually, write a port file.
   - Call `connect("slug")`.
   - Verify it reuses the existing server (no new process).

4. **Stale port file test**:
   - Write a port file with a dead PID.
   - Call `connect("slug")`.
   - Verify it cleans up the stale file and starts fresh.

## Cross-language integration test

The ultimate test: a holon built with SDK X calls a holon built
with SDK Y. The existing `cmd/echo-server-go` + `cmd/echo-client-go`
fixtures in most SDKs already do this — they prove Go↔Language
interop. The `connect` module makes this automatic.

```
# Rust holon connects to Go echo-server
let channel = holons::connect("echo-server").await?;
// send gRPC request, verify response
holons::disconnect(channel).await?;
```
