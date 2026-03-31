package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/organic-programming/grace-op/api"
	"github.com/organic-programming/grace-op/internal/holons"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var rootCmd *cobra.Command

const (
	viperKeyFormat  = "output_format"
	viperKeyQuiet   = "quiet_mode"
	viperKeyRoot    = "discovery_root"
	viperKeyBin     = "resolve_bin"
	viperKeyPath    = "runtime_path"
	viperKeyOPBIN   = "runtime_bin"
	viperKeyTimeout = "resolve_timeout"
	viperKeyOrigin  = "show_origin"
)

type commandExitError struct {
	code int
}

func (e commandExitError) Error() string {
	return fmt.Sprintf("exit status %d", e.code)
}

func init() {
	configureViper()
}

// Execute runs the Cobra CLI using os.Args and returns the process exit code.
func Execute() int {
	return Run(os.Args[1:], api.VersionString())
}

// Run executes the CLI with the provided arguments and version.
func Run(args []string, version string) int {
	originalEnv := captureGlobalEnv()
	defer restoreGlobalEnv(originalEnv)

	rootCmd = newRootCmd(version)
	if isShellCompletionRequest(args) {
		rootCmd.SetErr(io.Discard)
	}
	rootCmd.SetArgs(args)

	if err := rootCmd.Execute(); err != nil {
		var exitErr commandExitError
		if errors.As(err, &exitErr) {
			return exitErr.code
		}
		fmt.Fprintf(os.Stderr, "op: %v\n", err)
		return 1
	}
	return 0
}

type capturedEnv struct {
	oproot  string
	oppath  string
	opbin   string
	hasRoot bool
	hasPath bool
	hasBin  bool
}

func captureGlobalEnv() capturedEnv {
	root, hasRoot := os.LookupEnv("OPROOT")
	path, hasPath := os.LookupEnv("OPPATH")
	bin, hasBin := os.LookupEnv("OPBIN")
	return capturedEnv{
		oproot:  root,
		oppath:  path,
		opbin:   bin,
		hasRoot: hasRoot,
		hasPath: hasPath,
		hasBin:  hasBin,
	}
}

func restoreGlobalEnv(state capturedEnv) {
	restoreEnvValue("OPROOT", state.hasRoot, state.oproot)
	restoreEnvValue("OPPATH", state.hasPath, state.oppath)
	restoreEnvValue("OPBIN", state.hasBin, state.opbin)
}

func restoreEnvValue(key string, present bool, value string) {
	if present {
		_ = os.Setenv(key, value)
		return
	}
	_ = os.Unsetenv(key)
}

func configureViper() {
	viper.Reset()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
}

func newRootCmd(version string) *cobra.Command {
	configureViper()

	cmd := &cobra.Command{
		Use:   "op",
		Short: "Organic Programming CLI",
		Long:  "op dispatches holons over the transport chain, talks directly to gRPC transports, and manages local holon lifecycle commands.",
		Example: strings.Join([]string{
			"op <holon> <method> [json]",
			"op invoke <holon> <method> [json]",
			"op <holon> --clean <method> [--no-build] [json]",
			"op grpc://<slug|host:port> <method>",
			"op run <holon>",
			"op run <holon>:<port>",
			"op build [<holon-or-path>] --clean",
			"op tools <slug> --format openai",
		}, "\n"),
		Args:               cobra.ArbitraryArgs,
		ValidArgsFunction:  completeRootFallbackArgs,
		DisableFlagParsing: true,
		SilenceErrors:      true,
		SilenceUsage:       true,
		Version:            version,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if cmd == rootCmd || cmd.DisableFlagParsing {
				return nil
			}
			if _, err := currentFormat(); err != nil {
				return err
			}
			return applyGlobalEnvOverrides(internalGlobalOptions{
				root:  strings.TrimSpace(viper.GetString(viperKeyRoot)),
				path:  strings.TrimSpace(viper.GetString(viperKeyPath)),
				opbin: strings.TrimSpace(viper.GetString(viperKeyOPBIN)),
			})
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeRootFallback(cmd, args)
		},
	}

	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)

	registerRootPersistentFlags(cmd)
	registerRootCommands(cmd, version)
	cmd.InitDefaultCompletionCmd()

	return cmd
}

func completeRootFallbackArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	args = normalizeCompletionArgs(args, toComplete)

	if len(args) == 0 {
		if strings.HasPrefix(strings.TrimSpace(toComplete), "-") {
			return completeCommandFlags(cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveDefault
	}

	if err := applyCurrentEnvOverrides(); err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	switch len(args) {
	case 1:
		if strings.HasPrefix(strings.TrimSpace(toComplete), "-") {
			return completeCommandFlags(cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
		}
		return completeInvokeMethods(args[0], toComplete), cobra.ShellCompDirectiveNoFileComp
	default:
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func normalizeCompletionArgs(args []string, toComplete string) []string {
	if strings.TrimSpace(toComplete) != "" || len(args) == 0 {
		return args
	}
	if strings.TrimSpace(args[len(args)-1]) != "" {
		return args
	}
	return args[:len(args)-1]
}

func registerRootPersistentFlags(cmd *cobra.Command) {
	flags := cmd.PersistentFlags()
	flags.StringP("format", "f", string(FormatText), "output format for RPC responses (text or json)")
	flags.BoolP("quiet", "q", false, "suppress progress and suggestions")
	flags.String("root", "", "override discovery root")
	flags.String("bin", "", "print the resolved binary path and exit")
	flags.String("path", "", "override OPPATH")
	flags.String("opbin", "", "override OPBIN")
	flags.Int("timeout", 0, "discovery timeout in milliseconds")
	flags.Bool("origin", false, "print the resolved origin for matching targets")

	_ = flags.MarkHidden("path")
	_ = flags.MarkHidden("opbin")
	_ = flags.MarkHidden("timeout")
	_ = flags.MarkHidden("origin")

	mustBindPFlag(viperKeyFormat, flags.Lookup("format"))
	mustBindPFlag(viperKeyQuiet, flags.Lookup("quiet"))
	mustBindPFlag(viperKeyRoot, flags.Lookup("root"))
	mustBindPFlag(viperKeyBin, flags.Lookup("bin"))
	mustBindPFlag(viperKeyPath, flags.Lookup("path"))
	mustBindPFlag(viperKeyOPBIN, flags.Lookup("opbin"))
	mustBindPFlag(viperKeyTimeout, flags.Lookup("timeout"))
	mustBindPFlag(viperKeyOrigin, flags.Lookup("origin"))

	mustBindEnv(viperKeyFormat, "OPFORMAT")
	mustBindEnv(viperKeyQuiet, "OPQUIET")
	mustBindEnv(viperKeyRoot, "OPROOT")
	mustBindEnv(viperKeyPath, "OPPATH")
	mustBindEnv(viperKeyOPBIN, "OPBIN")
	mustBindEnv(viperKeyTimeout, "OPTIMEOUT")
	mustBindEnv(viperKeyOrigin, "OPORIGIN")
}

func mustBindPFlag(key string, flag *pflag.Flag) {
	if err := viper.BindPFlag(key, flag); err != nil {
		panic(err)
	}
}

func mustBindEnv(key string, env ...string) {
	args := append([]string{key}, env...)
	if err := viper.BindEnv(args...); err != nil {
		panic(err)
	}
}

func executeRootFallback(cmd *cobra.Command, rawArgs []string) error {
	opts, args, err := manualGlobalOptions(rawArgs, true)
	if err != nil {
		return err
	}
	if isHelpInvocation(args) {
		return cmd.Help()
	}
	if err := applyGlobalEnvOverrides(opts); err != nil {
		return err
	}

	format := opts.format
	runtimeOpts := opts.runtimeOptions()

	if strings.TrimSpace(opts.bin) != "" {
		binaryPath, err := holons.ResolveBinary(opts.bin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "op: %v\n", err)
			return commandExitError{code: 1}
		}
		if len(args) == 0 {
			fmt.Println(binaryPath)
			return nil
		}
		fmt.Fprintf(os.Stderr, "bin: %s\n", binaryPath)
		args = append([]string{opts.bin}, args...)
	}

	if len(args) == 0 {
		_ = cmd.Help()
		return commandExitError{code: 1}
	}

	target := args[0]
	rest := args[1:]

	switch {
	case isTransportURI(target):
		return commandExitError{code: cmdGRPC(format, target, rest)}
	case isExecutableFile(target):
		return commandExitError{code: cmdDirectBinary(format, target, rest)}
	default:
		return commandExitError{code: cmdHolon(format, runtimeOpts, target, rest)}
	}
}

func registerRootCommands(root *cobra.Command, version string) {
	addLifecycleCommands(root)
	addIdentityCommands(root)
	addModCommands(root)
	addMiscCommands(root, version)
}

func currentFormat() (Format, error) {
	value := strings.TrimSpace(viper.GetString(viperKeyFormat))
	if value == "" {
		return FormatText, nil
	}
	return parseFormat(value)
}

func currentRuntimeOptions() commandRuntimeOptions {
	return commandRuntimeOptions{
		quiet:   viper.GetBool(viperKeyQuiet),
		timeout: viper.GetInt(viperKeyTimeout),
		origin:  viper.GetBool(viperKeyOrigin),
	}
}

func manualGlobalOptions(args []string, consumeFormat bool) (internalGlobalOptions, []string, error) {
	defaults, err := globalOptionsFromEnvironment()
	if err != nil {
		return internalGlobalOptions{}, nil, err
	}
	return parseGlobalOptionsConfigured(args, defaults, globalParseConfig{consumeFormat: consumeFormat})
}

func globalOptionsFromEnvironment() (internalGlobalOptions, error) {
	format, err := currentFormat()
	if err != nil {
		return internalGlobalOptions{}, err
	}
	return internalGlobalOptions{
		format:  format,
		quiet:   viper.GetBool(viperKeyQuiet),
		root:    strings.TrimSpace(viper.GetString(viperKeyRoot)),
		path:    strings.TrimSpace(viper.GetString(viperKeyPath)),
		opbin:   strings.TrimSpace(viper.GetString(viperKeyOPBIN)),
		timeout: viper.GetInt(viperKeyTimeout),
		origin:  viper.GetBool(viperKeyOrigin),
	}, nil
}

func applyGlobalEnvOverrides(opts internalGlobalOptions) error {
	if strings.TrimSpace(opts.root) != "" {
		if err := os.Setenv("OPROOT", opts.root); err != nil {
			return err
		}
	}
	if strings.TrimSpace(opts.path) != "" {
		if err := os.Setenv("OPPATH", opts.path); err != nil {
			return err
		}
	}
	if strings.TrimSpace(opts.opbin) != "" {
		if err := os.Setenv("OPBIN", opts.opbin); err != nil {
			return err
		}
	}
	return nil
}

func (opts internalGlobalOptions) runtimeOptions() commandRuntimeOptions {
	return commandRuntimeOptions{
		quiet:   opts.quiet,
		timeout: opts.timeout,
		origin:  opts.origin,
	}
}

func runCommandCode(code int) error {
	if code == 0 {
		return nil
	}
	return commandExitError{code: code}
}

func executeRawCommand(cmd *cobra.Command, rawArgs []string, consumeFormat bool, handler func(Format, commandRuntimeOptions, []string) int) error {
	opts, args, err := manualGlobalOptions(rawArgs, consumeFormat)
	if err != nil {
		return err
	}
	if isHelpInvocation(args) {
		return cmd.Help()
	}
	if err := applyGlobalEnvOverrides(opts); err != nil {
		return err
	}
	return runCommandCode(handler(opts.format, opts.runtimeOptions(), args))
}

func isHelpInvocation(args []string) bool {
	return len(args) == 1 && isHelpToken(args[0])
}

func isHelpToken(arg string) bool {
	switch strings.TrimSpace(arg) {
	case "-h", "--help":
		return true
	default:
		return false
	}
}

func isTransportURI(target string) bool {
	switch {
	case strings.HasPrefix(target, "grpc://"):
		return true
	case strings.HasPrefix(target, "tcp://"):
		return true
	case strings.HasPrefix(target, "stdio://"):
		return true
	case strings.HasPrefix(target, "unix://"):
		return true
	case strings.HasPrefix(target, "ws://"):
		return true
	case strings.HasPrefix(target, "wss://"):
		return true
	case strings.HasPrefix(target, "http://"):
		return true
	case strings.HasPrefix(target, "https://"):
		return true
	default:
		return false
	}
}

func addDiscoveryFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.Bool("all", false, "search every discovery layer")
	flags.Bool("siblings", false, "search sibling workspaces")
	flags.Bool("cwd", false, "search the current working directory")
	flags.Bool("source", false, "search source holons only")
	flags.Bool("built", false, "search built artifacts only")
	flags.Bool("installed", false, "search installed binaries only")
	flags.Bool("cached", false, "search cached holons only")
}

func selectedDiscoveryArgs(cmd *cobra.Command) []string {
	flags := []string{"all", "siblings", "cwd", "source", "built", "installed", "cached"}
	args := make([]string, 0, len(flags))
	for _, name := range flags {
		value, err := cmd.Flags().GetBool(name)
		if err == nil && value {
			args = append(args, "--"+name)
		}
	}
	return args
}

func completeHolonSlugs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return completeCommandFlags(cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
	if err := applyCurrentEnvOverrides(); err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	seen := make(map[string]struct{})
	out := make([]string, 0)
	appendSlug := func(value string) {
		value = normalizeHolonCompletionValue(value)
		if value == "" || !strings.HasPrefix(value, toComplete) {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}

	if local, err := holons.DiscoverLocalHolons(); err == nil {
		for _, holon := range local {
			slug := holon.Identity.Slug()
			if slug == "" {
				slug = filepath.Base(holon.Dir)
			}
			appendHolonCompletionIdentity(appendSlug, slug, holon.Identity.Aliases)
		}
	}
	if cached, err := holons.DiscoverCachedHolons(); err == nil {
		for _, holon := range cached {
			slug := holon.Identity.Slug()
			if slug == "" {
				slug = filepath.Base(holon.Dir)
			}
			appendHolonCompletionIdentity(appendSlug, slug, holon.Identity.Aliases)
		}
	}
	for _, entry := range holons.DiscoverInOPBIN() {
		appendInstalledHolonCompletionEntry(appendSlug, entry)
	}

	return out, cobra.ShellCompDirectiveNoFileComp
}

func completeCommandFlags(cmd *cobra.Command, toComplete string) []string {
	if cmd == nil {
		return nil
	}

	trimmed := strings.TrimSpace(toComplete)
	if trimmed != "" && !strings.HasPrefix(trimmed, "-") {
		return nil
	}

	seen := make(map[string]struct{})
	out := make([]string, 0)
	appendFlags := func(flags *pflag.FlagSet) {
		if flags == nil {
			return
		}
		flags.VisitAll(func(flag *pflag.Flag) {
			if flag == nil || flag.Hidden {
				return
			}
			name := "--" + flag.Name
			if trimmed != "" && !strings.HasPrefix(name, trimmed) {
				return
			}
			if _, ok := seen[name]; ok {
				return
			}
			seen[name] = struct{}{}
			if usage := strings.TrimSpace(flag.Usage); usage != "" {
				out = append(out, name+"\t"+usage)
				return
			}
			out = append(out, name)
		})
	}

	appendFlags(cmd.InheritedFlags())
	appendFlags(cmd.LocalFlags())
	if trimmed == "" {
		if _, ok := seen["--help"]; !ok {
			out = append(out, "--help\thelp for "+cmd.Name())
		}
	}
	return out
}

func appendHolonCompletionIdentity(appendSlug func(string), slug string, aliases []string) {
	appendSlug(slug)
	for _, alias := range aliases {
		appendSlug(alias)
	}
}

func appendInstalledHolonCompletionEntry(appendSlug func(string), entry string) {
	name, path := splitCompletionEntry(entry)
	appendSlug(name)
	for _, candidate := range installedHolonCompletionCandidates(path) {
		appendSlug(candidate)
	}
}

func splitCompletionEntry(entry string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(entry), " -> ", 2)
	if len(parts) == 2 {
		return normalizeHolonCompletionValue(parts[0]), strings.TrimSpace(parts[1])
	}
	return normalizeHolonCompletionValue(entry), ""
}

func installedHolonCompletionCandidates(path string) []string {
	pkg, err := readInstalledHolonPackage(path)
	if err != nil || pkg == nil {
		return nil
	}

	candidates := make([]string, 0, 2+len(pkg.Aliases))
	candidates = append(candidates, strings.TrimSpace(pkg.Slug))
	candidates = append(candidates, pkg.Aliases...)
	if entrypoint := strings.TrimSpace(pkg.Entrypoint); entrypoint != "" {
		candidates = append(candidates, entrypoint)
	}
	return candidates
}

func readInstalledHolonPackage(path string) (*holons.HolonPackageJSON, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" || !strings.HasSuffix(strings.ToLower(trimmed), ".holon") {
		return nil, fmt.Errorf("not a holon package")
	}
	info, err := os.Stat(trimmed)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a holon package directory")
	}

	data, err := os.ReadFile(filepath.Join(trimmed, ".holon.json"))
	if err != nil {
		return nil, err
	}

	var pkg holons.HolonPackageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}
	return &pkg, nil
}

func applyCurrentEnvOverrides() error {
	return applyGlobalEnvOverrides(internalGlobalOptions{
		root:  strings.TrimSpace(viper.GetString(viperKeyRoot)),
		path:  strings.TrimSpace(viper.GetString(viperKeyPath)),
		opbin: strings.TrimSpace(viper.GetString(viperKeyOPBIN)),
	})
}

func normalizeHolonCompletionValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.Contains(value, " -> ") {
		value = strings.TrimSpace(strings.SplitN(value, " -> ", 2)[0])
	}
	switch {
	case strings.HasSuffix(value, ".holon"):
		value = strings.TrimSuffix(value, ".holon")
	case strings.HasSuffix(value, ".app"):
		value = strings.TrimSuffix(value, ".app")
	case strings.HasSuffix(value, ".exe"):
		value = strings.TrimSuffix(value, ".exe")
	}
	return strings.TrimSpace(value)
}

func isShellCompletionRequest(args []string) bool {
	for _, arg := range args {
		switch strings.TrimSpace(arg) {
		case cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd:
			return true
		}
	}
	return false
}
