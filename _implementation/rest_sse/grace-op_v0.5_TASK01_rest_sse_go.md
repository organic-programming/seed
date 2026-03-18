# TASK01 — REST + SSE Transport for Go SDK

## Objective

Implement REST + SSE transport in `go-holons` as the reference
implementation. All other SDK tasks follow this one.

## Repository

- `go-holons`: `github.com/organic-programming/go-holons`
## Reference

- [DESIGN_transport_rest_sse.md](./DESIGN_transport_rest_sse.md)

## Scope

### Server side (`serve`)

- POST endpoint for unary RPCs (`/v1/<service>/<method>`)
- SSE endpoint for server-streaming RPCs (EventSource)
- `protojson` encoding for all payloads
- Listener registration in `serve.Run` with `rest+sse://` URI

### Client side (`connect`)

- REST client: POST for unary calls
- SSE client: EventSource for streaming calls
- Auto-reconnect on SSE disconnect
- Transport selection via `rest+sse://` in discover response

### Integration with `op`

- `op serve` supports `rest+sse://` listener
- `op dial` supports `rest+sse://` transport
- `holon.yaml` `serve.listeners` accepts `rest+sse://` URIs

## Acceptance Criteria

- [ ] Unary RPC via POST — request/response round-trip
- [ ] Server-streaming via SSE — multi-event delivery
- [ ] Auto-reconnect on SSE disconnect
- [ ] `protojson` encoding verified
- [ ] Gudule Greeting works over REST + SSE
- [ ] `go test ./...` — zero failures

## Dependencies

None — this is the reference implementation.
