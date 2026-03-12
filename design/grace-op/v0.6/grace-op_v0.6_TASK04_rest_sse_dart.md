# TASK04 — REST + SSE Client for Dart SDK

## Objective

Implement REST + SSE **client** transport in `dart-holons`.
Dart is a frontend SDK — it connects to daemon holons but
does **not** serve REST+SSE endpoints.

## Repository

- `dart-holons`: `github.com/organic-programming/dart-holons`
## Reference

- [DESIGN_transport_rest_sse.md](./DESIGN_transport_rest_sse.md)
- TASK01 (Go reference implementation)

## Scope — Client Only

- REST client: `http.post()` for unary RPCs
- SSE client: `EventSource` for server-streaming RPCs
- Auto-reconnect on SSE disconnect
- Transport selection via `rest+sse://` in discover response

> The server side (serving REST+SSE endpoints) is **not** in
> scope. Dart is classified as a **frontend SDK** (Flutter).

## Acceptance Criteria

- [ ] Unary RPC via POST — Dart → Go daemon round-trip
- [ ] Server-streaming via SSE — multi-event delivery
- [ ] Auto-reconnect on SSE disconnect
- [ ] Cross-language interop verified (Dart client → Go server)
- [ ] `dart test` — zero failures

## Dependencies

TASK01, TASK02.

