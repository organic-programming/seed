//go:build !windows

package holons

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func lockFileExclusive(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	return f, nil
}

func unlockFileExclusive(f *os.File) error {
	if f == nil {
		return nil
	}
	lockErr := unix.Flock(int(f.Fd()), unix.LOCK_UN)
	closeErr := f.Close()
	if lockErr != nil {
		return lockErr
	}
	return closeErr
}
