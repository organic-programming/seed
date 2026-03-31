package cli

import (
	"fmt"

	"github.com/organic-programming/grace-op/internal/holons"
	"github.com/spf13/cobra"
)

func addLifecycleCommands(root *cobra.Command) {
	root.AddCommand(
		newBuildCmd(),
		newCheckCmd(),
		newTestCmd(),
		newCleanCmd(),
		newRunCmd(),
		newInstallCmd(),
		newUninstallCmd(),
	)
}

func newBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "build [holon-or-path]",
		Short:             "Build a holon artifact",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeHolonSlugs,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			buildArgs := lifecycleArgsFromCommand(cmd, args)
			installAfter, _ := cmd.Flags().GetBool("install")
			if installAfter {
				return runCommandCode(cmdBuildAndInstall(format, currentRuntimeOptions(), buildArgs))
			}
			return runCommandCode(cmdLifecycle(format, currentRuntimeOptions(), holons.OperationBuild, buildArgs))
		},
	}
	addLifecycleExecutionFlags(cmd, true)
	cmd.Flags().Bool("install", false, "install the artifact after a successful build")
	return cmd
}

func newCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check [holon-or-path]",
		Short: "Validate a holon manifest and prerequisites",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			return runCommandCode(cmdLifecycle(format, currentRuntimeOptions(), holons.OperationCheck, lifecycleArgsFromCommand(cmd, args)))
		},
	}
	addLifecycleExecutionFlags(cmd, false)
	return cmd
}

func newTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "test [holon-or-path]",
		Short:             "Run a holon's test contract",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeHolonSlugs,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			return runCommandCode(cmdLifecycle(format, currentRuntimeOptions(), holons.OperationTest, lifecycleArgsFromCommand(cmd, args)))
		},
	}
	addLifecycleExecutionFlags(cmd, false)
	return cmd
}

func newCleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "clean [holon-or-path]",
		Short:             "Remove .op build outputs",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeHolonSlugs,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			return runCommandCode(cmdLifecycle(format, currentRuntimeOptions(), holons.OperationClean, lifecycleArgsFromCommand(cmd, args)))
		},
	}
	addLifecycleExecutionFlags(cmd, false)
	return cmd
}

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "run <holon>",
		Short:             "Build a holon if needed, then launch it in the foreground",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeHolonSlugs,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			return runCommandCode(cmdRun(format, currentRuntimeOptions(), runArgsFromCommand(cmd, args[0])))
		},
	}
	cmd.Flags().String("listen", "", "listen address for service holons (default: tcp://127.0.0.1:0)")
	cmd.Flags().Bool("clean", false, "clean before building and running (cannot be combined with --no-build)")
	cmd.Flags().Bool("no-build", false, "fail if the artifact is missing instead of building")
	cmd.Flags().String("target", "", "pass the build target through if a build is needed")
	cmd.Flags().String("mode", "", "pass the build mode through if a build is needed")
	return cmd
}

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "install [holon-or-path]",
		Short:             "Install a holon artifact into $OPBIN",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeHolonSlugs,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			return runCommandCode(cmdInstall(format, currentRuntimeOptions(), installArgsFromCommand(cmd, args)))
		},
	}
	cmd.Flags().Bool("build", false, "build before installing instead of requiring a pre-built artifact")
	cmd.Flags().Bool("link-applications", false, "symlink installed .app bundles into /Applications (macOS only)")
	return cmd
}

func newUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "uninstall <holon>",
		Short:             "Remove an installed artifact from $OPBIN",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeHolonSlugs,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := currentFormat()
			if err != nil {
				return err
			}
			return runCommandCode(cmdUninstall(format, currentRuntimeOptions(), args))
		},
	}
	return cmd
}

func addLifecycleExecutionFlags(cmd *cobra.Command, includeBuildOnly bool) {
	cmd.Flags().String("target", "", "build target to resolve")
	cmd.Flags().String("mode", "", "build mode to resolve")
	cmd.Flags().Bool("dry-run", false, "print the resolved plan without executing it")
	addDiscoveryFlags(cmd)
	if includeBuildOnly {
		cmd.Flags().Bool("clean", false, "clean before building (cannot be combined with --dry-run)")
		cmd.Flags().Bool("no-sign", false, "skip automatic ad-hoc signing for bundle artifacts")
		return
	}
	cmd.Flags().Bool("no-sign", false, "skip automatic ad-hoc signing for bundle artifacts")
	_ = cmd.Flags().MarkHidden("no-sign")
}

func lifecycleArgsFromCommand(cmd *cobra.Command, positional []string) []string {
	args := make([]string, 0, 16)
	if target, _ := cmd.Flags().GetString("target"); target != "" {
		args = append(args, "--target", target)
	}
	if mode, _ := cmd.Flags().GetString("mode"); mode != "" {
		args = append(args, "--mode", mode)
	}
	if dryRun, _ := cmd.Flags().GetBool("dry-run"); dryRun {
		args = append(args, "--dry-run")
	}
	if clean, err := cmd.Flags().GetBool("clean"); err == nil && clean {
		args = append(args, "--clean")
	}
	if noSign, err := cmd.Flags().GetBool("no-sign"); err == nil && noSign {
		args = append(args, "--no-sign")
	}
	args = append(args, selectedDiscoveryArgs(cmd)...)
	args = append(args, positional...)
	return args
}

func runArgsFromCommand(cmd *cobra.Command, holon string) []string {
	args := make([]string, 0, 10)
	if listen, _ := cmd.Flags().GetString("listen"); listen != "" {
		args = append(args, "--listen", listen)
	}
	if clean, _ := cmd.Flags().GetBool("clean"); clean {
		args = append(args, "--clean")
	}
	if noBuild, _ := cmd.Flags().GetBool("no-build"); noBuild {
		args = append(args, "--no-build")
	}
	if target, _ := cmd.Flags().GetString("target"); target != "" {
		args = append(args, "--target", target)
	}
	if mode, _ := cmd.Flags().GetString("mode"); mode != "" {
		args = append(args, "--mode", mode)
	}
	args = append(args, holon)
	return args
}

func installArgsFromCommand(cmd *cobra.Command, positional []string) []string {
	args := make([]string, 0, 4)
	if build, _ := cmd.Flags().GetBool("build"); build {
		args = append(args, "--build")
	}
	if linkApplications, _ := cmd.Flags().GetBool("link-applications"); linkApplications {
		args = append(args, "--link-applications")
	}
	args = append(args, positional...)
	return args
}

func boolFlagArg(cmd *cobra.Command, name string) string {
	value, err := cmd.Flags().GetBool(name)
	if err != nil || !value {
		return ""
	}
	return "--" + name
}

func stringFlagArg(cmd *cobra.Command, name string) []string {
	value, err := cmd.Flags().GetString(name)
	if err != nil || value == "" {
		return nil
	}
	return []string{"--" + name, value}
}

func commandArgsWithFlag(cmd *cobra.Command, args []string, name string) []string {
	if value := boolFlagArg(cmd, name); value != "" {
		return append([]string{value}, args...)
	}
	return args
}

func requireCurrentFormat() (Format, error) {
	format, err := currentFormat()
	if err != nil {
		return "", fmt.Errorf("invalid global format: %w", err)
	}
	return format, nil
}
