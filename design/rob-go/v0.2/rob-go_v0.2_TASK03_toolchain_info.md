# TASK03 — ToolchainInfo RPC Implementation

## Context

Callers need to query which Go version Rob is running, along
with basic platform info (`GOOS`, `GOARCH`, `CGO_ENABLED`).

## Objective

Implement the server-side handler for `ToolchainInfo`.

## Changes

### `internal/service/service.go`

Add `ToolchainInfo` method to `GoServer`:

```go
func (s *GoServer) ToolchainInfo(ctx context.Context, req *pb.ToolchainInfoRequest) (*pb.ToolchainInfoResponse, error) {
    return &pb.ToolchainInfoResponse{
        Version:    s.toolchain.Version,
        Goroot:     s.toolchain.Root,
        Goos:       runtime.GOOS,
        Goarch:     runtime.GOARCH,
        CgoEnabled: cgoEnabled(),
    }, nil
}
```

### `internal/service/service_test.go`

- `TestToolchainInfo` — verify response fields match toolchain state

## Acceptance Criteria

- [ ] `ToolchainInfo` returns correct version and paths
- [ ] `goos` and `goarch` match `runtime.GOOS`/`runtime.GOARCH`
- [ ] `cgo_enabled` reflects actual CGO state

## Dependencies

TASK01 (proto stubs).
