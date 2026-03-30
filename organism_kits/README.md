# Organic Programming — Organism Kits Design Document

> **Status:** Discussion draft
>
> **Prerequisite reading:**
> - [PACKAGE.md](../PACKAGE.md) — The `.holon` package format and Bundle Integration
> - [HOLON_BUILD.md](../holons/grace-op/HOLON_BUILD.md) — `op build` and the recipe runner
> - [COAX.md](../holons/grace-op/COAX.md) — COAX principle and Organism definition
> - [CONVENTIONS.md](../holons/grace-op/CONVENTIONS.md) — Per-language structure (⚠️ outdated)

---

## What Is an Organism Kit?

An OP application is **an organism**: a living composite with an interface
(the UI) and internal organs (holons). An Organism Kit is the plumbing that
connects them — not a widget toolkit, not a UI framework, but the
organism's nervous system.

The plural **Organism Kits** reflects that we provide one kit per UI
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
│   │      Organism Kit            │       │
│   │  catalog · connect ·         │       │  COAX + MCP + SERVICES
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
│   │  bundle, OPBIN, or remote    │       │
│   └──────────────────────────────┘       │
└──────────────────────────────────────────┘
```

---

## Two Modes of Operation

| Mode | Organ source | Who manages discovery | `op` required? |
|------|-------------|----------------------|----------------|
| **Development** | Source-tree holons (sibling dirs), `op` discovers and runs them | `op` | Yes — installed via the seed repo or separately |
| **Published app** | `.holon` packages embedded in bundle or installed in `$OPBIN` | Organism Kit | Generally no — but external holon dependencies are acceptable |

During development, `op` is always available (typically from the **seed
repository**, which provides `op` and everything needed for agents and
humans to build OP systems, though `op` can also be installed separately).
The Organism Kit delegates discovery to `op` in dev mode.

In a published app, the main difference is that we **plant binary holons,
not source code**. Most published apps are self-contained (holons embedded
in the bundle), but external holon dependencies installed in `$OPBIN` are
valid — an organism can have both embedded and external organs.

---

## How This Articulates with Existing Specs

| Document | What it defines | Organism Kit's relationship |
|----------|----------------|----------------------|
| **COAX.md** | COAX principle and Organism definition | Organism Kit **embodies** this — it is the plumbing that makes an organism coaccessible |
| **HOLON_PACKAGE.md** | `.holon` package, `.holon.json`, Bundle Integration (`Contents/Resources/Holons/`), discovery order, execution command | Organism Kit **implements** discovery and execution per this spec |
| **HOLON_BUILD.md** | `op build`, recipe runner, `build_member` + `copy_artifact`, `.holon` package output | Organism Kit **consumes** what `op build` produces |
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

## Organism Kit Primitives

### 1. `HolonCatalog` — What Organs Are Available?

**Today:** Every host hard-codes descriptors with variant names, UUIDs,
build runners, binary paths, and sort ranks.

**Organism Kit should:** Discover available holons automatically.

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

**Organism Kit should:** Provide one call.

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
> handle start for all transports, not just stdio. The Organism Kit calls
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

The Organism Kit ships the **model only**. UI is fully in the app's hands.
The Gabriel greeting apps serve as examples.

---

### 4. Organism Lifecycle

The Organism Kit hooks into the framework's lifecycle automatically:
- On exit: disconnect + stop organ processes + cleanup.
- The app **never writes cleanup code**.

---

## COAX — Coaccessibility

> See [COAX.md](../holons/grace-op/COAX.md) for the full definition of COAX and Organism.

**COAX** stands for **coaccessibility**: every component in an OP
application must be equally accessible to humans *and* machines.
This is not an afterthought or an accessibility layer bolted on top —
it is a foundational design principle that shapes how Organism Kits are
built and how apps built on them behave.

Traditional frameworks treat human (UI) accessibility and machine
(API) accessibility as separate concerns.  COAX rejects that split:
a component that a human can discover, inspect, and operate should be
just as discoverable, inspectable, and operable by an agent, a script,
or another holon — through the same structural contracts.

See [CONSTITUTION.md Article 1](../CONSTITUTION.md#article-1--the-holon) for the
constitutional definition of COAX.

### Why Holons Are the Natural Foundation

Holons already carry the properties COAX requires:

| Property | Human side | Machine side |
|----------|-----------|-------------|
| **Identity** | Readable name, description in `.holon.json` | Slug, UUID, proto manifest — programmatic lookup |
| **Contract** | Proto definitions document the API a developer reads | Same protos generate stubs, enabling code-level introspection |
| **Discovery** | `HolonCatalog` lets a user browse available members | Same catalog is queryable by agents and tooling |
| **Lifecycle** | Organism Kit manages start/stop transparently for the user | Same lifecycle hooks are scriptable via `op` or SDK calls |

Because every holon is self-describing (manifest, proto contract,
typed RPC surface), any tool — IDE, AI agent, test harness,
orchestrator — can reason about the organism's structure without
special adaptation.

### The Organism's COAX Interaction Surface

A composite app holon is an **organism** — the living whole that
assembles its member holons.  Today the organism has a UI for humans
but no programmatic entry point.  The COAX interaction surface fixes
this: a shared proto service
([`coax.proto`](../holons/grace-op/protos/holons/v1/coax.proto)) that any organism
registers alongside its domain service.

The interaction surface generalizes what AppleScript achieved for
macOS apps — but universal (proto-based, any platform), typed
(protobuf messages, not string dictionaries), self-describing
(`Describe` exposes it), and recursive (an organism within an
organism inherits the same surface).

**Two layers compose the surface:**

1. **Shared COAX service** (`holons.v1.CoaxService`) — member
   discovery, lifecycle, and `Tell`.  Every organism registers this.
   It maps directly to the Organism Kit primitives:

   | Organism Kit primitive | COAX RPC |
   |-------------------|----------|
   | `HolonCatalog.discover()` | `ListMembers` |
   | `HolonConnector.connect()` | `ConnectMember` |
   | `HolonConnector.close()` | `DisconnectMember` |
   | `HolonPickerModel.status` | `MemberStatus` |
   | *(new)* | `Tell` — forward a command to a member by slug |

2. **App-specific domain verbs** — each organism defines its own
   service with the operations that make sense for *its* domain.
   For Gabriel Greeting App: `SelectHolon`, `SelectLanguage`, `Greet`.
   These are the actions a human performs through the UI, expressed as
   RPCs that an agent can call equivalently.

### Co-Interaction: Shared State, Visible Actions

COAX does not bypass the UI — it **drives through it**.

When an agent calls `Greet(name: "Marie")`, the name must appear in
the text field.  When it calls `SelectHolon(slug: "greeting-go")`, the
dropdown must update.  Both human and agent observe and mutate the same
organism state — the `HolonPickerModel`, the domain model, the
connection status — through the same reactive bindings.

This is the fundamental difference from a headless API: COAX
interactions are *visible*.  A human watching the screen sees exactly
what the agent is doing, in real time.

> [!IMPORTANT]
> COAX is not optional polish, but it is also not a rigid diktat.
> While core capabilities must satisfy both sides of the coaccessibility
> contract, Organism Kits recognise that some interactions may legitimately be
> reserved for one class of actant. For example, a purely visual layout
> adjustment (human-only) or a low-level diagnostic hook (machine-only).
> COAX ensures the organism's functional domain is shared, without
> demanding dogmatic symmetry where it provides no value.

### COAX Server Must Register `Describe`

The COAX gRPC server **must** register the `HolonMeta/Describe`
service alongside `CoaxService` and any app-specific domain services.
This is the same requirement as any holon — the organism's COAX
server is no exception.

**Why:** without `Describe`, a client connecting to the COAX server
has no way to discover what services and methods are available.  The
Dynamic Dispatch Workflow
([COMMUNICATION.md §3.6](../COMMUNICATION.md))
requires `Describe` as the schema source for building protobuf from
JSON dynamically.  This is how `op`, agents, and any SDK client
interact with the organism programmatically — without compiled stubs
and without gRPC reflection.

The `Describe` response from a COAX server includes **all** services
registered on that server:

```
DescribeResponse:
  slug: "gabriel-greeting-app-swiftui"
  motto: "A multilingual greeting app for macOS."
  services:
    - holons.v1.CoaxService          ← shared COAX surface
    - greeting_app.v1.GreetingAppService  ← app-specific domain verbs
```

This gives a caller a complete picture of the organism's interaction
surface in one round-trip — both the generic member management
(`ListMembers`, `Tell`) and the domain-specific actions
(`SelectHolon`, `SelectLanguage`, `Greet`).

### COAX + MCP: Agent-Native Access

`op mcp <slug>` already exposes any holon's RPCs as
[MCP](https://modelcontextprotocol.io) tools.  The COAX surface
extends this to composite apps: an AI agent can discover, connect to,
and operate an organism's members — all through standard MCP tool
calls.

**How it works:**

```
Agent (Claude, Copilot, …)
  │
  └─ MCP client ──── stdio ────→ op mcp tcp://<slug>:<port>
                                      │
                                      ├─ 1. Call Describe on organism
                                      │     → gets CoaxService + GreetingAppService
                                      │
                                      ├─ 2. Expose as MCP tools:
                                      │     • gabriel-greeting-app.CoaxService.ListMembers
                                      │     • gabriel-greeting-app.CoaxService.Tell
                                      │     • gabriel-greeting-app.GreetingAppService.Greet
                                      │     • gabriel-greeting-app.GreetingAppService.SelectHolon
                                      │     • ...
                                      │
                                      └─ 3. Bridge: tool_call → Dynamic Dispatch → gRPC
```

The key difference from `op mcp <slug>` on a regular holon: for COAX,
`op` connects to an **already running** server at a given address.
For a regular holon, `op` starts the holon first, then connects.
In both cases, the schema source is `Describe` — never local
`.proto` files.

**Two modes of operation:**

| Mode | Command | Schema source |
|------|---------|---------------|
| **Regular holon** | `op mcp gabriel-greeting-go` | `Describe` (op starts the holon, then calls Describe) |
| **COAX (running organism)** | `op mcp tcp://<slug>:<port>` | `Describe` (op connects to the running organism) |

Both produce the same MCP tool surface — JSON Schema from proto
fields, descriptions from proto comments, `@required` / `@example`
propagated to tool parameter schemas.

**Agent workflow example:**

```
Agent: tools/list
  → [ListMembers, ConnectMember, Tell, SelectHolon, Greet, ...]

Agent: tools/call ListMembers {}
  → [{slug: "gabriel-greeting-go", state: "CONNECTED"},
     {slug: "gabriel-greeting-rust", state: "AVAILABLE"}, ...]

Agent: tools/call SelectHolon {slug: "gabriel-greeting-go"}
  → (dropdown updates in the UI)

Agent: tools/call Greet {name: "Marie", lang_code: "fr"}
  → {greeting: "Bonjour, Marie !"} (text field updates in the UI)
```

The agent operates the app exactly as a human would — every action
visible in the UI — but through MCP tool calls.

> [!NOTE]
> An organism may also embed its own MCP server natively, without
> relying on `op` as a bridge.  The Organism Kit can provide a built-in
> MCP transport that maps COAX RPCs to MCP tools directly, giving
> the app a self-contained agent interface with no external dependencies.

---

## What the App Still Owns

After the Organism Kit absorbs organism plumbing, the app is **pure domain + interface:**

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

Organism Kit provides:                ← THIS DOCUMENT
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
| Widget opinions | **Model only.** The Organism Kit ships `HolonPickerModel` and lifecycle hooks. UI is fully the app's responsibility. Gabriel apps serve as examples. |
| Naming | **"Holon" only.** The word "daemon" does not exist in OP vocabulary. All APIs, types, logs, and docs use "holon". |
| Dev-mode discovery | **Use `op`.** The seed repo always provides `op`. Published apps don't need `op` — they read `.holon.json` from the bundle or `$OPBIN`. |

---

## Appendix: CONVENTIONS.md Alignment Prompt

`CONVENTIONS.md` needs updating to articulate with Organism Kits and the
proto-first model. Here is a prompt for a separate task:

```
Update CONVENTIONS.md to align with the current Organic Programming model:

1. Replace `protos/` with `api/v1/` as the canonical proto location.
2. Verify `holon.yaml` references are replaced with `api/v1/holon.proto` carrying
   `option (holons.v1.manifest)` (done — proto is the source of truth).
3. Add a "Facet Model" section documenting the 4 facets:
   Code API (api/public.*), CLI (api/cli.*), RPC (_internal/server.*),
   Test (*_test.*).
4. Add an "App Holon Structure" section describing how a composite/app
   holon is structured (recipe, members, Organism Kit usage).
5. Update the per-language source/gen/test table to reflect actual
   conventions used in the gabriel-greeting-* examples.
6. Confirm no remaining references to `holon.yaml` (proto is the source
   of truth; yaml has been removed).
7. Verify every example tree matches the actual directory structure of
   existing holons in examples/hello-world/.
```
