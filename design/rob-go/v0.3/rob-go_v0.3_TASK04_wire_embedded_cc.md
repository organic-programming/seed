# TASK04 — Wire Embedded CC into Hermetic Environment

## Context

Once the embedded C toolchain is provisioned (TASK03), the
hermetic environment must set `CC` and `CXX` to the bundled
compiler paths instead of inheriting from the host.

## Objective

Integrate the embedded C toolchain into `HermeticEnv` so
CGO builds use it automatically when available.

## Changes

### `internal/toolchain/env.go`

Add embedded CC resolution to `HermeticEnv`:

```go
// Priority for CC/CXX:
// 1. Caller override (per-RPC env field)
// 2. Embedded CC toolchain (if provisioned)
// 3. Host passthrough (if cgoPassthrough and no embedded CC)
// 4. Omitted (CGO disabled)
```

### `internal/service/service.go`

At startup, attempt `EnsureCC`. If available, store
`CCToolchain` in `GoServer` and use it for env construction.

### `internal/toolchain/env_test.go`

- `TestHermeticEnvEmbeddedCC` — verify CC/CXX point to bundled paths
- `TestHermeticEnvEmbeddedCCOverride` — caller override still wins
- `TestHermeticEnvFallbackPassthrough` — no embedded CC falls back to host

## Acceptance Criteria

- [ ] When embedded CC exists, `CC`/`CXX` point to bundled binaries
- [ ] Caller override takes final precedence
- [ ] Falls back to host passthrough when no embedded CC
- [ ] CGO builds succeed with embedded compiler

## Dependencies

TASK01 (passthrough), TASK03 (embedded CC provisioning).
