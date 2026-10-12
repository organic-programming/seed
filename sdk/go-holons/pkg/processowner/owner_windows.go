//go:build windows

// Package processowner owns the lifetime of descendants created by a holon.
package processowner

import (
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var ownerOnce sync.Once
var ownerErr error
var ownerJob windows.Handle

// Ensure places the current process in a non-inheritable, kill-on-close job
// before it creates children. Descendants inherit membership, not the handle;
// Windows closes the owner's only handle even after a crash or forced exit.
// The handle deliberately lives until process exit: closing it would also kill
// the owner. Nested jobs support callers already managed by a Windows runner.
func Ensure() error {
	ownerOnce.Do(func() {
		job, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			ownerErr = fmt.Errorf("create descendant job: %w", err)
			return
		}
		limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
		limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
		if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
			_ = windows.CloseHandle(job)
			ownerErr = fmt.Errorf("configure descendant job: %w", err)
			return
		}
		if err := windows.AssignProcessToJobObject(job, windows.CurrentProcess()); err != nil {
			_ = windows.CloseHandle(job)
			ownerErr = fmt.Errorf("own descendant process tree: %w", err)
			return
		}
		ownerJob = job
	})
	return ownerErr
}
