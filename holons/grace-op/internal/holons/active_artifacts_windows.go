//go:build windows

package holons

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const supportsActiveArtifacts = true

func describeActiveArtifactProcess(p ActiveArtifactProcess) ActiveArtifactProcess { return p }

func canonicalArtifactPath(path string) (string, error) { return canonicalWindowsArtifactPath(path) }
func guardActiveWindowsArtifacts(m *LoadedManifest, ctx BuildContext, op Operation) error {
	return CheckActiveArtifacts(m, ctx, op)
}

// Resolve existing ancestors as well: a clean checkout may not yet have an
// artifact directory, while its checkout path can already contain a junction.
func canonicalWindowsArtifactPath(path string) (string, error) {
	path = strings.ReplaceAll(path, "/", `\`)
	if strings.HasPrefix(path, `\\?\UNC\`) {
		path = `\\` + strings.TrimPrefix(path, `\\?\UNC\`)
	} else {
		path = strings.TrimPrefix(path, `\\?\`)
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var suffix []string
	for {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return strings.ToLower(filepath.Clean(resolved)), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", err
		}
		suffix = append(suffix, filepath.Base(path))
		path = parent
	}
}

func windowsArtifactContains(root activeArtifactRoot, path string) bool {
	if strings.EqualFold(root.path, path) {
		return true
	}
	return root.directory && strings.HasPrefix(strings.ToLower(path), strings.TrimRight(strings.ToLower(root.path), `\`)+`\`)
}

func activeArtifactProcesses() ([]ActiveArtifactProcess, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	err = windows.Process32First(snapshot, &entry)
	var processes []ActiveArtifactProcess
	for err == nil {
		// Protected/system processes cannot be queried by an ordinary user;
		// processes that exit after the snapshot also cannot be opened.
		process, openErr := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, entry.ProcessID)
		if openErr == nil {
			buffer := make([]uint16, 32768)
			size := uint32(len(buffer))
			queryErr := windows.QueryFullProcessImageName(process, 0, &buffer[0], &size)
			var created, exited, kernel, user windows.Filetime
			timeErr := windows.GetProcessTimes(process, &created, &exited, &kernel, &user)
			windows.CloseHandle(process)
			if queryErr == nil && timeErr == nil {
				processes = append(processes, ActiveArtifactProcess{PID: int(entry.ProcessID), Path: windows.UTF16ToString(buffer[:size]), Started: windowsProcessStart(created)})
			}
		}
		err = windows.Process32Next(snapshot, &entry)
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil, err
	}
	return processes, nil
}

func windowsProcessStart(created windows.Filetime) string {
	return fmt.Sprintf("%d:%d", created.HighDateTime, created.LowDateTime)
}

func stopActiveArtifactProcess(ctx context.Context, expected ActiveArtifactProcess, force bool) error {
	access := uint32(windows.PROCESS_QUERY_LIMITED_INFORMATION)
	if force {
		access |= windows.PROCESS_TERMINATE
	}
	process, err := windows.OpenProcess(access, false, uint32(expected.PID))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return nil // Already gone.
	}
	if err != nil {
		return err
	}
	defer windows.CloseHandle(process)
	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(process, 0, &buffer[0], &size); err != nil {
		return err
	}
	path, err := canonicalArtifactPath(windows.UTF16ToString(buffer[:size]))
	if err != nil {
		return err
	}
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(process, &created, &exited, &kernel, &user); err != nil {
		return err
	}
	if path != expected.Path || windowsProcessStart(created) != expected.Started {
		return ErrActiveArtifactProcessChanged
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if force {
		// The open handle pins the verified process, even if its PID is reused.
		return windows.TerminateProcess(process, 1)
	}
	// WM_CLOSE asks only windows belonging to the verified PID to close. Console
	// workers have no equivalent of SIGTERM; they remain for the separate force
	// decision. Never send a console event to an unverified process group.
	user32 := windows.NewLazySystemDLL("user32.dll")
	enumWindows := user32.NewProc("EnumWindows")
	getPID := user32.NewProc("GetWindowThreadProcessId")
	postMessage := user32.NewProc("PostMessageW")
	var requestErr error
	callback := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		if ctx.Err() != nil {
			requestErr = ctx.Err()
			return 0
		}
		var pid uint32
		getPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid == uint32(expected.PID) {
			var code uint32
			if err := windows.GetExitCodeProcess(process, &code); err != nil || code != 259 { // STILL_ACTIVE
				return 0
			}
			if ok, _, err := postMessage.Call(hwnd, 0x0010, 0, 0); ok == 0 { // WM_CLOSE
				requestErr = err
				return 0
			}
		}
		return 1
	})
	ok, _, callErr := enumWindows.Call(callback, 0)
	if requestErr != nil {
		return requestErr
	}
	if ok == 0 && callErr != syscall.Errno(0) {
		return callErr
	}
	return nil
}
