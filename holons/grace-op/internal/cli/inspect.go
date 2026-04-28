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
	"github.com/organic-programming/grace-op/internal/progress"
)

func cmdInspect(format Format, runtimeOpts commandRuntimeOptions, args []string) int {
	format, target, specifiers, noAutoInstall, err := parseInspectArgs(format, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "op inspect: %v\n", err)
		return 1
	}
	printer := commandProgress(format, runtimeOpts.quiet)
	defer printer.Close()

	var doc *inspectpkg.Document
	if strings.Contains(target, ":") {
		doc, err = inspectRemote(target)
	} else {
		emitOriginForExpression(runtimeOpts, target, specifiers)
		doc, err = inspectLocalWithOptions(target, runtimeOpts.timeout, specifiers, noAutoInstall, printer)
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

func parseInspectArgs(format Format, args []string) (Format, string, int, bool, error) {
	currentFormat := format
	positional := make([]string, 0, len(args))
	specifiers := 0
	noAutoInstall := false

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--json":
			currentFormat = FormatJSON
		case args[i] == "--format":
			if i+1 >= len(args) {
				return "", "", 0, false, fmt.Errorf("--format requires a value")
			}
			parsed, err := parseFormat(args[i+1])
			if err != nil {
				return "", "", 0, false, err
			}
			currentFormat = parsed
			i++
		case strings.HasPrefix(args[i], "--format="):
			parsed, err := parseFormat(strings.TrimPrefix(args[i], "--format="))
			if err != nil {
				return "", "", 0, false, err
			}
			currentFormat = parsed
		case args[i] == "--no-auto-install":
			noAutoInstall = true
		case isDiscoveryFlag(args[i]):
			specifiers = addDiscoverySpecifier(specifiers, args[i])
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) != 1 {
		return "", "", 0, false, fmt.Errorf("requires exactly one <slug> or <host:port>")
	}
	if specifiers == 0 {
		specifiers = sdkdiscover.ALL
	}
	return currentFormat, positional[0], specifiers, noAutoInstall, nil
}

func inspectLocal(ref string) (*inspectpkg.Document, error) {
	return inspectLocalWithOptions(ref, sdkdiscover.NO_TIMEOUT, sdkdiscover.ALL, false, progress.Silence())
}

func inspectLocalWithOptions(ref string, timeout int, specifiers int, noAutoInstall bool, reporter progress.Reporter) (*inspectpkg.Document, error) {
	root := openv.Root()
	target, err := holons.ResolveTargetWithOptions(ref, &root, specifiers, timeout)
	if err != nil {
		return nil, err
	}
	if target.ManifestErr != nil {
		return nil, target.ManifestErr
	}
	if target.Manifest != nil {
		ctx, err := holons.ResolveBuildContext(target.Manifest, holons.BuildOptions{
			NoAutoInstall: noAutoInstall,
			Progress:      reporter,
		})
		if err != nil {
			return nil, err
		}
		if err := holons.ResolveRequiredSDKPrebuilts(target.Manifest, &ctx); err != nil {
			return nil, err
		}
	}
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
