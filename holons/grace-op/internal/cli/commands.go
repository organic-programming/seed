// Package cli implements OP's command routing — transport-chain dispatch,
// URI dispatch, and OP's own commands.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"

	sdkdiscover "github.com/organic-programming/go-holons/pkg/discover"
	holonserve "github.com/organic-programming/go-holons/pkg/serve"
	"github.com/organic-programming/grace-op/api"
	openv "github.com/organic-programming/grace-op/internal/env"
	"github.com/organic-programming/grace-op/internal/grpcclient"
	"github.com/organic-programming/grace-op/internal/holons"
	"github.com/organic-programming/grace-op/internal/identity"
	"github.com/organic-programming/grace-op/internal/runpolicy"
	"github.com/organic-programming/grace-op/internal/server"

	"google.golang.org/grpc"
)

type commandRuntimeOptions struct {
	quiet   bool
	timeout int
	origin  bool
}

// Run dispatches the command and returns an exit code.
func Run(args []string, version string) int {
	gopts, args, err := parseGlobalOptions(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "op: %v\n", err)
		return 1
	}
	format := gopts.format
	quiet := gopts.quiet
	runtimeOpts := commandRuntimeOptions{
		quiet:   quiet,
		timeout: gopts.timeout,
		origin:  gopts.origin,
	}

	// Apply --root override via env var so all discovery paths see it.
	if gopts.root != "" {
		os.Setenv("OPROOT", gopts.root)
	}

	// --bin <slug>: print the resolved binary path and exit.
	if gopts.bin {
		if len(args) == 0 {
			fmt.Fprintln(os.Stderr, "op: --bin requires a holon slug")
			return 1
		}
		binaryPath, err := holons.ResolveBinary(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "op: %v\n", err)
			return 1
		}
		if len(args) == 1 {
			fmt.Println(binaryPath)
			return 0
		}
		fmt.Fprintf(os.Stderr, "bin: %s\n", binaryPath)
	}

	if len(args) == 0 {
		PrintUsage()
		return 1
	}

	cmd := args[0]
	rest := args[1:]

	switch cmd {
	// --- OP's own commands ---
	case "check":
		return cmdLifecycle(format, runtimeOpts, holons.OperationCheck, rest)
	case "build":
		return cmdLifecycle(format, runtimeOpts, holons.OperationBuild, rest)
	case "test":
		return cmdLifecycle(format, runtimeOpts, holons.OperationTest, rest)
	case "clean":
		return cmdLifecycle(format, runtimeOpts, holons.OperationClean, rest)
	case "install":
		return cmdInstall(format, runtimeOpts, rest)
	case "uninstall":
		return cmdUninstall(format, runtimeOpts, rest)
	case "mod":
		return cmdMod(format, quiet, rest)
	case "run":
		return cmdRun(format, runtimeOpts, rest)
	case "discover":
		return cmdDiscover(format)
	case "inspect":
		return cmdInspect(format, runtimeOpts, rest)
	case "do":
		return cmdDo(format, runtimeOpts, rest)
	case "mcp":
		return cmdMCP(runtimeOpts, rest, version)
	case "tools":
		return cmdTools(format, runtimeOpts, rest)
	case "env":
		return cmdEnv(format, rest)
	case "serve":
		return cmdServe(rest)
	case "version":
		fmt.Printf("op %s\n", version)
		return 0
	case "completion":
		return cmdCompletion(rest)
	case "__complete":
		return cmdComplete(rest)
	case "help":
		return cmdHelp(rest)
	case "--help", "-h":
		PrintUsage()
		return 0
	case "new", "list", "show":
		return cmdWho(format, runtimeOpts, cmd, rest)

	// --- URI dispatch: grpc://, tcp://, stdio://, unix://, ws://, http://, https:// ---
	default:
		if strings.HasPrefix(cmd, "grpc://") ||
			strings.HasPrefix(cmd, "tcp://") ||
			strings.HasPrefix(cmd, "stdio://") ||
			strings.HasPrefix(cmd, "unix://") ||
			strings.HasPrefix(cmd, "ws://") ||
			strings.HasPrefix(cmd, "wss://") ||
			strings.HasPrefix(cmd, "http://") ||
			strings.HasPrefix(cmd, "https://") {
			return cmdGRPC(format, cmd, rest)
		}
		// Direct binary dispatch: if cmd is an existing executable file,
		// launch it directly without discovery.
		if isExecutableFile(cmd) {
			return cmdDirectBinary(format, cmd, rest)
		}
		return cmdHolon(format, runtimeOpts, cmd, rest)
	}
}

// PrintUsage displays the help text.
func PrintUsage() {
	fmt.Print(`op — the Organic Programming CLI

Global flags (must come before <holon> or URI):
  -f, --format <text|json>              output format for RPC responses (default: text)
  -q, --quiet                           suppress progress and suggestions
  --root <path>                         override discovery root (default: cwd)
  --bin <slug>                          print the resolved binary path and exit

Holon dispatch (transport chain):
  op <holon> <command> [args]            dispatch via the SDK auto-connect chain
  op <holon> --clean <method> [--no-build] [json]
  op <holon> <method> [--no-build] [json]
                                         call a holon RPC; auto-build compiled slugs if needed
  op <binary-path> <method> [json]       call an executable directly (no discovery)

Direct gRPC URI dispatch:
  op grpc://<slug|host:port> <method>    gRPC auto-connect for slugs, direct TCP for host:port
  op tcp://<slug|host:port> <method>     force gRPC over TCP
  op stdio://<holon> <method>            force gRPC over stdio pipe (ephemeral)
  op unix://<path> <method>              gRPC over Unix socket
  op ws://<host:port> <method>           gRPC over WebSocket
  op wss://<host:port> <method>          gRPC over secure WebSocket
  op http://<host:port> <method>         gRPC over HTTP REST + SSE
  op https://<host:port> <method>        gRPC over secure HTTP REST + SSE
  op run <holon> [flags]                 build if needed, then launch in foreground
  op run <holon>:<port>                  shorthand for --listen tcp://:<port>

OP commands:
  op list [root]                         list local + cached holons natively
  op show <uuid-or-prefix>               display a holon identity natively
  op new [--json <payload>]              create a holon identity natively
  op new --list                          list shipped holon templates
  op new --template <name> <holon-name>  generate a holon scaffold from a template
  op inspect <slug|host:port> [--json]   inspect a holon's API offline or via Describe
  op do <holon> <sequence> [--param=value ...]
                                         run a declared manifest sequence
  op mcp <slug> [slug2...]               start an MCP server for one or more holons
  op mcp <tcp://host:port>               start an MCP server for a running gRPC server
  op tools <slug> [--format <fmt>]       output tool definitions (openai, anthropic, mcp)
  op check [<holon-or-path>]             validate the holon manifest and prerequisites
  op build [<holon-or-path>] [flags]     build a holon artifact via its runner
  op test [<holon-or-path>]              run a holon's test contract
  op clean [<holon-or-path>]             remove .op/ build outputs
  op install [<holon-or-path>] [flags]   install a pre-built artifact into $OPBIN
  op uninstall <holon>                   remove an installed artifact from $OPBIN
  op mod <command>                       manage holon.mod and holon.sum
  op env [--init] [--shell]              print resolved OPPATH / OPBIN / ROOT

Build flags:
  --clean                                      clean before building (cannot be combined with --dry-run)
  --target <macos|linux|windows|ios|ios-simulator|tvos|tvos-simulator|watchos|watchos-simulator|visionos|visionos-simulator|android|all>   platform target (default: current OS)
  --mode <debug|release|profile>               build mode (default: debug)
  --dry-run                                    print resolved plan, do not execute
  --no-sign                                    skip automatic ad-hoc signing for bundle artifacts

Install flags:
  --build                                      build before installing (default: install pre-built artifact as-is)
  --link-applications                          symlink installed .app bundles into /Applications (macOS only)

Run flags:
  --clean                                      clean before building and running (cannot be combined with --no-build)
  --listen <URI>                               listen address for service holons (default: tcp://127.0.0.1:0)
  --no-build                                   fail if the artifact is missing instead of building
  --target <...>                               pass build target through if a build is needed
  --mode <debug|release|profile>               pass build mode through if a build is needed

Dispatch flag:
  --clean                                      clean the slug target before auto-building and calling
  --no-build                                   fail if a slug-based RPC target is missing its built binary

  op discover                            list available holons
  op serve [--listen tcp://:9090]        start OP's own gRPC server
  op version                             show op version
  op help [command]                      this message or topic help (build, run)
`)
}

func cmdHelp(args []string) int {
	if len(args) == 0 {
		PrintUsage()
		return 0
	}

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "build":
		printBuildHelp()
		return 0
	case "run":
		printRunHelp()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "op help: unknown topic %q\n", args[0])
		return 1
	}
}

func printBuildHelp() {
	fmt.Print(`op build [<holon-or-path>] [flags]

Build a holon artifact via its runner.

Flags:
  --clean                                      clean before building (cannot be combined with --dry-run)
  --target <macos|linux|windows|ios|ios-simulator|tvos|tvos-simulator|watchos|watchos-simulator|visionos|visionos-simulator|android|all>   platform target (default: current OS)
  --mode <debug|release|profile>               build mode (default: debug)
  --dry-run                                    print resolved plan, do not execute
  --no-sign                                    skip automatic ad-hoc signing for bundle artifacts
`)
}

func printRunHelp() {
	fmt.Print(`op run <holon> [flags]
op run <holon>:<port>

Build a holon if needed, then launch it in the foreground.

Flags:
  --clean                                      clean before building and running (cannot be combined with --no-build)
  --listen <URI>                               listen address for service holons (default: tcp://127.0.0.1:0)
  --no-build                                   fail if the artifact is missing instead of building
  --target <...>                               pass build target through if a build is needed
  --mode <debug|release|profile>               pass build mode through if a build is needed
`)
}

// --- OP's own commands ---

type discoverEntry struct {
	Slug         string `json:"slug"`
	UUID         string `json:"uuid"`
	GivenName    string `json:"given_name"`
	FamilyName   string `json:"family_name"`
	Lang         string `json:"lang"`
	Clade        string `json:"clade"`
	Status       string `json:"status"`
	RelativePath string `json:"relative_path"`
	Origin       string `json:"origin"`
}

type discoverOutput struct {
	Entries           []discoverEntry `json:"entries"`
	InstalledBinaries []string        `json:"installed_binaries,omitempty"`
	PathBinaries      []string        `json:"path_binaries"`
}

func cmdDiscover(format Format) int {
	located, err := holons.DiscoverLocalHolons()
	if err != nil {
		fmt.Fprintf(os.Stderr, "op discover: %v\n", err)
		return 1
	}
	cached, err := holons.DiscoverCachedHolons()
	if err != nil {
		fmt.Fprintf(os.Stderr, "op discover: %v\n", err)
		return 1
	}

	entries := make([]discoverEntry, 0, len(located)+len(cached))
	for _, h := range append(append([]holons.LocalHolon{}, located...), cached...) {
		slug := h.Identity.Slug()
		if slug == "" {
			slug = filepath.Base(h.Dir)
		}
		entries = append(entries, discoverEntry{
			Slug:         slug,
			UUID:         h.Identity.UUID,
			GivenName:    h.Identity.GivenName,
			FamilyName:   h.Identity.FamilyName,
			Lang:         h.Identity.Lang,
			Clade:        h.Identity.Clade,
			Status:       h.Identity.Status,
			RelativePath: h.RelativePath,
			Origin:       discoverOrigin(h.Origin),
		})
	}
	installedHolons := holons.DiscoverInOPBIN()
	pathHolons := discoverInPath()

	if format == FormatJSON {
		payload := discoverOutput{
			Entries:           entries,
			InstalledBinaries: installedHolons,
			PathBinaries:      pathHolons,
		}
		out, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "op discover: %v\n", err)
			return 1
		}
		fmt.Println(string(out))
		return 0
	}

	printDiscoverTable(entries, installedHolons, pathHolons)
	return 0
}

func printDiscoverTable(entries []discoverEntry, installedHolons, pathHolons []string) {
	if len(entries) == 0 {
		fmt.Println("No holons found in known roots.")
	} else {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SLUG\tNAME\tLANG\tCLADE\tSTATUS\tORIGIN\tUUID")
		for _, entry := range entries {
			fmt.Fprintf(
				w,
				"%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				defaultDash(entry.Slug),
				discoverDisplayName(entry),
				defaultDash(entry.Lang),
				defaultDash(entry.Clade),
				defaultDash(entry.Status),
				defaultDash(entry.Origin),
				defaultDash(entry.UUID),
			)
		}
		_ = w.Flush()
	}

	if len(installedHolons) > 0 {
		fmt.Println("\nIn $OPBIN:")
		for _, name := range installedHolons {
			fmt.Printf("  %s\n", name)
		}
	}

	if len(pathHolons) > 0 {
		fmt.Println("\nIn $PATH:")
		for _, name := range pathHolons {
			fmt.Printf("  %s\n", name)
		}
	}
}

func discoverDisplayName(entry discoverEntry) string {
	name := strings.TrimSpace(entry.GivenName + " " + entry.FamilyName)
	if name == "" {
		return "-"
	}
	return name
}

func discoverOrigin(origin string) string {
	if strings.TrimSpace(origin) == "" {
		return "local"
	}
	return origin
}

func cmdServe(args []string) int {
	options := holonserve.ParseOptions(args)

	if err := holonserve.RunWithOptions(options.ListenURI, func(s *grpc.Server) {
		server.Register(s, api.RPCHandler{})
	}, options.Reflect); err != nil {
		fmt.Fprintf(os.Stderr, "op serve: %v\n", err)
		return 1
	}
	return 0
}

type runOptions struct {
	ListenURI      string
	ListenExplicit bool
	Clean          bool
	NoBuild        bool
	Target         string
	Mode           string
}

// cmdRun builds a holon artifact if needed, then launches it in the foreground.
func cmdRun(format Format, runtimeOpts commandRuntimeOptions, args []string) int {
	ui, args, _ := extractQuietFlag(args)
	quiet := runtimeOpts.quiet || ui.Quiet

	holonName, opts, err := parseRunArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "op run: %v\n", err)
		return 1
	}
	printer := commandProgress(format, quiet)
	defer printer.Close()

	emitOriginForExpression(runtimeOpts, holonName, sdkdiscover.INSTALLED|sdkdiscover.BUILT|sdkdiscover.SIBLINGS)

	printer.Step("resolving " + holonName + "...")

	var resolvedTarget *holons.Target
	if target, resolveErr := holons.ResolveTarget(holonName); resolveErr == nil && target != nil && target.ManifestErr == nil && target.Manifest != nil {
		resolvedTarget = target
	}

	if !opts.Clean {
		if binary := resolveInstalledBinary(holonName); binary != "" {
			printer.Step("launching " + holonName + "...")
			cmd, err := commandForInstalledArtifact(binary, resolvedTarget, opts.ListenURI)
			if err != nil {
				printer.Done("run failed", err)
				fmt.Fprintf(os.Stderr, "op run: %v\n", err)
				return 1
			}
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := runForeground(cmd); err != nil {
				if code, ok := commandExitCode(err); ok {
					return code
				}
				printer.Done("run failed", err)
				fmt.Fprintf(os.Stderr, "op run: %v\n", err)
				return 1
			}
			printer.Done(fmt.Sprintf("%s exited in %s", holonName, humanElapsed(printer)), nil)
			return 0
		}
	}

	target, err := holons.ResolveTarget(holonName)
	if err != nil {
		printer.Done("run failed", err)
		fmt.Fprintf(os.Stderr, "op run: %v\n", err)
		return 1
	}
	if target.ManifestErr != nil {
		printer.Done("run failed", target.ManifestErr)
		fmt.Fprintf(os.Stderr, "op run: %v\n", target.ManifestErr)
		return 1
	}
	if target.Manifest == nil {
		err := fmt.Errorf("no %s found in %s", identity.ProtoManifestFileName, target.RelativePath)
		printer.Done("run failed", err)
		fmt.Fprintf(os.Stderr, "op run: %v\n", err)
		return 1
	}

	if opts.Clean {
		if _, err := runCleanWithProgress(printer, target.Dir); err != nil {
			fmt.Fprintf(os.Stderr, "op run: %v\n", err)
			return 1
		}
	}

	ctx, err := holons.ResolveBuildContext(target.Manifest, holons.BuildOptions{
		Target: opts.Target,
		Mode:   opts.Mode,
	})
	if err != nil {
		printer.Done("run failed", err)
		fmt.Fprintf(os.Stderr, "op run: %v\n", err)
		return 1
	}
	if ctx.Target == "all" {
		err := fmt.Errorf("target %q cannot be launched", ctx.Target)
		printer.Done("run failed", err)
		fmt.Fprintf(os.Stderr, "op run: %v\n", err)
		return 1
	}

	isComposite := target.Manifest.Manifest.Kind == holons.KindComposite
	if isComposite && opts.ListenExplicit {
		err := fmt.Errorf("--listen is only supported for service holons")
		printer.Done("run failed", err)
		fmt.Fprintf(os.Stderr, "op run: %v\n", err)
		return 1
	}

	artifactPath := target.Manifest.ArtifactPath(ctx)
	if artifactPath == "" {
		err := fmt.Errorf("no artifact declared for target %q mode %q", ctx.Target, ctx.Mode)
		printer.Done("run failed", err)
		fmt.Fprintf(os.Stderr, "op run: %v\n", err)
		return 1
	}

	needBuild := opts.Clean
	if !needBuild {
		if _, err := os.Stat(artifactPath); err != nil {
			if !os.IsNotExist(err) {
				printer.Done("run failed", err)
				fmt.Fprintf(os.Stderr, "op run: %v\n", err)
				return 1
			}
			if opts.NoBuild {
				err := fmt.Errorf("artifact missing: %s", artifactPath)
				printer.Done("run failed", err)
				fmt.Fprintf(os.Stderr, "op run: %v\n", err)
				return 1
			}
			needBuild = true
		}
	}

	if needBuild {
		if _, err := holons.ExecuteLifecycle(holons.OperationBuild, target.Dir, holons.BuildOptions{
			Target:   opts.Target,
			Mode:     opts.Mode,
			Progress: printer,
		}); err != nil {
			printer.Done("run failed", err)
			fmt.Fprintf(os.Stderr, "op run: %v\n", err)
			return 1
		}
		if _, err := os.Stat(artifactPath); err != nil {
			if os.IsNotExist(err) {
				err = fmt.Errorf("artifact missing: %s", artifactPath)
			}
			printer.Done("run failed", err)
			fmt.Fprintf(os.Stderr, "op run: %v\n", err)
			return 1
		}
	}

	cmd, err := commandForArtifact(target.Manifest, ctx, opts.ListenURI)
	if err != nil {
		printer.Done("run failed", err)
		fmt.Fprintf(os.Stderr, "op run: %v\n", err)
		return 1
	}
	isApp := target.Manifest.Manifest.Kind == holons.KindComposite &&
		isMacAppBundle(target.Manifest.ArtifactPath(ctx))
	if isApp {
		printer.Step(holonName + " running — Cmd+Q to quit")
	} else {
		printer.Step("launching " + holonName + "...")
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := runForeground(cmd); err != nil {
		if code, ok := commandExitCode(err); ok {
			return code
		}
		printer.Done("run failed", err)
		fmt.Fprintf(os.Stderr, "op run: %v\n", err)
		return 1
	}
	printer.Done(fmt.Sprintf("%s exited in %s", holonName, humanElapsed(printer)), nil)
	return 0
}

// cmdGRPC handles gRPC URI dispatching.
//
// Transport schemes:
//   - grpc://host:port <method>       → TCP to existing server
//   - grpc://host:port                → list available methods
//   - grpc://holon <method>           → auto-connect chain for slug targets
//   - tcp://host:port <method>        → TCP to existing server
//   - tcp://holon <method>            → forced TCP startup for slug targets
//   - stdio://holon <method>          → forced stdio pipe
//   - unix://path <method>            → Unix domain socket connection
func cmdGRPC(format Format, uri string, args []string) int {
	switch {
	case strings.HasPrefix(uri, "stdio://"):
		return cmdGRPCStdio(format, uri, args)
	case strings.HasPrefix(uri, "unix://"):
		return cmdGRPCDirect(format, "unix://"+trimURIAnyPrefix(uri, "unix://"), args)
	case strings.HasPrefix(uri, "ws://") || strings.HasPrefix(uri, "wss://"):
		return cmdGRPCWebSocket(format, uri, args)
	case strings.HasPrefix(uri, "tcp://"):
		target := trimURIAnyPrefix(uri, "tcp://")
		if isHostPortTarget(target) {
			return cmdGRPCDirect(format, target, args)
		}
		return cmdGRPCConnected(format, uri, target, args, "tcp")
	default:
		return cmdGRPCTCP(format, uri, args)
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

// cmdGRPCTCP handles grpc://host:port directly and grpc://holon via auto-connect.
func cmdGRPCTCP(format Format, uri string, args []string) int {
	target := strings.TrimPrefix(uri, "grpc://")
	if isHostPortTarget(target) {
		return cmdGRPCDirect(format, target, args)
	}
	return cmdGRPCConnected(format, uri, target, args, "auto")
}

// cmdGRPCStdio handles stdio://holon — launches the holon with
// serve --listen stdio:// and communicates via stdin/stdout pipes.
func cmdGRPCStdio(format Format, uri string, args []string) int {
	holonName := trimURIAnyPrefix(uri, "stdio://")
	return cmdGRPCConnected(format, uri, holonName, args, "stdio")
}

// cmdGRPCWebSocket handles ws://host:port[/path] and wss://...
// Connects to an existing WebSocket gRPC server.
func cmdGRPCWebSocket(format Format, uri string, args []string) int {
	wsURI := trimURIAnyPrefix(uri, "grpc+")

	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "op grpc: method required")
		fmt.Fprintf(os.Stderr, "usage: op %s <method>\n", uri)
		return 1
	}

	method := args[0]
	inputJSON := "{}"
	if len(args) > 1 {
		inputJSON = args[1]
	}

	// Ensure path includes /grpc if not specified
	if !strings.Contains(wsURI[5:], "/") { // skip "ws://" prefix
		wsURI += "/grpc"
	}

	result, err := grpcclient.DialWebSocket(wsURI, method, inputJSON)
	if err != nil {
		fmt.Fprintf(os.Stderr, "op grpc: %v\n", err)
		return 1
	}

	fmt.Println(formatRPCOutput(format, method, []byte(result.Output)))
	return 0
}

// cmdGRPCDirect calls an RPC on an existing gRPC server at the given address.
func cmdGRPCDirect(format Format, address string, args []string) int {
	if len(args) == 0 {
		methods, err := grpcclient.ListMethods(address)
		if err != nil {
			fmt.Fprintf(os.Stderr, "op grpc: %v\n", err)
			return 1
		}
		fmt.Printf("Available methods at %s:\n", address)
		for _, m := range methods {
			fmt.Printf("  %s\n", m)
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
		fmt.Fprintf(os.Stderr, "op grpc: %v\n", err)
		return 1
	}

	fmt.Println(formatRPCOutput(format, method, []byte(result.Output)))
	return 0
}

func discoverInPath() []string {
	return holons.DiscoverInPath()
}

// --- Namespace dispatch ---

// cmdHolon runs `op <holon> <command> [args...]` through the transport chain.
func cmdHolon(format Format, runtimeOpts commandRuntimeOptions, holon string, args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "op: missing command for holon %q\n", holon)
		return 1
	}

	method, inputJSON, cleanFirst, noBuild, err := mapHolonCommandToRPC(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "op: %v\n", err)
		return 1
	}
	if cleanFirst && noBuild {
		fmt.Fprintln(os.Stderr, "op: --clean cannot be combined with --no-build")
		return 1
	}
	emitOriginForExpression(runtimeOpts, holon, sdkdiscover.ALL)
	if cleanFirst {
		printer := commandProgress(format, runtimeOpts.quiet)
		defer printer.Close()
		if _, err := runCleanWithProgress(printer, holon); err != nil {
			fmt.Fprintf(os.Stderr, "op: %v\n", err)
			return 1
		}
	}
	return runConnectedRPC(format, "op", holon, method, inputJSON, "auto", noBuild)
}

func isHostPortTarget(target string) bool {
	_, _, err := net.SplitHostPort(strings.TrimSpace(target))
	return err == nil
}

func mapHolonCommandToRPC(args []string) (method string, inputJSON string, cleanFirst bool, noBuild bool, err error) {
	if len(args) == 0 {
		return "", "", false, false, fmt.Errorf("method required")
	}
	if args[0] == "--clean" {
		cleanFirst = true
		args = args[1:]
		if len(args) == 0 {
			return "", "", false, false, fmt.Errorf("method required")
		}
	}

	command := strings.TrimSpace(args[0])
	rest := args[1:]

	method = mapCommandNameToMethod(command)
	for _, arg := range rest {
		if strings.TrimSpace(arg) == "--clean" {
			return "", "", false, false, fmt.Errorf("--clean must come immediately before the method")
		}
	}
	if len(rest) > 0 && rest[0] == "--no-build" {
		noBuild = true
		rest = rest[1:]
	}
	if len(rest) > 0 && looksLikeJSON(rest[0]) {
		for _, arg := range rest[1:] {
			if strings.TrimSpace(arg) == "--no-build" {
				return "", "", false, false, fmt.Errorf("--no-build must come immediately after the method")
			}
		}
		return method, rest[0], cleanFirst, noBuild, nil
	}

	switch strings.ToLower(command) {
	case "list":
		if len(rest) > 0 {
			payload, err := json.Marshal(map[string]string{"rootDir": rest[0]})
			if err != nil {
				return "", "", false, false, err
			}
			return method, string(payload), cleanFirst, noBuild, nil
		}
		return method, "{}", cleanFirst, noBuild, nil
	case "show":
		if len(rest) < 1 {
			return "", "", false, false, fmt.Errorf("show requires <uuid>")
		}
		payload, err := json.Marshal(map[string]string{"uuid": rest[0]})
		if err != nil {
			return "", "", false, false, err
		}
		return method, string(payload), cleanFirst, noBuild, nil
	default:
		if len(rest) > 0 && rest[0] == "--no-build" {
			return "", "", false, false, fmt.Errorf("--no-build must come immediately after the method")
		}
		return method, "{}", cleanFirst, noBuild, nil
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

// cmdDispatch runs `op <holon> <command> [args...]` by finding the
// holon binary and executing it as a subprocess.
func cmdDispatch(holon string, args []string) int {
	// Try to find the holon binary by selector.
	binary, err := resolveHolon(holon)
	if err != nil {
		fmt.Fprintf(os.Stderr, "op: unknown holon %q\n", holon)
		fmt.Fprintln(os.Stderr, "Run 'op discover' to see available holons.")
		return 1
	}

	cmd := exec.Command(binary, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "op: %v\n", err)
		return 1
	}
	return 0
}

// resolveHolon finds a holon binary by selector.
func resolveHolon(name string) (string, error) {
	return holons.ResolveBinary(name)
}

func resolveInstalledBinary(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || strings.ContainsAny(trimmed, `/\`) {
		return ""
	}
	return holons.ResolveInstalledBinary(trimmed)
}

func commandForInstalledArtifact(path string, target *holons.Target, listenURI string) (*exec.Cmd, error) {
	var manifest *holons.LoadedManifest
	if target != nil {
		manifest = target.Manifest
	}
	path = holons.LaunchableArtifactPath(path, manifest)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		if isMacAppBundle(path) && runtime.GOOS == "darwin" {
			return openAppBundleCommand(path, manifest), nil
		}
		return nil, fmt.Errorf("artifact is not directly launchable: %s", path)
	}
	if isHTMLFile(path) {
		cmd, err := openFileCommand(path)
		if err != nil {
			return nil, err
		}
		return withCompositeRunEnv(cmd, manifest), nil
	}
	if target != nil && target.Manifest != nil && target.Manifest.Manifest.Kind == holons.KindComposite {
		return withCompositeRunEnv(exec.Command(path), manifest), nil
	}
	return exec.Command(path, "serve", "--listen", listenURI), nil
}

func parseRunArgs(args []string) (string, runOptions, error) {
	opts := runOptions{}
	var positional []string

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--listen":
			if i+1 >= len(args) {
				return "", opts, fmt.Errorf("--listen requires a value")
			}
			opts.ListenURI = args[i+1]
			opts.ListenExplicit = true
			i++
		case args[i] == "--clean":
			opts.Clean = true
		case args[i] == "--no-build":
			opts.NoBuild = true
		case args[i] == "--target":
			if i+1 >= len(args) {
				return "", opts, fmt.Errorf("--target requires a value")
			}
			opts.Target = args[i+1]
			i++
		case args[i] == "--mode":
			if i+1 >= len(args) {
				return "", opts, fmt.Errorf("--mode requires a value")
			}
			opts.Mode = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--"):
			return "", opts, fmt.Errorf("unknown flag %q", args[i])
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) == 0 {
		return "", opts, fmt.Errorf("requires <holon> [flags]")
	}
	if len(positional) > 1 {
		return "", opts, fmt.Errorf("accepts exactly one <holon>")
	}
	if opts.Clean && opts.NoBuild {
		return "", opts, fmt.Errorf("--clean cannot be combined with --no-build")
	}

	holonName := strings.TrimSpace(positional[0])
	if legacyName, legacyListen, ok := parseLegacyRunTarget(holonName); ok {
		if opts.ListenExplicit {
			return "", opts, fmt.Errorf("cannot combine --listen with <holon>:<port> shorthand")
		}
		holonName = legacyName
		opts.ListenURI = legacyListen
		opts.ListenExplicit = true
	}
	if holonName == "" {
		return "", opts, fmt.Errorf("requires <holon> [flags]")
	}

	listenURI, err := runpolicy.NormalizeRunListenURI(opts.ListenURI, opts.ListenExplicit)
	if err != nil {
		return "", opts, err
	}
	opts.ListenURI = listenURI

	return holonName, opts, nil
}

func parseLegacyRunTarget(value string) (string, string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.ContainsAny(trimmed, `/\`) {
		return "", "", false
	}
	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), "tcp://:" + strings.TrimSpace(parts[1]), true
}

func commandForArtifact(manifest *holons.LoadedManifest, ctx holons.BuildContext, listenURI string) (*exec.Cmd, error) {
	if manifest == nil {
		return nil, fmt.Errorf("manifest required")
	}
	if manifest.Manifest.Kind == holons.KindComposite {
		artifactPath := manifest.ArtifactPath(ctx)
		info, err := os.Stat(artifactPath)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			if isMacAppBundle(artifactPath) && runtime.GOOS == "darwin" {
				// -W waits for the app to quit
				return openAppBundleCommand(artifactPath, manifest), nil
			}
			return nil, fmt.Errorf("artifact is not directly launchable: %s", artifactPath)
		}
		if isHTMLFile(artifactPath) {
			cmd, err := openFileCommand(artifactPath)
			if err != nil {
				return nil, err
			}
			return withCompositeRunEnv(cmd, manifest), nil
		}
		return withCompositeRunEnv(exec.Command(artifactPath), manifest), nil
	}

	binaryPath := manifest.BinaryPath()
	if strings.TrimSpace(binaryPath) == "" {
		return nil, fmt.Errorf("no binary declared for %s", manifest.Name)
	}
	return exec.Command(binaryPath, "serve", "--listen", listenURI), nil
}

func isMacAppBundle(path string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(path)), ".app")
}

func isHTMLFile(path string) bool {
	lower := strings.ToLower(strings.TrimSpace(path))
	return strings.HasSuffix(lower, ".html") || strings.HasSuffix(lower, ".htm")
}

// openFileCommand returns an exec.Cmd that opens a file with the platform's
// default handler (browser for HTML). On macOS the -W flag makes open(1) wait
// until the launched app exits, which prevents op run from returning immediately.
func openFileCommand(path string) (*exec.Cmd, error) {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", "-W", path), nil
	case "linux":
		return exec.Command("xdg-open", path), nil
	case "windows":
		return exec.Command("cmd", "/c", "start", "", path), nil
	default:
		return nil, fmt.Errorf("cannot open %s on %s", path, runtime.GOOS)
	}
}

func openAppBundleCommand(path string, manifest *holons.LoadedManifest) *exec.Cmd {
	normalizeMacOSAppBundleMetadata(path, manifest)
	args := []string{"-W"}
	if manifest != nil && manifest.Manifest.Kind == holons.KindComposite {
		displayFamily := compositeDisplayFamily(manifest)
		args = append(args,
			"--env", "OP_ASSEMBLY_FAMILY="+compositeRunEnvValue("OP_ASSEMBLY_FAMILY", manifest.Manifest.FamilyName),
			"--env", "OP_ASSEMBLY_DISPLAY_FAMILY="+compositeRunEnvValue("OP_ASSEMBLY_DISPLAY_FAMILY", displayFamily),
			"--env", "OP_ASSEMBLY_TRANSPORT="+compositeRunEnvValue("OP_ASSEMBLY_TRANSPORT", manifest.Manifest.Transport),
		)
	}
	args = append(args, path)
	return exec.Command("open", args...)
}

func withCompositeRunEnv(cmd *exec.Cmd, manifest *holons.LoadedManifest) *exec.Cmd {
	if cmd == nil || manifest == nil || manifest.Manifest.Kind != holons.KindComposite {
		return cmd
	}

	env := cmd.Env
	if len(env) == 0 {
		env = os.Environ()
	}
	env = setCommandEnv(env, "OP_ASSEMBLY_FAMILY", manifest.Manifest.FamilyName)
	env = setCommandEnv(env, "OP_ASSEMBLY_DISPLAY_FAMILY", compositeDisplayFamily(manifest))
	env = setCommandEnv(env, "OP_ASSEMBLY_TRANSPORT", manifest.Manifest.Transport)
	cmd.Env = env
	return cmd
}

func setCommandEnv(env []string, key, value string) []string {
	prefix := key + "="
	preserved := false
	out := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			out = append(out, entry)
			preserved = true
			continue
		}
		out = append(out, entry)
	}
	if !preserved {
		out = append(out, prefix+value)
	}
	return out
}

func compositeRunEnvValue(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func normalizeMacOSAppBundleMetadata(path string, manifest *holons.LoadedManifest) {
	if runtime.GOOS != "darwin" || manifest == nil || manifest.Manifest.Kind != holons.KindComposite || !isMacAppBundle(path) {
		return
	}

	displayName := compositeBundleDisplayName(manifest)
	if strings.TrimSpace(displayName) == "" {
		return
	}

	plistPath := filepath.Join(path, "Contents", "Info.plist")
	updates := map[string]string{
		"CFBundleName":        displayName,
		"CFBundleDisplayName": displayName,
	}
	if bundleID := compositeBundleIdentifier(manifest); bundleID != "" {
		updates["CFBundleIdentifier"] = bundleID
	}

	changed, err := rewriteMacOSPlistStrings(plistPath, updates)
	if err != nil || !changed {
		normalizeMacOSAppLauncherConfig(path, displayName)
		return
	}

	normalizeMacOSAppLauncherConfig(path, displayName)
	_ = exec.Command("codesign", "--force", "--deep", "--sign", "-", path).Run()
}

func compositeBundleDisplayName(manifest *holons.LoadedManifest) string {
	family := compositeDisplayFamily(manifest)
	if family == "" {
		return ""
	}
	if strings.HasPrefix(family, "Gudule ") {
		return family
	}
	return "Gudule " + family
}

func compositeDisplayFamily(manifest *holons.LoadedManifest) string {
	if manifest == nil {
		return ""
	}

	family := strings.TrimSpace(manifest.Manifest.FamilyName)
	if family == "" {
		family = strings.TrimSpace(manifest.Name)
	}
	if family == "" {
		return ""
	}

	label := compositeHostUILabel(family)
	if label == "" || strings.Contains(family, "("+label+")") {
		return family
	}
	return family + " (" + label + ")"
}

func compositeHostUILabel(family string) string {
	switch compositeHostUIKey(family) {
	case "compose", "kotlinui":
		return "Kotlin UI"
	case "flutter":
		return "Flutter UI"
	case "swiftui":
		return "SwiftUI"
	case "dotnet":
		return ".NET UI"
	case "qt":
		return "Qt UI"
	case "web":
		return "Web UI"
	default:
		return ""
	}
}

func compositeHostUIKey(family string) string {
	parts := strings.Split(strings.TrimSpace(family), "-")
	if len(parts) < 2 {
		return ""
	}
	if strings.EqualFold(parts[len(parts)-1], "web") {
		return "web"
	}
	if len(parts) < 3 {
		return ""
	}
	return strings.ToLower(parts[1])
}

func compositeBundleIdentifier(manifest *holons.LoadedManifest) string {
	if manifest == nil {
		return ""
	}

	name := strings.TrimSpace(manifest.Name)
	if name == "" {
		return ""
	}

	sanitized := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '.' || r == '-':
			return r
		default:
			return '-'
		}
	}, name)
	return "org.organicprogramming." + sanitized
}

func rewriteMacOSPlistStrings(path string, updates map[string]string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	content := string(data)
	changed := false
	for key, value := range updates {
		next, updated := upsertPlistString(content, key, value)
		if updated {
			changed = true
			content = next
		}
	}

	if !changed {
		return false, nil
	}
	return true, os.WriteFile(path, []byte(content), 0o644)
}

func upsertPlistString(content, key, value string) (string, bool) {
	escapedValue := xmlPlistEscape(value)
	re := regexp.MustCompile(`(?s)<key>` + regexp.QuoteMeta(key) + `</key>\s*<string>.*?</string>`)
	replacement := "<key>" + key + "</key>\n\t<string>" + escapedValue + "</string>"
	if re.MatchString(content) {
		updated := re.ReplaceAllString(content, replacement)
		return updated, updated != content
	}

	insert := replacement + "\n"
	if strings.Contains(content, "</dict>") {
		updated := strings.Replace(content, "</dict>", "\t"+insert+"</dict>", 1)
		return updated, updated != content
	}
	return content, false
}

func normalizeMacOSAppLauncherConfig(bundlePath, displayName string) {
	cfgDir := filepath.Join(bundlePath, "Contents", "app")
	entries, err := os.ReadDir(cfgDir)
	if err != nil {
		return
	}

	changedAny := false
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".cfg") {
			continue
		}
		cfgPath := filepath.Join(cfgDir, entry.Name())
		changed, err := rewriteMacOSDockName(cfgPath, displayName)
		if err == nil && changed {
			changedAny = true
		}
	}

	if changedAny {
		_ = exec.Command("codesign", "--force", "--deep", "--sign", "-", bundlePath).Run()
	}
}

func rewriteMacOSDockName(path, displayName string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	content := string(data)
	line := "java-options=-Xdock:name=" + displayName
	re := regexp.MustCompile(`(?m)^java-options=-Xdock:name=.*$`)
	if re.MatchString(content) {
		updated := re.ReplaceAllString(content, line)
		if updated == content {
			return false, nil
		}
		return true, os.WriteFile(path, []byte(updated), 0o644)
	}

	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += line + "\n"
	return true, os.WriteFile(path, []byte(content), 0o644)
}

var plistEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	"\"", "&quot;",
	"'", "&apos;",
)

func xmlPlistEscape(value string) string {
	return plistEscaper.Replace(value)
}

func runForeground(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	signals := []os.Signal{os.Interrupt}
	if runtime.GOOS != "windows" {
		signals = append(signals, syscall.SIGTERM)
	}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, signals...)
	defer signal.Stop(sigCh)

	for {
		select {
		case err := <-waitCh:
			return err
		case sig := <-sigCh:
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
		}
	}
}

func commandExitCode(err error) (int, bool) {
	if err == nil {
		return 0, false
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), true
	}
	return 0, false
}

// --- Flag helpers ---

// flagValue extracts --key value from args. Returns "" if not found.
func flagValue(args []string, key string) string {
	for i, a := range args {
		if a == key && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// flagOrDefault returns the flag value if present, else the default.
func flagOrDefault(args []string, key, defaultVal string) string {
	if v := flagValue(args, key); v != "" {
		return v
	}
	return defaultVal
}

func isDiscoveryFlag(arg string) bool {
	switch strings.TrimSpace(arg) {
	case "--all", "--siblings", "--cwd", "--source", "--built", "--installed", "--cached":
		return true
	default:
		return false
	}
}

func addDiscoverySpecifier(current int, arg string) int {
	switch strings.TrimSpace(arg) {
	case "--siblings":
		return current | sdkdiscover.SIBLINGS
	case "--cwd":
		return current | sdkdiscover.CWD
	case "--source":
		return current | sdkdiscover.SOURCE
	case "--built":
		return current | sdkdiscover.BUILT
	case "--installed":
		return current | sdkdiscover.INSTALLED
	case "--cached":
		return current | sdkdiscover.CACHED
	case "--all":
		return sdkdiscover.ALL
	default:
		return current
	}
}

func specifiersFromFlags(args []string) int {
	specs := 0
	for _, arg := range args {
		if isDiscoveryFlag(arg) {
			specs = addDiscoverySpecifier(specs, arg)
		}
	}
	if specs == 0 {
		return sdkdiscover.ALL
	}
	return specs
}

func emitOriginForExpression(runtimeOpts commandRuntimeOptions, expression string, specifiers int) {
	if !runtimeOpts.origin {
		return
	}
	root := openv.Root()
	resolved := holons.ResolveRef(expression, &root, specifiers, runtimeOpts.timeout)
	if resolved.Error != "" || resolved.Ref == nil {
		return
	}
	path, layer, err := holons.OriginDetails(resolved.Ref, &root)
	if err != nil {
		return
	}
	fmt.Fprintf(os.Stderr, "origin: %s (%s)\n", path, layer)
}

type internalGlobalOptions struct {
	format  Format
	quiet   bool
	root    string
	bin     bool
	timeout int
	origin  bool
}

func parseGlobalOptions(args []string) (internalGlobalOptions, []string, error) {
	opts := internalGlobalOptions{format: FormatText}
	var remaining []string
	i := 0
	for i < len(args) {
		switch {
		case args[i] == "--quiet" || args[i] == "-q":
			opts.quiet = true
			i++
		case args[i] == "--bin":
			opts.bin = true
			i++
		case args[i] == "--origin":
			opts.origin = true
			i++
		case args[i] == "--root":
			if i+1 >= len(args) {
				return internalGlobalOptions{}, nil, fmt.Errorf("--root requires a path")
			}
			opts.root = args[i+1]
			i += 2
		case strings.HasPrefix(args[i], "--root="):
			opts.root = strings.TrimPrefix(args[i], "--root=")
			i++
		case args[i] == "--timeout":
			if i+1 >= len(args) {
				return internalGlobalOptions{}, nil, fmt.Errorf("--timeout requires milliseconds")
			}
			value, parseErr := strconv.Atoi(strings.TrimSpace(args[i+1]))
			if parseErr != nil || value < 0 {
				return internalGlobalOptions{}, nil, fmt.Errorf("invalid --timeout %q", args[i+1])
			}
			opts.timeout = value
			i += 2
		case strings.HasPrefix(args[i], "--timeout="):
			value, parseErr := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(args[i], "--timeout=")))
			if parseErr != nil || value < 0 {
				return internalGlobalOptions{}, nil, fmt.Errorf("invalid --timeout %q", strings.TrimPrefix(args[i], "--timeout="))
			}
			opts.timeout = value
			i++
		case args[i] == "--format" || args[i] == "-f":
			if i+1 >= len(args) {
				return internalGlobalOptions{}, nil, fmt.Errorf("%s requires a value (text or json)", args[i])
			}
			parsed, err := parseFormat(args[i+1])
			if err != nil {
				remaining = append(remaining, args[i], args[i+1])
				i += 2
				continue
			}
			opts.format = parsed
			i += 2
		case strings.HasPrefix(args[i], "--format="):
			parsed, err := parseFormat(strings.TrimPrefix(args[i], "--format="))
			if err != nil {
				remaining = append(remaining, args[i])
				i++
				continue
			}
			opts.format = parsed
			i++
		case strings.HasPrefix(args[i], "-f="):
			parsed, err := parseFormat(strings.TrimPrefix(args[i], "-f="))
			if err != nil {
				remaining = append(remaining, args[i])
				i++
				continue
			}
			opts.format = parsed
			i++
		default:
			remaining = append(remaining, args[i])
			i++
		}
	}
	return opts, remaining, nil
}

func parseGlobalFormat(args []string) (Format, []string, error) {
	gopts, remaining, err := parseGlobalOptions(args)
	return gopts.format, remaining, err
}

func parseFormat(value string) (Format, error) {
	switch Format(strings.ToLower(strings.TrimSpace(value))) {
	case FormatText:
		return FormatText, nil
	case FormatJSON:
		return FormatJSON, nil
	default:
		return "", fmt.Errorf("invalid --format %q (supported: text, json)", value)
	}
}
