package holons

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/organic-programming/grace-op/internal/sdkprebuilts"
	"google.golang.org/protobuf/proto"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestMain(m *testing.M) {
	if os.Getenv("OP_TEST_CODEGEN_PLUGIN") == "1" {
		runCodegenTestPlugin()
		return
	}
	os.Exit(m.Run())
}

func TestExecuteLifecycleCodegenWritesPluginOutputs(t *testing.T) {
	if _, err := execLookPath("go"); err != nil {
		t.Skip("go command not available")
	}

	root := t.TempDir()
	chdirForHolonTest(t, root)
	runtimeHome := filepath.Join(root, ".op-runtime")
	t.Setenv("OPPATH", runtimeHome)
	t.Setenv("OP_TEST_CODEGEN_PLUGIN", "1")

	writeInstalledCodegenDistribution(t, runtimeHome, "test", "0.1.0", []codegenDistributionPlugin{{
		Name:      "test",
		Binary:    "bin/protoc-gen-test",
		OutSubdir: "test",
	}})
	dir := writeProtoCodegenHolonFixture(t, root, "demo-codegen", []string{"test"})

	report, err := ExecuteLifecycle(OperationBuild, dir)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if !hasEntryContaining(report.Notes, "codegen: wrote") {
		t.Fatalf("report notes missing codegen write: %v", report.Notes)
	}

	outputPath := filepath.Join(dir, "gen", "test", "api", "v1", "holon.codegen.txt")
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read generated output: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "files=api/v1/holon.proto") {
		t.Fatalf("generated output missing file_to_generate: %q", content)
	}

	cachePath := filepath.Join(dir, ".op", codegenManifestName)
	cacheData, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read codegen manifest: %v", err)
	}
	if !strings.Contains(string(cacheData), "gen/test/api/v1/holon.codegen.txt") {
		t.Fatalf("codegen manifest missing output path: %s", string(cacheData))
	}
}

func TestExecuteLifecycleCodegenMissingDistributionIsActionable(t *testing.T) {
	root := t.TempDir()
	chdirForHolonTest(t, root)
	t.Setenv("OPPATH", filepath.Join(root, ".op-runtime"))

	dir := writeProtoCodegenHolonFixture(t, root, "demo-missing-codegen", []string{"ghost"})
	_, err := ExecuteLifecycle(OperationBuild, dir, BuildOptions{DryRun: true})
	if err == nil {
		t.Fatal("expected missing codegen distribution error")
	}
	if !strings.Contains(err.Error(), "missing distribution for codegen") {
		t.Fatalf("error missing category: %v", err)
	}
	if !strings.Contains(err.Error(), "op sdk install ghost") {
		t.Fatalf("error missing install action: %v", err)
	}
}

func TestExecuteLifecycleCodegenUnsupportedLanguageListsPlugins(t *testing.T) {
	root := t.TempDir()
	chdirForHolonTest(t, root)
	runtimeHome := filepath.Join(root, ".op-runtime")
	t.Setenv("OPPATH", runtimeHome)

	writeInstalledCodegenDistribution(t, runtimeHome, "test", "0.1.0", []codegenDistributionPlugin{{
		Name:      "other",
		Binary:    "bin/protoc-gen-other",
		OutSubdir: "test",
	}})
	dir := writeProtoCodegenHolonFixture(t, root, "demo-unsupported-codegen", []string{"test"})

	_, err := ExecuteLifecycle(OperationBuild, dir, BuildOptions{DryRun: true})
	if err == nil {
		t.Fatal("expected unsupported codegen language error")
	}
	if !strings.Contains(err.Error(), "unsupported codegen language") {
		t.Fatalf("error missing category: %v", err)
	}
	if !strings.Contains(err.Error(), "other") {
		t.Fatalf("error missing declared plugin list: %v", err)
	}
}

func TestLoadManifestRejectsCodegenWithLegacyBeforeCommand(t *testing.T) {
	root := t.TempDir()
	dir := writeProtoCodegenHolonFixtureWithBuildExtra(t, root, "demo-legacy-codegen", []string{"go"}, `    before_commands: {
      cwd: "."
      argv: ["protoc", "--go_out=gen/go", "api/v1/holon.proto"]
    }
`)

	_, err := LoadManifest(dir)
	if err == nil {
		t.Fatal("expected legacy before_commands/codegen conflict")
	}
	if !strings.Contains(err.Error(), "build.codegen.languages") {
		t.Fatalf("error missing codegen field: %v", err)
	}
	if !strings.Contains(err.Error(), "build.before_commands") {
		t.Fatalf("error missing before_commands field: %v", err)
	}
}

func TestCodegenDescriptorFileOrderEmitsImportsBeforeImporters(t *testing.T) {
	files := map[string]*descriptorpb.FileDescriptorProto{
		"api/v1/holon.proto": {
			Name:       proto.String("api/v1/holon.proto"),
			Dependency: []string{"holons/v1/manifest.proto", "v1/greeting.proto"},
		},
		"google/protobuf/empty.proto": {
			Name: proto.String("google/protobuf/empty.proto"),
		},
		"holons/v1/manifest.proto": {
			Name: proto.String("holons/v1/manifest.proto"),
		},
		"v1/greeting.proto": {
			Name:       proto.String("v1/greeting.proto"),
			Dependency: []string{"google/protobuf/empty.proto"},
		},
	}

	got := codegenDescriptorFileOrder(files)
	positions := make(map[string]int, len(got))
	for i, path := range got {
		positions[path] = i
	}

	for importer, deps := range map[string][]string{
		"api/v1/holon.proto": {"holons/v1/manifest.proto", "v1/greeting.proto"},
		"v1/greeting.proto":  {"google/protobuf/empty.proto"},
	} {
		for _, dep := range deps {
			if positions[dep] > positions[importer] {
				t.Fatalf("descriptor order = %v; dependency %s appears after importer %s", got, dep, importer)
			}
		}
	}
}

func TestCodegenRequestBytesForPluginRewritesLegacySharedProtoPaths(t *testing.T) {
	req := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{"v1/greeting.proto"},
		ProtoFile: []*descriptorpb.FileDescriptorProto{{
			Name:    proto.String("v1/greeting.proto"),
			Package: proto.String("greeting.v1"),
		}, {
			Name:       proto.String("api/v1/holon.proto"),
			Package:    proto.String("greeting.v1"),
			Dependency: []string{"v1/greeting.proto"},
		}},
	}
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}

	for _, pluginName := range []string{"go", "python"} {
		t.Run(pluginName, func(t *testing.T) {
			gotBytes, err := codegenRequestBytesForPlugin(reqBytes, resolvedCodegenPlugin{
				Name:        pluginName,
				PathRewrite: codegenPathRewriteRequestLegacy,
			})
			if err != nil {
				t.Fatalf("rewrite request: %v", err)
			}

			got := &pluginpb.CodeGeneratorRequest{}
			if err := proto.Unmarshal(gotBytes, got); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if len(got.FileToGenerate) != 1 || got.FileToGenerate[0] != "greeting/v1/greeting.proto" {
				t.Fatalf("FileToGenerate = %v, want greeting/v1/greeting.proto", got.FileToGenerate)
			}
			if !requestHasProtoPath(got, "greeting/v1/greeting.proto") {
				t.Fatalf("rewritten descriptor missing: %v", descriptorNames(got))
			}
			if !requestHasProtoDependency(got, "v1/holon.proto", "greeting/v1/greeting.proto") {
				t.Fatalf("rewritten dependency missing from v1/holon.proto")
			}
		})
	}
}

func TestLegacyCodegenOutputPathRewriteForPython(t *testing.T) {
	files := map[string]*descriptorpb.FileDescriptorProto{
		"v1/greeting.proto": {
			Name:    proto.String("v1/greeting.proto"),
			Package: proto.String("greeting.v1"),
		},
	}
	rewrites := legacyCodegenOutputRewrites(files, []string{"v1/greeting.proto"})

	got := rewriteLegacyCodegenOutputPath("v1/greeting_pb2.py", rewrites)
	if got != "greeting/v1/greeting_pb2.py" {
		t.Fatalf("python output path = %q, want greeting/v1/greeting_pb2.py", got)
	}
}

func TestLegacyCodegenOutputPathRewriteForSwiftGRPC(t *testing.T) {
	files := map[string]*descriptorpb.FileDescriptorProto{
		"v1/greeting.proto": {
			Name:    proto.String("v1/greeting.proto"),
			Package: proto.String("greeting.v1"),
		},
	}
	rewrites := legacyCodegenOutputRewrites(files, []string{"v1/greeting.proto"})

	got := rewriteLegacyCodegenOutputPath("v1/greeting.grpc.swift", rewrites)
	if got != "greeting/v1/greeting.grpc.swift" {
		t.Fatalf("swift-grpc output path = %q, want greeting/v1/greeting.grpc.swift", got)
	}
	if got := codegenPluginPathRewriteMode("swift-grpc"); got != codegenPathRewriteOutputLegacy {
		t.Fatalf("swift-grpc path rewrite mode = %q, want %q", got, codegenPathRewriteOutputLegacy)
	}
}

func TestCodegenRequestBytesForPluginUsesParameters(t *testing.T) {
	req := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{"v1/greeting.proto"},
		ProtoFile: []*descriptorpb.FileDescriptorProto{{
			Name:    proto.String("v1/greeting.proto"),
			Package: proto.String("greeting.v1"),
		}},
	}
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		plugin string
		want   string
	}{
		{plugin: "dart", want: "grpc"},
		{plugin: "swift", want: "Visibility=Public"},
		{plugin: "swift-grpc", want: "Visibility=Public"},
	} {
		t.Run(tc.plugin, func(t *testing.T) {
			gotBytes, err := codegenRequestBytesForPlugin(reqBytes, resolvedCodegenPlugin{
				Name:      tc.plugin,
				Parameter: codegenPluginParameter(tc.plugin),
			})
			if err != nil {
				t.Fatalf("set parameter: %v", err)
			}

			got := &pluginpb.CodeGeneratorRequest{}
			if err := proto.Unmarshal(gotBytes, got); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if got.GetParameter() != tc.want {
				t.Fatalf("%s parameter = %q, want %q", tc.plugin, got.GetParameter(), tc.want)
			}
		})
	}
}

func runCodegenTestPlugin() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	req := &pluginpb.CodeGeneratorRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	resp := &pluginpb.CodeGeneratorResponse{}
	for _, file := range req.GetFileToGenerate() {
		name := strings.TrimSuffix(file, ".proto") + ".codegen.txt"
		content := fmt.Sprintf("files=%s\nproto_files=%d\n", strings.Join(req.GetFileToGenerate(), ","), len(req.GetProtoFile()))
		resp.File = append(resp.File, &pluginpb.CodeGeneratorResponse_File{
			Name:    &name,
			Content: &content,
		})
	}
	out, err := proto.Marshal(resp)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err := os.Stdout.Write(out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func requestHasProtoPath(req *pluginpb.CodeGeneratorRequest, path string) bool {
	for _, file := range req.GetProtoFile() {
		if file.GetName() == path {
			return true
		}
	}
	return false
}

func requestHasProtoDependency(req *pluginpb.CodeGeneratorRequest, filePath, dependency string) bool {
	for _, file := range req.GetProtoFile() {
		if file.GetName() != filePath {
			continue
		}
		for _, candidate := range file.Dependency {
			if candidate == dependency {
				return true
			}
		}
	}
	return false
}

func descriptorNames(req *pluginpb.CodeGeneratorRequest) []string {
	names := make([]string, 0, len(req.GetProtoFile()))
	for _, file := range req.GetProtoFile() {
		names = append(names, file.GetName())
	}
	return names
}

func writeInstalledCodegenDistribution(t *testing.T, runtimeHome, sdk, version string, plugins []codegenDistributionPlugin) string {
	t.Helper()

	hostTarget, err := sdkprebuilts.HostTriplet()
	if err != nil {
		t.Fatalf("host triplet: %v", err)
	}
	root := filepath.Join(runtimeHome, "sdk", sdk, version, hostTarget)
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}

	testBinary, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatalf("read test binary: %v", err)
	}
	for _, plugin := range plugins {
		binaryPath := filepath.Join(root, filepath.FromSlash(plugin.Binary))
		if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(binaryPath, testBinary, 0o755); err != nil {
			t.Fatalf("write test plugin binary: %v", err)
		}
	}

	manifest := codegenDistributionManifest{
		Lang:    sdk,
		Version: version,
		Target:  hostTarget,
	}
	manifest.Codegen.Plugins = plugins
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeProtoCodegenHolonFixture(t *testing.T, root, name string, languages []string) string {
	return writeProtoCodegenHolonFixtureWithBuildExtra(t, root, name, languages, "")
}

func writeProtoCodegenHolonFixtureWithBuildExtra(t *testing.T, root, name string, languages []string, buildExtra string) string {
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

	var languageLines strings.Builder
	for _, language := range languages {
		fmt.Fprintf(&languageLines, "      languages: %q\n", language)
	}

	protoSource := fmt.Sprintf(`syntax = "proto3";

package test.v1;

import "holons/v1/manifest.proto";

option (holons.v1.manifest) = {
  identity: {
    schema: "holon/v1"
    uuid: "%s-uuid"
    given_name: "Demo"
    family_name: "Codegen"
    motto: "Codegen-backed test holon."
    composer: "test"
    status: "draft"
    born: "2026-04-28"
  }
  kind: "native"
  lang: "go"
  build: {
    runner: "go-module"
    main: "./cmd/%s"
    codegen: {
%s    }
%s
  }
  requires: {
    commands: ["go"]
    files: ["go.mod"]
  }
  artifacts: {
    binary: "%s"
  }
};
`, name, name, languageLines.String(), buildExtra, name)
	if err := os.WriteFile(filepath.Join(dir, "api", "v1", "holon.proto"), []byte(protoSource), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}
