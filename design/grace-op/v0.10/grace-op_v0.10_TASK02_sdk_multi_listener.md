# TASK02 — SDK Multi-Listener with Security Modes

## Objective

Update `serve.Run` across SDKs to start multiple listeners with
different security modes (none, mesh, public) on the same server.

## Repositories

- `go-holons`: `github.com/organic-programming/go-holons` (reference)
- All other SDK repos (port after Go)

## Reference

- [DESIGN_public_holons.md](./DESIGN_public_holons.md) — §SDK Behavior, §How the SDK Wires It

## Scope

### `serve.Run` changes (Go reference)

1. Parse `serve.listeners` from `holon.yaml`
2. For each listener:
   - `security: none` → plain listener (no TLS)
   - `security: mesh` → load `~/.op/mesh/` certs, configure mTLS
   - `security: public` → load `serve.tls` cert, standard TLS
3. Start all listeners on the same gRPC server
4. Auto-detect: `stdio://` / `unix://` → `none`, `tcp://` with mesh certs → `mesh`

### Zero security code for holon developers

The holon developer writes business logic only. `serve.Run` handles
all TLS, mTLS, and listener setup transparently.

## Acceptance Criteria

- [ ] Go SDK: multi-listener with mixed security modes
- [ ] `none` listener works for local transports
- [ ] `mesh` listener rejects non-mesh certificates
- [ ] `public` listener accepts standard TLS connections
- [ ] Auto-detection works (no explicit `security` required for local)
- [ ] Port to at least Rust SDK
- [ ] `go test ./...` — zero failures

## Dependencies

TASK01 (schema), v0.9 (mesh certs).
