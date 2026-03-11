# TASK10 — 3×3 Cross-Language Validation Milestone

## Summary

Before generating all 48 assemblies, validate the DRY pattern across
a representative 3×3 matrix: 3 daemons from different toolchains ×
3 HostUIs from different frameworks. This catches cross-language
transport, proto inclusion, and `connect(slug)` issues early.

## Matrix

|              | Flutter | Web | Qt |
|---|---|---|---|
| **Go**       | ✅ (TASK04) | — | — |
| **Rust**     | — | — | — |
| **Swift**    | — | — | — |

9 assemblies, covering:
- 3 daemon runners: `go-module`, `cargo`, `swift-package`
- 3 HostUI runners: `flutter`, `npm`, `qt-cmake`
- Transport negotiation across tcp / stdio boundaries

## Assemblies to Validate

| Assembly | Daemon | HostUI |
|---|---|---|
| `gudule-greeting-flutter-go` | Go | Flutter (already TASK04) |
| `gudule-greeting-flutter-rust` | Rust | Flutter |
| `gudule-greeting-flutter-swift` | Swift | Flutter |
| `gudule-greeting-go-web` | Go | Web |
| `gudule-greeting-rust-web` | Rust | Web |
| `gudule-greeting-swift-web` | Swift | Web |
| `gudule-greeting-qt-go` | Go | Qt |
| `gudule-greeting-qt-rust` | Rust | Qt |
| `gudule-greeting-qt-swift` | Swift | Qt |

## Acceptance Criteria

- [ ] All 9 assemblies build with `op build`
- [ ] All 9 run with `op run` (daemon starts, UI connects)
- [ ] `connect(slug)` works across all daemon/HostUI combinations
- [ ] Transport negotiation succeeds (tcp and stdio)
- [ ] Works on macOS (primary), Linux and Windows where applicable

## Why This Matters

If transport, proto inclusion, or connect(slug) breaks for any of
these 9 combinations, it will break for the remaining 39 too. Fix
it once in a small matrix, then scale mechanically.

## Dependencies

TASK04 (PoC validated), TASK05 (Rust daemon), TASK06 (Swift daemon),
TASK08 (SwiftUI — needed for Swift HostUI extraction pattern),
TASK09 (Web + Qt HostUI).

> [!NOTE]
> This task runs after the first HostUI batch (TASK09) but before
> the full 48-assembly generation (TASK11). It validates the
> cross-language machinery at small scale.
