# Connect Specification

## What `connect` does

Resolves a holon by name, finds or starts its binary, and returns
a ready-to-use gRPC client channel. This is `op grpc://holon`
embedded in the SDK so any holon can call any other holon directly.

## Resolution logic

```
connect("rob-go") →
  1. If argument contains ":" → treat as host:port, dial directly
  2. Else → it's a holon slug
     a. discover() → find holon by slug
     b. Check if already running (port file: .op/run/<slug>.port)
     c. If running → dial whatever address the file advertises
     d. If not running → find binary, start with `serve --listen stdio://`
     e. Dial over stdio pipe (default) or TCP (explicit override)
  3. Return gRPC channel/connection
```

## API (same shape in every language)

```
connect(target: string) → GRPCChannel
connect(target: string, opts: ConnectOptions) → GRPCChannel

ConnectOptions:
  timeout:    duration      # how long to wait for server ready (default: 5s)
  transport:  string        # default: "stdio"; override: "tcp", "unix"
  start:      bool          # true = start if not running (default: true)
  port_file:  string        # override port file path

disconnect(channel: GRPCChannel) → void
```

> [!IMPORTANT]
> **Default transport is `stdio`.** This is non-negotiable.
> `connect("slug")` with no options MUST start the daemon with
> `serve --listen stdio://` and wire gRPC over the child's
> stdin/stdout pipes. TCP is an explicit opt-in via
> `ConnectOptions(transport: "tcp")`.

## Ephemeral vs persistent

- **Ephemeral** (default): `connect("slug")` starts the binary over
  stdio. `disconnect()` stops it. No port file is written because
  stdio has no addressable endpoint.
- **Persistent**: `connect("slug", {transport: "tcp"})` starts with
  TCP, writes a port file. `disconnect()` does NOT stop the server.
  Other callers reuse the port file.

## Port file convention

Port files are written **only for TCP or Unix** transports.

```
$ROOT/.op/run/<slug>.port
```

Contents: `tcp://localhost:<port>\n` or `unix://<path>\n`

Before starting: check if port file exists AND target responds.
If port file exists but target is dead, remove the stale file and
start fresh.

## Readiness check

After starting a daemon and establishing a connection, the SDK must
verify the daemon is actually serving before returning the channel.

Two acceptable strategies:
1. **gRPC connectivity state polling** — poll `GetState()` until
   `READY` (Go pattern).
2. **Describe probe** — call `HolonMeta/Describe` RPC with the
   connect timeout (Swift pattern). Heavier but proves the service
   responds.

All SDKs must use the same strategy within the same codebase.
The Go reference uses strategy 1.

## Implementation compliance matrix

Every SDK must pass 5 tests (see Testing section). Current state:

| SDK | Default | stdio impl | Tests |
|-----|:-------:|:----------:|:-----:|
| `go-holons` | ❌ tcp | ✅ exists | ⚠ tcp-only |
| `rust-holons` | ✅ stdio | ✅ exists | ✅ |
| `swift-holons` | ✅ stdio | ✅ exists | ? |
| `dart-holons` | ❌ tcp | ❌ missing | ⚠ tcp-only |
| `js-holons` | ❌ tcp | ✅ exists | ? |
| `python-holons` | ❌ tcp | ✅ exists | ? |
| `ruby-holons` | ✅ stdio | ✅ exists | ? |
| `kotlin-holons` | ? | ? | ? |
| `csharp-holons` | ? | ? | ? |
| `c-holons` | ? | ? | ? |
| `cpp-holons` | ? | ? | ? |
| `java-holons` | ? | ? | ? |
| `js-web-holons` | N/A | N/A | N/A |

## Testing pattern

For every SDK, these 5 tests must pass:

1. **Direct dial** — start a gRPC server on a known port,
   `connect("host:port")`, verify round-trip, disconnect.

2. **Slug via stdio** (default path) — create a holon fixture,
   `connect("slug")` with no options, verify the daemon was started
   with `stdio://`, verify gRPC round-trip, disconnect, verify
   process stopped.

3. **Slug via TCP** (explicit) — `connect("slug", {transport: "tcp"})`,
   verify port file written, disconnect, verify process stays alive.

4. **Port file reuse** — start a server manually, write a port file,
   `connect("slug")`, verify it reuses the existing server.

5. **Stale port file** — write a port file pointing at a dead address,
   `connect("slug")`, verify it removes the stale file and starts
   fresh.

## Cross-language integration test

```
# Rust holon connects to Go echo-server
let channel = holons::connect("echo-server").await?;
// send gRPC request, verify response
holons::disconnect(channel).await?;
```
