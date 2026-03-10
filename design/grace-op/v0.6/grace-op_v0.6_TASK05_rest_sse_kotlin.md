# TASK05 — REST + SSE Transport for Kotlin SDK

## Objective

Port the REST + SSE transport to `kotlin-holons`.

## Reference

- [DESIGN_transport_rest_sse.md](./DESIGN_transport_rest_sse.md)
- TASK01 (Go reference implementation)

## Scope

Same as TASK01. Uses OkHttp/Ktor for POST and SSE.

## Acceptance Criteria

- [ ] Unary RPC via POST
- [ ] Server-streaming via SSE
- [ ] Cross-language interop verified
- [ ] `gradle test` — zero failures

## Dependencies

TASK01.
