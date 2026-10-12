//go:build windows

package engine

import (
	"os/exec"

	"github.com/organic-programming/go-holons/pkg/processowner"
)

func prepareStepCommand(cmd *exec.Cmd) {}

func startedStepGroupID(cmd *exec.Cmd) int {
	return 0
}

func terminateStepProcessGroup(handle *stepProcessHandle) error {
	if handle == nil {
		return nil
	}
	return processowner.Terminate(handle.cmd)
}

func killStepProcessGroup(handle *stepProcessHandle) error {
	if handle == nil {
		return nil
	}
	return processowner.Kill(handle.cmd)
}

func stepProcessGroupRunning(handle *stepProcessHandle) bool {
	if handle == nil || handle.cmd == nil {
		return false
	}
	return processowner.Running(handle.cmd)
}
