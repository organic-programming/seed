# TASK02 — REST + SSE Transport for Rust SDK

## Objective

Port the REST + SSE transport from Go to `rust-holons`.

## Reference

- [DESIGN_transport_rest_sse.md](./DESIGN_transport_rest_sse.md)
- TASK01 (Go reference implementation)

## Scope

Same as TASK01: server (POST + SSE), client (REST + EventSource),
`protojson` encoding, auto-reconnect, `rest+sse://` URI scheme.

## Acceptance Criteria

- [ ] Unary RPC via POST
- [ ] Server-streaming via SSE
- [ ] Auto-reconnect on SSE disconnect
- [ ] Cross-language: Rust server ↔ Go client and vice versa
- [ ] `cargo test` — zero failures

## Dependencies

TASK01 (Go reference establishes the wire format).
