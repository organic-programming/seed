# CONVENTIONS.md — Mesh Directories (Draft)

> These entries are designed to be added to the project's
> `CONVENTIONS.md` or a separate standard directories reference.

---

## Standard Directories — Mesh

| Directory | Purpose |
|---|---|
| `~/.op/mesh/` | Mesh root — CA, registry, host certs |
| `~/.op/mesh/ca.key` | CA private key (operator only, never deployed) |
| `~/.op/mesh/ca.crt` | CA public certificate (distributed to all hosts) |
| `~/.op/mesh/mesh.yaml` | Mesh registry (all hosts, operator only) |
| `~/.op/mesh/revoked.yaml` | Revoked certificate serials |
| `~/.op/mesh/hosts/` | Per-host certificate storage |
| `~/.op/mesh/hosts/<host>/host.key` | Host private key |
| `~/.op/mesh/hosts/<host>/host.crt` | Host certificate (signed by CA) |

### Remote host layout

On each remote host (deployed via `op mesh add --deploy`):

| File | Purpose |
|---|---|
| `~/.op/mesh/ca.crt` | Trust anchor for verifying peer certs |
| `~/.op/mesh/host.key` | This host's private key |
| `~/.op/mesh/host.crt` | This host's certificate |
