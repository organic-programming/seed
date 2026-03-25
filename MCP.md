# MCP Support is provided by `op`

`op mcp` exposes any holon as an
[MCP](https://modelcontextprotocol.io/) server, letting AI agents
call holon RPCs as tools and read holon skills as prompts.

## Quick Start

```bash
# Single holon (starts it via connect)
op mcp gabriel-greeting-go

# Multiple holons — tools are namespaced by slug
op mcp gabriel-greeting-go rob-go

# Remote COAX server via URI
op mcp grpc+tcp://127.0.0.1:60000
```

The slug-based mode starts the holon via `connect(slug)`, calls
`HolonMeta/Describe`, and builds the MCP tool surface from the
runtime `DescribeResponse`. The URI mode connects to an
already-running gRPC server directly. Connections are cached for
the lifetime of the MCP session.

## Walkthrough: COAX Organism via MCP

This example shows an AI agent greeting Bob through a running
SwiftUI organism, entirely via MCP.

**1. Launch the organism**

```bash
op run gabriel-greeting-app-swiftui
```

**2. Enable COAX from the UI**

In the SwiftUI app, tap **Enable COAX** and set the listen
address to `tcp://127.0.0.1:60000`.

**3. Bridge COAX to MCP**

```bash
op mcp grpc+tcp://127.0.0.1:60000
```

This connects to the running organism, calls `HolonMeta/Describe`,
and exposes all COAX + app services as MCP tools.

**4. Agent discovers tools**

The agent sends `tools/list` and receives all available tools:

```
Gabriel-Greeting-App-SwiftUI.GreetingAppService.Greet
Gabriel-Greeting-App-SwiftUI.GreetingAppService.SelectHolon
Gabriel-Greeting-App-SwiftUI.GreetingAppService.SelectLanguage
Gabriel-Greeting-App-SwiftUI.CoaxService.ConnectMember
Gabriel-Greeting-App-SwiftUI.CoaxService.ListMembers
Gabriel-Greeting-App-SwiftUI.CoaxService.Tell
...
```

Each tool includes a full JSON Schema with descriptions, required
fields, and examples — all derived from the proto definitions.

**5. Greet Bob**

The agent calls `tools/call`:

```json
{
  "method": "tools/call",
  "params": {
    "name": "Gabriel-Greeting-App-SwiftUI.GreetingAppService.Greet",
    "arguments": { "name": "Bob" }
  }
}
```

Response:

```json
{
  "result": {
    "content": [{ "type": "text", "text": "{\"greeting\": \"Hello Bob\"}" }],
    "structuredContent": { "greeting": "Hello Bob" }
  }
}
```

## MCP Protocol Version

`op mcp` implements the
[2025-06-18](https://modelcontextprotocol.io/specification/2025-06-18)
revision of the Model Context Protocol.

## Supported MCP Methods

| Method | Status | Notes |
|--------|--------|-------|
| `initialize` | ✅ | Returns tools + prompts in initial response |
| `ping` | ✅ | |
| `tools/list` | ✅ | |
| `tools/call` | ✅ | Unary RPCs; streaming RPCs rejected gracefully |
| `prompts/list` | ✅ | Skills from `holon.proto` manifest |
| `prompts/get` | ✅ | |
| `notifications/initialized` | ✅ | Acknowledged silently |
| `resources/*` | ❌ | Not implemented |
| `sampling/*` | ❌ | Not implemented |

**Resources** let an MCP server expose readable content (files, data, state) that clients can browse and retrieve.
**Sampling** lets an MCP server ask the client's AI model to generate text, enabling agentic loops where the holon drives the conversation.

## Transports

### stdio (current)

Newline-delimited JSON-RPC over stdin/stdout, per the
[MCP stdio spec](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports#stdio).

This is the transport used by Claude Desktop, Cursor, Windsurf,
Codex, and other local MCP clients. The client launches `op mcp`
as a subprocess and communicates over pipes.

```json
// MCP client configuration example (Claude Desktop)
{
  "mcpServers": {
    "greeting": {
      "command": "op",
      "args": ["mcp", "gabriel-greeting-go"]
    }
  }
}
```

### Streamable HTTP

The [MCP 2025-06-18 spec](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports#streamable-http)
defines a second transport: **Streamable HTTP**, where clients
POST JSON-RPC requests to an HTTP endpoint and receive responses
as `application/json`.

This transport enables two scenarios that stdio cannot serve:

1. **Remote MCP** — AI agents running on a different machine
   from the holon (cloud-hosted agents, multi-user setups)
2. **Browser-native MCP** — web-based AI clients that cannot
   spawn subprocesses

#### CLI Surface

```bash
# stdio (default)
op mcp gabriel-greeting-go

# Streamable HTTP on a specific port
op mcp gabriel-greeting-go --listen http://127.0.0.1:8080
```

```bash
# Test with curl
curl -X POST http://127.0.0.1:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
```

#### Implementation Status

| Capability | Status |
|---|---|
| `POST /mcp` → JSON response | ✅ |
| `Origin` validation (localhost) | ✅ |
| Localhost binding by default | ✅ |
| `GET /mcp` → SSE stream | ❌ Not yet — no server-initiated events |
| Session management (`Mcp-Session-Id`) | ❌ Not yet |
| Resumability (`Last-Event-ID`) | ❌ Not yet |

#### Relationship to `op proxy`

For advanced scenarios (TLS, auth, multi-tenant), Streamable HTTP
MCP can be fronted by `op proxy` rather than embedding all of
that into `op mcp` itself:

```bash
# op proxy adds TLS + auth on top of the stdio MCP
op proxy gabriel-greeting-go --as mcp --listen https://:8443
```

This keeps `op mcp` focused on the MCP protocol while `op proxy`
handles transport-level concerns.

## Tool Naming

Tools are namespaced to avoid collisions when multiple holons
are loaded:

```
<slug>.<ServiceShortName>.<MethodName>
```

Examples:
- `gabriel-greeting-go.GreetingService.SayHello`
- `gabriel-greeting-go.GreetingService.ListLanguages`
- `rob-go.RobGoService.Build`

Sequence tools follow a separate pattern:

```
<slug>.sequence.<sequence-name>
```

Example: `gabriel-greeting-go.sequence.multilingual-greeting`

## Tool Schema

MCP tool schemas are derived from the `DescribeResponse` at
runtime. The mapping from `FieldDoc` to JSON Schema:

| Proto type | JSON Schema |
|---|---|
| `string` | `"string"` |
| `bytes` | `"string"` (base64) |
| `int32`, `int64`, `uint32`, `uint64` | `"integer"` |
| `float`, `double` | `"number"` |
| `bool` | `"boolean"` |
| message types | `"object"` with nested `properties` |
| enum types | `"string"` with `enum` values |
| `FIELD_LABEL_REPEATED` | `"array"` with `items` |
| `FIELD_LABEL_MAP` | `"object"` with `additionalProperties` |

Proto comment annotations flow through to JSON Schema:
- `@required` → `required` array
- `@example` → `examples`
- Leading comment → `description`

## Prompts

Skills declared in the `holon.proto` manifest are exposed as
MCP prompts. Each prompt includes:

- The holon slug and skill description
- When/steps metadata from the manifest
- A list of all available tools for that holon

## Invocation Pipeline

Tool calls use the existing `grpcclient.InvokeConn` path, which
resolves methods via `HolonMeta/Describe` and handles dynamic
protobuf serialization:

```
tools/call request
  → lookup cached connection + DescribeResponse
  → grpcclient.InvokeConn (Describe-based dynamic dispatch)
  → gRPC invoke on cached connection
  → JSON response
```

Streaming RPCs (`client_streaming` or `server_streaming`) are
detected from the `DescribeResponse` and rejected with a clear
error message — MCP's request/response model does not support
streaming.
