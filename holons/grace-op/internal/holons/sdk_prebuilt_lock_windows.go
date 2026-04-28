//go:build windows

package holons

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func lockFileExclusive(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	var overlapped windows.Overlapped
	if err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &overlapped); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	return f, nil
}

func unlockFileExclusive(f *os.File) error {
	if f == nil {
		return nil
	}
	var overlapped windows.Overlapped
	lockErr := windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &overlapped)
	closeErr := f.Close()
	if lockErr != nil {
		return lockErr
	}
	return closeErr
}
