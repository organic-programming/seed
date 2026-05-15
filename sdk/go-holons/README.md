# go-holons

Go reference SDK for holons.

## serve

```go
package main

import (
	"log"
	"os"

	"my-holon/gen"
	pb "my-holon/gen/go/my_service/v1"
	"github.com/organic-programming/go-holons/pkg/describe"
	"github.com/organic-programming/go-holons/pkg/serve"
	"google.golang.org/grpc"
)

func init() {
	describe.UseStaticResponse(gen.StaticDescribeResponse())
}

func main() {
	options := serve.ParseOptions(os.Args[1:])
	if err := serve.RunCLIOptions(options, func(s *grpc.Server) {
		pb.RegisterMyServiceServer(s, &server{})
	}); err != nil {
		log.Fatal(err)
	}
}
```

## transport

Choose the listener with `--listen`, for example `tcp://127.0.0.1:9090`, `unix:///tmp/gabriel.sock`, `stdio://`, `ws://127.0.0.1:8080/grpc`, or `wss://127.0.0.1:8443/grpc?cert=/path/cert.pem&key=/path/key.pem`.

For native JSON-RPC over HTTP+SSE, use `http://` or `https://` URIs, e.g. `--listen http://127.0.0.1:0/api/v1/rpc`.

Repeat `--listen` to expose both gRPC and HTTP+SSE from one process, for example `--listen tcp://127.0.0.1:9090 --listen http://127.0.0.1:8080/api/v1/rpc`.

## identity / describe

Wire the generated Incode Description with one line:

```go
func init() { describe.UseStaticResponse(gen.StaticDescribeResponse()) }
```

At build time, `op build` generates `gen/describe_generated.go`; at runtime, `serve` will fail fast with `no Incode Description registered — run op build` if that static response is missing.

## discover

```go
entry, err := discover.FindBySlug("gabriel-greeting-go")
```

## connect

```go
conn, err := connect.Connect("gabriel-greeting-go")
```

## Transitive observability

Every parent→child connection is transitive by default: spawning a
child via `composite.SpawnMember` opens long-lived
`HolonObservability.Logs(follow=true)` and `Events(follow=true)`
streams in background and republishes received entries into the
parent's local rings, appending a `ChainHop` for the child.
Peer-to-peer `composite.Dial` defaults to OFF; opt-in explicitly
when needed.

```go
// Default-ON: SpawnMember owns the child's lifecycle. DialOptions
// forwards dial-level options onto the connection the spawn opens;
// pass WithTransitiveObservability(false) to keep one member silent
// without affecting siblings.
silent, err := composite.SpawnMember(ctx, composite.SpawnOptions{
    Slug:       "gabriel-greeting-go",
    BinaryPath: "/abs/path/to/gabriel-greeting-go",
    DialOptions: []composite.DialOption{
        composite.WithTransitiveObservability(false),
    },
})
defer silent.Stop(ctx)

// composite.Dial takes a concrete address — tcp://host:port,
// unix:///path/to/socket, or host:port. Slug-based discovery is the
// caller's responsibility; resolve the slug via connect.Connect or a
// manifest lookup and pass the resulting address here. Transitivity
// is OFF by default on this peer-to-peer entry point; opt-in to tail
// a remote holon's streams.
peer, err := composite.Dial(ctx, "tcp://127.0.0.1:9090",
    composite.WithTransitiveObservability(true))
defer peer.Close()
```

Closing the connection returned by `composite.Dial` closes the relay
streams and lets the relay goroutines exit. Because `Dial` returns
`*grpc.ClientConn` directly, callers that leak the connection keep
the relay alive too.

Shutdown is bounded. `SpawnedMember.Stop(ctx)` enforces a 3-second
default deadline when the supplied context has none, killing the
child process if it does not exit cleanly within that window.
`observability.Relay.Stop()` bounds its internal `wg.Wait` at
2 seconds so a slow transport surfacing the canceled stream to `Recv`
cannot block process shutdown forever. Both invariants are
defensive — the visible API is unchanged.

See [OBSERVABILITY.md §Transitive Observability](../../OBSERVABILITY.md#transitive-observability)
for the full doctrine (defaults, peer-vs-spawn rules, per-emission
`Private()` opt-out, subscription replay-then-live, v1 metrics
non-coverage).

## Build and test

```sh
go build ./...
go test ./...
```
