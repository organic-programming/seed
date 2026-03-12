# TASK02 — Middleware Chain

## Context

The proxy must apply a chain of interceptors to each RPC for
observation and mutation. Middleware executes in order on requests
and reverse order on responses.

## Objective

Implement the middleware interface and chain execution.

## Changes

### `internal/middleware/middleware.go` [NEW]

```go
// Interceptor is a middleware function applied to proxied RPCs.
type Interceptor func(ctx context.Context, call *Call, next Handler) error

// Call captures the RPC metadata visible to middleware.
type Call struct {
    FullMethod string
    StartTime  time.Time
    Request    []byte       // raw protobuf frame
    Response   []byte       // filled after next()
    StatusCode codes.Code
    Metadata   metadata.MD
    Duration   time.Duration
}

// Handler is the next step in the chain.
type Handler func(ctx context.Context, call *Call) error

// Chain builds a Handler from a list of Interceptors and a
// terminal Handler (the actual proxy relay).
func Chain(interceptors []Interceptor, terminal Handler) Handler
```

### `internal/middleware/middleware_test.go` [NEW]

- `TestChainOrder` — verify interceptors execute in order
- `TestChainReverseOnResponse` — verify response path is reversed
- `TestEmptyChain` — terminal handler called directly

## Acceptance Criteria

- [ ] Interceptors execute in registration order on the request path
- [ ] Response path executes in reverse order
- [ ] Terminal handler (proxy relay) receives final call
- [ ] Empty chain passes through to terminal

## Dependencies

TASK01 (proxy core provides the terminal handler).
