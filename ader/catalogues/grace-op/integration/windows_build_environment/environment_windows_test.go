//go:build e2e && windows

package windows_build_environment_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	sdkgrpc "github.com/organic-programming/go-holons/pkg/grpcclient"
	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

// Copies of this native test executable act as the recipe tool and protobuf
// plugin. Only the declared project environment can make them work. No shell,
// real compiler or developer environment is used by the fixture lifecycle.
func TestMain(m *testing.M) {
	name := strings.ToLower(filepath.Base(os.Args[0]))
	if name == "env-probe.exe" || name == "protoc-gen-envprobe.exe" {
		if err := runProbe(name); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runProbe(name string) error {
	token, root, log := os.Getenv("ADER_PROJECT_TOKEN"), os.Getenv("CONFIG_DIR"), os.Getenv("ADER_PROJECT_LOG")
	if token == "" || root == "" || filepath.Clean(log) != filepath.Join(root, "events.jsonl") {
		return fmt.Errorf("missing or inconsistent declared project environment")
	}
	if filepath.Clean(os.Getenv("ADER_PROJECT_SENTINEL")) != filepath.Join(root, "required.txt") {
		return fmt.Errorf("dependent environment variable was not expanded")
	}
	if name == "protoc-gen-envprobe.exe" {
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		req := &pluginpb.CodeGeneratorRequest{}
		if err := proto.Unmarshal(input, req); err != nil {
			return err
		}
		if len(req.GetFileToGenerate()) == 0 {
			return fmt.Errorf("plugin received no proto input")
		}
		// The plugin must receive the same PATH as the recipe, before preflight.
		cmd := exec.Command("env-probe.exe", "codegen")
		cmd.Env = os.Environ()
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("codegen child: %w: %s", err, output)
		}
		content := "project=" + token + "\n"
		response := &pluginpb.CodeGeneratorResponse{File: []*pluginpb.CodeGeneratorResponse_File{{Name: proto.String("environment.txt"), Content: &content}}}
		output, err := proto.Marshal(response)
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(output)
		return err
	}
	if len(os.Args) != 2 {
		return fmt.Errorf("expected probe phase")
	}
	phase := os.Args[1]
	if phase != "build" && phase != "codegen" {
		return fmt.Errorf("unexpected phase %q", phase)
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Dir(executable), filepath.Join(root, "tools")) {
		return fmt.Errorf("resolved another project's probe: %s", executable)
	}
	event, err := json.Marshal(struct{ Phase, Project string }{phase, token})
	if err != nil {
		return err
	}
	f, err := os.OpenFile(log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(append(event, '\n'))
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	if phase == "build" {
		if _, err := os.Stat(filepath.Join("gen", "envprobe", "environment.txt")); err != nil {
			return fmt.Errorf("recipe ran before codegen: %w", err)
		}
		if err := os.MkdirAll(filepath.Join(".op", "build"), 0755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(".op", "build", "probe.txt"), []byte(token), 0600)
	}
	return nil
}

type reportView struct {
	Operation string `json:"operation"`
	Target    string `json:"build_target"`
}
type lifecycleCall func(string, string) (reportView, error)
type proof struct {
	Projects                    []string `json:"projects"`
	Phases                      []string `json:"phases"`
	CheckFromDeclaredPATH       bool     `json:"check_from_declared_path"`
	CodegenBeforeRecipe         bool     `json:"codegen_before_recipe"`
	CleanRequiresEnvironment    bool     `json:"clean_requires_environment"`
	UnconfiguredProjectRejected bool     `json:"unconfigured_project_rejected"`
	ConcurrentProjects          bool     `json:"concurrent_projects"`
	ParentEnvironmentUnchanged  bool     `json:"parent_environment_unchanged"`
}

func TestWindowsBuildEnvironment_CLI(t *testing.T) { prove(t, "CLI") }
func TestWindowsBuildEnvironment_API(t *testing.T) { prove(t, "API") }
func TestWindowsBuildEnvironment_RPC(t *testing.T) { prove(t, "RPC") }

func prove(t *testing.T, facet string) {
	op, results, snapshot := os.Getenv("ADER_BUILD_ENV_OP"), os.Getenv("ADER_BUILD_ENV_RESULTS"), os.Getenv("ADER_REPO_ROOT")
	if op == "" || results == "" || snapshot == "" {
		t.Fatal("run through the windows-build-environment Ader suite")
	}
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("OPROOT", root)
	t.Setenv("OPPATH", filepath.Join(root, "op-home"))
	t.Setenv("OPBIN", filepath.Join(root, "op-home", "bin"))
	t.Setenv("GRACE_OP_SHARED_CACHE_DIR", filepath.Join(root, "package-cache"))
	t.Setenv("PATH", filepath.Join(os.Getenv("SystemRoot"), "System32"))
	for _, key := range []string{"CONFIG_DIR", "ADER_PROJECT_TOKEN", "ADER_PROJECT_LOG", "ADER_PROJECT_REQUIRED", "ADER_PROJECT_SENTINEL", "VSINSTALLDIR", "VCINSTALLDIR", "VCToolsVersion", "WindowsSdkDir", "WindowsSDKVersion", "INCLUDE", "LIB", "LIBPATH", "PROTOC", "PROTOC_INCLUDE", "LIBCLANG_PATH", "VULKAN_SDK", "CMAKE_GENERATOR", "CMAKE_TOOLCHAIN_FILE_x86_64_pc_windows_msvc"} {
		t.Setenv(key, "")
	}
	if _, err := exec.LookPath("env-probe.exe"); err == nil {
		t.Fatal("probe is visible before project preparation")
	}
	installPlugin(t)
	a := newProject(t, root, snapshot, "alpha é", "alpha")
	b := newProject(t, root, snapshot, "beta 日本", "beta")
	missing := newProject(t, root, snapshot, "unconfigured", "missing")
	if err := os.Remove(filepath.Join(missing, "seed-toolchain.yaml")); err != nil {
		t.Fatal(err)
	}
	before := environment()
	call := newCaller(t, facet, op, root)
	assertEnvironment := func() {
		t.Helper()
		if !reflect.DeepEqual(before, environment()) {
			t.Fatal("lifecycle polluted the parent process environment")
		}
	}
	for _, project := range []string{a, b, a} {
		token := "alpha"
		if project == b {
			token = "beta"
		}
		write(t, filepath.Join(project, "events.jsonl"), "")
		for _, operation := range []string{"check", "build"} {
			report, err := call(operation, filepath.Join(project, "probe"))
			if err != nil {
				t.Fatalf("%s %s: %v", facet, operation, err)
			}
			if report.Operation != operation || report.Target != "windows" {
				t.Fatalf("unexpected report: %+v", report)
			}
			assertEnvironment()
		}
		verifyBuild(t, project, token)
		// Clean must prepare/validate the environment BEFORE destructive work.
		marker := filepath.Join(project, "probe", ".op", "preserve.txt")
		write(t, marker, "preserve on failed preparation")
		if err := os.Remove(filepath.Join(project, "required.txt")); err != nil {
			t.Fatal(err)
		}
		if _, err := call("clean", filepath.Join(project, "probe")); err == nil || !strings.Contains(err.Error(), "required.txt") {
			t.Fatalf("clean did not reject missing environment prerequisite: %v", err)
		}
		if _, err := os.Stat(marker); err != nil {
			t.Fatalf("clean mutated output before validating environment: %v", err)
		}
		assertEnvironment()
		write(t, filepath.Join(project, "required.txt"), "prepared")
		if _, err := call("clean", filepath.Join(project, "probe")); err != nil {
			t.Fatalf("clean: %v", err)
		}
		if _, err := os.Stat(filepath.Dir(marker)); !os.IsNotExist(err) {
			t.Fatalf("clean did not remove output: %v", err)
		}
		assertEnvironment()
	}
	if _, err := call("check", filepath.Join(missing, "probe")); err == nil || !strings.Contains(err.Error(), "env-probe") {
		t.Fatalf("unconfigured project inherited another project's PATH: %v", err)
	}
	assertEnvironment()
	// Code API runs these calls in this one process; RPC uses one server for
	// both projects. This detects global os.Setenv implementations and caches
	// keyed only by tool name or host rather than project configuration.
	for _, project := range []string{a, b} {
		write(t, filepath.Join(project, "events.jsonl"), "")
	}
	var wg sync.WaitGroup
	errors := make(chan error, 2)
	for _, project := range []string{a, b} {
		wg.Add(1)
		go func(project string) {
			defer wg.Done()
			_, err := call("build", filepath.Join(project, "probe"))
			errors <- err
		}(project)
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("concurrent build: %v", err)
		}
	}
	verifyBuild(t, a, "alpha")
	verifyBuild(t, b, "beta")
	assertEnvironment()
	if _, err := call("check", filepath.Join(missing, "probe")); err == nil {
		t.Fatal("project environment leaked after concurrent builds")
	}
	assertEnvironment()
	got := proof{[]string{"alpha", "beta", "alpha"}, []string{"codegen", "build"}, true, true, true, true, true, true}
	data, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(results, facet+".json"), string(data))
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
	t.Logf("%s: declared PATH, codegen, clean validation, concurrent project isolation and unchanged process environment passed", facet)
}

func newCaller(t *testing.T, facet, op, cwd string) lifecycleCall {
	t.Helper()
	if facet == "CLI" {
		return func(operation, target string) (reportView, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, op, "--format", "json", operation, target)
			cmd.Dir = cwd
			var out, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &stderr
			if err := cmd.Run(); err != nil {
				return reportView{}, fmt.Errorf("%w: %s %s", err, &out, &stderr)
			}
			var report reportView
			err := json.Unmarshal(out.Bytes(), &report)
			return report, err
		}
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
	return func(operation, target string) (reportView, error) {
		req := &opv1.LifecycleRequest{Target: target}
		var response *opv1.LifecycleResponse
		var err error
		if facet == "API" {
			switch operation {
			case "check":
				response, err = api.Check(req)
			case "build":
				response, err = api.Build(req)
			case "clean":
				response, err = api.Clean(req)
			}
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			switch operation {
			case "check":
				response, err = client.Check(ctx, req)
			case "build":
				response, err = client.Build(ctx, req)
			case "clean":
				response, err = client.Clean(ctx, req)
			}
		}
		return reportView{response.GetReport().GetOperation(), response.GetReport().GetBuildTarget()}, err
	}
}

func newProject(t *testing.T, root, snapshot, name, token string) string {
	t.Helper()
	project := filepath.Join(root, name)
	if err := os.CopyFS(filepath.Join(project, "_protos"), os.DirFS(filepath.Join(snapshot, "holons", "grace-op", "_protos"))); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(project, "probe", "api", "v1", "holon.proto"), fixtureProto)
	write(t, filepath.Join(project, "probe", "app", "fixture.txt"), "component")
	write(t, filepath.Join(project, "required.txt"), "prepared")
	copySelf(t, filepath.Join(project, "tools", "env-probe.exe"))
	write(t, filepath.Join(project, "seed-toolchain.yaml"), fmt.Sprintf(`build_environments:
  windows:
    path: ["${CONFIG_DIR}/tools"]
    env:
      ADER_PROJECT_TOKEN: %q
      ADER_PROJECT_LOG: "${CONFIG_DIR}/events.jsonl"
      ADER_PROJECT_REQUIRED: "${CONFIG_DIR}/required.txt"
      ADER_PROJECT_SENTINEL: "${ADER_PROJECT_REQUIRED}"
    required_files: ["${ADER_PROJECT_REQUIRED}", "${CONFIG_DIR}/tools/env-probe.exe"]
`, token))
	return project
}

func installPlugin(t *testing.T) {
	t.Helper()
	root := filepath.Join(os.Getenv("OPPATH"), "sdk", "envprobe", "0.0.1", "x86_64-windows-gnu")
	copySelf(t, filepath.Join(root, "bin", "protoc-gen-envprobe.exe"))
	write(t, filepath.Join(root, "manifest.json"), `{"lang":"envprobe","version":"0.0.1","target":"x86_64-windows-gnu","codegen":{"plugins":[{"name":"envprobe","binary":"bin/protoc-gen-envprobe.exe","out_subdir":"envprobe"}]}}`)
}

func verifyBuild(t *testing.T, project, token string) {
	t.Helper()
	for path, want := range map[string]string{"gen/envprobe/environment.txt": "project=" + token + "\n", ".op/build/probe.txt": token} {
		got, err := os.ReadFile(filepath.Join(project, "probe", filepath.FromSlash(path)))
		if err != nil || string(got) != want {
			t.Fatalf("%s: got %q, want %q, error %v", path, got, want, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(project, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var phases []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var event struct{ Phase, Project string }
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		if event.Project != token {
			t.Fatalf("cross-project environment: %+v", event)
		}
		phases = append(phases, event.Phase)
	}
	if !reflect.DeepEqual(phases, []string{"codegen", "build"}) {
		t.Fatalf("unexpected phases: %v", phases)
	}
}

func environment() []string { values := os.Environ(); sort.Strings(values); return values }
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
	write(t, destination, string(data))
}
func write(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
}

const fixtureProto = `syntax = "proto3";
package envprobe.v1;
import "holons/v1/manifest.proto";
option (holons.v1.manifest) = {
  identity: { schema: "holon/v1" uuid: "4455c9e6-bd91-4bdb-bc5a-a211a4c7c58b" given_name: "Environment" family_name: "Probe" status: "draft" born: "2026-10-13" version: "0.0.1" }
  lang: "go"
  kind: "composite"
  platforms: ["windows"]
  build: {
    runner: "recipe"
    codegen: { languages: ["envprobe"] }
    defaults: { target: "windows" mode: "debug" }
    members: { id: "app" path: "app" type: "component" }
    targets: { key: "windows" value: { steps: { exec: { cwd: "." argv: ["env-probe.exe", "build"] } } } }
  }
  requires: { commands: ["env-probe.exe"] }
  artifacts: { primary: ".op/build/probe.txt" }
};
`
