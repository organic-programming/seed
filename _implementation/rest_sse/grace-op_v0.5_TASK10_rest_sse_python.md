# TASK10 — REST + SSE Transport for Python SDK

## Objective

Port the REST + SSE transport to `python-holons`.

## Repository

- `python-holons`: `github.com/organic-programming/python-holons`
## Reference

- [DESIGN_transport_rest_sse.md](./DESIGN_transport_rest_sse.md)
- TASK01 (Go reference implementation)

## Scope

Same as TASK01. Uses `requests`/`httpx` for POST and
`sseclient` for SSE streaming.

## Acceptance Criteria

- [ ] Unary RPC via POST
- [ ] Server-streaming via SSE
- [ ] Cross-language interop verified
- [ ] `pytest` — zero failures

- [ ] Verify with `op run` over REST + SSE
## Dependencies

TASK01, TASK02.
