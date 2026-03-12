# OP v0.4.5 Design Tasks — Native Expansion (C++, C, Java)

Extends the assembly matrix with three new native HostUI frameworks
(**24 new assemblies**, 72 total) and adds C as a composition orchestrator
language (**3 new composition recipes**, 36 total).

## Tasks

| # | File | Summary | Depends on | Status |
|---|---|---|---|---|
| 01 | [TASK01](./grace-op_v0.4.5_TASK01_hostui_cpp.md) | C++ HostUI (Qt/imgui) × 8 daemons | v0.4.4 | — |
| 02 | [TASK02](./grace-op_v0.4.5_TASK02_hostui_c.md) | C HostUI (GTK/SDL) × 8 daemons | TASK01 | — |
| 03 | [TASK03](./grace-op_v0.4.5_TASK03_hostui_java.md) | Java HostUI (Swing/JavaFX) × 8 daemons | TASK01 | — |
| 04 | [TASK04](./grace-op_v0.4.5_TASK04_composition_c.md) | C composition orchestrators (direct, pipeline, fan-out) | TASK02 | — |
| 05 | [TASK05](./grace-op_v0.4.5_TASK05_update_docs.md) | Update VERIFICATION_composition.md & docs (preserve ❌⚠️✅ annotations) | TASK01–04 | — |

> [!NOTE]
> TASK02, TASK03, and TASK04 can run in parallel once TASK01 establishes the native pattern. TASK04 shares the C tooling from TASK02.

## Dependency Graph

```
v0.4.4 → TASK01 (C++ HostUI)
              ├─→ TASK02 (C HostUI, parallel)
              ├─→ TASK03 (Java HostUI, parallel)
              ├─→ TASK04 (C composition, parallel, shares TASK02 tooling)
              └─→ TASK05 (doc updates, after all above)
```

Design document: [DESIGN_native_hostui_expansion.md](./DESIGN_native_hostui_expansion.md)
