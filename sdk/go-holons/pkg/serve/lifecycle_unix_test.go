//go:build !windows

package serve_test

import (
	"os"
	"syscall"
	"testing"
)

func runIsolatedTransportLifecycle(*testing.T) bool { return false }

func signalTransportLifecycle() error {
	self, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}
	return self.Signal(syscall.SIGTERM)
}
