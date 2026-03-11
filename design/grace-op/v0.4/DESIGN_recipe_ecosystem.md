# Recipe Ecosystem

## Problem Statement

The current recipe ecosystem is a **flat 2×6 matrix** focused exclusively on UI assembly (backend daemon + frontend client). This model successfully demonstrates transport bridges across platforms but leaves a critical gap: **there is no reference material for backend-to-backend holon composition**, which is the primary pattern in distributed execution.

With the upcoming infrastructure holons (Line/git, Al/brew, Marvin/winget, Phill/filesystem), developers need concrete examples of how holons call, chain, and coordinate with each other — regardless of implementation language.

## Current State

```
recipes/                          ← flat, UI-only
├── README.md
├── IMPLEMENTATION_ON_MAC_OS.md
├── IMPLEMENTATION_ON_WINDOWS.md
├── go-dart-holons/
├── go-swift-holons/
├── go-kotlin-holons/
├── go-web-holons/
├── go-dotnet-holons/
├── go-qt-holons/
├── rust-dart-holons/
├── rust-swift-holons/
├── rust-kotlin-holons/
├── rust-web-holons/
├── rust-dotnet-holons/
└── rust-qt-holons/
```

- 12 submodule repos, each containing a Gudule Greeting implementation
- The Gudule Greeting Pattern defines daemon-client separation, transport strategies, and lifecycle management
- Daemon sources are copied (not shared via submodule) for independence
- The `grace-op` recipe runner handles composite builds

---

## Target State

```
recipes/
├── README.md
├── protos/                                          ← shared proto (TASK01)
│   └── greeting/v1/
│       └── greeting.proto                           ← canonical GreetingService
│
├── daemons/                                         ← DRY daemons (TASK02, TASK05–07)
│   ├── gudule-daemon-greeting-go/
│   ├── gudule-daemon-greeting-rust/
│   ├── gudule-daemon-greeting-swift/
│   ├── gudule-daemon-greeting-kotlin/
│   ├── gudule-daemon-greeting-dart/
│   ├── gudule-daemon-greeting-python/
│   ├── gudule-daemon-greeting-csharp/
│   └── gudule-daemon-greeting-node/
│
├── hostui/                                          ← DRY HostUIs (TASK03, TASK08–09)
│   ├── gudule-greeting-hostui-flutter/
│   ├── gudule-greeting-hostui-swiftui/
│   ├── gudule-greeting-hostui-compose/              ← Kotlin Compose
│   ├── gudule-greeting-hostui-web/
│   ├── gudule-greeting-hostui-dotnet/
│   └── gudule-greeting-hostui-qt/
│
├── assemblies/                                      ← 48 thin manifests (TASK10)
│   ├── gudule-greeting-flutter-go/                  ← Flutter connects to Go
│   ├── gudule-greeting-flutter-rust/
│   ├── gudule-greeting-swiftui-go/
│   ├── gudule-greeting-go-web/                      ← reversed: Go serves web
│   ├── ...                                          ← (see DESIGN_recipe_monorepo.md for full 48)
│   └── gudule-greeting-qt-node/
│
├── composition/                                     ← backend-to-backend (TASK13)
│   ├── README.md
│   ├── workers/
│   │   ├── charon-worker-compute/
│   │   └── charon-worker-transform/
│   ├── direct-call/
│   │   ├── charon-direct-go-go/
│   │   ├── charon-direct-rust-go/
│   │   ├── charon-direct-swift-go/
│   │   └── ...                                      ← (11 languages, see monorepo doc)
│   ├── pipeline/
│   │   ├── charon-pipeline-go-go/
│   │   └── ...
│   └── fan-out/
│       ├── charon-fanout-go-go/
│       └── ...
│
├── testmatrix/                                      ← combinatorial testing (TASK12)
│   └── gudule-greeting-testmatrix/
│
├── IMPLEMENTATION_ON_MAC_OS.md
└── IMPLEMENTATION_ON_WINDOWS.md
```

> [!IMPORTANT]
> Composition recipes are organized by **pattern** in the directory
> tree. Each holon **name** encodes both caller and callee
> (`charon-direct-rust-go` = Rust orchestrator → Go workers) for
> expressiveness and forward-compatibility. Today the callee is
> always Go, but the naming supports future worker languages.

---

## Design Principles

### 1. "Recipe" Is the Universal Concept

> A **recipe** is a reference implementation showing how to compose holons in a specific pattern.

- **UI recipes** demonstrate how to bridge a gRPC backend to a platform-native UI via transport adaptation (stdio, unix sockets, named pipes, Connect).
- **Composition recipes** demonstrate how holons coordinate with each other as backends, using standard gRPC. No transport bridge needed.

### 2. Composition Recipes Are Pattern-First, Not Language-Pair

UI recipes need a full `backend × frontend` matrix because the **transport bridge is language-specific**. Composition recipes don't — gRPC is language-agnostic. A Go holon calling a Rust holon is just a gRPC call; the caller doesn't know or care about the callee's implementation language.

Therefore, composition recipes are organized by **pattern × caller language**:

```
composition/<pattern>/<caller-language>/
```

Only the **caller's language** matters, because that's where the composition logic (discover, connect, orchestrate, handle errors) lives. The callee can be written in anything.

### 3. Gudule Stays for UI, a New Canonical Scenario for Composition

The Gudule Greeting Pattern remains the reference for UI recipes. Composition recipes need their own canonical scenario. Proposed:

**"Computation Relay"** — A minimal scenario where:
- **Holon A** (the orchestrator) receives a request
- **Holon B** (the worker) performs a computation
- **Holon A** returns Holon B's result to the caller

This is the simplest meaningful composition. Each pattern extends it:
- **Direct Call**: A calls B once
- **Pipeline**: A calls B, then calls C with B's output
- **Fan-out**: A calls B and C in parallel, aggregates results
- **Sidecar**: B always runs alongside A, providing a utility
- **Proxy**: A decides at runtime which backend to forward to

---

## Composition Patterns Catalog

### Pattern 1: Direct Call

The simplest composition. One holon discovers another and makes a single RPC.

| Aspect | Detail |
|---|---|
| **Topology** | A → B |
| **Teaches** | `discover` + `connect`, basic error handling |
| **Proto** | Caller imports callee's service definition |
| **Real-world use** | A build holon asking Rob to compile |

### Pattern 2: Pipeline

Sequential chain where each step's output feeds the next step's input.

| Aspect | Detail |
|---|---|
| **Topology** | A → B → C (orchestrated by A) |
| **Teaches** | Multi-service coordination, error propagation through chain, partial failure handling |
| **Proto** | Orchestrator imports all service definitions |
| **Real-world use** | Line (git clone) → Rob (go build) → Phill (store artifact) |

### Pattern 3: Fan-Out

Parallel dispatch to multiple holons with result aggregation.

| Aspect | Detail |
|---|---|
| **Topology** | A → {B, C, D} in parallel |
| **Teaches** | Goroutines / async dispatch, partial success, timeout strategies, result merging |
| **Proto** | Orchestrator imports all service definitions |
| **Real-world use** | Install deps (Al) + clone code (Line) + prepare workspace (Phill) simultaneously |

### Pattern 4: Sidecar

Two holons that always run together; one provides utility to the other.

| Aspect | Detail |
|---|---|
| **Topology** | A ↔ B (co-deployed, tightly coupled lifecycle) |
| **Teaches** | Co-deployment, lifecycle coupling, health interdependency |
| **Proto** | Each imports the other's contract (bidirectional) |
| **Real-world use** | Any holon + Phill sidecar for workspace access |

### Pattern 5: Proxy / Router

One holon acts as a facade, routing requests to the appropriate backend.

| Aspect | Detail |
|---|---|
| **Topology** | Client → Proxy → {B or C or D} |
| **Teaches** | Runtime dispatch, OS/platform detection, configuration-driven routing |
| **Proto** | Proxy implements its own unified contract; delegates to backend-specific contracts |
| **Real-world use** | A "PackageManager" holon routing to Al (macOS), Marvin (Windows), or apt-holon (Linux) based on host OS |

---

## Priority for v1

Start with the three most immediately useful patterns:

1. **Direct Call** — the "hello world" of composition, simplest to implement
2. **Pipeline** — immediately needed for multi-holon build chains
3. **Fan-out** — essential for parallel operations in distributed builds

Defer to v2:
4. **Sidecar** — useful but requires lifecycle management conventions
5. **Proxy** — useful but requires the OS detection and routing logic

---

## Implementation Steps

See [_TASKS.md](./_TASKS.md) for the detailed task breakdown (TASK01–TASK13).

### Phase 1: DRY Extraction (TASK01–TASK09)

1. Create shared `recipes/protos/greeting/v1/greeting.proto` (TASK01)
2. Extract Go daemon into `recipes/daemons/gudule-daemon-greeting-go/` (TASK02)
3. Extract Flutter HostUI into `recipes/hostui/gudule-greeting-hostui-flutter/` (TASK03)
4. Create assembly `recipes/assemblies/gudule-greeting-flutter-go/` and validate (TASK04 — PoC milestone)
5. Extract remaining 7 daemons (TASK05–07)
6. Extract remaining 5 HostUIs (TASK08–09)

### Phase 2: Assembly & Cleanup (TASK10–TASK11)

1. Create all 48 assembly manifests in `recipes/assemblies/` (TASK10)
2. Remove the 12 old submodule repos and archive them (TASK11)

### Phase 3: Composition & Testing (TASK12–TASK13)

1. Implement `charon-worker-compute` and `charon-worker-transform` (Go workers)
2. Implement `charon-{direct,pipeline,fanout}-<lang>-go` for all 11 languages (TASK12)
3. Build `gudule-greeting-testmatrix` for combinatorial testing (TASK13)
4. Update CONVENTIONS.md / SDK_GUIDE.md to reference both recipe categories

---

## Composition Recipe Internal Structure

Each composition recipe follows a consistent layout:

```
recipes/composition/direct-call/charon-direct-rust-go/
├── holon.yaml                  ← composite holon manifest
├── README.md                   ← pattern explanation + how to run
├── orchestrator/               ← the Rust caller holon
│   ├── holon.yaml
│   ├── protos/
│   ├── gen/
│   ├── src/
│   └── Cargo.toml
└── Makefile                    ← builds + runs the full composition
```

Workers live in `recipes/composition/workers/` and are shared across
all orchestrators — they are not embedded in each recipe.

Each recipe is self-contained for orchestrator logic. The worker holons are intentionally simple — the focus is on the **orchestrator's composition logic**, not the worker's domain logic.

---

## Repo Strategy

All recipes live in the monorepo (`organic-programming/seed`). The 12
old submodule repos will be archived after TASK11. No separate repos
for composition recipes — workers are shared, orchestrators are small.

---

## Monorepo Evolution (from DRY analysis)

The original 2×6 UI matrix (Go/Rust × 6 UIs) expands into a
monorepo with shared components:

- **8 daemon languages:** Go, Rust, Python, Swift, Kotlin, Dart, C#, Node.js
- **6 HostUI technologies:** Flutter, SwiftUI, Compose, Web, Dotnet, Qt
- **48 assemblies:** 8 × 6 thin `holon.yaml` manifests (no source)
- **11 composition orchestrators:** Go through C++ (all SDKs with `connect(slug)`)
- **12 submodule repos archived** and replaced by monorepo

## Subtasks

See [_TASKS.md](./_TASKS.md) for the full decomposed task list with
dependency graph. Summary:

| Phase | Tasks | Summary |
|---|---|---|
| **Shared proto** | TASK01 | Single `greeting.proto` |
| **PoC (Go+Dart)** | TASK02, TASK03, TASK04 | Extract Go daemon + Flutter HostUI, validate assembly |
| **Remaining daemons** | TASK05, TASK06, TASK07 | Rust, Swift/Kotlin/Dart, Python/C#/Node |
| **Remaining HostUIs** | TASK08, TASK09 | SwiftUI, Kotlin/Web/.NET/Qt |
| **Assembly & cleanup** | TASK10, TASK11 | 48 manifests, remove submodules |
| **Composition** | TASK12 | 3 patterns × 11 languages |
| **Testing** | TASK13 | Combinatorial test matrix |
