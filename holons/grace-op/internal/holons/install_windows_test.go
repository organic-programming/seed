//go:build windows

package holons

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	sdkdiscover "github.com/organic-programming/go-holons/pkg/discover"
)

func TestWindowsPackageExecutableName(t *testing.T) {
	dir := writeProtoGoHolonFixture(t, t.TempDir(), "windows-fixture")
	manifest, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := filepath.Base(manifest.BinaryPath()); got != "windows-fixture.exe" {
		t.Fatalf("binary filename = %q", got)
	}
	if got := manifest.BinaryName(); got != "windows-fixture" {
		t.Fatalf("logical name changed: %q", got)
	}
	if err := os.MkdirAll(filepath.Dir(manifest.BinaryPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest.BinaryPath(), []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeHolonJSON(manifest, BuildContext{}); err != nil {
		t.Fatal(err)
	}
	pkg, err := readHolonPackageJSON(manifest.HolonPackageDir())
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Entrypoint != "windows-fixture.exe" {
		t.Fatalf("package entrypoint = %q", pkg.Entrypoint)
	}
	if got := LaunchableArtifactPath(manifest.HolonPackageDir(), manifest); got != manifest.BinaryPath() {
		t.Fatalf("launchable path = %q, want %q", got, manifest.BinaryPath())
	}
}

func TestWindowsInstallRunningExecutable(t *testing.T) {
	if os.Getenv("OP_INSTALL_TEST_CHILD") == "1" {
		fmt.Println("ready")
		_, _ = io.Copy(io.Discard, os.Stdin)
		return
	}
	source, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	installed := filepath.Join(t.TempDir(), "op.exe")
	if err := installExecutableCopy(source, installed); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, installed, "-test.run=^TestWindowsInstallRunningExecutable$")
	cmd.Env = append(os.Environ(), "OP_INSTALL_TEST_CHILD=1")
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
	t.Cleanup(func() { _ = stdin.Close(); _ = cmd.Wait() })
	if line, err := bufio.NewReader(stdout).ReadString('\n'); err != nil || line != "ready\n" {
		t.Fatalf("child readiness = %q, %v", line, err)
	}
	if err := installExecutableCopy(source, installed); err != nil {
		t.Fatalf("replace running launcher: %v", err)
	}
	if info, err := os.Lstat(installed); err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("launcher must be a regular executable: %v, %v", info, err)
	}
	probe := exec.CommandContext(ctx, installed, "-test.run=^TestWindowsInstallRunningExecutable$")
	probe.Env = append(os.Environ(), "OP_INSTALL_TEST_CHILD=1")
	if output, err := probe.CombinedOutput(); err != nil {
		t.Fatalf("replacement launch failed: %v\n%s", err, output)
	}
	if err := removeInstalledExecutable(installed); err != nil {
		t.Fatalf("uninstall launcher: %v", err)
	}
	if _, err := os.Stat(installed); !os.IsNotExist(err) {
		t.Fatalf("launcher still installed: %v", err)
	}
}

func TestWindowsSelfInstallAndUninstall(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OPPATH", filepath.Join(root, "runtime"))
	t.Setenv("OPBIN", filepath.Join(root, "runtime", "bin"))
	dir := writeProtoGoHolonFixture(t, root, "op")
	manifest, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(manifest.BinaryPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest.BinaryPath(), []byte("standalone executable fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeHolonJSON(manifest, BuildContext{}); err != nil {
		t.Fatal(err)
	}
	opts := InstallOptions{ResolveSpecifiers: sdkdiscover.SOURCE}
	report, err := Install(dir, opts)
	if err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(os.Getenv("OPBIN"), "op.exe")
	if info, err := os.Lstat(launcher); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("launcher must be a regular file: %v, %v", info, err)
	}
	if _, err := os.Stat(PackageBinaryPath(report.Installed, "op")); err != nil {
		t.Fatalf("package binary missing: %v", err)
	}
	if _, err := UninstallWithOptions(dir, opts); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{launcher, report.Installed} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("uninstall left %s: %v", path, err)
		}
	}
}
