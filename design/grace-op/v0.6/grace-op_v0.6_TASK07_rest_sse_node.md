# TASK07 — REST + SSE Transport for Node.js SDK

## Objective

Port the REST + SSE transport to `node-holons`.

## Reference

- [DESIGN_transport_rest_sse.md](./DESIGN_transport_rest_sse.md)
- TASK01 (Go reference implementation)

## Scope

Same as TASK01. Uses `fetch` for POST and `EventSource` /
`eventsource` npm package for SSE.

## Acceptance Criteria

- [ ] Unary RPC via POST
- [ ] Server-streaming via SSE
- [ ] Cross-language interop verified
- [ ] `npm test` — zero failures

## Dependencies

TASK01.
