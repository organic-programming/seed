package cli

import (
	"fmt"
	"os"
	"strings"

	inspectpkg "github.com/organic-programming/grace-op/internal/inspect"
	toolspkg "github.com/organic-programming/grace-op/internal/tools"
)

func cmdTools(_ Format, args []string) int {
	format, target, err := parseToolsArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "op tools: %v\n", err)
		return 1
	}

	catalog, err := inspectpkg.LoadLocal(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "op tools: %v\n", err)
		return 1
	}

	payload, err := toolspkg.MarshalDefinitions(
		toolspkg.DefinitionsForCatalogs([]*inspectpkg.LocalCatalog{catalog}),
		format,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "op tools: %v\n", err)
		return 1
	}

	fmt.Println(string(payload))
	return 0
}

func parseToolsArgs(args []string) (string, string, error) {
	format := toolspkg.FormatOpenAI
	positional := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--format":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("--format requires a value")
			}
			parsed, err := toolspkg.ParseFormat(args[i+1])
			if err != nil {
				return "", "", err
			}
			format = parsed
			i++
		case strings.HasPrefix(args[i], "--format="):
			parsed, err := toolspkg.ParseFormat(strings.TrimPrefix(args[i], "--format="))
			if err != nil {
				return "", "", err
			}
			format = parsed
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) != 1 {
		return "", "", fmt.Errorf("requires exactly one <slug>")
	}
	return format, positional[0], nil
}
