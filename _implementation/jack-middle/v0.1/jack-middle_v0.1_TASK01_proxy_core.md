# TASK01 — Transparent gRPC Proxy Core

## Context

Jack Middle must forward arbitrary gRPC calls without knowing
the target holon's proto definitions. Go's grpc package provides
`grpc.UnknownServiceHandler` for exactly this purpose.

## Objective

Implement the core proxy that accepts any gRPC call on a
frontend listener and forwards it to a backend connection.

## Changes

### `internal/proxy/proxy.go` [NEW]

```go
// Proxy forwards all gRPC traffic between a frontend listener
// and a backend connection.
type Proxy struct {
    backend *grpc.ClientConn
}

// Handler returns a grpc.StreamHandler suitable for use with
// grpc.UnknownServiceHandler. It relays frames bidirectionally.
func (p *Proxy) Handler(srv interface{}, stream grpc.ServerStream) error
```

Key mechanics:
- Extract full method name from `grpc.ServerTransportStream`
- Open a `ClientStream` to the backend with the same method
- Copy request frames: client → backend
- Copy response frames: backend → client
- Propagate headers, trailers, and status codes

### `internal/proxy/proxy_test.go` [NEW]

- `TestUnaryRelay` — proxy a unary RPC, verify request/response match
- `TestStreamingRelay` — proxy a server-streaming RPC
- `TestMetadataPropagation` — verify headers/trailers pass through
- `TestBackendError` — verify error codes forwarded correctly

## Acceptance Criteria

- [ ] Unary RPCs forwarded correctly (request + response)
- [ ] Server-streaming RPCs forwarded frame by frame
- [ ] gRPC metadata (headers, trailers) propagated
- [ ] Backend errors forwarded with correct status codes
- [ ] No proto definitions needed for target holon

## Dependencies

None.
