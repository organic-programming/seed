# `op proxy` — Universal Transport Adapter

## Problem

Every new transport (REST+SSE, WebSocket, mTLS mesh) requires
implementation in **every SDK** — 12 SDKs × N transports = O(12N)
work. Most SDKs (Swift, Dart, Kotlin, Rust, C#, C++) will never
need to *serve* these transports natively — they run as CLI tools
or mobile apps that communicate through `op`.

Meanwhile, every holon already implements `stdio://` (Article 11).

## Solution: `op proxy` as Universal Sidecar

`op` sits between external consumers and the holon, bridging any
external-facing protocol to the holon's native `stdio://` or
`tcp://` gRPC connection. The holon stays simple; `op` handles
transport complexity, protocol translation, and middleware.

```
┌──────────────┐        ┌──────────────────────┐        ┌──────────────┐
│   External   │  any   │      op proxy         │ stdio  │    Holon     │
│   Consumer   │───────▶│  adapter + middleware  │───────▶│  (gRPC only) │
│              │◀───────│                       │◀───────│              │
└──────────────┘        └──────────────────────┘        └──────────────┘
     REST+SSE            translates protocols             unchanged
     WebSocket           observes (log, metrics)
     MCP                 enforces (ACL, auth)
     COAX
     mTLS
```

### Design Principles

1. **Zero holon changes** — the holon implements `stdio://` gRPC
   as usual. No knowledge of the external protocol.
2. **One implementation** — adapters and middleware are written
   once in Go inside `op`. All 12 SDKs benefit immediately.
3. **Composable** — multiple adapters can run simultaneously on
   different listeners for the same holon.
4. **Observable** — built-in middleware (logging, metrics, tracing,
   recording) applies to every proxied call without touching the
   holon or adding a separate proxy process.
5. **Progressive** — SDKs can add native transport support later
   for performance. The proxy is a stepping stone, not a cage.

---

## Precedent: `op mcp`

`op mcp` already implements this pattern exactly:

```
MCP client (stdio) → op mcp → holon (stdio gRPC)
```

`op mcp` launches the holon, calls `Describe` to learn its contract,
then exposes each RPC as an MCP tool. The holon has no idea it's
being accessed via MCP — it just sees standard gRPC calls.

`op proxy` generalizes this into a reusable primitive with multiple
protocol adapters and a built-in middleware chain.

---

## Command Interface

```bash
# REST+SSE adapter — holon becomes an HTTP API
op proxy rob-go --as rest+sse --listen https://:8443

# WebSocket adapter — holon becomes a JSON-RPC 2.0 WebSocket server
op proxy rob-go --as ws --listen wss://:8443/api/v1/rpc

# MCP adapter (replaces current `op mcp`)
op proxy rob-go --as mcp --listen stdio

# Multiple adapters simultaneously
op proxy rob-go \
  --as rest+sse --listen https://:8443 \
  --as ws       --listen wss://:8444/api/v1/rpc

# With middleware (built-in)
op proxy rob-go --as rest+sse --listen https://:8443 \
  --middleware logger,metrics

# With middleware plugins (external holons)
op proxy rob-go --as rest+sse --listen https://:8443 \
  --middleware logger,metrics \
  --plugin snoopy-inspect,gate-guard

# COAX organism — expose all members
op proxy --coax my-organism.yaml --as rest+sse --listen https://:8443
```

### Relationship to Existing Commands

| Today | Tomorrow | Change |
|-------|----------|--------|
| `op mcp <slug>` | `op proxy <slug> --as mcp` | Alias kept |
| `op serve <slug> --listen tcp://` | unchanged | Native gRPC, no proxy |
| `jack-middle --target <slug>` | `op proxy <slug> --middleware ...` | Jack absorbed into `op` |

`op mcp` remains as a convenience alias.

---

## Built-In Middleware

Jack Middle's core capabilities are absorbed directly into `op proxy`.
No separate process, no extra hop — middleware runs in the same Go
process as the protocol adapters.

### Middleware Chain

```
External → [adapter] → [middleware 1] → [middleware 2] → ... → holon (gRPC)
         ↙             ↙                                ↘
     translate      observe / enforce              forward to holon
```

### Tier 1 — Built-in (compiled into `op`)

| Name | Purpose |
|------|---------|
| `logger` | Log method, duration, status code, payload size |
| `tracer` | Assign trace IDs, emit spans (OpenTelemetry-ready) |
| `metrics` | Count RPCs, measure latency, track error rates |
| `recorder` | Record full request/response payloads for replay |
| `latency` | Inject artificial latency (chaos testing) |
| `fault` | Return synthetic errors at configurable rate |

These are zero-config, zero-dependency — they work on an airgapped
machine with no observability stack.

### Tier 2 — Plugin Holons (external)

For custom logic that doesn't belong in `op`, the plugin model from
Jack Middle remains available. A plugin holon implements
`middleware.v1.PluginService` and is connected via `--plugin slug`:

```
External → [adapter] → [built-in chain] → [plugin holon A] → [plugin holon B] → holon
```

Plugin catalogue (future):

| Plugin | Slug | Purpose |
|--------|------|---------|
| Snoopy Inspect | `snoopy-inspect` | Proto-aware payload inspection |
| Gate Guard | `gate-guard` | ACL-based call authorization |
| Echo Replay | `echo-replay` | Record-and-replay for testing |
| Canary Split | `canary-split` | A/B traffic routing |

### Prometheus Endpoint

When `metrics` middleware is active, `op proxy` exposes
`/metrics` in Prometheus exposition format — scraped by any
standard Prometheus instance. Labels include `remote_slug`,
`transport`, `method` — enabling per-peer, per-method dashboards
without touching the holon's code.

---

## Protocol Adapters

Each adapter translates between the external protocol and gRPC.

### REST+SSE Adapter

| External | Internal (gRPC) |
|----------|----------------|
| `POST /api/v1/rpc/<Service>/<Method>` | Unary RPC call |
| `GET /api/v1/rpc/<Service>/<Method>` (SSE) | Server-streaming RPC |
| JSON body | protojson ↔ protobuf transcoding |

The adapter calls `Describe` on the holon to build the HTTP router
automatically — **no codegen, no configuration**.

### WebSocket Adapter

| External | Internal (gRPC) |
|----------|----------------|
| JSON-RPC 2.0 message (text frame) | Unary or streaming RPC |
| Subprotocol `holon-rpc` | Standard gRPC |

### MCP Adapter (existing `op mcp`)

| External | Internal (gRPC) |
|----------|----------------|
| MCP `tools/call` | Unary RPC |
| MCP `tools/list` | `Describe` RPC |

Already implemented. Becomes the reference adapter.

### COAX Adapter

| External | Internal (gRPC) |
|----------|----------------|
| COAX `Tell` | Dispatched to member holon |
| COAX `Describe` | Aggregated from all members |

### Future: mTLS Mesh Adapter

| External | Internal (gRPC) |
|----------|----------------|
| gRPC over mTLS | gRPC over stdio/tcp (local) |

`op mesh` bridges cross-host connections through the mesh CA.

---

## Architecture

### Connection Lifecycle

```
1. op proxy rob-go --as rest+sse --listen https://:8443
2. op discovers rob-go via standard connect() algorithm
3. op launches rob-go (if not running) → stdio gRPC channel
4. op calls Describe → learns service/method signatures
5. op builds HTTP router from Describe response
6. op starts HTTPS listener on :8443
7. External request arrives:
   POST /api/v1/rpc/greeting.v1.GreetingService/SayHello
8. Middleware chain runs (logger, metrics, etc.)
9. op transcodes JSON → protobuf → gRPC call to rob-go
10. rob-go responds → op transcodes protobuf → JSON → HTTP response
```

### Multi-Adapter Architecture

```
                    ┌─ REST+SSE listener (:8443) ──┐
                    │                               │
External ──────────▶├─ WebSocket listener (:8444) ──┤
                    │                               ├── middleware ── stdio ── holon
                    ├─ MCP listener (stdio) ────────┤   chain          gRPC
                    │                               │                 channel
                    └─ gRPC+mTLS listener (:9090) ──┘
```

All adapters share the **same gRPC channel** to the holon. The
holon sees a single connection with multiplexed calls.

### Multiplexing: stdio is Monovalent

`stdio://` supports only one connection. When `op proxy` proxies
N external consumers through one stdio channel:

- **Fan-in**: N concurrent requests → multiplexed gRPC calls
  (HTTP/2 handles this natively over stdio).
- **Fan-out**: responses routed back to the correct consumer
  via request correlation.
- **Backpressure**: if the holon is slow, `op` returns standard
  HTTP 503 with retry-after to external consumers.

For high-throughput scenarios, `op proxy` can use `tcp://` instead
of `stdio://` for multivalent connections.

---

## Impact on SDK Roadmap

### Before: every SDK implements every transport

| | tcp | unix | stdio | ws server | ws client | REST+SSE |
|---|:---:|:---:|:---:|:---:|:---:|:---:|
| Each SDK | ✅ | ✅ | ✅ | ⚠️ | ⚠️ | ❌ |
| **Total work** | 12 | 12 | 12 | 12 | 12 | 12 = **72** |

### After: SDKs do stdio/tcp, `op proxy` does the rest

| | tcp | unix | stdio | ws | REST+SSE | mTLS | MCP |
|---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| Each SDK | ✅ | opt | ✅ | — | — | — | — |
| `op proxy` | — | — | — | ✅ | ✅ | ✅ | ✅ |
| **Total work** | 12 | opt | 12 | **1** | **1** | **1** | **1** = **~28** |

60% less implementation work. Every new adapter benefits all SDKs
instantly.

---

## What Happens to Jack Middle?

Jack's capabilities are **absorbed**, not deleted. The holon
remains in the ecosystem as a historical identity, but his code
moves into `op`:

| Jack Middle feature | Where it lives now |
|--------------------|--------------------|
| Transparent gRPC forwarding | `op proxy` core (adapter layer) |
| Built-in middleware (logger, metrics, etc.) | `op proxy --middleware` |
| Plugin holon model | `op proxy --plugin` |
| Port file hijack | `op proxy` (same mechanism) |
| `MiddleService` control RPC | `op proxy` status endpoint |

Jack's motto — *"I see everything"* — now belongs to `op proxy`.

---

## Security Integration

Each adapter inherits the security stack from PROTOCOL.md:

| Adapter | Auth options |
|---------|-------------|
| REST+SSE | TLS, API key, JWT, OAuth (HTTP headers) |
| WebSocket | TLS, subprotocol negotiation |
| MCP | Inherited from transport (stdio = local trust) |
| mTLS mesh | Client certificate from mesh CA |

Per-listener security from `holon.yaml`:

```yaml
proxy:
  - as: rest+sse
    listen: https://:8443
    security: public
    auth: api-key

  - as: ws
    listen: wss://:8444/api/v1/rpc
    security: mesh
```

This is the `ACL · Routing (planned)` layer from PROTOCOL.md —
`op proxy` is where it materializes.

---

## Phased Delivery

| Phase | Adapter | Builds on |
|:---:|---------|-----------:|
| **0** | MCP (already done as `op mcp`) | — |
| **1** | REST+SSE + built-in middleware | Describe + protojson |
| **2** | WebSocket (JSON-RPC 2.0) | Phase 1 + WS framing |
| **3** | mTLS mesh relay | Phase 2 + mesh CA |
| **4** | COAX organism | Phase 1 + COAX aggregation |

Phase 1 is the critical one — it unblocks browser/mobile access
to any holon without SDK changes.

---

## What Does Not Change

- **PROTOCOL.md** — wire formats and transports are unchanged.
  `op proxy` speaks those protocols on both sides.
- **Holon contracts** — holons are unmodified. They see standard
  gRPC calls on stdio/tcp.
- **`connect()` algorithm** — unchanged. `op proxy` uses it
  internally to reach the holon.
- **`op serve`** — native gRPC listeners remain. `op proxy` adds
  external protocol bridges alongside them.
