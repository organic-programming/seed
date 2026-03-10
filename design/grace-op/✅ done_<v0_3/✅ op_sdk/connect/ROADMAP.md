# Connect Roadmap

5 tasks, sequential. Fix the reference first, then fan out.

## Dependency chain

```
TASK_001 (Go reference)
    ├──→ TASK_002 (Tier 1: Dart stdio + verify Rust, Swift)
    │       └──→ TASK_004 (Migrate SwiftUI recipes to SDK connect)
    │               └──→ TASK_005 (End-to-end QA)
    └──→ TASK_003 (Tier 2+3: JS, Python, Kotlin, C#, C, C++, Java)
```

## Execution order

| # | Task | What | Parallel? |
|---|------|------|:---------:|
| 1 | [TASK_001](./TASK_001.md) | Fix Go `connect` default to `stdio` + add stdio test | — |
| 2 | [TASK_002](./TASK_002.md) | Fix Dart `connect` (stdio impl) + verify Rust, Swift | After 1 |
| 3 | [TASK_003](./TASK_003.md) | Fix remaining SDKs (JS, Python, Kotlin, etc.) | After 1, parallel with 2 |
| 4 | [TASK_004](./TASK_004.md) | Migrate `go-swift` and `rust-swift` recipes to SDK connect | After 2 |
| 5 | [TASK_005](./TASK_005.md) | Full runtime QA of all 12 recipes + 13 examples | After 4 |

## What was wrong

CODEX implementations drifted from the spec in `CONNECT.md`:

- **Go** (reference) defaulted to TCP instead of stdio → every
  SDK that copied Go inherited the wrong default.
- **Dart** has no stdio implementation at all.
- **SwiftUI recipes** bypassed the SDK entirely, hardcoding raw
  `grpc-swift` connections instead of using `swift-holons.connect()`.

## Principle

**Stdio is the default because it is the simplest and most reliable
transport for local holon-to-holon calls.** No port conflicts, no
firewall issues, no stale port files. TCP is the opt-in escape hatch
for debuggability and multi-process sharing.
