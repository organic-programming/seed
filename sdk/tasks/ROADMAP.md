# Task Execution Roadmap

20 tasks in 5 phases. Phases 1–4 run on macOS.
Phase 5 splits into macOS recipe builds and Windows WinUI builds.

## Master Checklist

### Phase 1 — OP Introspection

- [x] **TASK001** — `op inspect` (offline proto docs) ✅ Codex
- [x] **TASK002** — `op mcp` + `op tools` (MCP bridge) ✅ Codex
- [x] **TASK003** — `@required`/`@example`/`skills` enrichment ✅ Codex

### Phase 2 — Describe RPC (parallel with Phase 1)

- [x] **TASK004** — `HolonMeta.Describe` in `go-holons` (reference)  ✅ Codex
- [x] **TASK005** — `Describe` across SDK fleet ✅ Codex

### Phase 3 — SDK Connect (parallel with Phase 2)

- [x] **TASK006** — Rust `connect` ✅ already implemented
- [x] **TASK007** — Swift `connect` ✅ Codex
- [x] **TASK008** — JS-web `connect` (browser, direct dial only) ✅ Codex
- [x] **TASK009** — Ruby `connect` ✅ Codex
- [x] **TASK010** — C `connect` ✅ Codex
- [x] **TASK011** — C++ `connect` ✅ Codex
- [x] **TASK012** — Obj-C `connect` ✅ Codex

### Phase 4 — Migrations (requires Phase 3)

- [x] **TASK013** — Go-backend recipe migration ✅ Codex
- [x] **TASK014** — Rust-backend recipe migration ✅ Codex
- [x] **TASK015** — Hello-world example migration ✅ Codex
- [x] **TASK016** — Update TODO.md / status reports ✅ Codex

### Phase 5 — Recipe Builds (requires Phase 4)

#### macOS

- [x] **mac_TASK001** — Build/verify 6 Go-backend recipes ✅ Codex
- [x] **mac_TASK002** — Build/verify 6 Rust-backend recipes ✅ Codex

#### Windows

- [ ] **windows_TASK001** — Build 2 WinUI-only recipes (`go-dotnet`, `rust-dotnet`)
- [ ] **windows_TASK002** — Add Windows targets to 8 cross-platform recipes (optional)

---

## Phase Details

### Phase 1 — OP Introspection (no dependencies)

Make `op` the single gateway to holon APIs, MCP, and LLM tools.

| Task | What | Depends on |
|------|------|-----------|
| **TASK001** | `op inspect` — offline proto docs | — |
| **TASK002** | `op mcp` + `op tools` — MCP bridge | TASK001 |
| **TASK003** | `@required`/`@example`/`skills` enrichment across all holons | TASK001 |

**Platform:** macOS only.
**Result:** `op inspect rob-go` shows full API docs, `op mcp rob-go`
exposes any holon as an MCP server.

---

### Phase 2 — Describe RPC (parallel with Phase 1)

SDK-side runtime introspection.

| Task | What | Depends on |
|------|------|-----------|
| **TASK004** | `HolonMeta.Describe` in `go-holons` (reference) | — |
| **TASK005** | `Describe` across all SDKs | TASK004 |

**Platform:** macOS only.
**Result:** every holon self-documents at runtime via `Describe` RPC.

---

### Phase 3 — SDK Connect (parallel with Phase 2)

Name-based resolution in every SDK.

| Task | What | Depends on |
|------|------|-----------|
| **TASK006** | Rust `connect` | — |
| **TASK007** | Swift `connect` | — |
| **TASK008** | JS-web `connect` (browser, direct dial only) | — |
| **TASK009** | Ruby `connect` | — |
| **TASK010** | C `connect` | — |
| **TASK011** | C++ `connect` | — |
| **TASK012** | Obj-C `connect` | — |

**Platform:** macOS only.
**Note:** Tasks 006–012 are independent — submit to Codex in parallel.

---

### Phase 4 — Migrations (requires Phase 3)

Wire recipes and examples to use SDK `connect`.

| Task | What | Depends on |
|------|------|-----------|
| **TASK013** | Go-backend recipe migration | TASK007, 008, 011 |
| **TASK014** | Rust-backend recipe migration | TASK006 + frontends |
| **TASK015** | Hello-world example migration | All connect tasks |
| **TASK016** | Update TODO.md / status reports | All previous |

**Platform:** macOS only.

---

### Phase 5 — Recipe Builds (requires Phase 4)

Build and verify every recipe on its target platform.

#### macOS

| Task | What | Depends on |
|------|------|-----------|
| **mac_TASK001** | Build/verify 6 Go-backend recipes | TASK013 |
| **mac_TASK002** | Build/verify 6 Rust-backend recipes | TASK014, mac_TASK001 |

Covers: Flutter, SwiftUI, Compose, Web, Qt, .NET Mac Catalyst.

#### Windows

| Task | What | Depends on |
|------|------|-----------|
| **windows_TASK001** | Build 2 WinUI-only recipes (`go-dotnet`, `rust-dotnet`) | mac_TASK001 |
| **windows_TASK002** | Add Windows targets to 8 cross-platform recipes | windows_TASK001 |

**Only `windows_TASK001` is strictly required** — the 2 WinUI recipes
cannot build on macOS. `windows_TASK002` is optional platform coverage.

---

## When is Windows Required?

**Only for 2 recipes out of 20.**

| Recipe | Frontend | Why Windows |
|--------|----------|-------------|
| `go-dotnet-holons` | WinUI 3 | WinUI is Windows-only |
| `rust-dotnet-holons` | WinUI 3 | WinUI is Windows-only |

Everything else builds on macOS:

| Frontend | macOS build | Windows build |
|----------|------------|---------------|
| Flutter (Dart) | ✅ `flutter build macos` | optional |
| SwiftUI | ✅ `xcodebuild` | N/A |
| Compose (Kotlin) | ✅ `./gradlew createDistributable` | optional |
| Web (TypeScript) | ✅ `npm run build` | identical |
| Qt (C++) | ✅ `cmake` | optional |
| .NET MAUI (Mac Catalyst) | ✅ `dotnet build` | — |
| **.NET WinUI 3** | **❌ not possible** | **✅ required** |

### Recommended approach

1. Complete mac_TASK001/002 — build all recipes on macOS.
2. Then move to Windows for windows_TASK001 (WinUI).
3. Optionally run windows_TASK002 for cross-platform coverage.

---

## Execution Diagram

```
Phase 1 (OP)              Phase 2 (Describe)      Phase 3 (Connect)
┌──────────┐              ┌──────────┐            ┌──────────────────┐
│ TASK001   │──→ TASK002   │ TASK004  │──→ TASK005 │ TASK006–012      │
│ op inspect│   op mcp     │ go-holons│   fleet    │ (7 SDKs parallel)│
│           │──→ TASK003   └──────────┘            └────────┬─────────┘
│           │   enrichment                                  │
└──────────┘                                                ▼
                                                  Phase 4 (Migrations)
                                                  ┌──────────────────┐
                                                  │ TASK013–016      │
                                                  └────────┬─────────┘
                                                           │
                                              ┌────────────┴────────────┐
                                              ▼                         ▼
                                    Phase 5a (macOS)          Phase 5b (Windows)
                                    ┌────────────────┐        ┌─────────────────┐
                                    │ mac_TASK001    │        │ windows_TASK001  │
                                    │ Go recipes     │        │ 2 WinUI recipes  │
                                    │ mac_TASK002    │        │ windows_TASK002  │
                                    │ Rust recipes   │        │ cross-plat targets│
                                    └────────────────┘        └─────────────────┘
```

---

### Phase 6 — Quality Control (after Phase 5)

- [ ] **QA_TASK001** — End-to-end verification of all recipes and examples

Builds, launches, and verifies every recipe (12) and hello-world
example (14). Checks UI integrity, RPC round-trip, and daemon
lifecycle. Fixes any issue found during verification.

See [QA_TASK001.md](./QA_TASK001.md) for the full checklist.

---

## Platform Summary

| Phase | macOS | Windows |
|-------|-------|---------|
| 1 — OP Introspection | ✅ all | ❌ not needed |
| 2 — Describe RPC | ✅ all | ❌ not needed |
| 3 — SDK Connect | ✅ all | ❌ not needed |
| 4 — Migrations | ✅ all | ❌ not needed |
| 5a — Recipe builds | ✅ mac_TASK001, mac_TASK002 | — |
| 5b — Recipe builds | — | ⚠️ windows_TASK001 (required), windows_TASK002 (optional) |
| 6 — QA | ✅ QA_TASK001 | ❌ not needed |

