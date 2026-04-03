package cli

import (
	"fmt"

	"github.com/organic-programming/grace-op/api"
	"github.com/spf13/cobra"
)

func addMiscCommands(root *cobra.Command, version string) {
	root.AddCommand(
		newDiscoverCmd(),
		newInvokeCmd(),
		newInspectCmd(),
		newDoCmd(),
		newMCPCmd(version),
		newToolsCmd(),
		newEnvCmd(),
		newServeCmd(),
		newVersionCmd(),
	)
}

func newDiscoverCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "discover",
		Short: "List available holons",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			return runCommandCode(cmdDiscover(format))
		},
	}
}

func newInspectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "inspect <slug-or-host:port>",
		Short:             "Inspect a holon's API offline or via Describe",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeHolonSlugs,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			return runCommandCode(cmdInspect(format, currentRuntimeOptions(), inspectArgsFromCommand(cmd, args[0])))
		},
	}
	cmd.Flags().Bool("json", false, "render inspect output as JSON")
	addDiscoveryFlags(cmd)
	return cmd
}

func newDoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "do <holon> <sequence> [--param=value ...]",
		Short:              "Run a declared manifest sequence",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeRawCommand(cmd, args, true, cmdDo)
		},
	}
	cmd.Flags().Bool("dry-run", false, "print the sequence plan without executing it")
	cmd.Flags().Bool("continue-on-error", false, "continue running later steps after an error")
	return cmd
}

func newMCPCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "mcp <slug-or-uri> [slug2...]",
		Short: "Start an MCP server for one or more holons",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runCommandCode(cmdMCP(currentRuntimeOptions(), args, version))
		},
	}
}

func newToolsCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "tools <slug> [--format <fmt>]",
		Short:              "Output tool definitions for a holon",
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeRawCommand(cmd, args, false, cmdTools)
		},
	}
}

func newEnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Print resolved OPPATH, OPBIN, and ROOT",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			return runCommandCode(cmdEnv(format, envArgsFromCommand(cmd)))
		},
	}
	cmd.Flags().Bool("init", false, "create the runtime directories if they do not exist")
	cmd.Flags().Bool("shell", false, "print a shell snippet for PATH setup")
	return cmd
}

func newServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start OP's own gRPC server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCommandCode(cmdServe(serveArgsFromCommand(cmd)))
		},
	}
	cmd.Flags().StringArray("listen", nil, "listen URI (repeatable)")
	cmd.Flags().String("port", "", "listen port shorthand for tcp://:<port>")
	cmd.Flags().Bool("reflect", false, "enable gRPC reflection")
	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show the op version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCommandCode(func() int {
				format, err := currentFormat()
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "op version: %v\n", err)
					return 1
				}

				response := api.VersionWithString(cmd.Root().Version)
				out := api.FormatResponse(api.Format(format), response)
				if out != "" {
					fmt.Fprintln(cmd.OutOrStdout(), out)
				}
				return 0
			}())
		},
	}
}

func inspectArgsFromCommand(cmd *cobra.Command, target string) []string {
	args := make([]string, 0, 10)
	if jsonOutput, _ := cmd.Flags().GetBool("json"); jsonOutput {
		args = append(args, "--json")
	}
	args = append(args, selectedDiscoveryArgs(cmd)...)
	args = append(args, target)
	return args
}

func envArgsFromCommand(cmd *cobra.Command) []string {
	args := make([]string, 0, 2)
	if initDirs, _ := cmd.Flags().GetBool("init"); initDirs {
		args = append(args, "--init")
	}
	if shell, _ := cmd.Flags().GetBool("shell"); shell {
		args = append(args, "--shell")
	}
	return args
}

func serveArgsFromCommand(cmd *cobra.Command) []string {
	args := make([]string, 0, 8)
	listen, _ := cmd.Flags().GetStringArray("listen")
	for _, uri := range listen {
		args = append(args, "--listen", uri)
	}
	if port, _ := cmd.Flags().GetString("port"); port != "" {
		args = append(args, "--port", port)
	}
	if reflectEnabled, _ := cmd.Flags().GetBool("reflect"); reflectEnabled {
		args = append(args, "--reflect")
	}
	return args
}
