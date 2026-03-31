package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	sdkconnect "github.com/organic-programming/go-holons/pkg/connect"
	sdkdiscover "github.com/organic-programming/go-holons/pkg/discover"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/grace-op/internal/grpcclient"
	"github.com/organic-programming/grace-op/internal/holons"
)

func (c cliState) runDiscoverCommand(format Format, args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(c.stderr, "op discover: accepts no positional arguments")
		return 1
	}
	resp, err := Discover(&opv1.DiscoverRequest{RootDir: "."})
	if err != nil {
		fmt.Fprintf(c.stderr, "op discover: %v\n", err)
		return 1
	}
	c.writeFormatted(format, resp)
	return 0
}

func (c cliState) runInspectCommand(format Format, args []string) int {
	currentFormat := format
	positional := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--json":
			currentFormat = FormatJSON
		case args[i] == "--format":
			if i+1 >= len(args) {
				fmt.Fprintln(c.stderr, "op inspect: --format requires a value")
				return 1
			}
			parsed, err := parseFormat(args[i+1])
			if err != nil {
				fmt.Fprintf(c.stderr, "op inspect: %v\n", err)
				return 1
			}
			currentFormat = parsed
			i++
		default:
			positional = append(positional, args[i])
		}
	}
	if len(positional) != 1 {
		fmt.Fprintln(c.stderr, "op inspect: requires exactly one <slug> or <host:port>")
		return 1
	}
	resp, err := Inspect(&opv1.InspectRequest{Target: positional[0]})
	if err != nil {
		fmt.Fprintf(c.stderr, "op inspect: %v\n", err)
		return 1
	}
	c.writeFormatted(currentFormat, resp)
	return 0
}

func (c cliState) runSequenceCommand(format Format, args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(c.stderr, "op do: requires <holon> and <sequence>")
		return 1
	}
	req := &opv1.RunSequenceRequest{
		Holon:    args[0],
		Sequence: args[1],
		Params:   make(map[string]string),
	}
	for i := 2; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--dry-run":
			req.DryRun = true
		case arg == "--continue-on-error":
			req.ContinueOnError = true
		case strings.HasPrefix(arg, "--"):
			name, value, ok := parseDoParam(arg, args, &i)
			if !ok {
				fmt.Fprintf(c.stderr, "op do: invalid param flag %q\n", arg)
				return 1
			}
			req.Params[name] = value
		default:
			fmt.Fprintf(c.stderr, "op do: unexpected argument %q\n", arg)
			return 1
		}
	}
	resp, err := RunSequence(req)
	if format == FormatJSON {
		c.writeFormatted(format, resp)
		if err != nil {
			return 1
		}
		return 0
	}
	if err != nil {
		fmt.Fprintf(c.stderr, "op do: %v\n", err)
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
	if *index+1 >= len(args) || strings.HasPrefix(args[*index+1], "--") {
		return "", "", false
	}
	*index = *index + 1
	return trimmed, args[*index], true
}

func (c cliState) runToolsCommand(args []string) int {
	format := "openai"
	positional := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--format":
			if i+1 >= len(args) {
				fmt.Fprintln(c.stderr, "op tools: --format requires a value")
				return 1
			}
			format = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--format="):
			format = strings.TrimPrefix(args[i], "--format=")
		default:
			positional = append(positional, args[i])
		}
	}
	if len(positional) != 1 {
		fmt.Fprintln(c.stderr, "op tools: requires exactly one <slug>")
		return 1
	}
	resp, err := Tools(&opv1.ToolsRequest{Target: positional[0], Format: format})
	if err != nil {
		fmt.Fprintf(c.stderr, "op tools: %v\n", err)
		return 1
	}
	fmt.Fprintln(c.stdout, string(resp.GetPayload()))
	return 0
}

func (c cliState) runEnvCommand(format Format, args []string) int {
	req := &opv1.EnvRequest{}
	for _, arg := range args {
		switch arg {
		case "--init":
			req.Init = true
		case "--shell":
			req.Shell = true
		default:
			if strings.HasPrefix(arg, "--") {
				fmt.Fprintf(c.stderr, "op env: unknown flag %q\n", arg)
				return 1
			}
			fmt.Fprintln(c.stderr, "op env: does not accept positional arguments")
			return 1
		}
	}
	resp, err := Env(req)
	if err != nil {
		fmt.Fprintf(c.stderr, "op env: %v\n", err)
		return 1
	}
	if format == FormatJSON {
		c.writeFormatted(format, resp)
		return 0
	}
	if req.GetShell() {
		if req.GetInit() {
			fmt.Fprintf(c.stderr, "created %s/\ncreated %s/\ncreated %s/\n", resp.GetOppath(), resp.GetOpbin(), resp.GetCacheDir())
		}
		fmt.Fprintln(c.stdout, resp.GetShell())
		return 0
	}
	if req.GetInit() {
		fmt.Fprintf(c.stdout, "created %s/\ncreated %s/\ncreated %s/\n", resp.GetOppath(), resp.GetOpbin(), resp.GetCacheDir())
		return 0
	}
	fmt.Fprintf(c.stdout, "OPPATH=%s\nOPBIN=%s\nROOT=%s\n", resp.GetOppath(), resp.GetOpbin(), resp.GetRoot())
	return 0
}

const connectDispatchTimeout = 10 * time.Second

func (c cliState) runConnectedRPC(format Format, errPrefix string, holonName string, method string, inputJSON string, _ string) int {
	result := holons.ConnectRef(holonName, nil, sdkdiscover.ALL, int(connectDispatchTimeout/time.Millisecond))
	if result.Error != "" {
		fmt.Fprintf(c.stderr, "%s: %s\n", errPrefix, result.Error)
		return 1
	}
	defer func() { _ = sdkconnect.Disconnect(result) }()

	ctx, cancel := context.WithTimeout(context.Background(), connectDispatchTimeout)
	defer cancel()

	callResult, err := grpcclient.InvokeConn(ctx, result.Channel, method, inputJSON)
	if err != nil {
		fmt.Fprintf(c.stderr, "%s: %v\n", errPrefix, err)
		return 1
	}
	fmt.Fprintln(c.stdout, formatRPCOutput(format, method, []byte(callResult.Output)))
	return 0
}

func (c cliState) runGRPCCommand(format Format, uri string, args []string) int {
	switch {
	case strings.HasPrefix(uri, "stdio://"):
		return c.runGRPCConnectedCommand(format, uri, trimURIAnyPrefix(uri, "stdio://"), args, "stdio")
	case strings.HasPrefix(uri, "unix://"):
		return c.runGRPCDirectCommand(format, "unix://"+trimURIAnyPrefix(uri, "unix://"), args)
	case strings.HasPrefix(uri, "ws://") || strings.HasPrefix(uri, "wss://"):
		return c.runGRPCWebSocketCommand(format, uri, args)
	case strings.HasPrefix(uri, "tcp://"):
		target := trimURIAnyPrefix(uri, "tcp://")
		if isHostPortTarget(target) {
			return c.runGRPCDirectCommand(format, target, args)
		}
		return c.runGRPCConnectedCommand(format, uri, target, args, "tcp")
	default:
		target := strings.TrimPrefix(uri, "grpc://")
		if isHostPortTarget(target) {
			return c.runGRPCDirectCommand(format, target, args)
		}
		return c.runGRPCConnectedCommand(format, uri, target, args, "auto")
	}
}

func trimURIAnyPrefix(uri string, prefixes ...string) string {
	for _, prefix := range prefixes {
		if strings.HasPrefix(uri, prefix) {
			return strings.TrimPrefix(uri, prefix)
		}
	}
	return uri
}

func (c cliState) runGRPCConnectedCommand(format Format, uri string, holonName string, args []string, transport string) int {
	if len(args) < 1 {
		fmt.Fprintln(c.stderr, "op grpc: method required")
		fmt.Fprintf(c.stderr, "usage: op %s <method>\n", uri)
		return 1
	}
	method := args[0]
	inputJSON := "{}"
	if len(args) > 1 {
		inputJSON = args[1]
	}
	return c.runConnectedRPC(format, "op grpc", holonName, method, inputJSON, transport)
}

func (c cliState) runGRPCDirectCommand(format Format, address string, args []string) int {
	if len(args) == 0 {
		methods, err := grpcclient.ListMethods(address)
		if err != nil {
			fmt.Fprintf(c.stderr, "op grpc: %v\n", err)
			return 1
		}
		fmt.Fprintf(c.stdout, "Available methods at %s:\n", address)
		for _, method := range methods {
			fmt.Fprintf(c.stdout, "  %s\n", method)
		}
		return 0
	}
	method := args[0]
	inputJSON := "{}"
	if len(args) > 1 {
		inputJSON = args[1]
	}
	result, err := grpcclient.Dial(address, method, inputJSON)
	if err != nil {
		fmt.Fprintf(c.stderr, "op grpc: %v\n", err)
		return 1
	}
	fmt.Fprintln(c.stdout, formatRPCOutput(format, method, []byte(result.Output)))
	return 0
}

func (c cliState) runGRPCWebSocketCommand(format Format, uri string, args []string) int {
	wsURI := trimURIAnyPrefix(uri, "grpc+")
	if len(args) < 1 {
		fmt.Fprintln(c.stderr, "op grpc: method required")
		fmt.Fprintf(c.stderr, "usage: op %s <method>\n", uri)
		return 1
	}
	method := args[0]
	inputJSON := "{}"
	if len(args) > 1 {
		inputJSON = args[1]
	}
	if !strings.Contains(wsURI[5:], "/") {
		wsURI += "/grpc"
	}
	result, err := grpcclient.DialWebSocket(wsURI, method, inputJSON)
	if err != nil {
		fmt.Fprintf(c.stderr, "op grpc: %v\n", err)
		return 1
	}
	fmt.Fprintln(c.stdout, formatRPCOutput(format, method, []byte(result.Output)))
	return 0
}

func (c cliState) runHolonCommand(format Format, holon string, args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(c.stderr, "op: missing command for holon %q\n", holon)
		return 1
	}
	method, inputJSON, err := mapHolonCommandToRPC(args)
	if err != nil {
		fmt.Fprintf(c.stderr, "op: %v\n", err)
		return 1
	}
	return c.runConnectedRPC(format, "op", holon, method, inputJSON, "auto")
}

func mapHolonCommandToRPC(args []string) (string, string, error) {
	command := strings.TrimSpace(args[0])
	rest := args[1:]
	method := mapCommandNameToMethod(command)
	if len(rest) > 0 && looksLikeJSON(rest[0]) {
		return method, rest[0], nil
	}
	switch strings.ToLower(command) {
	case "list":
		if len(rest) > 0 {
			payload, err := json.Marshal(map[string]string{"rootDir": rest[0]})
			if err != nil {
				return "", "", err
			}
			return method, string(payload), nil
		}
		return method, "{}", nil
	case "show":
		if len(rest) < 1 {
			return "", "", fmt.Errorf("show requires <uuid>")
		}
		payload, err := json.Marshal(map[string]string{"uuid": rest[0]})
		if err != nil {
			return "", "", err
		}
		return method, string(payload), nil
	default:
		return method, "{}", nil
	}
}

func mapCommandNameToMethod(command string) string {
	switch strings.ToLower(strings.TrimSpace(command)) {
	case "new":
		return "CreateIdentity"
	case "list":
		return "ListIdentities"
	case "show":
		return "ShowIdentity"
	default:
		return command
	}
}

func looksLikeJSON(value string) bool {
	trimmed := strings.TrimSpace(value)
	return strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")
}

func isHostPortTarget(target string) bool {
	_, _, err := net.SplitHostPort(strings.TrimSpace(target))
	return err == nil
}

func canonicalMethodName(method string) string {
	trimmed := strings.TrimSpace(method)
	if i := strings.LastIndex(trimmed, "/"); i >= 0 && i+1 < len(trimmed) {
		return trimmed[i+1:]
	}
	return trimmed
}
