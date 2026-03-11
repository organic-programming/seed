package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/organic-programming/codex-orchestrator/internal/cli"
)

func TestBuildIncludesAllLayersInOrder(t *testing.T) {
	root := t.TempDir()
	setDir := filepath.Join(root, "v1.0")
	taskFile := filepath.Join(setDir, "task01.md")
	resultFile := filepath.Join(setDir, "task00.md.result.md")

	for path, content := range map[string]string{
		filepath.Join(root, "AGENTS.md"): "agent rules",
		filepath.Join(setDir, "DESIGN.md"): "design text",
		taskFile: "# TASK01\n\nTask body",
		resultFile: "prior result",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	p, err := Build(cli.Config{Root: root}, setDir, taskFile, []string{resultFile})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	for _, want := range []string{
		"You are implementing tasks",
		"--- AGENTS.md ---",
		"agent rules",
		"--- DESIGN.md ---",
		"design text",
		"--- COMPLETED TASKS ---",
		"prior result",
		"--- CURRENT TASK ---",
		"Task body",
	} {
		if !strings.Contains(p, want) {
			t.Fatalf("prompt missing %q:\n%s", want, p)
		}
	}

	systemIndex := strings.Index(p, "--- AGENTS.md ---")
	designIndex := strings.Index(p, "--- DESIGN.md ---")
	historyIndex := strings.Index(p, "--- COMPLETED TASKS ---")
	taskIndex := strings.Index(p, "--- CURRENT TASK ---")
	if !(systemIndex < designIndex && designIndex < historyIndex && historyIndex < taskIndex) {
		t.Fatalf("unexpected section order:\n%s", p)
	}
}
