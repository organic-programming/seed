# COMMUNICATION.md — The Holon Communication Protocol

**Status:** stable

## 1. Overview
The communication protocol operates via two bindings over standard transports:
- **gRPC (Protobuf):** Holon-to-holon (high performance, local/LAN).
- **JSON-RPC 2.0:** Internet, browser, mobile, scripting.

> **Notice on Discovery & Connections:**  
> Name-based resolution, auto-building, package discovery, and the `connect` algorithms are entirely defined in **[DISCOVERY.md](DISCOVERY.md)**. This document specifies strictly the transport layers, wire formats, and routing behaviors once a channel is established.

## 2. Transports & Lifecycles

| Scheme | Description | Valence | Duplex |
|--------|-------------|:-------:|:------:|
| `tcp://` | TCP socket (default `:9090`) | Multi | Full |
| `unix://` | UNIX domain socket | Multi | Full |
| `stdio://` | Standard input/output pipes | Mono | Simulated |
| `ws://` | WebSocket (unencrypted) | Multi | Full |
| `wss://` | WebSocket (TLS) | Multi | Full |
| `http://` | HTTP (request/response + SSE) | Multi | Simulated |
| `https://` | HTTP over TLS (req/resp + SSE) | Multi | Simulated |

* **`Listen(uri)`:** Server-side bind. Accepts connections.
* **`Dial(uri)`:** Client-side connect. Initiates connections.
* **`Serve`:** Universal entry point binding the local listener, gRPC server, `HolonMeta` docs, and signal handlers.

### 2.1 Readiness Verification
Once connection logic (see [DISCOVERY.md](DISCOVERY.md)) completes a dial, SDKs **MUST** verify the target is actively processing.
- **RPC Probe:** Send a `HolonMeta/Describe` RPC call.
- **Polling:** Retry every `~100 ms` until the default `5s` timeout.
- **Zombie Prevention:** If verification outlasts the timeout, any child processes spawned by the SDK must be forcefully killed via `SIGKILL`.

## 3. gRPC Binding
- **Wire Format:** Standard HTTP/2 (for TCP/UNIX) or native length-prefixed framing (for stdio: 1-byte compression flag + 4-byte big-endian length + payload).
- **Dynamic Dispatch:** gRPC reflection is strictly **disabled**. External bridges (`op mcp`, `op inspect`) use `HolonMeta/Describe` as the single canonical schema source.

### Type Mapping for Dynamic Encoders
The `FieldDoc.type` mapped from `Describe` maps exactly to protobuf wire types:

| `type` | Wire Type | Description / Encoding |
|--------|:---------:|------------------------|
| `string`, `bytes`, `<package.Message>` | 2 | Length-delimited. |
| `int32`, `int64`, `uint32`, `uint64`, `sint32`, `sint64`, `bool`, `<Enum>` | 0 | Varint (zigzag for `sint`). |
| `float`, `fixed32`, `sfixed32` | 5 | 32-bit little-endian (IEEE 754). |
| `double`, `fixed64`, `sfixed64` | 1 | 64-bit little-endian (IEEE 754). |

*Note: Repeated scalar fields utilize packed encoding. Maps encode as repeated messages featuring `key=1`, `value=2`.*

## 4. JSON-RPC 2.0 Binding
Standard JSON-RPC 2.0 exactly as specified, with three conventions: Method naming (`package.Service/Method`), Server-originated IDs (`s`-prefixed), and Subprotocols (`holon-rpc` for WebSockets).

| Action | Payload / Format |
|--------|------------------|
| Request | `{"jsonrpc":"2.0", "id":"c42", "method":"echo.v1.Echo/Ping", "params":{"message":"hello"}}` |
| Response | `{"jsonrpc":"2.0", "id":"c42", "result":{"message":"hello"}}` |
| Error | `{"jsonrpc":"2.0", "id":"c42", "error":{"code":5, "message":"method not found"}}` |
| Notification | Standard request format **without** the `id` field. Sender expects no response. |
| Batch | Multi-request arrays. Receivers may reject by returning `-32600`. |

### 4.1 WebSocket Interactivity
The WebSockets handshake MUST include `Sec-WebSocket-Protocol: holon-rpc`.
Servers MUST confirm the subprotocol or the client will shut down the connection (`1002 Protocol Error`). WebSocket operates in full-duplex — either end can invoke remote methods.

### 4.2 HTTP+SSE Polling
WebSockets aren't viable across all environments; HTTP functions as a unidirectional fallback.
* **Unary POST:** `POST /api/v1/rpc/<package.Service>/<Method>` with JSON body.
* **Streaming GET:** `GET /api/v1/rpc/<package.Service>/<Method>`. Used for server-streaming SSE.

**SSE Chunking Format:**
The SSE stream leverages identical JSON-RPC 2.0 payloads encapsulated in SSE chunks:
```text
event: message           // 'error' for failure states
id: <integer>            // Linearly incrementing ID for reconnection
data: {"jsonrpc":"2.0", "id":"c42", "result":{"progress":42}}

event: done              // Instructs the client the stream is finished
data: 
```
### 4.3 Multiplexing & Routing Matrix
This routing logic applies exclusively to an SDK operating as a **Hub** (a multivalent server maintaining connections to multiple peers via WebSockets or HTTP+SSE). A Hub intercepts these message prefixes and routes the payloads appropriately. SDKs operating strictly as **Clients** (connecting outward to a Hub) do not implement internal routing and rely entirely on the Hub to distribute their broadcasts.

#### Hub Routing API
Clients control how their payloads are routed by injecting control directives alongside their `payload` object. 

There is no special "Hub Server" binary. A Hub is simply a standard holon instructed to bind to a network listener (like HTTP or WebSocket). For example, using the Go SDK:

```bash
# Launch a holon as a Hub
# (0.0.0.0 binds to all network interfaces so external clients can reach it)
op run gabriel-greeting-go serve --listen http://0.0.0.0:8080
```

Once running, any SDK (acting as a Client) can dial the Hub's JSON-RPC endpoint (`/api/v1/rpc`). For an HTTP listener, clients submit routing payloads via `POST` requests:

```json
/* POST /api/v1/rpc/greeting.v1.GreetingService/SayHello */
{
  "jsonrpc": "2.0",
  "id": "req-2",
  "method": "greeting.v1.GreetingService/SayHello",
  "params": { 
     // `_target` accepts any valid Discovery expression (see [Expression Types](./DISCOVERY.md#expression-types)).
     // e.g., "*" (everyone), "gabriel-greeting-ruby:*" (class multicast), or "gabriel-greeting-ruby:ea346efb" (unicast instance).
     // When `_target` is omitted, the Hub defaults to routing to the first capable peer.
     "_target": "gabriel-greeting-ruby:ea346efb",   

     // `_response` controls who receives the returned result.
     // "return" (default): send the result only back to the caller.
     // "broadcast": push the result to all connected peers.
     "_response": "broadcast",

     // The actual application data is cleanly isolated here:
     "payload": { "name": "World" }
  }
}
```

> **Note on HTTP constraints:** An HTTP client can *trigger* a Fan-out or Broadcast using the matrix (and receive the aggregated answers in the HTTP response), but because HTTP requests are unidirectional, it cannot act as a passive target to *receive* unprompted requests randomly sent by other peers. The Hub can only push dynamic, unprompted requests to clients actively holding open WebSocket or SSE connections.

*(Reserved control plane commands[^1]: `rpc.discover`, `rpc.heartbeat`, `rpc.peers`)*

### 4.4 Hub Control Plane Commands
These reserved methods are processed strictly by the Hub bridge itself[^1] and are never routed to an application's domain logic. They act as the underlying networking nervous system.

| Method | Request `params` | Expected `result` | Purpose |
|--------|------------------|-------------------|---------|
| `rpc.heartbeat` | `{}` | `{}` | Sent every `15s` to detect stale connections (with a `5s` timeout). Both clients and servers may send it. |
| `rpc.peers` | `{}` | `{"peers": ["gabriel-greeting-go:ea346efb", "ruby-app:3f08b5c3"]}` | Returns a list of all currently connected instance identities. Required for clients to discover valid routing targets. |
| `rpc.discover` | `{}` | `{"methods": ["greeting.v1.Greeting/SayHello", "log.v1.Logger/Write"]}` | Informs the Hub of all domain methods exposed by a connecting client. The Hub uses this to populate its active Fan-out and Broadcast routing registries. |

[^1]: These commands are processed internally at the transport layer by the JSON-RPC Hub bridge to manage connections, discover capabilities, and establish routing matrices. They are never dispatched to the holon's domain services.

## 5. Error Codes Space

gRPC and JSON-RPC share an identical root context originating from the gRPC status standards.

| Code | gRPC Mapping | Code | JSON-RPC Mapping |
|:----:|--------------|:----:|------------------|
| 0 | OK | -32700 | Parse error / Invalid JSON |
| 1 | CANCELLED | -32600 | Invalid Request / Missing fields |
| 2 | UNKNOWN | -32601 | Method not found |
| 3 | INVALID_ARGUMENT | -32602 | Invalid params |
| 4 | DEADLINE_EXCEEDED | -32603 | Internal error |
| 5 | NOT_FOUND | | *(Codes 0–16 share generic mapping context)* |
| 6 | ALREADY_EXISTS | | |
| 7 | PERMISSION_DENIED | | |
| 8 | RESOURCE_EXHAUSTED | | |
| 9 | FAILED_PRECONDITION | | |
| 10 | ABORTED | | |
| 11 | OUT_OF_RANGE | | |
| 12 | UNIMPLEMENTED | | |
| 13 | INTERNAL | | |
| 14 | UNAVAILABLE | | |
| 15 | DATA_LOSS | | |
| 16 | UNAUTHENTICATED | | |

## 6. Operational Constraints
Strict parameters enforced across all compliant SDK implementations to ensure high systemic stability.

* **Message Hard-Limit:** `1 MB`. Transports MUST reject single packets exceeding `1,048,576 bytes` via `RESOURCE_EXHAUSTED` (code 8) or HTTP `413`. Payload batching limits do not stretch this bounding constraint.
* **Graceful Lifecycling:** All processes MUST intercept `SIGTERM`/`SIGINT`. Discontinue new connection allocations, wait **10s** (`10-second drain deadline`) for active requests/transports to terminate gracefully, then forcefully `SIGKILL` survivors and exit 0.
* **Deadline Propagation:** Outbound or incoming connections explicitly defining an execution deadline via Context propagate automatically. SDK handlers stalling beyond this deadline MUST terminate processing immediately and yield `4 DEADLINE_EXCEEDED`.
* **Reconnection Timing (Clients):** Min `500ms`, Max `30s`, `Factor: 2.0`, `Jitter: 0.1` (`delay = min(minDelay × factor^n, maxDelay) × (1 + random(0, jitter))`).
* **Heartbeat Timeout:** `15s` polling intervals utilizing `"method": "rpc.heartbeat"`. Clients assume dead connection cascades and kill sockets if polling surpasses a terminal `5s` threshold limit.
