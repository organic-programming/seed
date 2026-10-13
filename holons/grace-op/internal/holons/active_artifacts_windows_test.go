//go:build windows

package holons

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/organic-programming/grace-op/internal/identity"
	"github.com/organic-programming/grace-op/internal/testutil"
)

// Copies of this test binary provide real, loaded Windows executable images.
func TestActiveWindowsArtifactProcessHelper(t *testing.T) {
	if os.Getenv("OP_ACTIVE_ARTIFACT_TEST_HELPER") != "1" {
		return
	}
	fmt.Fprintln(os.Stdout, "ready")
	_, _ = io.Copy(io.Discard, os.Stdin)
	os.Exit(0)
}

func startActiveArtifactTestProcess(t *testing.T, path string) (int, func()) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(path, "-test.run=^TestActiveWindowsArtifactProcessHelper$")
	cmd.Env = append(os.Environ(), "OP_ACTIVE_ARTIFACT_TEST_HELPER=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	stopped := false
	stop := func() {
		if stopped {
			return
		}
		stopped = true
		_ = stdin.Close()
		if err := cmd.Wait(); err != nil {
			t.Errorf("stop fixture process: %v", err)
		}
	}
	t.Cleanup(stop)
	ready := make(chan error, 1)
	go func() {
		var text string
		_, err := fmt.Fscanln(stdout, &text)
		if err == nil && text != "ready" {
			err = fmt.Errorf("unexpected helper output %q", text)
		}
		ready <- err
	}()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("fixture process did not start")
	}
	return cmd.Process.Pid, stop
}

func activeArtifactCompositeFixture(t *testing.T) (string, []string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "Projet été")
	parent := filepath.Join(root, "parent")
	child := writeProtoGoHolonFixture(t, root, "child")
	sibling := writeProtoGoHolonFixture(t, root, "sibling")
	manifest := `schema: holon/v0
kind: composite
build:
  runner: recipe
  members:
    - id: sibling
      path: ../sibling
      type: holon
    - id: child
      path: ../child
      type: holon
  targets:
    windows:
      steps:
        - build_member: sibling
        - build_member: child
artifacts:
  primary: .op/build/parent.holon
`
	if err := testutil.WriteManifestFile(filepath.Join(parent, identity.ManifestFileName), manifest); err != nil {
		t.Fatal(err)
	}
	var markers []string
	for _, dir := range []string{parent, sibling, child} {
		marker := filepath.Join(dir, ".op", "preserve.txt")
		if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(marker, []byte("original artifact\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		markers = append(markers, marker)
	}
	return parent, markers
}

func TestActiveWindowsArtifactsRejectBeforeAnyCompositeMutation(t *testing.T) {
	for _, location := range []string{"parent", "child", "duplicate-uuid"} {
		t.Run(location, func(t *testing.T) {
			parent, markers := activeArtifactCompositeFixture(t)
			processDir := parent
			if location != "parent" {
				processDir = filepath.Join(filepath.Dir(parent), "child")
			}
			if location == "duplicate-uuid" {
				// Two source copies can share an identity, but clean visits both.
				proto := filepath.Join(processDir, "api", "v1", "holon.proto")
				bytes, err := os.ReadFile(proto)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(proto, []byte(strings.ReplaceAll(string(bytes), "child-uuid", "sibling-uuid")), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(processDir, ".op", "build", "demo.holon", "bin", "windows_amd64", "demo.exe")
			pid, stop := startActiveArtifactTestProcess(t, path)
			manifest, err := LoadManifest(parent)
			if err != nil {
				t.Fatal(err)
			}
			// An ordinary build would normally clean this hardened state first.
			if err := writeBuildState(manifest, BuildContext{Target: "windows", Hardened: true}); err != nil {
				t.Fatal(err)
			}
			for _, op := range []Operation{OperationClean, OperationBuild} {
				report, err := ExecuteLifecycle(op, parent, BuildOptions{Target: "windows", Mode: "release"})
				if err == nil {
					t.Fatalf("%s accepted running process", op)
				}
				for _, want := range []string{"cannot " + string(op), "demo.exe", "PID " + strconv.Itoa(pid), path, "Close", "retry"} {
					if !strings.Contains(err.Error(), want) {
						t.Fatalf("%s error %q lacks %q", op, err, want)
					}
				}
				if len(report.Children) != 0 || len(report.Commands) != 0 {
					t.Fatalf("%s started lifecycle work before refusal: %+v", op, report)
				}
				for _, marker := range markers {
					bytes, err := os.ReadFile(marker)
					if err != nil || string(bytes) != "original artifact\n" {
						t.Fatalf("%s mutated %s: bytes=%q error=%v", op, marker, bytes, err)
					}
				}
				state, err := readBuildState(manifest)
				if err != nil || state == nil || !state.Hardened {
					t.Fatalf("%s changed prior build state: %+v, %v", op, state, err)
				}
			}
			if err := guardActiveWindowsArtifacts(manifest, BuildContext{DryRun: true}, OperationBuild); err != nil {
				t.Fatalf("dry-run rejected: %v", err)
			}
			for _, op := range []Operation{OperationCheck, OperationTest} {
				if err := guardActiveWindowsArtifacts(manifest, BuildContext{}, op); err != nil {
					t.Fatalf("%s rejected by mutation guard: %v", op, err)
				}
			}
			stop()
			if _, err := ExecuteLifecycle(OperationClean, parent); err != nil {
				t.Fatalf("clean after application exit: %v", err)
			}
			for _, marker := range markers {
				if _, err := os.Stat(marker); !os.IsNotExist(err) {
					t.Fatalf("artifact survived successful clean: %s, %v", marker, err)
				}
			}
		})
	}
}

func TestActiveWindowsArtifactsIgnoreSameNamedApplicationOutsideOutputs(t *testing.T) {
	parent, _ := activeArtifactCompositeFixture(t)
	// The prefix deliberately resembles .op without being its descendant.
	path := filepath.Join(parent, ".op-unrelated", "demo.exe")
	_, stop := startActiveArtifactTestProcess(t, path)
	defer stop()
	if _, err := ExecuteLifecycle(OperationClean, parent); err != nil {
		t.Fatalf("unrelated application blocked clean: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("unrelated application changed: %v", err)
	}
}

func TestActiveWindowsArtifactsCustomPrimaryFile(t *testing.T) {
	parent, _ := activeArtifactCompositeFixture(t)
	manifest, err := LoadManifest(parent)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Manifest.Artifacts.Primary = "custom/demo.exe"
	path := filepath.Join(parent, "custom", "demo.exe")
	_, stop := startActiveArtifactTestProcess(t, path)
	defer stop()
	for _, target := range []string{"windows", "all"} {
		if err := guardActiveWindowsArtifacts(manifest, BuildContext{Target: target}, OperationBuild); err == nil {
			t.Fatalf("custom primary executable not protected for target %q", target)
		}
	}
	manifest.Manifest.Artifacts.Primary = "custom/different.exe"
	if err := guardActiveWindowsArtifacts(manifest, BuildContext{Target: "windows"}, OperationBuild); err != nil {
		t.Fatalf("another executable in the same directory should not block: %v", err)
	}
}

func TestWindowsArtifactPathCanonicalization(t *testing.T) {
	dir := t.TempDir()
	root, err := canonicalWindowsArtifactPath(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{dir, strings.ToUpper(dir), `\\?\` + dir, filepath.Join(dir, "missing", "..", "demo.exe")} {
		canonical, err := canonicalWindowsArtifactPath(path)
		if err != nil {
			t.Fatalf("canonicalize %q: %v", path, err)
		}
		if !windowsArtifactContains(activeArtifactRoot{path: root, directory: true}, canonical) {
			t.Fatalf("%q not recognized as beneath %q", canonical, root)
		}
	}
	if windowsArtifactContains(activeArtifactRoot{path: root, directory: true}, root+"-other\\demo.exe") {
		t.Fatal("matched an unrelated path prefix")
	}
	if windowsArtifactContains(activeArtifactRoot{path: root, directory: false}, root+`\demo.exe`) {
		t.Fatal("matched a child beneath an exact file artifact")
	}
	link := filepath.Join(t.TempDir(), "junction-alias")
	if err := os.Symlink(dir, link); err != nil {
		t.Skipf("directory symlink is unavailable: %v", err)
	}
	canonical, err := canonicalWindowsArtifactPath(filepath.Join(link, "not-created", "demo.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "not-created", "demo.exe"); canonical != want {
		t.Fatalf("symlink ancestor path=%q, want %q", canonical, want)
	}
}
