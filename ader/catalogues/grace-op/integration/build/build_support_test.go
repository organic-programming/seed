package build_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

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

type buildTestEnv struct {
	AbsRoot string
	EnvVars []string
	OpBin   string
}

func newBuildTestEnv(t *testing.T) buildTestEnv {
	t.Helper()

	root := absoluteRootPath(t)
	integration.TeardownHolons(t, root)
	envVars, opBin := integration.SetupIsolatedOP(t, root)
	return buildTestEnv{
		AbsRoot: root,
		EnvVars: envVars,
		OpBin:   opBin,
	}
}

func absoluteRootPath(t *testing.T) string {
	t.Helper()
	return integration.DefaultWorkspaceDir(t)
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

func buildViaAPI(t *testing.T, env buildTestEnv, target string, build *opv1.BuildOptions) *lifecycleSnapshot {
	t.Helper()

	var snapshot *lifecycleSnapshot
	withBuildEnv(t, env, func() {
		resp, err := api.Build(&opv1.LifecycleRequest{
			Target: target,
			Build:  cloneBuildOptions(build),
		})
		if err != nil {
			t.Fatalf("api.Build(%s): %v", target, err)
		}
		snapshot = rpcLifecycleSnapshot(resp.GetReport())
	})
	return snapshot
}

func cleanViaAPI(t *testing.T, env buildTestEnv, target string) *lifecycleSnapshot {
	t.Helper()

	var snapshot *lifecycleSnapshot
	withBuildEnv(t, env, func() {
		resp, err := api.Clean(&opv1.LifecycleRequest{Target: target})
		if err != nil {
			t.Fatalf("api.Clean(%s): %v", target, err)
		}
		snapshot = rpcLifecycleSnapshot(resp.GetReport())
	})
	return snapshot
}

func installViaAPI(t *testing.T, env buildTestEnv, target string) *installSnapshot {
	t.Helper()

	var snapshot *installSnapshot
	withBuildEnv(t, env, func() {
		resp, err := api.Install(&opv1.InstallRequest{Target: target})
		if err != nil {
			t.Fatalf("api.Install(%s): %v", target, err)
		}
		snapshot = rpcInstallSnapshot(resp.GetReport())
	})
	return snapshot
}

func buildViaRPC(t *testing.T, env buildTestEnv, target string, build *opv1.BuildOptions) *lifecycleSnapshot {
	t.Helper()

	client, cleanup := integration.SetupStdioOPClient(t, env.AbsRoot, env.OpBin, env.EnvVars)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.Build(ctx, &opv1.LifecycleRequest{
		Target: target,
		Build:  cloneBuildOptions(build),
	})
	if err != nil {
		t.Fatalf("rpc Build(%s): %v", target, err)
	}
	return rpcLifecycleSnapshot(resp.GetReport())
}

func cleanViaRPC(t *testing.T, env buildTestEnv, target string) *lifecycleSnapshot {
	t.Helper()

	client, cleanup := integration.SetupStdioOPClient(t, env.AbsRoot, env.OpBin, env.EnvVars)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.Clean(ctx, &opv1.LifecycleRequest{Target: target})
	if err != nil {
		t.Fatalf("rpc Clean(%s): %v", target, err)
	}
	return rpcLifecycleSnapshot(resp.GetReport())
}

func installViaRPC(t *testing.T, env buildTestEnv, target string) *installSnapshot {
	t.Helper()

	client, cleanup := integration.SetupStdioOPClient(t, env.AbsRoot, env.OpBin, env.EnvVars)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.Install(ctx, &opv1.InstallRequest{Target: target})
	if err != nil {
		t.Fatalf("rpc Install(%s): %v", target, err)
	}
	return rpcInstallSnapshot(resp.GetReport())
}

func buildViaCLIJSON(t *testing.T, env buildTestEnv, target string, build *opv1.BuildOptions, cleanFirst bool) *lifecycleSnapshot {
	t.Helper()

	args := []string{"--format", "json", "--root", env.AbsRoot, "build"}
	args = append(args, buildFlagsFromProto(build)...)
	if cleanFirst {
		args = append(args, "--clean")
	}
	args = append(args, target)
	out := runOP(t, env.OpBin, env.EnvVars, args...)
	return cliLifecycleSnapshot(t, out)
}

func buildAndInstallViaCLIJSON(t *testing.T, env buildTestEnv, target string, build *opv1.BuildOptions) (*lifecycleSnapshot, *installSnapshot) {
	t.Helper()

	args := []string{"--format", "json", "--root", env.AbsRoot, "build"}
	args = append(args, buildFlagsFromProto(build)...)
	args = append(args, "--install", target)
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
	return &payload.Build, normalizeInstallSnapshot(&payload.Install)
}

func buildFlagsFromProto(build *opv1.BuildOptions) []string {
	args := make([]string, 0, 8)
	if build == nil {
		return args
	}
	if build.GetTarget() != "" {
		args = append(args, "--target", build.GetTarget())
	}
	if build.GetMode() != "" {
		args = append(args, "--mode", build.GetMode())
	}
	if build.GetDryRun() {
		args = append(args, "--dry-run")
	}
	if build.GetNoSign() {
		args = append(args, "--no-sign")
	}
	if build.GetHardened() {
		args = append(args, "--hardened")
	}
	return args
}

func cloneBuildOptions(build *opv1.BuildOptions) *opv1.BuildOptions {
	if build == nil {
		return nil
	}
	return &opv1.BuildOptions{
		Target:   build.GetTarget(),
		Mode:     build.GetMode(),
		DryRun:   build.GetDryRun(),
		NoSign:   build.GetNoSign(),
		Hardened: build.GetHardened(),
	}
}

func cliLifecycleSnapshot(t *testing.T, out []byte) *lifecycleSnapshot {
	t.Helper()

	var snapshot lifecycleSnapshot
	if err := json.Unmarshal(out, &snapshot); err != nil {
		t.Fatalf("parse CLI build JSON: %v\nOutput: %s", err, string(out))
	}
	return &snapshot
}

func rpcLifecycleSnapshot(report *opv1.LifecycleReport) *lifecycleSnapshot {
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
		snapshot.Children = append(snapshot.Children, *rpcLifecycleSnapshot(child))
	}
	return snapshot
}

func rpcInstallSnapshot(report *opv1.InstallReport) *installSnapshot {
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

func lifecycleChildHolons(snapshot *lifecycleSnapshot) []string {
	if snapshot == nil {
		return nil
	}
	out := make([]string, 0, len(snapshot.Children))
	for _, child := range snapshot.Children {
		out = append(out, child.Holon)
	}
	return out
}

func lifecycleHasNote(snapshot *lifecycleSnapshot, needle string) bool {
	if snapshot == nil {
		return false
	}
	for _, note := range snapshot.Notes {
		if note == needle {
			return true
		}
	}
	return false
}

func normalizeInstallSnapshot(snapshot *installSnapshot) *installSnapshot {
	if snapshot == nil {
		return nil
	}
	normalized := *snapshot
	normalized.Notes = append([]string(nil), snapshot.Notes...)
	if strings.TrimSpace(normalized.Target) == "." && strings.TrimSpace(normalized.Holon) != "" {
		normalized.Target = normalized.Holon
	}
	if filepath.IsAbs(normalized.Installed) {
		normalized.Installed = filepath.Base(normalized.Installed)
	}
	for i, note := range normalized.Notes {
		const prefix = "installed into "
		if !strings.HasPrefix(note, prefix) {
			continue
		}
		pathValue := strings.TrimSpace(strings.TrimPrefix(note, prefix))
		if filepath.IsAbs(pathValue) {
			normalized.Notes[i] = prefix + filepath.Base(pathValue)
		}
	}
	return &normalized
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

func runInstalledBinary(t *testing.T, binaryPath string, args ...string) string {
	t.Helper()

	out, err := exec.Command(binaryPath, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("run %s %s: %v\nOutput: %s", binaryPath, strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out))
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

func assertInstallEqual(t *testing.T, gotLabel string, got *installSnapshot, wantLabel string, want *installSnapshot) {
	t.Helper()

	if reflect.DeepEqual(got, want) {
		return
	}
	gotData, _ := json.MarshalIndent(got, "", "  ")
	wantData, _ := json.MarshalIndent(want, "", "  ")
	t.Fatalf("%s != %s.\n%s: %s\n%s: %s", gotLabel, wantLabel, gotLabel, string(gotData), wantLabel, string(wantData))
}

func assertSourceReport(t *testing.T, label string, report *lifecycleSnapshot, holon string) {
	t.Helper()

	if report == nil {
		t.Fatalf("%s report is nil", label)
	}
	wantDir := filepath.ToSlash(filepath.Join("examples", "hello-world", holon))
	wantManifest := filepath.ToSlash(filepath.Join("examples", "hello-world", holon, "api", "v1", "holon.proto"))
	if report.Dir != wantDir {
		t.Fatalf("%s dir = %q, want %q", label, report.Dir, wantDir)
	}
	if report.Manifest != wantManifest {
		t.Fatalf("%s manifest = %q, want %q", label, report.Manifest, wantManifest)
	}
}

func assertArtifactExists(t *testing.T, holon string) {
	t.Helper()

	if _, err := os.Stat(buildArtifactPath(t, holon)); err != nil {
		t.Fatalf("artifact missing for %s: %v", holon, err)
	}
}

func assertMarkerRemoved(t *testing.T, holon string) {
	t.Helper()

	if _, err := os.Stat(filepath.Join(holonOPDir(t, holon), "stale-marker.txt")); !os.IsNotExist(err) {
		t.Fatalf("stale marker still exists for %s: %v", holon, err)
	}
}

func buildArtifactPath(t *testing.T, holon string) string {
	t.Helper()
	return filepath.Join(
		absoluteRootPath(t),
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

func holonBinDir(t *testing.T, holon string) string {
	t.Helper()
	return filepath.Join(absoluteRootPath(t), "examples", "hello-world", holon, ".op", "build", holon+".holon", "bin")
}

func holonOPDir(t *testing.T, holon string) string {
	t.Helper()
	return filepath.Join(absoluteRootPath(t), "examples", "hello-world", holon, ".op")
}

func appBundlePath(t *testing.T, app string) string {
	t.Helper()
	return filepath.Join(absoluteRootPath(t), "examples", "hello-world", app, ".op", "build", "GabrielGreetingApp.app")
}

func withMutatedHolonVersion(t *testing.T, holon string, version string, fn func()) {
	t.Helper()

	protoPath := filepath.Join(absoluteRootPath(t), "examples", "hello-world", holon, "api", "v1", "holon.proto")
	content, err := os.ReadFile(protoPath)
	if err != nil {
		t.Fatalf("read holon.proto for %s: %v", holon, err)
	}

	re := regexp.MustCompile(`version:\s*".*?"`)
	mutatedContent := re.ReplaceAllString(string(content), `version: "`+version+`"`)
	if err := os.WriteFile(protoPath, []byte(mutatedContent), 0o644); err != nil {
		t.Fatalf("write holon.proto for %s: %v", holon, err)
	}
	defer func() {
		_ = os.WriteFile(protoPath, content, 0o644)
	}()

	fn()
}

func copyExecutable(t *testing.T, src, dst string) {
	t.Helper()

	sourceFile, err := os.Open(src)
	if err != nil {
		t.Fatalf("open source %s: %v", src, err)
	}
	defer sourceFile.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", dst, err)
	}
	destFile, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		t.Fatalf("open destination %s: %v", dst, err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		t.Fatalf("copy %s to %s: %v", src, dst, err)
	}
}

func copyBinaryBundle(t *testing.T, holon, dstDir string) string {
	t.Helper()

	srcDir := holonBinDir(t, holon)
	if err := exec.Command("cp", "-a", srcDir, dstDir).Run(); err != nil {
		t.Fatalf("copy binary bundle %s to %s: %v", srcDir, dstDir, err)
	}
	return filepath.Join(dstDir, runtime.GOOS+"_"+runtime.GOARCH, holon)
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
