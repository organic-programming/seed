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
	"github.com/organic-programming/grace-op/internal/progress"
	"github.com/organic-programming/grace-op/internal/testutil"
)

func TestRunnerRegistryAcceptsPythonDartAndRuby(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		setup    func(t *testing.T, root string)
	}{
		{
			name:     "python",
			manifest: "schema: holon/v0\nkind: composite\nbuild:\n  runner: python\nartifacts:\n  primary: app/main.py\n",
			setup: func(t *testing.T, root string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Join(root, "app"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "app", "main.py"), []byte("print('ok')\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:     "dart",
			manifest: "schema: holon/v0\nkind: native\nbuild:\n  runner: dart\nartifacts:\n  binary: dart-demo\n",
			setup: func(t *testing.T, root string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "pubspec.yaml"), []byte("name: demo\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "bin", "main.dart"), []byte("void main() {}\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:     "ruby",
			manifest: "schema: holon/v0\nkind: composite\nbuild:\n  runner: ruby\nartifacts:\n  primary: app/main.rb\n",
			setup: func(t *testing.T, root string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Join(root, "app"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "app", "main.rb"), []byte("puts 'ok'\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(t, root)
			writeRunnerManifest(t, root, tc.manifest)
			if _, err := LoadManifest(root); err != nil {
				t.Fatalf("LoadManifest failed: %v", err)
			}
		})
	}
}

func TestCargoRunnerDryRunBuild(t *testing.T) {
	root := t.TempDir()
	writeRunnerManifest(t, root, "schema: holon/v0\nkind: native\nbuild:\n  runner: cargo\nartifacts:\n  binary: cargo-demo\n")

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	var report Report
	err = (cargoRunner{}).build(manifest, BuildContext{
		Target:   canonicalRuntimeTarget(),
		Mode:     buildModeDebug,
		DryRun:   true,
		Progress: progress.Silence(),
	}, &report)
	if err != nil {
		t.Fatalf("cargo dry-run build failed: %v", err)
	}
	if len(report.Commands) != 1 || !strings.Contains(report.Commands[0], "cargo build --target-dir") {
		t.Fatalf("unexpected cargo commands: %v", report.Commands)
	}
}

func TestPythonRunnerDryRunBuildUsesFallbackInterpreter(t *testing.T) {
	root := t.TempDir()
	toolDir := t.TempDir()
	t.Setenv("PATH", toolDir)
	writeFakeCommand(t, toolDir, "python")
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "main.py"), []byte("print('ok')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "requirements.txt"), []byte("pytest\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRunnerManifest(t, root, "schema: holon/v0\nkind: native\nbuild:\n  runner: python\nartifacts:\n  binary: python-demo\n")

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	var report Report
	err = (pythonRunner{}).build(manifest, BuildContext{
		Target:   canonicalRuntimeTarget(),
		Mode:     buildModeDebug,
		DryRun:   true,
		Progress: progress.Silence(),
	}, &report)
	if err != nil {
		t.Fatalf("python dry-run build failed: %v", err)
	}
	if len(report.Commands) != 1 || !strings.Contains(report.Commands[0], "python -m pip install -r requirements.txt") {
		t.Fatalf("unexpected python commands: %v", report.Commands)
	}
}

func TestDartRunnerDryRunBuildUsesManagedBinaryOutput(t *testing.T) {
	root := t.TempDir()
	toolDir := t.TempDir()
	t.Setenv("PATH", toolDir)
	writeFakeCommand(t, toolDir, "dart")
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pubspec.yaml"), []byte("name: demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "main.dart"), []byte("void main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRunnerManifest(t, root, "schema: holon/v0\nkind: native\nbuild:\n  runner: dart\nartifacts:\n  binary: dart-demo\n")

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	var report Report
	err = (dartRunner{}).build(manifest, BuildContext{
		Target:   canonicalRuntimeTarget(),
		Mode:     buildModeDebug,
		DryRun:   true,
		Progress: progress.Silence(),
	}, &report)
	if err != nil {
		t.Fatalf("dart dry-run build failed: %v", err)
	}
	if len(report.Commands) != 2 {
		t.Fatalf("commands = %d, want 2", len(report.Commands))
	}
	if !strings.Contains(report.Commands[0], "dart pub get") {
		t.Fatalf("unexpected dart pub command: %v", report.Commands)
	}
	if !strings.Contains(report.Commands[1], "dart compile exe bin/main.dart -o ") || !strings.Contains(report.Commands[1], manifest.BinaryPath()) {
		t.Fatalf("unexpected dart compile command: %v", report.Commands)
	}
}

func TestPythonRunnerBuildEmbedsResolvedInterpreterPath(t *testing.T) {
	root := t.TempDir()
	toolDir := t.TempDir()
	t.Setenv("PATH", toolDir)
	writeFakeCommand(t, toolDir, "python3")
	pythonPath := filepath.Join(toolDir, "python3")
	if runtime.GOOS == "windows" {
		pythonPath += ".bat"
	}
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "main.py"), []byte("print('ok')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRunnerManifest(t, root, "schema: holon/v0\nkind: native\nbuild:\n  runner: python\nartifacts:\n  binary: python-demo\n")

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	var report Report
	err = (pythonRunner{}).build(manifest, BuildContext{
		Target:   canonicalRuntimeTarget(),
		Mode:     buildModeDebug,
		Progress: progress.Silence(),
	}, &report)
	if err != nil {
		t.Fatalf("python build failed: %v", err)
	}

	wrapper, err := os.ReadFile(manifest.BinaryPath())
	if err != nil {
		t.Fatalf("ReadFile(%q) failed: %v", manifest.BinaryPath(), err)
	}
	if !strings.Contains(string(wrapper), pythonPath) {
		t.Fatalf("wrapper missing resolved interpreter path %q: %s", pythonPath, string(wrapper))
	}
}

func TestRubyRunnerDryRunBuild(t *testing.T) {
	root := t.TempDir()
	toolDir := t.TempDir()
	t.Setenv("PATH", toolDir)
	writeFakeCommand(t, toolDir, "ruby")
	writeFakeCommand(t, toolDir, "bundle")
	if err := os.MkdirAll(filepath.Join(root, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Gemfile"), []byte("source 'https://example.test'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app", "main.rb"), []byte("puts 'ok'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRunnerManifest(t, root, "schema: holon/v0\nkind: composite\nbuild:\n  runner: ruby\nartifacts:\n  primary: app/main.rb\n")

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	var report Report
	err = (rubyRunner{}).build(manifest, BuildContext{
		Target:   canonicalRuntimeTarget(),
		Mode:     buildModeDebug,
		DryRun:   true,
		Progress: progress.Silence(),
	}, &report)
	if err != nil {
		t.Fatalf("ruby dry-run build failed: %v", err)
	}
	if len(report.Commands) < 3 || !hasEntryContaining(report.Commands, "bundle install") {
		t.Fatalf("unexpected ruby commands: %v", report.Commands)
	}
}

func TestRubyRunnerBuildCreatesNativeLauncher(t *testing.T) {
	root := t.TempDir()
	toolDir := t.TempDir()
	t.Setenv("PATH", toolDir)
	writeFakeCommand(t, toolDir, "env")
	writeFakeCommand(t, toolDir, "gem")
	writeFakeCommand(t, toolDir, "ruby")
	writeFakeCommand(t, toolDir, "bundle")
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Gemfile"), []byte("source 'https://example.test'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "main.rb"), []byte("puts 'ok'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRunnerManifest(t, root, "schema: holon/v0\nkind: native\nbuild:\n  runner: ruby\nartifacts:\n  binary: ruby-demo\n")

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	var report Report
	err = (rubyRunner{}).build(manifest, BuildContext{
		Target:   canonicalRuntimeTarget(),
		Mode:     buildModeDebug,
		Progress: progress.Silence(),
	}, &report)
	if err != nil {
		t.Fatalf("ruby build failed: %v", err)
	}

	launcher, err := os.ReadFile(manifest.BinaryPath())
	if err != nil {
		t.Fatalf("ReadFile(%s) failed: %v", manifest.BinaryPath(), err)
	}
	launcherText := string(launcher)
	if !strings.Contains(launcherText, filepath.Join(toolDir, "bundle")) || !strings.Contains(launcherText, filepath.Join(toolDir, "ruby")) {
		t.Fatalf("launcher missing bundle/ruby paths: %s", launcher)
	}
	if !strings.Contains(launcherText, filepath.Join(root, "bin", "main.rb")) {
		t.Fatalf("launcher missing entrypoint path: %s", launcher)
	}
}

func TestSwiftPackageRunnerDryRunBuild(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Package.swift"), []byte("// swift-tools-version: 6.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRunnerManifest(t, root, "schema: holon/v0\nkind: native\nbuild:\n  runner: swift-package\nartifacts:\n  binary: swift-demo\n")

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	var report Report
	err = (swiftPackageRunner{}).build(manifest, BuildContext{
		Target:   canonicalRuntimeTarget(),
		Mode:     buildModeRelease,
		DryRun:   true,
		Progress: progress.Silence(),
	}, &report)
	if err != nil {
		t.Fatalf("swift-package dry-run build failed: %v", err)
	}
	if len(report.Commands) != 1 || !strings.Contains(report.Commands[0], "swift build --build-path") {
		t.Fatalf("unexpected swift commands: %v", report.Commands)
	}
}

func TestSwiftPackageRunnerRecoversFromUnknownBuildDescription(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Package.swift"), []byte("// swift-tools-version: 6.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRunnerManifest(t, root, "schema: holon/v0\nkind: native\nbuild:\n  runner: swift-package\nartifacts:\n  binary: swift-demo\n")

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	originalRunCommand := swiftPackageRunCommand
	t.Cleanup(func() {
		swiftPackageRunCommand = originalRunCommand
	})

	buildPath := filepath.Join(manifest.OpRoot(), "build", "swift")
	buildCalls := 0
	var commands []string
	swiftPackageRunCommand = func(dir string, args []string) (string, error) {
		commands = append(commands, commandString(args))
		switch {
		case len(args) >= 2 && args[0] == "swift" && args[1] == "build":
			buildCalls++
			if buildCalls == 1 {
				return "error: unknown build description", fmt.Errorf("%s failed: exit status 1", commandString(args))
			}
			source := filepath.Join(buildPath, "debug", hostExecutableName(manifest.BinaryName()))
			if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
				t.Fatalf("mkdir build output: %v", err)
			}
			if err := os.WriteFile(source, []byte("swift-binary"), 0o755); err != nil {
				t.Fatalf("write build output: %v", err)
			}
			return "", nil
		case len(args) >= 3 && args[0] == "swift" && args[1] == "package" && args[2] == "clean":
			return "", nil
		default:
			t.Fatalf("unexpected command: %v", args)
			return "", nil
		}
	}

	var report Report
	err = (swiftPackageRunner{}).build(manifest, BuildContext{
		Target:   canonicalRuntimeTarget(),
		Mode:     buildModeDebug,
		Progress: progress.Silence(),
	}, &report)
	if err != nil {
		t.Fatalf("swift-package build recovery failed: %v", err)
	}
	if buildCalls != 2 {
		t.Fatalf("expected 2 swift build attempts, got %d", buildCalls)
	}
	if len(commands) != 3 {
		t.Fatalf("unexpected command sequence: %v", commands)
	}
	if !strings.Contains(commands[1], "swift package clean --build-path") {
		t.Fatalf("expected recovery clean command, got %v", commands)
	}
	if !strings.Contains(strings.Join(report.Notes, "\n"), "swift build cache reset after unknown build description") {
		t.Fatalf("expected recovery note, got %v", report.Notes)
	}
	if _, err := os.Stat(manifest.BinaryPath()); err != nil {
		t.Fatalf("expected recovered binary at %s: %v", manifest.BinaryPath(), err)
	}
}

func TestFlutterRunnerDryRunBuild(t *testing.T) {
	root := t.TempDir()
	writeRunnerManifest(t, root, "schema: holon/v0\nkind: composite\nbuild:\n  runner: flutter\nartifacts:\n  primary: build/macos/Build/Products/Debug/flutter-demo.app\n")

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	var report Report
	err = (flutterRunner{}).build(manifest, BuildContext{
		Target:   flutterTargetForTest(),
		Mode:     buildModeProfile,
		DryRun:   true,
		Progress: progress.Silence(),
	}, &report)
	if err != nil {
		t.Fatalf("flutter dry-run build failed: %v", err)
	}
	if len(report.Commands) != 1 || !strings.Contains(report.Commands[0], "flutter build") {
		t.Fatalf("unexpected flutter commands: %v", report.Commands)
	}
}

func TestNPMRunnerDryRunBuild(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte("{\"name\":\"demo\",\"scripts\":{\"build\":\"echo build\"}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRunnerManifest(t, root, "schema: holon/v0\nkind: native\nbuild:\n  runner: npm\nartifacts:\n  binary: npm-demo\n")

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	var report Report
	err = (npmRunner{}).build(manifest, BuildContext{
		Target:   canonicalRuntimeTarget(),
		Mode:     buildModeDebug,
		DryRun:   true,
		Progress: progress.Silence(),
	}, &report)
	if err != nil {
		t.Fatalf("npm dry-run build failed: %v", err)
	}
	if len(report.Commands) != 2 || !strings.Contains(report.Commands[0], "npm ci") || !strings.Contains(report.Commands[1], "npm run build") {
		t.Fatalf("unexpected npm commands: %v", report.Commands)
	}
}

func TestNPMRunnerBuildCreatesNativeLauncher(t *testing.T) {
	root := t.TempDir()
	toolDir := t.TempDir()
	t.Setenv("PATH", toolDir)

	logPath := filepath.Join(t.TempDir(), "node.log")
	writeRecordingNodeCommand(t, toolDir, logPath)
	writeFakeCommand(t, toolDir, "npm")

	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte("{\"name\":\"demo\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	entrypoint := filepath.Join(root, "dist", "npm-demo.js")
	if err := os.WriteFile(entrypoint, []byte("console.log('ok')\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRunnerManifest(t, root, "schema: holon/v0\nkind: native\nbuild:\n  runner: npm\nartifacts:\n  binary: npm-demo\n")

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	var report Report
	err = (npmRunner{}).build(manifest, BuildContext{
		Target:   canonicalRuntimeTarget(),
		Mode:     buildModeDebug,
		Progress: progress.Silence(),
	}, &report)
	if err != nil {
		t.Fatalf("npm build failed: %v", err)
	}

	launcher, err := os.ReadFile(manifest.BinaryPath())
	if err != nil {
		t.Fatalf("ReadFile(%s) failed: %v", manifest.BinaryPath(), err)
	}
	launcherText := string(launcher)

	nodePath := filepath.Join(toolDir, "node")
	if runtime.GOOS == "windows" {
		nodePath += ".bat"
	}
	if !strings.Contains(launcherText, nodePath) {
		t.Fatalf("launcher missing resolved node path %q: %s", nodePath, launcherText)
	}
	if !strings.Contains(launcherText, entrypoint) {
		t.Fatalf("launcher missing entrypoint path %q: %s", entrypoint, launcherText)
	}

	if runtime.GOOS == "windows" {
		return
	}

	copiedLauncher := filepath.Join(t.TempDir(), "npm-demo")
	if err := os.WriteFile(copiedLauncher, launcher, 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(copiedLauncher, "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("copied launcher failed: %v\n%s", err, output)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) failed: %v", logPath, err)
	}
	logText := string(logData)
	if !strings.Contains(logText, root) {
		t.Fatalf("node wrapper did not restore recipe cwd: %s", logText)
	}
	if !strings.Contains(logText, entrypoint) {
		t.Fatalf("node wrapper missing entrypoint invocation: %s", logText)
	}
	if !strings.Contains(logText, "version") {
		t.Fatalf("node wrapper missing forwarded args: %s", logText)
	}
}

func TestGradleRunnerDryRunBuildUsesWrapper(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "gradlew"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRunnerManifest(t, root, "schema: holon/v0\nkind: native\nbuild:\n  runner: gradle\nartifacts:\n  binary: gradle-demo\n")

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	var report Report
	err = (gradleRunner{}).build(manifest, BuildContext{
		Target:   canonicalRuntimeTarget(),
		Mode:     buildModeDebug,
		DryRun:   true,
		Progress: progress.Silence(),
	}, &report)
	if err != nil {
		t.Fatalf("gradle dry-run build failed: %v", err)
	}
	if len(report.Commands) != 1 || !strings.Contains(report.Commands[0], "./gradlew build installDist") {
		t.Fatalf("unexpected gradle commands: %v", report.Commands)
	}
}

func TestDotnetRunnerDryRunBuild(t *testing.T) {
	root := t.TempDir()
	writeRunnerManifest(t, root, "schema: holon/v0\nkind: native\nbuild:\n  runner: dotnet\nartifacts:\n  binary: dotnet-demo\n")

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	var report Report
	err = (dotnetRunner{}).build(manifest, BuildContext{
		Target:   canonicalRuntimeTarget(),
		Mode:     buildModeRelease,
		DryRun:   true,
		Progress: progress.Silence(),
	}, &report)
	if err != nil {
		t.Fatalf("dotnet dry-run build failed: %v", err)
	}
	if len(report.Commands) != 1 || !strings.Contains(report.Commands[0], "dotnet build -c Release -o") {
		t.Fatalf("unexpected dotnet commands: %v", report.Commands)
	}
}

func TestSyncDotnetArtifactsCopiesCompanionFiles(t *testing.T) {
	root := t.TempDir()
	writeRunnerManifest(t, root, "schema: holon/v0\nkind: native\nbuild:\n  runner: dotnet\nartifacts:\n  binary: dotnet-demo\n")

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	outputDir := filepath.Join(root, ".op", "build", "dotnet")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, contents := range map[string]string{
		"dotnet-demo":                    "apphost",
		"dotnet-demo.dll":                "assembly",
		"dotnet-demo.runtimeconfig.json": "{}",
		"dotnet-demo.deps.json":          "{}",
		"Holons.dll":                     "shared",
	} {
		if err := os.WriteFile(filepath.Join(outputDir, name), []byte(contents), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if err := syncDotnetArtifacts(manifest, outputDir); err != nil {
		t.Fatalf("syncDotnetArtifacts() failed: %v", err)
	}

	binDir := filepath.Dir(manifest.BinaryPath())
	for _, name := range []string{
		"dotnet-demo",
		"dotnet-demo.dll",
		"dotnet-demo.runtimeconfig.json",
		"dotnet-demo.deps.json",
		"Holons.dll",
	} {
		if !fileExists(filepath.Join(binDir, name)) {
			t.Fatalf("expected %s in build bin dir", name)
		}
	}
}

func TestSyncDotnetArtifactsCreatesLauncherWhenOnlyDLLExists(t *testing.T) {
	root := t.TempDir()
	writeRunnerManifest(t, root, "schema: holon/v0\nkind: native\nbuild:\n  runner: dotnet\nartifacts:\n  binary: dotnet-demo\n")

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	outputDir := filepath.Join(root, ".op", "build", "dotnet")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, contents := range map[string]string{
		"dotnet-demo.dll":                "assembly",
		"dotnet-demo.runtimeconfig.json": "{}",
		"dotnet-demo.deps.json":          "{}",
	} {
		if err := os.WriteFile(filepath.Join(outputDir, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := syncDotnetArtifacts(manifest, outputDir); err != nil {
		t.Fatalf("syncDotnetArtifacts() failed: %v", err)
	}

	launcher := manifest.BinaryPath()
	if !fileExists(launcher) {
		t.Fatalf("expected launcher at %s", launcher)
	}
	contents, err := os.ReadFile(launcher)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), `export PATH="/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin${PATH:+:$PATH}"`) {
		t.Fatalf("launcher contents missing PATH bootstrap: %s", contents)
	}
	if !strings.Contains(string(contents), `DOTNET_BIN=$(command -v dotnet 2>/dev/null || true)`) {
		t.Fatalf("launcher contents missing dotnet lookup: %s", contents)
	}
	if !strings.Contains(string(contents), `exec "$DOTNET_BIN" "$SCRIPT_DIR/dotnet-demo.dll" "$@"`) {
		t.Fatalf("launcher contents missing dotnet invocation: %s", contents)
	}
}

func TestDotnetProjectFileAndWorkloadDetection(t *testing.T) {
	root := t.TempDir()
	projectFile := filepath.Join(root, "demo.csproj")
	if err := os.WriteFile(projectFile, []byte("<Project><PropertyGroup><UseMaui>true</UseMaui></PropertyGroup></Project>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRunnerManifest(t, root, "schema: holon/v0\nkind: composite\nbuild:\n  runner: dotnet\nartifacts:\n  primary: bin/Debug/net8.0/Demo.app\n")

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	gotProject, err := dotnetProjectFile(manifest)
	if err != nil {
		t.Fatalf("dotnetProjectFile() failed: %v", err)
	}
	if gotProject != projectFile {
		t.Fatalf("project file = %q, want %q", gotProject, projectFile)
	}
	if workload := requiredDotnetWorkload(projectFile, "macos"); workload != "maui-maccatalyst" {
		t.Fatalf("requiredDotnetWorkload() = %q, want %q", workload, "maui-maccatalyst")
	}
}

func TestPythonInterpreterFallsBackToPython(t *testing.T) {
	toolDir := t.TempDir()
	t.Setenv("PATH", toolDir)
	writeFakeCommand(t, toolDir, "python")

	got, err := pythonInterpreter()
	if err != nil {
		t.Fatalf("pythonInterpreter() failed: %v", err)
	}
	if got != "python" {
		t.Fatalf("pythonInterpreter() = %q, want %q", got, "python")
	}
}

func TestDartEntrypointFallsBackToLibMain(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pubspec.yaml"), []byte("name: demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "lib", "main.dart"), []byte("void main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRunnerManifest(t, root, "schema: holon/v0\nkind: native\nbuild:\n  runner: dart\nartifacts:\n  binary: dart-demo\n")

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	got, err := dartEntrypoint(manifest)
	if err != nil {
		t.Fatalf("dartEntrypoint() failed: %v", err)
	}
	if got != "lib/main.dart" {
		t.Fatalf("dartEntrypoint() = %q, want %q", got, "lib/main.dart")
	}
}

func TestRubyRunnerCheckRequiresGemfile(t *testing.T) {
	root := t.TempDir()
	toolDir := t.TempDir()
	t.Setenv("PATH", toolDir)
	writeFakeCommand(t, toolDir, "ruby")
	writeFakeCommand(t, toolDir, "bundle")
	if err := os.MkdirAll(filepath.Join(root, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Gemfile"), []byte("source 'https://example.test'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app", "main.rb"), []byte("puts 'ok'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRunnerManifest(t, root, "schema: holon/v0\nkind: composite\nbuild:\n  runner: ruby\nartifacts:\n  primary: app/main.rb\n")

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}
	if err := (rubyRunner{}).check(manifest, BuildContext{}); err != nil {
		t.Fatalf("ruby check failed: %v", err)
	}
}

func TestPythonTestArgsPreferUnittestDiscover(t *testing.T) {
	root := t.TempDir()
	toolDir := t.TempDir()
	t.Setenv("PATH", toolDir)
	writeFakeCommand(t, toolDir, "python3")
	if err := os.MkdirAll(filepath.Join(root, "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRunnerManifest(t, root, "schema: holon/v0\nkind: composite\nbuild:\n  runner: python\nartifacts:\n  primary: app/main.py\n")

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	got, err := pythonTestArgs(manifest)
	if err != nil {
		t.Fatalf("pythonTestArgs() failed: %v", err)
	}
	if strings.Join(got, " ") != "python3 -m unittest discover" {
		t.Fatalf("pythonTestArgs() = %q", strings.Join(got, " "))
	}
}

func TestPythonTestArgsFallBackToPytest(t *testing.T) {
	toolDir := t.TempDir()
	t.Setenv("PATH", toolDir)
	writeFakeCommand(t, toolDir, "python")
	writeFakeCommand(t, toolDir, "pytest")
	root := t.TempDir()
	writeRunnerManifest(t, root, "schema: holon/v0\nkind: composite\nbuild:\n  runner: python\nartifacts:\n  primary: app/main.py\n")

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	got, err := pythonTestArgs(manifest)
	if err != nil {
		t.Fatalf("pythonTestArgs() failed: %v", err)
	}
	if strings.Join(got, " ") != "pytest" {
		t.Fatalf("pythonTestArgs() = %q", strings.Join(got, " "))
	}
}

func TestRubyTestArgsPreferRSpec(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "spec"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRunnerManifest(t, root, "schema: holon/v0\nkind: composite\nbuild:\n  runner: ruby\nartifacts:\n  primary: app/main.rb\n")

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	got, err := rubyTestArgs(manifest)
	if err != nil {
		t.Fatalf("rubyTestArgs() failed: %v", err)
	}
	if !strings.HasSuffix(strings.Join(got, " "), "bundle exec rspec") {
		t.Fatalf("rubyTestArgs() = %q", strings.Join(got, " "))
	}
}

func TestRubyTestArgsFallBackToRakeTest(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Rakefile"), []byte("task :test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRunnerManifest(t, root, "schema: holon/v0\nkind: composite\nbuild:\n  runner: ruby\nartifacts:\n  primary: app/main.rb\n")

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	got, err := rubyTestArgs(manifest)
	if err != nil {
		t.Fatalf("rubyTestArgs() failed: %v", err)
	}
	if !strings.HasSuffix(strings.Join(got, " "), "bundle exec rake test") {
		t.Fatalf("rubyTestArgs() = %q", strings.Join(got, " "))
	}
}

func TestRubyEntrypointPrefersBinMain(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "main.rb"), []byte("puts 'ok'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRunnerManifest(t, root, "schema: holon/v0\nkind: native\nbuild:\n  runner: ruby\nartifacts:\n  binary: ruby-demo\n")

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	got, err := rubyEntrypoint(manifest)
	if err != nil {
		t.Fatalf("rubyEntrypoint() failed: %v", err)
	}
	if got != "bin/main.rb" {
		t.Fatalf("rubyEntrypoint() = %q, want %q", got, "bin/main.rb")
	}
}

func TestRubyRunnerBuildWritesUTF8Wrapper(t *testing.T) {
	toolDir := t.TempDir()
	writeFakeCommand(t, toolDir, "bundle")
	writeFakeCommand(t, toolDir, "ruby")
	writeFakeCommand(t, toolDir, "gem")
	t.Setenv("PATH", toolDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Gemfile"), []byte("source 'https://example.com'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "main.rb"), []byte("puts 'ok'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRunnerManifest(t, root, "schema: holon/v0\nkind: native\nbuild:\n  runner: ruby\nartifacts:\n  binary: ruby-demo\n")

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	var report Report
	if err := (rubyRunner{}).build(manifest, BuildContext{
		Target:   canonicalRuntimeTarget(),
		Mode:     buildModeDebug,
		Progress: progress.Silence(),
	}, &report); err != nil {
		t.Fatalf("ruby build failed: %v", err)
	}

	contents, err := os.ReadFile(manifest.BinaryPath())
	if err != nil {
		t.Fatal(err)
	}
	wrapper := string(contents)
	if !strings.Contains(wrapper, `export PATH="/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin${PATH:+:$PATH}"`) {
		t.Fatalf("wrapper missing PATH bootstrap: %s", wrapper)
	}
	if !strings.Contains(wrapper, `export LANG=en_US.UTF-8`) {
		t.Fatalf("wrapper missing LANG bootstrap: %s", wrapper)
	}
	if !strings.Contains(wrapper, `export LC_ALL="$LANG"`) {
		t.Fatalf("wrapper missing LC_ALL bootstrap: %s", wrapper)
	}
}

func TestPythonDartAndRubyRunnersRejectCrossTargetBuilds(t *testing.T) {
	tests := []struct {
		name     string
		runner   runner
		manifest string
		setup    func(t *testing.T, root string)
		wantErr  string
	}{
		{
			name:     "python",
			runner:   pythonRunner{},
			manifest: "schema: holon/v0\nkind: composite\nbuild:\n  runner: python\nartifacts:\n  primary: app/main.py\n",
			setup: func(t *testing.T, root string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Join(root, "app"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "app", "main.py"), []byte("print('ok')\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantErr: "python cross-target build not implemented",
		},
		{
			name:     "dart",
			runner:   dartRunner{},
			manifest: "schema: holon/v0\nkind: native\nbuild:\n  runner: dart\nartifacts:\n  binary: dart-demo\n",
			setup: func(t *testing.T, root string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "pubspec.yaml"), []byte("name: demo\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "bin", "main.dart"), []byte("void main() {}\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantErr: "dart cross-target build not implemented",
		},
		{
			name:     "ruby",
			runner:   rubyRunner{},
			manifest: "schema: holon/v0\nkind: composite\nbuild:\n  runner: ruby\nartifacts:\n  primary: app/main.rb\n",
			setup: func(t *testing.T, root string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Join(root, "app"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "app", "main.rb"), []byte("puts 'ok'\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantErr: "ruby cross-target build not implemented",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(t, root)
			writeRunnerManifest(t, root, tc.manifest)
			manifest, err := LoadManifest(root)
			if err != nil {
				t.Fatalf("LoadManifest failed: %v", err)
			}

			var report Report
			err = tc.runner.build(manifest, BuildContext{
				Target:   unsupportedTargetForHost(),
				Mode:     buildModeDebug,
				DryRun:   true,
				Progress: progress.Silence(),
			}, &report)
			if err == nil {
				t.Fatal("expected cross-target error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestQtCMakeRunnerDryRunBuildUsesQt6Dir(t *testing.T) {
	root := t.TempDir()
	qtDir := filepath.Join(root, "qt", "lib", "cmake", "Qt6")
	if err := os.MkdirAll(qtDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("Qt6_DIR", qtDir)
	if err := os.WriteFile(filepath.Join(root, "CMakeLists.txt"), []byte("cmake_minimum_required(VERSION 3.20)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRunnerManifest(t, root, "schema: holon/v0\nkind: composite\nbuild:\n  runner: qt-cmake\nartifacts:\n  primary: build/qt-demo.app\n")

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	var report Report
	err = (qtCMakeRunner{}).build(manifest, BuildContext{
		Target:   canonicalRuntimeTarget(),
		Mode:     buildModeDebug,
		DryRun:   true,
		Progress: progress.Silence(),
	}, &report)
	if err != nil {
		t.Fatalf("qt-cmake dry-run build failed: %v", err)
	}
	if len(report.Commands) != 2 || !strings.Contains(report.Commands[0], "-DCMAKE_PREFIX_PATH="+qtDir) {
		t.Fatalf("unexpected qt-cmake commands: %v", report.Commands)
	}
}

func writeRunnerManifest(t *testing.T, dir, yaml string) {
	t.Helper()
	if err := testutil.WriteManifestFile(filepath.Join(dir, identity.ManifestFileName), yaml); err != nil {
		t.Fatal(err)
	}
}

func flutterTargetForTest() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	case "linux":
		return "linux"
	case "windows":
		return "windows"
	default:
		return canonicalRuntimeTarget()
	}
}

func unsupportedTargetForHost() string {
	switch canonicalRuntimeTarget() {
	case "linux":
		return "windows"
	default:
		return "linux"
	}
}

func writeFakeCommand(t *testing.T, dir, name string) {
	t.Helper()

	path := filepath.Join(dir, name)
	data := []byte("#!/bin/sh\nexit 0\n")
	mode := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		path += ".bat"
		data = []byte("@echo off\r\nexit /b 0\r\n")
		mode = 0o644
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func writeRecordingNodeCommand(t *testing.T, dir, logPath string) {
	t.Helper()

	if runtime.GOOS == "windows" {
		writeFakeCommand(t, dir, "node")
		return
	}

	path := filepath.Join(dir, "node")
	data := []byte("#!/bin/sh\nset -eu\nprintf '%s\\n' \"$PWD\" > " + shellQuote(logPath) + "\nfor arg in \"$@\"; do\n  printf '%s\\n' \"$arg\" >> " + shellQuote(logPath) + "\ndone\n")
	if err := os.WriteFile(path, data, 0o755); err != nil {
		t.Fatal(err)
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
