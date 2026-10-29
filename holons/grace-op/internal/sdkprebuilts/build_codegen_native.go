package sdkprebuilts

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/organic-programming/go-holons/pkg/fileuri"
	"github.com/organic-programming/grace-op/internal/buildenv"
)

// buildWindowsCodegen builds actual native plugin images from the pinned
// sources. Windows never needs a shell launcher, Bash, tar or symlink privileges.
// The caller still installs and verifies the normal SDK archive contract.
func buildWindowsCodegen(ctx context.Context, repoRoot, lang, target, version string, toolchain []ToolchainEntry, opts BuildOptions) error {
	if target != "x86_64-windows-gnu" && target != "x86_64-pc-windows-msvc" {
		return fmt.Errorf("native Windows %s codegen builder requires a Windows x64 target, got %s", lang, target)
	}
	environment, err := buildenv.Prepare(repoRoot)
	if err != nil {
		return err
	}
	work := filepath.Join(repoRoot, "sdk", lang+"-holons", ".codegen-prebuilt", target)
	if err := os.MkdirAll(work, 0o755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(work, "stage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	bin := filepath.Join(stage, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return err
	}
	pluginVersion := func(name string) (string, error) {
		for _, entry := range toolchain {
			if entry.Name == name && entry.Version != "" {
				return entry.Version, nil
			}
		}
		return "", fmt.Errorf("%s missing pinned %s version", seedToolchainFile, name)
	}
	run := func(dir, program string, args []string, env ...string) error {
		resolved, err := buildenv.Lookup(environment, program)
		if err != nil {
			return err
		}
		cmd := exec.CommandContext(ctx, resolved, args...)
		cmd.Dir = dir
		base := environment
		if base == nil {
			base = os.Environ()
		}
		cmd.Env = buildenv.Merge(base, env)
		cmd.Stdout = orDevNull(opts.Stdout)
		tail := newTailBuffer(40)
		cmd.Stderr = io.MultiWriter(orDevNull(opts.Stderr), tail)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s %v: %w\n%s", program, args, err, tail.String())
		}
		return nil
	}
	plugins := []CodegenPlugin{}
	switch lang {
	case "go":
		for _, plugin := range []struct{ name, source, output string }{
			{"protoc-gen-go", "google.golang.org/protobuf/cmd/protoc-gen-go", "go"},
			{"protoc-gen-go-grpc", "google.golang.org/grpc/cmd/protoc-gen-go-grpc", "go-grpc"},
		} {
			pinned, err := pluginVersion(plugin.name)
			if err != nil {
				return err
			}
			if err := run(repoRoot, "go", []string{"install", plugin.source + "@" + pinned}, "GOWORK=off", "CGO_ENABLED=0", "GOOS=windows", "GOARCH=amd64", "GOBIN="+bin); err != nil {
				return err
			}
			plugins = append(plugins, CodegenPlugin{Name: plugin.output, Binary: "bin/" + plugin.name + ".exe", OutSubdir: "go"})
		}
	case "dart":
		pinned, err := pluginVersion("protoc-gen-dart")
		if err != nil {
			return err
		}
		dart, err := nativeDartExecutable(environment)
		if err != nil {
			return err
		}
		project := filepath.Join(work, "dart-plugin")
		if err := os.MkdirAll(project, 0o755); err != nil {
			return err
		}
		pubspec := fmt.Sprintf("name: op_codegen_plugin_builder\nenvironment:\n  sdk: '>=3.0.0 <4.0.0'\ndependencies:\n  protoc_plugin: '%s'\n", pinned)
		if err := os.WriteFile(filepath.Join(project, "pubspec.yaml"), []byte(pubspec), 0o644); err != nil {
			return err
		}
		if err := run(project, dart, []string{"pub", "get"}); err != nil {
			return err
		}
		config := filepath.Join(project, ".dart_tool", "package_config.json")
		entrypoint, err := dartPluginEntrypoint(config)
		if err != nil {
			return err
		}
		if err := run(project, dart, []string{"compile", "exe", "--packages=" + config, "-o", filepath.Join(bin, "protoc-gen-dart.exe"), entrypoint}); err != nil {
			return err
		}
		plugins = append(plugins, CodegenPlugin{Name: "dart", Binary: "bin/protoc-gen-dart.exe", OutSubdir: "dart"})
	default:
		return fmt.Errorf("unsupported native Windows codegen SDK: %s", lang)
	}
	seed, err := LoadSeedToolchain(repoRoot)
	if err != nil {
		return err
	}
	manifest := Prebuilt{Lang: lang, Version: version, Target: target, SeedRelease: seed.SeedRelease, Codegen: &Codegen{Plugins: plugins}, Toolchain: toolchain}
	if err := writeMetadata(stage, manifest); err != nil {
		return err
	}
	archive := filepath.Join(repoRoot, "dist", "sdk-prebuilts", lang, target, fmt.Sprintf("%s-holons-v%s-%s.tar.gz", lang, version, target))
	return archiveNativeCodegen(stage, archive)
}

func nativeDartExecutable(environment ...[]string) (string, error) {
	var prepared []string
	if len(environment) != 0 {
		prepared = environment[0]
	}
	path, err := buildenv.Lookup(prepared, "dart")
	if err != nil {
		return "", err
	}
	if strings.EqualFold(filepath.Ext(path), ".exe") {
		return path, nil
	}
	// Flutter exposes dart.bat on PATH. Use its actual VM directly so no shell
	// command quoting or batch launcher becomes part of the protoc contract.
	native := filepath.Join(filepath.Dir(path), "cache", "dart-sdk", "bin", "dart.exe")
	if info, err := os.Stat(native); err == nil && !info.IsDir() {
		return native, nil
	}
	return "", fmt.Errorf("native dart.exe not found beside %s; add the Dart SDK bin directory to PATH", path)
}

func dartPluginEntrypoint(config string) (string, error) {
	data, err := os.ReadFile(config)
	if err != nil {
		return "", err
	}
	var packages struct {
		Packages []struct {
			Name    string `json:"name"`
			RootURI string `json:"rootUri"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(data, &packages); err != nil {
		return "", err
	}
	base, err := url.Parse(fileuri.FromPath(config))
	if err != nil {
		return "", err
	}
	for _, pkg := range packages.Packages {
		if pkg.Name != "protoc_plugin" {
			continue
		}
		root, err := url.Parse(pkg.RootURI)
		if err != nil {
			return "", err
		}
		path, err := fileuri.ToPath(base.ResolveReference(root).String())
		if err != nil {
			return "", err
		}
		entrypoint := filepath.Join(path, "bin", "protoc_plugin.dart")
		if info, err := os.Stat(entrypoint); err != nil || info.IsDir() {
			return "", fmt.Errorf("Dart plugin entrypoint missing: %s", entrypoint)
		}
		return entrypoint, nil
	}
	return "", fmt.Errorf("protoc_plugin not found in %s", config)
}

func archiveNativeCodegen(stage, archive string) error {
	if err := os.MkdirAll(filepath.Dir(archive), 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(archive), ".codegen-*.tar.gz")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	zipper := gzip.NewWriter(file)
	writer := tar.NewWriter(zipper)
	err = filepath.WalkDir(stage, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == stage {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() && !info.IsDir() {
			return fmt.Errorf("unsupported native codegen artifact %s", path)
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(stage, path)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		if strings.EqualFold(filepath.Ext(path), ".exe") {
			header.Mode = 0o755
		}
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		defer input.Close()
		_, err = io.Copy(writer, input)
		return err
	})
	closeTar, closeGzip, closeFile := writer.Close(), zipper.Close(), file.Close()
	for _, candidate := range []error{err, closeTar, closeGzip, closeFile} {
		if candidate != nil {
			return candidate
		}
	}
	if err := os.Remove(archive); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(file.Name(), archive); err != nil {
		return err
	}
	sum, err := fileSHA256(archive)
	if err != nil {
		return err
	}
	return os.WriteFile(archive+".sha256", []byte(sum+"  "+filepath.Base(archive)+"\n"), 0o644)
}
