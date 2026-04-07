package engine

import (
	"errors"
	"os/exec"
	"time"
)

const (
	stepShutdownGracePeriod  = 2 * time.Second
	stepShutdownPollInterval = 25 * time.Millisecond
)

type stepProcessHandle struct {
	cmd     *exec.Cmd
	groupID int
}

func startStepProcess(cmd *exec.Cmd) (*stepProcessHandle, error) {
	prepareStepCommand(cmd)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &stepProcessHandle{
		cmd:     cmd,
		groupID: startedStepGroupID(cmd),
	}, nil
}

func (handle *stepProcessHandle) terminateAndWait(waitCh <-chan error) error {
	if handle == nil {
		return nil
	}
	_ = terminateStepProcessGroup(handle)
	timer := time.NewTimer(stepShutdownGracePeriod)
	defer timer.Stop()
	select {
	case err := <-waitCh:
		return err
	case <-timer.C:
		_ = killStepProcessGroup(handle)
		return <-waitCh
	}
}

func (handle *stepProcessHandle) cleanup() error {
	if handle == nil {
		return nil
	}
	_ = terminateStepProcessGroup(handle)
	if handle.waitForExit(stepShutdownGracePeriod) {
		return nil
	}
	_ = killStepProcessGroup(handle)
	if handle.waitForExit(stepShutdownGracePeriod) {
		return nil
	}
	return errors.New("step process group did not exit after termination")
}

func (handle *stepProcessHandle) waitForExit(timeout time.Duration) bool {
	if handle == nil {
		return true
	}
	deadline := time.Now().Add(timeout)
	for {
		if !stepProcessGroupRunning(handle) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(stepShutdownPollInterval)
	}
}
