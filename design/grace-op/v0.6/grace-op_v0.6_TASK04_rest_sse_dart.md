# TASK04 — REST + SSE Transport for Dart SDK

## Objective

Port the REST + SSE transport to `dart-holons`.

## Repository

- `dart-holons`: `github.com/organic-programming/dart-holons`
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

- [ ] Verify with `op run` over REST + SSE
## Dependencies

TASK01, TASK02.
