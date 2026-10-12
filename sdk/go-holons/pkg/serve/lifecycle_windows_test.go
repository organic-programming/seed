package serve_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/organic-programming/go-holons/pkg/serve"
	"golang.org/x/sys/windows"
)

// A console control event is the real Windows graceful-shutdown signal. Keep
// this test in a hidden, private console so the event cannot reach the caller,
// another test process, or the user's terminal.
func runIsolatedTransportLifecycle(t *testing.T) bool {
	t.Helper()
	if os.Getenv("GO_SERVE_LIFECYCLE_CHILD") == "1" {
		return false
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestCurrentTransportTracksRunLifecycle$", "-test.v")
	cmd.Env = append(os.Environ(), "GO_SERVE_LIFECYCLE_CHILD=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE, HideWindow: true}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("isolated Windows lifecycle: %v\n%s", err, output)
	}
	t.Logf("isolated Windows lifecycle:\n%s", output)
	return true
}

func signalTransportLifecycle() error {
	// The listener starts before Run registers its signal channel. Keep this
	// private-console subscriber until shutdown and repeat the real event while
	// initialization finishes, instead of racing the default console exit action.
	delivered := make(chan os.Signal, 1)
	signal.Notify(delivered, os.Interrupt)
	defer signal.Stop(delivered)
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		if err := windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, 0); err != nil {
			return err
		}
		select {
		case <-delivered:
		case <-deadline.C:
			return fmt.Errorf("private console did not deliver CTRL_BREAK_EVENT")
		}
		if serve.CurrentTransport() == "" {
			return nil
		}
		select {
		case <-time.After(25 * time.Millisecond):
		case <-deadline.C:
			return fmt.Errorf("server did not shut down after CTRL_BREAK_EVENT")
		}
	}
}
