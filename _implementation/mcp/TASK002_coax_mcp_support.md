# TASK — `op mcp` COAX Support (URI Mode)

> [!IMPORTANT]
> **Prerequisite:** `TASK001_migrate_mcp_to_describe.md` must be completed first.
> That task migrates `op mcp` from local proto files to `Describe`.
> This task adds a new connection mode on top of that shared pipeline.

## Summary

After the `Describe` migration, `op mcp` already connects to holons
and calls `Describe` to build MCP tools.  This task adds a second
connection mode: connecting to an **already-running gRPC server** via
URI instead of starting a holon by slug.

```
op mcp gabriel-greeting-go                    # slug: start + connect + Describe
op mcp grpc+tcp://gabriel-greeting-app:64749  # URI: connect + Describe (server already running)
```

Both share the same schema pipeline (`DefinitionsFromDescribe`,
`BuildProtobuf`, `DecodeProtobuf`).

## Authoritative References

- `apps_kits/DESIGN.md` § "COAX + MCP: Agent-Native Access"
- `_protos/holons/v1/coax.proto` — `CoaxService` definition
- `_implementation/mcp/TASK001_migrate_mcp_to_describe.md` — prerequisite task
- `holons/grace-op/internal/mcp/server.go` — MCP server (post-migration)

## Implementation

### 1. Detect URI vs slug in `cmdMCP`

**File:** `holons/grace-op/internal/cli/mcp.go`

```go
if len(args) == 1 && strings.Contains(args[0], "://") {
    server, err = mcppkg.NewServerFromURI(args[0], version)
} else {
    server, err = mcppkg.NewServer(args, version)
}
```

### 2. Add `NewServerFromURI`

**File:** `holons/grace-op/internal/mcp/server_remote.go` (new)

```go
func NewServerFromURI(uri string, version string) (*Server, error) {
    // 1. Parse URI (grpc+tcp://host:port → host:port)
    // 2. grpc.Dial(address, plaintext)
    // 3. Call HolonMeta/Describe
    // 4. DefinitionsFromDescribe(slugFromResp, resp)  ← same as slug mode
    // 5. Build Server with conn + tools
}
```

The slug is derived from `DescribeResponse.slug` — the running server
identifies itself.

### 3. Update help text

**File:** `holons/grace-op/internal/cli/commands.go`

```
op mcp <slug> [slug2...]               start an MCP server for holons
op mcp grpc+tcp://<host>:<port>        start an MCP server for a running COAX server
```

## Acceptance Criteria

- [ ] `op mcp grpc+tcp://localhost:64749` connects, calls Describe,
      exposes all services as MCP tools
- [ ] `tools/list` returns CoaxService + domain service tools
- [ ] `tools/call` works via Dynamic Dispatch
- [ ] No regression on `op mcp gabriel-greeting-go`
- [ ] `op mcp --help` shows both usage modes

## Verification

```bash
# Start the Greeting App with COAX enabled
op run gabriel-greeting-app-swiftui
# Enable COAX toggle, note the listen URI

# MCP bridge to COAX server
op mcp grpc+tcp://gabriel-greeting-app-swiftui:64749

# tools/list → CoaxService + GreetingAppService tools
# tools/call Greet → greeting response (visible in app UI)
```

## Constraints

- **Do NOT modify** `_protos/holons/v1/describe.proto` or `coax.proto`
- Reuse the pipeline from the Describe migration — no parallel code path
- Follow existing code style in `server.go`
