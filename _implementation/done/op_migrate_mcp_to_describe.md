# TASK — Migrate `op mcp` from Proto Files to `Describe`

## Summary

`op mcp` currently reads local `.proto` files via `inspectpkg.LoadLocal`
to build MCP tool definitions and uses `dynamicpb.NewMessage` with proto
file descriptors to serialize/deserialize messages.

This must change: `op mcp` should use `HolonMeta/Describe` as its
schema source, like all dynamic dispatch
(see COMMUNICATION.md §3.6).

After this migration:
- `op mcp` starts the holon, connects, calls `Describe`, and builds
  MCP tools from the `DescribeResponse`
- No local `.proto` files are read for tool discovery or serialization

## Authoritative References

Read these files **before** making any changes:

- `COMMUNICATION.md` §3.6 — Dynamic Dispatch Workflow
- `holons/grace-op/_protos/v1/describe.proto` — `DescribeResponse`, `FieldDoc`
- `holons/grace-op/internal/mcp/server.go` — current MCP implementation
- `holons/grace-op/internal/inspect/local.go` — `LocalCatalog` (to be removed)
- `holons/grace-op/internal/tools/jsonschema.go` — `Definition` struct

## Current Architecture (to be replaced)

```
cmdMCP(args=["gabriel-greeting-go"])
  └→ NewServer(slugs)
       └→ inspectpkg.LoadLocal(slug)     ← reads local .proto files
            └→ LocalCatalog{Methods, Sequences, Skills}
                 └→ toolspkg.DefinitionsForCatalogs → MCP tool defs
  
  handleToolCall:
    └→ sdkconnect.Connect(slug)           ← starts holon, connects
    └→ dynamicpb.NewMessage(descriptor)   ← needs proto file descriptors
    └→ protojson.Unmarshal(args, inputMsg)
    └→ conn.Invoke(method, inputMsg, outputMsg)
    └→ protojson.Marshal(outputMsg)
```

## Target Architecture

```
cmdMCP(args=["gabriel-greeting-go"])
  └→ NewServer(slugs)
       └→ for each slug:
            └→ sdkconnect.Connect(slug)        ← start holon, connect
            └→ HolonMeta/Describe(conn)        ← one round-trip
            └→ DescribeResponse → tools        ← FieldDocs → JSON Schema
                 └→ cache conn + DescribeResponse per slug
  
  handleToolCall:
    └→ reuse cached conn
    └→ BuildProtobuf(json, inputFields)        ← Dynamic Dispatch (§3.6)
    └→ rawInvoke(conn, fullMethod, bytes)
    └→ DecodeProtobuf(outputBytes, outputFields) → JSON
```

## Implementation

### 1. `DescribeResponse` → MCP tool definitions

**File:** `holons/grace-op/internal/tools/describe.go` (new)

```go
func DefinitionsFromDescribe(slug string, resp *holonmetav1.DescribeResponse) []Definition
```

Map each `MethodDoc` to a `toolspkg.Definition`:
- `name`: `slug.ServiceName.MethodName`
- `description`: `method.Description`
- `inputSchema`: JSON Schema from `method.InputFields`

`FieldDoc` → JSON Schema mapping:

| `FieldDoc.type` | JSON Schema `type` |
|-----------------|-------------------|
| `string` | `"string"` |
| `bytes` | `"string"` (base64) |
| `int32`, `int64`, `uint32`, `uint64`, `sint32`, `sint64` | `"integer"` |
| `float`, `double` | `"number"` |
| `bool` | `"boolean"` |
| `<package.MessageType>` | `"object"` + `properties` from `nested_fields` |
| `<package.EnumType>` | `"string"` + `enum` from `enum_values[].name` |

- `label = FIELD_LABEL_REPEATED` → `"array"` with `items`
- `label = FIELD_LABEL_MAP` → `"object"` with `additionalProperties`
- `required = true` → JSON Schema `required`
- `description` → JSON Schema `description`
- `example` → JSON Schema `examples`

### 2. Dynamic protobuf serialization from `FieldDoc`

**File:** `holons/grace-op/internal/mcp/dynamic.go` (new)

Two functions implementing the Dynamic Dispatch Workflow (§3.6):

```go
// BuildProtobuf encodes JSON arguments to protobuf wire format using FieldDocs.
func BuildProtobuf(args map[string]any, fields []*holonmetav1.FieldDoc) ([]byte, error)

// DecodeProtobuf decodes protobuf wire format to JSON map using FieldDocs.
func DecodeProtobuf(data []byte, fields []*holonmetav1.FieldDoc) (map[string]any, error)
```

Use `FieldDoc.number` + `FieldDoc.type` to determine wire type.
Use `google.golang.org/protobuf/encoding/protowire` for encoding.

### 3. Rewrite `NewServer` to use `Describe`

**File:** `holons/grace-op/internal/mcp/server.go`

Replace `inspectpkg.LoadLocal(slug)` with:

```go
conn, err := sdkconnect.Connect(slug)
client := holonmetav1.NewHolonMetaClient(conn)
resp, err := client.Describe(ctx, &holonmetav1.DescribeRequest{})
tools := toolspkg.DefinitionsFromDescribe(slug, resp)
```

Add `connCache map[string]*grpc.ClientConn` to `Server`.

Remove dependency on `dynamicpb` and `inspectpkg.LoadLocal`.

### 4. Rewrite `handleToolCall` for dynamic dispatch

Replace the `dynamicpb` + `protojson` path:

```go
inputBytes, err := BuildProtobuf(request.Arguments, binding.inputFields)
outputBytes, err := rawInvoke(s.connCache[binding.slug], binding.fullMethod, inputBytes)
result, err := DecodeProtobuf(outputBytes, binding.outputFields)
```

Use `grpc.ForceCodec` with a raw bytes codec for `rawInvoke`.

### 5. Update `toolBinding`

```go
type toolBinding struct {
    slug         string
    definition   toolspkg.Definition
    fullMethod   string                        // "/package.Service/Method"
    inputFields  []*holonmetav1.FieldDoc
    outputFields []*holonmetav1.FieldDoc
    sequence     *inspectpkg.Sequence          // still used for op do
}
```

## Acceptance Criteria

- [ ] `op mcp gabriel-greeting-go` produces identical MCP tool surface
- [ ] No import of `inspectpkg` in the mcp package
- [ ] No import of `dynamicpb` in the mcp package
- [ ] Tool schemas include `@required` and `@example` from FieldDocs
- [ ] MCP test suite passes: `go test ./internal/mcp/... ./internal/cli/...`
- [ ] Existing MCP integration tests pass unchanged

## Verification

```bash
# Start MCP for a local holon
op mcp gabriel-greeting-go

# Send tools/list — tool names and schemas must match current output
# Send tools/call SayHello — greeting response must work

# Run test suite
cd holons/grace-op && go test ./internal/mcp/... ./internal/cli/...
```

## Constraints

- **Do NOT modify** `_protos/holons/v1/describe.proto`
- Follow existing code style in `server.go`
- Keep `server.go` under 600 lines — extract new code to separate files
- Implement this change against the current runtime `Describe` service,
  not a proto migration to `_protos/holons/v1/describe.proto`
