//go:build darwin

package holons

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const supportsActiveArtifacts = true

func activeArtifactProcesses() ([]ActiveArtifactProcess, error) {
	entries, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil, err
	}
	var processes []ActiveArtifactProcess
	for _, entry := range entries {
		if entry.Proc.P_pid <= 0 || entry.Proc.P_stat == 5 { // SZOMB
			continue
		}
		process, err := darwinArtifactProcessIdentity(int(entry.Proc.P_pid))
		if err != nil {
			// Processes may exit during the snapshot. The kernel also refuses
			// information about certain protected/system processes.
			if errors.Is(err, unix.ESRCH) || errors.Is(err, unix.ENOENT) ||
				errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) ||
				errors.Is(err, ErrActiveArtifactProcessChanged) {
				continue
			}
			return nil, fmt.Errorf("inspect process %d: %w", entry.Proc.P_pid, err)
		}
		processes = append(processes, process)
	}
	return processes, nil
}

// The kernel's executable vnode path is independent of argv[0], the launcher's
// PID, and the current working directory. This is the proc_pidpath operation
// from libproc, called directly so release builds keep working with CGO_ENABLED=0.
// Constants and ABI: XNU sys/proc_info.h and libsyscall/wrappers/libproc/libproc.c.
func darwinExecutablePath(pid int) (string, error) {
	const (
		procInfoCallPIDInfo = 2
		procPIDPathInfo     = 11
		procPIDPathMaxSize  = 4096
	)
	buffer := make([]byte, procPIDPathMaxSize)
	_, _, errno := syscall.Syscall6(syscall.SYS_PROC_INFO,
		procInfoCallPIDInfo, uintptr(pid), procPIDPathInfo, 0,
		uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if errno != 0 {
		return "", errno
	}
	end := bytes.IndexByte(buffer, 0)
	if end <= 0 {
		return "", fmt.Errorf("kernel returned an empty or unterminated executable path")
	}
	return canonicalArtifactPath(string(buffer[:end]))
}

func darwinProcessStart(info *unix.KinfoProc) string {
	return fmt.Sprintf("%d:%d", info.Proc.P_starttime.Sec, info.Proc.P_starttime.Usec)
}

func darwinArtifactProcess(pid int) (ActiveArtifactProcess, error) {
	process, err := darwinArtifactProcessIdentity(pid)
	if err != nil {
		return process, err
	}
	return describeActiveArtifactProcess(process), nil
}

func describeActiveArtifactProcess(process ActiveArtifactProcess) ActiveArtifactProcess {
	process.BundlePath = darwinExecutableBundle(process.Path)
	return process
}

func darwinArtifactProcessIdentity(pid int) (ActiveArtifactProcess, error) {
	before, err := darwinKinfoProc(pid)
	if err != nil {
		return ActiveArtifactProcess{}, err
	}
	if int(before.Proc.P_pid) != pid || before.Proc.P_stat == 5 {
		return ActiveArtifactProcess{}, unix.ESRCH
	}
	path, err := darwinExecutablePath(pid)
	if err != nil {
		return ActiveArtifactProcess{}, err
	}
	after, err := darwinKinfoProc(pid)
	if err != nil {
		return ActiveArtifactProcess{}, err
	}
	if int(after.Proc.P_pid) != pid || after.Proc.P_stat == 5 || darwinProcessStart(before) != darwinProcessStart(after) {
		return ActiveArtifactProcess{}, ErrActiveArtifactProcessChanged
	}
	return ActiveArtifactProcess{
		PID: pid, Path: path, Started: darwinProcessStart(after),
	}, nil
}

func darwinKinfoProc(pid int) (*unix.KinfoProc, error) {
	info, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	// x/sys reports EIO for the zero-length result of a PID that has exited.
	// Confirm disappearance; a genuine unexpected kernel response still blocks.
	if errors.Is(err, unix.EIO) && errors.Is(unix.Kill(pid, 0), unix.ESRCH) {
		return nil, unix.ESRCH
	}
	return info, err
}

// Only the declared Contents/MacOS executable is an application candidate.
// Workers alongside it must not receive TERM while their owning app is quitting.
// NSRunningApplication confirms registration when graceful quit is requested.
func darwinExecutableBundle(path string) string {
	macOS := filepath.Dir(path)
	contents := filepath.Dir(macOS)
	bundle := filepath.Dir(contents)
	if filepath.Base(macOS) == "MacOS" && filepath.Base(contents) == "Contents" &&
		strings.EqualFold(filepath.Ext(bundle), ".app") {
		if executable, err := darwinBundleExecutable(filepath.Join(contents, "Info.plist")); err == nil && executable != filepath.Base(path) {
			return ""
		}
		// Missing or unreadable metadata stays an application candidate: attempt
		// AppKit's quit instead of assuming it is safe to signal a GUI process.
		return bundle
	}
	return ""
}

func darwinBundleExecutable(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if bytes.HasPrefix(data, []byte("bplist")) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		value, err := exec.CommandContext(ctx, "/usr/bin/plutil", "-extract", "CFBundleExecutable", "raw", "-o", "-", path).Output()
		return strings.TrimSpace(string(value)), err
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	found := false
	for {
		token, err := decoder.Token()
		if err != nil {
			return "", err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local == "key" {
			var key string
			if err := decoder.DecodeElement(&key, &start); err != nil {
				return "", err
			}
			found = key == "CFBundleExecutable"
		} else if found && start.Name.Local == "string" {
			var value string
			if err := decoder.DecodeElement(&value, &start); err != nil {
				return "", err
			}
			return value, nil
		}
	}
}

func verifyDarwinArtifactProcess(expected ActiveArtifactProcess) (ActiveArtifactProcess, error) {
	actual, err := darwinArtifactProcessIdentity(expected.PID)
	if errors.Is(err, unix.ESRCH) || errors.Is(err, unix.ENOENT) {
		return ActiveArtifactProcess{}, nil // Already exited; never signal a replacement.
	}
	if err != nil {
		return ActiveArtifactProcess{}, err
	}
	if actual.Path != expected.Path || actual.Started != expected.Started {
		return ActiveArtifactProcess{}, ErrActiveArtifactProcessChanged
	}
	return actual, nil
}

func stopActiveArtifactProcess(ctx context.Context, expected ActiveArtifactProcess, force bool) error {
	actual, err := verifyDarwinArtifactProcess(expected)
	if err != nil || actual.PID == 0 {
		return err
	}
	if !force {
		actual = describeActiveArtifactProcess(actual)
	}
	if !force && actual.BundlePath != "" {
		// Bound the system API request itself, as well as the CLI's subsequent
		// wait. A rejected quit request must never become an implicit signal.
		quitCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		result, err := requestDarwinApplicationQuit(quitCtx, actual)
		if err != nil {
			if quitCtx.Err() != nil {
				return quitCtx.Err()
			}
			return fmt.Errorf("request normal application quit for PID %d: %w", actual.PID, err)
		}
		switch result {
		case "requested", "exited":
			return nil
		case "changed":
			return ErrActiveArtifactProcessChanged
		case "not-application":
			// A non-AppKit worker may live alongside the app's executable.
			// Revalidate after the helper before sending its normal TERM.
			actual, err = verifyDarwinArtifactProcess(expected)
			if err != nil || actual.PID == 0 {
				return err
			}
		default:
			return fmt.Errorf("application PID %d did not accept the normal quit request (%s)", actual.PID, result)
		}
	}
	// Metadata lookup or the AppKit helper may have taken time. Validate again
	// immediately before the signal, including for workers and explicit force.
	actual, err = verifyDarwinArtifactProcess(expected)
	if err != nil || actual.PID == 0 {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	signal := unix.SIGTERM
	if force {
		signal = unix.SIGKILL
	}
	if err := unix.Kill(actual.PID, signal); err != nil && !errors.Is(err, unix.ESRCH) {
		return err
	}
	return nil
}

func requestDarwinApplicationQuit(ctx context.Context, expected ActiveArtifactProcess) (string, error) {
	cmd := exec.CommandContext(ctx, "/usr/bin/osascript", "-l", "JavaScript",
		"-e", darwinQuitApplicationScript, strconv.Itoa(expected.PID), expected.Path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}
	defer stdin.Close()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return "", err
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	reader := bufio.NewReader(stdout)
	line, err := reader.ReadString('\n')
	if err != nil {
		_ = cmd.Wait()
		return "", fmt.Errorf("application quit helper: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if strings.TrimSpace(line) == "ready" {
		// The helper has already resolved its NSRunningApplication object.
		// Revalidate start time and executable *after* helper startup and before
		// allowing delivery; a reused PID, even for the same path, is rejected.
		actual, err := verifyDarwinArtifactProcess(expected)
		if err != nil {
			return "", err
		}
		if actual.PID == 0 {
			return "exited", nil
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if _, err := io.WriteString(stdin, "y"); err != nil {
			return "", err
		}
		line, err = reader.ReadString('\n')
		if err != nil {
			return "", err
		}
	}
	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("application quit helper: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(line), nil
}

// Passing PID and path as argv avoids interpolating either into executable code.
// AppKit's terminate is a normal, asynchronous quit request (equivalent to Quit).
// Resolve symlinks in AppKit too, and recheck its executable immediately before
// delivery; other checkouts sharing a bundle identifier are never addressed.
const darwinQuitApplicationScript = `
ObjC.import('AppKit');
function run(argv) {
    var app = $.NSRunningApplication.runningApplicationWithProcessIdentifier(Number(argv[0]));
    if (!app || app.isNil()) return 'not-application';
    if (app.isTerminated) return 'exited';
    var executable = app.executableURL;
    if (!executable || executable.isNil()) return 'changed';
    var expectedPath = ObjC.unwrap($.NSURL.fileURLWithPath(argv[1]).URLByResolvingSymlinksInPath.path);
    var path = ObjC.unwrap(executable.URLByResolvingSymlinksInPath.path);
    if (path !== expectedPath) return 'changed';
    $.NSFileHandle.fileHandleWithStandardOutput.writeData($('ready\n').dataUsingEncoding($.NSUTF8StringEncoding));
    var permission = $.NSFileHandle.fileHandleWithStandardInput.readDataOfLength(1);
    if (Number(permission.length) !== 1) return 'cancelled';
    if (app.isTerminated) return 'exited';
    if (ObjC.unwrap(app.executableURL.URLByResolvingSymlinksInPath.path) !== expectedPath) return 'changed';
    return app.terminate ? 'requested' : 'declined';
}
`
