package holons

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// EvalSymlinks alone preserves the caller's spelling on case-insensitive APFS.
// F_GETPATH uses the filesystem's actual spelling, just like proc_pidpath.
func canonicalExistingArtifactPath(path string) (string, error) {
	fd, err := unix.Open(path, unix.O_EVTONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return "", err
	}
	defer unix.Close(fd)
	var buffer [1024]byte // Darwin MAXPATHLEN, required by F_GETPATH.
	_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), unix.F_GETPATH, uintptr(unsafe.Pointer(&buffer[0])))
	if errno != 0 {
		return "", errno
	}
	return unix.ByteSliceToString(buffer[:]), nil
}
