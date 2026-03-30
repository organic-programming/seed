package scaffold

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/organic-programming/grace-op/internal/identity"
)

func TestListIncludesTemplatesAndCompositeAliases(t *testing.T) {
	entries, err := List()
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}

	names := make(map[string]Entry, len(entries))
	for _, entry := range entries {
		names[entry.Name] = entry
	}

	for _, name := range []string{"go-daemon", "hostui-web", "wrapper-cli", "composition-direct-call", "composite-go-swiftui"} {
		if _, ok := names[name]; !ok {
			t.Fatalf("template %q missing from catalog", name)
		}
	}
}

func TestGenerateGoDaemonAppliesOverrides(t *testing.T) {
	root := t.TempDir()

	result, err := Generate("go-daemon", "alpha-builder", GenerateOptions{
		Dir: root,
		Overrides: map[string]string{
			"service": "EchoService",
		},
	})
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	if result.Template != "go-daemon" {
		t.Fatalf("result.Template = %q, want %q", result.Template, "go-daemon")
	}

	mainPath := filepath.Join(root, "alpha-builder", "cmd", "alpha-builder", "main.go")
	data, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) failed: %v", mainPath, err)
	}
	if !strings.Contains(string(data), "EchoService ready for alpha-builder") {
		t.Fatalf("main.go missing overridden service: %s", string(data))
	}

	manifestPath := filepath.Join(root, "alpha-builder", identity.ManifestFileName)
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) failed: %v", manifestPath, err)
	}
	uuidPattern := regexp.MustCompile(`uuid: "[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}"`)
	if !uuidPattern.Match(manifestData) {
		t.Fatalf("generated manifest missing UUIDv4: %s", string(manifestData))
	}
}

func TestGenerateCompositeAliasRendersKinds(t *testing.T) {
	root := t.TempDir()

	result, err := Generate("composite-go-swiftui", "orbit-console", GenerateOptions{Dir: root})
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	if result.Template != "composite-go-swiftui" {
		t.Fatalf("result.Template = %q, want %q", result.Template, "composite-go-swiftui")
	}

	manifestPath := filepath.Join(root, "orbit-console", identity.ManifestFileName)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) failed: %v", manifestPath, err)
	}
	content := string(data)
	for _, expected := range []string{
		"motto: \"go + swiftui composite.\"",
		"primary: \"app/app.txt\"",
		"path: \"daemon\"",
		"path: \"app\"",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("manifest missing %q:\n%s", expected, content)
		}
	}
}

func TestGeneratePythonDaemonUsesPythonRunner(t *testing.T) {
	root := t.TempDir()

	result, err := Generate("python-daemon", "serene-service", GenerateOptions{Dir: root})
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}
	if result.Template != "python-daemon" {
		t.Fatalf("result.Template = %q, want %q", result.Template, "python-daemon")
	}

	manifestPath := filepath.Join(root, "serene-service", identity.ManifestFileName)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) failed: %v", manifestPath, err)
	}
	content := string(data)
	for _, expected := range []string{
		"kind: \"composite\"",
		"runner: \"python\"",
		"files: [\"app/main.py\"]",
		"primary: \"app/main.py\"",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("manifest missing %q:\n%s", expected, content)
		}
	}
	if strings.Contains(content, "runner: recipe") {
		t.Fatalf("manifest still references recipe runner:\n%s", content)
	}
}

func TestGenerateDartDaemonUsesDartRunnerAndBinMain(t *testing.T) {
	root := t.TempDir()

	result, err := Generate("dart-daemon", "steady-engine", GenerateOptions{Dir: root})
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}
	if result.Template != "dart-daemon" {
		t.Fatalf("result.Template = %q, want %q", result.Template, "dart-daemon")
	}

	manifestPath := filepath.Join(root, "steady-engine", identity.ManifestFileName)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) failed: %v", manifestPath, err)
	}
	content := string(data)
	for _, expected := range []string{
		"kind: \"native\"",
		"runner: \"dart\"",
		"commands: [\"dart\"]",
		"files: [\"pubspec.yaml\", \"bin/main.dart\"]",
		"binary: \"steady-engine\"",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("manifest missing %q:\n%s", expected, content)
		}
	}

	if _, err := os.Stat(filepath.Join(root, "steady-engine", "bin", "main.dart")); err != nil {
		t.Fatalf("bin/main.dart missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "steady-engine", "lib", "main.dart")); !os.IsNotExist(err) {
		t.Fatalf("lib/main.dart should not exist, got: %v", err)
	}
}

func TestGenerateCompositeAliasesUseUpdatedDaemonRunners(t *testing.T) {
	tests := []struct {
		template   string
		slug       string
		wantRunner string
	}{
		{template: "composite-python-swiftui", slug: "mist-console", wantRunner: "python"},
		{template: "composite-dart-web", slug: "pulse-console", wantRunner: "dart"},
	}

	for _, tc := range tests {
		t.Run(tc.template, func(t *testing.T) {
			root := t.TempDir()
			if _, err := Generate(tc.template, tc.slug, GenerateOptions{Dir: root}); err != nil {
				t.Fatalf("Generate() failed: %v", err)
			}

			readmePath := filepath.Join(root, tc.slug, "daemon", "README.md")
			data, err := os.ReadFile(readmePath)
			if err != nil {
				t.Fatalf("ReadFile(%s) failed: %v", readmePath, err)
			}
			if !strings.Contains(string(data), "Runner: "+tc.wantRunner) {
				t.Fatalf("daemon README missing runner %q:\n%s", tc.wantRunner, string(data))
			}
		})
	}
}
