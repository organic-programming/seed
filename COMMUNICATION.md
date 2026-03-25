# Holon Communication Protocol

> *"Every holon must be reachable."* — Constitution, Article 11
> *"Invent only what is truly new; for everything else, imitate what works."* — Constitution, Article 9

This document is the single, authoritative reference for how holons
communicate. It covers the transport layer, name-based resolution
(Connect), both RPC bindings (gRPC and JSON-RPC 2.0), error codes,
and operational concerns.

---

## 1. Architecture Overview

Holons communicate through two complementary RPC bindings, each with
its own set of transports:

```
┌─────────────────────────────────────────────────────────────┐
│                       Applications                          │
├─────────────────────────────────────────────────────────────┤
│                  ACL · Routing (planned)                    │
├──────────────────────────────┬──────────────────────────────┤
│       gRPC Binding           │    JSON-RPC 2.0 Binding      │
│      (protobuf)              │                              │
├──────────────────────────────┼──────────────────────────────┤
│  tcp://                      │   ws://                      │
│  unix://                     │   wss://                     │
│  stdio://                    │   http://  (REST + SSE)      │
│  ws://   (gRPC-over-WS)      │   https:// (REST + SSE)      │
│  wss://  (gRPC-over-WS)      │                              │
└──────────────────────────────┴──────────────────────────────┘
```

Transports differ in two structural properties beyond mere
connectivity — **valence** (how many simultaneous peers) and
**duplex** (can both sides send at the same time) — see §2.7.

| Binding | Wire format | Use case | Dependency |
|---------|-------------|----------|------------|
| **gRPC** | Protobuf | Holon-to-holon (high performance, local/LAN) | gRPC library required |
| **JSON-RPC 2.0** | JSON-RPC 2.0 over WebSocket or HTTP+SSE | Internet, browser, mobile, scripting | WebSocket or HTTP + JSON |

---

## 2. Transport Layer

### 2.1 Transport URIs

Every holon accepts a `--listen` flag specifying where to bind.
Transport URIs follow the scheme `scheme://authority`:

| Scheme | Description | Default | Mandatory |
|--------|-------------|---------|:---------:|
| `tcp://` | TCP socket | `tcp://:9090` | ✅ Article 11 |
| `unix://` | Unix domain socket (POSIX) | `unix:///tmp/holon.sock` | optional |
| `stdio://` | Standard input/output pipes | — | ✅ Article 11 |

| `ws://` | WebSocket (unencrypted) | `ws://:8080/api/v1/rpc` | optional |
| `wss://` | WebSocket (TLS) | `wss://:8443/api/v1/rpc` | optional |
| `http://` | HTTP (request/response + SSE) | `http://:8080/api/v1/rpc` | optional |
| `https://` | HTTP over TLS (request/response + SSE) | `https://:8443/api/v1/rpc` | optional |

### 2.2 Listen — Server-Side Binding

`Listen(uri) → Listener` opens a real OS-level socket (or pipe)
and returns something a gRPC server can accept connections on.

- **Input**: a transport URI.
- **Output**: a bound listener with an `Accept()` method.
- **Role**: the holon acts as a **server** — it waits for incoming connections.

```go
// Go reference
lis, err := transport.Listen("tcp://:9090")
grpcServer.Serve(lis)
```

### 2.3 Dial — Client-Side Connection

`Dial(uri) → Connection` connects to a remote holon's listener
and returns a gRPC client channel ready for RPC calls.

- **Input**: a transport URI (or address).
- **Output**: a gRPC client connection.
- **Role**: the holon acts as a **client** — it initiates calls to another holon.

```go
// Go reference
conn, err := grpcclient.Dial("tcp://otherholon:9090")
client := echopb.NewEchoClient(conn)
```

### 2.4 Serve — Wiring Listen to gRPC

`Serve` is the standard entry point that combines Listen, gRPC
server setup, `HolonMeta` self-documentation, and signal handling
into a single call.
See [Constitution, Article 11](./CONSTITUTION.md#article-11--the-serve--dial-convention).

```go
// Go reference
serve.Run(func(s *grpc.Server) {
    echopb.RegisterEchoServer(s, &myServer{})
})
```

### 2.5 Parse — URI Metadata (No I/O)

`Parse(uri) → metadata` extracts scheme, host, port, path from
a transport URI without opening any socket. This is the minimum
a SDK must implement before adding real Listen/Dial capabilities.

### 2.6 Scheme Details

#### `tcp://[host]:port`

Standard TCP socket. Default bind address: all interfaces (`:9090`).
Most portable scheme — works on every OS.

#### `unix://path`

Unix domain socket. Fast, zero-copy local IPC. POSIX only.
The socket file is created at the specified path.

#### `stdio://`

Redirects gRPC over standard input (read) and standard output (write).
Enables holon composition via process pipes:

```bash
holon-a --listen stdio:// | holon-b --connect stdio://
```

#### `ws://host:port/path` and `wss://host:port/path`

WebSocket transport. Two distinct uses:

1. **gRPC tunnel** — wraps gRPC binary frames inside WebSocket frames.
   The WebSocket is transparent; gRPC operates normally.
2. **JSON-RPC 2.0** — carries JSON-RPC 2.0 messages (see §5).
   No gRPC involvement.

The path distinguishes the two: convention is `/api/v1/grpc` for tunneled gRPC
and `/api/v1/rpc` for JSON-RPC 2.0. This is a recommendation, not a requirement.

#### `http://host:port/path` and `https://host:port/path`

HTTP transport for the JSON-RPC 2.0 binding. Two mechanisms:

1. **Request/response** — `POST /api/v1/rpc/<package.Service>/<Method>` with a
   JSON request body; server returns a JSON response.
2. **Server-push** — `GET /api/v1/rpc/<package.Service>/<Method>` with
   `Accept: text/event-stream` opens an SSE stream for server-streaming
   RPCs.

Unlike WebSocket, HTTP+SSE does **not** support bidirectional calls —
the server cannot invoke methods on the client. It is well suited for
environments where WebSocket upgrade is blocked (proxies, CDNs,
corporate firewalls) or where standard HTTP caching is desired.

### 2.7 Transport Properties

Transports differ in two structural properties that constrain how
holons can associate:

**Valence** — the maximum number of simultaneous connections a
transport supports:

- **Monovalent**: exactly one active connection per lifetime.
  The transport is a point-to-point pipe.
- **Multivalent**: N concurrent connections. The transport can
  serve or reach multiple peers simultaneously.

A holon using a multivalent transport **may** choose to operate
as monovalent (accept one connection, reject others). Valence is
a structural maximum, not an obligation.

**Duplex** — whether both sides of a connection can send
simultaneously:

- **Full-duplex**: both sides send and receive concurrently
  and independently. No turn-taking.
- **Simulated full-duplex**: two unidirectional channels glued
  together (e.g., separate stdin/stdout pipes). Behaves like
  full-duplex for RPC framing, but the underlying mechanism
  is two simplex streams.

#### Per-Scheme Properties

| Scheme | Valence | Duplex | Notes |
|--------|:-------:|:------:|-------|
| `tcp://` | Multi | Full | N concurrent connections, bidirectional byte stream |
| `unix://` | Multi | Full | Same as TCP — bidirectional, POSIX only |
| `stdio://` | **Mono** | **Simulated** | One pair of pipes per process lifetime |
| `ws://` | Multi | Full | WebSocket is explicitly full-duplex (RFC 6455 §1.1) |
| `wss://` | Multi | Full | Same as `ws://` with TLS |
| `http://` | Multi | **Simulated** | POST + SSE — request/response + server-push |
| `https://` | Multi | **Simulated** | Same as `http://` with TLS |

---

## 3. Connect — Name-Based Resolution

### 3.1 Overview

`Dial` (§2.3) requires a concrete transport address. In practice,
callers rarely know that address in advance — they know a **holon name**.
The `connect` primitive bridges that gap: given a slug, it discovers
the holon, starts it if necessary, dials it, and returns a ready gRPC
channel.

```
connect("rob-go")         → discover → start → dial → ready channel
connect("localhost:9090") → dial directly (host:port bypass)
```

Connect is an **SDK facility**, not a protocol extension — the wire
format is unchanged. It enables autonomous holon-to-holon composition
without `op` as an intermediary.

> [!IMPORTANT]
> Using raw gRPC dialing (e.g., hardcoded `localhost:9091`) is a
> **failure pattern**. Raw dials bypass readiness polling, dynamic
> port assignment, and automatic cleanup. Always use the SDK's
> `connect(slug)`.

### 3.2 Resolution Algorithm

All SDK implementations **must** follow the same resolution logic.

```
connect(target)
  │
  ├─ 1. Direct target?
  │     if target contains ":" or "://" → skip to step 7
  │
  ├─ 2. Discover by slug
  │     → scan OPPATH for holon.proto matching the slug
  │     → resolve holon directory
  │
  ├─ 3. Port file check (tcp/unix only)
  │     → look for .op/run/<slug>.port
  │     → if file exists AND target responds to probe → step 7
  │     → if dead → remove stale file, continue
  │
  ├─ 4. Resolve binary
  │     → check artifacts.binary in manifest
  │     → search <holon_dir>/.op/build/bin/
  │     → fallback to system PATH
  │
  ├─ 5. Launch daemon
  │     → spawn binary as child process
  │     → stdio (default): serve --listen stdio://
  │     → tcp  (opt-in):   serve --listen tcp://127.0.0.1:0
  │
  ├─ 6. Parse advertisement (tcp only)
  │     → capture stdout/stderr for first URI line
  │     → for stdio the URI is simply stdio://
  │
  ├─ 7. dialReady(URI)
  │     → open gRPC channel
  │     → await readiness verification (see §3.3)
  │
  └─ 8. Cache connection (tcp/unix only)
        → write .op/run/<slug>.port
        → stdio has no addressable endpoint, never cached
```

### 3.3 Readiness Verification

When a holon is started as a child process, the gRPC listener may
not be ready immediately. The SDK **must** verify readiness before
returning the channel to the caller.

| Strategy | Description | SDKs |
|----------|-------------|------|
| **Connectivity polling** | Poll `GetState()` until `READY` | Go, Dart, Kotlin |
| **RPC probe** | Call `HolonMeta/Describe` with the connect timeout | Swift |

The RPC probe is **recommended** for new SDK implementations — it
confirms that the service layer is responsive and processing protobuf
requests, not just that the socket is open.

**Requirements:**

- **Retry with backoff** — poll every ~100 ms with a sensible timeout
  (default 5 s).
- **Handle ephemeral ports** — if the daemon was started on port `0`,
  parse the advertised URI from stdout/stderr before probing.
- **Descriptive errors** — e.g. "holon exited before becoming ready"
  or "timed out waiting for readiness".
- **Zombie prevention** — if the readiness probe fails, kill the child
  process to prevent leaking half-started daemons.

### 3.4 Transport Cascade

When `connect` starts a holon (ephemeral mode), it uses a cascade
ordered by efficiency:

| Priority | Transport | Rationale |
|:--------:|-----------|----------|
| **1** | `stdio://` | Default — parent owns child's pipes, zero overhead |
| **2** | `tcp://127.0.0.1:0` | Explicit override via `ConnectOptions` |

`stdio://` is the default because it is the universal baseline (see
§2.6). The pipe is already wired at spawn time: no port allocation,
no loopback TCP overhead, no race conditions.

When `connect` finds an **already-running** holon (via step 3's port
file), there is no cascade — it dials whatever address the file
advertises.

### 3.5 Disconnect and Lifecycle

`Disconnect` reverses the process:

1. Close the gRPC channel.
2. If the SDK started the holon (ephemeral mode), stop the child
   process (SIGTERM → grace period → SIGKILL).
3. Remove the `.op/run/<slug>.port` file if it was created.

The SDK **must not** leak OS resources (child processes, file
descriptors) across connect/disconnect cycles — this aligns with
the robustness requirements in §7.4.2.

---

## 4. gRPC Binding

### 4.1 Overview

The primary holon-to-holon protocol. Uses [gRPC](https://grpc.io/)
with Protocol Buffers for serialization. Over TCP, gRPC uses its
standard HTTP/2 framing. Over stdio, gRPC uses its native
length-prefixed wire format (5-byte header: 1 compression flag +
4-byte big-endian message length, then the serialized protobuf).

Every holon that supports the gRPC binding **must**:

1. Accept a `--listen` flag for the transport URI.
2. Register the `HolonMeta.Describe` service
   (Constitution, Article 2 — Self-documentation).
3. Handle `SIGTERM` / `SIGINT` for graceful shutdown.

### 4.2 Service Definition

Services are defined using `.proto` files following Protocol Buffers
[style guide](https://protobuf.dev/programming-guides/style/):

```protobuf
syntax = "proto3";
package echo.v1;

service Echo {
  rpc Ping(PingRequest) returns (PingResponse);
}

message PingRequest {
  string message = 1;
}

message PingResponse {
  string message = 1;
  string sdk     = 2;
  string version = 3;
}
```

### 4.3 Supported Transports

The gRPC binding operates over `tcp://`, `unix://`, and `stdio://`.
It can also be tunneled through `ws://` / `wss://` where WebSocket wraps
the raw HTTP/2 stream.

### 4.4 References

| Document | URL |
|----------|-----|
| gRPC Documentation | [grpc.io](https://grpc.io/) |
| Protocol Buffers | [protobuf.dev](https://protobuf.dev/) |
| gRPC Status Codes | [grpc.io/docs/guides/status-codes](https://grpc.io/docs/guides/status-codes/) |

### 4.5 HolonMeta Service

Every holon **must** register the `HolonMeta` service alongside its
own domain services. The SDK auto-registers it when using the standard
`serve` runner — holon developers do not implement it manually.

`HolonMeta` provides a single RPC — `Describe` — that returns the
holon's full manifest and API catalog as a typed protobuf response.
The response includes enough type information for a caller to
dynamically construct and send requests without compiled stubs.

> [!IMPORTANT]
> gRPC reflection (`grpc.reflection.v1alpha.ServerReflection`) is
> **not required** and **must be disabled by default**. The `Describe`
> service provides all information needed for dynamic dispatch,
> including field numbers and types. SDKs **may** offer an opt-in
> flag (e.g. `--reflect`) as a development convenience for third-party
> tools like `grpcurl`.

**Canonical proto files** (the single source of truth):

- [`describe.proto`](./holons/grace-op/_protos/holons/v1/describe.proto) — `HolonMeta` service, `DescribeRequest/Response`, `ServiceDoc`, `MethodDoc`, `FieldDoc`
- [`manifest.proto`](./holons/grace-op/_protos/holons/v1/manifest.proto) — `HolonManifest` (identity, skills, sequences, contract, build, artifacts)
- [`coax.proto`](./holons/grace-op/_protos/holons/v1/coax.proto) — `CoaxService` (organism-level member discovery, lifecycle, and `Tell`)

Key design points:

- `DescribeResponse` wraps the full `HolonManifest` (identity, skills,
  build metadata) alongside a `repeated ServiceDoc` API catalog.
- `HolonMeta` is excluded from its own `Describe` output — the
  response documents only the holon's domain services.
- If the holon has no parseable `.proto` files, `Describe` returns
  a response with the manifest and an empty `services` list.
- Nested message fields are recursively expanded in `FieldDoc.nested_fields`
  up to a reasonable depth (the SDK may cap recursion).

#### External Bridges: `op inspect` and `op mcp`

JSON Schema generation, MCP tool definitions, and LLM function-calling
formats are **not** the holon's responsibility. `op` handles these
externally by reading the holon's `.proto` files:

- `op inspect <slug>` — rich offline API documentation from protos/
- `op mcp <slug>` — MCP server bridge exposing RPCs as MCP tools
- `op tools <slug>` — LLM tool definitions in any format

See [OP.md §15](./holons/grace-op/OP.md) for details.

### 4.6 Dynamic Dispatch Workflow

Any client that dispatches gRPC calls dynamically (without compiled
stubs) **must** use the `Describe` service as its schema source. This
applies to `op`, SDK `connect` modules, custom tools, and any other
client in the ecosystem.

> [!IMPORTANT]
> This workflow is **mandatory** for dynamic dispatch. It replaces
> gRPC reflection as the canonical schema-discovery mechanism.

#### Workflow

```
Client connects to holon
  │
  ├─ 1. Call HolonMeta/Describe         (one round-trip, ~5 KB)
  │     → DescribeResponse: services[], methods[], FieldDocs
  │
  ├─ 2. Cache the DescribeResponse       (schema is static)
  │
  ├─ 3. Receive dispatch: method + JSON payload
  │     → look up method in cached response
  │     → read input_fields: [{name, number, type, label}, ...]
  │
  ├─ 4. Build protobuf bytes dynamically
  │     → map each JSON key to its FieldDoc
  │     → encode using field number + wire type
  │     → handle nested_fields recursively
  │
  ├─ 5. Send the gRPC call
  │     → path: /package.Service/Method
  │
  └─ 6. Deserialize the response
        → use cached output_fields to decode protobuf → JSON
```

#### Why Describe, Not Reflection

| Property | `Describe` | gRPC Reflection |
|----------|:----------:|:---------------:|
| Protocol | Works over gRPC **and** JSON-RPC 2.0 | gRPC only |
| Schema data | field numbers, types, labels | raw file descriptors |
| Human-readable | ✅ descriptions, examples, `@required` | ❌ binary proto descriptors |
| Agent-friendly | ✅ structured, flat, cacheable | ❌ requires protobuf compiler |
| Holon-controlled | ✅ can exclude internal RPCs | ❌ exposes everything |
| One round-trip | ✅ single unary call | ❌ bidirectional stream |

#### Type Mapping

The `FieldDoc.type` string maps directly to protobuf wire types:

| `type` | Wire type | Encoding |
|--------|:---------:|----------|
| `string` | 2 (length-delimited) | UTF-8 bytes |
| `bytes` | 2 (length-delimited) | raw bytes |
| `int32`, `int64` | 0 (varint) | signed varint |
| `uint32`, `uint64` | 0 (varint) | unsigned varint |
| `sint32`, `sint64` | 0 (varint) | zigzag varint |
| `bool` | 0 (varint) | 0 or 1 |
| `float` | 5 (32-bit) | IEEE 754 |
| `double` | 1 (64-bit) | IEEE 754 |
| `fixed32`, `sfixed32` | 5 (32-bit) | little-endian |
| `fixed64`, `sfixed64` | 1 (64-bit) | little-endian |
| `<package.MessageType>` | 2 (length-delimited) | recursive, use `nested_fields` |
| `<package.EnumType>` | 0 (varint) | enum number from `enum_values` |

For `repeated` fields (`label = FIELD_LABEL_REPEATED`), scalar types
use packed encoding (a single length-delimited field containing
concatenated values). Message types repeat the field tag.

For `map` fields (`label = FIELD_LABEL_MAP`), encode as repeated
messages with `key` (field 1) and `value` (field 2), using the types
from `map_key_type` and `map_value_type`.

---

## 5. JSON-RPC 2.0 Binding

### 5.1 Overview

The JSON-RPC 2.0 binding uses standard
[JSON-RPC 2.0](https://www.jsonrpc.org/specification) exactly as specified.
No custom protocol is introduced. Three conventions adapt it to the
organic ecosystem:

1. **Method naming** — `package.Service/Method` (gRPC style).
2. **ID namespacing** — server-originated IDs prefixed with `s`.
3. **Subprotocol** — WebSocket handshake uses `holon-rpc`.

Any compliant JSON-RPC 2.0 library can produce and consume messages
without modification.

**Design principles:**

1. **No invention** — reuse JSON-RPC 2.0 exactly as specified.
2. **Symmetric** — over WebSocket, both sides can initiate calls.
3. **Lightweight** — no gRPC dependency, no protobuf, no HTTP/2.
4. **Universal** — any language with WebSocket or HTTP + JSON can participate.

### 5.2 When to Use

The JSON-RPC 2.0 binding exists for environments where gRPC is
unavailable or impractical:

- **Browsers** — no HTTP/2 trailer support.
- **Mobile apps** — lighter than a full gRPC dependency.
- **Scripting languages** — PHP, Ruby, etc. where gRPC setup is heavy.
- **Internet-facing access** — WebSocket and HTTP traverse NATs and proxies.

### 5.3 Transport Binding

The JSON-RPC 2.0 binding supports two transports:

#### 5.3.1 WebSocket Transport

Messages are exchanged as **WebSocket text frames** (opcode `0x1`).
Each frame contains exactly one JSON-RPC 2.0 message.

The WebSocket handshake **must** include the subprotocol `holon-rpc`:

```
GET /api/v1/rpc HTTP/1.1
Upgrade: websocket
Sec-WebSocket-Protocol: holon-rpc
```

The server **must** confirm the subprotocol in its response.
If the server does not confirm `holon-rpc`, the client **should** close
the connection with status `1002` (Protocol Error).

WebSocket is **full-duplex** — either side can initiate calls at any
time (see §5.6). This is the preferred transport when bidirectional
communication is needed.

#### 5.3.2 HTTP+SSE Transport

For environments where WebSocket upgrade is blocked (proxies, CDNs,
corporate firewalls), the binding also operates over plain HTTP:

- **Requests** — `POST /api/v1/rpc/<package.Service>/<Method>` with a JSON
  request body. The server returns a JSON response. The method is
  identified by the URL path, not the body.
- **Server-push** — `GET /api/v1/rpc/<package.Service>/<Method>` with
  `Accept: text/event-stream` opens an SSE stream for server-streaming
  RPCs. Each event is a JSON-encoded response message.

HTTP+SSE is **not bidirectional** — the server cannot invoke methods
on the client. It supports unary requests and server-streaming only.

| Verb | URL pattern | Body | Use |
|------|-------------|------|-----|
| `POST` | `/api/v1/rpc/<package.Service>/<Method>` | JSON request | Unary RPC |
| `GET` | `/api/v1/rpc/<package.Service>/<Method>` | — (query params) | Server-streaming via SSE |

> **Note**: unlike the WebSocket transport (which uses JSON-RPC 2.0
> message framing), the HTTP+SSE transport uses **direct per-method
> routes**. This gives better access-log visibility, standard
> URL-based rate limiting, and simpler infrastructure integration.

##### Content-Type

| Direction | Header |
|-----------|--------|
| Request (POST) | `Content-Type: application/json` |
| Response (POST) | `Content-Type: application/json` |
| Response (GET SSE) | `Content-Type: text/event-stream` |

##### HTTP Status Codes

The server **must** return standard HTTP status codes. JSON-RPC error
objects are still included in the body when applicable.

| Situation | HTTP Status | Body |
|-----------|:-----------:|------|
| Success (unary) | `200` | JSON response |
| Method not found | `404` | JSON-RPC error (`code: 5`) |
| Invalid JSON / bad request | `400` | JSON-RPC error (`code: -32700` or `-32600`) |
| Internal error | `500` | JSON-RPC error (`code: 13`) |
| SSE stream opened | `200` | `text/event-stream` |

##### Server-Streaming Requests

Server-streaming RPCs use `Accept: text/event-stream` to open an SSE
stream. Both verbs are supported:

| Verb | When to use | Payload |
|------|-------------|---------|
| `POST` | Request has a body (recommended) | JSON body, same as unary |
| `GET` | Request has no fields or only simple scalars | Query params (`?key=value`) |

Both **must** include `Accept: text/event-stream`. The server returns
`Content-Type: text/event-stream` and begins streaming.

```
POST /api/v1/rpc/build.v1.BuildService/WatchBuild
Accept: text/event-stream
Content-Type: application/json

{"project": "myapp", "filter": {"status": "active"}}
```

```
GET /api/v1/rpc/build.v1.BuildService/WatchBuild?project=myapp
Accept: text/event-stream
```

##### SSE Event Format

Each SSE event carries one JSON response message. The format follows
the [SSE specification](https://html.spec.whatwg.org/multipage/server-sent-events.html):

```
event: message
id: 1
data: {"jsonrpc":"2.0","id":"c42","result":{"status":"building","progress":42}}

event: message
id: 2
data: {"jsonrpc":"2.0","id":"c42","result":{"status":"done","progress":100}}

event: error
data: {"jsonrpc":"2.0","id":"c42","error":{"code":13,"message":"build failed"}}

```

| SSE field | Value | Required |
|-----------|-------|:--------:|
| `event` | `message` for results, `error` for errors | ✅ |
| `id` | Monotonically increasing integer (1, 2, 3…) | ✅ |
| `data` | One complete JSON-RPC 2.0 response object per line | ✅ |

- The `id` field enables `Last-Event-ID` reconnection per the SSE spec.
- The stream ends with an `event: done` event (empty `data:`), after
  which the server closes the connection.
- If the client disconnects, the server **must** cancel the
  underlying RPC.

```
event: done
data:

```

##### CORS (Browser Clients)

Servers exposing HTTP+SSE **must** handle CORS for browser access:

| Header | Value |
|--------|-------|
| `Access-Control-Allow-Origin` | Configurable (default: request `Origin`) |
| `Access-Control-Allow-Methods` | `GET, POST, OPTIONS` |
| `Access-Control-Allow-Headers` | `Content-Type, Accept, Last-Event-ID` |
| `Access-Control-Max-Age` | `86400` (24 h) |

Preflight `OPTIONS` requests **must** return `204` with the headers
above. The allowed origin **should** be configurable per listener —
`*` is acceptable for development but **must not** be the production
default.

#### URL Convention

```
WebSocket:
  ws://host:port/api/v1/rpc             — all methods via JSON-RPC body
  wss://host:port/api/v1/rpc            — TLS

HTTP+SSE:
  POST  https://host/api/v1/rpc/greeting.v1.GreetingService/SayHello
  GET   https://host/api/v1/rpc/build.v1.BuildService/WatchBuild
```

### 5.4 Wire Format

JSON-RPC 2.0 exactly as specified in
[jsonrpc.org/specification](https://www.jsonrpc.org/specification).

#### Request

```json
{
  "jsonrpc": "2.0",
  "id": "c42",
  "method": "echo.v1.Echo/Ping",
  "params": {
    "message": "hello"
  }
}
```

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `jsonrpc` | `"2.0"` | ✅ | Protocol version (JSON-RPC 2.0) |
| `id` | string or number | ✅ | Caller-assigned correlation ID |
| `method` | string | ✅ | Method name (see §4.5) |
| `params` | object | optional | Request payload. Always an object, never an array. |

#### Response (success)

```json
{
  "jsonrpc": "2.0",
  "id": "c42",
  "result": {
    "message": "hello",
    "sdk": "go",
    "version": "0.3.0"
  }
}
```

#### Response (error)

```json
{
  "jsonrpc": "2.0",
  "id": "c42",
  "error": {
    "code": 5,
    "message": "method not found"
  }
}
```

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `error.code` | integer | ✅ | Error code (see §5) |
| `error.message` | string | ✅ | Human-readable description |
| `error.data` | any | optional | Additional error details |

#### Notification (fire-and-forget)

```json
{
  "jsonrpc": "2.0",
  "method": "telemetry.v1.Telemetry/Event",
  "params": {
    "event": "page_view",
    "path": "/dashboard"
  }
}
```

A notification has **no `id` field**. The receiver **must not** send
a response. Useful for telemetry, logging, and events.

#### Batch (optional)

JSON-RPC 2.0 supports [batch requests](https://www.jsonrpc.org/specification#batch).
Implementations **may** support batching. If unsupported, the receiver
**should** respond with error code `-32600` (Invalid Request).

### 5.5 Method Naming Convention

Methods follow gRPC's `package.Service/Method` convention:

```
echo.v1.Echo/Ping
hello.v1.HelloService/Greet
ui.v1.UIService/GetViewport
```

This is a **convention**, not a protocol requirement. Any valid
JSON-RPC method string is accepted.

#### Reserved Methods

Methods prefixed with `rpc.` are reserved for protocol-level operations:

| Method | Direction | Description |
|--------|-----------|-------------|
| `rpc.discover` | client → server | List available methods (optional) |
| `rpc.heartbeat` | either direction | Keep-alive (see §7) |
| `rpc.peers` | client → hub | List connected peer IDs and their registered methods. Returns `{ "peers": [{ "id": "…", "methods": ["…"] }] }`. Required for `_target` unicast addressing (see §5.9). |
| `rpc.resume` | client → server | *Reserved — not yet specified.* Intent: reconnect session continuity (resume from a sequence number). |
| `rpc.transfer` | either direction | *Reserved — not yet specified.* Intent: out-of-band large payload negotiation (exchange a URI/hash instead of inline data). |

### 5.6 Bidirectional Calls (WebSocket Only)

Over WebSocket, the JSON-RPC 2.0 binding is **fully symmetric**: once
connected, either holon can call remote methods registered on the
other. The concepts of "client" and "server" refer only to who
initiated the connection — after the handshake, both peers are equal.

> **Note**: bidirectionality is orthogonal to valence. A monovalent
> link (e.g., `stdio://`) is still symmetric — both sides can call
> each other — but it connects exactly one pair. A multivalent
> transport adds the 1:N dimension, not the bidirectional one.

> **Note**: HTTP+SSE does **not** support bidirectional calls —
> the server cannot invoke methods on the client. Use WebSocket
> when server-initiated RPC is required.

#### ID Namespacing

To prevent ID collisions in bidirectional communication:

- **Client-originated IDs**: any string or number chosen by the client.
- **Server-originated IDs**: **must** be prefixed with `s` (e.g., `"s1"`, `"s2"`).

This convention is **mandatory** for the WebSocket transport to enable
safe bidirectional multiplexing. Over HTTP+SSE, only client-originated
IDs are used.

#### Handler Registration

Both sides **should** support registering handlers for incoming calls:

```javascript
// Client side (e.g., browser)
client.register("ui.v1.UIService/GetViewport", async (params) => {
    return { width: 1920, height: 1080 };
});
```

```go
// Server side (e.g., Go)
bridge.Register("echo.v1.Echo/Ping", func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
    // handle...
})
```

#### Message Dispatch

When a message is received:

1. If `method` is present → **request** (or notification if no `id`).
2. If `result` or `error` is present → **response** to a pending call.

### 5.7 Roles

The JSON-RPC 2.0 binding has two distinct roles:

- **Server**: accepts WebSocket and/or HTTP connections, dispatches
  incoming JSON calls to handlers. Over WebSocket, the server can also
  invoke methods on connected clients. Over HTTP+SSE, the server can
  push notifications via SSE but cannot call client methods.
  This is a **backend concern** — only SDKs running as servers need this.
  The Go SDK provides two complementary implementations:
    - `transport.WebBridge` — embeddable in an existing HTTP server
      (browser-facing applications serving static files + JSON-RPC).
    - Standalone server — owns its TCP listener (for Go-to-Go
      JSON-RPC and interop testing).

- **Client**: connects to a JSON-RPC server, sends requests, receives
  responses. Over WebSocket, the client can also register handlers
  for server-initiated calls. Over HTTP+SSE, the client receives
  server-push notifications via SSE.
  **Every SDK must implement this** — it is the universal on-ramp to the
  organic ecosystem over the internet. Currently implemented in JS
  (`js-web-holons`).

### 5.8 References

| Document | URL |
|----------|-----|
| JSON-RPC 2.0 Specification | [jsonrpc.org/specification](https://www.jsonrpc.org/specification) |
| WebSocket Protocol (RFC 6455) | [datatracker.ietf.org/doc/html/rfc6455](https://datatracker.ietf.org/doc/html/rfc6455) |
| WebSocket Subprotocol Registry | [iana.org/assignments/websocket](https://www.iana.org/assignments/websocket/websocket.xml) |
| Language Server Protocol (similar approach) | [microsoft.github.io/language-server-protocol](https://microsoft.github.io/language-server-protocol/) |

### 5.9 Routing Topologies

When a **multivalent** JSON-RPC bridge maintains connections to
N peers, the bridge can route messages beyond simple unicast.
Routing is an **application-level capability** of the bridge —
not a protocol extension. The wire format (JSON-RPC 2.0) is unchanged.

#### Dispatch Modes

| Mode | Notation | Semantics |
|------|:--------:|-----------|
| **Unicast** | `addr → addr` | A calls B, B responds to A. The caller specifies the target peer via `_target` in `params` (see below). |
| **Fan-out** | `addr → *` | A calls all peers implementing the method. Each responds to A independently. |
| **Broadcast response** | `* → addr` | A calls B, but the response is forwarded to all peers capable of processing it. |
| **Full broadcast** | `* → *` | A broadcasts to all peers; all capable peers receive all responses. |

All four modes are **mandatory** for hub-role SDKs.
Client-role SDKs implement unicast only and rely on a hub
for fan-out, broadcast response, and full broadcast.

#### SDK Roles

An SDK's **role** determines its routing obligations:

- **Hub**: accepts N peer connections, implements all four dispatch
  modes, aggregates responses. Currently: **Go, JS (Node), Kotlin, Python, Rust, Swift, Dart**.
- **Client**: connects to a hub, sends unicast or wildcard requests,
  receives aggregated responses. The routing logic lives in the hub,
  not the client. Currently: **all other SDKs**.

The role is declared in each SDK's `cert.json` via the `"role"` field.

#### Use Cases

| Mode | Pattern | Example |
|------|:-------:|---------|
| Unicast | `A → B` | Standard RPC — "holon-b, what's your status?" |
| Fan-out | `A → *` | Query all — "everyone, ping me back" (aggregate) |
| Broadcast response | `* ← B` | Notification — "everyone, holon-b just joined" |
| Full broadcast | `* → *` | Mesh sync — "everyone tell everyone your state" |

#### Method-Name Convention

Routing is signaled through method-name patterns — the JSON-RPC
`method` field remains a plain string, interpreted by the bridge:

```
"echo.v1.Echo/Ping"     → unicast (default — one caller, one target)
"*.Echo/Ping"           → fan-out to all peers implementing Echo/Ping
```

For **unicast**, the caller specifies the target peer in `params`:

```json
{
  "method": "echo.v1.Echo/Ping",
  "params": { "message": "hello", "_target": "peer-id-of-B" }
}
```

`_target` is the **connection ID** assigned by the hub when the peer
connects. The hub returns available peer IDs via `rpc.peers` (see §4.7).
If `_target` is omitted, the hub routes to the first peer that
implements the method (implicit unicast).

For **broadcast response** and **full broadcast**, the caller adds
a `_routing` hint in `params`:

```json
{
  "method": "echo.v1.Echo/Ping",
  "params": { "message": "hello", "_routing": "broadcast-response" }
}
```

| `_routing` value | Mode | Semantics |
|:----------------:|:----:|-----------|
| *(absent)* | Unicast | Default — response goes to caller only |
| `"broadcast-response"` | `* ← addr` | Response is forwarded to all peers |
| `"full-broadcast"` | `* → *` | Combined with `*` prefix: dispatches to all, responses go to all |

The `_routing` field is stripped by the bridge before forwarding
to handlers — handlers never see it.

The bridge resolves `*` by inspecting its peer capability registry
(populated via `rpc.discover` or explicit registration at connection
time).

#### Response Aggregation

For fan-out (`addr → *`), the bridge **must** aggregate responses
into a single JSON-RPC response where `result` is an array of
per-peer entries:

```json
{
  "jsonrpc": "2.0",
  "id": "c42",
  "result": [
    { "peer": "holon-b", "result": { "message": "pong", "sdk": "go" } },
    { "peer": "holon-c", "result": { "message": "pong", "sdk": "python" } }
  ]
}
```

Each entry **must** contain a `peer` identifier and either a
`result` (success) or an `error` (failure) field:

```json
{
  "jsonrpc": "2.0",
  "id": "c42",
  "result": [
    { "peer": "holon-b", "result": { "message": "pong", "sdk": "go" } },
    { "peer": "holon-c", "error": { "code": 4, "message": "deadline exceeded" } }
  ]
}
```

**Edge cases:**

- **No connected peers**: return error code 5 (`NOT_FOUND`).
- **Partial failure**: include all entries — successful results
  alongside per-peer errors. Never discard a peer's response.
- **Ordering**: entries appear in arrival order (first response
  first). No guaranteed ordering by peer ID.

#### Applicability

Routing only applies to **hub-role SDKs** running a multivalent
JSON-RPC server (WebSocket transport). It has no meaning for
monovalent transports (one peer, no routing decision), for the
gRPC binding (point-to-point by design), for client-role SDKs,
or for the HTTP+SSE transport (no bidirectional calls).

---

## 6. Error Codes

Both bindings share a common error code space based on
[gRPC status codes](https://grpc.io/docs/guides/status-codes/).

### 6.1 Application Error Codes (gRPC)

| Code | Name | Meaning |
|:----:|------|---------|
| 0 | OK | Not an error |
| 1 | CANCELLED | Operation cancelled by the caller |
| 2 | UNKNOWN | Unknown error |
| 3 | INVALID_ARGUMENT | Client sent invalid parameters |
| 4 | DEADLINE_EXCEEDED | Timeout |
| 5 | NOT_FOUND | Method or resource not found |
| 6 | ALREADY_EXISTS | Resource already exists |
| 7 | PERMISSION_DENIED | Caller not authorized |
| 8 | RESOURCE_EXHAUSTED | Rate limit, quota, or capacity exceeded |
| 9 | FAILED_PRECONDITION | Precondition not met |
| 10 | ABORTED | Operation aborted (conflict, transaction failure) |
| 11 | OUT_OF_RANGE | Value out of acceptable range |
| 12 | UNIMPLEMENTED | Method not implemented |
| 13 | INTERNAL | Internal server error |
| 14 | UNAVAILABLE | Service temporarily unavailable |
| 15 | DATA_LOSS | Unrecoverable data loss |
| 16 | UNAUTHENTICATED | Missing or invalid authentication |

### 6.2 Protocol Error Codes (JSON-RPC only)

These apply exclusively to the JSON-RPC 2.0 binding:

| Code | Name | Meaning |
|:----:|------|---------|
| -32700 | Parse error | Invalid JSON |
| -32600 | Invalid Request | Missing required fields |
| -32601 | Method not found | Unknown method string |
| -32602 | Invalid params | Invalid method parameters |
| -32603 | Internal error | JSON-RPC internal error |

---

## 7. Operational Concerns

### 7.1 Heartbeat

To detect stale connections, either side **may** send periodic heartbeat
calls using a standard JSON-RPC request:

```json
{ "jsonrpc": "2.0", "id": "hb-1", "method": "rpc.heartbeat", "params": {} }
```

Expected response: `{ "jsonrpc": "2.0", "id": "hb-1", "result": {} }`

| Parameter | Default | Description |
|-----------|:-------:|-------------|
| Interval | 15s | Time between heartbeat requests |
| Timeout | 5s | Max wait for heartbeat response |

If a heartbeat response is not received within the timeout, the
sender **should** consider the connection stale and close it.

### 7.2 Reconnection

Client implementations on bidirectional transports **must** support
automatic reconnection with exponential backoff. Under default
parameters, a client **must** re-establish a working connection
within 5 seconds of detecting a disconnect (assuming the server
is available).

| Parameter | Default | Description |
|-----------|:-------:|-------------|
| `minDelay` | 500ms | Initial reconnection delay |
| `maxDelay` | 30s | Maximum reconnection delay |
| `factor` | 2.0 | Backoff multiplier |
| `jitter` | 0.1 | Random jitter factor (0–1) |

```
delay = min(minDelay × factor^n, maxDelay) × (1 + random(0, jitter))
```

### 7.3 Graceful Shutdown

All servers (gRPC and JSON-RPC) **must** handle `SIGTERM` and
`SIGINT` for graceful shutdown:

1. Stop accepting new connections.
2. Drain in-flight RPCs within a **10-second deadline**.
3. If the deadline expires, force-terminate remaining RPCs.
4. Close all listeners and connections.
5. Exit with code 0.

### 7.4 Robustness Requirements

These requirements apply to all implementations, regardless of
transport or binding. They ensure production-grade reliability.

#### 7.4.1 Concurrency

Implementations **must** handle concurrent requests without
deadlock or data corruption. A server must be able to process
at least 50 simultaneous RPC calls from independent clients
without failure.

#### 7.4.2 Resource Lifecycle

Implementations **must not** leak operating system resources
(file descriptors, threads, goroutines, sockets) across
connection lifecycles. After N connect/disconnect cycles,
resource usage must return to within a small constant delta
of the baseline (≤ 5 units).

#### 7.4.3 Timeout Propagation

When a caller provides a context with a deadline, the
implementation **should** propagate that deadline to the
handler. If the handler exceeds the deadline, the
implementation **must** return a `DEADLINE_EXCEEDED` error
(gRPC code 4) to the caller rather than hang indefinitely.

#### 7.4.4 Abrupt Disconnect Resilience

A server **must** remain operational when individual
connections are terminated abnormally (TCP RST, process kill,
network partition). Abrupt disconnects on a subset of clients
**must not** affect the server's ability to serve remaining
clients. Pending calls on the dropped connection **must** be
failed with an appropriate error (code 14, `UNAVAILABLE`).

### 7.5 Message Size Limits

Implementations **must** accept single messages up to at least
**1 MB** (1,048,576 bytes). Messages exceeding an implementation's
configured limit **must** be rejected with an error — not silently
truncated or dropped.

| Binding | Mechanism | Default | Error on exceed |
|---------|-----------|:-------:|-----------------|
| gRPC | `MaxRecvMsgSize` / `MaxSendMsgSize` | 4 MB | `RESOURCE_EXHAUSTED` (code 8) |
| JSON-RPC 2.0 | WebSocket `ReadLimit` / HTTP `Content-Length` | 1 MB | Connection closed with status 1009 (WS) or HTTP 413 (HTTP) |

Implementations **may** allow operators to raise these limits
via configuration, but **must not** lower them below 1 MB.

> **Rationale**: 1 MB accommodates all realistic RPC payloads
> (metadata, configuration blobs, small binary embeddings) while
> protecting servers from memory exhaustion. Bulk data transfer
> (video, large files) belongs in a streaming or out-of-band
> mechanism, not inside an RPC envelope.

Applications requiring payloads larger than 1 MB **should** use
out-of-band transfer (HTTP upload, object storage reference) or
application-level pagination — not oversized RPC envelopes.

---

## 8. Summary

| Aspect | gRPC Binding | JSON-RPC 2.0 Binding |
|--------|:------------:|:--------------------:|
| Wire format | Protobuf | JSON-RPC 2.0 |
| Transports | tcp, unix, stdio, ws/wss (tunnel) | ws, wss, http, https (SSE) |
| Dependency | gRPC library + protoc | WebSocket or HTTP + JSON |
| Bidirectional | ❌ caller-initiated only | ✅ WebSocket: symmetric — ❌ HTTP+SSE: server-push only |
| Type safety | ✅ proto schemas | ⚠️ JSON (runtime validation) |
| Use case | Holon-to-holon (local, LAN, high perf) | Internet, browser, mobile, scripting |
| Subprotocol | — | `holon-rpc` (WebSocket only) |

The JSON-RPC 2.0 binding adds **three conventions** to standard
JSON-RPC 2.0 — method naming (`package.Service/Method`), ID
namespacing (server-originated IDs prefixed with `s`), and a
WebSocket subprotocol (`holon-rpc`). Everything else is standard.
No new protocol.

---

## 9. References

| Document | URL |
|----------|-----|
| Constitution — Article 9 (Mimesis) | [CONSTITUTION.md](./CONSTITUTION.md#article-9--mimesis) |
| Constitution — Article 11 (Serve) | [CONSTITUTION.md](./CONSTITUTION.md#article-11--the-serve--dial-convention) |
| JSON-RPC 2.0 Specification | [jsonrpc.org/specification](https://www.jsonrpc.org/specification) |
| WebSocket Protocol (RFC 6455) | [datatracker.ietf.org/doc/html/rfc6455](https://datatracker.ietf.org/doc/html/rfc6455) |
| WebSocket Subprotocol Registry | [iana.org/assignments/websocket](https://www.iana.org/assignments/websocket/websocket.xml) |
| gRPC Documentation | [grpc.io](https://grpc.io/) |
| Protocol Buffers | [protobuf.dev](https://protobuf.dev/) |
| gRPC Status Codes | [grpc.io/docs/guides/status-codes](https://grpc.io/docs/guides/status-codes/) |
| Language Server Protocol (similar approach) | [microsoft.github.io/language-server-protocol](https://microsoft.github.io/language-server-protocol/) |
