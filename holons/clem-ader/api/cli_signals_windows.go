//go:build windows

package api

import "os"

func cliTerminationSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
