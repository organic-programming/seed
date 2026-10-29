//go:build windows

package engine

import "golang.org/x/sys/windows"

func processRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		// A protected process can be alive without granting query access.
		return err == windows.ERROR_ACCESS_DENIED
	}
	defer windows.CloseHandle(process)
	state, err := windows.WaitForSingleObject(process, 0)
	return err != nil || state == uint32(windows.WAIT_TIMEOUT)
}
