# TASK08 — Plugin Holon Wiring

## Context

Beyond built-in middleware, Jack must support external plugin
holons that implement `middleware.v1.PluginService`. This makes
the middleware chain extensible to any language and any logic.

## Objective

Implement plugin holon connection, lifecycle management, and
integration into the middleware chain.

## Changes

### `protos/middleware/v1/middleware.proto` [NEW]

Define `PluginService` with `Intercept` and `Describe` RPCs
as specified in the design document.

### `internal/plugin/plugin.go` [NEW]

```go
// Plugin represents a connected plugin holon.
type Plugin struct {
    Slug         string
    Conn         *grpc.ClientConn
    Client       pb.PluginServiceClient
    Capabilities []string
}

// ConnectPlugins connects to a list of plugin holon slugs
// via the standard OP connect algorithm.
func ConnectPlugins(slugs []string) ([]*Plugin, error)

// PluginInterceptor wraps a Plugin into an Interceptor
// compatible with the built-in middleware chain.
func (p *Plugin) AsInterceptor() Interceptor
```

The `AsInterceptor` adapter translates between the Go middleware
interface (`Call` → `Handler`) and the gRPC plugin contract
(`InterceptRequest` → `InterceptResponse`), mapping `Verdict`
values to chain control flow.

### `cmd/jack-middle/main.go`

Parse `--plugin slug1,slug2`, call `ConnectPlugins`, append
resulting interceptors after built-in middleware.

### Tests

- `TestPluginConnect` — verify connect to a mock plugin holon
- `TestPluginIntercept` — verify Intercept RPC called per-RPC
- `TestPluginReject` — verify REJECT verdict aborts the chain
- `TestPluginSkip` — verify SKIP verdict jumps to target

## Acceptance Criteria

- [ ] `--plugin snoopy-inspect` connects to the plugin at startup
- [ ] Each proxied RPC calls `Intercept` on connected plugins
- [ ] `FORWARD` passes through, `REJECT` aborts, `SKIP` jumps
- [ ] Plugin failures are logged but don't crash Jack
- [ ] Plugins execute after all built-in middleware

## Dependencies

TASK01 (proxy core), TASK02 (middleware chain).
