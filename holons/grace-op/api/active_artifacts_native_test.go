//go:build darwin || windows

package api

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/grace-op/internal/identity"
	"github.com/organic-programming/grace-op/internal/testutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAPIActiveArtifactHelper(t *testing.T) {
	if os.Getenv("OP_API_ACTIVE_ARTIFACT_HELPER") != "1" {
		return
	}
	fmt.Fprintln(os.Stdout, "ready")
	_, _ = io.Copy(os.Stdout, os.Stdin)
	os.Exit(0)
}

type forbiddenPromptReader struct{ t *testing.T }

func (r forbiddenPromptReader) Read([]byte) (int, error) {
	r.t.Error("API attempted to read an interactive response")
	return 0, io.EOF
}

func TestAPIAndRPCRejectActiveArtifactsWithoutInteraction(t *testing.T) {
	root := t.TempDir()
	name := "worker"
	target := "macos"
	if runtime.GOOS == "windows" {
		name += ".exe"
		target = "windows"
	}
	manifest := fmt.Sprintf(`schema: holon/v0
kind: composite
build:
  runner: recipe
  members:
    - id: app
      path: app
      type: component
  defaults:
    target: %s
  targets:
    %s:
      steps:
        - assert_file:
            path: must-not-run.txt
artifacts:
  primary: .op/build/missing-app
`, target, target)
	if err := os.MkdirAll(filepath.Join(root, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testutil.WriteManifestFile(filepath.Join(root, identity.ManifestFileName), manifest); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".op", "build", "Previous.app", "Contents", "Helpers", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(path, "-test.run=^TestAPIActiveArtifactHelper$")
	cmd.Env = append(os.Environ(), "OP_API_ACTIVE_ARTIFACT_HELPER=1")
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
		if err := cmd.Wait(); err != nil {
			t.Errorf("fixture process unexpectedly stopped: %v", err)
		}
	})
	lines := make(chan string, 2)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		close(lines)
	}()
	waitLine := func(want string) {
		t.Helper()
		select {
		case got := <-lines:
			if got != want {
				t.Fatalf("fixture output = %q, want %q", got, want)
			}
		case <-time.After(20 * time.Second):
			t.Fatal("fixture process failed to respond")
		}
	}
	waitLine("ready")

	var stderr bytes.Buffer
	req := &opv1.RunRequest{Holon: root, Target: target}
	response, err := runWithIO(req, runIO{stdin: forbiddenPromptReader{t}, stderr: &stderr})
	var conflict *ActiveArtifactConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("API error = %v, want typed native conflict", err)
	}
	if response == nil || response.Command != "" {
		t.Fatalf("API launched a command: %+v", response)
	}
	if len(conflict.Processes) != 1 || conflict.Processes[0].PID != cmd.Process.Pid {
		t.Fatalf("processes = %+v, want fixture PID %d", conflict.Processes, cmd.Process.Pid)
	}
	if strings.Contains(stderr.String(), "[y/N]") {
		t.Fatalf("API prompted: %s", stderr.String())
	}
	_, rpcErr := (RPCHandler{}).Run(context.Background(), req)
	if status.Code(rpcErr) != codes.FailedPrecondition {
		t.Fatalf("RPC Run error = %v", rpcErr)
	}
	_, rpcErr = (RPCHandler{}).Clean(context.Background(), &opv1.LifecycleRequest{Target: root})
	if status.Code(rpcErr) != codes.FailedPrecondition {
		t.Fatalf("RPC Clean error = %v", rpcErr)
	}
	_, rpcErr = (RPCHandler{}).Build(context.Background(), &opv1.LifecycleRequest{Target: root})
	if status.Code(rpcErr) != codes.FailedPrecondition {
		t.Fatalf("RPC Build error = %v", rpcErr)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("artifact changed before refusal: %v", err)
	}
	if _, err := fmt.Fprintln(stdin, "still running"); err != nil {
		t.Fatalf("API stopped the fixture process: %v", err)
	}
	waitLine("still running")
}
