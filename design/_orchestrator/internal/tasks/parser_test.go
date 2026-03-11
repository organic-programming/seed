package tasks

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	dir := t.TempDir()

	taskFiles := []string{
		filepath.Join(dir, "task01.md"),
		filepath.Join(dir, "task02.md"),
		filepath.Join(dir, "task03.md"),
	}
	for _, path := range taskFiles {
		if err := os.WriteFile(path, []byte("# task\n"), 0o644); err != nil {
			t.Fatalf("write task file %s: %v", path, err)
		}
	}

	tasksFile := filepath.Join(dir, "_TASKS.md")
	content := strings.Join([]string{
		"# Tasks",
		"",
		"| # | File | Summary | Depends on | Status |",
		"|---|---|---|---|---|",
		"| 01 | [TASK01](./task01.md) | First task | — | — |",
		"| 02 | [TASK02](./task02.md) | Second task |  TASK01,  TASK03  | — |",
		"| 03 | [TASK03](./task03.md) | Third task | TASK01, v0.7 TASK02, TASK04–06 | — |",
	}, "\n")
	if err := os.WriteFile(tasksFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write tasks file: %v", err)
	}

	entries, err := Parse(tasksFile)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	if entries[0].Number != "01" {
		t.Fatalf("entry 0 number = %q, want %q", entries[0].Number, "01")
	}
	if entries[0].FilePath != taskFiles[0] {
		t.Fatalf("entry 0 path = %q, want %q", entries[0].FilePath, taskFiles[0])
	}
	if entries[0].Summary != "First task" {
		t.Fatalf("entry 0 summary = %q, want %q", entries[0].Summary, "First task")
	}
	if len(entries[0].DependsOn) != 0 {
		t.Fatalf("entry 0 depends_on = %v, want none", entries[0].DependsOn)
	}

	if !reflect.DeepEqual(entries[1].DependsOn, []string{"TASK01", "TASK03"}) {
		t.Fatalf("entry 1 depends_on = %v", entries[1].DependsOn)
	}

	wantThirdDeps := []string{"TASK01", "v0.7 TASK02", "TASK04–06"}
	if !reflect.DeepEqual(entries[2].DependsOn, wantThirdDeps) {
		t.Fatalf("entry 2 depends_on = %v, want %v", entries[2].DependsOn, wantThirdDeps)
	}
}

func TestFindSetDir(t *testing.T) {
	t.Run("design root", func(t *testing.T) {
		root := t.TempDir()
		setDir := filepath.Join(root, "design", "grace-op", "v0.4")
		if err := os.MkdirAll(setDir, 0o755); err != nil {
			t.Fatalf("mkdir set dir: %v", err)
		}

		gotSetDir, gotProject, err := FindSetDir(root, "v0.4")
		if err != nil {
			t.Fatalf("FindSetDir returned error: %v", err)
		}
		if gotSetDir != setDir {
			t.Fatalf("set dir = %q, want %q", gotSetDir, setDir)
		}
		if gotProject != "grace-op" {
			t.Fatalf("project = %q, want %q", gotProject, "grace-op")
		}
	})

	t.Run("project root", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "_orchestrator")
		setDir := filepath.Join(root, "✅ v1.0")
		if err := os.MkdirAll(setDir, 0o755); err != nil {
			t.Fatalf("mkdir set dir: %v", err)
		}

		gotSetDir, gotProject, err := FindSetDir(root, "v1.0")
		if err != nil {
			t.Fatalf("FindSetDir returned error: %v", err)
		}
		if gotSetDir != setDir {
			t.Fatalf("set dir = %q, want %q", gotSetDir, setDir)
		}
		if gotProject != "_orchestrator" {
			t.Fatalf("project = %q, want %q", gotProject, "_orchestrator")
		}
	})

	t.Run("ambiguous set", func(t *testing.T) {
		root := t.TempDir()
		for _, project := range []string{"grace-op", "rob-go"} {
			path := filepath.Join(root, "design", project, "v0.1")
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatalf("mkdir set dir: %v", err)
			}
		}

		_, _, err := FindSetDir(root, "v0.1")
		if err == nil {
			t.Fatal("expected ambiguity error, got nil")
		}
		if !strings.Contains(err.Error(), "ambiguous") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
