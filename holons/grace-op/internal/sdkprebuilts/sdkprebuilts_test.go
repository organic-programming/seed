package sdkprebuilts

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	seedtoolchain "github.com/organic-programming/seed-github-scripts/seed_toolchain"
)

const testTarget = "x86_64-unknown-linux-gnu"

func TestInstallPathUsesOPPATH(t *testing.T) {
	runtimeHome := t.TempDir()
	t.Setenv("OPPATH", runtimeHome)

	got := InstallPath("cpp", "1.80.0", testTarget)
	want := filepath.Join(runtimeHome, "sdk", "cpp", "1.80.0", testTarget)
	if got != want {
		t.Fatalf("InstallPath() = %q, want %q", got, want)
	}
}

func TestInstallLocalTarballVerifiesAndLocates(t *testing.T) {
	runtimeHome := t.TempDir()
	t.Setenv("OPPATH", runtimeHome)

	source := filepath.Join(t.TempDir(), "cpp-1.80.0-"+testTarget+".tar.gz")
	writeTestTarGz(t, source, map[string]testTarEntry{
		"include/grpc/grpc.h": {Mode: 0o644, Body: []byte("/* grpc */\n")},
		"lib/libgrpc.a":       {Mode: 0o644, Body: []byte("archive\n")},
		"bin/protoc":          {Mode: 0o755, Body: []byte("#!/bin/sh\n")},
	})
	writeSHA256Sidecar(t, source)

	prebuilt, notes, err := Install(context.Background(), InstallOptions{
		Lang:   "cpp",
		Target: testTarget,
		Source: source,
	})
	if err != nil {
		t.Fatalf("Install() returned error: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("Install() notes = %#v, want none", notes)
	}
	if prebuilt.Version != "1.80.0" {
		t.Fatalf("installed version = %q, want 1.80.0", prebuilt.Version)
	}
	if prebuilt.Path != filepath.Join(runtimeHome, "sdk", "cpp", "1.80.0", testTarget) {
		t.Fatalf("installed path = %q", prebuilt.Path)
	}
	for _, rel := range []string{"include/grpc/grpc.h", "lib/libgrpc.a", "bin/protoc", metadataFile} {
		if _, err := os.Stat(filepath.Join(prebuilt.Path, rel)); err != nil {
			t.Fatalf("installed file %s missing: %v", rel, err)
		}
	}

	verified, ok, err := Verify(QueryOptions{Lang: "cpp", Target: testTarget})
	if err != nil {
		t.Fatalf("Verify() returned error: %v", err)
	}
	if !ok {
		t.Fatalf("Verify() ok = false, want true")
	}
	if verified.Path != prebuilt.Path {
		t.Fatalf("Verify() path = %q, want %q", verified.Path, prebuilt.Path)
	}

	located, err := Locate(QueryOptions{Lang: "cpp", Target: testTarget})
	if err != nil {
		t.Fatalf("Locate() returned error: %v", err)
	}
	if located.Path != prebuilt.Path {
		t.Fatalf("Locate() path = %q, want %q", located.Path, prebuilt.Path)
	}
}

func TestInstallLocalHolonsReleaseTarballInfersVersion(t *testing.T) {
	runtimeHome := t.TempDir()
	t.Setenv("OPPATH", runtimeHome)

	source := filepath.Join(t.TempDir(), "zig-holons-v0.1.0-"+testTarget+".tar.gz")
	writeTestTarGz(t, source, map[string]testTarEntry{
		"include/holons_sdk.h": {Mode: 0o644, Body: []byte("/* holons */\n")},
		"lib/libholons_zig.a":  {Mode: 0o644, Body: []byte("archive\n")},
	})
	writeSHA256Sidecar(t, source)

	prebuilt, _, err := Install(context.Background(), InstallOptions{
		Lang:   "zig",
		Target: testTarget,
		Source: source,
	})
	if err != nil {
		t.Fatalf("Install() returned error: %v", err)
	}
	if prebuilt.Version != "0.1.0" {
		t.Fatalf("installed version = %q, want 0.1.0", prebuilt.Version)
	}
	if prebuilt.Path != filepath.Join(runtimeHome, "sdk", "zig", "0.1.0", testTarget) {
		t.Fatalf("installed path = %q", prebuilt.Path)
	}
}

func TestInstallSourceUsesManifestTargetAndVersionWhenUnset(t *testing.T) {
	runtimeHome := t.TempDir()
	t.Setenv("OPPATH", runtimeHome)

	host, err := HostTriplet()
	if err != nil {
		t.Fatalf("HostTriplet() returned error: %v", err)
	}
	archiveTarget := targetOtherThan(t, host)
	source := filepath.Join(t.TempDir(), "zig-holons-v0.2.0-"+archiveTarget+".tar.gz")
	writeTestTarGz(t, source, map[string]testTarEntry{
		"include/holons_sdk.h": {Mode: 0o644, Body: []byte("/* holons */\n")},
		"lib/libholons_zig.a":  {Mode: 0o644, Body: []byte("archive\n")},
		metadataFile: {Mode: 0o644, Body: []byte(fmt.Sprintf(`{
  "lang": "zig",
  "version": "0.2.0",
  "target": %q
}
`, archiveTarget))},
	})
	writeSHA256Sidecar(t, source)

	prebuilt, notes, err := Install(context.Background(), InstallOptions{
		Lang:   "zig",
		Source: source,
	})
	if err != nil {
		t.Fatalf("Install() returned error: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("Install() notes = %#v, want none", notes)
	}
	wantPath := filepath.Join(runtimeHome, "sdk", "zig", "0.2.0", archiveTarget)
	if prebuilt.Path != wantPath {
		t.Fatalf("installed path = %q, want %q", prebuilt.Path, wantPath)
	}
	if prebuilt.Target != archiveTarget {
		t.Fatalf("installed target = %q, want %q", prebuilt.Target, archiveTarget)
	}
	if prebuilt.Version != "0.2.0" {
		t.Fatalf("installed version = %q, want 0.2.0", prebuilt.Version)
	}
	hostPath := filepath.Join(runtimeHome, "sdk", "zig", "0.2.0", host)
	if _, err := os.Stat(hostPath); err == nil {
		t.Fatalf("host path %s was created for cross-target source archive", hostPath)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat host path: %v", err)
	}

	metadata, err := metadataForPath(prebuilt.Path)
	if err != nil {
		t.Fatalf("read installed metadata: %v", err)
	}
	if metadata.Target != archiveTarget || metadata.Version != "0.2.0" {
		t.Fatalf("installed metadata target/version = %q/%q, want %q/0.2.0", metadata.Target, metadata.Version, archiveTarget)
	}
}

func TestInstallSourceRejectsTargetMismatch(t *testing.T) {
	runtimeHome := t.TempDir()
	t.Setenv("OPPATH", runtimeHome)

	archiveTarget := "x86_64-unknown-linux-musl"
	requestedTarget := "aarch64-apple-darwin"
	source := filepath.Join(t.TempDir(), "zig-holons-v0.2.0-"+archiveTarget+".tar.gz")
	writeTestTarGz(t, source, map[string]testTarEntry{
		"include/holons_sdk.h": {Mode: 0o644, Body: []byte("/* holons */\n")},
		metadataFile: {Mode: 0o644, Body: []byte(fmt.Sprintf(`{
  "lang": "zig",
  "version": "0.2.0",
  "target": %q
}
`, archiveTarget))},
	})
	writeSHA256Sidecar(t, source)

	_, _, err := Install(context.Background(), InstallOptions{
		Lang:   "zig",
		Target: requestedTarget,
		Source: source,
	})
	if err == nil {
		t.Fatalf("Install() returned nil error, want target mismatch")
	}
	for _, want := range []string{archiveTarget, requestedTarget} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Install() error = %q, want it to contain %q", err.Error(), want)
		}
	}
}

func TestInstallSourceRejectsVersionMismatch(t *testing.T) {
	runtimeHome := t.TempDir()
	t.Setenv("OPPATH", runtimeHome)

	archiveTarget := "x86_64-unknown-linux-musl"
	source := filepath.Join(t.TempDir(), "zig-holons-v0.2.0-"+archiveTarget+".tar.gz")
	writeTestTarGz(t, source, map[string]testTarEntry{
		"include/holons_sdk.h": {Mode: 0o644, Body: []byte("/* holons */\n")},
		metadataFile: {Mode: 0o644, Body: []byte(fmt.Sprintf(`{
  "lang": "zig",
  "version": "0.2.0",
  "target": %q
}
`, archiveTarget))},
	})
	writeSHA256Sidecar(t, source)

	_, _, err := Install(context.Background(), InstallOptions{
		Lang:    "zig",
		Version: "0.3.0",
		Source:  source,
	})
	if err == nil {
		t.Fatalf("Install() returned nil error, want version mismatch")
	}
	for _, want := range []string{"0.2.0", "0.3.0"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Install() error = %q, want it to contain %q", err.Error(), want)
		}
	}
}

func TestInstallSourceRejectsLangMismatch(t *testing.T) {
	runtimeHome := t.TempDir()
	t.Setenv("OPPATH", runtimeHome)

	archiveTarget := "x86_64-unknown-linux-musl"
	source := filepath.Join(t.TempDir(), "zig-holons-v0.2.0-"+archiveTarget+".tar.gz")
	writeTestTarGz(t, source, map[string]testTarEntry{
		"include/holons_sdk.h": {Mode: 0o644, Body: []byte("/* holons */\n")},
		metadataFile: {Mode: 0o644, Body: []byte(fmt.Sprintf(`{
  "lang": "cpp",
  "version": "0.2.0",
  "target": %q
}
`, archiveTarget))},
	})
	writeSHA256Sidecar(t, source)

	_, _, err := Install(context.Background(), InstallOptions{
		Lang:   "zig",
		Source: source,
	})
	if err == nil {
		t.Fatalf("Install() returned nil error, want lang mismatch")
	}
	for _, want := range []string{"cpp", "zig"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Install() error = %q, want it to contain %q", err.Error(), want)
		}
	}
}

func TestInstallPreservesCodegenManifestBlock(t *testing.T) {
	runtimeHome := t.TempDir()
	t.Setenv("OPPATH", runtimeHome)

	source := filepath.Join(t.TempDir(), "cpp-1.80.0-"+testTarget+".tar.gz")
	writeTestTarGz(t, source, map[string]testTarEntry{
		"bin/protoc-gen-cpp": {Mode: 0o755, Body: []byte("#!/bin/sh\n")},
		metadataFile: {Mode: 0o644, Body: []byte(`{
  "codegen": {
    "plugins": [
      {
        "name": "cpp",
        "binary": "bin/protoc-gen-cpp",
        "out_subdir": "cpp"
      }
    ]
  }
}
`)},
	})
	writeSHA256Sidecar(t, source)

	prebuilt, _, err := Install(context.Background(), InstallOptions{
		Lang:   "cpp",
		Target: testTarget,
		Source: source,
	})
	if err != nil {
		t.Fatalf("Install() returned error: %v", err)
	}
	if prebuilt.Codegen == nil || len(prebuilt.Codegen.Plugins) != 1 {
		t.Fatalf("Codegen metadata = %#v, want one plugin", prebuilt.Codegen)
	}
	if prebuilt.Codegen.Plugins[0].Name != "cpp" {
		t.Fatalf("plugin name = %q, want cpp", prebuilt.Codegen.Plugins[0].Name)
	}

	metadata, err := metadataForPath(prebuilt.Path)
	if err != nil {
		t.Fatalf("read installed metadata: %v", err)
	}
	if metadata.Codegen == nil || len(metadata.Codegen.Plugins) != 1 {
		t.Fatalf("installed metadata codegen = %#v, want one plugin", metadata.Codegen)
	}
}

func TestProtocResolvedFromSharedPool(t *testing.T) {
	runtimeHome := t.TempDir()
	t.Setenv("OPPATH", runtimeHome)

	repoRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoRoot, "seed-toolchain.yaml"), []byte(testSeedToolchainYAML(t, "java", testTarget, "32.0", seedSharedProtoc(t, runtimeHome, "32.0", testTarget))), 0o644); err != nil {
		t.Fatalf("write seed toolchain: %v", err)
	}
	envOut := filepath.Join(repoRoot, "env.out")
	script := filepath.Join(repoRoot, "build.sh")
	if err := os.WriteFile(script, []byte("#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n%s\\n' \"$OP_SDK_PROTOC\" \"$OP_SDK_PROTOC_INCLUDE\" > "+shellQuote(envOut)+"\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	if err := runBuildScript(context.Background(), script, "java", testTarget, "0.1.0", BuildOptions{}, repoRoot); err != nil {
		t.Fatalf("runBuildScript() returned error: %v", err)
	}
	data, err := os.ReadFile(envOut)
	if err != nil {
		t.Fatalf("read env output: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("env output = %q, want two lines", string(data))
	}
	if want := SharedProtocBinary("32.0", testTarget); lines[0] != want {
		t.Fatalf("OP_SDK_PROTOC = %q, want %q", lines[0], want)
	}
	if want := SharedProtocInclude("32.0"); lines[1] != want {
		t.Fatalf("OP_SDK_PROTOC_INCLUDE = %q, want %q", lines[1], want)
	}
}

func TestSharedPoolSelfHealsOnInstall(t *testing.T) {
	runtimeHome := t.TempDir()
	t.Setenv("OPPATH", runtimeHome)
	protocSource, protocSHA := writeSharedProtocSource(t, "healthy")
	source := writeSDKTarballWithToolchain(t, "java", []ToolchainEntry{{
		Name:    "protoc",
		Version: "32.0",
		Target:  testTarget,
		SHA256:  protocSHA,
		Source:  protocSource,
	}})

	if _, _, err := Install(context.Background(), InstallOptions{Lang: "java", Target: testTarget, Source: source}); err != nil {
		t.Fatalf("Install() returned error: %v", err)
	}
	sharedBin := SharedProtocBinary("32.0", testTarget)
	firstInfo, err := os.Stat(sharedBin)
	if err != nil {
		t.Fatalf("shared protoc missing: %v", err)
	}
	snapshotData, err := os.ReadFile(filepath.Join(runtimeHome, "sdk", "shared", sharedSeedReleaseSnapshotName))
	if err != nil {
		t.Fatalf("shared seed release snapshot missing: %v", err)
	}
	var snapshot sharedSeedReleaseSnapshot
	if err := json.Unmarshal(snapshotData, &snapshot); err != nil {
		t.Fatalf("parse shared seed release snapshot: %v", err)
	}
	if snapshot.SeedRelease != "test" || !toolchainContains(snapshot.Toolchain, "protoc", "32.0") {
		t.Fatalf("shared seed release snapshot = %#v, want test/protoc 32.0", snapshot)
	}

	if _, notes, err := Install(context.Background(), InstallOptions{Lang: "java", Target: testTarget, Source: source}); err != nil {
		t.Fatalf("second Install() returned error: %v", err)
	} else if len(notes) != 1 || notes[0] != "already installed" {
		t.Fatalf("second Install() notes = %#v, want already installed only", notes)
	}
	secondInfo, err := os.Stat(sharedBin)
	if err != nil {
		t.Fatalf("shared protoc missing after second install: %v", err)
	}
	if !secondInfo.ModTime().Equal(firstInfo.ModTime()) {
		t.Fatalf("shared protoc mtime changed on idempotent install: %s -> %s", firstInfo.ModTime(), secondInfo.ModTime())
	}
}

func TestSharedPoolSha256Mismatch(t *testing.T) {
	runtimeHome := t.TempDir()
	t.Setenv("OPPATH", runtimeHome)
	protocSource, protocSHA := writeSharedProtocSource(t, "restored")
	source := writeSDKTarballWithToolchain(t, "java", []ToolchainEntry{{
		Name:    "protoc",
		Version: "32.0",
		Target:  testTarget,
		SHA256:  protocSHA,
		Source:  protocSource,
	}})

	if _, _, err := Install(context.Background(), InstallOptions{Lang: "java", Target: testTarget, Source: source}); err != nil {
		t.Fatalf("Install() returned error: %v", err)
	}
	sharedBin := SharedProtocBinary("32.0", testTarget)
	if err := os.WriteFile(sharedBin, []byte("corrupt"), 0o755); err != nil {
		t.Fatalf("corrupt shared protoc: %v", err)
	}
	if _, _, err := Install(context.Background(), InstallOptions{Lang: "java", Target: testTarget, Source: source}); err != nil {
		t.Fatalf("second Install() returned error: %v", err)
	}
	data, err := os.ReadFile(sharedBin)
	if err != nil {
		t.Fatalf("read shared protoc: %v", err)
	}
	if string(data) != "restored" {
		t.Fatalf("shared protoc body = %q, want restored", string(data))
	}
}

func TestSharedPoolNonExecutableTriggersRepair(t *testing.T) {
	runtimeHome := t.TempDir()
	t.Setenv("OPPATH", runtimeHome)
	protocSource, protocSHA := writeSharedProtocSource(t, "executable")
	source := writeSDKTarballWithToolchain(t, "java", []ToolchainEntry{{
		Name:    "protoc",
		Version: "32.0",
		Target:  testTarget,
		SHA256:  protocSHA,
		Source:  protocSource,
	}})

	if _, _, err := Install(context.Background(), InstallOptions{Lang: "java", Target: testTarget, Source: source}); err != nil {
		t.Fatalf("Install() returned error: %v", err)
	}
	sharedBin := SharedProtocBinary("32.0", testTarget)
	if err := os.Chmod(sharedBin, 0o644); err != nil {
		t.Fatalf("make shared protoc non-executable: %v", err)
	}
	if _, _, err := Install(context.Background(), InstallOptions{Lang: "java", Target: testTarget, Source: source}); err != nil {
		t.Fatalf("second Install() returned error: %v", err)
	}
	info, err := os.Stat(sharedBin)
	if err != nil {
		t.Fatalf("stat shared protoc: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("shared protoc mode = %v, want executable", info.Mode())
	}
}

func TestPurePluginSdkSkipsProtoc(t *testing.T) {
	runtimeHome := t.TempDir()
	t.Setenv("OPPATH", runtimeHome)
	source := writeSDKTarballWithToolchain(t, "go", []ToolchainEntry{{
		Name:    "protoc-gen-go",
		Version: "v1.36.11",
	}})

	if _, _, err := Install(context.Background(), InstallOptions{Lang: "go", Target: testTarget, Source: source}); err != nil {
		t.Fatalf("Install() returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(runtimeHome, "sdk", "shared")); !os.IsNotExist(err) {
		t.Fatalf("shared pool stat err = %v, want not exist", err)
	}
}

func TestSdkManifestToolchainEchoesCentralPin(t *testing.T) {
	repoRoot := testRepoRoot(t)
	javaToolchain, err := ToolchainForSDK(repoRoot, "java", "aarch64-apple-darwin")
	if err != nil {
		t.Fatalf("ToolchainForSDK(java) returned error: %v", err)
	}
	if len(javaToolchain) == 0 || javaToolchain[0].Name != "protoc" || javaToolchain[0].Version != "32.0" {
		t.Fatalf("java toolchain = %#v, want protoc 32.0 first", javaToolchain)
	}
	goToolchain, err := ToolchainForSDK(repoRoot, "go", testTarget)
	if err != nil {
		t.Fatalf("ToolchainForSDK(go) returned error: %v", err)
	}
	for _, entry := range goToolchain {
		if entry.Name == "protoc" {
			t.Fatalf("go toolchain declares protoc: %#v", goToolchain)
		}
	}
	if !toolchainContains(goToolchain, "protoc-gen-go", "v1.36.11") {
		t.Fatalf("go toolchain = %#v, want protoc-gen-go v1.36.11", goToolchain)
	}
}

func TestSeedToolchainScriptMatchesGoDerivation(t *testing.T) {
	repoRoot := testRepoRoot(t)
	seed, err := LoadSeedToolchain(repoRoot)
	if err != nil {
		t.Fatalf("LoadSeedToolchain() returned error: %v", err)
	}
	goEntries, err := ToolchainForSDK(repoRoot, "java", "aarch64-apple-darwin")
	if err != nil {
		t.Fatalf("ToolchainForSDK(java) returned error: %v", err)
	}
	scriptSeed, err := seedtoolchain.Load(repoRoot)
	if err != nil {
		t.Fatalf("seed_toolchain Load returned error: %v", err)
	}
	scriptEntries, err := seedtoolchain.ToolchainManifest(scriptSeed, "java", "aarch64-apple-darwin")
	if err != nil {
		t.Fatalf("seed_toolchain manifest returned error: %v", err)
	}
	goJSON, err := json.Marshal(goEntries)
	if err != nil {
		t.Fatalf("marshal Go toolchain: %v", err)
	}
	scriptJSON, err := json.Marshal(scriptEntries)
	if err != nil {
		t.Fatalf("marshal script toolchain: %v", err)
	}
	if string(goJSON) != string(scriptJSON) {
		t.Fatalf("script toolchain = %s, want %s", scriptJSON, goJSON)
	}
	if got, want := seedtoolchain.CPPProtobufTag(scriptSeed), strings.TrimSpace(seed.CPPRuntime.ProtobufSubmoduleTag); got != want {
		t.Fatalf("cpp-protobuf-tag = %q, want %q", got, want)
	}
}

func TestCppSubmoduleMatchesPin(t *testing.T) {
	repoRoot := testRepoRoot(t)
	seed, err := LoadSeedToolchain(repoRoot)
	if err != nil {
		t.Fatalf("LoadSeedToolchain() returned error: %v", err)
	}
	tag := strings.TrimSpace(seed.CPPRuntime.ProtobufSubmoduleTag)
	if tag == "" {
		t.Fatal("cpp_runtime.protobuf_submodule_tag is empty")
	}
	protobufDir := filepath.Join(repoRoot, "sdk", "zig-holons", "third_party", "grpc", "third_party", "protobuf")
	if _, err := os.Stat(filepath.Join(protobufDir, ".git")); err != nil {
		t.Skipf("protobuf submodule is not initialized at %s", protobufDir)
	}
	head := gitOutput(t, protobufDir, "rev-parse", "HEAD")
	tagCommit := gitOutput(t, protobufDir, "rev-list", "-n", "1", tag)
	if head != tagCommit {
		t.Fatalf("protobuf submodule HEAD = %s, want %s (%s)", head, tagCommit, tag)
	}
}

func TestLocalSourceTreeSHA256TreatsNonGitWorkspaceAsNoSource(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoRoot, "go.work"), []byte("go 1.26.1\n"), 0o644); err != nil {
		t.Fatalf("write go.work: %v", err)
	}
	sourceDir := filepath.Join(repoRoot, "sdk", "c-holons")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("create sdk source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "README.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatalf("write sdk source fixture: %v", err)
	}
	t.Chdir(repoRoot)

	got, ok, err := LocalSourceTreeSHA256("c")
	if err != nil {
		t.Fatalf("LocalSourceTreeSHA256() returned error: %v", err)
	}
	if got != "" || ok {
		t.Fatalf("LocalSourceTreeSHA256() = %q, %v, want empty hash and ok=false", got, ok)
	}
}

func TestBuildRejectsEmptyVersion(t *testing.T) {
	cases := []string{"", "   "}
	for _, version := range cases {
		_, _, err := Build(context.Background(), BuildOptions{
			Lang:    "go",
			Target:  testTarget,
			Version: version,
		})
		if err == nil {
			t.Fatalf("Build(version=%q): want error, got nil", version)
		}
		if !strings.Contains(err.Error(), "version is required") {
			t.Fatalf("Build(version=%q) error %q does not mention version requirement", version, err.Error())
		}
	}
}

func TestListInstalledIteratesRuntimeTree(t *testing.T) {
	runtimeHome := t.TempDir()
	t.Setenv("OPPATH", runtimeHome)

	writeInstalledMetadata(t, Prebuilt{
		Lang:          "ruby",
		Version:       "1.58.3",
		Target:        testTarget,
		Path:          filepath.Join(runtimeHome, "sdk", "ruby", "1.58.3", testTarget),
		ArchiveSHA256: "bbbb",
		Installed:     true,
	})
	writeInstalledMetadata(t, Prebuilt{
		Lang:          "cpp",
		Version:       "1.80.0",
		Target:        testTarget,
		Path:          filepath.Join(runtimeHome, "sdk", "cpp", "1.80.0", testTarget),
		ArchiveSHA256: "aaaa",
		Installed:     true,
	})
	if err := os.MkdirAll(filepath.Join(runtimeHome, "sdk", "cpp", "1.80.0", "ios-arm64"), 0o755); err != nil {
		t.Fatalf("create unsupported target dir: %v", err)
	}

	entries, err := ListInstalled("")
	if err != nil {
		t.Fatalf("ListInstalled() returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("ListInstalled() returned %d entries, want 2: %#v", len(entries), entries)
	}
	if entries[0].Lang != "cpp" || entries[1].Lang != "ruby" {
		t.Fatalf("entries sorted by lang = %#v", entries)
	}
	if entries[0].ArchiveSHA256 != "aaaa" || entries[1].ArchiveSHA256 != "bbbb" {
		t.Fatalf("metadata not loaded: %#v", entries)
	}

	filtered, err := ListInstalled("ruby")
	if err != nil {
		t.Fatalf("ListInstalled(ruby) returned error: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Lang != "ruby" {
		t.Fatalf("ListInstalled(ruby) = %#v", filtered)
	}
}

func TestListAvailableReadsGitHubReleases(t *testing.T) {
	server := newReleaseServer(t, map[string][]byte{})
	t.Setenv(releasesAPIEnv, server.URL+"/releases")

	entries, notes, err := ListAvailable(" zig ")
	if err != nil {
		t.Fatalf("ListAvailable() returned error: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("notes = %#v, want none", notes)
	}
	if len(entries) != 3 {
		t.Fatalf("entries = %#v, want three zig artifacts", entries)
	}
	if entries[0].Lang != "zig" || entries[0].Version != "0.1.1" || entries[0].Target != "aarch64-apple-darwin" {
		t.Fatalf("first entry = %#v", entries[0])
	}
	if entries[0].Source == "" {
		t.Fatalf("first entry has empty source")
	}
}

func TestInstallWithoutSourceResolvesLatestRelease(t *testing.T) {
	runtimeHome := t.TempDir()
	t.Setenv("OPPATH", runtimeHome)

	assets := map[string][]byte{}
	archiveName := "zig-holons-v0.1.1-" + testTarget + ".tar.gz"
	archivePath := filepath.Join(t.TempDir(), archiveName)
	writeTestTarGz(t, archivePath, map[string]testTarEntry{
		"include/holons_sdk.h": {Mode: 0o644, Body: []byte("/* holons */\n")},
		"lib/libholons_zig.a":  {Mode: 0o644, Body: []byte("archive\n")},
	})
	archiveData, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	sum := sha256.Sum256(archiveData)
	assets["/assets/"+archiveName] = archiveData
	assets["/assets/"+archiveName+".sha256"] = []byte(hex.EncodeToString(sum[:]) + "  " + archiveName + "\n")

	server := newReleaseServer(t, assets)
	t.Setenv(releasesAPIEnv, server.URL+"/releases")

	prebuilt, notes, err := Install(context.Background(), InstallOptions{
		Lang:   "zig",
		Target: testTarget,
	})
	if err != nil {
		t.Fatalf("Install() returned error: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("notes = %#v, want none", notes)
	}
	if prebuilt.Version != "0.1.1" {
		t.Fatalf("version = %q, want 0.1.1", prebuilt.Version)
	}
	if prebuilt.Source != server.URL+"/assets/"+archiveName {
		t.Fatalf("source = %q", prebuilt.Source)
	}
	if _, err := os.Stat(filepath.Join(prebuilt.Path, "lib/libholons_zig.a")); err != nil {
		t.Fatalf("installed archive missing: %v", err)
	}
}

func TestListAvailableReadsReleaseManifest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `[
  {
    "tag_name": "cpp-holons-v1.80.0",
    "draft": false,
    "assets": [
      {"name": "release-manifest.json", "browser_download_url": "%s/assets/release-manifest.json"},
      {"name": "cpp-holons-v1.80.0-%s.tar.gz", "browser_download_url": "%s/assets/legacy.tar.gz"}
    ]
  }
]`, serverURL(r), testTarget, serverURL(r))
		case "/assets/release-manifest.json":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{
  "schema": "sdk-prebuilts-release-manifest/v1",
  "sdk": "cpp",
  "version": "1.80.0",
  "tag": "cpp-holons-v1.80.0",
  "artifacts": [
    {
      "target": "%s",
      "source_tree_sha256": "source-tree-abc",
      "archive": {
        "name": "cpp-holons-v1.80.0-%s.tar.gz",
        "url": "%s/assets/from-manifest.tar.gz",
        "sha256": "abc123"
      },
      "debug": {
        "name": "cpp-holons-v1.80.0-%s-debug.tar.gz",
        "url": "%s/assets/from-manifest-debug.tar.gz",
        "sha256": "def456"
      },
      "sbom": {
        "name": "cpp-holons-v1.80.0-%s.tar.gz.spdx.json",
        "url": "%s/assets/from-manifest.tar.gz.spdx.json",
        "sha256": "789abc"
      }
    }
  ]
}`, testTarget, testTarget, serverURL(r), testTarget, serverURL(r), testTarget, serverURL(r))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv(releasesAPIEnv, server.URL+"/releases")

	entries, notes, err := ListAvailable("cpp")
	if err != nil {
		t.Fatalf("ListAvailable() returned error: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("notes = %#v, want none", notes)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %#v, want one manifest artifact", entries)
	}
	if entries[0].Source != server.URL+"/assets/from-manifest.tar.gz" {
		t.Fatalf("source = %q, want manifest URL", entries[0].Source)
	}
	if entries[0].ArchiveSHA256 != "abc123" {
		t.Fatalf("archive sha = %q, want abc123", entries[0].ArchiveSHA256)
	}
	if entries[0].SourceTreeSHA256 != "source-tree-abc" {
		t.Fatalf("source tree sha = %q, want source-tree-abc", entries[0].SourceTreeSHA256)
	}
}

func TestInstallWithoutSourceUsesReleaseManifestSHA(t *testing.T) {
	runtimeHome := t.TempDir()
	t.Setenv("OPPATH", runtimeHome)

	archiveName := "cpp-holons-v1.80.0-" + testTarget + ".tar.gz"
	archivePath := filepath.Join(t.TempDir(), archiveName)
	writeTestTarGz(t, archivePath, map[string]testTarEntry{
		"include/holons.grpc.pb.h": {Mode: 0o644, Body: []byte("/* holons */\n")},
		"lib/libholons_cpp.a":      {Mode: 0o644, Body: []byte("archive\n")},
		"bin/protoc":               {Mode: 0o755, Body: []byte("#!/bin/sh\n")},
	})
	archiveData, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	sum := sha256.Sum256(archiveData)
	expectedSHA := hex.EncodeToString(sum[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `[
  {
    "tag_name": "cpp-holons-v1.80.0",
    "draft": false,
    "assets": [
      {"name": "release-manifest.json", "browser_download_url": "%s/assets/release-manifest.json"}
    ]
  }
]`, serverURL(r))
		case "/assets/release-manifest.json":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{
  "schema": "sdk-prebuilts-release-manifest/v1",
  "sdk": "cpp",
  "version": "1.80.0",
  "tag": "cpp-holons-v1.80.0",
  "artifacts": [
    {
      "target": "%s",
      "archive": {
        "name": "%s",
        "url": "%s/assets/%s",
        "sha256": "%s"
      }
    }
  ]
}`, testTarget, archiveName, serverURL(r), archiveName, expectedSHA)
		case "/assets/" + archiveName:
			_, _ = w.Write(archiveData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv(releasesAPIEnv, server.URL+"/releases")

	prebuilt, notes, err := Install(context.Background(), InstallOptions{
		Lang:   "cpp",
		Target: testTarget,
	})
	if err != nil {
		t.Fatalf("Install() returned error: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("notes = %#v, want none", notes)
	}
	if prebuilt.Source != server.URL+"/assets/"+archiveName {
		t.Fatalf("source = %q", prebuilt.Source)
	}
	if prebuilt.ArchiveSHA256 != expectedSHA {
		t.Fatalf("archive sha = %q, want %q", prebuilt.ArchiveSHA256, expectedSHA)
	}
	if _, err := os.Stat(filepath.Join(prebuilt.Path, "bin/protoc")); err != nil {
		t.Fatalf("installed archive missing: %v", err)
	}
}

func TestNormalizeVersionRejectsPathTraversal(t *testing.T) {
	for _, version := range []string{"../1.80.0", `..\1.80.0`, ".", "1:80:0"} {
		if _, err := NormalizeVersion(version); err == nil {
			t.Fatalf("NormalizeVersion(%q) returned nil error", version)
		}
	}
}

func TestHostTripletForWindowsUsesTransitionalGNU(t *testing.T) {
	got, err := HostTripletFor("windows", "amd64", false)
	if err != nil {
		t.Fatalf("HostTripletFor() returned error: %v", err)
	}
	if got != "x86_64-windows-gnu" {
		t.Fatalf("HostTripletFor(windows, amd64) = %q, want x86_64-windows-gnu", got)
	}
}

type testTarEntry struct {
	Mode int64
	Body []byte
}

func writeTestTarGz(t *testing.T, path string, entries map[string]testTarEntry) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create tarball: %v", err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, entry := range entries {
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: entry.Mode,
			Size: int64(len(entry.Body)),
		}); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tw.Write(entry.Body); err != nil {
			t.Fatalf("write tar body: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close tarball: %v", err)
	}
}

func writeSHA256Sidecar(t *testing.T, path string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read tarball for sha256: %v", err)
	}
	sum := sha256.Sum256(data)
	if err := os.WriteFile(path+".sha256", []byte(hex.EncodeToString(sum[:])+"  "+filepath.Base(path)+"\n"), 0o644); err != nil {
		t.Fatalf("write sha256 sidecar: %v", err)
	}
}

func writeInstalledMetadata(t *testing.T, prebuilt Prebuilt) {
	t.Helper()

	if err := os.MkdirAll(prebuilt.Path, 0o755); err != nil {
		t.Fatalf("create install dir: %v", err)
	}
	data, err := json.Marshal(prebuilt)
	if err != nil {
		t.Fatalf("marshal prebuilt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(prebuilt.Path, metadataFile), data, 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}
}

func writeSharedProtocSource(t *testing.T, body string) (string, string) {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin", "protoc")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatalf("create protoc bin dir: %v", err)
	}
	if err := os.WriteFile(bin, []byte(body), 0o755); err != nil {
		t.Fatalf("write protoc: %v", err)
	}
	include := filepath.Join(root, "include", "google", "protobuf")
	if err := os.MkdirAll(include, 0o755); err != nil {
		t.Fatalf("create include dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(include, "empty.proto"), []byte("syntax = \"proto3\";\n"), 0o644); err != nil {
		t.Fatalf("write include file: %v", err)
	}
	sum := sha256.Sum256([]byte(body))
	return root, hex.EncodeToString(sum[:])
}

func seedSharedProtoc(t *testing.T, runtimeHome, version, target string) string {
	t.Helper()
	body := "seed-protoc"
	sum := sha256.Sum256([]byte(body))
	root := SharedProtocPath(version)
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatalf("create shared protoc bin: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "include"), 0o755); err != nil {
		t.Fatalf("create shared protoc include: %v", err)
	}
	if err := os.WriteFile(SharedProtocBinary(version, target), []byte(body), 0o755); err != nil {
		t.Fatalf("write shared protoc: %v", err)
	}
	_ = runtimeHome
	return hex.EncodeToString(sum[:])
}

func writeSDKTarballWithToolchain(t *testing.T, lang string, toolchain []ToolchainEntry) string {
	t.Helper()
	source := filepath.Join(t.TempDir(), lang+"-holons-v0.1.0-"+testTarget+".tar.gz")
	manifest := Prebuilt{
		Lang:        lang,
		Version:     "0.1.0",
		Target:      testTarget,
		SeedRelease: "test",
		Toolchain:   toolchain,
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	writeTestTarGz(t, source, map[string]testTarEntry{
		metadataFile: {Mode: 0o644, Body: data},
	})
	writeSHA256Sidecar(t, source)
	return source
}

func testSeedToolchainYAML(t *testing.T, lang, target, version, sha string) string {
	t.Helper()
	return fmt.Sprintf(`seed_release: "test"
protoc:
  upstream_tag: "v%s"
  version: "%s"
  required_by:
    %s: true
  sha256_per_target:
    %s: "%s"
cpp_runtime:
  protobuf_submodule_tag: "v%s"
plugins:
  %s:
    protoc-gen-grpc-java: "1.76.0"
`, version, version, lang, target, sha, version, lang)
}

func shellQuote(path string) string {
	return "'" + strings.ReplaceAll(path, "'", "'\"'\"'") + "'"
}

func toolchainContains(entries []ToolchainEntry, name, version string) bool {
	for _, entry := range entries {
		if entry.Name == name && entry.Version == version {
			return true
		}
	}
	return false
}

func testRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repo root not found")
		}
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

func targetOtherThan(t *testing.T, target string) string {
	t.Helper()

	for _, candidate := range AllowedTargets() {
		if candidate != target {
			return candidate
		}
	}
	t.Fatalf("no allowed target differs from %q", target)
	return ""
}

func newReleaseServer(t *testing.T, assets map[string][]byte) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if data, ok := assets[r.URL.Path]; ok {
			_, _ = w.Write(data)
			return
		}
		if r.URL.Path != "/releases" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `[
  {
    "tag_name": "zig-holons-v0.1.0",
    "draft": false,
    "assets": [
      {"name": "zig-holons-v0.1.0-%s.tar.gz", "browser_download_url": "%s/assets/zig-holons-v0.1.0-%s.tar.gz"},
      {"name": "zig-holons-v0.1.0-%s.tar.gz.sha256", "browser_download_url": "%s/assets/zig-holons-v0.1.0-%s.tar.gz.sha256"},
      {"name": "zig-holons-v0.1.0-%s-debug.tar.gz", "browser_download_url": "%s/assets/zig-holons-v0.1.0-%s-debug.tar.gz"}
    ]
  },
  {
    "tag_name": "zig-holons-v0.1.1",
    "draft": false,
    "assets": [
      {"name": "zig-holons-v0.1.1-%s.tar.gz", "browser_download_url": "%s/assets/zig-holons-v0.1.1-%s.tar.gz"},
      {"name": "zig-holons-v0.1.1-aarch64-apple-darwin.tar.gz", "browser_download_url": "%s/assets/zig-holons-v0.1.1-aarch64-apple-darwin.tar.gz"}
    ]
  },
  {
    "tag_name": "cpp-holons-v1.80.0",
    "draft": false,
    "assets": [
      {"name": "cpp-holons-v1.80.0-%s.tar.gz", "browser_download_url": "%s/assets/cpp-holons-v1.80.0-%s.tar.gz"}
    ]
  }
]`, testTarget, serverURL(r), testTarget, testTarget, serverURL(r), testTarget, testTarget, serverURL(r), testTarget, testTarget, serverURL(r), testTarget, serverURL(r), testTarget, serverURL(r), testTarget)
	}))
	return server
}

func serverURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
