package new_test

import (
	"os"
	"path/filepath"
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
	for _, expected := range []string{"composite-go-swiftui", "composite-go-web", "composite-python-web"} {
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
