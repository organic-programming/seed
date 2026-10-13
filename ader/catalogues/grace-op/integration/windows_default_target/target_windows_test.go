//go:build e2e && windows

package windows_default_target_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	sdkgrpc "github.com/organic-programming/go-holons/pkg/grpcclient"
	"github.com/organic-programming/go-holons/pkg/identity"
	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
)

// The fixture is this already-built native test executable. Its early child
// entry point avoids an additional compiler or a platform shell launcher.
func TestMain(m *testing.M) {
	if os.Getenv("ADER_DEFAULT_TARGET_FIXTURE_CHILD") == "1" {
		selected, err := os.ReadFile(filepath.Join(filepath.Dir(os.Args[0]), "selected.txt"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		fmt.Print("fixture:" + strings.TrimSpace(string(selected)))
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type selection struct {
	Name      string `json:"name"`
	Target    string `json:"build_target"`
	Mode      string `json:"build_mode"`
	RunOutput string `json:"run_output"`
}

func TestWindowsDefaultTarget_CLI(t *testing.T) { proveFacet(t, "CLI") }
func TestWindowsDefaultTarget_API(t *testing.T) { proveFacet(t, "API") }
func TestWindowsDefaultTarget_RPC(t *testing.T) { proveFacet(t, "RPC") }

func proveFacet(t *testing.T, facet string) {
	op := os.Getenv("ADER_DEFAULT_TARGET_OP")
	results := os.Getenv("ADER_DEFAULT_TARGET_RESULTS")
	if op == "" || results == "" {
		t.Fatal("run through the windows-default-target Ader suite")
	}
	var got []selection
	for _, tc := range []struct{ name, target, mode, wantTarget, wantMode string }{
		{"implicit", "", "", "windows", "debug"},
		{"explicit-macos", "macos", "", "macos", "debug"},
		{"explicit-mode", "", "release", "windows", "release"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workspace := fixture(t)
			request := &opv1.LifecycleRequest{Target: "target-probe", Build: &opv1.BuildOptions{Target: tc.target, Mode: tc.mode}}
			var report *opv1.LifecycleReport
			var output string
			switch facet {
			case "CLI":
				args := []string{"--format", "json", "--root", workspace, "build", "target-probe"}
				args = appendOptions(args, tc.target, tc.mode)
				var snapshot selection
				if err := json.Unmarshal(command(t, op, workspace, args...), &snapshot); err != nil {
					t.Fatal(err)
				}
				report = &opv1.LifecycleReport{BuildTarget: snapshot.Target, BuildMode: snapshot.Mode}
				// Preserve the exact user ordering and prove --clean rebuilds the
				// synthetic composite instead of launching its previous artifact.
				stale := filepath.Join(workspace, "target-probe", ".op", "stale.txt")
				write(t, stale, "stale")
				args = appendOptions([]string{"--root", workspace, "run", "target-probe", "--clean"}, tc.target, tc.mode)
				output = string(command(t, op, workspace, args...))
				if _, err := os.Stat(stale); !os.IsNotExist(err) {
					t.Fatalf("--clean did not remove stale artifact marker: %v", err)
				}
			case "API":
				response, err := api.Build(request)
				if err != nil {
					t.Fatal(err)
				}
				report = response.GetReport()
				run, err := api.Run(&opv1.RunRequest{Holon: "target-probe", Target: tc.target, Mode: tc.mode, NoBuild: true})
				if err != nil {
					t.Fatal(err)
				}
				if run.GetExitCode() != 0 {
					t.Fatalf("API run exit %d: %s", run.GetExitCode(), run.GetStderr())
				}
				output = run.GetStdout()
			case "RPC":
				client, cleanup := stdioClient(t, workspace, op)
				defer cleanup()
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				response, err := client.Build(ctx, request)
				if err != nil {
					t.Fatal(err)
				}
				report = response.GetReport()
				run, err := client.Run(ctx, &opv1.RunRequest{Holon: "target-probe", Target: tc.target, Mode: tc.mode, NoBuild: true})
				if err != nil {
					t.Fatal(err)
				}
				if run.GetExitCode() != 0 {
					t.Fatalf("RPC run exit %d: %s", run.GetExitCode(), run.GetStderr())
				}
				output = run.GetStdout()
			}
			result := selection{tc.name, report.GetBuildTarget(), report.GetBuildMode(), strings.TrimSpace(output)}
			want := selection{tc.name, tc.wantTarget, tc.wantMode, "fixture:" + tc.wantTarget + "/" + tc.wantMode}
			if result != want {
				t.Fatalf("%s selection = %+v, want %+v", facet, result, want)
			}
			got = append(got, result)
			t.Logf("%s: %+v", facet, result)
		})
	}
	if t.Failed() {
		return
	}
	data, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(results, facet+".json"), string(data))
	if facet == "RPC" {
		for _, earlier := range []string{"CLI", "API"} {
			data, err := os.ReadFile(filepath.Join(results, earlier+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var previous []selection
			if err := json.Unmarshal(data, &previous); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, previous) {
				t.Fatalf("%s and RPC differ: %+v / %+v", earlier, previous, got)
			}
		}
		t.Log("CLI, Code API and native stdio RPC selections and launched output match")
	}
}

func stdioClient(t *testing.T, workspace, op string) (opv1.OPServiceClient, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	cmd := exec.Command(op, "serve", "--listen", "stdio://")
	cmd.Dir, cmd.Env = workspace, os.Environ()
	conn, started, err := sdkgrpc.DialStdioCommand(ctx, cmd)
	if err != nil {
		cancel()
		t.Fatalf("start native stdio RPC: %v", err)
	}
	return opv1.NewOPServiceClient(conn), func() {
		_ = conn.Close()
		if started != nil && started.Process != nil {
			_ = started.Process.Kill()
			_ = started.Wait()
		}
		cancel()
	}
}

func fixture(t *testing.T) string {
	t.Helper()
	workspace := t.TempDir()
	// Use the same canonical source schema as OP's isolated build. The SDK's
	// generated descriptor fallback is not a source workspace schema.
	if err := os.CopyFS(filepath.Join(workspace, "_protos"), os.DirFS(filepath.Join(os.Getenv("ADER_REPO_ROOT"), "holons", "grace-op", "_protos"))); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(workspace, "target-probe")
	write(t, filepath.Join(dir, "api", "v1", "holon.proto"), fixtureProto)
	if _, err := identity.ResolveProtoFile(filepath.Join(dir, "api", "v1", "holon.proto")); err != nil {
		t.Fatalf("fixture manifest: %v", err)
	}
	write(t, filepath.Join(dir, "app", "build.ps1"), `param([string]$Selected)
$ErrorActionPreference = 'Stop'
$out = Join-Path $PSScriptRoot '../.op/build'
New-Item -ItemType Directory -Path $out -Force | Out-Null
Copy-Item -LiteralPath $env:ADER_DEFAULT_TARGET_FIXTURE_BINARY -Destination (Join-Path $out 'probe.exe')
[IO.File]::WriteAllText((Join-Path $out 'selected.txt'), "$Selected/$env:OP_BUILD_MODE")
`)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADER_DEFAULT_TARGET_FIXTURE_BINARY", executable)
	t.Setenv("ADER_DEFAULT_TARGET_FIXTURE_CHILD", "1")
	t.Setenv("OPROOT", workspace)
	t.Setenv("OPPATH", filepath.Join(workspace, "op-home"))
	t.Setenv("OPBIN", filepath.Join(workspace, "op-home", "bin"))
	t.Setenv("OP_RUN_DIR", filepath.Join(workspace, "run"))
	t.Setenv("GRACE_OP_SHARED_CACHE_DIR", filepath.Join(workspace, "package-cache"))
	t.Chdir(workspace)
	return workspace
}

func command(t *testing.T, op, cwd string, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, op, args...)
	cmd.Dir = cwd
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("OP %v: %v\n%s\n%s", args, err, &stdout, &stderr)
	}
	return stdout.Bytes()
}

func appendOptions(args []string, target, mode string) []string {
	if target != "" {
		args = append(args, "--target", target)
	}
	if mode != "" {
		args = append(args, "--mode", mode)
	}
	return args
}

func write(t *testing.T, name, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

const fixtureProto = `syntax = "proto3";
package targetprobe.v1;
import "holons/v1/manifest.proto";
option (holons.v1.manifest) = {
  identity: { schema: "holon/v1" uuid: "b9a4e833-21be-4c10-b8f2-5b90bf8356aa" given_name: "Target" family_name: "Probe" status: "draft" born: "2026-10-13" version: "0.0.1" aliases: ["target-probe"] }
  lang: "go"
  kind: "composite"
  platforms: ["macos", "windows"]
  build: {
    runner: "recipe"
    defaults: { mode: "debug" }
    members: { id: "app" path: "app" type: "component" }
    targets: { key: "macos" value: { steps: { exec: { cwd: "app" argv: ["pwsh", "-NoProfile", "-NonInteractive", "-File", "build.ps1", "macos"] } } } }
    targets: { key: "windows" value: { steps: { exec: { cwd: "app" argv: ["pwsh", "-NoProfile", "-NonInteractive", "-File", "build.ps1", "windows"] } } } }
  }
  artifacts: { primary: ".op/build/probe.exe" }
};
`
