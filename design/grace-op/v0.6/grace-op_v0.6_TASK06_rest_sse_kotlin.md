# TASK06 — REST + SSE Client for Kotlin SDK

## Objective

Implement REST + SSE **client** transport in `kotlin-holons`.
Kotlin is a frontend SDK — it connects to daemon holons via
Kotlin Desktop/Android but does **not** serve REST+SSE endpoints.

## Repository

- `kotlin-holons`: `github.com/organic-programming/kotlin-holons`
## Reference

- [DESIGN_transport_rest_sse.md](./DESIGN_transport_rest_sse.md)
- TASK01 (Go reference implementation)

## Scope — Client Only

- REST client: OkHttp/Ktor for unary RPCs
- SSE client: OkHttp's `EventSource` for server-streaming RPCs
- Auto-reconnect on SSE disconnect
- Transport selection via `rest+sse://` in discover response

> The server side (Ktor serving) is **not** in scope for v0.6.

## Acceptance Criteria

- [ ] Unary RPC via POST — Kotlin → Go daemon round-trip
- [ ] Server-streaming via SSE — multi-event delivery
- [ ] Auto-reconnect on SSE disconnect
- [ ] Cross-language interop verified (Kotlin client → Go server)
- [ ] `gradle test` — zero failures

## Dependencies

TASK01, TASK02.

