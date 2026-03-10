# TASK02 — REST + SSE Support in grace-op CLI

## Objective

Wire the REST + SSE transport into `op serve` and `op dial` so
the `op` CLI can orchestrate holons using REST + SSE.

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`

## Reference

- [DESIGN_transport_rest_sse.md](./DESIGN_transport_rest_sse.md)
- TASK01 (Go SDK provides the transport primitives)

## Scope

### `op serve`

- Accept `rest+sse://` in `serve.listeners` (holon.yaml)
- Start HTTP server with POST + SSE endpoints
- Coexists with existing gRPC listener

### `op dial`

- Recognize `rest+sse://` in discover response
- Use REST + SSE client from `go-holons` to connect

### `op check`

- Validate `rest+sse://` URIs in `serve.listeners`

## Acceptance Criteria

- [ ] `op serve` starts REST + SSE listener alongside gRPC
- [ ] `op dial` connects via REST + SSE
- [ ] Gudule Greeting: `op run` over REST + SSE
- [ ] `go test ./...` — zero failures

## Dependencies

TASK01 (Go SDK transport must exist first).
