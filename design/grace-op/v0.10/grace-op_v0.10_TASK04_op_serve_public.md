# TASK04 — `op serve` Public Listener Support

## Objective

Update `op serve` to configure and start public listeners with
TLS and auth interceptors.

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`

## Reference

- [DESIGN_public_holons.md](./DESIGN_public_holons.md) — §Relationship to `op mesh`

## Scope

### `op serve` changes

- Read `serve.listeners` with `security: public` annotations
- Load TLS cert/key from `serve.tls` paths
- Start public listener with auth interceptor from `serve.auth`
- Coexists with mesh and local listeners

### `op check` additions

- Validate TLS cert/key files exist when `public` listener declared
- Validate `auth` strategy is supported
- Warn if API keys are committed to git (suggest `secrets.yaml`)

## Acceptance Criteria

- [ ] `op serve` starts public listener with TLS
- [ ] Auth interceptor active on public listener
- [ ] Mesh + public + local listeners coexist
- [ ] `op check` validates TLS and auth config
- [ ] `go test ./...` — zero failures

## Dependencies

TASK01 (schema), TASK03 (auth interceptors).
