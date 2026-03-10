# TASK03 — Mesh Introspection: Status & Describe

## Objective

Implement `op mesh status` (health check all hosts) and
`op mesh describe <host>` (enumerate remote holons).

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`

## Reference

- [DESIGN_mesh.md](./DESIGN_mesh.md) — §Commands: status, describe

## Scope

### `op mesh status`

- Connect to each host in `mesh.yaml` via mTLS gRPC
- Call `grpc.health.v1.Health/Check`
- Optionally call `HolonMeta/Describe` for holon count
- Display: host, port, holons, latency, status (✅/❌)

### `op mesh describe <host>`

- Connect to specific host via mTLS
- Call `HolonMeta/Describe`
- Display: host details, cert validity, OS/arch, holon list

### Error handling

- Unreachable hosts → `❌ unreachable` (not fatal)
- Cert expired → `⚠️ cert expired`
- Connection timeout: 5s default, configurable

## Acceptance Criteria

- [ ] `op mesh status` pings all hosts, shows table
- [ ] `op mesh describe` shows holon inventory
- [ ] Unreachable hosts reported gracefully
- [ ] mTLS connection using mesh certs
- [ ] `go test ./...` — zero failures

## Dependencies

TASK01 (registry), TASK02 (certs deployed).
