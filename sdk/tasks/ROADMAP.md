# Task Execution Roadmap

## Overview

16 tasks organized in 4 phases. Each phase can start once its
dependencies are met. All work happens on **macOS** except two
WinUI recipes at the very end.

---

## Phase 1 — OP Introspection (no dependencies)

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

## Phase 2 — Describe RPC (parallel with Phase 1)

SDK-side runtime introspection.

| Task | What | Depends on |
|------|------|-----------|
| **TASK004** | `HolonMeta.Describe` in `go-holons` (reference) | — |
| **TASK005** | `Describe` across all SDKs | TASK004 |

**Platform:** macOS only.
**Result:** every holon self-documents at runtime via `Describe` RPC.

---

## Phase 3 — SDK Connect (parallel with Phase 2)

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

## Phase 4 — Migrations (requires Phase 3)

Wire recipes and examples to use SDK `connect`.

| Task | What | Depends on | Windows? |
|------|------|-----------|----------|
| **TASK013** | Go-backend recipe migration | TASK007, 008, 011 | ⚠️ See below |
| **TASK014** | Rust-backend recipe migration | TASK006 + frontends | ⚠️ See below |
| **TASK015** | Hello-world example migration | All connect tasks | ❌ No |
| **TASK016** | Update TODO.md / status reports | All previous | ❌ No |

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

1. Complete TASK013/014 on macOS first — build all 8 non-WinUI recipes.
2. Then move to a Windows machine (or VM) for the two WinUI recipes.
3. Use `IMPLEMENTATION_ON_WINDOWS.md` as the prompt for that session.

The macOS builds already prove the full architecture (gRPC contracts,
composite holon assembly, transport chains). The Windows session adds
platform coverage, not architectural validation.

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
                                                  │ TASK013–014      │
                                                  │ macOS recipes    │
                                                  │   then Windows   │
                                                  │   (2 WinUI only) │
                                                  │ TASK015 examples │
                                                  │ TASK016 docs     │
                                                  └──────────────────┘
```

---

## Platform Summary

| Phase | macOS | Windows |
|-------|-------|---------|
| 1 — OP Introspection | ✅ all | ❌ not needed |
| 2 — Describe RPC | ✅ all | ❌ not needed |
| 3 — SDK Connect | ✅ all | ❌ not needed |
| 4 — Migrations | ✅ 18/20 recipes | ⚠️ 2 WinUI recipes |
