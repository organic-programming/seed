// Package cli implements the hello-world command-line interface.
package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/organic-programming/examples/hello-world/pkg/server"
)

// Run processes command-line arguments and returns an exit code.
func Run(args []string) int {
	if len(args) < 1 {
		PrintUsage()
		return 0
	}

	cmd := args[0]
	rest := args[1:]

	switch cmd {
	case "greet":
		return cmdGreet(rest)
	case "serve":
		return cmdServe(rest)
	case "version":
		fmt.Println("hello-world v0.1.0")
		return 0
	case "help", "--help", "-h":
		PrintUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "hello: unknown command %q\n", cmd)
		PrintUsage()
		return 1
	}
}

// PrintUsage displays the help text.
func PrintUsage() {
	fmt.Print(`hello — the simplest possible holon

Commands:
  hello greet [name]                     greet someone (default: World)
  hello serve [--listen <URI>]           start the gRPC server
  hello version                          show version
  hello help                             this message
`)
}

func cmdGreet(args []string) int {
	name := "World"
	if len(args) > 0 {
		name = strings.Join(args, " ")
	}
	fmt.Printf("Hello, %s!\n", name)
	return 0
}

func cmdServe(args []string) int {
	uri := "tcp://:9090"
	reflection := true

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--listen":
			if i+1 < len(args) {
				uri = args[i+1]
				i++
			}
		case "--no-reflection":
			reflection = false
		}
	}

	fmt.Fprintf(os.Stderr, "hello: serving on %s\n", uri)
	if err := server.ListenAndServe(uri, reflection); err != nil {
		fmt.Fprintf(os.Stderr, "hello: %v\n", err)
		return 1
	}
	return 0
}
