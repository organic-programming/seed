//go:build darwin

package holons

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestDarwinActiveArtifactProcessHelper(t *testing.T) {
	if os.Getenv("OP_DARWIN_ARTIFACT_HELPER") != "1" {
		return
	}
	if os.Getenv("OP_DARWIN_ARTIFACT_IGNORE_TERM") == "1" {
		signal.Ignore(syscall.SIGTERM)
	}
	fmt.Fprintln(os.Stdout, "ready")
	_, _ = io.Copy(io.Discard, os.Stdin)
	os.Exit(0)
}

// Every signal in these tests targets a private copy of this test executable.
func startDarwinArtifactTestProcess(t *testing.T, path string, ignoreTERM bool) (*exec.Cmd, ActiveArtifactProcess) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o755); err != nil {
		t.Fatal(err)
	}
	// A relative executable path tests that native inventory never relies on
	// the original command line or the caller's working directory.
	cmd := exec.Command("./"+filepath.Base(path), "-test.run=^TestDarwinActiveArtifactProcessHelper$")
	cmd.Dir = filepath.Dir(path)
	cmd.Env = append(os.Environ(), "OP_DARWIN_ARTIFACT_HELPER=1")
	if ignoreTERM {
		cmd.Env = append(cmd.Env, "OP_DARWIN_ARTIFACT_IGNORE_TERM=1")
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	ready := make(chan string, 1)
	go func() { line, _ := bufio.NewReader(stdout).ReadString('\n'); ready <- line }()
	select {
	case line := <-ready:
		if line != "ready\n" {
			t.Fatalf("unexpected helper output %q", line)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("helper did not become ready")
	}
	process, err := darwinArtifactProcess(cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	return cmd, process
}

func waitDarwinArtifactExit(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := darwinArtifactProcess(pid); errors.Is(err, syscall.ESRCH) || errors.Is(err, syscall.EIO) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("fixture PID %d did not exit", pid)
}

func TestDarwinArtifactProcessInventoryAndIdentity(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Projet été", "App.app", "Contents", "Frameworks", "worker")
	_, process := startDarwinArtifactTestProcess(t, path, true)
	want, err := canonicalArtifactPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if process.Path != want || process.Started == "" || process.BundlePath != "" {
		t.Fatalf("unexpected worker identity: %+v; want %s", process, want)
	}
	processes, err := activeArtifactProcesses()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, candidate := range processes {
		if candidate.PID == process.PID {
			found = candidate.Path == want && candidate.Started == process.Started
		}
	}
	if !found {
		t.Fatal("native inventory omitted the dedicated worker")
	}
	for _, change := range []string{"path", "start"} {
		t.Run(change, func(t *testing.T) {
			wrong := process
			if change == "path" {
				wrong.Path = filepath.Join(root, "other-checkout", "worker")
			} else {
				wrong.Started += "-other-incarnation"
			}
			for _, force := range []bool{false, true} {
				if err := RequestActiveArtifactStop(context.Background(), wrong, force); !errors.Is(err, ErrActiveArtifactProcessChanged) {
					t.Fatalf("force=%v: got %v; want identity refusal", force, err)
				}
			}
			if _, err := darwinArtifactProcess(process.PID); err != nil {
				t.Fatalf("changed identity was signalled: %v", err)
			}
		})
	}
	if err := RequestActiveArtifactStop(context.Background(), process, false); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	if _, err := darwinArtifactProcess(process.PID); err != nil {
		t.Fatalf("TERM-ignoring worker exited without force: %v", err)
	}
	if err := RequestActiveArtifactStop(context.Background(), process, true); err != nil {
		t.Fatal(err)
	}
	waitDarwinArtifactExit(t, process.PID)
}

func TestDarwinArtifactWorkerNormalStop(t *testing.T) {
	_, process := startDarwinArtifactTestProcess(t, filepath.Join(t.TempDir(), "worker"), false)
	if err := RequestActiveArtifactStop(context.Background(), process, false); err != nil {
		t.Fatal(err)
	}
	waitDarwinArtifactExit(t, process.PID)
	if err := RequestActiveArtifactStop(context.Background(), process, true); err != nil {
		t.Fatalf("already exited process: %v", err)
	}
}

func TestDarwinExecutablePathFollowsRenameAndSymlink(t *testing.T) {
	dir := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(dir, alias); err != nil {
		t.Fatal(err)
	}
	_, process := startDarwinArtifactTestProcess(t, filepath.Join(alias, "worker"), false)
	renamed := filepath.Join(dir, "renamed-worker")
	if err := os.Rename(filepath.Join(dir, "worker"), renamed); err != nil {
		t.Fatal(err)
	}
	actual, err := darwinArtifactProcess(process.PID)
	if err != nil {
		t.Fatal(err)
	}
	want, err := canonicalArtifactPath(renamed)
	if err != nil {
		t.Fatal(err)
	}
	if actual.Path != want {
		t.Fatalf("kernel executable path %q; want renamed path %q", actual.Path, want)
	}
	if err := RequestActiveArtifactStop(context.Background(), process, true); !errors.Is(err, ErrActiveArtifactProcessChanged) {
		t.Fatalf("stale pre-rename path accepted: %v", err)
	}
}

func TestDarwinExecutableBundleDistinguishesSiblingWorker(t *testing.T) {
	bundle := filepath.Join(t.TempDir(), "App.app")
	contents := filepath.Join(bundle, "Contents")
	if err := os.MkdirAll(contents, 0o755); err != nil {
		t.Fatal(err)
	}
	plist := filepath.Join(contents, "Info.plist")
	if err := os.WriteFile(plist, []byte(`<?xml version="1.0"?><plist version="1.0"><dict><key>CFBundleExecutable</key><string>main</string></dict></plist>`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"xml", "binary"} {
		t.Run(format, func(t *testing.T) {
			if format == "binary" {
				if output, err := exec.Command("/usr/bin/plutil", "-convert", "binary1", plist).CombinedOutput(); err != nil {
					t.Fatalf("binary fixture conversion: %v: %s", err, output)
				}
			}
			if got := darwinExecutableBundle(filepath.Join(contents, "MacOS", "main")); got != bundle {
				t.Fatalf("main bundle=%q; want %q", got, bundle)
			}
			if got := darwinExecutableBundle(filepath.Join(contents, "MacOS", "worker")); got != "" {
				t.Fatalf("sibling worker classified as main app: %q", got)
			}
			if got := darwinExecutableBundle(filepath.Join(contents, "Frameworks", "worker")); got != "" {
				t.Fatalf("embedded worker classified as main app: %q", got)
			}
		})
	}
}

func TestDarwinNormalApplicationQuit(t *testing.T) {
	clang, err := exec.LookPath("clang")
	if err != nil {
		t.Skip("native application fixture requires clang")
	}
	dir := t.TempDir()
	bundle := filepath.Join(dir, "Dedicated Quit Fixture.app")
	executable := filepath.Join(bundle, "Contents", "MacOS", "fixture")
	if err := os.MkdirAll(filepath.Dir(executable), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "Contents", "Info.plist"), []byte(`<?xml version="1.0"?><plist version="1.0"><dict><key>CFBundleExecutable</key><string>fixture</string><key>CFBundleIdentifier</key><string>org.organic-programming.op-test-quit</string><key>CFBundlePackageType</key><string>APPL</string><key>LSUIElement</key><true/></dict></plist>`), 0o644); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "fixture.m")
	code := `#import <AppKit/AppKit.h>
@interface QuitDelegate : NSObject <NSApplicationDelegate>
@end
@implementation QuitDelegate
- (void)applicationDidFinishLaunching:(NSNotification *)notification {
    printf("ready\n"); fflush(stdout);
}
- (NSApplicationTerminateReply)applicationShouldTerminate:(NSApplication *)sender {
    [@"normal quit" writeToFile:@(getenv("OP_QUIT_MARKER")) atomically:YES encoding:NSUTF8StringEncoding error:nil];
    return NSTerminateNow;
}
@end
int main(void) { @autoreleasepool {
    [NSApplication sharedApplication];
    [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
    QuitDelegate *delegate = [QuitDelegate new];
    NSApp.delegate = delegate;
    [NSApp run];
} return 0; }
`
	if err := os.WriteFile(source, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(clang, "-x", "objective-c", source, "-framework", "AppKit", "-o", executable).CombinedOutput(); err != nil {
		t.Fatalf("compile native test application: %v: %s", err, output)
	}
	marker := filepath.Join(dir, "quit-marker")
	cmd := exec.Command(executable)
	cmd.Env = append(os.Environ(), "OP_QUIT_MARKER="+marker)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	ready := make(chan string, 1)
	go func() { line, _ := bufio.NewReader(stdout).ReadString('\n'); ready <- line }()
	select {
	case line := <-ready:
		if !strings.Contains(line, "ready") {
			t.Fatalf("native app failed to start: %q", line)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("native app did not become ready")
	}
	process, err := darwinArtifactProcess(cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if process.BundlePath == "" {
		t.Fatal("native app was classified as a worker")
	}
	// Exercise the second identity check after AppKit has resolved its object,
	// not just the check before the helper starts. A PID reused for the same
	// executable must never receive the stale user's quit request.
	wrong := process
	wrong.Started += "-different-process"
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	_, err = requestDarwinApplicationQuit(ctx, wrong)
	cancel()
	if !errors.Is(err, ErrActiveArtifactProcessChanged) {
		t.Fatalf("AppKit handshake accepted changed start time: %v", err)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("AppKit quit delivered despite changed identity: %v", err)
	}
	expired, cancelExpired := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelExpired()
	if _, err := requestDarwinApplicationQuit(expired, process); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expired helper request: %v", err)
	}
	if _, err := darwinArtifactProcess(process.PID); err != nil {
		t.Fatalf("expired request stopped the app: %v", err)
	}
	if err := RequestActiveArtifactStop(context.Background(), process, false); err != nil {
		t.Fatal(err)
	}
	waitDarwinArtifactExit(t, process.PID)
	data, err := os.ReadFile(marker)
	if err != nil || string(data) != "normal quit" {
		t.Fatalf("normal app delegate quit was not requested: data=%q error=%v", data, err)
	}
}
