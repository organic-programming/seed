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
├── README.md                         ← updated: explains the two categories
├── ui/                               ← renamed from root; the existing 2×6 matrix
│   ├── README.md                     ← current recipes/README.md, adapted
│   ├── IMPLEMENTATION_ON_MAC_OS.md
│   ├── IMPLEMENTATION_ON_WINDOWS.md
│   ├── go-dart-holons/
│   ├── go-swift-holons/
│   ├── go-kotlin-holons/
│   ├── go-web-holons/
│   ├── go-dotnet-holons/
│   ├── go-qt-holons/
│   ├── rust-dart-holons/
│   ├── rust-swift-holons/
│   ├── rust-kotlin-holons/
│   ├── rust-web-holons/
│   ├── rust-dotnet-holons/
│   └── rust-qt-holons/
└── composition/                      ← NEW: pattern × caller-language
    ├── README.md                     ← pattern catalog & guide
    ├── direct-call/
    │   ├── go/                       ← how to do a direct call FROM Go
    │   └── rust/                     ← same pattern FROM Rust
    ├── pipeline/
    │   ├── go/
    │   └── rust/
    ├── fan-out/
    │   ├── go/
    │   └── rust/
    ├── sidecar/                      ← v2
    │   ├── go/
    │   └── rust/
    └── proxy/                        ← v2
        ├── go/
        └── rust/
```

> [!IMPORTANT]
> Composition recipes are organized by **pattern × caller language**, not by language pair. Only the caller's language matters — the callee can be written in anything (gRPC is language-agnostic). A developer building a go-rust-dart fan-out just needs the `fan-out/go/` recipe; the callee languages are irrelevant.

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

### Phase 1: Restructure Directory

1. Create `recipes/ui/` directory
2. Move all 12 existing submodule repos into `recipes/ui/`
3. Move [IMPLEMENTATION_ON_MAC_OS.md](file:///Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/recipes/IMPLEMENTATION_ON_MAC_OS.md) and [IMPLEMENTATION_ON_WINDOWS.md](file:///Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/recipes/IMPLEMENTATION_ON_WINDOWS.md) into `recipes/ui/`
4. Move current [README.md](file:///Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/recipes/README.md) into `recipes/ui/README.md` (adapt content)
5. Write new top-level [recipes/README.md](file:///Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/recipes/README.md) explaining both categories
6. Update all references in external documentation (CONVENTIONS.md, SDK_GUIDE.md, etc.)

> [!WARNING]
> Moving submodules requires updating `.gitmodules` paths. Each of the 12 submodule entries must be updated to reflect the new `ui/` prefix. This is a non-trivial git operation — test on a branch first.

### Phase 2: Create Composition Recipes

For each v1 pattern (direct-call, pipeline, fan-out):

1. **Define the proto contract** — keep it minimal, focused on demonstrating the pattern
2. **Implement the Go caller** — using `go-holons` SDK's `discover` + `connect`
3. **Implement the Rust caller** — using `rust-holons` SDK's equivalent
4. **Implement worker holons** — at least one Go and one Rust worker (shared across patterns)
5. **Write a [README.md](file:///Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/recipes/README.md) per pattern** — explaining the pattern, when to use it, and how to run the example
6. **Write a `composition/README.md`** — the pattern catalog with links to each
7. **Apply the same `BLOCKED.md` failure policy** as UI recipes (3 attempt rule)

### Phase 3: Update Documentation

1. Update `CONVENTIONS.md` to reference both recipe categories
2. Update `SDK_GUIDE.md` to explain `discover` + `connect` in the context of composition recipes
3. Update KI artifacts for the recipe ecosystem

---

## Composition Recipe Internal Structure

Each composition recipe follows a consistent layout:

```
composition/<pattern>/<caller-lang>/
├── holon.yaml                  ← composite holon manifest
├── README.md                   ← pattern explanation + how to run
├── orchestrator/               ← the caller holon
│   ├── holon.yaml
│   ├── protos/
│   ├── gen/
│   ├── cmd/
│   └── internal/ (or src/)
├── worker/                     ← the callee holon(s)
│   ├── holon.yaml
│   ├── protos/
│   ├── gen/
│   ├── cmd/
│   └── internal/ (or src/)
└── Makefile                    ← builds + runs the full composition
```

Each recipe is self-contained (following the daemon copy independence rule). The worker holons are intentionally simple — the focus is on the **orchestrator's composition logic**, not the worker's domain logic.

---

## Repo Strategy

> [!IMPORTANT]
> Unlike UI recipes which need 12 separate repos (one per combination), composition recipes can use **fewer repos** since they are pattern-focused.

Two options:

**Option A: One repo per pattern** (6 repos total for v1+v2)
- `organic-programming/recipe-direct-call`
- `organic-programming/recipe-pipeline`
- `organic-programming/recipe-fan-out`
- Pro: mirrors the UI recipe approach (submodule per repo)
- Con: many small repos

**Option B: Single composition repo** (1 repo)
- `organic-programming/composition-recipes`
- All patterns in subdirectories
- Pro: simpler management, patterns share worker code
- Con: breaks the submodule-per-repo convention

**Recommendation**: Option A for consistency with the UI recipe model. Each pattern is different enough to warrant its own repo, and the submodule convention is well established.

---

## Open Questions

1. **Canonical scenario naming** — is "Computation Relay" a good name, or should it follow the Gudule pattern and have a holon identity (e.g., a holon named "Relay" or "Echo")?
2. **Worker language** — should the default worker always be in the *other* language (Go orchestrator + Rust worker) to demonstrate cross-language, or same language for simplicity?

---

## Monorepo Evolution (from DRY analysis)

The original 2×6 UI matrix (Go/Rust × 6 UIs) expands into a
monorepo with shared components:

- **8 daemon languages:** Go, Rust, Python, Swift, Kotlin, Dart, C#, Node.js
- **6 HostUI technologies:** SwiftUI, Flutter, Kotlin, Web, .NET, Qt
- **48 assemblies:** 8 × 6 thin `holon.yaml` manifests (no source)
- **11 composition orchestrators:** Go through C++ (all SDKs with `connect(slug)`)
- **12 submodule repos archived** and replaced by monorepo

## Subtasks

| # | File | Summary | Depends on |
|---|---|---|---|
| 01 | [TASK01](./grace-op_v0.4_TASK01_dry_daemons.md) | Extract 8 DRY daemons | — |
| 02 | [TASK02](./grace-op_v0.4_TASK02_dry_hostui.md) | Extract 6 DRY HostUIs | — |
| 03 | [TASK03](./grace-op_v0.4_TASK03_assembly_manifests.md) | Create 48 assembly manifests | 01, 01.02 |
| 04 | [TASK04](./grace-op_v0.4_TASK04_remove_submodules.md) | Remove 12 submodules | 03 |
| 05 | [TASK05](./grace-op_v0.4_TASK05_testmatrix.md) | Combinatorial testing (Go program) | 03, 01.06 |
| 06 | [TASK06](./grace-op_v0.4_TASK06_composition_recipes.md) | 3 patterns × 11 languages | — |
