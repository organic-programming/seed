package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestDiscoverTargetsIgnoresNestedHolons(t *testing.T) {
	root := t.TempDir()

	writeManifest(t, root, "recipes/assemblies/gudule-greeting-flutter-go/holon.yaml", "kind: composite\ntransport: tcp\n")
	writeManifest(t, root, "recipes/composition/direct-call/charon-direct-go-go/holon.yaml", "kind: composite\n")
	writeManifest(t, root, "recipes/composition/direct-call/charon-direct-go-go/orchestrator/holon.yaml", "kind: native\n")
	writeManifest(t, root, "recipes/composition/workers/charon-worker-compute/holon.yaml", "kind: native\n")

	targets, err := discoverTargets(root)
	if err != nil {
		t.Fatalf("discoverTargets: %v", err)
	}

	got := []string{
		string(targets[0].Kind) + ":" + targets[0].Name,
		string(targets[1].Kind) + ":" + targets[1].Name,
	}
	want := []string{
		"assembly:gudule-greeting-flutter-go",
		"composition:charon-direct-go-go",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("targets = %v, want %v", got, want)
	}
}

func TestExecuteTargetsRespectsFilterSkipAndDryRun(t *testing.T) {
	cfg := MatrixConfig{
		RepoRoot:   t.TempDir(),
		FilterExpr: "direct",
		Filter:     regexp.MustCompile("direct"),
		SkipExpr:   "node",
		Skip:       regexp.MustCompile("node"),
		Timeout:    time.Second,
		Format:     "json",
		DryRun:     true,
	}

	targets := []Target{
		{Name: "charon-direct-go-go", Path: "recipes/composition/direct-call/charon-direct-go-go", Kind: compositionTarget},
		{Name: "charon-direct-node-go", Path: "recipes/composition/direct-call/charon-direct-node-go", Kind: compositionTarget},
		{Name: "charon-pipeline-go-go", Path: "recipes/composition/pipeline/charon-pipeline-go-go", Kind: compositionTarget},
	}

	runner := &fakeRunner{}
	report := executeTargets(context.Background(), cfg, targets, runner, RuntimeEnv{
		GOOS:     "darwin",
		LookPath: func(string) (string, error) { return "/usr/bin/tool", nil },
	})

	if runner.calls != 0 {
		t.Fatalf("runner calls = %d, want 0", runner.calls)
	}
	if report.Summary.Selected != 2 {
		t.Fatalf("selected = %d, want 2", report.Summary.Selected)
	}
	if len(report.Results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(report.Results))
	}
	if report.Results[0].Status != statusDryRun {
		t.Fatalf("first status = %s, want %s", report.Results[0].Status, statusDryRun)
	}
	if report.Results[1].Status != statusSkipped {
		t.Fatalf("second status = %s, want %s", report.Results[1].Status, statusSkipped)
	}
	if !strings.Contains(report.Results[1].Reason, "--skip") {
		t.Fatalf("skip reason = %q, want matched --skip", report.Results[1].Reason)
	}

	var decoded MatrixReport
	if err := json.Unmarshal([]byte(renderReport(report, "json")), &decoded); err != nil {
		t.Fatalf("renderReport(json): %v", err)
	}
}

func TestSkipReasonDetectsMissingToolchain(t *testing.T) {
	target := Target{
		Name:             "gudule-greeting-dotnet-go",
		Path:             "recipes/assemblies/gudule-greeting-dotnet-go",
		Kind:             assemblyTarget,
		Platforms:        []string{"macos"},
		RequiresCommands: []string{"go", "dotnet"},
	}

	reason := skipReason(target, RuntimeEnv{
		GOOS: "darwin",
		LookPath: func(name string) (string, error) {
			if name == "go" {
				return "/usr/bin/go", nil
			}
			return "", errors.New("missing")
		},
	})

	if reason != "missing commands: dotnet" {
		t.Fatalf("reason = %q, want missing commands: dotnet", reason)
	}
}

func TestExecuteTargetsHandlesTimeouts(t *testing.T) {
	cfg := MatrixConfig{
		RepoRoot: t.TempDir(),
		Timeout:  10 * time.Millisecond,
	}
	runner := &fakeRunner{
		run: func(ctx context.Context, _ string, _ string, args ...string) commandResult {
			if len(args) >= 3 && args[2] == "build" {
				return commandResult{ExitCode: 0}
			}
			<-ctx.Done()
			return commandResult{Err: ctx.Err(), TimedOut: true, ExitCode: -1}
		},
	}

	report := executeTargets(context.Background(), cfg, []Target{
		{Name: "gudule-greeting-go-web", Path: "recipes/assemblies/gudule-greeting-go-web", Kind: assemblyTarget},
		{Name: "charon-direct-go-go", Path: "recipes/composition/direct-call/charon-direct-go-go", Kind: compositionTarget},
	}, runner, RuntimeEnv{
		GOOS:     "darwin",
		LookPath: func(string) (string, error) { return "/usr/bin/tool", nil },
	})

	if got := report.Results[0].Status; got != statusSmokePassed {
		t.Fatalf("assembly status = %s, want %s", got, statusSmokePassed)
	}
	if got := report.Results[1].Status; got != statusTimedOut {
		t.Fatalf("composition status = %s, want %s", got, statusTimedOut)
	}
}

func TestActualRepoInventoryAndTransport(t *testing.T) {
	root, err := locateRepoRootFrom(".")
	if err != nil {
		t.Fatalf("locateRepoRootFrom: %v", err)
	}

	targets, err := discoverTargets(root)
	if err != nil {
		t.Fatalf("discoverTargets: %v", err)
	}

	var assemblyNames []string
	var compositionCount int
	for _, target := range targets {
		switch target.Kind {
		case assemblyTarget:
			assemblyNames = append(assemblyNames, target.Name)
			wantTransport := "tcp"
			if strings.Contains(target.Name, "swiftui-") {
				wantTransport = "stdio"
			}
			if target.Transport != wantTransport {
				t.Fatalf("%s transport = %q, want %q", target.Name, target.Transport, wantTransport)
			}
		case compositionTarget:
			compositionCount++
		}
	}

	if len(assemblyNames) != 48 {
		t.Fatalf("assembly count = %d, want 48", len(assemblyNames))
	}
	if compositionCount != 33 {
		t.Fatalf("composition count = %d, want 33", compositionCount)
	}

	inventoryNames := loadInventoryAssemblyNames(t, filepath.Join(root, "design", "grace-op", "v0.4", "recipes.yaml"))
	sort.Strings(assemblyNames)
	sort.Strings(inventoryNames)
	if !reflect.DeepEqual(assemblyNames, inventoryNames) {
		t.Fatalf("assembly names do not match recipes.yaml")
	}
}

type fakeRunner struct {
	calls int
	run   func(ctx context.Context, dir string, name string, args ...string) commandResult
}

func (f *fakeRunner) Run(ctx context.Context, dir string, name string, args ...string) commandResult {
	f.calls++
	if f.run != nil {
		return f.run(ctx, dir, name, args...)
	}
	return commandResult{}
}

func writeManifest(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", path, err)
	}
	content := "schema: holon/v0\n" + body + "build:\n  runner: recipe\nartifacts:\n  primary: build/out\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func loadInventoryAssemblyNames(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}

	var inventory struct {
		Assemblies []struct {
			Name string `yaml:"name"`
		} `yaml:"assemblies"`
	}
	if err := yaml.Unmarshal(data, &inventory); err != nil {
		t.Fatalf("yaml.Unmarshal(%s): %v", path, err)
	}

	names := make([]string, 0, len(inventory.Assemblies))
	for _, assembly := range inventory.Assemblies {
		names = append(names, assembly.Name)
	}
	return names
}
