# Jack-Middle v0.1 — Transparent gRPC Proxy

Jack Middle is not a holon that offers its own services. He is
a **man-in-the-middle**: a transparent gRPC proxy that can
impersonate any holon and relay all traffic to the real backend,
while providing middleware hooks for observation and mutation.

---

## Problem

1. **No observability**: When holon A calls holon B via `connect("b")`,
   the gRPC exchange is invisible. There is no way to trace,
   log, or profile the RPCs without instrumenting both sides.
2. **No interception**: Testing scenarios like injecting faults,
   adding latency, or modifying payloads require custom code
   in each holon — there is no reusable middleware layer.
3. **No instrumentation**: Performance profiling, call counting,
   and payload inspection require per-holon changes.

## Solution

Jack Middle is a **generic gRPC reverse proxy** that:
- Accepts any gRPC call on its frontend (client-facing side)
- Forwards it to a real holon backend (server-facing side)
- Applies a configurable middleware chain in between

He doesn't need to know the proto definitions of the holon he
impersonates — he works at the raw gRPC frame level.

### Identity

```
Given name: Jack
Family name: Middle
Motto: I see everything.
Kind: native
Clade: deterministic/side-effects
```

### The Jack Griffin Principle

> When it is not possible to be invisible, be visible.

Jack does **not** impersonate the target holon's identity on
the network. He is a first-class participant — his own mesh
member with his own certificate. The caller knows it is talking
to Jack, and Jack knows he is talking to the target. Transparency,
not deception.

---

## Architecture

```
                     ┌─────────────────────────────────┐
   Caller            │          Jack Middle              │          Target
  (holon A)          │                                   │         (holon B)
     │               │  ┌──────────┐   ┌─────────────┐  │            │
     │──── gRPC ────▶│  │ Frontend │──▶│ Middleware   │  │            │
     │               │  │ Listener │   │    Chain     │  │            │
     │               │  └──────────┘   └──────┬──────┘  │            │
     │               │                        │         │            │
     │               │                 ┌──────┴──────┐  │            │
     │◀── gRPC ─────│  │              │  Backend    │──┼────gRPC───▶│
     │               │                 │  Dial       │  │            │
     │               │                 └─────────────┘  │            │
     └───────────────┴─────────────────────────────────┘            │
```

---

## Runtime Models

Jack can be deployed in three ways, from most explicit to
most transparent.

### Model 1 — Explicit Sidecar

A caller holon launches Jack as a sidecar for a specific
outbound dependency.

```
caller → connect("jack-middle --target rob-go") → Jack → rob-go
```

**Who decides**: caller holon code.
**Limitation**: requires modifying caller's connect call.

### Model 2 — Port File Hijack

Jack starts, connects to the real target, then overwrites
`.op/run/<slug>.port` with its own frontend address. All
subsequent `connect(slug)` calls from any holon transparently
route through Jack.

```bash
jack-middle --target rob-go --hijack
```

**Who decides**: operator (CLI).
**Limitation**: TCP/Unix only (stdio has no port file).

### Model 3 — `op` Injection (v0.2)

Grace orchestrates the interposition. The caller, Jack, and
the target are all launched and wired by `op`.

```bash
op serve rob-go --via jack --middleware logger,metrics
```

Or in a recipe/assembly manifest:

```yaml
members:
  - id: daemon
    path: rob-go
    proxy:
      middleware: [logger, metrics, recorder]
      plugins: [snoopy-inspect]
```

**Stdio piping**: For stdio transports, `op` builds a pipeline:

```
caller ↔ jack (stdin/stdout pipe) ↔ target (stdin/stdout pipe)
```

Jack's stdin receives the caller's gRPC frames, applies the
middleware chain, and forwards to the target's stdin. Responses
flow back. This is the Unix process-pipe model applied to gRPC.

**Who decides**: `op` / recipe manifest.
**Limitation**: requires Grace v0.4+ support.

| Model | Who decides | Config location | Transports | Version |
|---|---|---|---|---|
| Sidecar | Caller code | Source code | All | v0.1 |
| Hijack | Operator | CLI flag | TCP, Unix | v0.1 |
| `op` injection | Grace/recipe | Manifest | All (incl. stdio) | v0.2 |

---

## Proxy Mechanics

### Transparent Forwarding

Jack uses gRPC's `UnknownServiceHandler` — a server option
that catches all RPCs for unregistered services and forwards
them to the backend connection.

```go
grpc.UnknownServiceHandler(func(srv interface{}, stream grpc.ServerStream) error {
    // Extract method name from stream
    // Apply middleware chain (before)
    // Forward to backend via ClientStream
    // Apply middleware chain (after)
    // Return response to caller
})
```

This means Jack does **not** need generated stubs for any
holon — he operates on raw bytes and method names.

### Supported Call Types

| Pattern | Mechanism |
|---|---|
| Unary (req → resp) | Frame relay with middleware hooks |
| Server streaming | Frame-by-frame relay |
| Client streaming | Frame-by-frame relay |
| Bidirectional | Frame-by-frame relay |

### Metadata Propagation

All gRPC metadata (headers, trailers) is forwarded transparently.
Jack can add its own metadata (e.g. `x-jack-trace-id`).

---

## Middleware Chain

Jack applies a chain of middleware to each RPC. Middleware
executes in order on the request path, reverse order on the
response path. Two tiers exist:

1. **Built-in middleware** — compiled into Jack, zero-hop,
   maximum performance.
2. **Plugin holons** — external holons implementing a middleware
   contract. Any language, full OP isolation, one extra hop.

### Tier 1 — Built-in Middleware

| Name | Purpose |
|---|---|
| `logger` | Log method, duration, status code, payload size |
| `tracer` | Assign trace IDs, emit spans (OpenTelemetry-ready) |
| `metrics` | Count RPCs, measure latency, track error rates |
| `recorder` | Record full request/response payloads for replay |
| `latency` | Inject artificial latency |
| `fault` | Return synthetic errors for chaos testing |

### Tier 2 — Plugin Holons

A plugin holon is any holon that implements the
`middleware.v1.PluginService` contract. Jack connects to it
via the standard `connect(slug)` algorithm and delegates
each intercepted RPC to it.

```
caller → Jack → [built-in 1] → [built-in 2] → [plugin holon A] → [plugin holon B] → target
```

#### Plugin Contract

```protobuf
syntax = "proto3";
package middleware.v1;

service PluginService {
  // Intercept is called for each proxied RPC.
  // The plugin inspects / mutates the call and returns a verdict.
  rpc Intercept(InterceptRequest) returns (InterceptResponse);

  // Describe returns the plugin's capabilities and metadata.
  rpc Describe(DescribeRequest) returns (DescribeResponse);
}

message InterceptRequest {
  string full_method = 1;           // e.g. "/go.v1.GoService/Build"
  bytes  request_payload = 2;       // raw protobuf frame
  map<string, string> metadata = 3; // gRPC headers
  Direction direction = 4;          // REQUEST or RESPONSE
}

enum Direction {
  REQUEST = 0;
  RESPONSE = 1;
}

message InterceptResponse {
  Verdict verdict = 1;
  bytes   payload = 2;              // potentially mutated payload
  map<string, string> metadata = 3; // potentially mutated headers
  int32   status_code = 4;          // only for REJECT verdict
  string  status_message = 5;
}

enum Verdict {
  FORWARD = 0;   // pass through (payload may be mutated)
  REJECT = 1;    // block the call, return status_code to caller
  SKIP = 2;      // pass through, don't call further plugins
}

message DescribeRequest {}

message DescribeResponse {
  string name = 1;
  string description = 2;
  repeated string capabilities = 3;  // e.g. ["inspect", "mutate", "block"]
}
```

#### Plugin Lifecycle

1. Jack starts → reads `--plugin slug1,slug2` from CLI
2. For each plugin: `connect(slug)` → readiness check
3. For each RPC: built-in chain first, then plugin chain
4. Each plugin receives `InterceptRequest`, returns `InterceptResponse`
5. Verdict `FORWARD` → next plugin; `REJECT` → abort; `SKIP` → jump to target

#### Plugin Catalogue

| Plugin Holon | Slug | Capabilities | Purpose |
|---|---|---|---|
| **Snoopy Inspect** | `snoopy-inspect` | inspect | Deep proto-aware payload inspection (knows proto descriptors, pretty-prints fields) |
| **Morph Rewrite** | `morph-rewrite` | mutate | Rule-based payload rewriting (e.g. anonymize fields, inject headers) |
| **Gate Guard** | `gate-guard` | block | ACL-based call authorization (method allowlists, rate limiting) |
| **Tempo Delay** | `tempo-delay` | inspect | Latency profiling with statistical analysis (P50/P95/P99) |
| **Echo Replay** | `echo-replay` | inspect, mutate | Record-and-replay: capture exchanges, replay them deterministically |
| **Canary Split** | `canary-split` | mutate | A/B routing: send a percentage of traffic to an alternate backend |
| **Schema Audit** | `schema-audit` | inspect | Validate payloads against proto descriptors, flag unknown fields |
| **Tap Stream** | `tap-stream` | inspect | Real-time event streaming to an external sink (WebSocket, Kafka, file) |

These are **future holons**, not part of v0.1. They demonstrate
the extensibility of the plugin model.

### Middleware Interface (built-in)

```go
// Interceptor is a built-in middleware function.
type Interceptor func(ctx context.Context, call *Call, next Handler) error

// Call captures the RPC metadata visible to middleware.
type Call struct {
    FullMethod  string    // e.g. "/go.v1.GoService/Build"
    StartTime   time.Time
    Request     []byte    // raw protobuf (lazy-decoded)
    Response    []byte    // filled by next handler
    StatusCode  codes.Code
    Metadata    metadata.MD
    Duration    time.Duration
}

// Handler is the next step in the chain.
type Handler func(ctx context.Context, call *Call) error
```

---

## Configuration

Jack is configured via CLI flags, not a static config file.
This keeps him simple and composable.

```bash
jack-middle \
  --listen tcp://127.0.0.1:0 \
  --target rob-go \
  --middleware logger,metrics,recorder \
  --plugin snoopy-inspect \
  --record-dir /tmp/jack-traces/
```

| Flag | Description |
|---|---|
| `--listen` | Frontend listener URI (tcp, unix, stdio) |
| `--target` | Holon slug or direct URI to proxy to |
| `--middleware` | Comma-separated built-in middleware chain |
| `--plugin` | Comma-separated plugin holon slugs (Tier 2 chain) |
| `--record-dir` | Directory for recorded payloads (recorder middleware) |
| `--latency` | Artificial latency for `latency` middleware (e.g. `200ms`) |
| `--fault-rate` | Error injection probability for `fault` middleware (0.0–1.0) |
| `--fault-code` | gRPC status code for injected faults (e.g. `UNAVAILABLE`) |

### Port File Hijacking

To transparently intercept all callers of a target holon:

```bash
jack-middle --target rob-go --hijack
```

This starts the real `rob-go`, then overwrites
`.op/run/rob-go.port` with Jack's frontend address. All
subsequent `connect("rob-go")` calls route through Jack.

---

## Proto Contract

Jack exposes a small **control service** for querying and
managing the proxy at runtime:

```protobuf
syntax = "proto3";
package middle.v1;

service MiddleService {
  // Query proxy status and statistics.
  rpc Status(StatusRequest) returns (StatusResponse);

  // List recorded calls (when recorder middleware is active).
  rpc ListRecords(ListRecordsRequest) returns (ListRecordsResponse);

  // Hot-reload the middleware chain.
  rpc SetMiddleware(SetMiddlewareRequest) returns (SetMiddlewareResponse);
}
```

This control service is registered alongside the
`UnknownServiceHandler`, making it the **only** service Jack
knows about — everything else is forwarded.

---

## holon.yaml

```yaml
# ── Identity ──────────────────────────────────────────
schema: holon/v0
uuid: "a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d"
given_name: Jack
family_name: Middle
motto: I see everything.
composer: B. ALTER
clade: deterministic/side-effects
status: draft
born: "2026-03-12"

# ── Description ───────────────────────────────────────
description: |
  Transparent gRPC man-in-the-middle proxy for the OP ecosystem.
  Jack can impersonate any holon, relay traffic to its real backend,
  and apply a configurable middleware chain for logging, tracing,
  metrics, recording, and fault injection.

# ── Contract ──────────────────────────────────────────
contract:
  service: middle.v1.MiddleService

# ── Operational ───────────────────────────────────────
kind: native
build:
  runner: go-module
  main: ./cmd/jack-middle
artifacts:
  binary: jack-middle
```

---

## What Does Not Change

- **Holon contracts** — Target holons are unmodified. Jack
  forwards raw gRPC frames; he doesn't depend on any proto.
- **Connect flow** — The standard `connect(slug)` algorithm
  is unchanged. Jack integrates via port file or explicit slug.
- **Transport layer** — All OP transports (tcp, unix, stdio)
  work. Jack just needs to listen on one and dial on another.

---

## Impact on the Ecosystem

- **Debugging**: `jack-middle --target rob-go --middleware logger`
  gives instant visibility into every RPC Rob handles.
- **Testing**: `--middleware fault --fault-rate 0.5` enables
  chaos testing without modifying any holon.
- **Profiling**: `--middleware metrics` provides call counts
  and latency percentiles per method.
- **Replay**: `--middleware recorder` captures full exchanges
  for regression testing.
- **CI**: Jack can be inserted into recipe test runs to
  validate that holon interactions match expectations.
