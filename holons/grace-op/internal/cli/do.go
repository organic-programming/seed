package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	dopkg "github.com/organic-programming/grace-op/internal/do"
)

func cmdDo(format Format, quiet bool, args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "op do: requires <holon> and <sequence>")
		return 1
	}

	opts := dopkg.Options{
		Params: make(map[string]string),
	}
	if format != FormatJSON {
		if !quiet {
			opts.Progress = os.Stdout
		}
		opts.Stdout = os.Stdout
		opts.Stderr = os.Stderr
	}

	for i := 2; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--dry-run":
			opts.DryRun = true
		case arg == "--continue-on-error":
			opts.ContinueOnError = true
		case strings.HasPrefix(arg, "--"):
			name, value, ok := parseDoParam(arg, args, &i)
			if !ok {
				fmt.Fprintf(os.Stderr, "op do: invalid param flag %q\n", arg)
				return 1
			}
			opts.Params[name] = value
		default:
			fmt.Fprintf(os.Stderr, "op do: unexpected argument %q\n", arg)
			return 1
		}
	}

	result, err := dopkg.Run(args[0], args[1], opts)
	if format == FormatJSON {
		payload := any(result)
		if err != nil {
			payload = struct {
				*dopkg.Result
				Error string `json:"error"`
			}{
				Result: result,
				Error:  err.Error(),
			}
		}
		out, marshalErr := json.MarshalIndent(payload, "", "  ")
		if marshalErr != nil {
			fmt.Fprintf(os.Stderr, "op do: %v\n", marshalErr)
			return 1
		}
		fmt.Println(string(out))
		if err != nil {
			return 1
		}
		return 0
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "op do: %v\n", err)
		return 1
	}
	return 0
}

func parseDoParam(arg string, args []string, index *int) (string, string, bool) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(arg, "--"))
	if trimmed == "" {
		return "", "", false
	}
	if name, value, ok := strings.Cut(trimmed, "="); ok {
		name = strings.TrimSpace(name)
		if name == "" {
			return "", "", false
		}
		return name, value, true
	}
	if *index+1 >= len(args) {
		return "", "", false
	}
	name := strings.TrimSpace(trimmed)
	if name == "" {
		return "", "", false
	}
	if strings.HasPrefix(args[*index+1], "--") {
		return "", "", false
	}
	*index = *index + 1
	return name, args[*index], true
}
