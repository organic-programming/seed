package cli

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func addModCommands(root *cobra.Command) {
	modCmd := &cobra.Command{
		Use:   "mod",
		Short: "Manage holon.mod and holon.sum",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return commandExitError{code: 1}
		},
	}

	modCmd.AddCommand(
		newModInitCmd(),
		newModAddCmd(),
		newModRemoveCmd(),
		newModTidyCmd(),
		newModPullCmd(),
		newModUpdateCmd(),
		newModListCmd(),
		newModGraphCmd(),
	)

	root.AddCommand(modCmd)
}

func newModInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init [holon-path]",
		Short: "Create a holon.mod file",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			return runCommandCode(cmdModInit(format, viper.GetBool(viperKeyQuiet), args))
		},
	}
}

func newModAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <module> [version]",
		Short: "Add a holon dependency",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			return runCommandCode(cmdModAdd(format, viper.GetBool(viperKeyQuiet), args))
		},
	}
}

func newModRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <module>",
		Short: "Remove a holon dependency",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			return runCommandCode(cmdModRemove(format, viper.GetBool(viperKeyQuiet), args))
		},
	}
}

func newModTidyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tidy",
		Short: "Prune and normalize dependency metadata",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			return runCommandCode(cmdModTidy(format, viper.GetBool(viperKeyQuiet), args))
		},
	}
}

func newModPullCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pull",
		Short: "Fetch deferred dependencies",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			return runCommandCode(cmdModPull(format, viper.GetBool(viperKeyQuiet), args))
		},
	}
}

func newModUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update [module]",
		Short: "Update one dependency or all dependencies",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			return runCommandCode(cmdModUpdate(format, viper.GetBool(viperKeyQuiet), args))
		},
	}
}

func newModListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List declared dependencies",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			return runCommandCode(cmdModList(format, viper.GetBool(viperKeyQuiet), args))
		},
	}
}

func newModGraphCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "graph",
		Short: "Render the dependency graph",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			return runCommandCode(cmdModGraph(format, viper.GetBool(viperKeyQuiet), args))
		},
	}
}
