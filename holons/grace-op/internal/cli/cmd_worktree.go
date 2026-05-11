package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/organic-programming/grace-op/internal/worktree"
	"github.com/spf13/cobra"
)

func newWorktreeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worktree",
		Short: "Create git worktrees with isolated OP runtimes",
	}
	cmd.AddCommand(
		newWorktreeCreateCmd(),
		newWorktreeBootstrapCmd(),
		newWorktreeLaunchCmd(),
		newWorktreeShellCmd(),
		newWorktreeDoctorCmd(),
	)
	return cmd
}

func newWorktreeCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <branch>",
		Short: "Create a plain or isolated OP worktree",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			isolated, _ := cmd.Flags().GetBool("isolated")
			plain, _ := cmd.Flags().GetBool("plain")
			asJSON, _ := cmd.Flags().GetBool("json")
			mode, err := resolveWorktreeMode(cmd, isolated, plain)
			if err != nil {
				return err
			}
			result, err := worktree.Create(args[0], mode)
			if err != nil {
				return err
			}
			return writeWorktreeResult(cmd.OutOrStdout(), result, asJSON)
		},
	}
	cmd.Flags().Bool("isolated", false, "create/reuse the worktree and bootstrap a local .op runtime")
	cmd.Flags().Bool("plain", false, "create/reuse only the git worktree")
	cmd.Flags().Bool("json", false, "print machine-readable JSON")
	return cmd
}

func newWorktreeBootstrapCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bootstrap <branch>",
		Short: "Bootstrap an isolated OP runtime in a worktree",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			result, err := worktree.Bootstrap(args[0])
			if err != nil {
				return err
			}
			return writeWorktreeResult(cmd.OutOrStdout(), result, asJSON)
		},
	}
	cmd.Flags().Bool("json", false, "print machine-readable JSON")
	return cmd
}

func newWorktreeLaunchCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "launch <branch> -- <command...>",
		Short:              "Launch a command inside an isolated OP worktree",
		DisableFlagParsing: true,
		Args:               cobra.MinimumNArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			branch := args[0]
			commandArgs := args[1:]
			if len(commandArgs) > 0 && commandArgs[0] == "--" {
				commandArgs = commandArgs[1:]
			}
			if len(commandArgs) == 0 {
				return errors.New("launch requires -- <command...>")
			}
			result, err := worktree.Bootstrap(branch)
			if err != nil {
				return err
			}
			cwd, envMap, err := worktree.ActivationFromResult(result)
			if err != nil {
				return err
			}
			path, err := worktree.LookPathInEnv(commandArgs[0], envMap)
			if err != nil {
				return fmt.Errorf("resolve %q in isolated PATH: %w", commandArgs[0], err)
			}
			child := exec.Command(path, commandArgs[1:]...)
			child.Dir = cwd
			child.Env = worktree.MergeEnv(os.Environ(), envMap)
			child.Stdin = os.Stdin
			child.Stdout = os.Stdout
			child.Stderr = os.Stderr
			if err := child.Run(); err != nil {
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) {
					return commandExitError{code: exitErr.ExitCode()}
				}
				return err
			}
			return nil
		},
	}
}

func newWorktreeShellCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shell <branch>",
		Short: "Start a shell inside an isolated OP worktree",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			result, err := worktree.Bootstrap(args[0])
			if err != nil {
				return err
			}
			cwd, envMap, err := worktree.ActivationFromResult(result)
			if err != nil {
				return err
			}
			shell := os.Getenv("SHELL")
			if strings.TrimSpace(shell) == "" {
				shell = "/bin/sh"
			}
			child := exec.Command(shell)
			child.Dir = cwd
			child.Env = worktree.MergeEnv(os.Environ(), envMap)
			child.Stdin = os.Stdin
			child.Stdout = os.Stdout
			child.Stderr = os.Stderr
			if err := child.Run(); err != nil {
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) {
					return commandExitError{code: exitErr.ExitCode()}
				}
				return err
			}
			return nil
		},
	}
}

func newWorktreeDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Verify the current shell is using a worktree-local OP runtime",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			code, result := worktree.Doctor()
			if err := writeWorktreeResult(cmd.OutOrStdout(), result, asJSON); err != nil {
				return err
			}
			return runCommandCode(code)
		},
	}
	cmd.Flags().Bool("json", false, "print machine-readable JSON")
	return cmd
}

func resolveWorktreeMode(cmd *cobra.Command, isolated, plain bool) (worktree.Mode, error) {
	if isolated && plain {
		return "", errors.New("choose only one of --isolated or --plain")
	}
	if isolated {
		return worktree.ModeIsolated, nil
	}
	if plain {
		return worktree.ModePlain, nil
	}
	if !isTerminal(os.Stdin) {
		return "", errors.New("non-interactive create requires --isolated or --plain")
	}
	for {
		fmt.Fprint(cmd.OutOrStdout(), "Create isolated worktree? [isolated/plain/cancel] ")
		var answer string
		if _, err := fmt.Fscan(os.Stdin, &answer); err != nil {
			return "", err
		}
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "isolated", "i":
			return worktree.ModeIsolated, nil
		case "plain", "p":
			return worktree.ModePlain, nil
		case "cancel", "c", "q", "quit":
			return "", errors.New("cancelled")
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Please answer isolated, plain, or cancel.")
	}
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func writeWorktreeResult(w io.Writer, result *worktree.Result, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	if result.Command == string(worktree.CommandDoctor) {
		if result.Doctor != nil && result.Doctor.OK {
			fmt.Fprintln(w, "op worktree doctor: ok")
			return nil
		}
		fmt.Fprintln(w, "op worktree doctor: failed")
		return nil
	}
	fmt.Fprintf(w, "op worktree %s: %s (%s)\n", result.Command, result.Status, result.Mode)
	fmt.Fprintf(w, "branch:   %s\nworktree: %s\n", result.Branch, result.Worktree)
	if result.Mode == string(worktree.ModeIsolated) {
		fmt.Fprintf(w, "OPPATH:   %s\nOPBIN:    %s\n", result.Oppath, result.Opbin)
	}
	return nil
}
