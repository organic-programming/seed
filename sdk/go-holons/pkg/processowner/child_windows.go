//go:build windows

package processowner

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var childJobsMu sync.Mutex
var childJobs = make(map[*exec.Cmd]windows.Handle) // released by Wait

// Start creates a child inside a dedicated job before its initial thread runs.
// The host already owns a job, so a crash before child-job assignment also
// terminates the suspended child through inherited host-job membership.
func Start(cmd *exec.Cmd) error {
	if err := Ensure(); err != nil {
		return err
	}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return fmt.Errorf("create child job: %w", err)
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		_ = windows.CloseHandle(job)
		return fmt.Errorf("configure child job: %w", err)
	}
	attr := syscall.SysProcAttr{}
	if cmd.SysProcAttr != nil {
		attr = *cmd.SysProcAttr
	}
	if attr.CreationFlags&windows.CREATE_BREAKAWAY_FROM_JOB != 0 {
		_ = windows.CloseHandle(job)
		return errors.New("owned children cannot break away from their job")
	}
	attr.CreationFlags |= windows.CREATE_SUSPENDED | windows.CREATE_NEW_PROCESS_GROUP
	cmd.SysProcAttr = &attr
	if cmd.WaitDelay == 0 {
		cmd.WaitDelay = 2 * time.Second
	}
	if cmd.Cancel != nil {
		cmd.Cancel = func() error { return Kill(cmd) }
	}
	if err := cmd.Start(); err != nil {
		_ = windows.CloseHandle(job)
		return err
	}
	fail := func(err error) error {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = windows.CloseHandle(job)
		return err
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, uint32(cmd.Process.Pid))
	if err != nil {
		return fail(fmt.Errorf("open child for job assignment: %w", err))
	}
	err = windows.AssignProcessToJobObject(job, process)
	if err != nil {
		_ = windows.CloseHandle(process)
		return fail(fmt.Errorf("assign child job: %w", err))
	}
	childJobsMu.Lock()
	childJobs[cmd] = job
	childJobsMu.Unlock()
	if err := resumeInitialThread(uint32(cmd.Process.Pid)); err != nil {
		childJobsMu.Lock()
		delete(childJobs, cmd)
		childJobsMu.Unlock()
		_ = windows.CloseHandle(process)
		return fail(err)
	}
	// Keep ownership independent of callers using exec.Cmd.Wait directly.
	// Closing the job here also releases output pipes inherited by surviving
	// grandchildren, which otherwise can keep Cmd.Wait blocked indefinitely.
	go func() {
		_, _ = windows.WaitForSingleObject(process, windows.INFINITE)
		_ = releaseChildJob(cmd)
		_ = windows.CloseHandle(process)
	}()
	return nil
}

func resumeInitialThread(pid uint32) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return fmt.Errorf("snapshot child thread: %w", err)
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}
		thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if err != nil {
			return fmt.Errorf("open suspended child thread: %w", err)
		}
		_, err = windows.ResumeThread(thread)
		_ = windows.CloseHandle(thread)
		if err != nil {
			return fmt.Errorf("resume child thread: %w", err)
		}
		return nil
	}
	return fmt.Errorf("initial thread for suspended process %d not found: %w", pid, err)
}

// Wait reaps the child and closes its private job, including any descendants
// that outlived the direct child. Cmd.Wait remains supported by the exit watcher.
func Wait(cmd *exec.Cmd) error {
	err := cmd.Wait()
	return errors.Join(err, releaseChildJob(cmd))
}

func releaseChildJob(cmd *exec.Cmd) error {
	childJobsMu.Lock()
	defer childJobsMu.Unlock()
	if job, ok := childJobs[cmd]; ok {
		delete(childJobs, cmd)
		return windows.CloseHandle(job)
	}
	return nil
}

// Terminate requests a cooperative console break for the child's process group.
// GUI or detached children may reject it; callers must apply a bounded grace
// period and then call Kill. No signal is broadcast to the owner's console.
func Terminate(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	childJobsMu.Lock()
	defer childJobsMu.Unlock()
	if _, ok := childJobs[cmd]; !ok {
		return os.ErrProcessDone
	}
	return windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(cmd.Process.Pid))
}

// Kill terminates the child's entire job, including grandchildren.
func Kill(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	childJobsMu.Lock()
	defer childJobsMu.Unlock()
	if job, ok := childJobs[cmd]; ok {
		return windows.TerminateJobObject(job, 1)
	}
	if err := cmd.Process.Kill(); !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}

// Running reports active members, even after the direct child has exited.
func Running(cmd *exec.Cmd) bool {
	childJobsMu.Lock()
	defer childJobsMu.Unlock()
	job, ok := childJobs[cmd]
	if !ok {
		return false
	}
	var accounting struct {
		User, Kernel, PeriodUser, PeriodKernel int64
		Faults, Total, Active, Terminated      uint32
	}
	err := windows.QueryInformationJobObject(job, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&accounting)), uint32(unsafe.Sizeof(accounting)), nil)
	return err != nil || accounting.Active != 0
}
