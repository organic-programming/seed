package api

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	_ "observability-cascade-go-node/internal"

	"github.com/organic-programming/go-holons/pkg/composite"
	"github.com/organic-programming/go-holons/pkg/observability"
	"github.com/organic-programming/go-holons/pkg/relay"
	"github.com/organic-programming/go-holons/pkg/serve"
	"google.golang.org/grpc"
)

// RunCLI dispatches the observability-cascade-go-node CLI and returns a process exit code.
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
		children, remaining := serve.ParseChildFlags(args[1:])
		options := serve.ParseOptions(remaining)
		transportName := parseTransport(remaining)
		observability.FromEnv(observability.Config{})
		var downstream *composite.SpawnedMember
		if len(children) > 0 {
			var err error
			downstream, err = composite.SpawnMember(context.Background(), composite.SpawnOptions{
				Slug:            children[0].Slug,
				BinaryPath:      children[0].Binary,
				Transport:       transportName,
				DownstreamChain: children[1:],
			})
			if err != nil {
				fmt.Fprintf(stderr, "serve: %v\n", err)
				return 1
			}
			defer downstream.Stop(context.Background()) //nolint:errcheck
		}
		if err := serve.RunCLIOptions(options, func(s *grpc.Server) {
			var conn *grpc.ClientConn
			if downstream != nil {
				conn = downstream.Conn
			}
			relay.RegisterServer(s, relay.RelayOptions{DownstreamConn: conn})
		}); err != nil {
			if downstream != nil {
				_ = downstream.Stop(context.Background())
			}
			fmt.Fprintf(stderr, "serve: %v\n", err)
			return 1
		}
		return 0
	case "version":
		fmt.Fprintf(stdout, "observability-cascade-go-node %s\n", VersionString())
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

func parseTransport(args []string) string {
	for i, arg := range args {
		if arg == "--transport" && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(arg, "--transport=") {
			return strings.TrimPrefix(arg, "--transport=")
		}
	}
	return "stdio"
}

func canonicalCommand(raw string) string {
	replacer := strings.NewReplacer("-", "", "_", "", " ", "")
	return replacer.Replace(strings.ToLower(strings.TrimSpace(raw)))
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: observability-cascade-go-node <command> [args] [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  serve [--listen <uri>] [--transport <name>] [--child <slug>=<binary>]  Start the gRPC server")
	fmt.Fprintln(w, "  version                                                           Print version and exit")
	fmt.Fprintln(w, "  help                                                              Print this help")
}
