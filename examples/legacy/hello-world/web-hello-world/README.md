# web-hello-world

Browser client invoking a Go holon via WebSocket. This example
demonstrates the full **browser → WebSocket → Go** pipeline using
`go-holons/pkg/transport.WebBridge` and `js-web-holons`.

WebBridge (not `holonrpc.Server`) is used here because the HTTP server
also serves static files — WebBridge embeds into an existing `http.ServeMux`.

## Architecture

```
┌──────────────┐   ws://:8080/ws   ┌──────────────────────────────┐
│  Browser     │ ◄──────────────►  │  Go HTTP server              │
│  index.html  │  holon-rpc JSON   │  ┌──────────┐  ┌──────────┐ │
│  holons.mjs  │                   │  │ WebBridge │  │ static/  │ │
└──────────────┘                   │  │ /ws       │  │ /        │ │
                                   │  └──────────┘  └──────────┘ │
                                   └──────────────────────────────┘
```

## Run

```bash
go run main.go
# → listening on http://localhost:8080
```

Open [http://localhost:8080](http://localhost:8080) in your browser.

## How it works

1. `main.go` creates a `transport.WebBridge` and registers the
   `hello.v1.HelloService/Greet` handler — a pure JSON-in/JSON-out
   function with no proto dependency.

2. An `http.ServeMux` mounts the bridge at `/ws` and a file server
   at `/` for the static assets.

3. The browser loads `index.html`, which imports `holons.mjs`
   (the `js-web-holons` SDK). On load, it connects to `ws://host/ws`
   using the `holon-rpc` subprotocol.

4. When the user clicks **Invoke Greet**, the SDK sends:
   ```json
   { "jsonrpc": "2.0", "id": "1", "method": "hello.v1.HelloService/Greet", "params": {"name":"Bob"} }
   ```

5. The WebBridge dispatches to the registered handler, which returns:
   ```json
   { "jsonrpc": "2.0", "id": "1", "result": {"message":"Hello, Bob!"} }
   ```

6. The SDK resolves the promise and the page displays the greeting.

## Files

| File | Purpose |
|------|---------|
| `main.go` | Go server — WebBridge + static file server |
| `static/index.html` | Browser UI — dark-themed card with input and result |
| `static/holons.mjs` | `js-web-holons` SDK (synced copy of `sdk/js-web-holons/src/index.mjs`) |
| `go.mod` | Go module with local replace to `go-holons` |

## Keep SDK in sync

```bash
# From repo root
bash scripts/sync-web-sdk.sh sync
bash scripts/sync-web-sdk.sh check
```

## Test

```bash
go build -o web-hello-world .
./web-hello-world
# Open http://localhost:8080, type a name, click Invoke Greet
```

## SDK Sanity Checks

```bash
# From repo root
# Browser SDK tests
(cd sdk/js-web-holons && npm test)

# Go WebBridge tests
(cd sdk/go-holons && go test ./pkg/transport -run WebBridge -count=1)
```

## Invoke via stdio (zero config)

This example is browser-based — it does not support stdio invocation.
Use the standard `go-hello-world` example for CLI/stdio usage.
