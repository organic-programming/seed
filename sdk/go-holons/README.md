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

Every parent→child connection is transitive by default: dialing a
child you spawn opens long-lived `HolonObservability.Logs(follow=true)`
and `Events(follow=true)` streams in background and republishes received
entries into the parent's local rings, appending a `ChainHop` for the
child. Peer-to-peer `connect.Connect` defaults to OFF; opt-in
explicitly when needed.

```go
// Opt-out for a single member; the rest of the tree stays observable.
conn, err := composite.Dial(ctx, "gabriel-greeting-go",
    composite.WithTransitiveObservability(false))
```

See [OBSERVABILITY.md §Transitive Observability](../../OBSERVABILITY.md#transitive-observability)
for the full doctrine (defaults, peer-vs-spawn rules, per-emission
`Private()` opt-out, v1 metrics non-coverage).

## Build and test

```sh
go build ./...
go test ./...
```
