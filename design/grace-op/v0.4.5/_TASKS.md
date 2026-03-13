# OP v0.4.5 Design Tasks — Native Daemon Expansion (C++, C, Java)

Extends the assembly matrix with three new native Daemon backend languages
(**18 new assemblies**, 66 total) and adds **C** as a primary composition orchestrator
language (**3 new composition recipes**, 36 total).

## Tasks

| # | File | Summary | Depends on | Status |
|---|---|---|---|---|
| 01 | [TASK01](./grace-op_v0.4.5_TASK01_daemon_cpp.md) | C++ Daemon × 6 HostUIs | v0.4.4 | — |
| 02 | [TASK02](./grace-op_v0.4.5_TASK02_daemon_c.md) | C Daemon × 6 HostUIs | TASK01 | — |
| 03 | [TASK03](./grace-op_v0.4.5_TASK03_daemon_java.md) | Java Daemon × 6 HostUIs | TASK02 | — |
| 04 | [TASK04](./grace-op_v0.4.5_TASK04_composition_c.md) | C composition orchestrators (direct, pipeline, fan-out) | TASK02 | — |
| 05 | [TASK05](./grace-op_v0.4.5_TASK05_update_docs.md) | Update docs | TASK01–04 | — |

> [!NOTE]
> TASK02, TASK03, and TASK04 can run in parallel once TASK01 establishes the native pattern. TASK04 shares the C tooling from TASK02.

## Dependency Graph

```text
v0.4.4 → TASK01 (C++ Daemon)
              ├─→ TASK02 (C Daemon, parallel)
              ├─→ TASK03 (Java Daemon, parallel)
              ├─→ TASK04 (C composition, parallel, shares TASK02 tooling)
              └─→ TASK05 (doc updates, after all above)
```

Design document: [DESIGN_native_daemon_expansion.md](./DESIGN_native_daemon_expansion.md)
