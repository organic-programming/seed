# TASK05 — Go: ws Dial in `connect()` + Validate v0.5 SSE+REST

## Summary

Two related completions for `go-holons`:

1. **ws dial** — `connect(slug)` resolves a `ws://` or `wss://` URI but then has
   no ws dial path. The `transport` package already parses ws URIs. Add a
   `DialWS(uri string) (grpc.ClientConnInterface, error)` function so that
   `connect()` can reach ws-served daemons.

2. **SSE+REST validation** — v0.5 delivers the REST+SSE gateway. This task
   validates it end-to-end from `js-web-holons` (browser SDK using `EventSource`)
   through a Go daemon, and ensures the `WebBridge` (`pkg/transport/wsweb.go`)
   integrates cleanly with the SSE endpoint added in v0.5.

## Target

| Feature | Before | After |
|---------|:------:|:-----:|
| ws dial in `connect()` | ❌ | ✅ |
| SSE+REST validate (v0.5) | — | ✅ |

## Implementation

### ws Dial (`pkg/transport/connect.go` or `pkg/connect/connect.go`)

When the resolved daemon URI scheme is `ws` or `wss`:

```go
func DialWS(ctx context.Context, uri string) (*grpc.ClientConn, error) {
    // Use nhooyr.io/websocket as the transport dialer.
    // Wrap the ws.Conn as a net.Conn and pass to grpc.Dial
    // with a custom dialer via grpc.WithContextDialer.
    conn, _, err := websocket.Dial(ctx, uri, &websocket.DialOptions{
        Subprotocols: []string{"grpc"},
    })
    if err != nil {
        return nil, err
    }
    netConn := websocket.NetConn(ctx, conn, websocket.MessageBinary)
    return grpc.NewClient("passthrough:///ws",
        grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
            return netConn, nil
        }),
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
}
```

Integrate into the resolve-and-dial step in `connect.go`.

### SSE+REST Validation (`recipes/testmatrix/` or `sdk/go-holons/pkg/serve/`)

Write a round-trip integration test that:
1. Starts the Go greeting daemon with `--listen tcp://:0`.
2. Connects via the REST+SSE endpoint added by v0.5.
3. Calls `GET /greeting/v1/GreetingService/ListLanguages` (REST transcription).
4. Opens an `EventSource` on `/greeting/v1/GreetingService/SayHelloStream` (SSE).
5. Verifies responses match the gRPC equivalents.

This can be a Go `_test.go` file that uses `net/http` + `bufio.Scanner` for SSE.

## Acceptance Criteria

- [ ] `go test ./pkg/transport/...` includes `TestDialWS` — dial a local ws server (from `TestNewWSListener`) and call any gRPC method
- [ ] `go test ./pkg/serve/...` or `recipes/testmatrix` includes `TestRestSSERoundTrip` — validates v0.5 SSE+REST endpoint
- [ ] `connect("gudule-daemon-greeting-go")` resolves `ws://` URI and returns a ready channel
- [ ] `wsweb.WebBridge` registers next to SSE handler on same `http.ServeMux` without conflict

## Dependencies

`sdk/go-holons` only for the dial changes.  
Requires v0.5 SSE+REST gateway to be delivered first for the validation step.
