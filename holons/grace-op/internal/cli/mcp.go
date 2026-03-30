package cli

import (
	"context"
	"fmt"
	"os"

	mcppkg "github.com/organic-programming/grace-op/internal/mcp"
)

func cmdMCP(args []string, version string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "op mcp: requires at least one <slug> or URI")
		return 1
	}

	var server *mcppkg.Server
	var err error

	if len(args) == 1 && mcppkg.IsURI(args[0]) {
		server, err = mcppkg.NewServerFromURI(args[0], version)
	} else {
		server, err = mcppkg.NewServer(args, version)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "op mcp: %v\n", err)
		return 1
	}
	defer func() { _ = server.Close() }()

	if err := server.ServeStdio(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "op mcp: %v\n", err)
		return 1
	}
	return 0
}
