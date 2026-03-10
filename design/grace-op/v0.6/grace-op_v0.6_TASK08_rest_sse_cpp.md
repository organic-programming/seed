# TASK08 — REST + SSE Transport for C++ SDK

## Objective

Port the REST + SSE transport to `cpp-holons`.

## Reference

- [DESIGN_transport_rest_sse.md](./DESIGN_transport_rest_sse.md)
- TASK01 (Go reference implementation)

## Scope

Same as TASK01. Uses `libcurl` or similar for HTTP,
custom SSE parser for streaming.

## Acceptance Criteria

- [ ] Unary RPC via POST
- [ ] Server-streaming via SSE
- [ ] Cross-language interop verified
- [ ] Tests pass

## Dependencies

TASK01.
