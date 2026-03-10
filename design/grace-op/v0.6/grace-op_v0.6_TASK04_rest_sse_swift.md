# TASK04 — REST + SSE Transport for Swift SDK

## Objective

Port the REST + SSE transport to `swift-holons`.

## Reference

- [DESIGN_transport_rest_sse.md](./DESIGN_transport_rest_sse.md)
- TASK01 (Go reference implementation)

## Scope

Same as TASK01. Uses `URLSession` for POST and
`EventSource` (or custom SSE parser) for streaming.

## Acceptance Criteria

- [ ] Unary RPC via POST
- [ ] Server-streaming via SSE
- [ ] Cross-language interop verified
- [ ] `swift test` — zero failures

## Dependencies

TASK01.
