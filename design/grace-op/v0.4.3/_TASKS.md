# OP v0.4.3 Design Tasks — Assembly & Composition

> [!CAUTION]
> **The current implementation is globally correct — do NOT rewrite it.**
> Extract, factor, and extend — do not redesign.

> [!IMPORTANT]
> **Always use the language SDK.** Every daemon and HostUI must use
> its language's Organic Programming SDK for server bootstrap,
> `connect(slug)`, and transport negotiation.

## Execution Strategy

Strictly linear — each task gates the next. This final phase scales the architecture to 48 assemblies and 33 composition orchestrators, concluding with an automated test matrix.

## Tasks

| # | File | Summary | Depends on |
|---|---|---|---|
| | | **— Assembly & Cleanup —** | |
| 11 | [TASK11](./grace-op_v0.4.3_TASK01_assembly_manifests.md) | Create 48 assembly manifests | TASK10 (from v0.4.2) |
| 12 | [TASK12](./grace-op_v0.4.3_TASK02_remove_submodules.md) | Remove 12 submodules, archive repos | TASK11 (parallel, not blocking) |
| 13 | [TASK13](./grace-op_v0.4.3_TASK03_composition_recipes.md) | 3 patterns × 11 orchestrator languages | TASK11 |
| 14 | [TASK14](./grace-op_v0.4.3_TASK04_testmatrix.md) | Combinatorial testing (Go testmatrix) | TASK13 |

## Design Documents

Shared ecosystem design documents remain in the parent directory:
- [DESIGN_recipe_ecosystem.md](../v0.4/DESIGN_recipe_ecosystem.md)
- [DESIGN_recipe_monorepo.md](../v0.4/DESIGN_recipe_monorepo.md)

## Dependency Graph

```
v0.4.2 → TASK01 → TASK03 → TASK04
                └─→ TASK02 (parallel cleanup)
```
