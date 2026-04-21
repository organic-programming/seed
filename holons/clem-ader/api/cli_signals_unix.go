//go:build !windows

package api

import (
	"os"
	"syscall"
)

func cliTerminationSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM}
}
