package main

import (
	"fmt"
	"os"

	"github.com/organic-programming/go-holons/pkg/serve"
	pb "github.com/organic-programming/seed/recipes/composition/workers/charon-worker-transform/gen/go/transform/v1"
	"github.com/organic-programming/seed/recipes/composition/workers/charon-worker-transform/internal/server"

	"google.golang.org/grpc"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	switch os.Args[1] {
	case "serve":
		listenURI := serve.ParseFlags(os.Args[2:])
		if err := serve.Run(listenURI, func(gs *grpc.Server) {
			pb.RegisterTransformServiceServer(gs, &server.Server{})
		}); err != nil {
			fmt.Fprintf(os.Stderr, "serve error: %v\n", err)
			os.Exit(1)
		}
	case "version":
		fmt.Println("charon-worker-transform v0.4.3")
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: charon-worker-transform <serve|version> [flags]")
	os.Exit(1)
}
