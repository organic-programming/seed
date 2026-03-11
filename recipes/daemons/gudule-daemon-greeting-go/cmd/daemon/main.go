// greeting-daemon — greets users in 56 languages.
package main

import (
	"fmt"
	"os"

	"github.com/organic-programming/go-holons/pkg/serve"
	pb "github.com/organic-programming/seed/recipes/daemons/gudule-daemon-greeting-go/gen/go/greeting/v1"
	"github.com/organic-programming/seed/recipes/daemons/gudule-daemon-greeting-go/internal/server"

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
			pb.RegisterGreetingServiceServer(gs, &server.Server{})
		}); err != nil {
			fmt.Fprintf(os.Stderr, "serve error: %v\n", err)
			os.Exit(1)
		}
	case "version":
		fmt.Println("gudule-daemon-greeting-go v0.4.1")
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: daemon <serve|version> [flags]")
	fmt.Fprintln(os.Stderr, "  serve   Start the gRPC server (--listen <uri>)")
	fmt.Fprintln(os.Stderr, "  version Print version and exit")
	os.Exit(1)
}
