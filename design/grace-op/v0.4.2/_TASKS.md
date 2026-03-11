# OP v0.4.2 Design Tasks — Matrix Extraction

> [!CAUTION]
> **The current implementation is globally correct — do NOT rewrite it.**
> Extract, factor, and extend — do not redesign.

> [!IMPORTANT]
> **Always use the language SDK.** Every daemon and HostUI must use
> its language's Organic Programming SDK for server bootstrap,
> `connect(slug)`, and transport negotiation.

## Execution Strategy

Strictly linear — each task gates the next. TASK10 is the 3x3 cross-language validation milestone. Do not proceed to v0.4.3 until TASK10 passes.

## Tasks

| # | File | Summary | Depends on |
|---|---|---|---|
| | | **— Remaining Daemons —** | |
| 05 | [TASK05](./grace-op_v0.4_TASK05_dry_daemon_rust.md) | Extract Rust daemon | TASK04 (from v0.4.1) |
| 06 | [TASK06](./grace-op_v0.4_TASK06_dry_daemons_swift_kotlin_dart.md) | Extract Swift/Kotlin, create Dart daemon | TASK05 |
| 07 | [TASK07](./grace-op_v0.4_TASK07_dry_daemons_python_csharp_node.md) | Extract C#, create Python/Node daemons | TASK06 |
| | | **— Remaining HostUIs —** | |
| 08 | [TASK08](./grace-op_v0.4_TASK08_dry_hostui_swiftui.md) | Extract SwiftUI HostUI | TASK07 |
| 09 | [TASK09](./grace-op_v0.4_TASK09_dry_hostui_kotlin_web_dotnet_qt.md) | Extract Kotlin, Web, .NET, Qt HostUIs | TASK08 |
| | | **— Cross-Language Validation —** | |
| 10 | [TASK10](./grace-op_v0.4_TASK10_cross_language_validation.md) | **★ 3×3 cross-language validation (MILESTONE)** | TASK09 |

## Design Documents

Shared ecosystem design documents remain in the parent directory:
- [DESIGN_recipe_ecosystem.md](../v0.4/DESIGN_recipe_ecosystem.md)
- [DESIGN_recipe_monorepo.md](../v0.4/DESIGN_recipe_monorepo.md)

## Dependency Graph

```
v0.4.1 → TASK05 → TASK06 → TASK07 → TASK08 → TASK09 → TASK10 ★
```
