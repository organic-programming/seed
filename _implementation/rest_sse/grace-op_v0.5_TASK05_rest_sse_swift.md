# TASK05 — REST + SSE Client for Swift SDK

## Objective

Implement REST + SSE **client** transport in `swift-holons`.
Swift is a frontend SDK — it connects to daemon holons via
SwiftUI applications but does **not** serve REST+SSE endpoints.

## Repository

- `swift-holons`: `github.com/organic-programming/swift-holons`
## Reference

- [DESIGN_transport_rest_sse.md](./DESIGN_transport_rest_sse.md)
- TASK01 (Go reference implementation)

## Scope — Client Only

- REST client: `URLSession.data(for:)` for unary RPCs
- SSE client: custom `EventSource` parser (or `EventSource` library) for server-streaming RPCs
- Auto-reconnect on SSE disconnect
- Transport selection via `rest+sse://` in discover response

> The server side is **not** in scope for v0.5. A future
> version may add REST+SSE serving for macOS SwiftUI apps
> acting as local mesh holons (via Vapor or SwiftNIO).

## Acceptance Criteria

- [ ] Unary RPC via POST — Swift → Go daemon round-trip
- [ ] Server-streaming via SSE — multi-event delivery
- [ ] Auto-reconnect on SSE disconnect
- [ ] Cross-language interop verified (Swift client → Go server)
- [ ] `swift test` — zero failures

## Dependencies

TASK01, TASK02.

