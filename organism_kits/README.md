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

Each app:
1. Discovers sibling `gabriel-greeting-*` holons via the SDK.
2. Connects a selected holon.
3. Starts an embedded gRPC server (COAX surface) registering `holons.v1.CoaxService`,
   `greeting.v1.GreetingAppService`, and `HolonMeta/Describe`.

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
