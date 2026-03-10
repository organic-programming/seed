# TASK06 — REST + SSE Transport for C# SDK

## Objective

Port the REST + SSE transport to `dotnet-holons`.

## Reference

- [DESIGN_transport_rest_sse.md](./DESIGN_transport_rest_sse.md)
- TASK01 (Go reference implementation)

## Scope

Same as TASK01. Uses `HttpClient` for POST and
`HttpClient.GetStreamAsync` for SSE parsing.

## Acceptance Criteria

- [ ] Unary RPC via POST
- [ ] Server-streaming via SSE
- [ ] Cross-language interop verified
- [ ] `dotnet test` — zero failures

## Dependencies

TASK01.
