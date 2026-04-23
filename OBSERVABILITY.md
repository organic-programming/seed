# Observability — Structured Logs, Metrics, and Lifecycle Events

Status: DRAFT
⚠️ This specification describes the cross-SDK observability layer —
structured logs, metrics, and lifecycle events — and its integration with
Prometheus, Grafana, and OpenTelemetry.

## Problem

Holons speak to each other over `.proto`-defined RPCs, across 14 SDKs,
and compose into organisms. When something misbehaves — a handler
panics, a session hangs, a recipe child quietly errors — the operator
has no uniform way to see what happened. Today the state is:

- No logging facade exists in any SDK. Some use stdlib loggers ad-hoc;
  most don't log at all.
- No metrics are collected anywhere. No `/metrics` endpoint, no
  counters, no histograms.
- [SESSIONS.md](SESSIONS.md) defines per-connection `SessionMetrics`
  with four-phase time decomposition (`wire_out`, `queue`, `work`,
  `wire_in`), but that spec is session-scoped — it says nothing about
  process-level signals, structured logs, or export to external systems.
- [`holons/grace-op/OP_PROXY.md`](holons/grace-op/OP_PROXY.md) sketches
  `op proxy` as a zero-config Prometheus exporter, but `op proxy` is
  not yet implemented and cannot gate this layer.

The holon abstraction promises parity between humans and machines
(COAX). Observability is where that promise is tested: a composer
debugging a complex scenario needs the same surface that a Prometheus
scraper or an OTLP collector needs, and that surface must exist on
every holon by construction, not by individual wiring.

## Solution: SDK-Managed Observability

Every holon exposes a single auto-registered `HolonObservability` gRPC
service with three RPCs — `Logs`, `Metrics`, `Events` — carrying
structured signals with enough identity to join across specs
(`slug`, `instance_uid`, `session_id`, `rpc_method`). On top of this
canonical RPC, two optional sidecars connect to external systems: a
Prometheus text-format `/metrics` HTTP endpoint living in the holon
itself, and an OTLP push exporter for collectors.

### Design Principles

1. **Opt-in debug mode** — observability is **off by default**. Zero
   overhead when disabled: no collectors, no interceptors, no
   `HolonObservability` service registration. Identical discipline to
   SESSIONS.md.
2. **Parent-controlled family activation** — the decision to turn a
   family of signals (logs, metrics, events) **on or off** belongs to
   whoever launches the holon (typically `op run`), via `OP_OBS`.
   Within an enabled family, the developer's Code API tunes the
   granularity (logger levels, metric inclusion, redaction). A holon
   cannot silently promote a family from off to on at runtime.
3. **SDK-managed, not protocol-level** — observability is bookkeeping
   inside the SDK; the gRPC/Holon-RPC stream is unchanged. Metrics and
   logs travel over their own RPCs, not as wire metadata.
4. **Join by identity, never by address** — logs, metrics, and events
   all carry `slug`, `instance_uid`, `session_id` (when applicable),
   and `rpc_method` (when applicable). Correlation across signals is a
   matter of equality on these keys, not timestamp matching or IP
   parsing.
5. **Pull is canonical for metrics; push is opt-in** — Prometheus
   scrape matches the shape of metrics (periodic snapshot). OTLP push
   is available for environments that require it, but the gRPC
   `Metrics()` RPC is the single source of truth.
6. **No dependency on `op proxy`** — v1 ships without `op proxy`.
   Holons expose their own `/metrics` endpoint when asked. When
   `op proxy` arrives, it becomes an aggregator on top of this layer,
   not a precondition for it.

### Identifier chain

A worked example of how signals correlate through the specs:

```
op run gabriel-greeting-go:9090 --observe
   │
   ├── INSTANCES.md : creates .op/run/gabriel-greeting-go/ea346efb/
   │                 exports OP_INSTANCE_UID=ea346efb to child
   │
   ├── SESSIONS.md  : on each accepted connection, creates SessionInfo
   │                 with session_id=d9e0f1a2 and instance_uid=ea346efb
   │
   └── OBSERVABILITY.md : every LogEntry, MetricSample, EventInfo
                         carries slug=gabriel-greeting-go,
                         instance_uid=ea346efb, and session_id
                         (when the signal originates inside an RPC).
```

Every join key is visible on both sides of every join.
`instance_uid` bridges INSTANCES.md ↔ SESSIONS.md ↔ OBSERVABILITY.md;
`session_id` bridges SESSIONS.md ↔ OBSERVABILITY.md.

---

## Activation

Three layers, each with a distinct scope. Unlike a simple override
chain, they compose by scope: Layer 3 (per-holon) gates the families
on or off; Layer 1 (per-site) tunes the granularity within what
Layer 3 allowed. A family that Layer 3 has not enabled cannot be
turned on by Layer 1.

| Layer | Scope | Decides | Set at |
|---|---|---|---|
| **Layer 3 — Config** | Per-holon, whole process | Which families (`logs` / `metrics` / `events` / `prom` / `otel`) are active | `OP_OBS` env by the parent launcher; `holon.proto` static defaults |
| **Layer 2 — RPC** | Per-listener, runtime | *(reserved for v2)* | — |
| **Layer 1 — Code API** | Per-logger, per-metric, per-gauge | Logger levels, metric inclusion, redaction, exposition addresses | Process init or per-site |

Layer 2 (runtime toggle via RPC) is intentionally deferred to v2.
Observability cost is low enough that a runtime toggle is not worth
the extra protocol surface; operators turn it on by restarting with
`--observe` or by setting `OP_OBS=all` before starting.

### Layer 1 — Code API

The developer wires fine-grained configuration for when the parent
has enabled a family. When `OP_OBS` is empty, every call below
degenerates into a no-op at zero cost — see §Cost when disabled.

Typical Go pattern:

```go
import "github.com/organic-programming/go-holons/pkg/observability"

func main() {
    // Configure runs at startup. On/off per family is read from
    // OP_OBS; this call supplies the knobs for when a family IS on.
    obs := observability.Configure(observability.Config{
        LogLevel:     observability.INFO,     // default level when logs=on
        PromAddr:     ":0",                    // ephemeral when OP_OBS=prom
        OTLPInterval: 15 * time.Second,        // cadence when OP_OBS=otel
    })
    defer obs.Close()

    // The serve runner reads obs from context and auto-registers
    // HolonObservability alongside HolonMeta when OP_OBS is non-empty.
    serve.Run(ctx, /* service impl */)
}
```

Per-site control within what OP_OBS allowed:

```go
logger := obs.Logger("recipe.runner")
logger.SetLevel(observability.DEBUG)          // per-logger override
logger.Info("recipe started", "recipe", name, "members", len(members))

obs.Counter("recipe_starts_total").Add(1)      // no-op if metrics off
obs.RedactFields("password", "api_key")        // apply globally
```

Selective tracing of a single RPC is handled by filtering at the
consumer side (e.g. `op logs --session=<id>`), not by per-call
toggles in the handler.

### Layer 3 — Config (static defaults)

`OP_OBS` is a **comma-separated list** of signal families and export
sidecars:

```
OP_OBS=                      # disabled (default)
OP_OBS=logs                  # structured logs only
OP_OBS=metrics               # metrics only
OP_OBS=events                # lifecycle events only
OP_OBS=logs,metrics          # logs + metrics
OP_OBS=logs,metrics,prom     # logs + metrics + /metrics endpoint
OP_OBS=all                   # shorthand for logs,metrics,events,prom
OP_OBS=all,otel              # add OTLP push — reserved, v2 only
```

Valid tokens in v1: `logs | metrics | events | prom | all`.
The token `otel` is **reserved** for v2 (see §Push — OTLP below)
and is a startup error in v1 until the OTLP exporter ships.
Any other unknown token is also a startup error (fail-fast), not
a silent warning.

The `OP_OBS` env var is set by the **parent launcher** — the holon
itself never decides to self-activate via config. Same discipline as
`OP_SESSIONS` in SESSIONS.md.

| Launcher | Mechanism |
|---|---|
| `op run <slug> --observe` | `op` sets `OP_OBS=logs,metrics,events` |
| `op run <slug> --observe=metrics,prom` | explicit subset |
| `op run <slug> --prom` | implies `--metrics`; sets `OP_OBS=metrics,prom` |
| `op run <slug> --otel=otel-collector:4317` | **v2 only** — implies `--metrics`; sets `OP_OBS=metrics,otel` plus `OP_OTEL_ENDPOINT`. Rejected in v1 |
| Manual launch | `OP_OBS=all ./my-holon serve --listen tcp://:9090` |

`OP_OBS` propagates naturally: if `op run` launches a composite
recipe with `--observe`, every child holon inherits `OP_OBS` unchanged
and, from the launcher, also receives:

- `OP_INSTANCE_UID` — distinct per child (see [INSTANCES.md §Registry](INSTANCES.md#registry))
- `OP_RUN_DIR` — the chosen registry root (same across the tree)
- `OP_ORGANISM_UID`, `OP_ORGANISM_SLUG` — invariant across every hop
  of the tree (see [INSTANCES.md §Organism Hierarchy](INSTANCES.md#organism-hierarchy)). These unlock the hierarchical
  registry layout and the root's `multilog.jsonl`.

### Composition with `OP_SESSIONS`

`OP_SESSIONS` and `OP_OBS` compose; neither replaces the other.

- `OP_SESSIONS=1` alone: session identity + state + `rpc_count`
  tracked; no metrics; no `HolonObservability` service.
- `OP_SESSIONS=metrics` alone: session metrics with four-phase
  decomposition collected; exposed via `HolonSession.Sessions` only.
  No `HolonObservability`.
- `OP_OBS=metrics` alone: process-level metrics (handler panics,
  in-flight, memory, build info) exposed via
  `HolonObservability.Metrics`. No session rollup.
- `OP_OBS=metrics` **+** `OP_SESSIONS=metrics`:
  `MetricsSnapshot.session_rollup` is populated by quantile-merging
  session histograms.

### `op proxy` — deferred to v2

`op proxy` (see [OP_PROXY.md](holons/grace-op/OP_PROXY.md), status
NOT IMPLEMENTED) will act as a cross-holon aggregator: scraping each
child `/metrics` endpoint, merging per-method latency across the
mesh, and publishing a unified Prometheus surface. Nothing in this v1
spec depends on `op proxy` being present.

---

## Log Model

Structured only. The SDK logger API is `(level, message,
fields map[string]string)`. No printf-style formatting reaches the
wire. The `message` field is a short, stable string ("recipe started",
"dial failed"); arbitrary structured detail goes in `fields`.

### Levels

| Level | Numeric | Use |
|---|:---:|---|
| `TRACE` | 1 | Very fine-grained; off except during bug hunts |
| `DEBUG` | 2 | Developer detail, visible when `--level=debug` |
| `INFO` | 3 | Normal operation milestones; default threshold |
| `WARN` | 4 | Unexpected but recovered |
| `ERROR` | 5 | Unexpected and not recovered; RPC failed |
| `FATAL` | 6 | Process cannot continue; entry flushed, then exit(1) |

Six levels. `FATAL` is terminal: the SDK flushes the ring buffer to
disk (see below), emits an `INSTANCE_CRASHED` event, then calls
`os.Exit(1)`.

### Well-known fields

The SDK auto-injects the following fields on every `LogEntry`. Handler
code must not set them; if it does, the SDK overwrites.

| Field | Source | Always present |
|---|---|:---:|
| `slug` | `holon.proto` manifest | ✓ |
| `instance_uid` | `OP_INSTANCE_UID` | when launched by `op run` |
| `session_id` | context-scoped correlator | when emitted inside an RPC handler |
| `rpc_method` | gRPC interceptor context | when inside a handler |
| `caller_file:line` | runtime caller info | ✓ |
| `goroutine_id` | Go runtime | Go SDK only |

Developers add domain fields freely (`"recipe": "greeting-app"`,
`"bytes_written": "42"`, etc.). All values are stringified by the
SDK — `fields` is `map<string,string>`, not `map<string,Any>`. Keeps
the wire format flat and JSON-safe.

### Ring buffer and disk

- **In-memory ring buffer**: bounded, default 1024 entries, per
  process. FIFO eviction. Used for `HolonObservability.Logs
  --since=<duration>` replay without disk reads.
- **Disk copy**: when `OP_OBS` contains `logs`, every entry is also
  written as a JSON object on its own line to
  `.op/run/<address>/<uid>/stdout.log` (see
  [INSTANCES.md §Log Contract](INSTANCES.md#log-contract)). Rotated at
  16 MB, ring of 4 files.
- **Disk is the durable store**, ring buffer is the replay cache.
  After a crash, post-mortem tooling reads the rotated files.

### Tailing UX

```
op logs <slug>                      # live tail from now
op logs <slug> --since 5m           # replay last 5 minutes from ring
op logs <slug> --follow=false       # drain the buffer and exit
op logs <slug> --level=warn         # filter to WARN+
op logs <slug> --session=<id>       # filter to one session
op logs <slug> --method=SayHello    # filter to one RPC method
op logs <slug> --json               # emit one JSON object per line
op logs --all                       # every running instance on OPPATH
```

The `--since` form is buffer-bounded: entries older than the ring
buffer's oldest resident are not recoverable via RPC; the operator
must read the rotated file directly.

---

## Metrics Model

Three metric types — `Counter`, `Gauge`, `Histogram`. No `Summary`
(Prometheus recommends histograms for aggregation). Histograms use
the HDR sketch (see [cross-SDK library table](#cross-sdk-implementation-notes)).

### Naming conventions

Follow Prometheus naming verbatim:

- Lowercase snake_case.
- `holon_` namespace prefix on every metric.
- `holon_session_` sub-namespace for session-scoped metrics.
- Counters end in `_total`.
- Histograms / gauges end in a unit suffix: `_seconds`, `_bytes`,
  `_bucket`.

### Baseline holon metrics

Emitted by every holon when `OP_OBS` contains `metrics`:

| Metric | Type | Labels | Description |
|---|---|---|---|
| `holon_build_info` | Gauge | `version, lang, commit` | Static 1; carries identity in labels |
| `holon_process_start_time_seconds` | Gauge | — | Unix seconds at spawn |
| `holon_memory_bytes` | Gauge | `class` | `rss`, `heap`, `stack` |
| `holon_goroutines` | Gauge | — | Go only; analogous field per-SDK |
| `holon_handler_in_flight` | Gauge | `method` | Currently executing RPCs |
| `holon_handler_panics_total` | Counter | `method` | Recovered panics per method |
| `holon_session_rpc_total` | Counter | `method, direction, phase, remote_slug` | Session-scoped count |
| `holon_session_rpc_duration_seconds` | Histogram | `method, direction, phase` | Four-phase decomposition |

The `phase` label takes values `wire_out`, `queue`, `work`, `wire_in`,
`total` — folding SESSIONS.md's four-phase decomposition into a
label dimension rather than separate metrics.

### `MetricsSnapshot.session_rollup`

When `OP_SESSIONS=metrics` and `OP_OBS=metrics` are both enabled,
`MetricsSnapshot.session_rollup` contains a quantile-merged view
across all live sessions. Closed sessions (in the ring buffer) are
not included in the rollup but remain accessible via
`HolonSession.Sessions --history`.

The rollup reuses the `SessionMetrics` message from SESSIONS.md by
proto import; no duplication.

---

## Event Model

Lifecycle events are discrete, coarser-grained than logs, and often
cross the boundary between the holon process and its parent. The
initial enum is narrow; new types are added by amendment.

| Type | Emitter | Meaning |
|---|---|---|
| `INSTANCE_SPAWNED` | parent (`op run`) | Process forked and PID written |
| `INSTANCE_READY` | child (SDK) | First listener bound; RPCs accepted |
| `INSTANCE_EXITED` | parent | Process exited cleanly (exit code 0) |
| `INSTANCE_CRASHED` | parent or child | Non-zero exit, signal, or FATAL log |
| `SESSION_STARTED` | SDK serve runner | New inbound session accepted |
| `SESSION_ENDED` | SDK serve runner | Session transitioned to CLOSED |
| `HANDLER_PANIC` | SDK interceptor | Handler panicked (before recovery) |
| `CONFIG_RELOADED` | handler | App-level config change |

Events ride a bounded in-memory buffer (default 256 entries). They
are emitted over `HolonObservability.Events(stream)` and — when
`OP_OBS` contains `events` — appended to
`.op/run/<address>/<uid>/events.jsonl` (see
[INSTANCES.md §Log Contract](INSTANCES.md#log-contract)).

`EventInfo.payload` is a `map<string,string>` for event-specific
context: `SESSION_STARTED` carries `{"session_id", "remote_slug",
"transport"}`; `HANDLER_PANIC` carries `{"method", "stack"}`;
`INSTANCE_EXITED` carries `{"exit_code"}`.

---

## Organism Relay

A composite app — a Flutter macOS binary hosting five Go/Rust/Python
subprocesses, or a mesh of remote holons bound together by a recipe —
is modelled as a **tree of holons** rooted at the organism. See
[INSTANCES.md §Organism Hierarchy](INSTANCES.md#organism-hierarchy)
for identity propagation.

The observability layer reuses **existing bidirectional connections**
(COMMUNICATION.md transports are all full-duplex once established).
No new RPC, no new dial direction, no `Ingest`:

- Each holon opens `child.Logs(follow=true)` and `child.Events(follow=true)`
  on every direct child it knows — typically the ones it spawned, or
  declared organism members from the mesh.
- It polls `child.Metrics()` on a configurable interval (default 15s).
- The child's stream handler is a long-lived emitter: it pushes local
  signals as they happen **and** re-emits signals it received from
  its own direct children's streams after appending one `ChainHop`.

### Chain semantics

`LogEntry.chain`, `EventInfo.chain`, `MetricSample.chain` are the
**wire chain** — the ordered hops from the originator up to but
**not including** the emitter of the stream the reader is consuming.
Two rules, applied by the SDK on every re-emission:

- **Local emission** — the signal was produced by this holon →
  `chain = []` on its own `Logs/Events/Metrics` stream.
- **Relay from a direct child `C`** — a signal arrived via
  `C.Logs`/`C.Events`/`C.Metrics`. The SDK **appends**
  `(C.slug, C.instance_uid)` to the chain it received, then re-emits
  the signal on its own stream.

A reader of a stream from emitter `E` thus sees, for every entry:

- `slug`, `instance_uid` — the **originator** (where the signal was
  first produced, always preserved through every relay).
- `chain` — the ordered hops between originator and `E`, exclusive
  of `E`. Empty if the originator is `E` itself.

Full path reconstruction at the reader:

```
originator  = <slug of the entry>
path (wire) = originator → chain[0] → chain[1] → … → chain[-1] → E (stream source) → reader
```

### Multilog chain enrichment

When an organism root (or any consumer writing the stream to a
persistent log) records an entry it just read from stream source
`E`, it **appends** `(E.slug, E.instance_uid)` to the entry's chain
before serialising. This produces the **enriched chain**:

```
chain (enriched) = chain (wire) ++ [E]
```

so the serialised record stands alone — the full relay path is
preserved without needing to remember which stream produced it.
The [Multilog Contract](#multilog-contract) below shows enriched
entries.

### Example: three-level organism

An illustrative Flutter organism `gabriel-greeting-app` spawns
`gabriel-greeting-go`, which in turn dials `gabriel-greeting-rust`
for a CPU-bound rendering subtask. In real composite apps, two
levels (root + direct members) is the common case; three levels
exercises the relay machinery most cleanly.

```
root: gabriel-greeting-app (Dart, organism UID 4a7b8c9d…)
   │  opens gabriel-greeting-go.Logs(follow=true)
   │
   └── gabriel-greeting-go (Go, instance UID ea346efb…)
         │  opens gabriel-greeting-rust.Logs(follow=true)
         │
         └── gabriel-greeting-rust (Rust, instance UID 1c2d3e4f…)
```

`gabriel-greeting-rust` emits a log `"rendered banner"`:

1. On `gabriel-greeting-rust.Logs` (local emission) →
   `{slug=gabriel-greeting-rust, uid=1c2d…, chain=[], message="rendered banner"}`.
2. `gabriel-greeting-go` receives it from
   `gabriel-greeting-rust.Logs` and appends that stream source to
   the chain before re-emitting on its own `Logs` stream →
   `{slug=gabriel-greeting-rust, uid=1c2d…, chain=[{slug=gabriel-greeting-rust, uid=1c2d…}]}`.
3. Root reads the entry on `gabriel-greeting-go.Logs`. Wire view:
   `chain=[{gabriel-greeting-rust}]`; the stream source
   (`gabriel-greeting-go`) is implicit because the root is reading
   that stream.
4. Before writing to `multilog.jsonl`, the root applies
   [multilog chain enrichment](#multilog-chain-enrichment) — it
   appends the stream source (`gabriel-greeting-go`) →
   `chain=[{gabriel-greeting-rust}, {gabriel-greeting-go}]`. The
   serialised multilog line stands alone and carries the full relay
   path.

### Transports without outbound dial

Holons that can only **dial** and cannot **serve** (see the
[SDK Transport Matrix](sdk/README.md#transport-matrix)) — notably
`js-web-holons` in the browser — are reached the other way around:
they dial the organism root (or an intermediate Hub) as clients.
COMMUNICATION.md §4.1 specifies that Holon-RPC over WebSocket is
full-duplex at the application layer — either end can invoke methods.
The organism calls `Logs`/`Events`/`Metrics` on the browser-hosted
holon via the WebSocket it accepted. Relay works identically.

### Multilog Contract

Only the **root** writes the multilog. Intermediate holons only
relay; their own local disk still holds their own signals for
forensic use (see [INSTANCES.md §Log Contract](INSTANCES.md#log-contract)).

Location:

```
<run_root>/<organism_slug>/<organism_uid>/
  multilog.jsonl          # every signal observed by the root, local or relayed
  multilog.jsonl.1        # rotated (16 MB × ring of 4, same as stdout.log)
  stdout.log              # the root's OWN logs only (not enriched)
  events.jsonl            # the root's OWN events only (not enriched)
  members/
    gabriel-greeting-go/ea346efb…/
      stdout.log          # written by the child itself (local copy)
      events.jsonl
      members/
        gabriel-greeting-rust/1c2d…/
          stdout.log      # written by the direct-child holon itself
```

Format: **JSON lines**, one object per line. Each object carries a
`kind` discriminant (`"log"`, `"event"`, `"metric_sample"`), the
union of fields from the corresponding proto message, and the **enriched
`chain`** described in [§Multilog chain enrichment](#multilog-chain-enrichment):
when the root records a signal it read from stream source `E`, it
appends `(E.slug, E.instance_uid)` to the wire chain. For a signal
the root emitted itself, the enriched chain is also empty.

Example lines (three-level organism, using the worked example above):

```jsonl
{"kind":"log","ts":"2026-04-23T18:42:03.112Z","level":"INFO","slug":"gabriel-greeting-rust","instance_uid":"1c2d3e4f","chain":[{"slug":"gabriel-greeting-rust","instance_uid":"1c2d3e4f"},{"slug":"gabriel-greeting-go","instance_uid":"ea346efb"}],"session_id":"d9e0f1a2","rpc_method":"SayHello","message":"rendered banner","fields":{"name":"Bob","lang":"en"},"caller":"greeting.rs:42"}
{"kind":"event","ts":"2026-04-23T18:42:03.200Z","type":"SESSION_STARTED","slug":"gabriel-greeting-go","instance_uid":"ea346efb","chain":[{"slug":"gabriel-greeting-go","instance_uid":"ea346efb"}],"session_id":"f3a4b5c6","payload":{"remote_slug":"gabriel-greeting-rust","transport":"stdio"}}
{"kind":"metric_sample","ts":"2026-04-23T18:42:15.000Z","name":"holon_handler_in_flight","labels":{"slug":"gabriel-greeting-go","method":"SayHello"},"value":2.0,"chain":[{"slug":"gabriel-greeting-go","instance_uid":"ea346efb"}]}
```

In the first line, `gabriel-greeting-rust` is the originator; its
wire chain at the root was `[{gabriel-greeting-rust}]`; enrichment
appended `{gabriel-greeting-go}` (the stream the root was reading)
to produce the two-element chain shown.

Merge order: the root's emitter writes strictly in arrival order,
**not** in timestamp order. Clock skew between holons is the reader's
problem; `jq` and downstream tooling can re-sort if needed.

Resilience:
- Root crash → the `multilog.jsonl` up to the last flush survives;
  each member's own local `stdout.log` / `events.jsonl` also survives
  (dual filesystem: the multilog is the live narrative, the locals
  are the per-member forensic record).
- Member crash → gap in the multilog for that member's signals after
  the last-received one; the grandparent's own stream continues.
- Network transient → child ring buffers continue; on reconnect the
  stream replays from the last acknowledged timestamp.

See [OBSERVABILITY_HANDBOOK.md §Dispositions des flux remontants](OBSERVABILITY_HANDBOOK.md#dispositions-des-flux-remontants)
for concrete usage patterns — tail, filter, jq recipes, Loki forward,
Grafana Live streaming.

---

## Transport Constraints

Observability data rides the same transports defined in
[COMMUNICATION.md §2](COMMUNICATION.md#2-transports--lifecycles). No
new schemes. Per-transport properties for observability endpoints:

| Scheme | Carries `HolonObservability` gRPC | Carries Prometheus `/metrics` | Notes |
|---|:---:|:---:|---|
| `tcp://` | ✓ | ✓ (on a dedicated HTTP/1.1 port) | gRPC and `/metrics` on distinct listeners |
| `unix://` | ✓ | ✗ | Prometheus scrapers speak TCP; UNIX sockets force socket-path mode on the scraper side |
| `stdio://` | ✓ (via parent mux) | ✗ | `/metrics` is a no-op over stdio; `OP_OBS=prom` is silently ignored with a WARN log |
| `ws://` | ✓ (Holon-RPC) | ✗ | Prometheus scrape uses pure HTTP; not WebSocket |
| `wss://` | ✓ (Holon-RPC) | ✗ | Same as `ws://` |
| `http://` | ✓ (Holon-RPC) | ✓ (same HTTP listener, different route) | gRPC and `/metrics` share the HTTP server; `metrics_addr` reports the same address |
| `https://` | ✓ (Holon-RPC) | ✓ (same TLS listener, different route) | Same as `http://`, TLS |

Binding rule: when the listener's scheme is HTTP-capable (`http://` /
`https://`), the SDK serves gRPC and `/metrics` on the same listener
and `metrics_addr` echoes that listener's address. For any other
scheme, enabling `prom` binds a **dedicated ephemeral HTTP/1.1 port**
distinct from the holon's primary listener, and its address is
recorded in `.op/run/<address>/<uid>/meta.json` as `metrics_addr`.
Multiplexing gRPC and HTTP on a single non-HTTP listener via
HTTP/2 routing is an optimisation deferred to v2.

### Valence and cross-transport observability

For `stdio://` holons (monovalent per [SESSIONS.md §Transport
Constraints](SESSIONS.md#per-transport-session-behavior)), the single
session also carries observability traffic: the parent mux demultiplexes
`HolonObservability.Logs` streams from regular handler RPCs. No
Prometheus endpoint is bound regardless of `OP_OBS`; if the operator
includes `prom` in the env, the SDK emits one WARN log
(`"prom ignored on stdio:// transport"`) and continues. `op` can
still read metrics over gRPC and re-expose them on behalf of the
child via its own aggregator when one is configured.

---

## Collection & Export

### Pull — Prometheus

When `OP_OBS` contains `prom`, the SDK binds an HTTP/1.1 listener on
an ephemeral port (unless `OP_PROM_ADDR` overrides) and serves
`GET /metrics` in Prometheus text exposition format.

```
# HELP holon_build_info Holon build information
# TYPE holon_build_info gauge
holon_build_info{slug="gabriel-greeting-go",instance_uid="ea346efb",version="0.3.1",lang="go",commit="a1b2c3d"} 1
# HELP holon_session_rpc_duration_seconds Session RPC duration by phase
# TYPE holon_session_rpc_duration_seconds histogram
holon_session_rpc_duration_seconds_bucket{method="SayHello",direction="inbound",phase="work",le="0.001"} 42
holon_session_rpc_duration_seconds_bucket{method="SayHello",direction="inbound",phase="work",le="0.010"} 198
…
holon_session_rpc_duration_seconds_sum{method="SayHello",direction="inbound",phase="work"} 0.423
holon_session_rpc_duration_seconds_count{method="SayHello",direction="inbound",phase="work"} 198
```

Labels `slug` and `instance_uid` are injected on every metric by the
exposition formatter; individual metrics do not need to declare them.

### Push — OpenTelemetry Protocol (OTLP)

> **v2.** The OTLP exporter is specified here for implementers
> planning ahead; the v1 delivery ships only the in-holon Prometheus
> `/metrics` endpoint described above. In v1, including `otel` in
> `OP_OBS` or passing `--otel=...` to `op run` is a **startup error**
> (unknown token — see fail-fast rule in §Layer 3) on every SDK until
> this section becomes implemented. The flag and env tokens are
> reserved so that existing usages do not need to migrate.

When `OP_OBS` contains `otel` (v2), the SDK pushes metrics, logs, and
events to `OP_OTEL_ENDPOINT` on an interval (default 15s, override
with `OP_OTEL_INTERVAL=<duration>`). The mapping is:

- **Metrics** → OTLP Metrics (`Counter` → `Sum`, `Gauge` → `Gauge`,
  `Histogram` → `Histogram`).
- **Logs** → OTLP LogRecord with `severity_number` derived from the
  level enum, `body` from `message`, and `attributes` from `fields`.
- **Events** → OTLP LogRecord with attribute `holon.event_type =
  <EventType>`. No dedicated OTLP event signal exists; LogRecord is
  the accepted vehicle.

Distributed tracing (OTLP spans with W3C trace-context propagation)
is deferred to v3. The v2 export contract is metrics + logs + events
only.

### Grafana

No spec contribution is required. Grafana consumes the Prometheus
endpoint and the OTLP pipeline unchanged. The labels this spec emits
(`slug`, `instance_uid`, `method`, `phase`, `direction`, `remote_slug`)
are designed to partition cleanly in Grafana dashboards.

---

## Proto Definition

New file: `holons/grace-op/_protos/holons/v1/observability.proto`.
Auto-registered by every SDK's serve runner, like `HolonMeta` and
`HolonSession`. Holons don't import or implement it.

```protobuf
syntax = "proto3";
package holonobservability.v1;

import "google/protobuf/timestamp.proto";
import "holons/v1/session.proto";  // for SessionMetrics reuse

// HolonObservability is auto-registered by the SDK's serve runner
// when OP_OBS is set. Provides structured logs, metrics snapshots,
// and lifecycle events.
service HolonObservability {
  // Logs streams log entries. If follow=true, the stream stays open
  // and emits new entries as they arrive. If follow=false, drains
  // the current ring buffer and ends.
  rpc Logs(LogsRequest) returns (stream LogEntry);

  // Metrics returns a point-in-time snapshot of all current metrics.
  // Unary, not streaming — scraping cadence is the caller's concern.
  rpc Metrics(MetricsRequest) returns (MetricsSnapshot);

  // Events streams lifecycle events. If follow=true, stays open.
  rpc Events(EventsRequest) returns (stream EventInfo);
}

message LogsRequest {
  // Filter: minimum level returned. UNSPECIFIED = INFO.
  LogLevel min_level = 1;

  // Filter: return only entries with these session_ids (empty = all).
  repeated string session_ids = 2;

  // Filter: return only entries for these rpc_methods (empty = all).
  repeated string rpc_methods = 3;

  // Replay window. If set, drain buffer entries whose ts >= (now - since).
  // If unset and follow=false, drains the full buffer.
  google.protobuf.Duration since = 4;

  // If true, stream stays open and emits new entries as they arrive.
  // If false, stream ends after replay is complete.
  bool follow = 5;
}

message LogEntry {
  google.protobuf.Timestamp ts = 1;
  LogLevel level = 2;
  string slug = 3;
  string instance_uid = 4;
  string session_id = 5;           // empty outside handler scope
  string rpc_method = 6;           // empty outside handler scope
  string message = 7;
  map<string, string> fields = 8;
  string caller = 9;               // "file:line"

  // Relay path: ordered hops from the originator up through each
  // relay before arriving on the stream being read. Empty when the
  // entry was emitted by the holon whose stream the reader is
  // consuming. See §Organism Relay.
  repeated ChainHop chain = 10;
}

message ChainHop {
  string slug = 1;
  string instance_uid = 2;
}

enum LogLevel {
  LOG_LEVEL_UNSPECIFIED = 0;
  TRACE = 1;
  DEBUG = 2;
  INFO = 3;
  WARN = 4;
  ERROR = 5;
  FATAL = 6;
}

message MetricsRequest {
  // Filter: return only metric names matching these prefixes.
  repeated string name_prefixes = 1;

  // If true, include the session rollup (requires OP_SESSIONS=metrics).
  bool include_session_rollup = 2;
}

message MetricsSnapshot {
  google.protobuf.Timestamp captured_at = 1;
  string slug = 2;
  string instance_uid = 3;
  repeated MetricSample samples = 4;

  // Populated only when OP_SESSIONS=metrics and the request asks for it.
  // Merged across all live sessions; closed sessions excluded.
  holons.v1.SessionMetrics session_rollup = 5;
}

message MetricSample {
  string name = 1;
  map<string, string> labels = 2;
  oneof value {
    int64 counter = 3;
    double gauge = 4;
    HistogramSample histogram = 5;
  }
  // Optional. Purely informational; Prometheus HELP string.
  string help = 6;

  // Relay path (see LogEntry.chain). Typically empty for metrics:
  // the caller asks `child.Metrics()` directly on each direct child,
  // so the stream identifies the source. Populated only when a holon
  // folds a direct child's cached samples into its own snapshot.
  repeated ChainHop chain = 7;
}

message HistogramSample {
  // Cumulative buckets — bucket.count includes all samples
  // where value <= bucket.upper_bound. Prometheus semantics.
  repeated Bucket buckets = 1;
  // Implicit +Inf bucket count.
  int64 count = 2;
  // Sum of all observed values.
  double sum = 3;
}

message Bucket {
  double upper_bound = 1;
  int64 count = 2;
}

message EventsRequest {
  // Filter: return only these event types (empty = all).
  repeated EventType types = 1;

  // Replay window (see LogsRequest.since).
  google.protobuf.Duration since = 2;

  // Stream stays open if true.
  bool follow = 3;
}

message EventInfo {
  google.protobuf.Timestamp ts = 1;
  EventType type = 2;
  string slug = 3;
  string instance_uid = 4;
  string session_id = 5;               // when applicable
  map<string, string> payload = 6;

  // Relay path (see LogEntry.chain, same semantics).
  repeated ChainHop chain = 7;
}

enum EventType {
  EVENT_TYPE_UNSPECIFIED = 0;
  INSTANCE_SPAWNED = 1;
  INSTANCE_READY = 2;
  INSTANCE_EXITED = 3;
  INSTANCE_CRASHED = 4;
  SESSION_STARTED = 5;
  SESSION_ENDED = 6;
  HANDLER_PANIC = 7;
  CONFIG_RELOADED = 8;
}
```

Field-tag discipline:

- `MetricSample.value` is `oneof` — a sample is exactly one of
  counter, gauge, or histogram. Encoding on the wire is the
  protobuf-standard oneof tag.
- `Bucket.upper_bound = +Inf` is expressed as the Go `math.Inf(1)` /
  IEEE 754 positive infinity. Consumers normalise to the Prometheus
  `le="+Inf"` label when exporting.
- `EventType` and `LogLevel` start at 1 after a canonical
  `UNSPECIFIED=0`, per protobuf style.

---

## SDK Changes (Go Reference)

### New packages

```
sdk/go-holons/
  pkg/observability/
    observability.go        // facade: Enable(), Close(), Logger(), MetricRegistry(), EventBus()
    logs/
      logger.go             // public Logger type; Info/Warn/Error/… methods
      ringbuffer.go         // bounded ring
      interceptor.go        // UnaryServerInterceptor injecting session_id + method
      writer.go             // JSON-lines writer to .op/run/<uid>/stdout.log
    metrics/
      registry.go           // Counter/Gauge/Histogram types + registry
      histogram.go          // HDR sketch wrapper (hdrhistogram-go)
      collector.go          // baseline metrics collector (process, runtime)
    events/
      bus.go                // bounded event buffer, pub/sub
      writer.go             // JSON-lines writer to .op/run/<uid>/events.jsonl
    prom/
      exposition.go         // text-format encoder
      http_handler.go       // net/http.Handler for /metrics
    otel/
      exporter.go           // OTLP exporter (metrics + logs + events)

  internal/observability/
    service/
      service.go            // HolonObservability gRPC impl
      filter.go             // visibility enforcement
```

### Auto-registration

`sdk/go-holons/pkg/serve/serve.go` inspects `OP_OBS` at startup. If
non-empty, it constructs a singleton `observability.Config`, starts
the requested sidecars (prom/otel), and registers
`HolonObservability` on the gRPC server alongside `HolonMeta` and
(when `OP_SESSIONS` is set) `HolonSession`.

### Interceptors

- `UnaryServerInterceptor` injects `session_id` and `rpc_method` into
  the context before calling the handler. The logger picks them up
  via context.
- `UnaryClientInterceptor` does the same for outbound calls, with
  `direction=OUTBOUND`.

### Exclusions from `HolonMeta.Describe`

`HolonObservability`, like `HolonMeta` and `HolonSession`, is
excluded from `Describe` output. Introspection tooling finds it
through the fixed service name, not through the manifest.

### Cost when disabled

When `OP_OBS` is empty:

- No `observability.Config` singleton is allocated.
- No interceptor is chained.
- No `HolonObservability` service is registered.
- `pkg/observability.Logger(...)` returns a no-op logger that
  discards calls in a single `if enabled == false` check.
- Metric registry returns zero-cost `NoOpCounter` / `NoOpGauge` /
  `NoOpHistogram` instances.

The goal is that a holon running without observability pays nothing
beyond the cost of one additional import.

---

## `op` Integration

Four new verbs, parallel to `op sessions`:

```
op logs <slug>              # tail live
op metrics <slug>           # snapshot
op events <slug>            # stream events
op ps                       # list running instances (see INSTANCES.md)
```

### `op logs`

```
$ op logs gabriel-greeting-go --since 1m --level=info
2026-04-23T18:42:03.112Z INFO  slug=gabriel-greeting-go instance_uid=ea346efb session_id=d9e0f1a2 method=SayHello msg="handled greeting" name=Bob lang=en
2026-04-23T18:42:04.801Z WARN  slug=gabriel-greeting-go instance_uid=ea346efb msg="retry budget low" remaining=3
…
```

Flags: `--since <duration>`, `--level <level>`, `--session <id>`,
`--method <method>`, `--follow=true|false` (default true),
`--json`, `--all`.

When `<slug>` resolves to an organism root (its registry entry has
`organism_uid == instance_uid`), `op logs` reads from the root's
`multilog.jsonl` by default — the single interleaved view of every
signal in the tree. Add `--local` to bypass the multilog and read
only the root's own ring buffer. Add `--chain-origin=<slug>` to
filter to signals originated in a specific descendant.

### `op metrics`

```
$ op metrics gabriel-greeting-go
METRIC                                                           TYPE       VALUE
holon_build_info{version="0.3.1",lang="go"}                      gauge      1
holon_process_start_time_seconds                                 gauge      1714075323.1
holon_memory_bytes{class="rss"}                                  gauge      14680064
holon_handler_in_flight{method="SayHello"}                       gauge      0
holon_handler_panics_total{method="SayHello"}                    counter    0
holon_session_rpc_total{method="SayHello",phase="work"}          counter    198
holon_session_rpc_duration_seconds{method="SayHello",phase="work"} histogram p50=1.1ms p99=4.8ms
```

Flags: `--prom` (emit Prometheus exposition format instead of the
human table), `--prefix <name>`, `--include-session-rollup`, `--all`.

### `op events`

```
$ op events gabriel-greeting-go --follow
2026-04-23T18:42:01.000Z INSTANCE_SPAWNED     instance_uid=ea346efb pid=12341
2026-04-23T18:42:01.102Z INSTANCE_READY       instance_uid=ea346efb listener=tcp://127.0.0.1:9090
2026-04-23T18:42:03.111Z SESSION_STARTED      instance_uid=ea346efb session_id=d9e0f1a2 remote_slug=grace-op
```

Flags: `--type <type>` (repeatable), `--since <duration>`,
`--follow=true|false`, `--json`, `--all`.

### `op ps`

Full surface (flags, tree rendering, remote-member walk) is owned by
[INSTANCES.md §CLI — `op ps` and `op instances`](INSTANCES.md#cli--op-ps-and-op-instances).
For observability purposes, the relevant additions are the
`METRICS_ADDR` column (populated from `meta.json` when the holon bound
a Prometheus endpoint) and the `--tree` / `--flat` rendering used by
`op logs <organism>` to scope the multilog and per-member files.

---

## Cross-SDK Implementation Notes

The proto definition is canonical. Each SDK chooses its own idiomatic
vehicle for log facade, metric registry, HDR histogram, and OTLP
exporter. Language-native libraries should be preferred — the Organic
Programming surface is observability-native in every SDK, not a
wrapper around a single library's types.

| SDK | Logs facade | Metrics registry | HDR histogram | OTLP exporter |
|---|---|---|---|---|
| Go | `log/slog` (stdlib) | thin wrapper on `expvar` | `HdrHistogram/hdrhistogram-go` | `go.opentelemetry.io/otel/exporters/otlp/otlpmetricgrpc` |
| Swift | `swift-log` | `swift-metrics` | `HdrHistogram.swift` | `opentelemetry-swift` |
| Dart | `package:logging` | custom | `hdr_histogram` (pub) | `opentelemetry` (pub) |
| Rust | `tracing` | `metrics` crate | `hdrhistogram` crate | `opentelemetry-otlp` |
| Kotlin | `kotlin-logging` | `micrometer-core` | `HdrHistogram` (JVM) | `opentelemetry-kotlin` |
| C# | `Microsoft.Extensions.Logging` | `System.Diagnostics.Metrics` | `HdrHistogram.NET` | `OpenTelemetry.Exporter.OpenTelemetryProtocol` |
| Python | `structlog` | `prometheus_client` | `hdrh` | `opentelemetry-exporter-otlp` |
| Node.js | `pino` | `prom-client` | `hdr-histogram-js` | `@opentelemetry/exporter-trace-otlp-grpc` |
| Java | SLF4J + Logback | Micrometer | `HdrHistogram` | `opentelemetry-exporter-otlp` |
| Ruby | `semantic_logger` | `prometheus-client` | `hdrhistogram` gem | `opentelemetry-exporter-otlp` |
| C | custom (syslog fallback) | custom | `HdrHistogram_c` | libcurl + OTLP/HTTP manually |
| C++ | `spdlog` | `prometheus-cpp` | `HdrHistogram_c` | `opentelemetry-cpp` |
| JS-web | `pino` (browser build) | custom | `hdr-histogram-js` | `@opentelemetry/exporter-trace-otlp-http` |

Cross-SDK invariants that the proto cannot encode:

- Every SDK emits log entries **in-memory first, disk second**. A
  disk-write failure must not propagate to the handler; the failure
  itself is recorded as a `FileSystem` field on a WARN entry and
  emitted downstream.
- Every SDK guarantees `FATAL` logs are flushed to disk before exit.
- Every SDK enforces the 1 MB gRPC message limit from
  [COMMUNICATION.md §6](COMMUNICATION.md#6-operational-constraints)
  on `LogsRequest` / `MetricsSnapshot`. Chunking a snapshot larger
  than 1 MB is the implementer's concern.

---

## Security Considerations

`HolonObservability` is auto-registered on every holon, and like
`HolonSession` is reachable by any peer that can connect. Logs,
metrics, and events may leak operational secrets (structured
fields, remote slugs, internal IPs). The security model extends
SESSIONS.md's visibility discipline.

### Unified visibility enum

The `ObservabilityVisibility` enum (declared in `holons/v1/manifest.proto`
— see [SESSIONS.md §Manifest additions](SESSIONS.md#manifest-additions)) covers both
session tracing and observability. One dial, both services.

| Level | `HolonObservability` behavior | Prometheus `/metrics` |
|---|---|---|
| `OFF` | All RPCs return `PERMISSION_DENIED (7)` | 403 Forbidden |
| `SUMMARY` | `Metrics` returns metric *names* and counts only (no values); `Logs` returns `ts+level+slug` only (no message, no fields); `Events` returns `ts+type` only (no payload). | Names + counts only |
| `FULL` | All fields returned. | Full exposition |

### Per-listener defaults

Same shape as SESSIONS.md. The SDK infers the default from each
listener's scheme and security mode:

| Listener | Default |
|---|---|
| `stdio://` | `FULL` |
| `tcp://` with `security: mesh` | `FULL` |
| `unix://` | `FULL` |
| `tcp://` (plain) | `SUMMARY` |
| `ws://`, `wss://` (plain) | `SUMMARY` |
| `wss://` with `security: public` | `OFF` |
| `https://` with `security: public` | `OFF` |

Per-listener overrides follow the `ListenerVisibilityOverride` form
in `HolonManifest` (declared in SESSIONS.md closures).

### `op` is not special

`op logs` / `op metrics` / `op events` call the same RPCs as any
other holon. They get the same visibility level. For full diagnostics
on a remote holon, either:

1. Set `session_visibility: FULL` globally in `holon.proto`
   (development only).
2. Use `stdio://` transport (inherently local, defaults to `FULL`).
3. Add a mesh listener (mTLS peers get `FULL`).

### Field redaction

Even at `FULL` visibility, log `fields` may carry sensitive data.
The SDK offers an opt-in field-level redaction hook:

```go
obs.RedactFields("password", "api_key", "authorization")
```

Any field whose key matches the redaction list is replaced by
`"<redacted>"` before emission. Redaction is a per-SDK concern;
downstream consumers see redacted fields as-is.

### Message integrity

Logs, metrics, and events are **diagnostic signals**, not audit
records. The SDK makes no integrity guarantees (no signatures, no
hash chain). Operators needing audit trails should pipe OTLP into a
backend that provides them.

---

## Phasing

- **v1** — logs + metrics + events, Prometheus `/metrics` endpoint in
  the holon, no `op proxy`, no OTLP push. Ships with SESSIONS.md and
  INSTANCES.md closures.
- **v2** — `op proxy` aggregates child `/metrics`; OTLP push; recipe
  rollup with `recipe_uid` label; runtime-toggle RPC (Layer 2) if a
  concrete operator ask materialises.
- **v3** — distributed tracing with W3C trace-context propagation
  across gRPC metadata; OTLP spans; span/log correlation via
  `trace_id` + `span_id` attributes on `LogEntry`.

---

## Relationship to SESSIONS.md and INSTANCES.md

| Concept | Owner | Used here |
|---|---|---|
| `instance_uid` | [INSTANCES.md](INSTANCES.md) | Injected on every `LogEntry`, `MetricSample`, `EventInfo` via `OP_INSTANCE_UID` |
| `.op/run/<uid>/stdout.log` / `events.jsonl` | INSTANCES.md | Disk target for ring buffer flush |
| `session_id` | [SESSIONS.md](SESSIONS.md) | Join key on logs and handler metrics |
| `SessionMetrics` (four-phase) | SESSIONS.md | Imported into `MetricsSnapshot.session_rollup` |
| `ObservabilityVisibility` enum | SESSIONS.md (declares) / here (reuses) | Single security dial |
| `OP_SESSIONS` | SESSIONS.md | Orthogonal to `OP_OBS`, composes |
| `HolonInstance.List` RPC | INSTANCES.md | Called by `op ps` / `op metrics --all` |

If SESSIONS.md's `instance_uid` addition is deferred, observability
cannot produce per-instance Prometheus labels on session metrics —
this is the single most load-bearing upstream edit.

---

## Open Questions

1. **Runtime-toggle RPC (Layer 2)** — deferred to v2. If operators
   need to flip observability on a running holon without restart,
   the toggle surface needs design. Leaning toward a single
   `HolonObservability.Configure(ConfigureRequest)` RPC that accepts
   the same token list as `OP_OBS` and applies it live.
2. **Disk rotation policy** — the 16 MB / 4-file ring is a guess.
   Should it be tunable per-holon via `holon.proto`? Should the
   rotation suffix be `.N` or a timestamp?
3. **OTLP transport choice** — gRPC vs HTTP/protobuf. gRPC is
   assumed in the table; a pure-HTTP browser-hosted holon (via
   `sdk/js-web-holons`) cannot open outbound gRPC. JS-web SDK needs
   OTLP/HTTP.
4. **Recipe-level rollup semantics** — in v2, when `op proxy`
   aggregates children, what identifier anchors the rollup? A new
   `recipe_uid` env var injected by `op run`? A hash of the recipe
   manifest? Not decided.
5. **Sampling** — high-traffic holons may need head-based sampling
   for logs and tail-based sampling for traces (v3). Not specified
   in v1; all log entries pass through.
6. **Metric cardinality limits** — unbounded labels (e.g. a user-
   supplied `remote_slug` from an anonymous WebSocket) can explode
   Prometheus label cardinality. A per-metric cardinality cap with
   a fallback `_overflow` label is likely needed; design deferred.
