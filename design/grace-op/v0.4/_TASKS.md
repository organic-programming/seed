# OP v0.4 Design Tasks — Master Index

To prevent LLM context saturation during implementation (14 tasks, 48 assemblies, 33 composition recipes), the v0.4 design phase is split into four strictly linear execution milestones.

You must implement these **in order** and validate the milestones before moving on.

## Execution Milestones

### 1. [v0.4.1: The Core Pattern & PoC](../v0.4.1/_TASKS.md)
**Tasks 01–04:** Establish the shared proto schema and build the canonical Go daemon + Flutter HostUI assembly.  
**Milestone:** Complete validation of the declarative `connect(slug)` architecture end-to-end.

### 2. [v0.4.2: Component Extraction & Matrix](../v0.4.2/_TASKS.md)
**Tasks 05–10:** Mechanical extraction of the remaining 7 daemons and 5 HostUIs.  
**Milestone:** 3×3 Cross-Language Validation (9 assemblies). Proves transport negotiation works across all core toolchains (Go, Rust, Swift, Web, Dart/Flutter, Qt).

### 3. [v0.4.3: Scale, Composition & Testing](../v0.4.3/_TASKS.md)
**Tasks 11–14:** Generate the remaining 39 assemblies (48 total), implement the 3 backend-to-backend composition patterns in 11 languages, and build the CI Testmatrix.  
**Milestone:** Combinatorial test matrix passes and the 12 old submodule repos are fully deprecated.

### 4. [v0.4.4: Bundle Auto-Signing](../v0.4.4/_TASKS.md)
**Tasks 01–02:** Auto ad-hoc signing in the recipe runner, then removal of hand-rolled codesign from 10 assembly manifests.  
**Milestone:** `op build` auto-signs `.app`/`.framework` bundles; no codesign boilerplate remains in any assembly.

---

> [!TIP]  
> See [DESIGN_recipe_ecosystem.md](./DESIGN_recipe_ecosystem.md) for the high-level theory driving all four milestones.
