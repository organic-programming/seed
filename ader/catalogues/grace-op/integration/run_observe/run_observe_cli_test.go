//go:build e2e

package run_observe_test

import (
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestRunObserve_CLI_TailsLifecycleEvents(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGTERM lifecycle check is Unix-only")
	}
	integration.SkipIfShort(t, integration.ShortTestReason)

	sb := integration.NewSandbox(t)
	process := sb.StartProcess(t, integration.RunOptions{}, "run", "gabriel-greeting-go", "--observe", "--listen", "tcp://127.0.0.1:0")
	defer process.Stop(t)

	process.WaitForStdoutContains(t, "INSTANCE_READY", 2*time.Minute)

	process.Signal(t, syscall.SIGTERM)
	if err := process.Wait(30 * time.Second); err != nil {
		t.Fatalf("op run --observe did not exit cleanly after SIGTERM: %v\nstdout:\n%s\nstderr:\n%s", err, process.Stdout(), process.Stderr())
	}
	if !strings.Contains(process.Stdout(), "INSTANCE_EXITED") {
		t.Fatalf("op run --observe stdout missing INSTANCE_EXITED after SIGTERM\nstdout:\n%s\nstderr:\n%s", process.Stdout(), process.Stderr())
	}
}
