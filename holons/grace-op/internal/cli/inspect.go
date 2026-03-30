package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	sdkconnect "github.com/organic-programming/go-holons/pkg/connect"
	inspectpkg "github.com/organic-programming/grace-op/internal/inspect"
)

func cmdInspect(format Format, args []string) int {
	format, target, err := parseInspectArgs(format, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "op inspect: %v\n", err)
		return 1
	}

	var doc *inspectpkg.Document
	if strings.Contains(target, ":") {
		doc, err = inspectRemote(target)
	} else {
		doc, err = inspectLocal(target)
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

func parseInspectArgs(format Format, args []string) (Format, string, error) {
	currentFormat := format
	positional := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--json":
			currentFormat = FormatJSON
		case args[i] == "--format":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("--format requires a value")
			}
			parsed, err := parseFormat(args[i+1])
			if err != nil {
				return "", "", err
			}
			currentFormat = parsed
			i++
		case strings.HasPrefix(args[i], "--format="):
			parsed, err := parseFormat(strings.TrimPrefix(args[i], "--format="))
			if err != nil {
				return "", "", err
			}
			currentFormat = parsed
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) != 1 {
		return "", "", fmt.Errorf("requires exactly one <slug> or <host:port>")
	}
	return currentFormat, positional[0], nil
}

func inspectLocal(ref string) (*inspectpkg.Document, error) {
	catalog, err := inspectpkg.LoadLocal(ref)
	if err != nil {
		return nil, err
	}
	return catalog.Document, nil
}

func inspectRemote(address string) (*inspectpkg.Document, error) {
	conn, err := sdkconnect.Connect(address)
	if err != nil {
		return nil, err
	}
	defer func() { _ = sdkconnect.Disconnect(conn) }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := holonsv1.NewHolonMetaClient(conn)
	response, err := client.Describe(ctx, &holonsv1.DescribeRequest{})
	if err != nil {
		return nil, err
	}

	return inspectpkg.FromDescribeResponse(response), nil
}
