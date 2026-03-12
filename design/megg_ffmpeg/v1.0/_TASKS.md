# Megg FFmpeg v1.0 — Production Tasks

> Stabilization, hardening, cross-platform CI, performance, documentation.
> No new features — ship-quality milestone.

## Execution Strategy

Most tasks are independent and can run in parallel. TASK07 (API freeze)
must come last.

## Tasks

| # | File | Summary | Depends on |
|---|------|---------|------------|
| | | **— Quality —** | |
| 01 | [TASK01](./MEGG_TASK001_test_suite.md) | Full test suite (unit + integration + fuzz) | v0.6 |
| 02 | [TASK02](./MEGG_TASK002_memory_safety.md) | ASAN/MSAN/TSAN pass on all tests | TASK01 |
| 03 | [TASK03](./MEGG_TASK003_performance.md) | Performance profiling + optimization | v0.6 |
| | | **— Platform —** | |
| 04 | [TASK04](./MEGG_TASK004_ci_matrix.md) | Cross-platform CI: macOS arm64/x86_64, Linux, Windows | v0.6 |
| 05 | [TASK05](./MEGG_TASK005_binary_size.md) | Binary size optimization (strip, LTO, codec subset) | v0.6 |
| | | **— Release —** | |
| 06 | [TASK06](./MEGG_TASK006_documentation.md) | Full API docs, examples, integration cookbook | v0.6 |
| 07 | [TASK07](./MEGG_TASK007_api_freeze.md) | Proto v3 API stability freeze | TASK01–06 |
| 08 | [TASK08](./MEGG_TASK008_license_audit.md) | LGPL/GPL compliance audit | v0.6 |
| 09 | [TASK09](./MEGG_TASK009_testmatrix.md) | Testmatrix entry for megg-ffmpeg | TASK04 |

## Dependency Graph

```
v0.6 → TASK01 → TASK02 ──┐
     → TASK03 ────────────┤
     → TASK04 → TASK09 ───┤→ TASK07
     → TASK05 ────────────┤
     → TASK06 ────────────┤
     → TASK08 ────────────┘
```
