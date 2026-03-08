# TASK012 — HolonMeta Proto + Reference Implementation in `go-holons`

## Context

Every holon should be self-documenting. The `HolonMeta` service exposes a
`Describe` RPC that returns the holon's API catalog in plain English —
method names, purposes, input/output fields with descriptions.

This is **auto-registered** by the SDK's `serve` runner. Holon developers
do not write any code for it.

See `PROTOCOL.md` §3.5 for the full proto definition.
See `AGENT.md` Article 2 "Self-documentation via Describe".

## Workspace

- SDK root: `sdk/go-holons/`
- Reference proto: `PROTOCOL.md` §3.5
- Reference serve runner: `sdk/go-holons/pkg/serve/serve.go`

## What to implement

### 1. Create the proto

Create `sdk/go-holons/protos/holonmeta/v1/holonmeta.proto` with the
exact definition from `PROTOCOL.md` §3.5. Generate Go stubs into
`sdk/go-holons/gen/go/holonmeta/v1/`.

### 2. Create `pkg/describe/describe.go`

This module:

1. **Parses `.proto` files** from a given directory tree.
   Use a proto parser library (e.g., `github.com/jhump/protoreflect/desc/protoparse`
   or `github.com/bufbuild/protocompile`) to extract:
   - Service names and comments
   - RPC method names and comments
   - Message field names, types, numbers, and comments
   - `@required` tags in field comments → `FieldDoc.required = true`
   - `@example` tags in field comments → `FieldDoc.example`
   - `@example` tags in RPC comments → `MethodDoc.example_input`

2. **Returns a populated `DescribeResponse`** from the parsed proto data,
   enriched with `slug` and `motto` from `holon.yaml` (use `pkg/identity`).

3. **Implements the gRPC handler**:
   ```go
   type metaServer struct {
       holonmetapb.UnimplementedHolonMetaServer
       response *holonmetapb.DescribeResponse
   }

   func (s *metaServer) Describe(ctx context.Context, req *holonmetapb.DescribeRequest) (*holonmetapb.DescribeResponse, error) {
       return s.response, nil
   }
   ```

4. **Provides a `Register` function** for use by `serve.Run`:
   ```go
   func Register(s *grpc.Server, protoDir string, holonYamlPath string) error
   ```

### 3. Auto-register in `serve.Run`

Modify `pkg/serve/serve.go` so that `Run` and `RunWithOptions` automatically
register `HolonMeta` if a `protos/` directory and `holon.yaml` exist in the
current working directory. If neither exists, skip silently.

The auto-registration must:
- Parse protos from `./protos/` (relative to cwd)
- Read identity from `./holon.yaml`
- Call `describe.Register(server, "./protos/", "./holon.yaml")`
- Exclude `holonmeta.v1.HolonMeta` from its own output

### 4. Add tests

- Parse the echo-server's proto (`protos/echo/v1/echo.proto`) and verify
  the `DescribeResponse` contains correct service name, method name,
  and comments.
- Start a serve runner with `HolonMeta` auto-registered, call `Describe`,
  verify the response.
- Test graceful degradation: no `protos/` dir → no error, empty services list.

## Rules

- Follow existing code style in `pkg/connect/connect.go` and `pkg/serve/serve.go`.
- Do not modify the holon developer's code path — `HolonMeta` is invisible to them.
- Run `go test ./...` — all existing tests must still pass.
- Run `go vet ./...` — no new warnings.
