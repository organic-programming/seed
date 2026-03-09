# TASK002 — `op mcp` and `op tools` Commands

## Context

Depends on: TASK001 (`op inspect` — provides the proto parser).

`op mcp` starts an MCP server that exposes any holon's RPCs as MCP
tools. `op tools` outputs LLM tool definitions in various formats.
Together, they make **every holon in the ecosystem instantly
compatible with AI agents** — without any code changes to the holons.

See `OP.md` §14 "Introspection" for the full specification.

## Workspace

- `op` source: `holons/grace-op/`
- Proto parser: `pkg/inspect/` (from TASK001)
- New modules: `pkg/mcp/`, `pkg/tools/`
- MCP spec: https://spec.modelcontextprotocol.io/

## What to implement

### `op mcp <slug> [slug2...]`

Start an MCP server over stdio that exposes holon RPCs as MCP tools.

```bash
op mcp rob-go                        # single holon
op mcp rob-go jess-npm echo-server   # multiple holons
```

#### MCP server implementation

Create `pkg/mcp/server.go`:

1. **Parse protos** — reuse `pkg/inspect/` parser for each slug.
2. **Generate tool definitions** — for each RPC method:
   - `name`: `<slug>.<ServiceName>.<MethodName>`
     (e.g. `rob-go.RobGoService.Build`)
   - `description`: from proto comment
   - `inputSchema`: JSON Schema generated from the proto field tree
     (map proto types → JSON Schema types, mark `@required` fields,
     include `@example` as schema examples)
3. **Start stdio MCP server** — implement the MCP protocol:
   - `initialize` → return server info and tool list
   - `tools/list` → return generated tool definitions
   - `tools/call` → receive JSON args → translate to gRPC →
     `connect(slug)` → call RPC → return JSON result
4. **Connect on demand** — use `connect.Connect(slug)` when a tool
   is called, `connect.Disconnect` after the response.

#### JSON Schema generation

Create `pkg/tools/jsonschema.go`:

Map proto types to JSON Schema:

| Proto type | JSON Schema type |
|-----------|-----------------|
| `string` | `{"type": "string"}` |
| `int32`, `int64` | `{"type": "integer"}` |
| `float`, `double` | `{"type": "number"}` |
| `bool` | `{"type": "boolean"}` |
| `bytes` | `{"type": "string", "format": "byte"}` |
| enum | `{"type": "string", "enum": [...values]}` |
| message | `{"type": "object", "properties": {...}}` |
| repeated T | `{"type": "array", "items": <T>}` |
| map<K,V> | `{"type": "object", "additionalProperties": <V>}` |

Populate `required` array from `@required`-tagged fields.
Populate `examples` from `@example`-tagged fields.

### `op tools <slug>`

Output LLM tool definitions without starting an MCP server.

```bash
op tools rob-go                      # default (OpenAI format)
op tools rob-go --format openai
op tools rob-go --format anthropic
op tools rob-go --format mcp
```

Create `pkg/tools/format.go`:

- **OpenAI**: `{"type": "function", "function": {"name": ..., "description": ..., "parameters": <json_schema>}}`
- **Anthropic**: `{"name": ..., "description": ..., "input_schema": <json_schema>}`
- **MCP**: `{"name": ..., "description": ..., "inputSchema": <json_schema>}`

Reuse the JSON Schema generator from `pkg/tools/jsonschema.go`.

## Testing

1. Parse echo-server protos, generate JSON Schema, verify correctness.
2. Start `op mcp echo-server`, simulate MCP `tools/list` request,
   verify tool definitions include correct names, descriptions, schemas.
3. Start `op mcp echo-server`, simulate MCP `tools/call` for Ping,
   verify the gRPC call is made and response is returned as JSON.
4. Test `op tools echo-server --format openai`, verify valid JSON output.
5. Test multi-holon: `op mcp echo-server rob-go`, verify both holons'
   tools appear in the tool list.
6. Verify skills from `holon.yaml` are exposed as MCP prompts.

## Rules

- The MCP server uses stdio transport (per MCP specification).
- JSON Schema generation is `op`'s responsibility — holons never see it.
- Skills from `holon.yaml` are exposed as MCP prompts alongside tools.
- Reuse `pkg/inspect/` parser from TASK001 — do not duplicate parsing.
- Follow existing `op` CLI code style in Grace.
