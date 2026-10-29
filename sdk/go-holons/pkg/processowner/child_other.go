//go:build !windows

package processowner

import (
	"os/exec"
	"syscall"
)

// Start preserves the command's existing Unix process configuration.
func Start(cmd *exec.Cmd) error { return cmd.Start() }

// Wait preserves the command's existing Unix wait behavior.
func Wait(cmd *exec.Cmd) error { return cmd.Wait() }

// Kill preserves direct-process termination on Unix.
func Kill(cmd *exec.Cmd) error { return cmd.Process.Kill() }

// Terminate preserves SIGTERM on Unix.
func Terminate(cmd *exec.Cmd) error { return cmd.Process.Signal(syscall.SIGTERM) }
