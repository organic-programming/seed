# TASK01 — Mesh Foundation: CA, Certs, Registry

## Objective

Implement `op mesh init`, `op mesh add` (local only), `mesh.yaml`
registry, and `op mesh list`. Pure Go, no external dependencies.

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`

## Reference

- [DESIGN_mesh.md](./DESIGN_mesh.md) — §Commands: init, add, list; §Data Model

## Scope

### `op mesh init`

- Generate ECDSA P-256 CA key pair
- Create `~/.op/mesh/ca.key`, `~/.op/mesh/ca.crt`
- Initialize empty `~/.op/mesh/mesh.yaml`
- CA validity: 10 years, `CN=OP Mesh CA`

### `op mesh add <host>`

- Generate host key pair (ECDSA P-256, 1 year validity)
- Sign host cert with CA
- Store in `~/.op/mesh/hosts/<host>/host.key`, `host.crt`
- Update `mesh.yaml` with new host entry
- SAN: DNS name + IP if resolvable

### `op mesh list`

- Parse `mesh.yaml`, display formatted table
- Show: host, port, cert expiry

### `mesh.yaml` format

```yaml
ca:
  cert: ~/.op/mesh/ca.crt
  created: 2026-03-09T12:00:00Z
hosts:
  - address: paris.example.com
    port: 9090
    cert: ~/.op/mesh/hosts/paris.example.com/host.crt
    added: 2026-03-09T12:01:00Z
```

## Acceptance Criteria

- [ ] `op mesh init` creates CA + empty registry
- [ ] `op mesh add` generates signed host cert
- [ ] `op mesh list` displays topology
- [ ] Pure Go (`crypto/x509`, `crypto/ecdsa`) — no openssl
- [ ] `go test ./...` — zero failures

## Dependencies

None — foundation.
