---
# Cartouche v1
title: "Holon Communication Protocol"
author:
  name: "B. ALTER & Claude"
  copyright: "© 2026 Benoit Pereira da Silva"
created: 2026-02-13
revised: 2026-02-16
lang: en-US
origin_lang: en-US
access:
  humans: true
  agents: true
version: "1.1"
status: review
---
# Holon Communication Protocol

> *"Every holon must be reachable."* — Constitution, Article 11
> *"Invent only what is truly new; for everything else, imitate what works."* — Constitution, Article 9

This document is the single, authoritative reference for how holons
communicate. It covers the transport layer, both RPC bindings (gRPC
and Holon-RPC), error codes, and operational concerns.

---

## 1. Architecture Overview

Holons communicate through two complementary RPC bindings, each with
its own set of transports:

```
┌───────────────────────────────────────────────────┐
│                  Applications                     │
├──────────────────────┬────────────────────────────┤
│      gRPC Binding    │    Holon-RPC Binding       │
│     (protobuf)       │  (JSON-RPC 2.0)            │
├──────────────────────┼────────────────────────────┤
│  tcp://  unix://     │                            │
│  stdio://  mem://    │       ws://  wss://        │
│ws://  wss:// (tunnel)│                            │
└──────────────────────┴────────────────────────────┘
```

Transports differ in two structural properties beyond mere
connectivity — **valence** (how many simultaneous peers) and
**duplex** (can both sides send at the same time). See §2.7.

| Binding | Wire format | Use case | Dependency |
|---------|-------------|----------|------------|
| **gRPC** | Protobuf | Holon-to-holon (high performance, local/LAN) | gRPC library required |
| **Holon-RPC** | JSON-RPC 2.0 over WebSocket | Internet, browser, mobile, scripting | WebSocket + JSON only |

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
| `mem://` | In-process channel (testing) | — | optional |
| `ws://` | WebSocket (unencrypted) | `ws://:8080/rpc` | optional |
| `wss://` | WebSocket (TLS) | `wss://:8443/rpc` | optional |

### 2.2 Listen — Server-Side Binding

`Listen(uri) → Listener` opens a real OS-level socket (or pipe,
or in-process channel) and returns something a gRPC server can
accept connections on.

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
server setup, reflection, and signal handling into a single call.
See [Constitution, Article 11](./AGENT.md#article-11--the-serve--dial-convention).

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

#### `mem://`

In-process channel using connected buffer pairs (e.g., `socketpair()`,
`bufconn`, `DuplexStream`). Used exclusively for unit testing where
server and client coexist in the same process.

#### `ws://host:port/path` and `wss://host:port/path`

WebSocket transport. Two distinct uses:

1. **gRPC tunnel** — wraps gRPC binary frames inside WebSocket frames.
   The WebSocket is transparent; gRPC operates normally.
2. **Holon-RPC** — carries JSON-RPC 2.0 messages (see §4).
   No gRPC involvement.

The path distinguishes the two: convention is `/grpc` for tunneled gRPC
and `/rpc` for Holon-RPC. This is a recommendation, not a requirement.

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
| `mem://` | **Mono** | Full | One in-process channel pair per lifetime |
| `ws://` | Multi | Full | WebSocket is explicitly full-duplex (RFC 6455 §1.1) |
| `wss://` | Multi | Full | Same as `ws://` with TLS |

---

## 3. gRPC Binding

### 3.1 Overview

The primary holon-to-holon protocol. Uses [gRPC](https://grpc.io/)
with Protocol Buffers for serialization. Over TCP, gRPC uses its
standard HTTP/2 framing. Over stdio and mem, gRPC uses its native
length-prefixed wire format (5-byte header: 1 compression flag +
4-byte big-endian message length, then the serialized protobuf).

Every holon that supports the gRPC binding **must**:

1. Accept a `--listen` flag for the transport URI.
2. Enable [gRPC server reflection](https://grpc.io/docs/guides/reflection/)
   (Constitution, Article 2 — Introspection).
3. Handle `SIGTERM` / `SIGINT` for graceful shutdown.

### 3.2 Service Definition

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

### 3.3 Supported Transports

The gRPC binding operates over `tcp://`, `unix://`, `stdio://`, and `mem://`.
It can also be tunneled through `ws://` / `wss://` where WebSocket wraps
the raw HTTP/2 stream.

### 3.4 References

| Document | URL |
|----------|-----|
| gRPC Documentation | [grpc.io](https://grpc.io/) |
| Protocol Buffers | [protobuf.dev](https://protobuf.dev/) |
| gRPC Server Reflection | [grpc.io/docs/guides/reflection](https://grpc.io/docs/guides/reflection/) |
| gRPC Status Codes | [grpc.io/docs/guides/status-codes](https://grpc.io/docs/guides/status-codes/) |

---

## 4. Holon-RPC Binding

### 4.1 Overview

**Holon-RPC** is a convention layer on top of two established standards:

- [**JSON-RPC 2.0**](https://www.jsonrpc.org/specification) — the wire format
- [**WebSocket** (RFC 6455)](https://datatracker.ietf.org/doc/html/rfc6455) — the transport

Holon-RPC is **not a new protocol**. It is JSON-RPC 2.0 transmitted over
WebSocket, with gRPC-style method naming and bidirectional call support.
Any compliant JSON-RPC 2.0 library can produce and consume Holon-RPC
messages without modification.

**Design principles:**

1. **No invention** — reuse JSON-RPC 2.0 exactly as specified.
2. **Symmetric** — both client and server can initiate calls.
3. **Lightweight** — no gRPC dependency, no protobuf, no HTTP/2.
4. **Universal** — any language with WebSocket + JSON can participate.

### 4.2 When to Use

Holon-RPC exists for environments where gRPC is unavailable or impractical:

- **Browsers** — no HTTP/2 trailer support.
- **Mobile apps** — lighter than a full gRPC dependency.
- **Scripting languages** — PHP, Ruby, etc. where gRPC setup is heavy.
- **Internet-facing access** — WebSocket traverses NATs and proxies.

### 4.3 Transport Binding

Holon-RPC messages are exchanged as **WebSocket text frames** (opcode `0x1`).
Each frame contains exactly one JSON-RPC 2.0 message.

#### Subprotocol

The WebSocket handshake **must** include the subprotocol `holon-rpc`:

```
GET /rpc HTTP/1.1
Upgrade: websocket
Sec-WebSocket-Protocol: holon-rpc
```

The server **must** confirm the subprotocol in its response.
If the server does not confirm `holon-rpc`, the client **should** close
the connection with status `1002` (Protocol Error).

#### URL Convention

Recommended default endpoint: `/rpc`.

```
ws://host:port/rpc        — unencrypted
wss://host:port/rpc       — TLS encrypted
```

### 4.4 Wire Format

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
Holon-RPC implementations **may** support batching. If unsupported,
the receiver **should** respond with error code `-32600` (Invalid Request).

### 4.5 Method Naming Convention

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
| `rpc.heartbeat` | either direction | Keep-alive (see §6) |
| `rpc.peers` | client → hub | List connected peer IDs and their registered methods. Returns `{ "peers": [{ "id": "…", "methods": ["…"] }] }`. Required for `_target` unicast addressing (see §4.9). |
| `rpc.resume` | client → server | *Reserved — not yet specified.* Intent: reconnect session continuity (resume from a sequence number). |
| `rpc.transfer` | either direction | *Reserved — not yet specified.* Intent: out-of-band large payload negotiation (exchange a URI/hash instead of inline data). |

### 4.6 Bidirectional Calls

Holon-RPC is **fully symmetric**: once connected, either holon can
call remote methods registered on the other. The concepts of "client"
and "server" refer only to who initiated the WebSocket connection —
after the handshake, both peers are equal.

> **Note**: bidirectionality is orthogonal to valence. A monovalent
> link (e.g., `stdio://`) is still symmetric — both sides can call
> each other — but it connects exactly one pair. A multivalent
> transport adds the 1:N dimension, not the bidirectional one.

#### ID Namespacing

To prevent ID collisions in bidirectional communication:

- **Client-originated IDs**: any string or number chosen by the client.
- **Server-originated IDs**: **must** be prefixed with `s` (e.g., `"s1"`, `"s2"`).

This convention is **mandatory** for Holon-RPC to enable safe
bidirectional multiplexing.

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

### 4.7 Roles

The Holon-RPC binding has two distinct roles:

- **Server**: accepts WebSocket connections, dispatches incoming JSON
  calls to handlers, and can invoke methods on connected clients.
  This is a **backend concern** — only SDKs running as servers need this.
  The Go SDK provides two complementary implementations:
    - `transport.WebBridge` — embeddable in an existing HTTP server
      (for browser-facing applications serving static files + Holon-RPC).
    - `holonrpc.Server` — standalone server (owns its TCP listener,
      for Go-to-Go Holon-RPC and interop testing).

- **Client**: connects to a Holon-RPC server, sends requests, receives
  responses, and can register handlers for server-initiated calls.
  **Every SDK must implement this** — it is the universal on-ramp to the
  organic ecosystem over the internet. Currently implemented in JS
  (`js-web-holons`).

### 4.8 References

| Document | URL |
|----------|-----|
| JSON-RPC 2.0 Specification | [jsonrpc.org/specification](https://www.jsonrpc.org/specification) |
| WebSocket Protocol (RFC 6455) | [datatracker.ietf.org/doc/html/rfc6455](https://datatracker.ietf.org/doc/html/rfc6455) |
| WebSocket Subprotocol Registry | [iana.org/assignments/websocket](https://www.iana.org/assignments/websocket/websocket.xml) |
| Language Server Protocol (similar approach) | [microsoft.github.io/language-server-protocol](https://microsoft.github.io/language-server-protocol/) |

### 4.9 Routing Topologies

When a **multivalent** Holon-RPC bridge maintains connections to
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
Holon-RPC server. It has no meaning for monovalent transports
(one peer, no routing decision), for the gRPC binding
(point-to-point by design), or for client-role SDKs.

---

## 5. Error Codes

Both bindings share a common error code space based on
[gRPC status codes](https://grpc.io/docs/guides/status-codes/).

### 5.1 Application Error Codes (gRPC)

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

### 5.2 Protocol Error Codes (JSON-RPC only)

These apply exclusively to the Holon-RPC binding:

| Code | Name | Meaning |
|:----:|------|---------|
| -32700 | Parse error | Invalid JSON |
| -32600 | Invalid Request | Missing required fields |
| -32601 | Method not found | Unknown method string |
| -32602 | Invalid params | Invalid method parameters |
| -32603 | Internal error | JSON-RPC internal error |

---

## 6. Operational Concerns

### 6.1 Heartbeat

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

### 6.2 Reconnection

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

### 6.3 Graceful Shutdown

All servers (gRPC and Holon-RPC) **must** handle `SIGTERM` and
`SIGINT` for graceful shutdown:

1. Stop accepting new connections.
2. Drain in-flight RPCs within a **10-second deadline**.
3. If the deadline expires, force-terminate remaining RPCs.
4. Close all listeners and connections.
5. Exit with code 0.

### 6.4 Robustness Requirements

These requirements apply to all implementations, regardless of
transport or binding. They ensure production-grade reliability.

#### 6.4.1 Concurrency

Implementations **must** handle concurrent requests without
deadlock or data corruption. A server must be able to process
at least 50 simultaneous RPC calls from independent clients
without failure.

#### 6.4.2 Resource Lifecycle

Implementations **must not** leak operating system resources
(file descriptors, threads, goroutines, sockets) across
connection lifecycles. After N connect/disconnect cycles,
resource usage must return to within a small constant delta
of the baseline (≤ 5 units).

#### 6.4.3 Timeout Propagation

When a caller provides a context with a deadline, the
implementation **should** propagate that deadline to the
handler. If the handler exceeds the deadline, the
implementation **must** return a `DEADLINE_EXCEEDED` error
(gRPC code 4) to the caller rather than hang indefinitely.

#### 6.4.4 Abrupt Disconnect Resilience

A server **must** remain operational when individual
connections are terminated abnormally (TCP RST, process kill,
network partition). Abrupt disconnects on a subset of clients
**must not** affect the server's ability to serve remaining
clients. Pending calls on the dropped connection **must** be
failed with an appropriate error (code 14, `UNAVAILABLE`).

### 6.5 Message Size Limits

Implementations **must** accept single messages up to at least
**1 MB** (1,048,576 bytes). Messages exceeding an implementation's
configured limit **must** be rejected with an error — not silently
truncated or dropped.

| Binding | Mechanism | Default | Error on exceed |
|---------|-----------|:-------:|-----------------|
| gRPC | `MaxRecvMsgSize` / `MaxSendMsgSize` | 4 MB | `RESOURCE_EXHAUSTED` (code 8) |
| Holon-RPC | WebSocket `ReadLimit` | 1 MB | Connection closed with status 1009 (Message Too Big) |

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

## 7. Summary

| Aspect | gRPC Binding | Holon-RPC Binding |
|--------|:------------:|:-----------------:|
| Wire format | Protobuf | JSON-RPC 2.0 over WebSocket |
| Transports | tcp, unix, stdio, mem, ws/wss (tunnel) | ws, wss |
| Dependency | gRPC library + protoc | WebSocket + JSON |
| Bidirectional | ❌ caller-initiated only | ✅ symmetric — either side initiates |
| Type safety | ✅ proto schemas | ⚠️ JSON (runtime validation) |
| Use case | Holon-to-holon (local, LAN, high perf) | Internet, browser, mobile, scripting |
| Subprotocol | — | `holon-rpc` |

**Holon-RPC = JSON-RPC 2.0 + WebSocket + three conventions:**

1. **Method naming**: `package.Service/Method` (gRPC style).
2. **ID namespacing**: server-originated IDs prefixed with `s`.
3. **Subprotocol**: `holon-rpc`.

Everything else is standard JSON-RPC 2.0. No new protocol.

---

## 8. References

| Document | URL |
|----------|-----|
| Constitution — Article 9 (Mimesis) | [AGENT.md](./AGENT.md#article-9--mimesis) |
| Constitution — Article 11 (Serve) | [AGENT.md](./AGENT.md#article-11--the-serve--dial-convention) |
| JSON-RPC 2.0 Specification | [jsonrpc.org/specification](https://www.jsonrpc.org/specification) |
| WebSocket Protocol (RFC 6455) | [datatracker.ietf.org/doc/html/rfc6455](https://datatracker.ietf.org/doc/html/rfc6455) |
| WebSocket Subprotocol Registry | [iana.org/assignments/websocket](https://www.iana.org/assignments/websocket/websocket.xml) |
| gRPC Documentation | [grpc.io](https://grpc.io/) |
| Protocol Buffers | [protobuf.dev](https://protobuf.dev/) |
| gRPC Server Reflection | [grpc.io/docs/guides/reflection](https://grpc.io/docs/guides/reflection/) |
| gRPC Status Codes | [grpc.io/docs/guides/status-codes](https://grpc.io/docs/guides/status-codes/) |
| Language Server Protocol (similar approach) | [microsoft.github.io/language-server-protocol](https://microsoft.github.io/language-server-protocol/) |
