package api

import (
	"fmt"
	"io"
	"os"
	"strings"

	"cascade-node-go/internal"

	"github.com/organic-programming/go-holons/pkg/serve"
)

// RunCLI dispatches the cascade-node-go CLI and returns a process exit code.
func RunCLI(args []string, outputs ...io.Writer) int {
	stdout := io.Writer(os.Stdout)
	stderr := io.Writer(os.Stderr)
	if len(outputs) > 0 && outputs[0] != nil {
		stdout = outputs[0]
	}
	if len(outputs) > 1 && outputs[1] != nil {
		stderr = outputs[1]
	}

	if len(args) == 0 {
		printUsage(stderr)
		return 1
	}

	switch canonicalCommand(args[0]) {
	case "serve":
		options := serve.ParseOptions(args[1:])
		if err := internal.ListenAndServe(options.ListenURI, options.Reflect); err != nil {
			fmt.Fprintf(stderr, "serve: %v\n", err)
			return 1
		}
		return 0
	case "version":
		fmt.Fprintf(stdout, "cascade-node-go %s\n", VersionString())
		return 0
	case "help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printUsage(stderr)
		return 1
	}
}

func canonicalCommand(raw string) string {
	replacer := strings.NewReplacer("-", "", "_", "", " ", "")
	return replacer.Replace(strings.ToLower(strings.TrimSpace(raw)))
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: cascade-node-go <command> [args] [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  serve [--listen <uri>]  Start the gRPC server")
	fmt.Fprintln(w, "  version                 Print version and exit")
	fmt.Fprintln(w, "  help                    Print this help")
}
