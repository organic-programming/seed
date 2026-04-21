//go:build windows

package engine

import (
	"errors"
	"os"
	"os/exec"
)

func prepareStepCommand(cmd *exec.Cmd) {}

func startedStepGroupID(cmd *exec.Cmd) int {
	return 0
}

func terminateStepProcessGroup(handle *stepProcessHandle) error {
	return stopStepProcess(handle)
}

func killStepProcessGroup(handle *stepProcessHandle) error {
	return stopStepProcess(handle)
}

func stepProcessGroupRunning(handle *stepProcessHandle) bool {
	if handle == nil || handle.cmd == nil {
		return false
	}
	if handle.cmd.ProcessState != nil {
		return !handle.cmd.ProcessState.Exited()
	}
	return handle.cmd.Process != nil
}

func stopStepProcess(handle *stepProcessHandle) error {
	if handle == nil || handle.cmd == nil || handle.cmd.Process == nil {
		return nil
	}
	if err := handle.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}
