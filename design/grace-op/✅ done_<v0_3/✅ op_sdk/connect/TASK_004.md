# TASK_004 — Migrate All Recipes to SDK `connect`

## Context

Depends on: `TASK_001`, `TASK_002` (SDKs must be correct first).

Every recipe frontend must use its SDK's `connect(slug)` to obtain
a gRPC channel. No raw gRPC channel creation allowed.

## Current state

| Recipe | Uses SDK connect? | Fix needed |
|--------|:-:|---|
| `go-dart-holons` | ✅ | — |
| `go-web-holons` | ✅ | — |
| `go-qt-holons` | ✅ | — |
| `go-kotlin-holons` | ✅ | — |
| `go-dotnet-holons` | ✅ | — |
| `go-swift-holons` | ❌ | Migrate to `swift-holons.connect` |
| `rust-dart-holons` | ✅ | — |
| `rust-web-holons` | ✅ | — |
| `rust-qt-holons` | ✅ | — |
| `rust-kotlin-holons` | ✅ | — |
| `rust-dotnet-holons` | ✅ | — |
| `rust-swift-holons` | ❌ | Migrate to `swift-holons.connect` |

## What to do

### `go-swift-holons`

- [ ] Add `swift-holons` as a dependency in `Package.swift`.
- [ ] Rewrite `GreetingClient.swift` to use
      `connect("gudule-greeting-goswift")`.
- [ ] Remove hardcoded `host`/`port` constants.
- [ ] Keep `DaemonProcess.swift` subprocess management if still needed,
      or remove it entirely if `connect()` handles the launch.
- [ ] Build and verify: launch, greet, quit.

### `rust-swift-holons`

- [ ] Same migration with slug `"gudule-greeting-rustswift"`.
- [ ] Build and verify.

## Verification

Build each recipe and run end-to-end:

```bash
# go-swift
cd recipes/go-swift-holons/examples/greeting/greeting-swiftui
swift build && swift run

# rust-swift
cd recipes/rust-swift-holons/examples/greeting/greeting-swiftui
swift build && swift run
```

Confirm: app launches, language picker populates, greeting RPC works.

## Rules

- SDK `connect` is mandatory — no raw gRPC channel creation.
- Commit per-recipe.
- If `connect()` handles daemon launch, remove duplicate subprocess
  management code from the recipe.
