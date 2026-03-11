# OP v0.4 Design Tasks — Recipe Ecosystem

> [!CAUTION]
> **The current implementation is globally correct — do NOT rewrite it.**
> Some samples may be stuck or incomplete, but the architecture and
> patterns are sound. This work is a **DRY generalization and extension**
> of working code. Extract, factor, and extend — do not redesign,
> refactor for style, or "improve" what already works. Fix what is
> stuck, preserve what works.

> [!IMPORTANT]
> **Always use the language SDK.** Every daemon and HostUI must use
> its language's Organic Programming SDK for server bootstrap,
> `connect(slug)`, and transport negotiation. All SDKs exist and are
> tested — no raw gRPC fallback.

> [!IMPORTANT]
> **Connect approach only.** Every UI assembly must use SDK
> `connect(slug)` for daemon resolution — no raw `GrpcChannel`
> or hardcoded addresses.

## Execution Strategy

Strictly linear — each task gates the next. TASK04 is the PoC
milestone; TASK04b is the 3×3 cross-language validation gate.
Nothing scales to 48 assemblies until TASK04b passes.

## Tasks

| # | File | Summary | Depends on | Status |
|---|---|---|---|---|
| | | **— Shared Proto —** | | |
| 01 | [TASK01](./grace-op_v0.4_TASK01_shared_proto.md) | Shared `greeting.proto` | — | — |
| | | **— PoC: Go + Dart —** | | |
| 02 | [TASK02](./grace-op_v0.4_TASK02_dry_daemon_go.md) | Extract Go daemon | TASK01 | — |
| 03 | [TASK03](./grace-op_v0.4_TASK03_dry_hostui_flutter.md) | Extract Flutter/Dart HostUI | TASK02 | — |
| 04 | [TASK04](./grace-op_v0.4_TASK04_validate_go_dart_poc.md) | **★ Validate Go+Dart assembly (MILESTONE)** | TASK03 | — |
| | | **— Remaining Daemons —** | | |
| 05 | [TASK05](./grace-op_v0.4_TASK05_dry_daemon_rust.md) | Extract Rust daemon | TASK04 | — |
| 06 | [TASK06](./grace-op_v0.4_TASK06_dry_daemons_swift_kotlin_dart.md) | Extract Swift/Kotlin, create Dart daemon | TASK05 | — |
| 07 | [TASK07](./grace-op_v0.4_TASK07_dry_daemons_python_csharp_node.md) | Extract C#, create Python/Node daemons | TASK06 | — |
| | | **— Remaining HostUIs —** | | |
| 08 | [TASK08](./grace-op_v0.4_TASK08_dry_hostui_swiftui.md) | Extract SwiftUI HostUI | TASK07 | — |
| 09 | [TASK09](./grace-op_v0.4_TASK09_dry_hostui_kotlin_web_dotnet_qt.md) | Extract Kotlin, Web, .NET, Qt HostUIs | TASK08 | — |
| | | **— Cross-Language Validation —** | | |
| 10 | [TASK10](./grace-op_v0.4_TASK10_cross_language_validation.md) | **★ 3×3 cross-language validation (MILESTONE)** | TASK09 | — |
| | | **— Assembly & Cleanup —** | | |
| 11 | [TASK11](./grace-op_v0.4_TASK11_assembly_manifests.md) | Create 48 assembly manifests | TASK10 | — |
| 12 | [TASK12](./grace-op_v0.4_TASK12_remove_submodules.md) | Remove 12 submodules, archive repos | TASK11 (parallel, not blocking) | — |
| 13 | [TASK13](./grace-op_v0.4_TASK13_composition_recipes.md) | 3 patterns × 11 orchestrator languages | TASK11 | — |
| 14 | [TASK14](./grace-op_v0.4_TASK14_testmatrix.md) | Combinatorial testing (Go testmatrix) | TASK13 | — |

## Design Documents

- [DESIGN_recipe_ecosystem.md](./DESIGN_recipe_ecosystem.md) — architecture, patterns, rationale
- [DESIGN_recipe_monorepo.md](./DESIGN_recipe_monorepo.md) — proto contracts, assembly matrix

## Dependency Graph

```
TASK01 → TASK02 → TASK03 → TASK04 ★ → TASK05 → TASK06 → TASK07
→ TASK08 → TASK09 → TASK10 ★ → TASK11 → TASK13 → TASK14
                                        └─→ TASK12 (parallel cleanup)
```
