package cli

import (
	"strconv"

	"github.com/spf13/cobra"
)

func addIdentityCommands(root *cobra.Command) {
	root.AddCommand(
		newListCmd(),
		newShowCmd(),
		newNewCmd(),
	)
}

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [root]",
		Short: "List local and cached holons",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			return runCommandCode(cmdWho(format, currentRuntimeOptions(), "list", listArgsFromCommand(cmd, args)))
		},
	}
	addDiscoveryFlags(cmd)
	cmd.Flags().Int("limit", -1, "maximum number of entries to return")
	return cmd
}

func newShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "show <uuid-or-prefix>",
		Short:             "Display a holon identity",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeHolonSlugs,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			return runCommandCode(cmdWho(format, currentRuntimeOptions(), "show", showArgsFromCommand(cmd, args[0])))
		},
	}
	addDiscoveryFlags(cmd)
	return cmd
}

func newNewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "new",
		Short:              "Create a holon identity or scaffold",
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeRawCommand(cmd, args, true, func(format Format, runtimeOpts commandRuntimeOptions, remaining []string) int {
				return cmdWho(format, runtimeOpts, "new", remaining)
			})
		},
	}
	cmd.Flags().String("json", "", "raw JSON payload for identity creation")
	cmd.Flags().Bool("list", false, "list shipped holon templates")
	cmd.Flags().String("template", "", "template name to scaffold")
	cmd.Flags().StringArray("set", nil, "template override in key=value form")
	return cmd
}

func listArgsFromCommand(cmd *cobra.Command, positional []string) []string {
	args := make([]string, 0, 10)
	args = append(args, selectedDiscoveryArgs(cmd)...)
	if limit, _ := cmd.Flags().GetInt("limit"); limit >= 0 {
		args = append(args, "--limit", strconv.Itoa(limit))
	}
	args = append(args, positional...)
	return args
}

func showArgsFromCommand(cmd *cobra.Command, target string) []string {
	args := make([]string, 0, 8)
	args = append(args, selectedDiscoveryArgs(cmd)...)
	args = append(args, target)
	return args
}
