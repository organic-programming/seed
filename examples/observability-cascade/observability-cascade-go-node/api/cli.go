package api

import (
	"fmt"
	"io"
	"os"
	"strings"

	"observability-cascade-go-node/internal"

	"github.com/organic-programming/go-holons/pkg/serve"
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
		options := serve.ParseOptions(args[1:])
		members, err := parseMemberRefs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "serve: %v\n", err)
			return 1
		}
		if err := internal.ListenAndServe(options.ListenURI, options.Reflect, members, options.ListenURIs[1:]...); err != nil {
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

func parseMemberRefs(args []string) ([]serve.MemberRef, error) {
	var members []serve.MemberRef
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "--member":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--member requires <slug>=<address>")
			}
			ref, err := parseMemberRef(args[i+1])
			if err != nil {
				return nil, err
			}
			members = append(members, ref)
			i++
		case strings.HasPrefix(arg, "--member="):
			ref, err := parseMemberRef(strings.TrimPrefix(arg, "--member="))
			if err != nil {
				return nil, err
			}
			members = append(members, ref)
		}
	}
	return members, nil
}

func parseMemberRef(raw string) (serve.MemberRef, error) {
	left, address, ok := strings.Cut(raw, "=")
	if !ok {
		return serve.MemberRef{}, fmt.Errorf("--member requires <slug>=<address>")
	}
	slug := strings.TrimSpace(left)
	address = strings.TrimSpace(address)
	if slug == "" || address == "" {
		return serve.MemberRef{}, fmt.Errorf("--member requires non-empty slug and address")
	}
	return serve.MemberRef{Slug: slug, Address: address}, nil
}

func canonicalCommand(raw string) string {
	replacer := strings.NewReplacer("-", "", "_", "", " ", "")
	return replacer.Replace(strings.ToLower(strings.TrimSpace(raw)))
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: observability-cascade-go-node <command> [args] [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  serve [--listen <uri>] [--member <slug>=<address>]  Start the gRPC server")
	fmt.Fprintln(w, "  version                                           Print version and exit")
	fmt.Fprintln(w, "  help                                              Print this help")
}
