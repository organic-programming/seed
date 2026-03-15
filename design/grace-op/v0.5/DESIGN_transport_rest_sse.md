# Transport Design Note: REST + SSE for Holon Communication

## Motivation

Holons can run locally or distributed across hosts behind proxies, CDNs, firewalls, and load balancers. WebSocket's connection upgrade handshake is fragile in these environments. REST + SSE stays on standard HTTP throughout, making it the most infrastructure-compatible transport.

## Two Directions, Two Protocols

| Direction | Protocol | Mapping |
|---|---|---|
| Client → Server | **REST** (POST) | Unary RPCs, commands |
| Server → Client | **SSE** (EventSource) | Streaming responses, progress, logs |

## REST + SSE vs WebSocket

| Criterion | REST + SSE | WebSocket |
|---|---|---|
| Proxy / CDN / LB | ✅ Standard HTTP | ⚠️ Upgrade can be blocked |
| Auto-reconnect | ✅ Built into EventSource | ❌ Manual |
| Firewalls | ✅ Port 80/443, plain HTTP | ⚠️ Upgrade rejected |
| Load balancing | ✅ Stateless round-robin | ❌ Sticky sessions |
| Debugging | ✅ curl, devtools | ⚠️ Opaque frames |
| HTTP/2 multiplexing | ✅ Many streams, one conn | N/A |
| Full-duplex | ❌ Separate channels | ✅ Same connection |
| Binary payload | ⚠️ Requires protojson | ✅ Native |
| Per-message overhead | Slightly higher | Lower after handshake |

**WebSocket wins only for** high-frequency bidirectional messaging (collaborative editing, gaming, live audio). This is not a typical holon-to-holon pattern.

## RPC Mapping

### Unary RPC → POST

```
POST /v1/greeting.v1.GreetingService/SayHello
Content-Type: application/json

{"name": "Gudule", "language": "fr"}

← 200 OK
{"message": "Bonjour, Gudule !"}
```

### Server-Streaming RPC → SSE

```
GET /v1/build.v1.BuildService/WatchBuild?id=abc123
Accept: text/event-stream

← 200 OK
Content-Type: text/event-stream

event: log
data: {"line": "compiling main.go..."}

event: log
data: {"line": "linking..."}

event: done
data: {"status": "success", "artifact": "/out/binary"}
```

### Client-Streaming / Bidi → Chunked POST (or avoid)

True client-streaming (large file uploads to Phill) doesn't map cleanly to SSE. Options:
- **Chunked POST** with `Transfer-Encoding: chunked`
- **Multipart upload** for files
- **Fall back to raw gRPC** for high-throughput binary transfers

This is acceptable: use REST + SSE for control plane, raw gRPC for data plane.

## Relationship to Connect Protocol

Connect is essentially a formalized, code-generated version of REST + SSE:
- Unary RPCs as HTTP POST (JSON or proto)
- Server-streaming via chunked HTTP responses
- Works over HTTP/1.1+

REST + SSE doesn't replace Connect — it **aligns with it**. Connect is for typed/generated clients; raw REST + SSE is for no-codegen, lightweight, or third-party integrations.

## Security Layer

REST + SSE runs over standard HTTPS. For cross-network holon communication, **mTLS** (mutual TLS) secures the transport: both caller and callee present certificates signed by the mesh CA. This is managed by `op mesh` — see [DESIGN_op_mesh.md](../v0.8/DESIGN_mesh.md).

## Revised Transport Table

| Context | Transport | Security | When to use |
|---|---|---|---|
| Co-located (same host) | `stdio://` / `unix://` / `npipe://` | None needed | Maximum performance, no network |
| LAN / trusted network | gRPC over TCP | Optional TLS | Low-latency, binary proto |
| Web browser (typed) | Connect (`gRPC-Web`) | TLS | Generated client, type safety |
| Web browser (light) | REST + SSE | TLS | No codegen, easy integration |
| Cross-network / distributed | REST + SSE | **mTLS** | Proxies, LBs, firewalls in the path |
| Large binary transfers | gRPC streaming or chunked HTTP | **mTLS** | Phill file operations, artifacts |

## Constraints

- **SSE is text-only**: binary proto payloads must use `protojson` encoding. Negligible overhead for control RPCs; avoid for multi-MB transfers.
- **HTTP/1.1 connection limit**: browsers limit ~6 SSE connections per domain. Non-issue with HTTP/2 (multiplexed) or server-to-server (no browser limit).
- **No bidi on one connection**: acceptable trade-off. Holon RPCs are overwhelmingly request-response with occasional server-push.

---

## Client vs Server Roles

Not all SDKs need to implement both sides of REST+SSE.

### Rule

> **All SDKs must be REST+SSE clients.**
> **Only daemon SDKs must be REST+SSE servers.**

### Why

Frontend SDKs (Swift, Dart, Kotlin) drive UIs and call
daemons — they **consume** REST+SSE endpoints but don't
**serve** them. Requiring them to implement the server side
is wasted effort and architecturally wrong.

### Obligation per SDK role

| Role | REST+SSE Client (connect) | REST+SSE Server (serve) |
|---|---|---|
| **Daemon** (Go, Rust, Node, Python, C) | ✅ mandatory | ✅ mandatory |
| **Frontend** (Swift, Dart, Kotlin, C#, C++) | ✅ mandatory | ⚠️ optional |
| **Browser** (js-web) | ✅ native EventSource | ❌ impossible |
| **Utility** (Java, Ruby) | ✅ mandatory | ⚠️ optional |

Frontend SDKs _can_ add the server side later if a use case
emerges (e.g. a SwiftUI app serving as a local mesh holon on
macOS). But it is not a v0.5 requirement.
## Implementation Strategy

### Phase 1: Go Reference (TASK01)

`go-holons` implements the reference REST + SSE transport, establishing:
- URL routing: `POST /v1/<service>/<method>` for unary
- SSE event structure: `event:` + `data:` fields with `protojson`
- Auto-reconnect behavior for EventSource clients
- `rest+sse://` URI scheme for discover/listener registration
- **Both sides**: client (connect) and server (serve)

### Phase 2: grace-op CLI (TASK02)

Wire the Go transport into `op serve` and `op dial`:
- `op serve` starts a REST + SSE listener alongside gRPC
- `op dial` connects via REST + SSE when discover returns `rest+sse://`
- `holon.yaml` `serve.listeners` accepts `rest+sse://` URIs

### Phase 3: Daemon SDK Ports (TASK03, TASK07–10)

Daemon SDKs implement **both** client and server sides:
- Rust, C#, Node.js, C++, Python
- Cross-language interop: any daemon SDK server ↔ any SDK client
- End-to-end: `op run` with REST + SSE transport

### Phase 4: Frontend SDK Ports (TASK04–06)

Frontend SDKs implement **client only**:
- Dart, Swift, Kotlin
- POST for unary calls, EventSource for streaming
- Connect to daemon holons over REST + SSE
- No server-side requirement (optional future extension)

## Verdict

REST + SSE is **sustainable and recommended** as the default transport for distributed holon communication. Reserve raw gRPC for co-located / LAN scenarios and large binary data transfers.
