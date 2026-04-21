//go:build !windows

package engine

import (
	"errors"
	"os/exec"
	"syscall"
)

func prepareStepCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func startedStepGroupID(cmd *exec.Cmd) int {
	if cmd == nil || cmd.Process == nil {
		return 0
	}
	return cmd.Process.Pid
}

func terminateStepProcessGroup(handle *stepProcessHandle) error {
	return signalStepProcessGroup(handle, syscall.SIGTERM)
}

func killStepProcessGroup(handle *stepProcessHandle) error {
	return signalStepProcessGroup(handle, syscall.SIGKILL)
}

func stepProcessGroupRunning(handle *stepProcessHandle) bool {
	if handle == nil || handle.groupID <= 0 {
		return false
	}
	err := syscall.Kill(-handle.groupID, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func signalStepProcessGroup(handle *stepProcessHandle, sig syscall.Signal) error {
	if handle == nil || handle.groupID <= 0 {
		return nil
	}
	if err := syscall.Kill(-handle.groupID, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}
