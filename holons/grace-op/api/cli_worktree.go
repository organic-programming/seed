package api

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	wt "github.com/organic-programming/grace-op/internal/worktree"
)

func (c cliState) runWorktreeCommand(format Format, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(c.stderr, "op worktree: requires create, bootstrap, launch, shell, or doctor")
		return 1
	}
	switch args[0] {
	case "create":
		return c.runWorktreeCreateCommand(format, args[1:])
	case "bootstrap":
		return c.runWorktreeBootstrapCommand(format, args[1:])
	case "launch":
		return c.runWorktreeLaunchCommand(args[1:])
	case "shell":
		return c.runWorktreeShellCommand(args[1:])
	case "doctor":
		return c.runWorktreeDoctorCommand(format, args[1:])
	default:
		fmt.Fprintf(c.stderr, "op worktree: unknown command %q\n", args[0])
		return 1
	}
}

func (c cliState) runWorktreeCreateCommand(format Format, args []string) int {
	currentFormat := format
	var branch string
	var mode opv1.WorktreeMode
	for _, arg := range args {
		switch arg {
		case "--isolated":
			if mode == opv1.WorktreeMode_WORKTREE_MODE_PLAIN {
				fmt.Fprintln(c.stderr, "op worktree create: choose only one of --isolated or --plain")
				return 1
			}
			mode = opv1.WorktreeMode_WORKTREE_MODE_ISOLATED
		case "--plain":
			if mode == opv1.WorktreeMode_WORKTREE_MODE_ISOLATED {
				fmt.Fprintln(c.stderr, "op worktree create: choose only one of --isolated or --plain")
				return 1
			}
			mode = opv1.WorktreeMode_WORKTREE_MODE_PLAIN
		case "--json":
			currentFormat = FormatJSON
		default:
			if strings.HasPrefix(arg, "--") {
				fmt.Fprintf(c.stderr, "op worktree create: unknown flag %q\n", arg)
				return 1
			}
			if branch != "" {
				fmt.Fprintln(c.stderr, "op worktree create: requires exactly one <branch>")
				return 1
			}
			branch = arg
		}
	}
	if branch == "" {
		fmt.Fprintln(c.stderr, "op worktree create: requires <branch>")
		return 1
	}
	if mode == opv1.WorktreeMode_WORKTREE_MODE_UNSPECIFIED {
		fmt.Fprintln(c.stderr, "op worktree create: non-interactive create requires --isolated or --plain")
		return 1
	}
	resp, err := Worktree(&opv1.WorktreeRequest{Command: string(wt.CommandCreate), Branch: branch, Mode: mode})
	return c.writeWorktreeResponse(currentFormat, resp, err, "op worktree create")
}

func (c cliState) runWorktreeBootstrapCommand(format Format, args []string) int {
	currentFormat := format
	var branch string
	for _, arg := range args {
		switch arg {
		case "--json":
			currentFormat = FormatJSON
		default:
			if strings.HasPrefix(arg, "--") {
				fmt.Fprintf(c.stderr, "op worktree bootstrap: unknown flag %q\n", arg)
				return 1
			}
			if branch != "" {
				fmt.Fprintln(c.stderr, "op worktree bootstrap: requires exactly one <branch>")
				return 1
			}
			branch = arg
		}
	}
	if branch == "" {
		fmt.Fprintln(c.stderr, "op worktree bootstrap: requires <branch>")
		return 1
	}
	resp, err := Worktree(&opv1.WorktreeRequest{Command: string(wt.CommandBootstrap), Branch: branch})
	return c.writeWorktreeResponse(currentFormat, resp, err, "op worktree bootstrap")
}

func (c cliState) runWorktreeDoctorCommand(format Format, args []string) int {
	currentFormat := format
	for _, arg := range args {
		switch arg {
		case "--json":
			currentFormat = FormatJSON
		default:
			fmt.Fprintf(c.stderr, "op worktree doctor: unknown argument %q\n", arg)
			return 1
		}
	}
	resp, err := Worktree(&opv1.WorktreeRequest{Command: string(wt.CommandDoctor)})
	if err != nil {
		fmt.Fprintf(c.stderr, "op worktree doctor: %v\n", err)
		return 1
	}
	c.writeFormatted(currentFormat, resp)
	if resp.GetDoctor() != nil && !resp.GetDoctor().GetOk() {
		if code := resp.GetDoctor().GetRecommendedCode(); code != 0 {
			return int(code)
		}
		return 1
	}
	return 0
}

func (c cliState) runWorktreeLaunchCommand(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(c.stderr, "op worktree launch: requires <branch> -- <command...>")
		return 1
	}
	branch := args[0]
	commandArgs := args[1:]
	if len(commandArgs) > 0 && commandArgs[0] == "--" {
		commandArgs = commandArgs[1:]
	}
	if len(commandArgs) == 0 {
		fmt.Fprintln(c.stderr, "op worktree launch: requires -- <command...>")
		return 1
	}
	result, err := wt.Bootstrap(branch)
	if err != nil {
		fmt.Fprintf(c.stderr, "op worktree launch: %v\n", err)
		return 1
	}
	return c.runActivatedWorktreeCommand(result, commandArgs)
}

func (c cliState) runWorktreeShellCommand(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(c.stderr, "op worktree shell: requires exactly one <branch>")
		return 1
	}
	result, err := wt.Bootstrap(args[0])
	if err != nil {
		fmt.Fprintf(c.stderr, "op worktree shell: %v\n", err)
		return 1
	}
	shell := os.Getenv("SHELL")
	if strings.TrimSpace(shell) == "" {
		shell = "/bin/sh"
	}
	return c.runActivatedWorktreeCommand(result, []string{shell})
}

func (c cliState) runActivatedWorktreeCommand(result *wt.Result, commandArgs []string) int {
	cwd, envMap, err := wt.ActivationFromResult(result)
	if err != nil {
		fmt.Fprintf(c.stderr, "op worktree: %v\n", err)
		return 1
	}
	path, err := wt.LookPathInEnv(commandArgs[0], envMap)
	if err != nil {
		fmt.Fprintf(c.stderr, "op worktree: resolve %q in isolated PATH: %v\n", commandArgs[0], err)
		return 1
	}
	child := exec.Command(path, commandArgs[1:]...)
	child.Dir = cwd
	child.Env = wt.MergeEnv(os.Environ(), envMap)
	child.Stdin = os.Stdin
	child.Stdout = c.stdout
	child.Stderr = c.stderr
	if err := child.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(c.stderr, "op worktree: %v\n", err)
		return 1
	}
	return 0
}

func (c cliState) writeWorktreeResponse(format Format, resp *opv1.WorktreeResponse, err error, prefix string) int {
	if err != nil {
		fmt.Fprintf(c.stderr, "%s: %v\n", prefix, err)
		return 1
	}
	c.writeFormatted(format, resp)
	return 0
}
