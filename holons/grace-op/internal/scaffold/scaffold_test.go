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

	for _, name := range []string{"composite-go-swiftui", "composite-go-web", "composite-dart-flutter"} {
		if _, ok := names[name]; !ok {
			t.Fatalf("template %q missing from catalog", name)
		}
	}
}

func TestGenerateCompositeAliasRendersKindsAndFiles(t *testing.T) {
	root := t.TempDir()

	result, err := Generate("composite-go-swiftui", "orbit-console", GenerateOptions{Dir: root})
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	if result.Template != "composite-go-swiftui" {
		t.Fatalf("result.Template = %q, want %q", result.Template, "composite-go-swiftui")
	}

	manifestPath := filepath.Join(root, "orbit-console", "api", "v1", identity.ManifestFileName)
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) failed: %v", manifestPath, err)
	}
	content := string(manifestData)
	for _, expected := range []string{
		`kind: "composite"`,
		`runner: "recipe"`,
		`motto: "go + swiftui holon composite."`,
		`path: "holon"`,
		`path: "app"`,
		`primary: "app/app.txt"`,
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("manifest missing %q:\n%s", expected, content)
		}
	}

	uuidPattern := regexp.MustCompile(`uuid: "[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}"`)
	if !uuidPattern.Match(manifestData) {
		t.Fatalf("generated manifest missing UUIDv4: %s", string(manifestData))
	}

	holonReadmePath := filepath.Join(root, "orbit-console", "holon", "README.md")
	holonReadme, err := os.ReadFile(holonReadmePath)
	if err != nil {
		t.Fatalf("ReadFile(%s) failed: %v", holonReadmePath, err)
	}
	if !strings.Contains(string(holonReadme), "Runner: go-module") {
		t.Fatalf("holon README missing go runner:\n%s", string(holonReadme))
	}

	appReadmePath := filepath.Join(root, "orbit-console", "app", "README.md")
	appReadme, err := os.ReadFile(appReadmePath)
	if err != nil {
		t.Fatalf("ReadFile(%s) failed: %v", appReadmePath, err)
	}
	if !strings.Contains(string(appReadme), "Runner: swift-package") {
		t.Fatalf("app README missing swiftui runner:\n%s", string(appReadme))
	}
	appTextPath := filepath.Join(root, "orbit-console", "app", "app.txt")
	if data, err := os.ReadFile(appTextPath); err != nil {
		t.Fatalf("ReadFile(%s) failed: %v", appTextPath, err)
	} else if !strings.Contains(string(data), "orbit-console scaffold placeholder") {
		t.Fatalf("unexpected app placeholder content:\n%s", string(data))
	}
}

func TestGenerateCompositeAliasesUseUpdatedHolonRunners(t *testing.T) {
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

			readmePath := filepath.Join(root, tc.slug, "holon", "README.md")
			data, err := os.ReadFile(readmePath)
			if err != nil {
				t.Fatalf("ReadFile(%s) failed: %v", readmePath, err)
			}
			if !strings.Contains(string(data), "Runner: "+tc.wantRunner) {
				t.Fatalf("holon README missing runner %q:\n%s", tc.wantRunner, string(data))
			}
		})
	}
}
