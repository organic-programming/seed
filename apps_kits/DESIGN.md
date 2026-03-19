# Organic Programming — Apps Kits Design Document

> **Status:** Discussion draft
>
> **Prerequisite reading:**
> - [HOLON_PACKAGE.md](../HOLON_PACKAGE.md) — The `.holon` package format and Bundle Integration
> - [HOLON_BUILD.md](../HOLON_BUILD.md) — `op build` and the recipe runner
> - [CONVENTIONS.md](../CONVENTIONS.md) — Per-language structure (⚠️ outdated)

---

## What Is an App Kit?

An OP application is **an organism**: a living composite with an interface
(the UI) and internal organs (holons). An App Kit is the plumbing that
connects them — not a widget toolkit, not a UI framework, but the
organism's nervous system.

The plural **Apps Kits** reflects that we provide one kit per UI
framework (Flutter, SwiftUI, Kotlin Compose…), covering multiple app
types: native desktop, mobile, visionOS, embedded — any context where
an interface hosts holon organs.

```
┌──────────────────────────────────────────┐
│              THE ORGANISM                │
│                                          │
│   ┌──────────────────────────────┐       │
│   │       Interface (UI)         │       │
│   │  SwiftUI / Flutter / Compose │       │
│   │  (app code, examples only)   │       │
│   └─────────┬────────────────────┘       │
│             │ gRPC / mem                 │
│   ┌─────────▼────────────────────┐       │
│   │        App Kit               │       │
│   │  catalog · connect ·         │       │
│   │  lifecycle · picker model    │       │
│   └─────────┬────────────────────┘       │
│             │ delegates to               │
│   ┌─────────▼────────────────────┐       │
│   │       Lang SDK               │       │
│   │  connect(slug) · transport   │       │
│   │  process launch · readiness  │       │
│   └─────────┬────────────────────┘       │
│             │                            │
│   ┌─────────▼────────────────────┐       │
│   │     Holon Organs             │       │
│   │  in bundle or OPBIN          │       │
│   └──────────────────────────────┘       │
└──────────────────────────────────────────┘
```

---

## Two Modes of Operation

| Mode | Organ source | Who manages discovery | `op` required? |
|------|-------------|----------------------|----------------|
| **Development** | Source-tree holons (sibling dirs), `op` discovers and runs them | `op` | Yes — installed via the seed repo or separately |
| **Published app** | `.holon` packages embedded in bundle or installed in `$OPBIN` | App Kit | Generally no — but external holon dependencies are acceptable |

During development, `op` is always available (typically from the **seed
repository**, which provides `op` and everything needed for agents and
humans to build OP systems, though `op` can also be installed separately).
The App Kit delegates discovery to `op` in dev mode.

In a published app, the main difference is that we **plant binary holons,
not source code**. Most published apps are self-contained (holons embedded
in the bundle), but external holon dependencies installed in `$OPBIN` are
valid — an organism can have both embedded and external organs.

---

## How This Articulates with Existing Specs

| Document | What it defines | App Kit's relationship |
|----------|----------------|----------------------|
| **HOLON_PACKAGE.md** | `.holon` package, `.holon.json`, Bundle Integration (`Contents/Resources/Holons/`), discovery order, execution command | App Kit **implements** discovery and execution per this spec |
| **HOLON_BUILD.md** | `op build`, recipe runner, `build_member` + `copy_artifact`, `.holon` package output | App Kit **consumes** what `op build` produces |
| **CONVENTIONS.md** | Per-language holon structure | ⚠️ Outdated — needs alignment (see §Appendix) |

---

## The Problem: Duplicated Organism Plumbing

Every existing host app re-implements the same organism-level concerns in
500–1000 lines. The actual app-specific code (view + RPC stub) is only ~100 lines.

| Concern | Duplicated in every host |
|---------|------------------------|
| Organ discovery (binary resolution, identity) | 200–400 LOC |
| Organ lifecycle (fork, port parse, readiness, cleanup) | 150–300 LOC |
| Organ selection (picker model, refresh, preference) | 50–100 LOC |
| Exit cleanup (stop processes, delete temp dirs) | 30–50 LOC |

---

## App Kit Primitives

### 1. `HolonCatalog` — What Organs Are Available?

**Today:** Every host hard-codes descriptors with variant names, UUIDs,
build runners, binary paths, and sort ranks.

**App Kit should:** Discover available holons automatically.

- **Published app:** Scans `Contents/Resources/Holons/*.holon/`, reads
  `.holon.json` per HOLON_PACKAGE.md §Discovery.
- **Development:** Delegates to `op discover` which scans source trees,
  built packages, and `$OPBIN`.

```
catalog.discover() → [HolonIdentity]
```

The app never lists binaries — it asks the catalog what organs are available.

---

### 2. `HolonConnector` — Activate an Organ

**Today:** Every host implements: create temp dir → write manifest →
copy protos → chdir → (if stdio: `connect(slug)`) or (if tcp/unix: fork
binary, parse URI, write port file, `connect(slug, portFile)`).

**App Kit should:** Provide one call.

```
connector.connect(identity, transport) → ActiveHolon { channel, close() }
```

Internally:
1. Resolves the binary path from `HolonIdentity`.
2. Delegates process launch to the **Lang SDK** (see below).
3. Returns a channel ready for gRPC calls.
4. `close()` disconnects, stops the organ process, cleans up.

> [!IMPORTANT]
> **Design decision:** Process launch (fork, port parsing, readiness
> probing) should be **standardized in the Lang SDK** as a convenience,
> not reimplemented per host. The Lang SDK's `connect(slug)` should
> handle start for all transports, not just stdio. The App Kit calls
> `connect()` and trusts the SDK to manage the process.
>
> **Nuance:** For complex topologies (multi-holon orchestration, custom
> networking), a developer can bypass the convenience and manage process
> launch explicitly. The SDK process launch is the default, not a cage.

---

### 3. `HolonPickerModel` — Organ Selection State

A reactive model naturally bound to the framework:

| Framework | Pattern |
|-----------|---------|
| Flutter/Dart | `ChangeNotifier` |
| SwiftUI | `ObservableObject` / `@Observable` |
| Kotlin Compose | `StateFlow` |

```
class HolonPickerModel {
  available: [HolonIdentity]     // from HolonCatalog
  selected: HolonIdentity?       // stop → reconnect on change
  transport: String               // stop → reconnect on change  
  status: .idle | .connecting | .connected | .error(msg)
  
  refresh()                       // re-scans catalog
}
```

The app binds this to a dropdown and a status indicator. No lifecycle code.

The App Kit ships the **model only**. UI is fully in the app's hands.
The Gabriel greeting apps serve as examples.

---

### 4. Organism Lifecycle

The App Kit hooks into the framework's lifecycle automatically:
- On exit: disconnect + stop organ processes + cleanup.
- The app **never writes cleanup code**.

---

## COAX — Coaccessibility

**COAX** stands for **coaccessibility**: every component in an OP
application must be equally accessible to humans *and* machines.
This is not an afterthought or an accessibility layer bolted on top —
it is a foundational design principle that shapes how Apps Kits are
built and how apps built on them behave.

Traditional frameworks treat human (UI) accessibility and machine
(API) accessibility as separate concerns.  COAX rejects that split:
a component that a human can discover, inspect, and operate should be
just as discoverable, inspectable, and operable by an agent, a script,
or another holon — through the same structural contracts.

### Why Holons Are the Natural Foundation

Holons already carry the properties COAX requires:

| Property | Human side | Machine side |
|----------|-----------|-------------|
| **Identity** | Readable name, description in `.holon.json` | Slug, UUID, proto manifest — programmatic lookup |
| **Contract** | Proto definitions document the API a developer reads | Same protos generate stubs, enabling code-level introspection |
| **Discovery** | `HolonCatalog` lets a user browse available organs | Same catalog is queryable by agents and tooling |
| **Lifecycle** | App Kit manages start/stop transparently for the user | Same lifecycle hooks are scriptable via `op` or SDK calls |

Because every holon is self-describing (manifest, proto contract,
typed RPC surface), any tool — IDE, AI agent, test harness,
orchestrator — can reason about the organism's structure without
special adaptation.

### COAX in Practice

Apps Kits enforce COAX by design:

- **Machine-readable components.** Every `HolonIdentity` exposes
  structured metadata (slugs, capabilities, transport options) — not
  just display strings.  UI labels are derived from the same data
  agents consume.
- **Scriptable lifecycle.** Any action the user triggers through the
  UI (connect, disconnect, refresh catalog) has an equivalent
  programmatic path.  No operation is UI-only.
- **Introspectable state.** `HolonPickerModel` state (available
  organs, selected organ, connection status) is observable both by
  the framework's reactive bindings *and* by external tooling through
  the SDK.
- **Uniform contract surface.** Proto-first design means the same
  `.proto` files serve as human documentation, machine-generated
  stubs, and the basis for automated testing — one source of truth
  for both audiences.

> [!IMPORTANT]
> COAX is not optional polish.  An App Kit component that is operable
> only through a visual interface — with no programmatic equivalent —
> is **incomplete by definition**.  Every primitive (`HolonCatalog`,
> `HolonConnector`, `HolonPickerModel`, lifecycle hooks) must satisfy
> both sides of the coaccessibility contract.

---

## What the App Still Owns

After the App Kit absorbs organism plumbing, the app is **pure domain + interface:**

| Responsibility | Example |
|----------------|---------|
| UI layout | `ContentView.swift`, `greeting_screen.dart` |
| RPC stub wiring | `GreetingClient` — thin wrapper over proto stubs |
| Domain UI state | Language list, selected language, greeting text |

---

## Layer Responsibilities

```
HOLON_PACKAGE.md defines:
  • .holon package format, .holon.json schema
  • Bundle layout (Contents/Resources/Holons/)
  • Discovery order, execution command

HOLON_BUILD.md defines:
  • How op build produces .holon packages
  • Recipe runner (build_member + copy_artifact)
  • How packages get embedded into bundles

Lang SDK provides:
  • connect(slug) → transport negotiation → channel
  • Process launch + readiness for ALL transports  ← evolve here
  • disconnect(channel)

App Kit provides:                    ← THIS DOCUMENT
  • HolonCatalog (implements HOLON_PACKAGE §Discovery)
  • HolonConnector (delegates to Lang SDK connect)
  • HolonPickerModel (reactive state, model only)
  • Organism lifecycle (exit cleanup)

App code provides:
  • UI views (examples furnished, not prescribed)
  • Domain-specific RPC stub binding
  • Domain UI state
```

---

## Priority Tiers

| Tier | Framework | Package | Justification |
|------|-----------|---------|---------------|
| **T1** | Flutter (Dart) | `package:holons_app` | Cross-platform, most recipes |
| **T1** | SwiftUI (Swift) | `HolonsApp` | Apple-native, gabriel-greeting-app-swiftui is the multi-holon reference |
| **T2** | Kotlin Compose | `holons-app` | Android + Desktop, future growth |
| **T3** | .NET, Qt, Web | thin adapters | Lower priority |

---

## Migration Path

1. **Extract** organism plumbing from `gabriel-greeting-app-swiftui`
   into a `HolonsApp` Swift package. This is the new-generation reference.
2. **Build** `gabriel-greeting-app-flutter` as the first Flutter App Kit
   consumer (the gudule hostui-flutter recipes serve as inspiration but
   will be retired when the gabriel apps are complete).
3. **Build** `gabriel-greeting-app-kotlin-compose` using the Kotlin App Kit.
4. **Update** `CONVENTIONS.md` with a new "App Holon Structure" section
   once the pattern stabilizes.

All new apps are named `gabriel-*`.

---

## Resolved Decisions

| Question | Answer |
|----------|--------|
| Process launch ownership | **Lang SDK.** Standardize process management in the SDK for all transports, not just stdio. |
| Widget opinions | **Model only.** The App Kit ships `HolonPickerModel` and lifecycle hooks. UI is fully the app's responsibility. Gabriel apps serve as examples. |
| Naming | **"Holon" only.** The word "daemon" does not exist in OP vocabulary. All APIs, types, logs, and docs use "holon". |
| Dev-mode discovery | **Use `op`.** The seed repo always provides `op`. Published apps don't need `op` — they read `.holon.json` from the bundle or `$OPBIN`. |

---

## Appendix: CONVENTIONS.md Alignment Prompt

`CONVENTIONS.md` needs updating to articulate with this Apps Kits and the
proto-first model. Here is a prompt for a separate task:

```
Update CONVENTIONS.md to align with the current Organic Programming model:

1. Replace `protos/` with `api/v1/` as the canonical proto location.
2. Replace `holon.yaml` references with `api/v1/holon.proto` carrying
   `option (holons.v1.manifest)`.
3. Add a "Facet Model" section documenting the 4 facets:
   Code API (api/public.*), CLI (api/cli.*), RPC (_internal/server.*),
   Test (*_test.*).
4. Add an "App Holon Structure" section describing how a composite/app
   holon is structured (recipe, members, App Kit usage).
5. Update the per-language source/gen/test table to reflect actual
   conventions used in the gabriel-greeting-* examples.
6. Remove or deprecate references to `holon.yaml` (proto is the source
   of truth, yaml is legacy fallback).
7. Verify every example tree matches the actual directory structure of
   existing holons in examples/hello-world/.
```
