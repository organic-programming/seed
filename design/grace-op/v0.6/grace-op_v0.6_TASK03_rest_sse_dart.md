# TASK03 — REST + SSE Transport for Dart SDK

## Objective

Port the REST + SSE transport to `dart-holons`.

## Reference

- [DESIGN_transport_rest_sse.md](./DESIGN_transport_rest_sse.md)
- TASK01 (Go reference implementation)

## Scope

Same as TASK01. Dart's `http` and `EventSource` APIs map naturally
to REST + SSE.

## Acceptance Criteria

- [ ] Unary RPC via POST
- [ ] Server-streaming via SSE
- [ ] Cross-language interop verified
- [ ] `dart test` — zero failures

## Dependencies

TASK01.
