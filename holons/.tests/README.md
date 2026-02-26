# Holons Integration Tests

This test level validates cross-holon behavior between `grace-op` and `sophia-who`.

Coverage:
- `op` dispatch to `who` over `mem://` (create + list + show round-trip)
- `op` dispatch to `who` over `stdio://` (create + list + show round-trip)
- parity check that identity results from `mem://` and `stdio://` are equivalent

Run:

```bash
go test ./...
```

Prerequisites:
- Go toolchain available on `PATH`
- Source modules available at:
  - `../grace-op`
  - `../sophia-who`
- Tests build temporary `op` and `who` binaries in an isolated temp workspace.
