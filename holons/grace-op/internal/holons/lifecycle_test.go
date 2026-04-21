package holons

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/organic-programming/grace-op/internal/identity"
	"github.com/organic-programming/grace-op/internal/testutil"
)

func writeManifestWithIdentity(t *testing.T, dir string, id identity.Identity, suffix string) {
	t.Helper()

	manifest := fmt.Sprintf(
		"schema: holon/v0\nuuid: %q\ngiven_name: %q\nfamily_name: %q\nmotto: %q\ncomposer: %q\nclade: %q\nstatus: %s\nborn: %q\nparents: []\nreproduction: %q\naliases: [%q]\ngenerated_by: %q\nlang: %q\nproto_status: draft\n%s",
		id.UUID,
		id.GivenName,
		id.FamilyName,
		id.Motto,
		id.Composer,
		id.Clade,
		id.Status,
		id.Born,
		id.Reproduction,
		strings.Join(id.Aliases, ", "),
		id.GeneratedBy,
		id.Lang,
		suffix,
	)
	if err := testutil.WriteManifestFile(filepath.Join(dir, identity.ManifestFileName), manifest); err != nil {
		t.Fatal(err)
	}
}

func TestLoadManifestRequiresProtoManifest(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "manifest.txt"), []byte("kind: native\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadManifest(root)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), identity.ProtoManifestFileName) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadManifestRejectsBinaryAndPrimaryTogether(t *testing.T) {
	root := t.TempDir()
	if err := testutil.WriteManifestFile(filepath.Join(root, identity.ManifestFileName), "schema: holon/v0\nkind: composite\nbuild:\n  runner: recipe\n  members:\n    - id: app\n      path: app\n      type: component\n  targets:\n    macos:\n      steps:\n        - assert_file:\n            path: app/demo.app\nartifacts:\n  binary: demo\n  primary: app/demo.app\n"); err != nil {
		t.Fatal(err)
	}

	_, err := LoadManifest(root)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "artifacts.binary and artifacts.primary are mutually exclusive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadManifestFromProtoForGoModule(t *testing.T) {
	root := t.TempDir()
	dir := writeProtoGoHolonFixture(t, root, "demo-proto")

	manifest, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest returned error: %v", err)
	}
	if got := filepath.Base(manifest.Path); got != identity.ProtoManifestFileName {
		t.Fatalf("manifest path basename = %q, want %q", got, identity.ProtoManifestFileName)
	}
	if got := manifest.Manifest.Build.Runner; got != RunnerGoModule {
		t.Fatalf("build runner = %q, want %q", got, RunnerGoModule)
	}
	if got := manifest.Manifest.Build.Main; got != "./cmd/demo-proto" {
		t.Fatalf("build main = %q, want %q", got, "./cmd/demo-proto")
	}
	if got := manifest.Manifest.Artifacts.Binary; got != "demo-proto" {
		t.Fatalf("binary = %q, want %q", got, "demo-proto")
	}
	if got := manifest.Manifest.Kind; got != KindNative {
		t.Fatalf("kind = %q, want %q", got, KindNative)
	}
}

func TestLoadManifestFromProtoForCargo(t *testing.T) {
	root := t.TempDir()
	dir := writeProtoCargoHolonFixture(t, root, "demo-cargo")

	manifest, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest returned error: %v", err)
	}
	if got := filepath.Base(manifest.Path); got != identity.ProtoManifestFileName {
		t.Fatalf("manifest path basename = %q, want %q", got, identity.ProtoManifestFileName)
	}
	if got := manifest.Manifest.Build.Runner; got != RunnerCargo {
		t.Fatalf("build runner = %q, want %q", got, RunnerCargo)
	}
	if got := manifest.Manifest.Build.Main; got != "" {
		t.Fatalf("build main = %q, want empty", got)
	}
	if got := manifest.Manifest.Artifacts.Binary; got != "demo-cargo" {
		t.Fatalf("binary = %q, want %q", got, "demo-cargo")
	}
}

func TestResolveTargetBySlugForProtoManifest(t *testing.T) {
	root := t.TempDir()
	chdirForHolonTest(t, root)
	dir := writeProtoGoHolonFixture(t, root, "demo-proto")

	target, err := ResolveTarget("demo-proto")
	if err != nil {
		t.Fatalf("ResolveTarget returned error: %v", err)
	}
	gotDir, err := filepath.EvalSymlinks(target.Dir)
	if err != nil {
		t.Fatalf("EvalSymlinks(target.Dir) failed: %v", err)
	}
	wantDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks(dir) failed: %v", err)
	}
	if gotDir != wantDir {
		t.Fatalf("target dir = %q, want %q", gotDir, wantDir)
	}
	if target.ManifestErr != nil {
		t.Fatalf("target manifest error = %v", target.ManifestErr)
	}
	if target.Manifest == nil {
		t.Fatal("target manifest should be loaded")
	}
	if target.Identity == nil {
		t.Fatal("target identity should be loaded")
	}
	if got := target.Identity.Slug(); got != "demo-proto" {
		t.Fatalf("identity slug = %q, want %q", got, "demo-proto")
	}
}

func TestResolveTargetLoadsRepoGraceOP(t *testing.T) {
	root := holonsRepoRoot(t)
	chdirForHolonTest(t, root)

	target, err := ResolveTarget("op")
	if err != nil {
		t.Fatalf("ResolveTarget(op) returned error: %v", err)
	}
	if target.ManifestErr != nil {
		t.Fatalf("target manifest error = %v", target.ManifestErr)
	}
	if target.Manifest == nil {
		t.Fatal("target manifest should be loaded")
	}
	if target.Identity == nil {
		t.Fatal("target identity should be loaded")
	}
	if got := target.Identity.Slug(); got != "grace-op" {
		t.Fatalf("identity slug = %q, want %q", got, "grace-op")
	}
	if got := filepath.Base(target.Manifest.Path); got != identity.ProtoManifestFileName {
		t.Fatalf("manifest path basename = %q, want %q", got, identity.ProtoManifestFileName)
	}
}

func TestExecuteLifecycleBuildGoModuleFromProtoManifest(t *testing.T) {
	if _, err := execLookPath("go"); err != nil {
		t.Skip("go command not available")
	}

	root := t.TempDir()
	chdirForHolonTest(t, root)
	dir := writeProtoGoHolonFixture(t, root, "demo-proto")

	buildReport, err := ExecuteLifecycle(OperationBuild, dir)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".op", "build", "demo-proto.holon", "bin", runtimeArchitecture(), "demo-proto")); err != nil {
		t.Fatalf("binary missing after build: %v", err)
	}
	if buildReport.Runner != RunnerGoModule {
		t.Fatalf("runner = %q, want %q", buildReport.Runner, RunnerGoModule)
	}
	if !strings.HasSuffix(buildReport.Manifest, filepath.ToSlash(filepath.Join("demo-proto", "api", "v1", "holon.proto"))) {
		t.Fatalf("manifest report = %q", buildReport.Manifest)
	}
}

func TestResolveTargetBySlugAcrossRoots(t *testing.T) {
	root := t.TempDir()
	chdirForHolonTest(t, root)

	dir := filepath.Join(root, "organic-programming", "holons", "dummy-test")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	id := identity.Identity{
		UUID:        "1234",
		GivenName:   "Sophia",
		FamilyName:  "TestHolon",
		Motto:       "Know thyself.",
		Composer:    "test",
		Clade:       "deterministic/pure",
		Status:      "draft",
		Born:        "2026-03-06",
		Aliases:     []string{"who"},
		GeneratedBy: "test",
		Lang:        "go",
	}
	writeManifestWithIdentity(t, dir, id, "kind: native\nbuild:\n  runner: go-module\n  main: ./cmd/who\nrequires:\n  commands: [go]\n  files: [go.mod]\nartifacts:\n  binary: dummy-test\n")

	target, err := ResolveTarget("dummy-test")
	if err != nil {
		t.Fatalf("ResolveTarget returned error: %v", err)
	}
	if got := filepath.Base(target.Dir); got != "dummy-test" {
		t.Fatalf("dir basename = %q, want %q", got, "dummy-test")
	}
}

func TestResolveBinaryUsesCanonicalArtifactNameForSlug(t *testing.T) {
	root := t.TempDir()
	chdirForHolonTest(t, root)

	dir := filepath.Join(root, "organic-programming", "holons", "dummy-test")
	if err := os.MkdirAll(filepath.Join(dir, ".op", "build", "dummy-test.holon", "bin", runtimeArchitecture()), 0755); err != nil {
		t.Fatal(err)
	}
	id := identity.Identity{
		UUID:        "5678",
		GivenName:   "Sophia",
		FamilyName:  "TestHolon",
		Motto:       "Know thyself.",
		Composer:    "test",
		Clade:       "deterministic/pure",
		Status:      "draft",
		Born:        "2026-03-06",
		Aliases:     []string{"who"},
		GeneratedBy: "test",
		Lang:        "go",
	}
	writeManifestWithIdentity(t, dir, id, "kind: native\nbuild:\n  runner: go-module\n  main: ./cmd/who\nrequires:\n  commands: [go]\n  files: [go.mod]\nartifacts:\n  binary: dummy-test\n")
	binaryPath := filepath.Join(dir, ".op", "build", "dummy-test.holon", "bin", runtimeArchitecture(), "dummy-test")
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	resolved, err := ResolveBinary("dummy-test")
	if err != nil {
		t.Fatalf("ResolveBinary returned error: %v", err)
	}
	resolvedEval, err := filepath.EvalSymlinks(resolved)
	if err != nil {
		t.Fatalf("EvalSymlinks(resolved) failed: %v", err)
	}
	binaryEval, err := filepath.EvalSymlinks(binaryPath)
	if err != nil {
		t.Fatalf("EvalSymlinks(binaryPath) failed: %v", err)
	}
	if resolvedEval != binaryEval {
		t.Fatalf("resolved = %q, want %q", resolvedEval, binaryEval)
	}
}

func TestExecuteLifecycleBuildAndCleanGoModule(t *testing.T) {
	if _, err := execLookPath("go"); err != nil {
		t.Skip("go command not available")
	}

	root := t.TempDir()
	chdirForHolonTest(t, root)

	dir := filepath.Join(root, "demo")
	if err := os.MkdirAll(filepath.Join(dir, "cmd", "demo"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/demo\n\ngo 1.24.0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mainSrc := "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"demo\") }\n"
	if err := os.WriteFile(filepath.Join(dir, "cmd", "demo", "main.go"), []byte(mainSrc), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := "schema: holon/v0\nkind: native\nbuild:\n  runner: go-module\nrequires:\n  commands: [go]\n  files: [go.mod]\nartifacts:\n  binary: demo\n"
	if err := testutil.WriteManifestFile(filepath.Join(dir, identity.ManifestFileName), manifest); err != nil {
		t.Fatal(err)
	}

	buildReport, err := ExecuteLifecycle(OperationBuild, dir)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".op", "build", "demo.holon", "bin", runtimeArchitecture(), "demo")); err != nil {
		t.Fatalf("binary missing after build: %v", err)
	}
	if buildReport.Runner != RunnerGoModule {
		t.Fatalf("runner = %q, want %q", buildReport.Runner, RunnerGoModule)
	}

	cleanReport, err := ExecuteLifecycle(OperationClean, dir)
	if err != nil {
		t.Fatalf("clean failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".op")); !os.IsNotExist(err) {
		t.Fatalf(".op still exists after clean: %v", err)
	}
	if len(cleanReport.Notes) == 0 {
		t.Fatalf("expected clean notes, got %+v", cleanReport)
	}
}

func TestExecuteLifecycleBuildRejectsCrossTargetGoModule(t *testing.T) {
	root := t.TempDir()
	chdirForHolonTest(t, root)

	dir := filepath.Join(root, "demo")
	if err := os.MkdirAll(filepath.Join(dir, "cmd", "demo"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/demo\n\ngo 1.24.0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmd", "demo", "main.go"), []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := testutil.WriteManifestFile(filepath.Join(dir, identity.ManifestFileName), "schema: holon/v0\nkind: native\nbuild:\n  runner: go-module\nrequires:\n  commands: [go]\n  files: [go.mod]\nartifacts:\n  binary: demo\n"); err != nil {
		t.Fatal(err)
	}

	otherTarget := "linux"
	if canonicalRuntimeTarget() == "linux" {
		otherTarget = "windows"
	}

	_, err := ExecuteLifecycle(OperationBuild, dir, BuildOptions{Target: otherTarget, DryRun: true})
	if err == nil {
		t.Fatal("expected cross-target error")
	}
	if !strings.Contains(err.Error(), "cross-target build not implemented") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteLifecycleCleanRecursesCompositeMembers(t *testing.T) {
	root := t.TempDir()
	chdirForHolonTest(t, root)

	target := runtime.GOOS
	if target == "darwin" {
		target = "macos"
	}

	leafDir := filepath.Join(root, "leaf")
	siblingDir := filepath.Join(root, "sibling")
	nestedDir := filepath.Join(root, "nested")
	appDir := filepath.Join(nestedDir, "app")
	rootAppDir := filepath.Join(root, "app")
	for _, dir := range []string{leafDir, siblingDir, appDir, rootAppDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if err := testutil.WriteManifestFile(filepath.Join(leafDir, identity.ManifestFileName), "schema: holon/v0\nkind: native\nbuild:\n  runner: go-module\nartifacts:\n  binary: leaf\n"); err != nil {
		t.Fatal(err)
	}
	if err := testutil.WriteManifestFile(filepath.Join(siblingDir, identity.ManifestFileName), "schema: holon/v0\nkind: native\nbuild:\n  runner: go-module\nartifacts:\n  binary: sibling\n"); err != nil {
		t.Fatal(err)
	}
	nestedManifest := fmt.Sprintf("schema: holon/v0\nkind: composite\nbuild:\n  runner: recipe\n  defaults:\n    target: %s\n    mode: debug\n  members:\n    - id: leaf\n      path: ../leaf\n      type: holon\n    - id: app\n      path: app\n      type: component\n  targets:\n    %s:\n      steps:\n        - build_member: leaf\nartifacts:\n  primary: app/app\n", target, target)
	if err := testutil.WriteManifestFile(filepath.Join(nestedDir, identity.ManifestFileName), nestedManifest); err != nil {
		t.Fatal(err)
	}
	rootManifest := fmt.Sprintf("schema: holon/v0\nkind: composite\nbuild:\n  runner: recipe\n  defaults:\n    target: %s\n    mode: debug\n  members:\n    - id: nested\n      path: nested\n      type: holon\n    - id: sibling\n      path: sibling\n      type: holon\n    - id: app\n      path: app\n      type: component\n  targets:\n    %s:\n      steps:\n        - build_member: nested\n        - build_member: sibling\nartifacts:\n  primary: app/app\n", target, target)
	if err := testutil.WriteManifestFile(filepath.Join(root, identity.ManifestFileName), rootManifest); err != nil {
		t.Fatal(err)
	}

	for _, marker := range []string{
		filepath.Join(root, ".op", "stale.txt"),
		filepath.Join(nestedDir, ".op", "stale.txt"),
		filepath.Join(leafDir, ".op", "stale.txt"),
		filepath.Join(siblingDir, ".op", "stale.txt"),
	} {
		if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(marker, []byte("stale"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	report, err := ExecuteLifecycle(OperationClean, root)
	if err != nil {
		t.Fatalf("clean failed: %v", err)
	}

	for _, dir := range []string{
		filepath.Join(root, ".op"),
		filepath.Join(nestedDir, ".op"),
		filepath.Join(leafDir, ".op"),
		filepath.Join(siblingDir, ".op"),
	} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("%s still exists after recursive clean: %v", dir, err)
		}
	}

	if len(report.Children) != 2 {
		t.Fatalf("root clean children = %d, want 2", len(report.Children))
	}

	var nestedReport *Report
	for i := range report.Children {
		if report.Children[i].Holon == "nested" {
			nestedReport = &report.Children[i]
			break
		}
	}
	if nestedReport == nil {
		t.Fatalf("nested child report missing: %+v", report.Children)
	}
	if len(nestedReport.Children) != 1 || nestedReport.Children[0].Holon != "leaf" {
		t.Fatalf("nested child reports = %+v, want one leaf child", nestedReport.Children)
	}
}

func writeProtoGoHolonFixture(t *testing.T, root, name string) string {
	t.Helper()

	dir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(dir, "cmd", name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "api", "v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/"+name+"\n\ngo 1.24.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmd", name, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeSharedHolonManifestProto(t, root)

	proto := fmt.Sprintf(`syntax = "proto3";

package test.v1;

import "holons/v1/manifest.proto";

option (holons.v1.manifest) = {
  identity: {
    schema: "holon/v1"
    uuid: "%s-uuid"
    given_name: "Demo"
    family_name: "Proto"
    motto: "Proto-backed test holon."
    composer: "test"
    status: "draft"
    born: "2026-03-15"
  }
  kind: "native"
  lang: "go"
  build: {
    runner: "go-module"
    main: "./cmd/%s"
  }
  requires: {
    commands: ["go"]
    files: ["go.mod"]
  }
  artifacts: {
    binary: "%s"
  }
};
`, name, name, name)
	if err := os.WriteFile(filepath.Join(dir, "api", "v1", "holon.proto"), []byte(proto), 0o644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func writeProtoCargoHolonFixture(t *testing.T, root, name string) string {
	t.Helper()

	dir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(dir, "api", "v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \""+name+"\"\nversion = \"0.1.0\"\nedition = \"2021\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeSharedHolonManifestProto(t, root)

	proto := fmt.Sprintf(`syntax = "proto3";

package test.v1;

import "holons/v1/manifest.proto";

option (holons.v1.manifest) = {
  identity: {
    schema: "holon/v1"
    uuid: "%s-uuid"
    given_name: "Demo"
    family_name: "Cargo"
    motto: "Proto-backed cargo test holon."
    composer: "test"
    status: "draft"
    born: "2026-03-16"
  }
  kind: "native"
  lang: "rust"
  build: {
    runner: "cargo"
  }
  requires: {
    commands: ["cargo"]
    files: ["Cargo.toml"]
  }
  artifacts: {
    binary: "%s"
  }
};
`, name, name)
	if err := os.WriteFile(filepath.Join(dir, "api", "v1", "holon.proto"), []byte(proto), 0o644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func writeSharedHolonManifestProto(t *testing.T, root string) {
	t.Helper()

	source := filepath.Join(holonsRepoRoot(t), "holons", "grace-op", "_protos", "holons", "v1", "manifest.proto")
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read %s: %v", source, err)
	}

	targetDir := filepath.Join(root, "_protos", "holons", "v1")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", targetDir, err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "manifest.proto"), data, 0o644); err != nil {
		t.Fatalf("write manifest.proto: %v", err)
	}
}

func writeSharedHolonCoaxProto(t *testing.T, root string) {
	t.Helper()

	source := filepath.Join(holonsRepoRoot(t), "holons", "grace-op", "_protos", "holons", "v1", "coax.proto")
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read %s: %v", source, err)
	}

	targetDir := filepath.Join(root, "_protos", "holons", "v1")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", targetDir, err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "coax.proto"), data, 0o644); err != nil {
		t.Fatalf("write coax.proto: %v", err)
	}
}

func holonsRepoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..")
}

func TestCMakeRunnerDryRunUsesModeSpecificConfig(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "cmake-demo")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := testutil.WriteManifestFile(filepath.Join(dir, identity.ManifestFileName), "schema: holon/v0\nkind: native\nbuild:\n  runner: cmake\nartifacts:\n  binary: demo\n"); err != nil {
		t.Fatal(err)
	}

	manifest, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}
	var report Report
	ctx := BuildContext{Target: canonicalRuntimeTarget(), Mode: buildModeProfile, DryRun: true}
	if err := (cmakeRunner{}).build(manifest, ctx, &report); err != nil {
		t.Fatalf("cmake dry-run build failed: %v", err)
	}
	if len(report.Commands) != 2 {
		t.Fatalf("commands = %d, want 2", len(report.Commands))
	}
	if !strings.Contains(report.Commands[0], "CMAKE_BUILD_TYPE=RelWithDebInfo") {
		t.Fatalf("configure command missing profile config: %q", report.Commands[0])
	}
	if !strings.Contains(report.Commands[1], "--config RelWithDebInfo") {
		t.Fatalf("build command missing profile config: %q", report.Commands[1])
	}
}

func chdirForHolonTest(t *testing.T, dir string) {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
}

func execLookPath(file string) (string, error) {
	return exec.LookPath(file)
}
