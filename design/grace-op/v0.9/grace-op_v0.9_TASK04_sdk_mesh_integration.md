# TASK04 — SDK Mesh Integration: Discover, Connect, Serve

## Objective

Extend the SDK's `discover`, `connect`, and `serve.Run` to be
mesh-aware: resolve remote holons, dial with mTLS, serve with
mTLS auto-detection.

## Repositories

- `go-holons`: `github.com/organic-programming/go-holons` (reference)
- `rust-holons`: `github.com/organic-programming/rust-holons`
- All other SDKs (port after Go + Rust)

## Reference

- [DESIGN_mesh.md](./DESIGN_mesh.md) — §SDK Integration

## Scope

### Enhanced `discover`

```
discover("phill-files") search order:
1. Local OPPATH scan (existing)
2. Read ~/.op/mesh/mesh.yaml
3. For each host, query HolonMeta/Describe (cached)
4. Return matching holon with remote address
```

### Enhanced `connect`

- If remote holon → load `host.key` + `host.crt` + `ca.crt`
- Dial with mTLS credentials
- Transparent: holon code doesn't change

### `serve.Run` mTLS mode

- Auto-detect: if `~/.op/mesh/` certs exist → enable mTLS
- `RequireAndVerifyClientCert` for mesh listeners
- No config needed in `holon.yaml` for basic mesh mode

## Acceptance Criteria

- [ ] Go SDK: `discover` finds remote holons via mesh
- [ ] Go SDK: `connect` dials remote with mTLS
- [ ] Go SDK: `serve.Run` auto-enables mTLS
- [ ] Rust SDK: same behavior
- [ ] Cross-host round-trip: holon A on host 1 calls holon B on host 2
- [ ] Existing local-only holons unaffected
- [ ] `go test ./...` — zero failures

## Dependencies

TASK01 (registry), TASK02 (certs deployed), v0.6 (REST+SSE as fallback transport).
