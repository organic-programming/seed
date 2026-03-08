# TASK003 — Implement `connect` in `js-web-holons`

## Context

The Organic Programming SDK fleet requires a `connect` module in every SDK.
`js-web-holons` runs in the **browser** — it cannot spawn processes. Its
`connect` is therefore a **reduced variant**: direct dial only (host:port),
no slug resolution, no process start.

The **reference implementation** is `go-holons/pkg/connect/connect.go` — study
it for the general pattern, but only the direct-dial path applies here.

## Workspace

- SDK root: `sdk/js-web-holons/`
- Existing modules: `src/discover.mjs`, `src/index.mjs`, `src/server.mjs`
- Reference: `sdk/go-holons/pkg/connect/connect.go`
- Spec: `sdk/TODO_CONNECT.md` § `js-web-holons`

## What to implement

Create `src/connect.mjs` and re-export from `src/index.mjs`.

### Public API

```javascript
export function connect(hostPort) → GrpcWebClient
export function disconnect(client) → void
```

### Behavior

- `connect("host:port")` → create a gRPC-Web client connection.
- `connect("ws://host:port/rpc")` → create a Holon-RPC (WebSocket JSON-RPC)
  connection if the SDK already supports it via `index.mjs`.
- **No slug resolution.** Browser JS cannot scan filesystems or spawn binaries.
  If someone passes a bare slug, throw an error explaining this limitation.
- `disconnect(client)` → close the connection.

### Why this is limited

Document in a JSDoc comment at the top of `connect.mjs`:

```
Browser environments cannot spawn processes or scan the filesystem.
connect() in js-web-holons only supports direct host:port addressing.
For slug-based resolution, use a Node.js environment with js-holons.
```

## Testing

1. Test `connect("localhost:9090")` returns a client object.
2. Test `connect("my-holon")` (bare slug) throws a descriptive error.
3. Test `disconnect()` on a valid client does not throw.

## Rules

- Follow existing code style in `src/discover.mjs`.
- Use the same import patterns as `src/index.mjs`.
- Do not add Node.js-only dependencies.
