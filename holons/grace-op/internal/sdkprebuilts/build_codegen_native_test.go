package sdkprebuilts

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeCodegenArchiveInstallsThroughSDKContract(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OPPATH", filepath.Join(root, "op"))
	stage := filepath.Join(root, "stage")
	if err := os.MkdirAll(filepath.Join(stage, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	const binary = "bin/protoc-gen-dart.exe"
	if err := os.WriteFile(filepath.Join(stage, filepath.FromSlash(binary)), []byte("plugin fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := Prebuilt{Lang: "dart", Version: "1.0.0", Target: "x86_64-windows-gnu", Codegen: &Codegen{Plugins: []CodegenPlugin{{Name: "dart", Binary: binary, OutSubdir: "dart"}}}}
	if err := writeMetadata(stage, manifest); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(root, "dart-holons-v1.0.0-x86_64-windows-gnu.tar.gz")
	if err := archiveNativeCodegen(stage, archive); err != nil {
		t.Fatal(err)
	}
	installed, _, err := Install(context.Background(), InstallOptions{Lang: "dart", Target: manifest.Target, Version: manifest.Version, Source: archive})
	if err != nil {
		t.Fatal(err)
	}
	if installed.Codegen == nil || len(installed.Codegen.Plugins) != 1 || installed.Codegen.Plugins[0].Binary != binary {
		t.Fatalf("installed manifest lost plugin: %+v", installed)
	}
	if data, err := os.ReadFile(filepath.Join(installed.Path, filepath.FromSlash(binary))); err != nil || string(data) != "plugin fixture" {
		t.Fatalf("installed plugin = %q, %v", data, err)
	}
}

func TestDartPluginEntrypointUsesPackageResolution(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	plugin := filepath.Join(root, "plugin with spaces")
	config := filepath.Join(project, ".dart_tool", "package_config.json")
	entrypoint := filepath.Join(plugin, "bin", "protoc_plugin.dart")
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(entrypoint), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entrypoint, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte(`{"packages":[{"name":"protoc_plugin","rootUri":"../../plugin%20with%20spaces/"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := dartPluginEntrypoint(config); err != nil || got != entrypoint {
		t.Fatalf("entrypoint = %q, %v; want %q", got, err, entrypoint)
	}
}
