package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	sdkconnect "github.com/organic-programming/go-holons/pkg/connect"
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

func oneShotConnectOptions(transport string) sdkconnect.ConnectOptions {
	return sdkconnect.ConnectOptions{
		Timeout:   connectDispatchTimeout,
		Transport: transport,
		Lifecycle: sdkconnect.LifecycleEphemeral,
		Start:     true,
	}
}

func (c cliState) runConnectedRPC(format Format, errPrefix string, holonName string, method string, inputJSON string, opts sdkconnect.ConnectOptions) int {
	conn, err := sdkconnect.ConnectWithOpts(holonName, opts)
	if err != nil {
		fmt.Fprintf(c.stderr, "%s: %v\n", errPrefix, err)
		return 1
	}
	defer func() { _ = sdkconnect.Disconnect(conn) }()

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	result, err := grpcclient.InvokeConn(ctx, conn, method, inputJSON)
	if err != nil {
		fmt.Fprintf(c.stderr, "%s: %v\n", errPrefix, err)
		return 1
	}
	fmt.Fprintln(c.stdout, formatRPCOutput(format, method, []byte(result.Output)))
	return 0
}

func (c cliState) runGRPCCommand(format Format, uri string, args []string) int {
	switch {
	case strings.HasPrefix(uri, "grpc+stdio://"):
		return c.runGRPCConnectedCommand(format, uri, strings.TrimPrefix(uri, "grpc+stdio://"), args, sdkconnect.TransportStdio)
	case strings.HasPrefix(uri, "grpc+unix://"):
		return c.runGRPCDirectCommand(format, "unix://"+strings.TrimPrefix(uri, "grpc+unix://"), args)
	case strings.HasPrefix(uri, "grpc+ws://") || strings.HasPrefix(uri, "grpc+wss://"):
		return c.runGRPCWebSocketCommand(format, uri, args)
	case strings.HasPrefix(uri, "grpc+tcp://"):
		target := strings.TrimPrefix(uri, "grpc+tcp://")
		if isHostPortTarget(target) {
			return c.runGRPCDirectCommand(format, target, args)
		}
		return c.runGRPCConnectedCommand(format, uri, target, args, sdkconnect.TransportTCP)
	default:
		target := strings.TrimPrefix(uri, "grpc://")
		if isHostPortTarget(target) {
			return c.runGRPCDirectCommand(format, target, args)
		}
		return c.runGRPCConnectedCommand(format, uri, target, args, sdkconnect.TransportAuto)
	}
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
	return c.runConnectedRPC(format, "op grpc", holonName, method, inputJSON, oneShotConnectOptions(transport))
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
	wsURI := strings.TrimPrefix(uri, "grpc+")
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
	return c.runConnectedRPC(format, "op", holon, method, inputJSON, oneShotConnectOptions(sdkconnect.TransportAuto))
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

func (c cliState) runCompletionCommand(args []string) int {
	if len(args) == 0 || args[0] == "zsh" {
		fmt.Fprint(c.stdout, zshCompletion)
		return 0
	}
	if args[0] == "bash" {
		fmt.Fprint(c.stdout, bashCompletion)
		return 0
	}
	fmt.Fprintf(c.stderr, "op completion: unsupported shell %q (use zsh or bash)\n", args[0])
	return 1
}

func (c cliState) runCompleteCommand(args []string) int {
	if len(args) < 1 {
		return 0
	}
	verb := args[0]
	prefix := ""
	if len(args) > 1 {
		prefix = strings.ToLower(args[1])
	}
	switch verb {
	case "build", "run", "install", "check", "test", "clean", "inspect", "show", "do":
		completeSlugs(c.stdout, prefix)
	case "uninstall":
		completeInstalled(c.stdout, prefix)
	default:
		completeVerbs(c.stdout, prefix)
	}
	return 0
}

func completeSlugs(w io.Writer, prefix string) {
	local, _ := holons.DiscoverLocalHolons()
	cached, _ := holons.DiscoverCachedHolons()
	seen := map[string]struct{}{}
	for _, h := range append(local, cached...) {
		slug := h.Identity.Slug()
		if slug == "" {
			slug = h.Identity.GivenName
		}
		if slug == "" {
			continue
		}
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
		if strings.HasPrefix(slug, prefix) {
			fmt.Fprintln(w, slug)
		}
	}
	for _, entry := range holons.DiscoverInOPBIN() {
		name := strings.SplitN(entry, " -> ", 2)[0]
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		if strings.HasPrefix(name, prefix) {
			fmt.Fprintln(w, name)
		}
	}
}

func completeInstalled(w io.Writer, prefix string) {
	for _, entry := range holons.DiscoverInOPBIN() {
		name := strings.SplitN(entry, " -> ", 2)[0]
		if strings.HasPrefix(name, prefix) {
			fmt.Fprintln(w, name)
		}
	}
}

func completeVerbs(w io.Writer, prefix string) {
	verbs := []string{
		"build", "check", "clean", "completion", "discover", "do",
		"env", "help", "inspect", "install", "list", "mcp",
		"mod", "new", "run", "serve", "show", "test", "tools",
		"uninstall", "version",
	}
	for _, verb := range verbs {
		if strings.HasPrefix(verb, prefix) {
			fmt.Fprintln(w, verb)
		}
	}
}

const zshCompletion = `#compdef op

_op() {
    local -a commands
    local curcontext="$curcontext" state line

    if (( CURRENT == 2 )); then
        commands=($(op __complete verb "${words[CURRENT]}"))
        _describe 'op commands' commands
        return
    fi

    case "${words[2]}" in
        build|run|install|check|test|clean|inspect|show|do)
            local -a slugs
            slugs=($(op __complete "${words[2]}" "${words[CURRENT]}"))
            _describe 'holons' slugs
            ;;
        uninstall)
            local -a installed
            installed=($(op __complete uninstall "${words[CURRENT]}"))
            _describe 'installed holons' installed
            ;;
    esac
}

compdef _op op
`

const bashCompletion = `_op() {
    local cur prev
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    if [[ ${COMP_CWORD} -eq 1 ]]; then
        COMPREPLY=($(compgen -W "$(op __complete verb "$cur")" -- "$cur"))
        return
    fi

    case "${COMP_WORDS[1]}" in
        build|run|install|check|test|clean|inspect|show|do)
            COMPREPLY=($(compgen -W "$(op __complete "${COMP_WORDS[1]}" "$cur")" -- "$cur"))
            ;;
        uninstall)
            COMPREPLY=($(compgen -W "$(op __complete uninstall "$cur")" -- "$cur"))
            ;;
    esac
}

complete -F _op op
`
