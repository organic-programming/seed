//go:build !windows

package engine

import (
	"fmt"
	"os"
	"os/exec"
)

func buildScriptCommand(step StepSpec, info os.FileInfo) (*exec.Cmd, error) {
	if info.Mode()&0o111 == 0 {
		return nil, fmt.Errorf("script %s is not executable", step.Script)
	}
	return exec.Command(step.Script, step.Args...), nil
}
