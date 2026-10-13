//go:build e2e && windows

package windows_live_artifact_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	sdkgrpc "github.com/organic-programming/go-holons/pkg/grpcclient"
	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
)

// A copy of this native executable holds an actual Windows image section in
// the output tree. Its stdin protocol proves OP did not stop the process.
func TestMain(m *testing.M) {
	if strings.EqualFold(filepath.Base(os.Args[0]), "live-probe.exe") {
		fmt.Println("ready")
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			switch scanner.Text() {
			case "ping":
				fmt.Println("alive")
			case "quit":
				os.Exit(0)
			}
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type proof struct {
	Locations       []string `json:"locations"`
	Operations      []string `json:"operations"`
	GraphUnchanged  bool     `json:"graph_unchanged"`
	ProcessesAlive  bool     `json:"processes_alive"`
	ActionableError bool     `json:"actionable_error"`
	CleanAfterExit  bool     `json:"clean_after_exit"`
}

type lifecycleCall func(operation, target string) error

func TestWindowsLiveArtifact_CLI(t *testing.T) { prove(t, "CLI") }
func TestWindowsLiveArtifact_API(t *testing.T) { prove(t, "API") }
func TestWindowsLiveArtifact_RPC(t *testing.T) { prove(t, "RPC") }

func prove(t *testing.T, facet string) {
	op, results, snapshot := os.Getenv("ADER_LIVE_ARTIFACT_OP"), os.Getenv("ADER_LIVE_ARTIFACT_RESULTS"), os.Getenv("ADER_REPO_ROOT")
	if op == "" || results == "" || snapshot == "" {
		t.Fatal("run through the windows-live-artifact Ader suite")
	}
	workspace := t.TempDir()
	t.Chdir(workspace)
	t.Setenv("OPROOT", workspace)
	t.Setenv("OPPATH", filepath.Join(workspace, "op-home"))
	t.Setenv("OPBIN", filepath.Join(workspace, "op-home", "bin"))
	t.Setenv("GRACE_OP_SHARED_CACHE_DIR", filepath.Join(workspace, "package-cache"))
	call := newCaller(t, facet, op, workspace)
	for _, location := range []string{"parent", "first-member", "last-member", "same-uuid-last-member"} {
		t.Run(location, func(t *testing.T) {
			project := filepath.Join(workspace, "project é 日本 "+location)
			parent, roots := fixture(t, project, snapshot, location == "same-uuid-last-member")
			index := map[string]int{"parent": 0, "first-member": 1, "last-member": 2, "same-uuid-last-member": 2}[location]
			executable := filepath.Join(roots[index], "build", location+".holon", "bin", "windows_amd64", "live-probe.exe")
			copySelf(t, executable)
			process := startProbe(t, executable)
			before := outputSnapshot(t, roots)
			for _, operation := range []string{"clean", "build"} {
				err := call(operation, parent)
				assertRefusal(t, err, process.cmd.Process.Pid, executable)
				if !reflect.DeepEqual(before, outputSnapshot(t, roots)) {
					t.Fatalf("%s changed parent/member outputs before refusing the live artifact", operation)
				}
				process.ping(t)
			}
			if facet == "CLI" {
				// The exact user flow is CLI-only: the public RunRequest has no
				// clean field. Clean and Build above prove API/RPC symmetry.
				err := runCLI(op, workspace, "run", parent, "--clean")
				assertRefusal(t, err, process.cmd.Process.Pid, executable)
				if !reflect.DeepEqual(before, outputSnapshot(t, roots)) {
					t.Fatal("run --clean changed outputs before refusing")
				}
				process.ping(t)
			}
			process.close(t)
			if err := call("clean", parent); err != nil {
				t.Fatalf("clean after graceful fixture exit: %v", err)
			}
			for _, root := range roots {
				if _, err := os.Stat(root); !os.IsNotExist(err) {
					t.Fatalf("output remained after successful clean: %s: %v", root, err)
				}
			}
		})
	}
	if t.Failed() {
		return
	}
	got := proof{[]string{"parent", "first-member", "last-member", "same-uuid-last-member"}, []string{"clean", "build"}, true, true, true, true}
	data, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(results, facet+".json"), data)
	t.Logf("persistent %s proof: %s", facet, data)
	if facet == "RPC" {
		for _, previous := range []string{"CLI", "API"} {
			data, err := os.ReadFile(filepath.Join(results, previous+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var earlier proof
			if err := json.Unmarshal(data, &earlier); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, earlier) {
				t.Fatalf("%s/RPC proofs differ", previous)
			}
		}
	}
}

func assertRefusal(t *testing.T, err error, pid int, executable string) {
	t.Helper()
	if err == nil {
		t.Fatal("operation accepted a running artifact")
	}
	for _, fragment := range []string{"running", "PID " + strconv.Itoa(pid), filepath.Base(executable), "Close", "retry"} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("refusal lacks %q: %v", fragment, err)
		}
	}
}

func newCaller(t *testing.T, facet, op, cwd string) lifecycleCall {
	t.Helper()
	if facet == "CLI" {
		return func(operation, target string) error { return runCLI(op, cwd, operation, target) }
	}
	var client opv1.OPServiceClient
	if facet == "RPC" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		cmd := exec.Command(op, "serve", "--listen", "stdio://")
		cmd.Dir, cmd.Env = cwd, os.Environ()
		conn, started, err := sdkgrpc.DialStdioCommand(ctx, cmd)
		if err != nil {
			cancel()
			t.Fatalf("native stdio RPC: %v", err)
		}
		t.Cleanup(func() {
			_ = conn.Close()
			if started != nil && started.Process != nil {
				_ = started.Process.Kill()
				_ = started.Wait()
			}
			cancel()
		})
		client = opv1.NewOPServiceClient(conn)
	}
	return func(operation, target string) error {
		req := &opv1.LifecycleRequest{Target: target}
		var response *opv1.LifecycleResponse
		var err error
		if facet == "API" {
			if operation == "clean" {
				response, err = api.Clean(req)
			} else {
				response, err = api.Build(req)
			}
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if operation == "clean" {
				response, err = client.Clean(ctx, req)
			} else {
				response, err = client.Build(ctx, req)
			}
		}
		if err == nil && response.GetReport().GetOperation() != operation {
			return fmt.Errorf("unexpected lifecycle report: %v", response)
		}
		return err
	}
}

func runCLI(op, cwd string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, op, args...)
	cmd.Dir = cwd
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, output)
	}
	return nil
}

type liveProbe struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	lines  chan string
	done   chan error
	closed bool
}

func startProbe(t *testing.T, path string) *liveProbe {
	t.Helper()
	cmd := exec.Command(path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	p := &liveProbe{cmd: cmd, stdin: stdin, lines: make(chan string, 4), done: make(chan error, 1)}
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			p.lines <- scanner.Text()
		}
		close(p.lines)
	}()
	go func() { p.done <- cmd.Wait() }()
	t.Cleanup(func() {
		if !p.closed {
			p.close(t)
		}
	})
	p.expect(t, "ready")
	return p
}

func (p *liveProbe) expect(t *testing.T, want string) {
	t.Helper()
	select {
	case got, ok := <-p.lines:
		if !ok || got != want {
			t.Fatalf("fixture response = %q, want %q (open %v)", got, want, ok)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("fixture timed out waiting for %q", want)
	}
}

func (p *liveProbe) ping(t *testing.T) {
	t.Helper()
	if _, err := io.WriteString(p.stdin, "ping\n"); err != nil {
		t.Fatalf("fixture stopped by OP: %v", err)
	}
	p.expect(t, "alive")
}

func (p *liveProbe) close(t *testing.T) {
	t.Helper()
	p.closed = true
	_, _ = io.WriteString(p.stdin, "quit\n")
	_ = p.stdin.Close()
	select {
	case err := <-p.done:
		if err != nil {
			t.Errorf("fixture did not exit normally: %v", err)
		}
	case <-time.After(10 * time.Second):
		_ = p.cmd.Process.Kill() // Test-owned fixture only; never a user process.
		<-p.done
		t.Error("fixture required forced test cleanup")
	}
}

func fixture(t *testing.T, project, snapshot string, sameIdentity bool) (string, []string) {
	t.Helper()
	if err := os.CopyFS(filepath.Join(project, "_protos"), os.DirFS(filepath.Join(snapshot, "holons", "grace-op", "_protos"))); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(project, "live-app")
	write(t, filepath.Join(parent, "api", "v1", "holon.proto"), []byte(parentProto))
	write(t, filepath.Join(parent, "app", "fixture.txt"), []byte("component"))
	roots := []string{filepath.Join(parent, ".op")}
	for index, name := range []string{"first-member", "last-member"} {
		dir := filepath.Join(project, name)
		identity := fmt.Sprintf("db8a2dc5-2e47-48b6-90d3-21999c06707%d", index)
		if sameIdentity {
			identity = "db8a2dc5-2e47-48b6-90d3-21999c067070"
		}
		manifest := strings.ReplaceAll(strings.ReplaceAll(memberProto, "MEMBER", name), "UUID", identity)
		write(t, filepath.Join(dir, "api", "v1", "holon.proto"), []byte(manifest))
		write(t, filepath.Join(dir, "cmd", "probe", "main.go"), []byte("package main\nfunc main() {}\n"))
		write(t, filepath.Join(dir, "go.mod"), []byte("module example.org/"+name+"\n\ngo 1.24\n"))
		roots = append(roots, filepath.Join(dir, ".op"))
	}
	for _, root := range roots {
		write(t, filepath.Join(root, "build", "preserve.txt"), []byte("output must survive a refused operation"))
		write(t, filepath.Join(root, "metadata.json"), []byte(`{"original":true}`))
	}
	return parent, roots
}

func outputSnapshot(t *testing.T, roots []string) map[string]string {
	t.Helper()
	result := map[string]string{}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				result[path] = "directory"
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			hash := sha256.Sum256(data)
			result[path] = hex.EncodeToString(hash[:])
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func copySelf(t *testing.T, destination string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	write(t, destination, data)
}

func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

const parentProto = `syntax = "proto3";
package liveartifact.v1;
import "holons/v1/manifest.proto";
option (holons.v1.manifest) = {
 identity: { schema: "holon/v1" uuid: "1b023273-caf8-454d-8377-2a03aed05df0" given_name: "Live" family_name: "App" status: "draft" born: "2026-10-13" version: "0.0.1" }
 lang: "go" kind: "composite" platforms: ["windows"]
 build: { runner: "recipe" defaults: { target: "windows" mode: "debug" }
  members: { id: "first-member" path: "../first-member" type: "holon" }
  members: { id: "last-member" path: "../last-member" type: "holon" }
  members: { id: "app" path: "app" type: "component" }
  targets: { key: "windows" value: { steps: { build_member: "first-member" } steps: { build_member: "last-member" } } }
 }
 artifacts: { primary: ".op/build/live-app.holon" }
};
`

const memberProto = `syntax = "proto3";
package liveworker.v1;
import "holons/v1/manifest.proto";
option (holons.v1.manifest) = {
 identity: { schema: "holon/v1" uuid: "UUID" given_name: "MEMBER" status: "draft" born: "2026-10-13" version: "0.0.1" }
 lang: "go" kind: "native" platforms: ["windows"]
 build: { runner: "go-module" main: "./cmd/probe" }
 requires: { files: ["go.mod"] }
 artifacts: { binary: "MEMBER" }
};
`
