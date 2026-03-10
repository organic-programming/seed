# TASK03 — Auth Interceptors (API Key, JWT, OAuth)

## Objective

Implement gRPC auth interceptors for public listeners, supporting
API key, JWT, and OAuth validation strategies.

## Repository

- `go-holons`: `github.com/organic-programming/go-holons` (reference)

## Reference

- [DESIGN_public_holons.md](./DESIGN_public_holons.md) — §Auth Strategies

## Scope

### API Key interceptor (`auth: api-key`)

- Read `x-api-key` from gRPC metadata
- Validate against `serve.api_keys` list
- Attach consumer name + scopes to context

### JWT interceptor (`auth: jwt`)

- Read `authorization: Bearer <token>` from metadata
- Validate signature using configured public key / JWKS
- Extract claims, attach to context

### OAuth interceptor (`auth: oauth`)

- Read `authorization: Bearer <token>` from metadata
- Validate against OAuth provider's JWKS endpoint
- Cache JWKS for performance
- Extract claims, attach to context

### Consumer identity on context

All interceptors attach a `ConsumerIdentity` to the gRPC context:
- `Name` (string) — consumer identifier
- `Scopes` ([]string) — allowed scopes
- Holon logic reads identity via `auth.ConsumerFromContext(ctx)`

## Acceptance Criteria

- [ ] API key interceptor validates and rejects correctly
- [ ] JWT interceptor validates signatures
- [ ] OAuth interceptor fetches and caches JWKS
- [ ] Consumer identity available on gRPC context
- [ ] Invalid/missing credentials → gRPC `Unauthenticated` error
- [ ] `go test ./...` — zero failures

## Dependencies

TASK02 (multi-listener must route to interceptor).
