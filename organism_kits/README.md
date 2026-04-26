# Organism Kits

An **Organism Kit** is the shared library that sits between a UI framework and the Holons SDK
in a composite app holon (a **COAX**). It handles discovery, connection, lifecycle, and COAX
server plumbing so that app code stays in the domain — views, RPC stubs, domain state.

Two kits ship in this directory:

| Kit | Package | Framework |
|-----|---------|-----------|
| `flutter/` | `holons_app` (Dart) | Flutter desktop / mobile |
| `swiftui/` | `HolonsApp` (Swift) | SwiftUI macOS / iOS |

---

## What Is a COAX?

A **COAX** is a composite app holon — the whole organism. Like every holon it exposes
`HolonMeta/Describe` and is controllable via RPC. The UI is its human-facing surface; the
gRPC server is its machine-facing surface. Both drive the same underlying state.

> [!IMPORTANT]
> The COAX gRPC server is a **user opt-in**. The human user enables it via a toggle in the
> app's settings. It is off by default. When enabled, it can also be pre-configured via
> environment variables (`OP_COAX_SERVER_ENABLED`, `OP_COAX_SERVER_LISTEN_URI`).

A COAX hosts **member holons** (the organs) and gives agents and humans equivalent access
to its capabilities:

| Action | Human | Agent |
|--------|-------|-------|
| Browse members | holon picker | `ListMembers` RPC |
| Select a member | click dropdown | `ConnectMember` RPC |
| Drive domain actions | interact with UI | domain service RPCs |

COAX is not limited to CLI control. Some organisms (Go + web UI) preserve a CLI; others
(SwiftUI, Flutter) are RPC-only. The common invariant is: **all functional interactions are
reachable via RPC when COAX is enabled**.

### Sandboxed (hardened) vs. development

Both apps support a `--hardened` build mode (`op build --hardened`) for App Store / sandbox
environments:

- `op build --hardened` sets `OP_BUILD_HARDENED=true` in the recipe environment.
- **SwiftUI**: `xcodegen` expands a `{{ if .Hardened }}` guard in `project.yml` to inject
  `CODE_SIGN_ENTITLEMENTS`, enabling App Sandbox.
- **Flutter**: `tool/package_desktop.dart` reads `OP_BUILD_HARDENED` via `_isHardenedBuild()`
  and passes entitlements to `codesign`.
- At runtime both apps automatically fall back from `unix` to `tcp` when running inside a
  sandboxed `.app` bundle (`effectiveHolonTransport()` / `ConnectOptions` detection).
- The kit packages themselves are sandbox-agnostic; hardened mode is a build/packaging concern.

---

## Layer Responsibilities

```
SDK (sdk/dart-holons · sdk/swift-holons)
  • Discover()                   — enumerate available holons
  • connect(slug, options)       — fork, negotiate transport, return channel
  • disconnect(channel)          — teardown
  • startWithOptions(uri, svcs)  — start a gRPC server (COAX surface)
  • Describe.useStaticResponse() — register the app's Describe payload

Organism Kit (this directory)
  • HolonCatalog     — wrap Discover(), filter, deduplicate, sort
  • HolonConnector   — wrap connect(), handle sandbox transport fallbacks
  • CoaxServer       — manage server lifecycle, persist settings, env overrides
  • CoaxConfiguration — CoaxServerTransport, CoaxSettingsSnapshot, env parsing

App code (examples — not prescribed)
  • UI views
  • Domain RPC service provider (e.g. GreetingAppService)
  • Domain state (language list, greeting text, user name)
  • Describe payload registration (app-specific proto content)
  • Member identity type (e.g. GabrielHolonIdentity — slug filter, display names, sort ranks)
```

> [!IMPORTANT]
> **Kits wrap the SDK — they never rewrite it.**
> `holons_app` depends on [`sdk/dart-holons`](../sdk/dart-holons) and `HolonsApp` depends on
> [`sdk/swift-holons`](../sdk/swift-holons). If a kit needs a capability the SDK does not yet
> expose, the correct fix is to **add it to the SDK**, not to re-implement it in the kit.

---

## `holons_app` — Flutter / Dart

**Package path:** `organism_kits/flutter/`  
**Used by:** `examples/hello-world/gabriel-greeting-app-flutter` (`pubspec.yaml` path dep)  
**Scaffolding:** `op new --template coax-flutter <slug>`

### Public API

| Symbol | Role |
|--------|------|
| `HolonCatalog` / `DesktopHolonCatalog` | Discover + deduplicate holons via SDK |
| `HolonConnector` / `DesktopHolonConnector` | Connect a holon; `effectiveHolonTransport()` handles sandbox fallback |
| `CoaxController` | `ChangeNotifier` — COAX server start/stop, settings persistence |
| `CoaxServerTransport` | `tcp` \| `unix` enum |
| `CoaxSettingsSnapshot` | Persisted server endpoint settings |
| `CoaxSurfaceState` / `CoaxSurfaceStatus` | Server state badge model |
| `AppPlatformCapabilities` | Available transports for the current platform |
| `normalizedTransportSelection()` | Canonical transport name from any string |
| `sanitizedPort()` | Port validation with fallback |
| `findAppProtoDir()` | Locates the proto dir for `Describe` registration |
| `SettingsStore` | Thin `SharedPreferences` wrapper |

### What the app still owns

- `GabrielHolonIdentity` (slug filter, sort ranks, display names, `fromDiscovered`)
- `GreetingAppRpcService` handler
- The `Describe` payload (`ensureAppDescribeRegistered`, `_coaxServiceDoc`, all field docs)
- `GreetingController` domain state (language list, greeting text, user name)

---

## `HolonsApp` — SwiftUI / Swift

**Package path:** `organism_kits/swiftui/`  
**Used by:** `examples/hello-world/gabriel-greeting-app-swiftui` (`Modules/Package.swift` path dep)  
**Scaffolding:** `op new --template coax-swiftui <slug>`

### Public API

| Symbol | Role |
|--------|------|
| `CoaxServerTransport` | `.tcp` \| `.unix` — `Codable`, `CaseIterable` |
| `CoaxSettingsSnapshot` | Persisted server endpoint settings |
| `CoaxLaunchOverrides` / `coaxLaunchOverrides()` | Parse `OP_COAX_SERVER_ENABLED` / `OP_COAX_SERVER_LISTEN_URI` |
| `resolvedCoaxEnabled()` | Merge stored + env-override enable state |
| `CoaxServer` | `ObservableObject` — COAX server start/stop, settings persistence |
| `CoaxServiceProvider` | `holons.v1.CoaxService` handler |
| `OrganismState` | Protocol that `CoaxServiceProvider` calls into the app model |
| `GreetingTransportName` | Transport name normalization |

### What the app still owns

- `GabrielHolonIdentity` (slug filter, sort ranks, display names, `init(entry:)`)
- `HolonProcess` — concrete `ObservableObject` with discovery + connection lifecycle
- `GreetingClient`, `GreetingAppServiceProvider`, `GreetingSelectionLogic`, `Models`
- `CoaxDescribeProvider` — app-specific `Describe` payload builder + registration

---

## Reference Examples

Both examples are fully working hello-world COAXes:

| App | Framework | COAX transport | Sandbox |
|-----|-----------|---------------|---------|
| `gabriel-greeting-app-swiftui` | SwiftUI + Swift | tcp or unix | ✓ hardened build |
| `gabriel-greeting-app-flutter` | Flutter + Dart | tcp or unix | ✓ hardened build |

In development, the examples surface Gabriel member holons written in C, C++,
C#, Dart, Go, Java, Kotlin, Node, Python, Ruby, Rust, Swift, and Zig. Hardened
builds package the standalone C, C++, Dart, Go, Rust, Swift, and Zig members.

Each app:
1. Discovers sibling `gabriel-greeting-*` holons via the SDK.
2. Connects a selected holon.
3. Starts an embedded gRPC server (COAX surface) registering `holons.v1.CoaxService`,
   `greeting.v1.GreetingAppService`, and `HolonMeta/Describe`.

---

## Observability (tier 1)

Each kit ships the **full observability chain** alongside its COAX
plumbing. Tier-1 obligations per OBSERVABILITY.md §Tier scope matrix:

| Surface | Flutter (`holons_app`) | SwiftUI (`HolonsApp`) |
|---|---|---|
| `ObservabilityKit.standalone(...)` | `lib/observability/observability_kit.dart` | `Sources/HolonsApp/Observability/ObservabilityKit.swift` |
| Per-family runtime gate | `RuntimeGate` (`ValueNotifier`) | `RuntimeGate` (`ObservableObject`) |
| Per-member relay gate (master + overrides) | `RelayController` | `RelayController` |
| Console controller (filtered LogRing view) | `ConsoleController` | `ConsoleController` |
| Metrics controller (snapshot + sparkline history) | `MetricsController` | `MetricsController` |
| Events controller (filtered event stream) | `EventsController` | `EventsController` |
| In-memory Prometheus `/metrics` HTTP toggle | `HttpServer.bind(...)` via `dart:io` | `NWListener` via Network.framework |
| Export snapshot bundle | `ExportController` (JSONL + Prom text + metadata) | `ExportController` (same fields) |

### Reusable widgets

Each kit provides atomic, minimalist widgets designed to be embedded
in any app that depends on the kit. The widgets carry **no external
styling dependency** — they render on primitive `Material` (Flutter)
and `SwiftUI` primitives only. Apps customise the appearance by
wrapping in their own theme.

| Widget (Flutter) | SwiftUI equivalent | Role |
|---|---|---|
| `ObservabilityPanel` | `ObservabilityPanel` | 4-tab aggregator: Settings / Logs / Metrics / Events |
| `LogConsoleView` | `LogConsoleView` | Virtualised list with filters, pause, export |
| `MetricsView` | `MetricsView` | Counters table + gauges with sparklines + histograms |
| `EventsView` | `EventsView` | Lifecycle event timeline |
| `RelaySettingsView` | `RelaySettingsView` | Master + per-member overrides |
| `SparklineView` | `SparklineView` | Custom painter / custom renderer for gauge history |
| `HistogramChart` | `HistogramChart` | Bucket distribution bar chart |

Each widget has its own README.md next to the source file. The
README documents:

1. **Public API** — constructor parameters (kit instance, initial
   filters, refresh interval).
2. **Visual customisation points** — theme overrides (`ThemeData`
   inheritance in Flutter; `Environment(\.colorScheme)` and custom
   modifiers in SwiftUI), colour palettes for level badges / origin
   slugs / event-type badges, typography, icon set swap.
3. **Sizing contract** — minimum / preferred dimensions, whether the
   widget flexes or is fixed.
4. **Embedding recipes** — example of nesting inside the app's own
   navigation (tab, drawer, sheet, split-view).

### App-level customisation expectation

The reference apps (`gabriel-greeting-app-flutter` and
`gabriel-greeting-app-swiftui`) are expected to customise the widgets
so their visual identity stays intact after adding the observability
panel. The customisation lives entirely in the app, not in the kit:
the kit ships a neutral default; the app wraps the widgets in its
own theme. The two reference apps serve as end-to-end examples of
the theming contract.

### Settings persistence

All toggles (master, per-family, per-level, per-member relay,
prom on/off, prom addr) persist through the existing
`SettingsStore` API the kit already uses for COAX, under the
`observability.*` namespace. No second persistence mechanism.

### COAX widget parity

The existing COAX control bar and settings dialog
(`flutter/lib/src/ui/coax_control_bar.dart`,
`flutter/lib/src/ui/coax_settings_dialog.dart`, and the SwiftUI
equivalents) follow the same widget contract: minimalist defaults,
theming documented in a neighbouring README, apps override to
preserve their look. They are first-class reusable widgets, not
app-local components.

---

## Regression Tests

Both example apps have dedicated [ader](../ader/README.md) catalogues that are the
authoritative regression gate for any change to kit or example code:

| Catalogue | Run command |
|-----------|-------------|
| Flutter | `go run holons/clem-ader/cmd/main.go test ader/catalogues/gabriel-greeting-app-flutter --lane regression` |
| SwiftUI | `go run holons/clem-ader/cmd/main.go test ader/catalogues/gabriel-greeting-app-swiftui --lane regression` |

Key checks:

| Check pattern | What it verifies |
|--------------|-----------------|
| `flutter-app-analyze` | Static analysis |
| `flutter-app-unit` / `swiftui-modules-unit` | Unit + widget tests |
| `integration-build-composite-*` | Full `op build` of the composite app |
| `integration-coax-*-cold-build-*` | COAX cold build — build + verify server starts |
| `integration-coax-*-surface-*` | COAX runtime — call RPCs, verify UI state changes |
| `integration-coax-*-matrix-*` | Domain matrix — all holons × transports × RPCs |

Any change to kit or example code must pass **both** catalogues.
