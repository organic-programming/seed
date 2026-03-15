// gabriel-greeting-go — standalone greeting daemon example.
package main

import (
	"fmt"
	"os"

	"gabriel-greeting-go/internal"

	"github.com/organic-programming/go-holons/pkg/serve"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	switch os.Args[1] {
	case "serve":
		listenURI := serve.ParseFlags(os.Args[2:])
		if err := internal.ListenAndServe(listenURI, true); err != nil {
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
