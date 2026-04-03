package build_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

const rootPath = "../../../../.."

type lifecycleSnapshot struct {
	Operation   string              `json:"operation"`
	Target      string              `json:"target"`
	Holon       string              `json:"holon"`
	Dir         string              `json:"dir"`
	Manifest    string              `json:"manifest"`
	Runner      string              `json:"runner,omitempty"`
	Kind        string              `json:"kind,omitempty"`
	Binary      string              `json:"binary,omitempty"`
	BuildTarget string              `json:"build_target,omitempty"`
	BuildMode   string              `json:"build_mode,omitempty"`
	Artifact    string              `json:"artifact,omitempty"`
	Commands    []string            `json:"commands,omitempty"`
	Notes       []string            `json:"notes,omitempty"`
	Children    []lifecycleSnapshot `json:"children,omitempty"`
}

type installSnapshot struct {
	Operation   string   `json:"operation"`
	Target      string   `json:"target"`
	Holon       string   `json:"holon"`
	Dir         string   `json:"dir"`
	Manifest    string   `json:"manifest"`
	Binary      string   `json:"binary,omitempty"`
	BuildTarget string   `json:"build_target,omitempty"`
	BuildMode   string   `json:"build_mode,omitempty"`
	Artifact    string   `json:"artifact,omitempty"`
	Installed   string   `json:"installed,omitempty"`
	Notes       []string `json:"notes,omitempty"`
}

type buildStep struct {
	Kind      string             `json:"kind"`
	Lifecycle *lifecycleSnapshot `json:"lifecycle,omitempty"`
	Install   *installSnapshot   `json:"install,omitempty"`
}

type buildOutcome struct {
	Steps []buildStep `json:"steps"`
}

type buildScenario struct {
	Name         string
	Target       string
	Build        *opv1.BuildOptions
	InstallAfter bool
	CleanFirst   bool
	Prepare      func(t *testing.T, env *buildTestEnv)
	AssertAPI    func(t *testing.T, outcome buildOutcome, env buildTestEnv)
	AssertAll    func(t *testing.T, apiOutcome buildOutcome, apiEnv buildTestEnv, cliOutcome buildOutcome, cliEnv buildTestEnv, rpcOutcome buildOutcome, rpcEnv buildTestEnv)
}

type buildTestEnv struct {
	AbsRoot string
	EnvVars []string
	OpBin   string
}

func buildScenarios() []buildScenario {
	return []buildScenario{
		{
			Name:   "dry-run-basic",
			Target: "gabriel-greeting-go",
			Build:  &opv1.BuildOptions{DryRun: true},
		},
		{
			Name:   "dry-run-no-sign",
			Target: "gabriel-greeting-app-swiftui",
			Build:  &opv1.BuildOptions{DryRun: true, NoSign: true},
		},
		{
			Name:   "source-preferred-over-opbin",
			Target: "gabriel-greeting-go",
			Build:  &opv1.BuildOptions{DryRun: true},
			Prepare: func(t *testing.T, env *buildTestEnv) {
				runOP(t, env.OpBin, env.EnvVars, "--root", env.AbsRoot, "build", "gabriel-greeting-go", "--install")
			},
			AssertAPI: func(t *testing.T, outcome buildOutcome, _ buildTestEnv) {
				assertSourceOutcome(t, "API", outcome, "gabriel-greeting-go")
			},
			AssertAll: func(t *testing.T, apiOutcome buildOutcome, _ buildTestEnv, cliOutcome buildOutcome, _ buildTestEnv, rpcOutcome buildOutcome, _ buildTestEnv) {
				assertSourceOutcome(t, "API", apiOutcome, "gabriel-greeting-go")
				assertSourceOutcome(t, "CLI", cliOutcome, "gabriel-greeting-go")
				assertSourceOutcome(t, "RPC", rpcOutcome, "gabriel-greeting-go")
				assertOutcomeEqual(t, "CLI", cliOutcome, "API", apiOutcome)
				assertOutcomeEqual(t, "RPC", rpcOutcome, "API", apiOutcome)
			},
		},
		{
			Name:   "source-preferred-over-path",
			Target: "gabriel-greeting-go",
			Build:  &opv1.BuildOptions{DryRun: true},
			Prepare: func(t *testing.T, env *buildTestEnv) {
				runOP(t, env.OpBin, env.EnvVars, "--root", env.AbsRoot, "build", "gabriel-greeting-go")

				sourceBinary := buildArtifactPath("gabriel-greeting-go")
				pathDir := t.TempDir()
				pathBinary := filepath.Join(pathDir, "gabriel-greeting-go")
				copyExecutable(t, sourceBinary, pathBinary)

				integration.TeardownHolons(t, rootPath)

				pathValue := pathDir
				if existing := envValue(env.EnvVars, "PATH"); existing != "" {
					pathValue += string(os.PathListSeparator) + existing
				}
				env.EnvVars = withEnvEntry(env.EnvVars, "PATH", pathValue)
			},
			AssertAPI: func(t *testing.T, outcome buildOutcome, _ buildTestEnv) {
				assertSourceOutcome(t, "API", outcome, "gabriel-greeting-go")
			},
			AssertAll: func(t *testing.T, apiOutcome buildOutcome, _ buildTestEnv, cliOutcome buildOutcome, _ buildTestEnv, rpcOutcome buildOutcome, _ buildTestEnv) {
				assertSourceOutcome(t, "API", apiOutcome, "gabriel-greeting-go")
				assertSourceOutcome(t, "CLI", cliOutcome, "gabriel-greeting-go")
				assertSourceOutcome(t, "RPC", rpcOutcome, "gabriel-greeting-go")
				assertOutcomeEqual(t, "CLI", cliOutcome, "API", apiOutcome)
				assertOutcomeEqual(t, "RPC", rpcOutcome, "API", apiOutcome)
			},
		},
		{
			Name:         "install-sequence",
			Target:       "gabriel-greeting-go",
			InstallAfter: true,
		},
		{
			Name:       "clean-sequence",
			Target:     "gabriel-greeting-go",
			CleanFirst: true,
			Prepare: func(t *testing.T, env *buildTestEnv) {
				runOP(t, env.OpBin, env.EnvVars, "--root", env.AbsRoot, "build", "gabriel-greeting-go")
				markerPath := filepath.Join(holonOPDir("gabriel-greeting-go"), "stale-marker.txt")
				if err := os.WriteFile(markerPath, []byte("stale"), 0o644); err != nil {
					t.Fatalf("write stale marker %s: %v", markerPath, err)
				}
			},
			AssertAPI: func(t *testing.T, outcome buildOutcome, env buildTestEnv) {
				assertCleanSequenceContract(t, "API", outcome, env, "gabriel-greeting-go")
			},
			AssertAll: func(t *testing.T, apiOutcome buildOutcome, apiEnv buildTestEnv, cliOutcome buildOutcome, cliEnv buildTestEnv, rpcOutcome buildOutcome, rpcEnv buildTestEnv) {
				assertCleanSequenceContract(t, "API", apiOutcome, apiEnv, "gabriel-greeting-go")
				assertCleanSequenceContract(t, "RPC", rpcOutcome, rpcEnv, "gabriel-greeting-go")
				assertMarkerRemoved(t, cliEnv, "gabriel-greeting-go")
				assertArtifactExists(t, cliEnv, "gabriel-greeting-go")
				assertOutcomeEqual(t, "RPC", rpcOutcome, "API", apiOutcome)

				apiBuildStep := apiOutcome.Steps[len(apiOutcome.Steps)-1]
				if len(cliOutcome.Steps) != 1 {
					t.Fatalf("CLI clean-sequence steps = %d, want 1 build step", len(cliOutcome.Steps))
				}
				if apiBuildStep.Kind != "build" || cliOutcome.Steps[0].Kind != "build" {
					t.Fatalf("clean-sequence build steps malformed.\nAPI: %s\nCLI: %s", prettyOutcome(apiOutcome), prettyOutcome(cliOutcome))
				}
				assertLifecycleEqual(t, "CLI build", cliOutcome.Steps[0].Lifecycle, "API build", apiBuildStep.Lifecycle)
			},
		},
	}
}

func (s buildScenario) assertAPIContract(t *testing.T, outcome buildOutcome, env buildTestEnv) {
	t.Helper()

	switch {
	case s.CleanFirst:
		assertCleanSequenceContract(t, "API", outcome, env, s.Target)
	case s.InstallAfter:
		if len(outcome.Steps) != 2 {
			t.Fatalf("API install-sequence steps = %d, want 2\n%s", len(outcome.Steps), prettyOutcome(outcome))
		}
		if outcome.Steps[0].Kind != "build" || outcome.Steps[1].Kind != "install" {
			t.Fatalf("API install-sequence kinds = %v, want [build install]\n%s", stepKinds(outcome), prettyOutcome(outcome))
		}
	default:
		if len(outcome.Steps) != 1 || outcome.Steps[0].Kind != "build" {
			t.Fatalf("API build steps = %v, want [build]\n%s", stepKinds(outcome), prettyOutcome(outcome))
		}
	}

	if s.AssertAPI != nil {
		s.AssertAPI(t, outcome, env)
	}
}

func (s buildScenario) assertSymmetry(t *testing.T, apiOutcome buildOutcome, apiEnv buildTestEnv, cliOutcome buildOutcome, cliEnv buildTestEnv, rpcOutcome buildOutcome, rpcEnv buildTestEnv) {
	t.Helper()

	if s.AssertAll != nil {
		s.AssertAll(t, apiOutcome, apiEnv, cliOutcome, cliEnv, rpcOutcome, rpcEnv)
		return
	}
	assertOutcomeEqual(t, "CLI", cliOutcome, "API", apiOutcome)
	assertOutcomeEqual(t, "RPC", rpcOutcome, "API", apiOutcome)
}

func newBuildTestEnv(t *testing.T) buildTestEnv {
	t.Helper()

	integration.TeardownHolons(t, rootPath)
	envVars, opBin := integration.SetupIsolatedOP(t, rootPath)
	return buildTestEnv{
		AbsRoot: absoluteRootPath(t),
		EnvVars: envVars,
		OpBin:   opBin,
	}
}

func runAPIScenario(t *testing.T, scenario buildScenario) (buildOutcome, buildTestEnv) {
	t.Helper()

	env := newBuildTestEnv(t)
	if scenario.Prepare != nil {
		scenario.Prepare(t, &env)
	}

	var outcome buildOutcome
	withBuildEnv(t, env, func() {
		outcome = buildOutcomeForAPI(t, scenario)
	})
	return outcome, env
}

func runRPCScenario(t *testing.T, scenario buildScenario) (buildOutcome, buildTestEnv) {
	t.Helper()

	env := newBuildTestEnv(t)
	if scenario.Prepare != nil {
		scenario.Prepare(t, &env)
	}

	client, cleanup := integration.SetupStdioOPClient(t, rootPath, env.OpBin, env.EnvVars)
	defer cleanup()

	return buildOutcomeForRPC(t, client, scenario), env
}

func runCLIScenario(t *testing.T, scenario buildScenario) (buildOutcome, buildTestEnv) {
	t.Helper()

	env := newBuildTestEnv(t)
	if scenario.Prepare != nil {
		scenario.Prepare(t, &env)
	}

	switch {
	case scenario.InstallAfter:
		return runCLIBuildInstallScenario(t, env, scenario), env
	default:
		return runCLIBuildScenario(t, env, scenario), env
	}
}

func buildOutcomeForAPI(t *testing.T, scenario buildScenario) buildOutcome {
	t.Helper()

	var outcome buildOutcome
	if scenario.CleanFirst {
		resp, err := api.Clean(&opv1.LifecycleRequest{Target: scenario.Target})
		if err != nil {
			t.Fatalf("api.Clean(%s): %v", scenario.Target, err)
		}
		outcome.Steps = append(outcome.Steps, buildStep{Kind: "clean", Lifecycle: rpcSnapshotFromProto(resp.GetReport())})
	}

	resp, err := api.Build(&opv1.LifecycleRequest{
		Target: scenario.Target,
		Build:  cloneBuildOptions(scenario.Build),
	})
	if err != nil {
		t.Fatalf("api.Build(%s): %v", scenario.Target, err)
	}
	outcome.Steps = append(outcome.Steps, buildStep{Kind: "build", Lifecycle: rpcSnapshotFromProto(resp.GetReport())})

	if scenario.InstallAfter {
		resp, err := api.Install(&opv1.InstallRequest{Target: scenario.Target})
		if err != nil {
			t.Fatalf("api.Install(%s): %v", scenario.Target, err)
		}
		outcome.Steps = append(outcome.Steps, buildStep{Kind: "install", Install: rpcInstallSnapshotFromProto(resp.GetReport())})
	}

	return outcome
}

func buildOutcomeForRPC(t *testing.T, client opv1.OPServiceClient, scenario buildScenario) buildOutcome {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var outcome buildOutcome
	if scenario.CleanFirst {
		resp, err := client.Clean(ctx, &opv1.LifecycleRequest{Target: scenario.Target})
		if err != nil {
			t.Fatalf("Build Clean RPC(%s): %v", scenario.Target, err)
		}
		outcome.Steps = append(outcome.Steps, buildStep{Kind: "clean", Lifecycle: rpcSnapshotFromProto(resp.GetReport())})
	}

	resp, err := client.Build(ctx, &opv1.LifecycleRequest{
		Target: scenario.Target,
		Build:  cloneBuildOptions(scenario.Build),
	})
	if err != nil {
		t.Fatalf("Build RPC(%s): %v", scenario.Target, err)
	}
	outcome.Steps = append(outcome.Steps, buildStep{Kind: "build", Lifecycle: rpcSnapshotFromProto(resp.GetReport())})

	if scenario.InstallAfter {
		resp, err := client.Install(ctx, &opv1.InstallRequest{Target: scenario.Target})
		if err != nil {
			t.Fatalf("Install RPC(%s): %v", scenario.Target, err)
		}
		outcome.Steps = append(outcome.Steps, buildStep{Kind: "install", Install: rpcInstallSnapshotFromProto(resp.GetReport())})
	}

	return outcome
}

func runCLIBuildScenario(t *testing.T, env buildTestEnv, scenario buildScenario) buildOutcome {
	t.Helper()

	args := []string{"--format", "json", "--root", env.AbsRoot, "build"}
	args = append(args, buildFlagsFromScenario(scenario)...)
	if scenario.CleanFirst {
		args = append(args, "--clean")
	}
	args = append(args, scenario.Target)
	out := runOP(t, env.OpBin, env.EnvVars, args...)
	return buildOutcome{
		Steps: []buildStep{{
			Kind:      "build",
			Lifecycle: cliLifecycleSnapshotFromJSON(t, out),
		}},
	}
}

func runCLIBuildInstallScenario(t *testing.T, env buildTestEnv, scenario buildScenario) buildOutcome {
	t.Helper()

	args := []string{"--format", "json", "--root", env.AbsRoot, "build"}
	args = append(args, buildFlagsFromScenario(scenario)...)
	args = append(args, "--install", scenario.Target)
	out := runOP(t, env.OpBin, env.EnvVars, args...)

	var payload struct {
		Build   lifecycleSnapshot `json:"build"`
		Install installSnapshot   `json:"install"`
		Error   string            `json:"error,omitempty"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("parse CLI build/install JSON: %v\nOutput: %s", err, string(out))
	}
	if payload.Error != "" {
		t.Fatalf("CLI build/install returned error payload: %s", payload.Error)
	}
	return buildOutcome{
		Steps: []buildStep{
			{Kind: "build", Lifecycle: &payload.Build},
			{Kind: "install", Install: normalizeInstallSnapshot(&payload.Install)},
		},
	}
}

func buildFlagsFromScenario(scenario buildScenario) []string {
	args := make([]string, 0, 6)
	if scenario.Build == nil {
		return args
	}
	if scenario.Build.GetTarget() != "" {
		args = append(args, "--target", scenario.Build.GetTarget())
	}
	if scenario.Build.GetMode() != "" {
		args = append(args, "--mode", scenario.Build.GetMode())
	}
	if scenario.Build.GetDryRun() {
		args = append(args, "--dry-run")
	}
	if scenario.Build.GetNoSign() {
		args = append(args, "--no-sign")
	}
	return args
}

func withBuildEnv(t *testing.T, env buildTestEnv, fn func()) {
	t.Helper()

	for _, entry := range env.EnvVars {
		parts := strings.SplitN(entry, "=", 2)
		key := parts[0]
		value := ""
		if len(parts) == 2 {
			value = parts[1]
		}
		t.Setenv(key, value)
	}
	t.Setenv("OPROOT", env.AbsRoot)

	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(env.AbsRoot); err != nil {
		t.Fatalf("Chdir(%s): %v", env.AbsRoot, err)
	}
	defer func() {
		_ = os.Chdir(original)
	}()

	fn()
}

func cliLifecycleSnapshotFromJSON(t *testing.T, out []byte) *lifecycleSnapshot {
	t.Helper()

	var snapshot lifecycleSnapshot
	if err := json.Unmarshal(out, &snapshot); err != nil {
		t.Fatalf("parse CLI build JSON: %v\nOutput: %s", err, string(out))
	}
	return &snapshot
}

func rpcSnapshotFromProto(report *opv1.LifecycleReport) *lifecycleSnapshot {
	if report == nil {
		return nil
	}

	snapshot := &lifecycleSnapshot{
		Operation:   report.GetOperation(),
		Target:      report.GetTarget(),
		Holon:       report.GetHolon(),
		Dir:         report.GetDir(),
		Manifest:    report.GetManifest(),
		Runner:      report.GetRunner(),
		Kind:        report.GetKind(),
		Binary:      report.GetBinary(),
		BuildTarget: report.GetBuildTarget(),
		BuildMode:   report.GetBuildMode(),
		Artifact:    report.GetArtifact(),
		Commands:    append([]string(nil), report.GetCommands()...),
		Notes:       append([]string(nil), report.GetNotes()...),
	}
	for _, child := range report.GetChildren() {
		snapshot.Children = append(snapshot.Children, *rpcSnapshotFromProto(child))
	}
	return snapshot
}

func rpcInstallSnapshotFromProto(report *opv1.InstallReport) *installSnapshot {
	if report == nil {
		return nil
	}

	return normalizeInstallSnapshot(&installSnapshot{
		Operation:   report.GetOperation(),
		Target:      report.GetTarget(),
		Holon:       report.GetHolon(),
		Dir:         report.GetDir(),
		Manifest:    report.GetManifest(),
		Binary:      report.GetBinary(),
		BuildTarget: report.GetBuildTarget(),
		BuildMode:   report.GetBuildMode(),
		Artifact:    report.GetArtifact(),
		Installed:   report.GetInstalled(),
		Notes:       append([]string(nil), report.GetNotes()...),
	})
}

func runOP(t *testing.T, opBin string, envVars []string, args ...string) []byte {
	t.Helper()

	cmd := exec.Command(opBin, args...)
	cmd.Dir = absoluteRootPath(t)
	cmd.Env = envVars

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("op %s failed: %v\nOutput: %s", strings.Join(args, " "), err, string(out))
	}
	return out
}

func cloneBuildOptions(build *opv1.BuildOptions) *opv1.BuildOptions {
	if build == nil {
		return nil
	}
	return &opv1.BuildOptions{
		Target: build.GetTarget(),
		Mode:   build.GetMode(),
		DryRun: build.GetDryRun(),
		NoSign: build.GetNoSign(),
	}
}

func assertSourceOutcome(t *testing.T, label string, outcome buildOutcome, holon string) {
	t.Helper()

	if len(outcome.Steps) == 0 || outcome.Steps[0].Lifecycle == nil {
		t.Fatalf("%s outcome missing lifecycle step:\n%s", label, prettyOutcome(outcome))
	}

	wantDir := filepath.ToSlash(filepath.Join("examples", "hello-world", holon))
	wantManifest := filepath.ToSlash(filepath.Join("examples", "hello-world", holon, "api", "v1", "holon.proto"))
	got := outcome.Steps[0].Lifecycle
	if got.Dir != wantDir {
		t.Fatalf("%s dir = %q, want %q", label, got.Dir, wantDir)
	}
	if got.Manifest != wantManifest {
		t.Fatalf("%s manifest = %q, want %q", label, got.Manifest, wantManifest)
	}
}

func assertCleanSequenceContract(t *testing.T, label string, outcome buildOutcome, env buildTestEnv, holon string) {
	t.Helper()

	if len(outcome.Steps) != 2 {
		t.Fatalf("%s clean-sequence steps = %d, want 2\n%s", label, len(outcome.Steps), prettyOutcome(outcome))
	}
	if outcome.Steps[0].Kind != "clean" || outcome.Steps[1].Kind != "build" {
		t.Fatalf("%s clean-sequence kinds = %v, want [clean build]\n%s", label, stepKinds(outcome), prettyOutcome(outcome))
	}
	assertMarkerRemoved(t, env, holon)
	assertArtifactExists(t, env, holon)
}

func assertMarkerRemoved(t *testing.T, _ buildTestEnv, holon string) {
	t.Helper()

	if _, err := os.Stat(filepath.Join(holonOPDir(holon), "stale-marker.txt")); !os.IsNotExist(err) {
		t.Fatalf("stale marker still exists for %s: %v", holon, err)
	}
}

func assertArtifactExists(t *testing.T, _ buildTestEnv, holon string) {
	t.Helper()

	if _, err := os.Stat(buildArtifactPath(holon)); err != nil {
		t.Fatalf("artifact missing for %s: %v", holon, err)
	}
}

func buildArtifactPath(holon string) string {
	return filepath.Join(
		rootPath,
		"examples",
		"hello-world",
		holon,
		".op",
		"build",
		holon+".holon",
		"bin",
		runtime.GOOS+"_"+runtime.GOARCH,
		holon,
	)
}

func holonOPDir(holon string) string {
	return filepath.Join(rootPath, "examples", "hello-world", holon, ".op")
}

func assertOutcomeEqual(t *testing.T, gotLabel string, got buildOutcome, wantLabel string, want buildOutcome) {
	t.Helper()

	if reflect.DeepEqual(got, want) {
		return
	}
	t.Fatalf("%s outcome != %s outcome.\n%s: %s\n%s: %s", gotLabel, wantLabel, gotLabel, prettyOutcome(got), wantLabel, prettyOutcome(want))
}

func assertLifecycleEqual(t *testing.T, gotLabel string, got *lifecycleSnapshot, wantLabel string, want *lifecycleSnapshot) {
	t.Helper()

	if reflect.DeepEqual(got, want) {
		return
	}
	gotData, _ := json.MarshalIndent(got, "", "  ")
	wantData, _ := json.MarshalIndent(want, "", "  ")
	t.Fatalf("%s != %s.\n%s: %s\n%s: %s", gotLabel, wantLabel, gotLabel, string(gotData), wantLabel, string(wantData))
}

func prettyOutcome(outcome buildOutcome) string {
	data, err := json.MarshalIndent(outcome, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(data)
}

func stepKinds(outcome buildOutcome) []string {
	kinds := make([]string, 0, len(outcome.Steps))
	for _, step := range outcome.Steps {
		kinds = append(kinds, step.Kind)
	}
	return kinds
}

func copyExecutable(t *testing.T, src, dst string) {
	t.Helper()

	sourceFile, err := os.Open(src)
	if err != nil {
		t.Fatalf("open source binary %s: %v", src, err)
	}
	defer sourceFile.Close()

	info, err := sourceFile.Stat()
	if err != nil {
		t.Fatalf("stat source binary %s: %v", src, err)
	}

	targetFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		t.Fatalf("create copied binary %s: %v", dst, err)
	}
	defer targetFile.Close()

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		t.Fatalf("copy %s to %s: %v", src, dst, err)
	}
}

func withEnvEntry(envVars []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(envVars)+1)
	for _, entry := range envVars {
		if strings.HasPrefix(entry, prefix) {
			continue
		}
		out = append(out, entry)
	}
	return append(out, prefix+value)
}

func envValue(envVars []string, key string) string {
	prefix := key + "="
	for _, entry := range envVars {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix)
		}
	}
	return ""
}

func normalizeInstallSnapshot(snapshot *installSnapshot) *installSnapshot {
	if snapshot == nil {
		return nil
	}

	normalized := *snapshot
	normalized.Installed = normalizeRuntimePath(normalized.Installed)
	if len(normalized.Notes) > 0 {
		normalized.Notes = make([]string, 0, len(snapshot.Notes))
		for _, note := range snapshot.Notes {
			normalized.Notes = append(normalized.Notes, normalizeInstallNote(note))
		}
	}
	return &normalized
}

func normalizeInstallNote(note string) string {
	const prefix = "installed into "
	if strings.HasPrefix(note, prefix) {
		return prefix + normalizeRuntimePath(strings.TrimPrefix(note, prefix))
	}
	return note
}

func normalizeRuntimePath(value string) string {
	if strings.TrimSpace(value) == "" {
		return value
	}

	slashed := filepath.ToSlash(value)
	if idx := strings.LastIndex(slashed, "/bin/"); idx >= 0 {
		return slashed[idx+1:]
	}
	return slashed
}

func absoluteRootPath(t *testing.T) string {
	t.Helper()

	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		t.Fatalf("resolve root path %s: %v", rootPath, err)
	}
	return absRoot
}
