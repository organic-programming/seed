# OP v0.4.1 Design Tasks — Core Pattern & PoC

> [!CAUTION]
> **The current implementation is globally correct — do NOT rewrite it.**
> Extract, factor, and extend — do not redesign.

> [!IMPORTANT]
> **Always use the language SDK.** Every daemon and HostUI must use
> its language's Organic Programming SDK for server bootstrap,
> `connect(slug)`, and transport negotiation.

## Execution Strategy

Strictly linear — each task gates the next. TASK04 is the PoC milestone.

## Tasks

| # | File | Summary | Depends on |
|---|---|---|---|
| | | **— Shared Proto —** | |
| 01 | [TASK01](./grace-op_v0.4_TASK01_shared_proto.md) | Shared `greeting.proto` | — |
| | | **— PoC: Go + Dart —** | |
| 02 | [TASK02](./grace-op_v0.4_TASK02_dry_daemon_go.md) | Extract Go daemon | TASK01 |
| 03 | [TASK03](./grace-op_v0.4_TASK03_dry_hostui_flutter.md) | Extract Flutter/Dart HostUI | TASK02 |
| 04 | [TASK04](./grace-op_v0.4_TASK04_validate_go_dart_poc.md) | **★ Validate Go+Dart assembly (MILESTONE)** | TASK03 |

## Design Documents

Shared ecosystem design documents remain in the parent directory:
- [DESIGN_recipe_ecosystem.md](../v0.4/DESIGN_recipe_ecosystem.md)
- [DESIGN_recipe_monorepo.md](../v0.4/DESIGN_recipe_monorepo.md)

## Dependency Graph

```
TASK01 → TASK02 → TASK03 → TASK04 ★
```
