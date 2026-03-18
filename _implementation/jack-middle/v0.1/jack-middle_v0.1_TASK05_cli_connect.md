# TASK05 — CLI Entrypoint and Backend Connect

## Context

Jack needs a CLI entrypoint that wires the frontend listener,
backend connection, and middleware chain together.

## Objective

Implement `cmd/jack-middle/main.go` with CLI flag parsing,
backend connect via the OP SDK, and server startup.

## Changes

### `cmd/jack-middle/main.go` [NEW]

```go
func main() {
    // Parse flags: --listen, --target, --middleware, etc.
    // Connect to backend via SDK connect(target) or direct URI
    // Build middleware chain from --middleware flag
    // Start gRPC server with UnknownServiceHandler
    // Register MiddleService for control plane
}
```

CLI flags:

| Flag | Default | Description |
|---|---|---|
| `--listen` | `tcp://127.0.0.1:0` | Frontend listener URI |
| `--target` | (required) | Holon slug or direct URI |
| `--middleware` | `logger` | Comma-separated chain |
| `--record-dir` | `/tmp/jack-traces/` | Recorder output dir |
| `--latency` | `0ms` | Artificial latency |
| `--fault-rate` | `0.0` | Error injection rate |
| `--fault-code` | `UNAVAILABLE` | Injected error code |
| `--hijack` | `false` | Overwrite target's .port file |

### Backend Connection

Jack uses the standard OP `connect(slug)` algorithm to reach
the target holon. This means he benefits from auto-start,
readiness verification, and transport negotiation — just like
any other caller.

### Port File Hijacking (--hijack)

When `--hijack` is set:
1. Connect to the real target (creates its `.port` file)
2. Start Jack's frontend listener
3. Overwrite `.op/run/<slug>.port` with Jack's address
4. All subsequent `connect(slug)` calls route through Jack

## Acceptance Criteria

- [ ] `jack-middle --target rob-go` starts and proxies RPCs
- [ ] `--middleware logger,metrics` activates both middleware
- [ ] `--hijack` overwrites the target's port file
- [ ] Clean shutdown closes backend connection and removes hijacked port file

## Dependencies

TASK01 (proxy core), TASK02 (middleware chain).
