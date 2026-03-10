# TASK01 — Per-Listener Security Schema in `holon.yaml`

## Objective

Extend the `holon.yaml` manifest to support per-listener `security`
and `auth` annotations in `serve.listeners`.

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`

## Reference

- [DESIGN_public_holons.md](./DESIGN_public_holons.md) — §Configuration in `holon.yaml`

## Scope

### `serve.listeners` schema

```yaml
serve:
  listeners:
    - uri: tcp://:9090
      security: mesh
    - uri: tcp://:443
      security: public
      auth: api-key
    - uri: unix:///tmp/holon.sock
      security: none
```

Fields per listener:
- `uri` (required): transport URI
- `security` (optional): `none`, `mesh`, `public` (default: auto-detect)
- `auth` (optional, public only): `api-key`, `jwt`, `oauth`

### `serve.tls` schema

```yaml
serve:
  tls:
    cert: /path/to/fullchain.pem
    key: /path/to/privkey.pem
```

### `serve.api_keys` schema

```yaml
serve:
  api_keys:
    - name: consumer-alpha
      key: sk_live_abc123...
      scopes: [read]
```

### `op check` validation

- `security` must be one of: `none`, `mesh`, `public`
- `auth` only allowed when `security: public`
- `tls` required when any listener uses `public`
- Warn if `none` is used with `tcp://` (insecure remote)

## Acceptance Criteria

- [ ] `serve.listeners` parsed with security/auth fields
- [ ] `serve.tls` and `serve.api_keys` parsed
- [ ] `op check` validates all constraints
- [ ] Existing holons without listeners unaffected
- [ ] `go test ./...` — zero failures

## Dependencies

v0.9 (mesh schema must be stable).
