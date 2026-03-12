# Sessions — Roadmap

Feature versioning for transport-level session identification,
introspection, and metrics across the Organic Programming SDK ecosystem.

---

## v0.1 — Specification & Protocol ✍️

Define the session model, proto contract, activation mechanism,
transport constraints, and security model. Sessions are anchored
at the transport/serve layer, not tied to `connect()`.

- Session model (UUID, transport, address, direction, states)
- Lifecycle state machine (CONNECTING → ACTIVE → CLOSED)
- `OP_SESSIONS` activation model (parent-controlled debug mode)
- Transport constraints (valence, duplex, process coupling)
- `HolonSession` proto definition (`holonsession.v1`)
- Security: visibility levels (full / summary / off)
- Ring buffer history design + pagination

**Deliverable:** [DESIGN.md](./v0.1/DESIGN.md) ← current document

---

## v0.2 — Go Reference Implementation

Implement the session store and wire it into `go-holons` SDK.
This is the reference that all other SDKs will follow.

- `pkg/session` — `Store`, `Session`, state machine, ring buffer
- Client side: `Connect()` / `Disconnect()` session lifecycle
- Server side: `serve.Run` → `HolonSession` auto-registration
- `x-holon-slug` metadata interceptor (caller identification)
- Unit tests: store CRUD, state transitions, concurrency, ring buffer
- Integration tests: extend existing `connect_test.go`
- `OP_SESSIONS=1` env var activation in serve runner

---

## v0.3 — Session Metrics

Add the opt-in second-level instrumentation to the Go reference.

- `OP_SESSIONS=metrics` activation level
- Per-session latency percentiles (HDR histogram sketch)
- Error count, bytes sent/received
- Per-method breakdown (`MethodMetrics`)
- Memory budget validation (≤ 300 KB for 100 sessions × 10 methods)
- `SessionMetrics` proto message

---

## v0.4 — `op` CLI & Recipe Integration

Wire sessions into grace-op commands and recipe runner.

- `op sessions <slug>` — query a running holon's sessions
- `op sessions <slug> --history` — include closed sessions
- `op sessions <slug> --metrics` / `--methods` — metrics display
- `op sessions --all` — aggregate across local running holons
- `op run <slug> --sessions` / `--sessions=metrics` — activation flag
- `--limit` / `--page` pagination flags
- Table formatter for session output
- **Recipe propagation**: `op run --sessions` on a recipe assembly
  sets `OP_SESSIONS` for all daemons in the recipe graph
- **Recipe-aware display**: `op sessions --recipe <recipe>` shows
  sessions grouped by member holon within the assembly
- Update `recipes.yaml` documentation with session activation examples

---

## v0.5 — Jack Middle Integration

Make Jack session-aware and add Prometheus/Grafana export.
Depends on Jack Middle v0.1 and sessions v0.3.

- Jack activates/inhibits sessions per side (frontend, backend)
- Three modes: observe, frontend-only, inhibit
- Session IDs correlated with Jack's middleware metrics
- Prometheus `/metrics` HTTP endpoint with session labels
  (`remote_slug`, `transport`, `direction`, `method`)
- Grafana dashboard templates for per-peer, per-method breakdown
- `MiddleService.SetMiddleware` toggles session tracing at runtime

---

## v0.6 — Cross-SDK Ports

Port the session store and `HolonSession` service to all SDKs.
Each port implements v0.2 + v0.3 scope.

| SDK | Store primitive | Interceptor | Priority |
|---|---|---|:---:|
| Rust | `Arc<Mutex<HashMap>>` + `VecDeque` | tonic `Interceptor` | 1 |
| Swift | Actor-based store | `ServerInterceptorFactory` | 2 |
| Dart | `Map` + `StreamController` | gRPC interceptor | 2 |
| Kotlin | `ConcurrentHashMap` + `ArrayDeque` | gRPC `ServerInterceptor` | 3 |
| C# | `ConcurrentDictionary` + bounded channel | gRPC interceptor | 3 |
| Node.js | `Map` + circular buffer | gRPC middleware | 3 |
| Python | `dict` + `deque(maxlen=N)` | gRPC interceptor | 4 |
| C++ | `std::unordered_map` + `std::deque` | gRPC interceptor | 4 |

Cross-language interop test: Go client → Rust server (and reverse),
verifying session creation, state transitions, and metrics.

---

## v0.7 — Advanced Transports

Extend session tracking to non-gRPC transports and multi-host
topologies. Depends on grace-op v0.6+ (REST+SSE, mesh, public).

- **REST + SSE sessions** — SSE stream as session anchor,
  `X-Session-ID` POST correlation, stateless coalescing
- **Mesh sessions** — `mesh_host` from mTLS CN, cross-host
  `op sessions --all --mesh` aggregation
- **Public holon sessions** — `remote_slug` from auth layer
  (API key name, JWT `sub`), per-listener `session_visibility`
- **Holon-RPC sessions** — WebSocket connections tracked with
  the same model
- **Recipe composition insights** — cross-daemon session graph
  for composite recipes (which daemon talks to which, latency
  between members, bottleneck identification)

---

## Dependency Chain

```
v0.1 (specification)
  └─ v0.2 (Go reference)
       ├─ v0.3 (metrics)
       │    └─ v0.4 (op CLI + recipes)
       │         └─ v0.5 (Jack Middle + Prometheus)
       └─ v0.6 (cross-SDK ports)
            └─ v0.7 (advanced transports)
```

v0.3 and v0.6 can proceed in parallel after v0.2. v0.4 needs
v0.3 (to display metrics). v0.5 needs Jack Middle v0.1 and
sessions v0.3. v0.7 depends on both the SDK ports and on
grace-op v0.6/v0.9/v0.10 being available.
