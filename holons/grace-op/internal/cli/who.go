package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	sdkdiscover "github.com/organic-programming/go-holons/pkg/discover"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	openv "github.com/organic-programming/grace-op/internal/env"
	"github.com/organic-programming/grace-op/internal/scaffold"
	"github.com/organic-programming/grace-op/internal/suggest"
	"github.com/organic-programming/grace-op/internal/who"

	"google.golang.org/protobuf/proto"
)

const newUsage = "usage: op new [--json <payload>] | op new --list | op new --template <name> <holon-name> [--set key=value]"

func cmdWho(format Format, runtimeOpts commandRuntimeOptions, verb string, args []string) int {
	switch verb {
	case "list":
		return cmdWhoList(format, runtimeOpts, args)
	case "show":
		return cmdWhoShow(format, runtimeOpts, args)
	case "new":
		return cmdWhoNew(format, runtimeOpts.quiet, args)
	default:
		fmt.Fprintf(os.Stderr, "op %s: unsupported identity verb\n", verb)
		return 1
	}
}

func cmdWhoList(format Format, runtimeOpts commandRuntimeOptions, args []string) int {
	specifiers := 0
	limit := sdkdiscover.NO_LIMIT
	positional := make([]string, 0, 1)
	for i := 0; i < len(args); i++ {
		switch {
		case isDiscoveryFlag(args[i]):
			specifiers = addDiscoverySpecifier(specifiers, args[i])
		case args[i] == "--limit":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "op list: --limit requires a value")
				return 1
			}
			value, err := strconv.Atoi(strings.TrimSpace(args[i+1]))
			if err != nil || value < 0 {
				fmt.Fprintf(os.Stderr, "op list: invalid --limit %q\n", args[i+1])
				return 1
			}
			limit = value
			i++
		case strings.HasPrefix(args[i], "--limit="):
			value, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(args[i], "--limit=")))
			if err != nil || value < 0 {
				fmt.Fprintf(os.Stderr, "op list: invalid --limit %q\n", strings.TrimPrefix(args[i], "--limit="))
				return 1
			}
			limit = value
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) > 1 {
		fmt.Fprintln(os.Stderr, "usage: op list [root]")
		return 1
	}

	root := openv.Root()
	if len(positional) == 1 {
		root = positional[0]
	}
	if specifiers == 0 {
		specifiers = sdkdiscover.ALL
	}

	resp, err := who.ListWithDetailedOrigins(root, specifiers, limit, runtimeOpts.timeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "op list: %v\n", err)
		return 1
	}

	printFormattedResponse(format, resp)
	return 0
}

func cmdWhoShow(format Format, runtimeOpts commandRuntimeOptions, args []string) int {
	specifiers := 0
	positional := make([]string, 0, 1)
	for _, arg := range args {
		if isDiscoveryFlag(arg) {
			specifiers = addDiscoverySpecifier(specifiers, arg)
			continue
		}
		positional = append(positional, arg)
	}

	if len(positional) != 1 {
		fmt.Fprintln(os.Stderr, "usage: op show <uuid-or-prefix>")
		return 1
	}
	if specifiers == 0 {
		specifiers = sdkdiscover.ALL
	}

	root := openv.Root()
	emitOriginForExpression(runtimeOpts, positional[0], specifiers)
	resp, err := who.ShowWithOptions(positional[0], &root, specifiers, runtimeOpts.timeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "op show: %v\n", err)
		return 1
	}

	printFormattedResponse(format, resp)
	return 0
}

func cmdWhoNew(format Format, globalQuiet bool, args []string) int {
	ui, args, _ := extractQuietFlag(args)
	quiet := globalQuiet || ui.Quiet
	printer := commandProgress(format, quiet)
	defer printer.Close()

	if usesTemplateMode(args) {
		return cmdTemplateNew(format, quiet, args)
	}

	payload, err := whoNewPayload(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "op new: %v\n", err)
		return 1
	}

	var resp proto.Message
	var createdResp *opv1.CreateIdentityResponse
	if payload == "" {
		created, createErr := who.CreateInteractive(os.Stdin, os.Stdout)
		if createErr != nil {
			printer.Done("birth failed", createErr)
			fmt.Fprintf(os.Stderr, "op new: %v\n", createErr)
			return 1
		}
		resp = created
		createdResp = created
	} else {
		created, createErr := who.CreateFromJSON(payload)
		if createErr != nil {
			printer.Done("birth failed", createErr)
			fmt.Fprintf(os.Stderr, "op new: %v\n", createErr)
			return 1
		}
		resp = created
		createdResp = created
	}

	if createdResp != nil && createdResp.GetIdentity() != nil {
		name := strings.TrimSpace(createdResp.GetIdentity().GetGivenName() + " " + createdResp.GetIdentity().GetFamilyName())
		if name != "" {
			printer.Done("Born: "+name, nil)
		}
	}
	printFormattedResponse(format, resp)
	if createdResp != nil && createdResp.GetIdentity() != nil {
		holon := strings.ToLower(strings.TrimSpace(createdResp.GetIdentity().GetGivenName() + "-" + strings.TrimSuffix(createdResp.GetIdentity().GetFamilyName(), "?")))
		holon = strings.ReplaceAll(holon, " ", "-")
		holon = strings.Trim(holon, "-")
		emitSuggestions(os.Stderr, format, quiet, suggest.Context{Command: "new", Holon: holon})
	}
	return 0
}

func cmdTemplateNew(format Format, quiet bool, args []string) int {
	listOnly, templateName, slug, overrides, err := parseTemplateArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "op new: %v\n", err)
		return 1
	}
	if listOnly {
		entries, err := scaffold.List()
		if err != nil {
			fmt.Fprintf(os.Stderr, "op new: %v\n", err)
			return 1
		}
		if format == FormatJSON {
			out, marshalErr := json.MarshalIndent(entries, "", "  ")
			if marshalErr != nil {
				fmt.Fprintf(os.Stderr, "op new: %v\n", marshalErr)
				return 1
			}
			fmt.Println(string(out))
			return 0
		}
		for _, entry := range entries {
			if entry.Description != "" {
				fmt.Printf("%s\t%s\n", entry.Name, entry.Description)
			} else {
				fmt.Println(entry.Name)
			}
		}
		return 0
	}

	printer := commandProgress(format, quiet)
	defer printer.Close()
	printer.Step("rendering template " + templateName + "...")
	result, err := scaffold.Generate(templateName, slug, scaffold.GenerateOptions{Overrides: overrides})
	if err != nil {
		printer.Done("template generation failed", err)
		fmt.Fprintf(os.Stderr, "op new: %v\n", err)
		return 1
	}
	printer.Done("created "+result.Dir, nil)
	if format == FormatJSON {
		out, marshalErr := json.MarshalIndent(result, "", "  ")
		if marshalErr != nil {
			fmt.Fprintf(os.Stderr, "op new: %v\n", marshalErr)
			return 1
		}
		fmt.Println(string(out))
		return 0
	}
	fmt.Printf("Created %s from %s at %s\n", slug, templateName, result.Dir)
	return 0
}

func whoNewPayload(args []string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}

	if len(args) == 1 && looksLikeJSON(args[0]) {
		return args[0], nil
	}

	switch {
	case args[0] == "--json":
		if len(args) != 2 {
			return "", fmt.Errorf(newUsage)
		}
		return args[1], nil
	case strings.HasPrefix(args[0], "--json="):
		if len(args) != 1 {
			return "", fmt.Errorf(newUsage)
		}
		return strings.TrimPrefix(args[0], "--json="), nil
	default:
		return "", fmt.Errorf(newUsage)
	}
}

func usesTemplateMode(args []string) bool {
	for _, arg := range args {
		if arg == "--list" || arg == "--template" || strings.HasPrefix(arg, "--template=") || arg == "--set" || strings.HasPrefix(arg, "--set=") {
			return true
		}
	}
	return false
}

func parseTemplateArgs(args []string) (bool, string, string, map[string]string, error) {
	overrides := make(map[string]string)
	listOnly := false
	templateName := ""
	positional := make([]string, 0, 1)
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--list":
			listOnly = true
		case args[i] == "--template":
			if i+1 >= len(args) {
				return false, "", "", nil, fmt.Errorf("--template requires a value")
			}
			templateName = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--template="):
			templateName = strings.TrimPrefix(args[i], "--template=")
		case args[i] == "--set":
			if i+1 >= len(args) {
				return false, "", "", nil, fmt.Errorf("--set requires key=value")
			}
			key, value, err := parseSetOverride(args[i+1])
			if err != nil {
				return false, "", "", nil, err
			}
			overrides[key] = value
			i++
		case strings.HasPrefix(args[i], "--set="):
			key, value, err := parseSetOverride(strings.TrimPrefix(args[i], "--set="))
			if err != nil {
				return false, "", "", nil, err
			}
			overrides[key] = value
		case args[i] == "--json" || strings.HasPrefix(args[i], "--json="):
			return false, "", "", nil, fmt.Errorf("--json cannot be combined with template flags")
		case strings.HasPrefix(args[i], "--"):
			return false, "", "", nil, fmt.Errorf("unknown flag %q", args[i])
		default:
			positional = append(positional, args[i])
		}
	}
	if listOnly {
		if templateName != "" || len(positional) > 0 || len(overrides) > 0 {
			return false, "", "", nil, fmt.Errorf("--list does not accept <holon-name> or template overrides")
		}
		return true, "", "", overrides, nil
	}
	if templateName == "" {
		return false, "", "", nil, fmt.Errorf(newUsage)
	}
	if len(positional) != 1 {
		return false, "", "", nil, fmt.Errorf(newUsage)
	}
	return false, templateName, positional[0], overrides, nil
}

func parseSetOverride(value string) (string, string, error) {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		return "", "", fmt.Errorf("--set requires key=value")
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

func printFormattedResponse(format Format, resp proto.Message) {
	if resp == nil {
		return
	}
	out := strings.TrimSpace(FormatResponse(format, resp))
	if out != "" {
		fmt.Println(out)
	}
}
