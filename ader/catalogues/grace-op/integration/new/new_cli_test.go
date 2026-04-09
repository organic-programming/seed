package new_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestNew_CLI_JSONCreatesIdentity(t *testing.T) {
	sb := integration.NewSandbox(t)
	root := t.TempDir()
	result := sb.RunOPWithOptions(t, integration.RunOptions{SkipDiscoverRoot: true, WorkDir: root}, "new", "--json", `{"given_name":"Alpha","family_name":"Builder","motto":"Builds holons.","composer":"test","clade":"deterministic/io_bound","lang":"go"}`)
	integration.RequireSuccess(t, result)

	createdPath := filepath.Join(root, "holons", "alpha-builder", "holon.proto")
	if _, err := os.Stat(createdPath); err != nil {
		t.Fatalf("created holon manifest missing: %v", err)
	}
	data, err := os.ReadFile(createdPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	integration.RequireContains(t, string(data), `option (holons.v1.manifest) = {`)
	integration.RequireContains(t, string(data), `given_name: "Alpha"`)
	integration.RequireContains(t, result.Stdout, "Identity created")
	integration.RequireContains(t, result.Stdout, "Alpha Builder")
}

func TestNew_CLI_ListTemplates(t *testing.T) {
	sb := integration.NewSandbox(t)
	root := t.TempDir()
	result := sb.RunOPWithOptions(t, integration.RunOptions{SkipDiscoverRoot: true, WorkDir: root}, "new", "--list")
	integration.RequireSuccess(t, result)
	for _, expected := range []string{
		"coax-flutter",
		"coax-swiftui",
		"composite-go-swiftui",
		"composite-go-web",
		"composite-python-web",
	} {
		integration.RequireContains(t, result.Stdout, expected)
	}
}

func TestNew_CLI_TemplateJSONOutput(t *testing.T) {
	sb := integration.NewSandbox(t)
	root := t.TempDir()
	result := sb.RunOPWithOptions(t, integration.RunOptions{SkipDiscoverRoot: true, WorkDir: root}, "--format", "json", "new", "--template", "composite-go-web", "my-console")
	integration.RequireSuccess(t, result)

	var payload struct {
		Template string `json:"template"`
		Dir      string `json:"dir"`
	}
	payload = integration.DecodeJSON[struct {
		Template string `json:"template"`
		Dir      string `json:"dir"`
	}](t, result.Stdout)
	if payload.Template != "composite-go-web" {
		t.Fatalf("template = %q, want composite-go-web", payload.Template)
	}
	if !strings.Contains(payload.Dir, "my-console") {
		t.Fatalf("dir = %q, want generated scaffold path", payload.Dir)
	}
}

func TestNew_CLI_GenerateCoaxFlutterHenriNobody(t *testing.T) {
	if testing.Short() {
		t.Skip("template toolchain smoke is slow")
	}
	if _, err := exec.LookPath("flutter"); err != nil {
		t.Skip("flutter not available")
	}

	sb := integration.NewSandbox(t)
	root := t.TempDir()
	linkTemplateDependencies(t, root)

	result := sb.RunOPWithOptions(
		t,
		integration.RunOptions{SkipDiscoverRoot: true, WorkDir: root},
		"new",
		"--template",
		"coax-flutter",
		"henri-nobody",
	)
	integration.RequireSuccess(t, result)

	generated := filepath.Join(root, "henri-nobody")
	requireFile(t, filepath.Join(generated, "app", "lib", "main.dart"))
	requireFile(t, filepath.Join(generated, "app", "lib", "src", "app.dart"))
	requireFile(t, filepath.Join(generated, "app", "tool", "package_desktop.dart"))

	runExternalCommand(t, filepath.Join(generated, "app"), "flutter", "pub", "get")
	runExternalCommand(t, filepath.Join(generated, "app"), "flutter", "analyze", "lib")
}

func TestNew_CLI_GenerateCoaxSwiftUIHenriNobody(t *testing.T) {
	if testing.Short() {
		t.Skip("template toolchain smoke is slow")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("swiftui scaffold is macOS-oriented")
	}
	if _, err := exec.LookPath("swift"); err != nil {
		t.Skip("swift not available")
	}

	sb := integration.NewSandbox(t)
	root := t.TempDir()
	linkTemplateDependencies(t, root)

	result := sb.RunOPWithOptions(
		t,
		integration.RunOptions{SkipDiscoverRoot: true, WorkDir: root},
		"new",
		"--template",
		"coax-swiftui",
		"henri-nobody",
	)
	integration.RequireSuccess(t, result)

	generated := filepath.Join(root, "henri-nobody")
	requireFile(t, filepath.Join(generated, "App", "HenriNobodyApp.swift"))
	requireFile(t, filepath.Join(generated, "App", "ContentView.swift"))
	requireFile(t, filepath.Join(generated, "Modules", "Sources", "AppKit", "HolonProcess.swift"))

	projectYAML, err := os.ReadFile(filepath.Join(generated, "project.yml"))
	if err != nil {
		t.Fatalf("read project.yml: %v", err)
	}
	integration.RequireContains(t, string(projectYAML), "{{ if .Hardened }}")

	runExternalCommand(t, filepath.Join(generated, "Modules"), "swift", "build")
}

func linkTemplateDependencies(t *testing.T, root string) {
	t.Helper()
	for _, name := range []string{"organism_kits", "sdk"} {
		target := filepath.Join(integration.SeedRoot(t), name)
		link := filepath.Join(root, name)
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("symlink %s -> %s: %v", link, target, err)
		}
	}
}

func requireFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file %s: %v", path, err)
	}
}

func runExternalCommand(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed in %s: %v\n%s", name, strings.Join(args, " "), dir, err, string(output))
	}
}
