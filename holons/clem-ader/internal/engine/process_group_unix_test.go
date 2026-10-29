//go:build !windows

package engine

import "testing"

func TestRunStepCommandTerminatesBackgroundProcesses(t *testing.T) {
	testUnixRunStepCommandTerminatesBackgroundProcesses(t)
}
