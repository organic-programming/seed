package api

import (
	"fmt"
	"strings"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
)

const newUsage = "usage: op new [--json <payload>] | op new --list | op new --template <name> <holon-name> [--set key=value]"

func (c cliState) runIdentityCommand(format Format, quiet bool, verb string, args []string) int {
	_ = quiet
	switch verb {
	case "list":
		return c.runListCommand(format, args)
	case "show":
		return c.runShowCommand(format, args)
	case "new":
		return c.runNewCommand(format, args)
	default:
		fmt.Fprintf(c.stderr, "op %s: unsupported identity verb\n", verb)
		return 1
	}
}

func (c cliState) runListCommand(format Format, args []string) int {
	if len(args) > 1 {
		fmt.Fprintln(c.stderr, "usage: op list [root]")
		return 1
	}
	root := "."
	if len(args) == 1 {
		root = args[0]
	}
	resp, err := ListIdentities(&opv1.ListIdentitiesRequest{RootDir: root})
	if err != nil {
		fmt.Fprintf(c.stderr, "op list: %v\n", err)
		return 1
	}
	c.writeFormatted(format, resp)
	return 0
}

func (c cliState) runShowCommand(format Format, args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(c.stderr, "usage: op show <uuid-or-prefix>")
		return 1
	}
	resp, err := ShowIdentity(&opv1.ShowIdentityRequest{Uuid: args[0]})
	if err != nil {
		fmt.Fprintf(c.stderr, "op show: %v\n", err)
		return 1
	}
	c.writeFormatted(format, resp)
	return 0
}

func (c cliState) runNewCommand(format Format, args []string) int {
	if usesTemplateMode(args) {
		return c.runTemplateCommand(format, args)
	}

	payload, err := whoNewPayload(args)
	if err != nil {
		fmt.Fprintf(c.stderr, "op new: %v\n", err)
		return 1
	}

	var resp *opv1.CreateIdentityResponse
	if payload == "" {
		resp, err = createInteractive(c.stdout)
	} else {
		resp, err = createFromJSON(payload)
	}
	if err != nil {
		fmt.Fprintf(c.stderr, "op new: %v\n", err)
		return 1
	}
	c.writeFormatted(format, resp)
	return 0
}

func (c cliState) runTemplateCommand(format Format, args []string) int {
	listOnly, templateName, slug, overrides, err := parseTemplateArgs(args)
	if err != nil {
		fmt.Fprintf(c.stderr, "op new: %v\n", err)
		return 1
	}
	if listOnly {
		resp, err := ListTemplates(&opv1.ListTemplatesRequest{})
		if err != nil {
			fmt.Fprintf(c.stderr, "op new: %v\n", err)
			return 1
		}
		if format == FormatJSON {
			if err := printJSON(c.stdout, resp); err != nil {
				fmt.Fprintf(c.stderr, "op new: %v\n", err)
				return 1
			}
			return 0
		}
		for _, entry := range templateEntriesFromResponse(resp) {
			if entry.Description != "" {
				fmt.Fprintf(c.stdout, "%s\t%s\n", entry.Name, entry.Description)
			} else {
				fmt.Fprintln(c.stdout, entry.Name)
			}
		}
		return 0
	}

	resp, err := GenerateTemplate(&opv1.GenerateTemplateRequest{
		Template:  templateName,
		Slug:      slug,
		Overrides: overrides,
	})
	if err != nil {
		fmt.Fprintf(c.stderr, "op new: %v\n", err)
		return 1
	}
	if format == FormatJSON {
		c.writeFormatted(format, resp)
		return 0
	}
	fmt.Fprintf(c.stdout, "Created %s from %s at %s\n", slug, templateName, resp.GetDir())
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
	if templateName == "" || len(positional) != 1 {
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
