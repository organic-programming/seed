# Observability — robust cross-SDK implementation

You are implementing the observability layer described in the seed
repository's specification documents. The specs have been refined to
a point where the architecture is unambiguous; your job is to take
them from specified to shipped.

---

## Ground truth (read first, in this order)

1. `OBSERVABILITY.md` — main spec. Pay special attention to:
   - §Tier scope matrix — defines exactly which SDK carries which
     obligations. **Do not add Prometheus / OTLP / multilog writer to
     tier-2 SDKs.**
   - §Design Principles — §2 covers *Explicit activation, never
     silent*. The standalone-app clause is load-bearing for the GUI
     kits.
   - §Runtime gates — always-alloc rings + atomic gate at emit. This
     is the mental model for the tier-1 kits; tier-2 stays figé.
   - §Proto Definition — the wire contract for everything you emit
     or consume.
   - §Organism Relay + §Multilog Contract — the append-child-hop
     rule and the enriched chain the root writes to disk.
2. `OBSERVABILITY_HANDBOOK.md` — Case A (`op run --observe`) and
   Case B (double-click GUI app) recipes are your acceptance targets.
3. `INSTANCES.md` — §Path Resolution now clearly states `OP_RUN_DIR`
   is the **registry root**; the SDK derives `$OP_RUN_DIR/$slug/$uid/`.
   `op run` must honour this; `op ps` walks from the root.
4. `SESSIONS.md` — v2-only. Reject `OP_SESSIONS` at startup in v1.
5. `organism_kits/README.md` — tier-1 widget obligations for the
   kits plus the theming / customisation contract.

---

## Scope by phase

Work in the listed order. Each phase ends with a commit on
`feat/observability-impl`, `go build ./...` clean on the workspace,
unit tests green, and `ader test ader/catalogues/grace-op` green at
smoke profile when affected. Do not proceed to the next phase until
the previous one is committed and pushed.

### R — Robustness blockers on the Go reference

- **R1**. Fix `OP_RUN_DIR` semantics everywhere:
  - SDK (`sdk/go-holons/pkg/observability/writer.go`,
    `pkg/serve/serve.go`): `OP_RUN_DIR` + slug + UID → compute
    `<OP_RUN_DIR>/<slug>/<uid>/`. Create the directory. Write
    `meta.json`, `stdout.log`, `events.jsonl` there.
  - `op` launcher (`holons/grace-op/internal/cli/run_observe.go`):
    stop injecting a per-instance `OP_RUN_DIR`; inject the registry
    root only. The SDK derives its own subpath.
  - `op ps` (`holons/grace-op/internal/cli/cmd_observability.go`):
    walk `<root>/<slug>/<uid>/meta.json` as already coded; ensure the
    root is the registry root, not a per-instance path.
  - Add unit test asserting the pattern.

- **R2**. Strip v2-only tokens in v1:
  - `sdk/go-holons/pkg/observability/observability.go` CheckEnv /
    parseOPOBS: reject `sessions` / `OP_SESSIONS` at startup the same
    way `otel` is rejected. Propagate the pattern to every tier-2 SDK.
  - Remove references to `session_rollup` being populated in v1 from
    code comments; the field stays in the proto but is always empty
    in v1.

- **R3**. Complete Go baseline metrics per OBSERVABILITY.md §Baseline
  holon metrics:
  - `holon_build_info` (gauge, labels: version, lang, commit) — set
    once at start.
  - `holon_process_start_time_seconds` — set once.
  - `holon_memory_bytes{class=rss|heap|stack}` — refreshed each
    snapshot request via `runtime.ReadMemStats`.
  - `holon_goroutines` (gauge) — `runtime.NumGoroutine`.
  - `holon_handler_in_flight{method}` — already present; ensure
    inc/dec on every unary call.
  - `holon_handler_panics_total{method}` — incremented inside the
    interceptor's recover.
  - `holon_session_rpc_total{method,direction,phase}` — `phase=total`
    in v1 only; tagging parity with the documented label set.
  - `holon_session_rpc_duration_seconds{method,direction,phase}` —
    same.
  - Unit tests that snapshot the Registry and assert each metric
    exists after a handful of RPC calls.

- **R4a**. Wire relay + multilog in the Go serve runner:
  - When `Current().IsOrganismRoot()` returns true, spawn a
    `MultilogWriter` automatically in `serve.RunWithOptions`.
  - Add a `ServeOptions.MemberEndpoints []MemberRef` (slug, uid,
    dial address) so composite apps can declare their direct
    children. When present, spawn a `Relay` per member respecting
    gate state.
  - The kits (R4b, see below) pass member refs from their app-level
    discovery.

- **R5**. Align OTLP language:
  - `OBSERVABILITY.md` already rejects `otel` in v1; ensure all tier-2
    SDKs align identically in their CheckEnv / parseOpObs.
  - `OBSERVABILITY_HANDBOOK.md` §5.1 already carries the v2 note;
    verify no code example suggests `otel` works in v1.

- **R6**. Prometheus HTTP sharing:
  - `sdk/go-holons/pkg/serve/serve.go`: when an `http://` or `https://`
    listener is present and `prom` is enabled, mount `/metrics` on
    that listener's http.ServeMux instead of binding a dedicated
    port. For non-HTTP transports, keep the dedicated ephemeral port.
  - `meta.json.metrics_addr` reflects whichever path is in effect.

- **R7**. New `op run --observe` tail-mode:
  - After a successful spawn + `meta.json` write, if `--observe` is
    set, `op run` dials the launched holon via
    `HolonObservability.Logs(follow=true)` and prints each entry to
    its own stdout in human-readable form (same format as
    `op logs <slug>`). `--json` forwards raw JSON.
  - `op run` continues to propagate the child's stderr inherited as
    usual; the observability stream is **additional** output on
    op's stdout.
  - SIGINT / SIGTERM on `op run` cancels the stream and graceful-shuts
    the child.
  - Ader suite `op_run_observe.yaml` + integration test that spawns
    `gabriel-greeting-go` with `--observe`, asserts INSTANCE_READY
    appears on stdout, then sends SIGTERM and asserts INSTANCE_EXITED.

### A — Real ader coverage

Extend `ader/catalogues/grace-op/integration/` with a fixture capable
of spawning a real observability-enabled holon from within a test
sandbox. Add the `Sandbox.SpawnObservable(slug, options)` helper on
the shared `integration` package that returns a handle with
`RunDir()`, `Address()`, `Stop()` methods.

- **A1**. `ader/catalogues/grace-op/integration/ps/ps_live_test.go`:
  spawn `gabriel-greeting-go` with `OP_OBS=logs,metrics,events`,
  assert `op ps --json` lists exactly 1 instance with matching
  slug/uid, PID alive, address non-empty.

- **A2**. `logs_live_test.go`, `metrics_live_test.go`,
  `events_live_test.go`: spawn the holon, make a couple of dummy
  SayHello calls to generate activity, then run each CLI against
  it. Assert JSON output contains expected fields.

- **A3**. `multilog_live_test.go`: spawn a composite Go scenario
  (one organism root + two direct children), verify
  `multilog.jsonl` contains both relayed entries with chain depth 2
  plus the root's own entries with chain depth 1 (after
  §Multilog chain enrichment).

- **A4**. Register the four suites in
  `ader/bouquets/grace-op-integration.yaml` so they run as part of
  the default integration gate.

- **A5**. Update suite YAML descriptions and test docstrings to
  match what the tests actually do (no overclaim).

### SDK — tier-2 parity

For every SDK listed in `sdk/*-holons/` EXCEPT `go-holons` (tier 1,
already done) and `js-web-holons` (tier 3, see below):

Per SDK, the scope is **minimal emitter + auto-register**:

1. Load + check OP_OBS, reject `sessions` + `otel` + unknown tokens.
2. Build rings (logs, events) + Registry (counters, gauges,
   histograms) when the relevant family is enabled. These are
   already in place for most SDKs; audit and fix gaps.
3. **Auto-register `HolonObservability` on the SDK's serve runner**
   when any family is enabled. This is the piece that is missing in
   most SDKs today. The service implementation streams from the
   rings / registry / event bus; reuse the proto types that have
   already been generated.
4. Write `stdout.log`, `events.jsonl`, `meta.json` under
   `$OP_RUN_DIR/$slug/$uid/` (derive subpath from root + slug + uid
   exactly as the Go SDK does).
5. **Do not** add Prometheus HTTP, OTLP push, relay, or multilog.
6. A focused unit / integration test per SDK that asserts:
   - CheckEnv rejects bad tokens.
   - Log entries, events, and metric samples flow end-to-end when
     families are enabled.
   - `HolonObservability.Logs(follow=false)` returns the ring contents.

Target SDKs: `python-holons`, `js-holons`, `dart-holons`,
`swift-holons`, `rust-holons`, `java-holons`, `kotlin-holons`,
`csharp-holons`, `ruby-holons`, `c-holons`, `cpp-holons`.

For `js-web-holons` (tier 3): the service serves over the existing
WebSocket channel, not a local gRPC listener. No disk writers. Keep
the surface minimal; the Hub pulls via the bidirectional Holon-RPC
binding of COMMUNICATION.md §4.1.

### K — Organism kits (tier 1, full chain)

Duplicate the full Go-side observability chain at the kit level, in
both `organism_kits/flutter/` (Dart) and `organism_kits/swiftui/`
(Swift). See `organism_kits/README.md` §Observability (tier 1) for
the required surfaces.

- **K1**. `ObservabilityKit.standalone(...)` bootstrap. Takes a
  slug, declared families list, persisted `SettingsStore`,
  `bundledHolons` iterable. Returns the kit handle.

- **K2**. Runtime gates controller — per-family atomic bool and
  per-member tri-valued override (`default` / `on` / `off`) with
  a master.

- **K3**. Console / Metrics / Events controllers — subscribe to the
  LogRing / EventBus / Registry respectively, expose filtered
  observables the widgets can bind to. Metrics controller
  maintains a sliding history (last 30 snapshots) for sparklines.

- **K4**. Relay controller — one `Relay` per bundled member, honouring
  the tri-valued gate. Start / stop on flip.

- **K5**. Prometheus HTTP toggle — when ON, bind
  `HttpServer.bind(...)` (Dart) or `NWListener` (Swift) on an
  ephemeral port, serve `/metrics` with the same exposition format
  as the Go SDK. Expose the bound address as a reactive property.

- **K6**. Export controller — snapshot bundle (`observability-<slug>-<ts>/`)
  containing `logs.jsonl`, `events.jsonl`, `metrics.prom`,
  `metadata.json`. Target dir chosen via `file_picker` (Dart) /
  `NSSavePanel` (Swift).

- **K7**. Reusable widgets:
  - `LogConsoleView`, `MetricsView`, `EventsView`,
    `RelaySettingsView`, `SparklineView`, `HistogramChart`.
  - `ObservabilityPanel` — 4-tab aggregator.
  - **Minimalist** (option (b) in the plan): no external theming
    dependency; Flutter uses stock Material primitives, Swift uses
    stock SwiftUI primitives.
  - Each widget ships a neighbouring `README.md` documenting public
    API, visual customisation points (theme inheritance, colour
    palette for level badges / origin slugs / event-type badges,
    typography, icon swap), sizing contract, embedding recipes.

- **K8**. Parity tests between Flutter and SwiftUI kits on the same
  fixture data (JSONL sample exported from a real greeter run).
  Unit tests per controller on each platform.

### COAX widget refactor (parallel to K)

Existing `organism_kits/flutter/lib/src/ui/coax_control_bar.dart`
and `coax_settings_dialog.dart` (and their SwiftUI equivalents) must
graduate to the same widget contract as the observability widgets:
minimalist defaults, neighbouring README documenting theming, no
external theming dep. Move them out of `src/ui/` into a public
widget surface (`lib/widgets/` in Flutter, `Sources/HolonsApp/Views/`
in Swift). The two reference apps then re-skin these widgets through
their own theme to preserve the current look.

### E — Example apps wire-up

- **E1**. `examples/hello-world/gabriel-greeting-app-flutter`:
  - `app/lib/main.dart`: bootstrap via
    `ObservabilityKit.standalone(...)`. Feed `bundledHolons` from
    `GreetingController.holons`.
  - Add a menu entry / drawer item opening an
    `ObservabilityPanel` (full-screen tab or modal sheet).
  - App theme is applied on the panel — demonstrate the theming
    contract of the widgets.
  - Persist gate state through the same `SettingsStore` the app
    already uses for COAX.

- **E2**. `examples/hello-world/gabriel-greeting-app-swiftui`:
  - `App/GabrielGreetingApp.swift`: `@StateObject var obs =
    ObservabilityKit.standalone(...)`.
  - Add an `ObservabilityPanel` accessible from the main window
    (tab, split view, or menu command).
  - App theme (colors / typography) applied; the panel inherits.

Both apps must pass their existing ader catalogue after the
wire-up. Add a new check per app that verifies the panel opens
and reads the kit state.

### D — Docs cleanup

- **D1**. `OBSERVABILITY.md` line ~10: already says 13 SDKs now;
  verify no stale "14 SDKs" reference remains anywhere.
- **D2**. Handbook OTLP language already marked v2; verify.
- **D3**. Ader checks.yaml descriptions updated to match what the
  tests actually do.
- **D4**. `INDEX.md`: add a cross-SDK observability conformance
  matrix (one row per SDK, columns: Logs / Metrics / Events / Register /
  DiskWriters / Prom / Relay / Multilog). Tier-1 SDKs fill all
  columns; tier-2 fill only the emitter columns.

---

## Acceptance criteria (all must hold at the end)

1. `go build ./...` clean on the entire `go.work`.
2. `go test ./... -race` clean on every Go module in `go.work`.
3. Per-SDK unit / integration tests pass for all 13 SDKs, with
   build instructions documented in each SDK's README.
4. `ader test ader/catalogues/grace-op --profile smoke` green,
   including the new `op_ps`, `op_logs`, `op_metrics`, `op_events`,
   `op_run_observe`, `multilog_live` suites.
5. `op build gabriel-greeting-app-flutter` and
   `op build gabriel-greeting-app-swiftui` both produce their
   respective artifacts.
6. Launching either app (`op run …` or double-click) → opening the
   `ObservabilityPanel` → flipping master ON → flipping
   `logs` ON → interacting with the app produces live log entries
   in the console view.
7. `op run gabriel-greeting-app-flutter --observe` tails all logs
   from all members to op's stdout, with `chain` annotations.
8. `curl http://127.0.0.1:<metrics_addr>/metrics` returns a valid
   Prometheus text body for any tier-1 holon with `prom` active.
9. Grafana can scrape the endpoint; a starter dashboard at
   `docs/grafana/holon-overview.json` (optional, if not present
   already) renders.
10. No code path references `OP_SESSIONS` as a working feature in
    v1.

---

## Discipline

- One commit per phase (R1, R2, … E2, D4). Atomic.
- Branch: `feat/observability-impl` (recreated from dev if absent).
- No `--no-verify`, no amends, no force-push.
- `Co-Authored-By:` trailer on every commit identifying the CODEX
  session.
- Open a PR against `dev` once R + A + SDK + K + E + D are all
  green. No direct push to `dev` from the branch — the merge is the
  composer's decision.
- When a step reveals a spec ambiguity, stop and surface the
  ambiguity in the PR description with two or three proposed
  resolutions. Do not guess silently.

---

## What is **out** of scope

- OTLP exporter (v2).
- `op proxy` integration (v2).
- Distributed tracing / OpenTelemetry spans (v3).
- `HolonSession` / session metrics store / four-phase wire_out /
  queue / work / wire_in decomposition (v2).
- Any change to `COMMUNICATION.md`, `CONSTITUTION.md`,
  `CONVENTIONS.md`, `DISCOVERY.md`, `PROTO.md`, `PACKAGE.md`.
- Changing the proto package of any existing service.

---

## Final sanity check before opening the PR

Run through the list manually:

- [ ] All tier-2 SDKs auto-register `HolonObservability`.
- [ ] All tier-2 SDKs reject `sessions` and `otel` at startup.
- [ ] `OP_RUN_DIR` is the registry root everywhere; `op ps` walks it.
- [ ] `op run --observe` tails stdout for the launched holon.
- [ ] Both composite apps ship an `ObservabilityPanel` that works
      under double-click launch.
- [ ] Per-widget README exists for the kit widgets AND the COAX
      widgets.
- [ ] Ader bouquet runs the new suites by default.
- [ ] No doc claims a v2 feature as active in v1.
