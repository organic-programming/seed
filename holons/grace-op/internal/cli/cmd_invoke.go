package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	sdkdiscover "github.com/organic-programming/go-holons/pkg/discover"
	internalgrpc "github.com/organic-programming/grace-op/internal/grpcclient"
	inspectpkg "github.com/organic-programming/grace-op/internal/inspect"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const invokeCompletionTimeout = 2 * time.Second

type invokeMethodCandidate struct {
	service  string
	method   string
	examples [][]string
}

func newInvokeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "invoke <holon-or-uri> <method> [json]",
		Short:             "Invoke a holon's RPC method",
		Args:              cobra.RangeArgs(2, 3),
		ValidArgsFunction: completeInvokeArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			return runCommandCode(runInvokeCommand(format, currentRuntimeOptions(), cmd, args))
		},
	}

	cmd.Flags().Bool("clean", false, "clean a holon target before invoking it")
	cmd.Flags().Bool("no-build", false, "do not auto-build a missing holon artifact")

	return cmd
}

func runInvokeCommand(format Format, runtimeOpts commandRuntimeOptions, cmd *cobra.Command, args []string) int {
	target := strings.TrimSpace(args[0])
	method := strings.TrimSpace(args[1])
	inputJSON := "{}"
	if len(args) > 2 {
		inputJSON = args[2]
	}

	cleanFirst, _ := cmd.Flags().GetBool("clean")
	noBuild, _ := cmd.Flags().GetBool("no-build")

	return cmdInvoke(format, runtimeOpts, target, method, inputJSON, cleanFirst, noBuild)
}

func cmdInvoke(
	format Format,
	runtimeOpts commandRuntimeOptions,
	target string,
	method string,
	inputJSON string,
	cleanFirst bool,
	noBuild bool,
) int {
	target = strings.TrimSpace(target)
	method = strings.TrimSpace(method)
	if target == "" {
		fmt.Fprintln(os.Stderr, "op invoke: target required")
		return 1
	}
	if method == "" {
		fmt.Fprintln(os.Stderr, "op invoke: method required")
		return 1
	}

	switch {
	case isTransportURI(target):
		return cmdInvokeTransportTarget(format, target, method, inputJSON, cleanFirst, noBuild)
	case isHostPortTarget(target):
		if cleanFirst {
			fmt.Fprintln(os.Stderr, "op invoke: --clean is only supported for holon targets")
			return 1
		}
		if noBuild {
			fmt.Fprintln(os.Stderr, "op invoke: --no-build is only supported for holon targets")
			return 1
		}
		return cmdGRPCDirect(format, target, invokeDirectArgs(method, inputJSON))
	case isExecutableFile(target):
		if cleanFirst {
			fmt.Fprintln(os.Stderr, "op invoke: --clean is only supported for holon targets")
			return 1
		}
		if noBuild {
			fmt.Fprintln(os.Stderr, "op invoke: --no-build is only supported for holon targets")
			return 1
		}
		return cmdDirectBinary(format, target, invokeDirectArgs(method, inputJSON))
	default:
		return cmdInvokeHolon(format, runtimeOpts, target, method, inputJSON, cleanFirst, noBuild)
	}
}

func cmdInvokeHolon(
	format Format,
	runtimeOpts commandRuntimeOptions,
	holonName string,
	method string,
	inputJSON string,
	cleanFirst bool,
	noBuild bool,
) int {
	if cleanFirst && noBuild {
		fmt.Fprintln(os.Stderr, "op invoke: --clean cannot be combined with --no-build")
		return 1
	}

	emitOriginForExpression(runtimeOpts, holonName, sdkdiscover.ALL)

	if cleanFirst {
		printer := commandProgress(format, runtimeOpts.quiet)
		defer printer.Close()
		if _, err := runCleanWithProgress(printer, holonName); err != nil {
			fmt.Fprintf(os.Stderr, "op invoke: %v\n", err)
			return 1
		}
	}

	return runConnectedRPC(format, "op invoke", holonName, method, inputJSON, "auto", noBuild)
}

func cmdInvokeTransportTarget(format Format, target string, method string, inputJSON string, cleanFirst bool, noBuild bool) int {
	if cleanFirst {
		fmt.Fprintln(os.Stderr, "op invoke: --clean is only supported for holon targets")
		return 1
	}

	trimmed := strings.TrimSpace(target)
	switch {
	case strings.HasPrefix(trimmed, "grpc://"):
		remainder := strings.TrimPrefix(trimmed, "grpc://")
		if isHostPortTarget(remainder) {
			if noBuild {
				fmt.Fprintln(os.Stderr, "op invoke: --no-build is only supported for holon targets")
				return 1
			}
			return cmdGRPCDirect(format, remainder, invokeDirectArgs(method, inputJSON))
		}
		return cmdGRPC(format, target, invokeConnectedArgs(method, inputJSON, noBuild))
	case strings.HasPrefix(trimmed, "tcp://"):
		remainder := trimURIAnyPrefix(trimmed, "tcp://")
		if isHostPortTarget(remainder) {
			if noBuild {
				fmt.Fprintln(os.Stderr, "op invoke: --no-build is only supported for holon targets")
				return 1
			}
			return cmdGRPCDirect(format, remainder, invokeDirectArgs(method, inputJSON))
		}
		return cmdGRPC(format, target, invokeConnectedArgs(method, inputJSON, noBuild))
	case strings.HasPrefix(trimmed, "stdio://"):
		return cmdGRPC(format, target, invokeConnectedArgs(method, inputJSON, noBuild))
	case strings.HasPrefix(trimmed, "unix://"):
		remainder := trimURIAnyPrefix(trimmed, "unix://")
		if isUnixSocketTarget(remainder) {
			if noBuild {
				fmt.Fprintln(os.Stderr, "op invoke: --no-build is only supported for holon targets")
				return 1
			}
			return cmdGRPC(format, target, invokeDirectArgs(method, inputJSON))
		}
		return cmdGRPC(format, target, invokeConnectedArgs(method, inputJSON, noBuild))
	case strings.HasPrefix(trimmed, "ws://"),
		strings.HasPrefix(trimmed, "wss://"):
		if noBuild {
			fmt.Fprintln(os.Stderr, "op invoke: --no-build is only supported for holon targets")
			return 1
		}
		return cmdGRPC(format, target, invokeDirectArgs(method, inputJSON))
	default:
		if noBuild {
			fmt.Fprintln(os.Stderr, "op invoke: --no-build is only supported for holon targets")
			return 1
		}
		return cmdGRPC(format, target, invokeDirectArgs(method, inputJSON))
	}
}

func invokeDirectArgs(method string, inputJSON string) []string {
	args := []string{method}
	if strings.TrimSpace(inputJSON) != "" && strings.TrimSpace(inputJSON) != "{}" {
		args = append(args, inputJSON)
	}
	return args
}

func invokeConnectedArgs(method string, inputJSON string, noBuild bool) []string {
	args := []string{method}
	if noBuild {
		args = append(args, "--no-build")
	}
	if strings.TrimSpace(inputJSON) != "" && strings.TrimSpace(inputJSON) != "{}" {
		args = append(args, inputJSON)
	}
	return args
}

func completeInvokeArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	args = normalizeCompletionArgs(args, toComplete)

	if err := applyCurrentEnvOverrides(); err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	switch {
	case len(args) == 0:
		return completeHolonSlugs(nil, args, toComplete)
	case len(args) == 1:
		return completeInvokeMethods(args[0], toComplete), cobra.ShellCompDirectiveNoFileComp
	default:
		return completeInvokePayload(cmd, args[0], args[1], args[2:], toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

func completeInvokeMethods(target string, toComplete string) []string {
	methods, err := discoverInvokeMethodCandidates(target)
	if err != nil {
		return nil
	}
	return formatInvokeMethodCompletions(methods, toComplete)
}

func discoverInvokeMethodCandidates(target string) ([]invokeMethodCandidate, error) {
	timeoutMS := currentRuntimeOptions().timeout
	if localTarget, ok := invokeInspectableTarget(target); ok {
		localCatalog, err := inspectpkg.LoadLocalWithOptions(localTarget, nil, sdkdiscover.ALL, timeoutMS)
		if err == nil && localCatalog != nil && localCatalog.Document != nil {
			if methods := invokeMethodCandidatesFromDocument(localCatalog.Document); len(methods) > 0 {
				return methods, nil
			}
		}
	}

	return discoverInvokeRemoteMethods(target)
}

func discoverInvokeRemoteMethods(target string) ([]invokeMethodCandidate, error) {
	trimmed := strings.TrimSpace(target)
	timeout := invokeCompletionTimeout
	if configured := currentRuntimeOptions().timeout; configured > 0 {
		timeout = time.Duration(configured) * time.Millisecond
	}

	switch {
	case strings.HasPrefix(trimmed, "grpc://"):
		remainder := strings.TrimPrefix(trimmed, "grpc://")
		if isHostPortTarget(remainder) {
			return invokeMethodCandidatesFromAddress(remainder, timeout)
		}
		return invokeMethodCandidatesFromConnectedTarget(remainder, "auto", timeout)
	case strings.HasPrefix(trimmed, "tcp://"):
		remainder := trimURIAnyPrefix(trimmed, "tcp://")
		if isHostPortTarget(remainder) {
			return invokeMethodCandidatesFromAddress(remainder, timeout)
		}
		return invokeMethodCandidatesFromConnectedTarget(remainder, "tcp", timeout)
	case strings.HasPrefix(trimmed, "stdio://"):
		return invokeMethodCandidatesFromConnectedTarget(trimURIAnyPrefix(trimmed, "stdio://"), "stdio", timeout)
	case strings.HasPrefix(trimmed, "unix://"):
		remainder := trimURIAnyPrefix(trimmed, "unix://")
		if isUnixSocketTarget(remainder) {
			return invokeMethodCandidatesFromAddress(trimmed, timeout)
		}
		return invokeMethodCandidatesFromConnectedTarget(remainder, "unix", timeout)
	case isHostPortTarget(trimmed):
		return invokeMethodCandidatesFromAddress(trimmed, timeout)
	case isExecutableFile(trimmed):
		return nil, fmt.Errorf("method completion is not available for direct executables")
	case strings.HasPrefix(trimmed, "ws://"),
		strings.HasPrefix(trimmed, "wss://"),
		strings.HasPrefix(trimmed, "http://"),
		strings.HasPrefix(trimmed, "https://"):
		return nil, fmt.Errorf("method completion is not available for %s", trimmed)
	default:
		return invokeMethodCandidatesFromConnectedTarget(trimmed, "auto", timeout)
	}
}

func invokeInspectableTarget(target string) (string, bool) {
	trimmed := strings.TrimSpace(target)
	switch {
	case trimmed == "":
		return "", false
	case strings.HasPrefix(trimmed, "grpc://"):
		remainder := strings.TrimPrefix(trimmed, "grpc://")
		if remainder == "" || isHostPortTarget(remainder) {
			return "", false
		}
		return remainder, true
	case strings.HasPrefix(trimmed, "tcp://"):
		remainder := trimURIAnyPrefix(trimmed, "tcp://")
		if remainder == "" || isHostPortTarget(remainder) {
			return "", false
		}
		return remainder, true
	case strings.HasPrefix(trimmed, "stdio://"):
		remainder := trimURIAnyPrefix(trimmed, "stdio://")
		if remainder == "" {
			return "", false
		}
		return remainder, true
	case strings.HasPrefix(trimmed, "unix://"):
		remainder := trimURIAnyPrefix(trimmed, "unix://")
		if remainder == "" || isUnixSocketTarget(remainder) {
			return "", false
		}
		return remainder, true
	case isTransportURI(trimmed), isHostPortTarget(trimmed), isExecutableFile(trimmed):
		return "", false
	default:
		return trimmed, true
	}
}

func invokeMethodCandidatesFromDocument(doc *inspectpkg.Document) []invokeMethodCandidate {
	if doc == nil {
		return nil
	}

	out := make([]invokeMethodCandidate, 0)
	for _, service := range doc.Services {
		serviceName := strings.TrimSpace(service.Name)
		for _, method := range service.Methods {
			methodName := strings.TrimSpace(method.Name)
			if methodName == "" {
				continue
			}
			out = append(out, invokeMethodCandidate{
				service:  serviceName,
				method:   methodName,
				examples: method.Examples,
			})
		}
	}
	return out
}

func invokeMethodCandidatesFromConnectedTarget(target string, transport string, timeout time.Duration) ([]invokeMethodCandidate, error) {
	conn, err := connectForRPCWithTimeout(target, transport, timeout)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.close() }()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	methods, err := internalgrpc.ListMethodsConn(ctx, conn.conn)
	if err != nil {
		return nil, err
	}
	return invokeMethodCandidatesFromQualifiedList(methods), nil
}

func invokeMethodCandidatesFromAddress(address string, timeout time.Duration) ([]invokeMethodCandidate, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	methods, err := internalgrpc.ListMethodsConn(ctx, conn)
	if err != nil {
		return nil, err
	}
	return invokeMethodCandidatesFromQualifiedList(methods), nil
}

func invokeMethodCandidatesFromQualifiedList(methods []string) []invokeMethodCandidate {
	out := make([]invokeMethodCandidate, 0, len(methods))
	for _, entry := range methods {
		normalized := strings.Trim(strings.TrimSpace(entry), "/")
		if normalized == "" {
			continue
		}
		service := ""
		method := normalized
		if idx := strings.LastIndex(normalized, "/"); idx >= 0 {
			service = normalized[:idx]
			method = normalized[idx+1:]
		}
		if strings.TrimSpace(method) == "" {
			continue
		}
		out = append(out, invokeMethodCandidate{
			service: service,
			method:  method,
		})
	}
	return out
}

func completeInvokePayload(
	cmd *cobra.Command,
	target string,
	method string,
	trailingArgs []string,
	toComplete string,
) []string {
	flagCmd := completionFlagCommand(cmd)
	candidates, err := discoverInvokeMethodCandidates(target)
	if err != nil {
		return completeCommandFlags(flagCmd, toComplete)
	}

	seen := make(map[string]int)
	out := make([]string, 0)
	add := func(token string) {
		key := completionTokenKey(token)
		if idx, ok := seen[key]; ok {
			if !strings.Contains(out[idx], "\t") && strings.Contains(token, "\t") {
				out[idx] = token
			}
			return
		}
		seen[key] = len(out)
		out = append(out, token)
	}

	for _, candidate := range candidates {
		if !matchesInvokePayloadMethod(candidate, method) {
			continue
		}
		for _, seq := range candidate.examples {
			next, ok := nextExampleToken(seq, trailingArgs, toComplete)
			if !ok {
				continue
			}
			add(next)
		}
	}

	for _, flag := range completeCommandFlags(flagCmd, toComplete) {
		add(flag)
	}
	return out
}

func nextExampleToken(seq []string, typed []string, toComplete string) (string, bool) {
	visible := visibleExampleSequence(seq)
	if len(visible) <= len(typed) {
		return "", false
	}

	for i, token := range typed {
		if !exampleTokenMatches(visible[i], token) {
			return "", false
		}
	}

	candidate := visible[len(typed)]
	display := candidate
	if looksLikeJSON(candidate) {
		display = shellQuoteJSON(candidate)
	}
	if toComplete != "" &&
		!strings.HasPrefix(display, toComplete) &&
		!strings.HasPrefix(candidate, toComplete) {
		return "", false
	}
	return display, true
}

func visibleExampleSequence(seq []string) []string {
	if len(seq) == 0 {
		return nil
	}
	if isEmptyJSON(seq[0]) {
		return seq[1:]
	}
	return seq
}

func exampleTokenMatches(expected string, actual string) bool {
	unquotedExpected := unquote(expected)
	unquotedActual := unquote(actual)
	if looksLikeJSON(unquotedExpected) || looksLikeJSON(unquotedActual) {
		return unquotedExpected == unquotedActual
	}
	return strings.EqualFold(unquotedExpected, unquotedActual)
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') ||
			(s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func shellQuoteJSON(json string) string {
	return "'" + json + "'"
}

func isEmptyJSON(s string) bool {
	trimmed := strings.TrimSpace(s)
	return trimmed == "" || trimmed == "{}" || trimmed == "[]"
}

func matchesInvokePayloadMethod(candidate invokeMethodCandidate, method string) bool {
	trimmedMethod := strings.TrimSpace(method)
	if trimmedMethod == "" {
		return false
	}
	if strings.EqualFold(candidate.method, trimmedMethod) {
		return true
	}
	service := strings.TrimSpace(candidate.service)
	if service == "" {
		return false
	}
	return strings.EqualFold(service+"/"+candidate.method, trimmedMethod)
}

func completionTokenKey(token string) string {
	if idx := strings.IndexRune(token, '\t'); idx >= 0 {
		return token[:idx]
	}
	return token
}

func completionFlagCommand(cmd *cobra.Command) *cobra.Command {
	if cmd == nil {
		return nil
	}
	if root := cmd.Root(); root != nil {
		return root
	}
	return cmd
}

func formatInvokeMethodCompletions(methods []invokeMethodCandidate, toComplete string) []string {
	counts := make(map[string]int, len(methods))
	for _, method := range methods {
		counts[method.method]++
	}

	seen := make(map[string]struct{}, len(methods))
	out := make([]string, 0, len(methods))
	for _, method := range methods {
		candidate := method.method
		if counts[method.method] > 1 && strings.TrimSpace(method.service) != "" {
			candidate = strings.TrimSpace(method.service) + "/" + method.method
		}
		if !matchesInvokeMethodCompletion(candidate, method.method, toComplete) {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}

	sort.Strings(out)
	return out
}

func matchesInvokeMethodCompletion(candidate string, shortMethod string, toComplete string) bool {
	trimmed := strings.TrimSpace(toComplete)
	if trimmed == "" {
		return true
	}
	return strings.HasPrefix(candidate, trimmed) || strings.HasPrefix(shortMethod, trimmed)
}
