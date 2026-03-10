# TASK02 — Mesh Deployment: SSH Provisioning & Removal

## Objective

Implement `--deploy` flag for SSH-based cert provisioning,
`op mesh remove`, and `--force` for cert renewal.

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`

## Reference

- [DESIGN_mesh.md](./DESIGN_mesh.md) — §Commands: add --deploy, remove

## Scope

### `op mesh add <host> --deploy`

- SSH into remote host (`golang.org/x/crypto/ssh`)
- Create `~/.op/mesh/` on remote
- Copy `host.key`, `host.crt`, `ca.crt`
- Verify files in place
- Support `--user`, `--key` flags

### `op mesh remove <host>`

- Delete local cert files
- Remove entry from `mesh.yaml`
- Add cert serial to `~/.op/mesh/revoked.yaml`
- Optional `--deploy` to delete remote certs

### `op mesh add <host> --force`

- Re-generate cert (renewal)
- Replace local + remote files

## Acceptance Criteria

- [ ] SSH deploy copies 3 files to remote host
- [ ] `--user` and `--key` flags work
- [ ] `op mesh remove` cleans up local + registry
- [ ] `--force` renews expired certs
- [ ] Pure Go SSH (no system ssh binary)
- [ ] `go test ./...` — zero failures

## Dependencies

TASK01 (CA + registry must exist).
