# TASK_002 — Fix Tier 1 SDK `connect` Defaults

## Context

Depends on: `TASK_001` (Go reference must be correct first).

Tier 1 SDKs are those used by recipe frontends. They must match the
Go reference: default transport is `stdio`.

## Current state

| SDK | Default | stdio impl | Action needed |
|-----|:-------:|:----------:|---------------|
| `rust-holons` | ✅ `stdio` | ✅ | Verify tests pass |
| `swift-holons` | ✅ `stdio` | ✅ | Verify tests pass |
| `dart-holons` | ❌ `tcp` | ❌ | Fix default + implement stdio |

## What to do

### 1. `dart-holons` — fix default + implement stdio

- [ ] Change `ConnectOptions.transport` default from `'tcp'` to `'stdio'`.
- [ ] Remove the guard that rejects non-tcp transport (line 42).
- [ ] Implement `_startStdioHolon()`: use `Process.start` with
      `stdin`/`stdout` piped, wire gRPC over the pipes.
- [ ] Add test: `connect(slug)` with no options → starts via stdio,
      round-trip works, disconnect stops process, no port file.
- [ ] Verify all existing tests still pass.

### 2. `rust-holons` — verify

- [ ] `cargo test` passes all connect tests.
- [ ] Default is confirmed `stdio` in source.

### 3. `swift-holons` — verify

- [ ] `swift test` passes all connect tests.
- [ ] Default is confirmed `stdio` in source.

## Verification

```bash
# Dart
cd sdk/dart-holons && dart test

# Rust
cd sdk/rust-holons && cargo test

# Swift
cd sdk/swift-holons && swift test
```

## Rules

- Each SDK is a separate commit.
- Do not modify the connect API signature.
