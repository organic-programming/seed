package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	sdkconnect "github.com/organic-programming/go-holons/pkg/connect"
	sdkdiscover "github.com/organic-programming/go-holons/pkg/discover"
	openv "github.com/organic-programming/grace-op/internal/env"
	"github.com/organic-programming/grace-op/internal/holons"
	inspectpkg "github.com/organic-programming/grace-op/internal/inspect"
)

func cmdInspect(format Format, runtimeOpts commandRuntimeOptions, args []string) int {
	format, target, specifiers, err := parseInspectArgs(format, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "op inspect: %v\n", err)
		return 1
	}

	var doc *inspectpkg.Document
	if strings.Contains(target, ":") {
		doc, err = inspectRemote(target)
	} else {
		emitOriginForExpression(runtimeOpts, target, specifiers)
		doc, err = inspectLocalWithOptions(target, runtimeOpts.timeout, specifiers)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "op inspect: %v\n", err)
		return 1
	}

	if format == FormatJSON {
		out, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "op inspect: %v\n", err)
			return 1
		}
		fmt.Println(string(out))
		return 0
	}

	fmt.Print(inspectpkg.RenderText(doc))
	return 0
}

func parseInspectArgs(format Format, args []string) (Format, string, int, error) {
	currentFormat := format
	positional := make([]string, 0, len(args))
	specifiers := 0

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--json":
			currentFormat = FormatJSON
		case args[i] == "--format":
			if i+1 >= len(args) {
				return "", "", 0, fmt.Errorf("--format requires a value")
			}
			parsed, err := parseFormat(args[i+1])
			if err != nil {
				return "", "", 0, err
			}
			currentFormat = parsed
			i++
		case strings.HasPrefix(args[i], "--format="):
			parsed, err := parseFormat(strings.TrimPrefix(args[i], "--format="))
			if err != nil {
				return "", "", 0, err
			}
			currentFormat = parsed
		case isDiscoveryFlag(args[i]):
			specifiers = addDiscoverySpecifier(specifiers, args[i])
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) != 1 {
		return "", "", 0, fmt.Errorf("requires exactly one <slug> or <host:port>")
	}
	if specifiers == 0 {
		specifiers = sdkdiscover.ALL
	}
	return currentFormat, positional[0], specifiers, nil
}

func inspectLocal(ref string) (*inspectpkg.Document, error) {
	return inspectLocalWithOptions(ref, sdkdiscover.NO_TIMEOUT, sdkdiscover.ALL)
}

func inspectLocalWithOptions(ref string, timeout int, specifiers int) (*inspectpkg.Document, error) {
	root := openv.Root()
	catalog, err := inspectpkg.LoadLocalWithOptions(ref, &root, specifiers, timeout)
	if err != nil {
		return nil, err
	}
	return catalog.Document, nil
}

func inspectRemote(address string) (*inspectpkg.Document, error) {
	result := holons.ConnectRef(address, nil, sdkdiscover.ALL, int((5*time.Second)/time.Millisecond))
	if result.Error != "" {
		return nil, errors.New(result.Error)
	}
	defer func() { _ = sdkconnect.Disconnect(result) }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := holonsv1.NewHolonMetaClient(result.Channel)
	response, err := client.Describe(ctx, &holonsv1.DescribeRequest{})
	if err != nil {
		return nil, err
	}

	return inspectpkg.FromDescribeResponse(response), nil
}
