# Sessions — Transport-Level Connection Identification & Introspection

## Problem

Holons communicate over gRPC connections — but today, no one tracks
**who** is connected, **since when**, or **in what state**. Whether a
connection was established via `connect()`, a direct `grpc.Dial()`, a
manual `serve --listen`, or a recipe assembly launch, there is no way to:

- List the connections a holon is currently serving or consuming.
- Diagnose a stuck or stale connection without killing the process.
- Correlate a transport-level error with a specific peer.
- Review past connection history for debugging or auditing.

## Solution: SDK-Managed Sessions

Every gRPC connection — inbound (server-side) or outbound (client-side),
regardless of how it was established — gets a **session** with a UUID,
transport metadata, and a lifecycle state. The SDK tracks sessions at
the **transport/serve layer** and exposes them through a new
`HolonSession` gRPC service.

### Design Principles

1. **Opt-in debug mode** — sessions are **off by default**. The parent
   launcher (typically `op`) enables them explicitly. When disabled, zero
   overhead: no store, no interceptor, no `HolonSession` service.
2. **Parent-controlled** — the decision to enable sessions belongs to
   whoever launches the holon, not to the holon itself. A holon cannot
   self-enable sessions. This keeps the feature out of production unless
   the operator wants it.
3. **SDK-managed, not protocol-level** — sessions are bookkeeping inside
   the SDK, not a wire-level extension. The gRPC/Holon-RPC stream is
   unchanged.
4. **Dual view** — the **client SDK** tracks outbound sessions (holons it
   connected to); the **server SDK** tracks inbound sessions (holons
   connected to it). Both are queryable when enabled.

---

## Activation

Sessions are **off by default**. Three layers control activation, each
overriding the one below:

### Priority Model

| Priority | Layer | Scope | When it's decided |
|:---:|---|---|---|
| **1 (highest)** | **Code API** | Per-dial, per-listener | At connect/serve call time |
| **2** | **RPC** | Per-listener, runtime | Operator toggles on a live holon |
| **3 (lowest)** | **Config** | Per-holon, static | `OP_SESSIONS` env or `holon.proto` |

Higher-priority layers override lower ones. Each SDK expresses these
layers in its own idiomatic way — the principle is universal, the
syntax is language-specific.

### Layer 1 — Code API

The holon developer decides **per transport call** whether to enable
sessions. This is the most granular control: a holon can trace the
dial to one peer but not another, or trace one listener but not the
rest. The SDK's dial and serve primitives accept a session option.

This is the primary control point. It allows selective tracing
without any configuration — useful during development, debugging a
specific interaction, or instrumenting a known-slow path.

### Layer 2 — RPC (Runtime Toggle)

An operator can enable or disable session tracking on a live holon
without restarting it. The `HolonSession` service exposes a toggle
RPC that activates or deactivates tracing per listener at runtime.

This is useful for production diagnostics: enable tracing, observe,
disable — without a deploy cycle.

### Layer 3 — Config (Static Defaults)

Environment variable and `holon.proto` provide the static baseline.

```
OP_SESSIONS=1         # enable session tracking (all listeners)
OP_SESSIONS=metrics   # enable sessions + per-session metrics
OP_SESSIONS=          # disabled (default)
```

The `OP_SESSIONS` env var is set by the **parent launcher** — the
holon itself never decides to self-activate via config. This keeps
the feature out of production unless the operator wants it.

| Launcher | Mechanism |
|---|---|
| `op run <slug> --sessions` | `op` sets `OP_SESSIONS=1` in the child's env |
| `op sessions <slug>` | `op` starts the holon with `OP_SESSIONS=1` if needed |
| Manual launch | `OP_SESSIONS=1 ./my-holon serve --listen tcp://:9090` |

The env var propagates naturally: if `op` launches a composite recipe
with `--sessions`, all child daemons inherit `OP_SESSIONS=1`. Each
child also receives its **own distinct** `OP_INSTANCE_UID` (see
[INSTANCES.md §Instance ↔ Session Linkage](INSTANCES.md#instance--session-linkage)),
so sessions from different recipe members can be disambiguated by
their owning `instance_uid`. Rolling session metrics up to the
recipe as a whole is a v2 concern handled by `op proxy`; v1 leaves
per-child sessions as the finest granularity.

### Override Semantics

When multiple layers disagree, the higher-priority layer wins:

- Code API says `sessions: true` on a specific dial → traced,
  even if `OP_SESSIONS` is unset.
- Code API says `sessions: false` on a listener → not traced,
  even if `OP_SESSIONS=1`.
- RPC enables tracing at runtime → overrides config, but code API
  still has final say on new connections.

When no layer has an explicit opinion, the default is **off**.

### `op proxy` — Network-Level Activation

> **v2 (requires `op proxy`).** `op proxy` is NOT IMPLEMENTED in v1 (see
> [OP_PROXY.md](holons/grace-op/OP_PROXY.md)). This subsection describes
> the intended integration once the proxy ships; nothing in v1 depends
> on it.

`op proxy` is the transparent routing daemon that sits between caller and target instances. It sees every connection and every RPC — which makes it a natural session activation point.

`op proxy` can **activate** or **inhibit** sessions on the connections it proxies, independently for each side:

| Side | Connection | `op proxy`'s control |
|---|---|---|
| **Frontend** | Caller → Proxy | Can enable sessions on its listener (inbound) |
| **Backend** | Proxy → Target | Can enable sessions on its dial (outbound) |

This gives `op proxy` three modes for session control:

| Mode | Frontend sessions | Backend sessions | Use case |
|---|:---:|:---:|---|
| **Observe** | ✅ on | ✅ on | Full visibility: trace both sides |
| **Frontend only** | ✅ on | ❌ off | Trace callers without the target knowing |
| **Inhibit** | ❌ off | ❌ off | Proxy is invisible to target's session tracking |

`op proxy`'s built-in routing engine naturally collects per-method
latency and counts. With session activation, these metrics can be
**attached to session IDs** — correlating the proxy's middleware data
with the session store on both sides.

**Layer mapping**: `op proxy` activates sessions using Layer 1 (Code API)
when it dials or listens, and Layer 2 (RPC) when an operator toggles tracing at runtime. The
target holon's own session policy (Layer 1/2/3) is independent —
the proxy's backend session is the target's inbound session, and both
sides keep their own store.

---

## Session Model

### Identity Fields

Each session carries a single `remote_slug` that names the peer
seen from this holon's point of view — the caller for `INBOUND`
sessions, the dialed target for `OUTBOUND` sessions. The direction
field disambiguates.

| Field | Type | Source | Description |
|---|---|---|---|
| `session_id` | `string` (UUID v4) | Generated at dial or accept time | Unique per connection |
| `remote_slug` | `string` | `x-holon-slug` gRPC metadata header (inbound) or `holon.proto` / `--listen` URI (outbound); `"anonymous"` when no header is present | The peer's slug, interpreted per `direction` |
| `transport` | `string` | `"stdio"`, `"tcp"`, `"unix"`, `"ws"`, `"wss"` | Wire transport |
| `address` | `string` | `"stdio://"`, `"tcp://127.0.0.1:54321"`, etc. | Concrete endpoint |
| `direction` | `enum` | `INBOUND` / `OUTBOUND` | Server-side or client-side; picks which peer `remote_slug` names |
| `started_at` | `Timestamp` | Clock at gRPC channel ready | When the session became active |
| `ended_at` | `Timestamp` | Clock at session close | When the session reached `CLOSED` (zero if still open) |
| `instance_uid` | `string` | `OP_INSTANCE_UID` env (from supervisor, see [INSTANCES.md](INSTANCES.md)) | Owning process UID; empty for manually launched holons |

### Caller Identification

The caller SDK injects its holon slug as a gRPC metadata header
(`x-holon-slug`) on every outbound dial. The server SDK extracts it
from the first incoming RPC and populates the `INBOUND` session's
`remote_slug`. For `OUTBOUND` sessions, `remote_slug` is set from
the dial target (slug from `holon.proto` or the `--listen` URI). If
no `x-holon-slug` header is present on an inbound dial (e.g. a raw
gRPC client, a browser, a third-party tool), `remote_slug` is
`"anonymous"`.

This is lightweight (no handshake RPC), interceptor-friendly, and aligned
with standard gRPC metadata patterns.

### Lifecycle States

```
                    ┌──────────────┐
                    │  CONNECTING  │ ← dial / accept
                    └──────┬───────┘
                           │ gRPC READY
                           ▼
                    ┌──────────────┐
        ┌───────── │    ACTIVE    │ ─────────┐
        │          └──────┬───────┘          │
        │ heartbeat       │ Disconnect()     │ transport error
        │ missed          ▼                  ▼
   ┌────┴─────┐    ┌──────────────┐   ┌──────────┐
   │  STALE   │    │  DRAINING   │   │  FAILED  │
   └────┬─────┘    └──────┬───────┘   └────┬─────┘
        │ recovered       │ drain done      │ cleanup
        └─→ ACTIVE        ▼                 ▼
                    ┌──────────────┐
                    │   CLOSED    │ → history ring buffer
                    └──────────────┘
```

| State | Meaning | Trigger |
|---|---|---|
| `CONNECTING` | Dial initiated, gRPC channel not yet READY | Dial or accept entry |
| `ACTIVE` | Channel READY, RPCs flowing | `waitForReady` succeeds |
| `STALE` | No heartbeat response within threshold *(opt-in)* | Heartbeat timeout |
| `DRAINING` | `Disconnect()` called, in-flight RPCs finishing | `Disconnect()` entry |
| `FAILED` | Transport error or connect timeout | gRPC → `TransientFailure` / `Shutdown` |
| `CLOSED` | Session terminated and moved to history | Cleanup complete |

`STALE` only applies when heartbeat monitoring is enabled (opt-in via
`ConnectOptions`). Without heartbeat, sessions go directly from `ACTIVE`
to `FAILED` on transport error.

---

## Transport Constraints

Session semantics depend on the transport's structural properties
(see [COMMUNICATION.md §2.7](../../../COMMUNICATION.md#27-transport-properties)).
Not every transport behaves the same way, and the session store must
account for these differences.

### Per-Transport Session Behavior

| Scheme | Valence | Duplex | Max sessions | Reconnectable | Session identity |
|--------|:-------:|:------:|:---:|:---:|---|
| `tcp://` | Multi | Full | N | Yes | Per TCP connection (peer address) |
| `unix://` | Multi | Full | N | Yes | Per Unix connection (inode) |
| `stdio://` | **Mono** | **Simulated** | **1** | **No** | The pipe pair — tied to process lifetime |
| `ws://` | Multi | Full | N | Yes | Per WebSocket connection |
| `wss://` | Multi | Full | N | Yes | Per WebSocket connection (TLS) |

### Valence Constraints

**Monovalent transports** (`stdio://`) have exactly one session
per holon lifetime. The implications:

- **No session list** — there is at most one active session. The ring
  buffer may contain zero or one past session.
- **Session = process** — for `stdio://`, the session's lifetime is the
  child process's lifetime. `CLOSED` means the process exited.
- **No concurrent sessions** — the store enforces this: creating a
  second session on a monovalent transport is an error.

**Multivalent transports** (`tcp://`, `unix://`, `ws://`, `wss://`) can
have N concurrent sessions. Each accepted connection creates a new
session. The ring buffer matters here — a busy holon may see hundreds
of sessions over time.

### Duplex Constraints

**Full-duplex** transports have symmetric sessions — both sides can
initiate RPCs freely. The session's `rpc_count` tracks calls in both
directions combined.

**Simulated full-duplex** (`stdio://`) uses two unidirectional pipes
glued together. For session tracking this behaves identically to full
duplex — gRPC handles the framing. The only difference is that pipe
failure is **asymmetric**: if `stdout` is closed but `stdin` remains open,
the session should transition to `FAILED` (not `STALE`, since there is
no heartbeat recovery on a broken pipe).

### Process Coupling

On `stdio://`, the session is tightly coupled to the child process. The
session store must handle these scenarios:

| Event | Session transition | Notes |
|---|---|---|
| Child process exits cleanly | `ACTIVE` → `CLOSED` | Normal: `Disconnect()` or `SIGTERM` |
| Child process crashes | `ACTIVE` → `FAILED` → `CLOSED` | Pipe breaks → gRPC detects, session cleans up |
| Parent process exits | *child inherits broken pipe* | Child's serve runner detects and shuts down |
| Pipe closed by parent | `ACTIVE` → `FAILED` | Same as crash from child's perspective |

For `tcp://` and `unix://`, the session is decoupled from the process.
A holon can disconnect and reconnect — each reconnection is a **new
session** with a new UUID. The old session moves to `CLOSED` in the ring
buffer.

### Reconnection and Session Identity

When a client reconnects after a transport error (COMMUNICATION.md §6.2),
a **new session** is created. The old session transitions to `CLOSED`.
Reconnection does not reuse the same session UUID — identity is per
connection, not per logical relationship.

This means `op sessions --history` shows the reconnection pattern:

```
SESSION          REMOTE     TRANSPORT  STATE   STARTED       ENDED         RPCs
──────────────────────────────────────────────────────────────────────────────
d9e0f1a2-...     grace-op   tcp        active  12:05         —             12
a1b2c3d4-...     grace-op   tcp        closed  12:01         12:04:58      42
f3a4b5c6-...     grace-op   tcp        closed  11:50         12:00:55      87
```

Three sessions, same `remote_slug` — two reconnections visible in the
history.

### Resource Lifecycle Alignment

Session tracking itself must respect COMMUNICATION.md §6.4.2 (resource
lifecycle): after N connect/disconnect cycles, the session store's
memory footprint must remain bounded. The ring buffer guarantees this —
the store never grows beyond `max_history` closed sessions + N active
sessions.

### REST + SSE Sessions (v0.6)

REST + SSE ([DESIGN_transport_rest_sse.md](../../grace-op/v0.6/DESIGN_transport_rest_sse.md))
splits communication into two separate channels:

| Channel | Protocol | Lifetime | Session implication |
|---|---|---|---|
| Client → Server | POST | Per-request (stateless) | No persistent connection |
| Server → Client | SSE (EventSource) | Long-lived | Persistent, reconnectable |

This is fundamentally different from gRPC, where a single channel carries
all traffic. Sessions on REST + SSE must handle **split identity**:

- **SSE stream** = the session anchor. When an SSE connection is
  established, a session is created. The session stays `ACTIVE` as long
  as the EventSource is connected.
- **POST requests** are correlated to the session via an
  `X-Session-ID` header injected by the SDK. The server matches the
  POST to the SSE session and increments `rpc_count`.
- **No SSE** (pure REST, no streaming) = **stateless sessions**. Each
  POST creates a session in `ACTIVE`, counts the RPC, and immediately
  transitions to `CLOSED`. This floods the ring buffer, so the store
  coalesces: consecutive stateless calls from the same `remote_slug`
  within a window (e.g. 30s) merge into a single session entry.

**Auto-reconnect**: SSE's built-in `EventSource` reconnection creates a
continuity challenge. When EventSource reconnects, the SDK should reuse
the same session UUID (via `Last-Event-ID` carrying the session ID) to
avoid the history showing dozens of 1-second sessions. If the reconnect
fails beyond the backoff window, a new session is created.

**Half-duplex**: REST + SSE is not full-duplex — the client cannot send
while receiving. For session tracking, this doesn't matter — `rpc_count`
still tracks requests, and `last_rpc_at` still records the last POST.

Per-transport session table update:

| Scheme | Valence | Duplex | Max sessions | Reconnectable | Session identity |
|--------|:-------:|:------:|:---:|:---:|---|
| `rest+sse://` | Multi | **Half** | N | Yes (EventSource) | Per SSE stream or `X-Session-ID` header |
| `rest://` (no SSE) | Multi | **Half** | N | N/A (stateless) | Coalesced by `remote_slug` + time window |

### Mesh Sessions (v0.9)

Mesh networking ([DESIGN_mesh.md](../../grace-op/v0.9/DESIGN_mesh.md))
introduces cross-host connections secured by mTLS. Session tracking gains
new information from the TLS handshake:

| Field | Source | Description |
|---|---|---|
| `mesh_host` | TLS peer certificate CN | The remote host name from `mesh.yaml` |
| `tls_verified` | mTLS handshake result | Whether the peer presented a valid mesh cert |

These are added to `SessionInfo` only when the transport has a TLS layer.
For local transports (`stdio://`), these fields are empty.

**Cross-host session identity**: on a mesh, the same holon slug can
appear from multiple hosts. `remote_slug` alone is ambiguous — two
instances of `rob-go` on `paris.example.com` and `lyon.example.com` are
different peers. The session must record both `remote_slug` and
`mesh_host` to be unambiguous.

**Mesh-wide `op sessions --all`**: with mesh, `--all` expands beyond
local port files. `op` queries each mesh host's running holons (via
`op mesh describe`) and aggregates sessions across the network. This
is a supervisor-level view — it requires the operator to have mesh
credentials.

```
$ op sessions --all --mesh
paris.example.com:
  rob-go:      3 active (1 from lyon, 2 local)
  phill-files:  1 active (1 from lyon)
lyon.example.com:
  wisupaa:     2 active (2 from paris)

Total: 6 active across 2 hosts, 3 holons
```

### Public Holon Sessions (v0.10)

Public holons ([DESIGN_public_holons.md](../../grace-op/v0.10/DESIGN_public_holons.md))
serve external consumers via TLS + auth interceptors. Sessions on public
listeners gain **consumer identity** from the auth layer:

| Auth strategy | `remote_slug` source |
|---|---|
| API key | Key `name` from `holon.proto` (e.g. `"consumer-alpha"`) |
| JWT | `sub` claim from the token |
| OAuth | `sub` claim from validated token |
| None (mesh peer) | `x-holon-slug` header as before |

This means `remote_slug` on a public listener is the **authenticated
consumer name**, not a holon slug. The field serves double duty: holon
identity on the mesh, consumer identity on public endpoints.

**Per-listener `session_visibility`**: the global `session_visibility`
setting in `holon.proto` can be overridden per listener:

```yaml
serve:
  session_visibility: off          # global default: disabled

  listeners:
    - uri: stdio://
      session_visibility: full     # local: always full

    - uri: tcp://:9090
      security: mesh
      session_visibility: full     # mesh peers: full

    - uri: tcp://:443
      security: public
      auth: api-key
      session_visibility: off      # internet: never expose sessions
```

This makes the security model composable: a holon can expose full session
diagnostics on its mesh listener while hiding them entirely from the
public internet.

---

## Session History

Closed sessions are not discarded — they move to a **bounded ring buffer**
in the session store. This allows post-mortem debugging without unbounded
memory growth.

### Ring Buffer Parameters

| Parameter | Default | Description |
|---|:---:|---|
| `max_history` | 256 | Maximum closed sessions retained |
| `min_retention` | 1 hour | Minimum age before eviction (FIFO beyond max) |

When the buffer is full, the oldest entry is evicted. These defaults are
chosen to be useful for debugging without being a memory concern (each
`SessionInfo` is ~200 bytes → 256 entries ≈ 50 KB).

The ring buffer is in-memory only. It resets when the holon restarts.
Persistent history (log files, databases) is out of scope — that belongs
to observability tooling.

---

## Session Metrics

Beyond `rpc_count` and `last_rpc_at`, sessions can collect **detailed
metrics** as an opt-in second level of instrumentation. Metrics are
off by default even when sessions are enabled — they add interceptor
overhead for latency measurement and memory for per-method counters.

### Activation

Metrics are activated by setting `OP_SESSIONS=metrics` instead of
`OP_SESSIONS=1`:

```
OP_SESSIONS=1         # sessions only (identity + state + rpc_count)
OP_SESSIONS=metrics   # sessions + per-session metrics
```

`op run --sessions=metrics` sets this level.

### Time Decomposition

A single "latency" number is meaningless for long-running holons like
Megg FFmpeg, where a transcoding call can take minutes. The SDK
decomposes each RPC's total time into **four phases**:

```
  caller                           server
    │                                │
    ├─── wire_out ──────────────────▶│         request serialization + transport
    │                                ├─ queue  waiting for handler slot
    │                                ├─ work   handler execution
    │◀────────────────── wire_in ────┤         response serialization + transport
    │                                │
    total = wire_out + queue + work + wire_in
```

| Phase | Measured by | What it reveals |
|---|---|---|
| `wire_out` | Client SDK: time from send to first server byte | Transport latency, serialization cost |
| `queue` | Server SDK: time from accept to handler start | Concurrency saturation, backpressure |
| `work` | Server SDK: time inside the handler function | Actual computation (FFmpeg, Whisper, etc.) |
| `wire_in` | Client SDK: time from last server byte to client done | Response size, deserialization cost |

Each phase tracks independent P50/P99 histograms per method.

### Why This Matters

| Holon | Typical RPC | wire | queue | work | Diagnosis |
|---|---|:---:|:---:|:---:|---|
| `rob-go` | `Build()` | 1ms | 0ms | 800ms | Work-bound — build is the bottleneck |
| `phill-files` | `Read()` | 1ms | 0ms | 3ms | Wire-bound — I/O is fast |
| `gudule-greeting` | `SayHello()` | 1ms | 0ms | 0.1ms | Wire-dominated — trivial handler |

Without decomposition, `Transcode()` at P99 = 180s tells you nothing.
With it, you see `wire_out: 2ms, queue: 500ms, work: 179s, wire_in: 5ms`
— the problem is the transcoding itself, not the transport.

### Measurement Protocol

**Client-side** (outbound sessions): the client SDK timestamps before
send and after receive. It knows `wire_out` and `wire_in` but cannot
measure `queue` and `work` — those happen inside the server.

**Server-side** (inbound sessions): the server interceptor timestamps
at accept, handler entry, handler exit, and response flush. It knows
`queue` and `work` precisely.

**Correlation**: when both sides have sessions enabled, the full
4-phase picture is available by combining client and server views.
`op proxy`, sitting in the middle, can measure all four from a
single observation point.

### What Is Collected

| Metric | Type | Description |
|---|---|---|
| `total_p50_us` | `int64` | Median total RPC time (client-measured) |
| `total_p99_us` | `int64` | 99th percentile total RPC time |
| `wire_out_p50_us` | `int64` | Median request transport time |
| `wire_in_p50_us` | `int64` | Median response transport time |
| `queue_p50_us` | `int64` | Median server queue wait (server-side only) |
| `work_p50_us` | `int64` | Median handler execution time (server-side only) |
| `error_count` | `int64` | RPCs that returned a non-OK gRPC status |
| `bytes_sent` | `int64` | Total bytes sent on this session |
| `bytes_received` | `int64` | Total bytes received |
| `in_flight` | `int32` | Currently executing RPCs (useful for long calls) |
| `methods` | `map<string, MethodMetrics>` | Per-method breakdown |

Per-method breakdown includes all four phase histograms, call count,
error count, and `in_flight` for that specific method.

### Streaming-Aware Metrics

Server-streaming RPCs (e.g. `WatchBuild`, live transcription progress)
need special treatment:

- `work` is the **total handler time**, not the time to first message
- `wire_in` is measured as **time-to-first-message** (TTFM) for
  streaming calls, since clients care about responsiveness
- `messages_sent` and `messages_received` count stream messages
  (distinct from `bytes_sent` / `bytes_received`)

### Memory Budget

Metrics use a fixed-size structure per session:
- Base metrics: ~128 bytes (12 counters + in_flight)
- Per-method entry: ~160 bytes (method name + 4-phase counters)
- Latency percentiles (4 phases): HDR histogram sketch, ~8 KB per session

With 100 active sessions and 10 methods each, total ≈ 960 KB. This is
acceptable for a debug mode — well under 1 MB.

### CLI Display

```
$ op sessions rob-go --metrics
SESSION          REMOTE     STATE   RPCs  FLY  ERR  TOTAL   WIRE   QUEUE   WORK    SENT     RECV
────────────────────────────────────────────────────────────────────────────────────────────────
a1b2c3d4-...     grace-op   active  42    0    0    1.2ms   0.1ms  0ms     1.1ms   52 KB    1.2 MB

$ op sessions megg-ffmpeg --metrics
SESSION          REMOTE     STATE   RPCs  FLY  ERR  TOTAL    WIRE    QUEUE    WORK     SENT     RECV
────────────────────────────────────────────────────────────────────────────────────────────────
b7c8d9e0-...     grace-op   active  3     1    0    3m02s    7ms    500ms    3m01s    1 KB     2.4 GB

$ op sessions megg-ffmpeg --metrics --methods
SESSION b7c8d9e0-...  (grace-op, active, 1 in-flight)
  METHOD                              CALLS  FLY  ERR  TOTAL   WIRE    QUEUE   WORK
  media.v1.MediaService/Transcode       1      1    0    3m02s   7ms     500ms   3m01s
  media.v1.MediaService/Probe           2      0    0    85ms    2ms     0ms     83ms
```

### Relationship to External Observability

Session metrics are a lightweight, zero-dependency diagnostic tool that
works out of the box — even on an airgapped machine with no observability
stack. They are **not** a replacement for Prometheus, OpenTelemetry, or
structured logging. But they can **feed into** those systems.

#### `op proxy` as Metrics Collector

> **v2 (requires `op proxy`).** In v1, the Prometheus `/metrics`
> endpoint lives **in the holon itself** (see
> [OBSERVABILITY.md §Collection & Export](OBSERVABILITY.md#collection--export)),
> not in the proxy. When `op proxy` ships, it will aggregate child
> `/metrics` endpoints and expose a unified surface, but the per-holon
> exposition is load-bearing on its own.

Because `op proxy` governs network topology, it is the
natural bridge between session metrics and production observability. It
already collects per-method latency, error rates, and call counts.
With session awareness, `op proxy` natively acts as a
**zero-config Prometheus exporter**:

- `op proxy` exposes a `/metrics` HTTP endpoint in Prometheus exposition
  format, scraped by any standard Prometheus instance.
- Each metric is labeled with session metadata: `remote_slug`,
  `transport`, `direction`, `method` — enabling Grafana dashboards
  that show per-peer, per-method, per-transport breakdowns.
- The operator relies entirely on `op proxy` without needing any secondary sidecars.

This provides a remarkably clean observability stack:

| Layer | Tool | Scope | Dependency |
|---|---|---|---|
| **In-process** | Session metrics (`OP_SESSIONS=metrics`) | Per-holon, in-memory | None |
| **Network** | `op proxy` + Prometheus exporter | Cross-holon, time-series | Natively handled by `op` |
| **Full stack** | OpenTelemetry SDK integration | Distributed tracing, logs | OTel collector + backend |

The session model is designed to feed all three layers. `SessionMetrics`
is the in-process source of truth; `op proxy` aggregates and exports across multiplexed connections; OTel
provides the full distributed picture.

---

## Proto Definition

A new `.proto` file in the shared SDK protos. Like `HolonMeta`, this is
auto-registered by every SDK — holons don't import or implement it.

```protobuf
syntax = "proto3";
package holonsession.v1;

import "google/protobuf/timestamp.proto";

// HolonSession is auto-registered by the SDK's serve runner
// when OP_SESSIONS is enabled. Provides session introspection.
service HolonSession {
  // Sessions returns active and optionally past sessions.
  rpc Sessions(SessionsRequest) returns (SessionsResponse);

  // WatchSessions streams session lifecycle events as they happen.
  // Used by `op sessions --watch` and by tooling that needs live
  // visibility without polling. When the client disconnects, the
  // server-side subscription is torn down.
  rpc WatchSessions(WatchSessionsRequest) returns (stream SessionEvent);
}

message SessionsRequest {
  // Filter by state. Empty = all non-CLOSED sessions.
  repeated SessionState state_filter = 1;

  // Filter by direction. UNSPECIFIED = both.
  SessionDirection direction_filter = 2;

  // Include closed sessions from the history ring buffer.
  bool include_closed = 3;

  // Maximum number of sessions to return. 0 = server default (100).
  // Protects against unbounded output.
  int32 limit = 4;

  // Opaque continuation token from a previous response.
  // Empty = start from the beginning.
  string page_token = 5;
}

message SessionsResponse {
  // The holon's own slug.
  string slug = 1;

  // Matching sessions, ordered by started_at descending.
  repeated SessionInfo sessions = 2;

  // Continuation token for the next page. Empty = no more results.
  string next_page_token = 3;

  // Total number of sessions matching the filter (across all pages).
  int32 total_count = 4;
}

message SessionInfo {
  // Unique session identifier (UUID v4).
  string session_id = 1;

  // Slug of the remote holon ("anonymous" if unknown).
  string remote_slug = 2;

  // Transport scheme used for this session.
  string transport = 3;

  // Concrete transport address.
  string address = 4;

  // Whether this holon is the server or client side.
  SessionDirection direction = 5;

  // Current lifecycle state.
  SessionState state = 6;

  // When the session was created (dial or accept).
  google.protobuf.Timestamp started_at = 7;

  // When the state last changed.
  google.protobuf.Timestamp state_changed_at = 8;

  // When the session reached CLOSED (zero if still open).
  google.protobuf.Timestamp ended_at = 9;

  // Number of RPCs completed in this session.
  int64 rpc_count = 10;

  // Last RPC completed timestamp (zero if none).
  google.protobuf.Timestamp last_rpc_at = 11;

  // Optional per-session metrics (only when OP_SESSIONS=metrics).
  SessionMetrics metrics = 20;

  // Mesh host name (only on mTLS connections, empty otherwise).
  string mesh_host = 21;

  // Owning instance UID (see INSTANCES.md). Populated from OP_INSTANCE_UID
  // set by the parent supervisor. Empty for manually launched holons.
  // This is the join key for observability signals (OBSERVABILITY.md).
  string instance_uid = 22;
}

// SessionMetrics is populated only when OP_SESSIONS=metrics.
// Time is decomposed into four phases: wire_out, queue, work, wire_in.
message SessionMetrics {
  // Total RPC time (client-measured end-to-end).
  int64 total_p50_us = 1;
  int64 total_p99_us = 2;

  // Request transport time (client → server).
  int64 wire_out_p50_us = 3;
  int64 wire_out_p99_us = 4;

  // Server queue wait (accept → handler start). Server-side only.
  int64 queue_p50_us = 5;
  int64 queue_p99_us = 6;

  // Handler execution time. Server-side only.
  int64 work_p50_us = 7;
  int64 work_p99_us = 8;

  // Response transport time (server → client).
  int64 wire_in_p50_us = 9;
  int64 wire_in_p99_us = 10;

  int64 error_count = 11;
  int64 bytes_sent = 12;
  int64 bytes_received = 13;
  int32 in_flight = 14;          // currently executing RPCs

  // Streaming counters (for server-streaming / bidi RPCs).
  int64 messages_sent = 15;
  int64 messages_received = 16;

  map<string, MethodMetrics> methods = 20;
}

message MethodMetrics {
  int64 call_count = 1;
  int64 error_count = 2;
  int32 in_flight = 3;

  // Per-phase decomposition (all in microseconds).
  int64 total_p50_us = 10;
  int64 total_p99_us = 11;
  int64 wire_out_p50_us = 12;
  int64 wire_out_p99_us = 13;
  int64 queue_p50_us = 14;
  int64 queue_p99_us = 15;
  int64 work_p50_us = 16;
  int64 work_p99_us = 17;
  int64 wire_in_p50_us = 18;
  int64 wire_in_p99_us = 19;
}

message WatchSessionsRequest {
  // Filter by state. Empty = all non-CLOSED sessions.
  repeated SessionState state_filter = 1;

  // Filter by direction. UNSPECIFIED = both.
  SessionDirection direction_filter = 2;

  // If true, an initial SessionEvent with kind=SNAPSHOT is emitted
  // for every currently matching session before the stream transitions
  // to live events. Useful for clients that need a consistent view.
  bool send_initial_snapshot = 3;
}

message SessionEvent {
  // When the event was observed.
  google.protobuf.Timestamp ts = 1;

  // What kind of change occurred.
  SessionEventKind kind = 2;

  // The session involved. Always populated.
  SessionInfo session = 3;

  // For STATE_CHANGED: the previous state. Zero for other kinds.
  SessionState previous_state = 4;
}

enum SessionEventKind {
  SESSION_EVENT_KIND_UNSPECIFIED = 0;
  SNAPSHOT = 1;          // replay of pre-existing session at stream start
  SESSION_CREATED = 2;   // new SessionInfo entered the store
  STATE_CHANGED = 3;     // transition between lifecycle states
  METRICS_UPDATED = 4;   // rpc_count / last_rpc_at changed (rate-limited)
  SESSION_CLOSED = 5;    // transitioned to CLOSED and moved to history
}

enum SessionState {
  SESSION_STATE_UNSPECIFIED = 0;
  CONNECTING = 1;
  ACTIVE = 2;
  STALE = 3;
  DRAINING = 4;
  FAILED = 5;
  CLOSED = 6;
}

enum SessionDirection {
  SESSION_DIRECTION_UNSPECIFIED = 0;
  INBOUND = 1;
  OUTBOUND = 2;
}
```

### Manifest additions

The visibility vocabulary needs a home in the manifest proto. These
additions are shared with [OBSERVABILITY.md](OBSERVABILITY.md) — the
same enum gates both `HolonSession.*` and `HolonObservability.*`.

```protobuf
// Added to holons/v1/manifest.proto

enum ObservabilityVisibility {
  OBSERVABILITY_VISIBILITY_UNSPECIFIED = 0;
  OFF = 1;       // Sessions / Metrics / Logs / Events return PERMISSION_DENIED
  SUMMARY = 2;   // Counts and states only; no payloads, no session/method ids
  FULL = 3;      // All fields returned
}

message ListenerVisibilityOverride {
  // Listener URI to apply the override to. Matches the listener's
  // `uri` field in the manifest's `serve.listeners` list.
  string listener_uri = 1;

  ObservabilityVisibility visibility = 2;
}

// HolonManifest gains two optional fields:
//   ObservabilityVisibility session_visibility = 16;
//   repeated ListenerVisibilityOverride session_visibility_overrides = 17;
//
// When `session_visibility` is UNSPECIFIED, the SDK infers the default
// from the active listener's transport scheme (see the table above).
```

The field is named `session_visibility` for continuity with this
spec; the enum type `ObservabilityVisibility` signals that the same
dial applies to logs, metrics, and events in `HolonObservability`.
A single knob, a single source of truth.

---

## SDK Changes (Go Reference)

### New package: `pkg/session`

A thread-safe session store shared between `connect` (client) and `serve`
(server):

```go
type Store struct { ... }

func NewStore() *Store
func (s *Store) Create(id, remoteSlug, transport, address string, dir Direction) *Session
func (s *Store) Get(id string) *Session
func (s *Store) Transition(id string, state State) error
func (s *Store) List(filter ListFilter) ListResult
func (s *Store) Close(id string) // moves to ring buffer
```

`ListResult` supports pagination:

```go
type ListFilter struct {
    States     []State
    Direction  Direction
    Limit      int    // 0 = default (100)
    PageToken  string
}

type ListResult struct {
    Sessions      []*Session
    NextPageToken string
    TotalCount    int
}
```

### Client side — outbound sessions

- Any outbound dial (via `Connect()`, `ConnectWithOpts()`, or direct
  `grpc.Dial` through the SDK) generates a UUID, creates a session in
  `CONNECTING`, injects `x-holon-slug` metadata, transitions to `ACTIVE`
  after `waitForReady`, and to `FAILED` on error.
- `Disconnect()` transitions to `DRAINING` → `CLOSED`.
- The session store is a package-level singleton (replaces the existing
  `started` map in `pkg/connect`).

### Server side — inbound sessions

- The serve runner creates a `session.Store` and passes it to an internal
  `HolonSession` gRPC service implementation.
- On each accepted connection — regardless of how the client dialed —
  the server creates an `INBOUND` session (transport + peer address from
  the listener).
- The `Sessions` RPC reads from the store, applying filters and pagination.
- Holons launched manually (`./my-holon serve --listen tcp://:9090`)
  track sessions identically to those launched via `connect()` or `op run`.

### `HolonSession` exclusions

- Excluded from `HolonMeta.Describe` output (like `HolonMeta` itself).
- The `rpc_count` and `last_rpc_at` fields use atomic operations — zero
  overhead for normal RPC flow.

---

## `op` Integration

### `op sessions <slug>`

Calls `HolonSession.Sessions` on a running holon:

```
$ op sessions rob-go
SESSION                                  REMOTE     TRANSPORT  ADDRESS                  STATE   STARTED              RPCs
─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
a1b2c3d4-...-1e2f3a4b5c6d               grace-op   stdio      stdio://                 active  2026-03-12 12:01     42
d9e0f1a2-...-9b0c1d2e3f4a               anonymous  tcp        tcp://127.0.0.1:54321    active  2026-03-12 12:05     7

2 active sessions
```

### `op sessions <slug> --history`

Includes closed sessions from the ring buffer:

```
$ op sessions rob-go --history
SESSION                                  REMOTE     TRANSPORT  ADDRESS                  STATE   STARTED              RPCs
─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
a1b2c3d4-...-1e2f3a4b5c6d               grace-op   stdio      stdio://                 active  2026-03-12 12:01     42
d9e0f1a2-...-9b0c1d2e3f4a               anonymous  tcp        tcp://127.0.0.1:54321    active  2026-03-12 12:05     7
f3a4b5c6-...-1a2b3c4d5e6f               jess-npm   stdio      stdio://                 closed  2026-03-12 11:45     128

2 active, 1 closed (showing 3 of 3)
```

### `op sessions --all`

Discovers all running holons (via port files) and aggregates:

```
$ op sessions --all
rob-go:     2 active, 1 closed
jess-npm:   0 active
who:        1 active

Total: 3 active, 1 closed across 3 holons
```

### Pagination flags

| Flag | Default | Description |
|---|:---:|---|
| `--limit` | 100 | Maximum rows per page |
| `--page` | *(none)* | Continuation token from previous output |

When output exceeds the limit, the last line shows:

```
Showing 100 of 347 sessions. Next page: --page=eyJvZmZzZXQiOjEwMH0=
```

---

## Cross-SDK Implementation Notes

Each SDK implements the same model. Key considerations:

| SDK | Session store | Interceptor mechanism | HDR histogram library |
|---|---|---|---|
| **Go** | `sync.Mutex` + `map` + ring buffer | gRPC `UnaryServerInterceptor` | `HdrHistogram/hdrhistogram-go` |
| **Swift** | Actor-based store | `ServerInterceptorFactory` | `HdrHistogram.swift` |
| **Dart** | `Map` + `StreamController` | gRPC interceptor | `hdr_histogram` (pub) |
| **Rust** | `Arc<Mutex<HashMap>>` + `VecDeque` | tonic `Interceptor` | `hdrhistogram` crate |
| **Kotlin** | `ConcurrentHashMap` + `ArrayDeque` | gRPC `ServerInterceptor` | `HdrHistogram` (JVM) |
| **C#** | `ConcurrentDictionary` + bounded channel | gRPC interceptor | `HdrHistogram.NET` |
| **Python** | `dict` + `deque(maxlen=N)` | gRPC interceptor | `hdrh` |
| **Node.js** | `Map` + circular buffer | gRPC middleware | `hdr-histogram-js` |
| **Java** | `ConcurrentHashMap` + `ArrayDeque` | gRPC `ServerInterceptor` | `HdrHistogram` |
| **Ruby** | `Hash` + bounded array | gRPC interceptor | `hdrhistogram` gem |
| **C** | hash table + ring buffer | interceptor via bridge | `HdrHistogram_c` |
| **C++** | `std::unordered_map` + mutex | gRPC interceptor | `HdrHistogram_c` |
| **JS-web** | `Map` + circular buffer | gRPC-web middleware | `hdr-histogram-js` |

The proto definition is canonical. All SDKs implement the same states and
the same `Sessions` RPC response shape.

---

## Security Considerations

The `HolonSession.Sessions` RPC is auto-registered on every holon.
Because any holon can `connect()` to any other holon and call its RPCs,
the `Sessions` endpoint is reachable by any peer — not just `op`. This
raises a legitimate concern: **a holon could query another holon's active
sessions and learn who else is connected, from where, and since when.**

### Threat Model

| Risk | Severity | Description |
|---|:---:|---|
| **Connection enumeration** | Medium | A malicious holon enumerates all peers connected to a target, revealing the network topology. |
| **Timing analysis** | Low | `started_at` / `last_rpc_at` reveals activity patterns. |
| **Address leak** | Medium | `address` exposes internal IPs and port numbers. |

### Mitigation: Visibility Levels

The `Sessions` RPC respects a **visibility** policy declared in
`holon.proto`. The SDK enforces it before returning results:

| Visibility | Behavior | Default for |
|---|---|---|
| `full` | All fields returned | `stdio://` (local-only transports) |
| `summary` | Returns `session_id`, `state`, `rpc_count`. Omits `remote_slug`, `address`, `transport`. | `tcp://`, `unix://` |
| `off` | `Sessions` returns `PERMISSION_DENIED` (code 7) | `ws://`, `wss://` (internet-facing) |

Configuration in `holon.proto`:

```yaml
session_visibility: summary   # full | summary | off
```

When not declared, the SDK infers the default from the active listener's
transport scheme (see table above). A holon serving on `tcp://` defaults
to `summary` — you see that 3 sessions are active but not who they are.

### `op` Is Not Special

`op sessions` calls the same RPC as any other holon. It gets the same
visibility level. For full diagnostics on a private holon, the operator
can:

1. Set `session_visibility: full` in `holon.proto` (development).
2. Use `stdio://` transport (inherently local, defaults to `full`).
3. Add the future `security: mesh` listener (mTLS peers get `full`).

This follows the existing security model from
[DESIGN_public_holons.md](../../../design/grace-op/v0.10/DESIGN_public_holons.md):
the SDK enforces policy, the holon developer declares it, and `op` has no
privileged backdoor.

---

## Open Questions

1. **Persistent history** — is the in-memory ring buffer sufficient for
   v1, or should a file-backed log be considered? Note:
   [INSTANCES.md §Log Contract](INSTANCES.md#log-contract) already
   defines `events.jsonl` per-instance on disk, and `SESSION_STARTED` /
   `SESSION_ENDED` events are emitted into it; this gives post-mortem
   reconstruction without a dedicated sessions log. The question
   remains whether `HolonSession.Sessions --history` should read the
   disk store as a fallback when the in-memory ring is exhausted.
2. **Holon-RPC sessions** — WebSocket connections via Holon-RPC binding
   should have sessions too. Same model, different transport metadata. Is
   this in scope for v1?

*Open Question #2 from an earlier draft — `WatchSessions` streaming RPC
— has been adopted and is part of the service definition above.*
