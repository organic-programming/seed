# TASK06 — Enable gRPC Reflection

## Context

The OP conventions mandate gRPC reflection for all holons
exposing a gRPC service. The current `main.go` does not call
`reflection.Register(s)`.

## Objective

Enable gRPC reflection.

## Changes

### `cmd/rob-go/main.go`

Add import and register call:

```go
import "google.golang.org/grpc/reflection"

// inside the register function:
reflection.Register(s)
```

## Acceptance Criteria

- [ ] `reflection.Register(s)` called in `main.go`
- [ ] `op describe` can list `go.v1.GoService` RPCs

## Dependencies

None.
