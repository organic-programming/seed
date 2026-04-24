# Observability Handbook

Status: DRAFT
A companion to [OBSERVABILITY.md](OBSERVABILITY.md). The spec says *what
the observability layer is*; this handbook says *what to do with it
when debugging at 3 a.m.* Recipes are scenario-first: each starts from
a symptom, ends with an action.

---

## 1. Starting out

### 1.1 Enable observability on one holon

```bash
# One-shot: logs + metrics + events, plus /metrics HTTP endpoint
op run gabriel-greeting-go:9090 --observe --prom

# Equivalent via env
OP_OBS=logs,metrics,events,prom ./gabriel-greeting-go serve --listen tcp://:9090
```

Check it's on: `op metrics gabriel-greeting-go` should print a table;
`op logs gabriel-greeting-go` should tail.

### 1.2 Where do my logs go?

| Destination | When |
|---|---|
| `HolonObservability.Logs` gRPC stream | always, while holon is running |
| In-memory ring buffer (1024 entries) | always, while holon is running |
| `.op/run/<slug>/<uid>/stdout.log` | when `OP_OBS` contains `logs` |
| Prometheus `/metrics` endpoint | when `OP_OBS` contains `prom` |
| OTLP collector | when `OP_OBS` contains `otel` + `OP_OTEL_ENDPOINT` |

`op logs <slug>` reads the gRPC stream. `cat .op/run/.../stdout.log`
reads the disk. Both contain the same JSON lines.

### 1.3 Scraping with Prometheus in 3 minutes

1. Start the holon with `--prom`. Note the `metrics_addr` printed:
   `http://127.0.0.1:54321`.
2. Point Prometheus at it:

   ```yaml
   # prometheus.yml
   scrape_configs:
     - job_name: 'holons'
       static_configs:
         - targets: ['127.0.0.1:54321']
   ```
3. Reload Prometheus. Verify with `curl http://127.0.0.1:54321/metrics`.

For multiple holons, list their `metrics_addr` values (get them from
`op ps --json`).

### 1.4 Wiring up Grafana dashboards

Every metric carries `slug`, `instance_uid`, and (where applicable)
`method`, `phase`, `direction`, `remote_slug`. Build dashboards
partitioned by these labels; the starter dashboard at
`docs/grafana/holon-overview.json` covers RPC rate, error rate, and
four-phase latency (wire_out / queue / work / wire_in).

---

## 2. Organism composites

### 2.1 Launching an organism with centralized logs

There are two distinct launch modes, with the same end result but
very different UIs.

#### Case A — CLI-driven via `op run --observe`

A Flutter app spawning a Go backend, a Rust file helper, and a Python
worker:

```bash
op run gabriel-greeting-app --observe
```

`op run` sets `OP_ORGANISM_UID` / `OP_ORGANISM_SLUG` / `OP_OBS` in
every member's env. Members don't need to know about the organism —
their streams are consumed by the root, which writes the aggregated
multilog to `.op/run/gabriel-greeting-app/<uid>/multilog.jsonl`.

With `--observe`, `op run` **stays resident** after the spawn and
tails `HolonObservability.Logs(follow=true)` on the launched holon.
Every entry — the app's own logs plus every relayed entry from each
member, carrying its `chain` — lands on `op`'s stdout in real time:

```
2026-04-23T18:42:03.112Z INFO  [gabriel-greeting-rust ← gabriel-greeting-go] rendered banner
2026-04-23T18:42:03.200Z INFO  [gabriel-greeting-go]                          handled greeting
2026-04-23T18:42:03.320Z INFO  [gabriel-greeting-app]                         tab changed
```

One stdout, all languages, ordered by arrival.

Passing `--json` prints one JSON object per line so downstream tools
(`jq`, `fluentd`, `promtail`) can consume the stream unchanged:

```bash
op run gabriel-greeting-app --observe --json | jq 'select(.level=="ERROR")'
```

The same multilog is also persisted on disk for post-mortem:

```bash
op logs gabriel-greeting-app --since 1h    # replay from the on-disk multilog
```

#### Case B — GUI-driven, double-clicking the app

The app is launched directly (double-click the `.app` bundle, tap the
icon on the dock, etc.). There is no parent launcher — no `OP_OBS`,
no `OP_INSTANCE_UID`. The app still offers full observability via an
in-app panel.

On first run the panel is OFF. Opening it reveals:

1. **Master switch** — `Enable observability`. Flipping it on turns
   every declared family on at the SDK level (the rings were
   always-allocated; see OBSERVABILITY.md §Runtime Gates).
2. **Per-family toggles** — Logs, Metrics, Events, Prom. Independent.
3. **Log level slider** — TRACE … FATAL.
4. **Relay settings** — master `Relay all members` + one row per
   bundled member with a tri-valued override (`default` / `on` / `off`).
5. **Console** tab — live view of the aggregated LogRing (own logs
   + relayed entries), filters by origin `chain[0].slug`, level,
   free text; pause / resume; export as JSONL.
6. **Metrics** tab — counters, gauges (with inline sparklines),
   histograms (expandable to bucket distribution); refresh interval
   slider; export as Prometheus text.
7. **Events** tab — timeline of lifecycle events.
8. **Prometheus endpoint** toggle — when ON, the kit binds a local
   HTTP listener on an ephemeral port and shows the address for
   `curl http://127.0.0.1:<port>/metrics`. No `op run` required.
9. **Export snapshot** button — writes a bundle
   (`observability-<slug>-<ts>/`) containing `logs.jsonl`,
   `events.jsonl`, `metrics.prom`, `metadata.json` (app version, OS,
   SDK versions, gate state at snapshot time) to a location chosen
   by the user.

All choices persist via the app's `SettingsStore` under the
`observability.*` namespace, so the next launch of the app re-enters
the exact state the user had left.

The console view reads the same ring buffers that `op run --observe`
would scrape remotely in Case A. The behaviour is identical; only the
UX differs.

### 2.2 Debugging "which child emitted this?"

The `chain` field on every entry traces the relay path. One-liner to
pretty-print it:

```bash
op logs gabriel-greeting-app --json --follow \
  | jq -r '"\(.ts) \(.level) [\(.chain | map(.slug) | join(" ← "))] \(.message)"'

# 2026-04-23T18:42:03.112Z INFO [gabriel-greeting-rust ← gabriel-greeting-go] rendered banner
# 2026-04-23T18:42:03.200Z INFO [gabriel-greeting-go] handled greeting
```

The bracket contains the full lineage; the outermost is the nearest
relay to the root.

### 2.3 Three-level trees

The model is recursive. A Flutter organism that itself is embedded
inside a larger supervisor produces `chain` arrays of depth 3+. `jq`
handles any depth:

```bash
# Count signals per originator across the whole tree
op logs gabriel-greeting-app --json --follow=false \
  | jq -s 'group_by(.slug) | map({slug: .[0].slug, count: length}) | sort_by(-.count)'
```

---

## 3. Dispositions des flux remontants

The multilog is the canonical landing zone for everything a composite
produces. These recipes cover what to **do** with it.

### 3.1 Tail the multilog

```bash
# Live, structured
op logs gabriel-greeting-app --follow

# Live, JSON (one object per line) — pipe to any JSON-native tool
op logs gabriel-greeting-app --follow --json

# Replay the last hour and exit
op logs gabriel-greeting-app --since 1h --follow=false

# Filter by origin (any depth)
op logs gabriel-greeting-app --chain-origin gabriel-greeting-rust --follow
```

### 3.2 Reconstruct signal genealogy

```bash
# Show who relayed what, one line per signal
op logs gabriel-greeting-app --json --follow \
  | jq -r 'select(.kind=="log")
           | "\(.ts) \(.slug) via \(.chain | map(.slug) | join("/"))"'
```

### 3.3 Forward to Loki / Splunk / Elasticsearch

The multilog file is standard JSON lines — any log shipper reads it
directly.

**Loki (via Promtail)**:

```yaml
# promtail.yml
scrape_configs:
  - job_name: holon-multilog
    static_configs:
      - targets: [localhost]
        labels:
          job: holon
          __path__: /Users/bpds/.op/run/*/*/multilog.jsonl*
    pipeline_stages:
      - json:
          expressions:
            level: level
            slug: slug
            origin: chain[0].slug
      - labels:
          level:
          slug:
          origin:
```

**Splunk Universal Forwarder**: point `inputs.conf` at the multilog
path with `sourcetype=_json`. The JSON fields are indexed natively.

**Elasticsearch (Filebeat)**:

```yaml
# filebeat.yml
filebeat.inputs:
  - type: filestream
    paths:
      - /Users/bpds/.op/run/*/*/multilog.jsonl*
    parsers:
      - ndjson:
          target: ""
```

### 3.4 Grafana Live streaming

Grafana Live accepts newline-delimited JSON over WebSocket. One-liner
to stream the multilog into a Grafana dashboard without any collector:

```bash
op logs gabriel-greeting-app --follow --json \
  | curl -N --data-binary @- \
         -H "Authorization: Bearer $GRAFANA_TOKEN" \
         https://grafana.example.com/api/live/push/holon-multilog
```

The Grafana panel consumes the `holon-multilog` channel with a Logs
visualization.

### 3.5 Post-mortem on a crashed organism

If the root crashed:

```bash
# Multilog holds everything up to the last flush
tail -n 1000 .op/run/gabriel-greeting-app/<uid>/multilog.jsonl | jq .

# Per-member forensic (each member wrote its own copy)
find .op/run/gabriel-greeting-app/<uid>/members -name 'stdout.log' -exec \
  sh -c 'echo "=== $1 ==="; tail -n 100 "$1"' _ {} \;
```

If only a member crashed, the root's multilog has all its signals up
to the crash; the local `stdout.log` in that member's directory is
identical up to the same point (dual filesystem).

### 3.6 Replay / record & play-back

The multilog is a time-ordered signal trace — it can be replayed for
offline analysis or reproduction:

```bash
# Feed a recorded multilog through the same tooling as a live stream
cat .op/run/gabriel-greeting-app/<uid>/multilog.jsonl \
  | op logs --replay --json \
  | <any downstream consumer>
```

(The `op logs --replay` pass-through honors `--since`, `--level`, and
`--chain-origin` filters on the file contents.)

---

## 4. Diagnostic recipes

### 4.1 Production error — find the guilty RPC

```bash
# All ERROR-level entries from the last hour, grouped by rpc_method
op logs gabriel-greeting-app --level=error --since 1h --json \
  | jq -r '.rpc_method' | sort | uniq -c | sort -rn
```

Then pivot into the offending method:

```bash
op logs gabriel-greeting-app --method <MethodName> --level=error --json \
  | jq -s 'group_by(.slug) | map({slug: .[0].slug, n: length, sample: .[0]})'
```

### 4.2 Memory grows over time — who holds?

Session-scoped `holon_session_rpc_in_flight` gauge shows which
handlers are open. Sort descending:

```bash
op metrics gabriel-greeting-app --prom \
  | grep holon_session_rpc_in_flight \
  | sort -t'=' -k2 -n -r | head
```

For Go holons specifically, `holon_goroutines` gauge indicates
goroutine growth; pair with `op events --type HANDLER_PANIC --since 1h`
to catch handlers leaking on panic paths.

### 4.3 Latency spike — queue, work, or wire?

Four-phase decomposition tells you where the time went:

```bash
op metrics gabriel-greeting-app --prom \
  | grep -E 'holon_session_rpc_duration_seconds_bucket\{.*method="Transcode".*'
```

Group by `phase` label. If P99 is in `work`, the handler is slow. If
`queue`, concurrency saturation. If `wire_out`/`wire_in`, message
size or transport. Cross-reference with the live
[SESSIONS.md](SESSIONS.md) `SessionMetrics` via
`op sessions <slug> --metrics`.

### 4.4 A child holon crashed — read the post-mortem

```bash
# Root events stream catches the crash
op events gabriel-greeting-app --type INSTANCE_CRASHED --since 10m

# Then look at the victim's local stderr and last entries
uid=$(op events gabriel-greeting-app --type INSTANCE_CRASHED --json --follow=false \
      | jq -r '.instance_uid' | head -n1)
slug=$(op events gabriel-greeting-app --type INSTANCE_CRASHED --json --follow=false \
      | jq -r '.slug' | head -n1)
tail -n 500 .op/run/gabriel-greeting-app/*/members/$slug/$uid/stderr.log
```

Every `INSTANCE_CRASHED` event carries `payload.exit_code` and (if
the holon emitted a FATAL log before dying) `payload.fatal_msg`.

### 4.5 Two holons disagree — compare sessions

When an RPC between holon A and holon B gives conflicting diagnoses,
compare both ends of the session:

```bash
session_id=<id>
op logs A --session=$session_id --json | jq -s 'sort_by(.ts)' > a.json
op logs B --session=$session_id --json | jq -s 'sort_by(.ts)' > b.json
diff <(jq -c '.[]' a.json) <(jq -c '.[]' b.json)
```

---

## 5. Exporting

### 5.1 OTLP to an existing collector (v2)

> **v2.** OTLP push is not wired in v1. The `otel` token and
> `--otel=...` flag are reserved and produce a startup error on
> current SDKs. Use the Prometheus `/metrics` endpoint (§1.3)
> in the meantime — Grafana Agent and OTel Collector both support
> Prometheus scrape as an ingest source for OTLP backends.

When the OTLP exporter ships (tracked in [OBSERVABILITY.md §Phasing](OBSERVABILITY.md#phasing)),
the recipe becomes:

```bash
op run gabriel-greeting-app --observe --otel=otel-collector:4317
```

Or:

```bash
OP_OBS=all,otel OP_OTEL_ENDPOINT=otel-collector:4317 \
  op run gabriel-greeting-app
```

The SDK will push every 15 seconds (override with
`OP_OTEL_INTERVAL=30s`). Metrics map to OTLP Metrics; logs and events
map to OTLP LogRecords.

### 5.2 Keeping cardinality under control

Unbounded labels explode Prometheus storage. Discipline:

- Never derive a label from a user-supplied string (IP, hostname, raw
  input).
- `remote_slug` is safe — it's one of your own holon slugs or
  `"anonymous"`.
- If you must label by a free dimension, add it to the log `fields`
  map instead, not to metric labels. The multilog keeps it
  structured; metrics stay bounded.

---

## 6. Reference quick cards

### 6.1 `OP_OBS` env vocabulary

```
OP_OBS=                      # disabled (default)
OP_OBS=logs                  # structured logs only
OP_OBS=metrics               # metrics collection only
OP_OBS=events                # lifecycle events only
OP_OBS=logs,metrics,events   # combined signal capture
OP_OBS=all                   # alias for logs,metrics,events,prom
OP_OBS=all,otel              # v2 only — plus OTLP push; rejected in v1
```

### 6.2 `op` flag table

| Verb | Key flags |
|---|---|
| `op logs <slug>` | `--since`, `--level`, `--session`, `--method`, `--follow`, `--json`, `--chain-origin`, `--local`, `--all` |
| `op metrics <slug>` | `--prom`, `--prefix`, `--include-session-rollup`, `--all` |
| `op events <slug>` | `--type`, `--since`, `--follow`, `--json`, `--all` |
| `op ps` | `--all`, `--slug`, `--stale`, `--json`, `--tree`, `--flat` |

### 6.3 Metric naming cheat sheet

- Prefix `holon_` for every metric.
- `_total` suffix → counter.
- `_seconds` / `_bytes` suffix → unit.
- `holon_session_` sub-namespace → session-scoped.
- `phase ∈ {wire_out, queue, work, wire_in, total}` is the label
  for four-phase decomposition.

---

## 7. When this handbook lies

Recipes assume behavior specified in [OBSERVABILITY.md](OBSERVABILITY.md)
has actually landed in code. Implementation across the 14 SDKs is
staged; use [INDEX.md](INDEX.md) to check which SDK currently claims
conformance. When the spec and the code disagree,
[CLAUDE.md](CLAUDE.md) rule applies: cross-check doc vs code vs proto,
ask the composer when still ambiguous.
