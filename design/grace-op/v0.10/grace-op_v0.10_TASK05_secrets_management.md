# TASK05 — Secrets Management (API Keys Outside `holon.yaml`)

## Objective

Address DESIGN open question #1: move API keys and secrets out of
`holon.yaml` into a separate `secrets.yaml` that is gitignored.

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`
- `go-holons`: `github.com/organic-programming/go-holons`

## Reference

- [DESIGN_public_holons.md](./DESIGN_public_holons.md) — §Open Questions #1

## Scope

### `secrets.yaml`

```yaml
api_keys:
  - name: consumer-alpha
    key: sk_live_abc123...
    scopes: [read]
jwt:
  public_key: /path/to/key.pem
oauth:
  jwks_url: https://auth.example.com/.well-known/jwks.json
```

- Lives alongside `holon.yaml`, gitignored by convention
- SDK loads it automatically if present
- `holon.yaml` `serve.api_keys` still works as inline alternative
- `op check` warns if `api_keys` found in `holon.yaml` (suggests `secrets.yaml`)

## Acceptance Criteria

- [ ] `secrets.yaml` parsed and loaded by SDK
- [ ] Inline `api_keys` in `holon.yaml` still works (backward compat)
- [ ] `op check` warns about inline secrets
- [ ] `.gitignore` template includes `secrets.yaml`
- [ ] `go test ./...` — zero failures

## Dependencies

TASK03 (interceptors need key source).
