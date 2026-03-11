package lifecycle

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/organic-programming/codex-orchestrator/internal/codex"
	gitpkg "github.com/organic-programming/codex-orchestrator/internal/git"
	"github.com/organic-programming/codex-orchestrator/internal/state"
	"github.com/organic-programming/codex-orchestrator/internal/tasks"
)

func TestLifecycleFlow(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test User")

	setDir := filepath.Join(repo, "v1.0")
	if err := os.MkdirAll(setDir, 0o755); err != nil {
		t.Fatalf("mkdir set dir: %v", err)
	}
	taskFile := filepath.Join(setDir, "task01.md")
	tasksFile := filepath.Join(setDir, "_TASKS.md")
	if err := os.WriteFile(taskFile, []byte("# TASK01\n\n## Acceptance Criteria\n\n- [ ] `go test ./...`\n"), 0o644); err != nil {
		t.Fatalf("write task file: %v", err)
	}
	if err := os.WriteFile(tasksFile, []byte("| # | File | Summary | Depends on | Status |\n|---|---|---|---|---|\n| 01 | [TASK01](./task01.md) | First | — | — |\n"), 0o644); err != nil {
		t.Fatalf("write tasks file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "holon.yaml"), []byte("version: v0.0\n"), 0o644); err != nil {
		t.Fatalf("write holon.yaml: %v", err)
	}

	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "initial")

	entry := tasks.Entry{Number: "01", FilePath: taskFile}
	gitOps := &gitpkg.Ops{Root: repo}

	if err := StartTask(entry, setDir, gitOps); err != nil {
		t.Fatalf("StartTask returned error: %v", err)
	}
	assertContains(t, tasksFile, "💭")

	result := codex.Result{Success: true, Outcome: state.OutcomeSuccess, Attempts: 1}
	if err := CompleteTask(entry, result, setDir, gitOps); err != nil {
		t.Fatalf("CompleteTask returned error: %v", err)
	}
	assertContains(t, tasksFile, "✅")
	assertContains(t, taskFile, "## Status")

	if err := UpdateVersionStatus(setDir, []tasks.Entry{entry}, gitOps); err != nil {
		t.Fatalf("UpdateVersionStatus returned error: %v", err)
	}
	renamedSetDir := filepath.Join(repo, "✅ v1.0")
	if _, err := os.Stat(renamedSetDir); err != nil {
		t.Fatalf("expected renamed set dir: %v", err)
	}

	renamedTaskFile := filepath.Join(renamedSetDir, "task01.md")
	st := state.Load(filepath.Join(repo, ".codex_orchestrator_state.json"))
	st.SetTask(renamedTaskFile, state.TaskState{Outcome: state.OutcomeSuccess})
	if err := st.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	if err := Reset(renamedSetDir, st, gitOps); err != nil {
		t.Fatalf("Reset returned error: %v", err)
	}
	resetTaskFile := filepath.Join(repo, "v1.0", "task01.md")
	assertContains(t, filepath.Join(repo, "v1.0", "_TASKS.md"), "| 01 | [TASK01](./task01.md) | First | — | — |")
	assertNotContains(t, resetTaskFile, "## Status")

	if err := Release(filepath.Join(repo, "v1.0"), "v1.0", gitOps); err != nil {
		t.Fatalf("Release returned error: %v", err)
	}
	assertContains(t, filepath.Join(repo, "holon.yaml"), "version: v1.0")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}

func assertContains(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("%s does not contain %q:\n%s", path, want, string(data))
	}
}

func assertNotContains(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if strings.Contains(string(data), want) {
		t.Fatalf("%s unexpectedly contains %q:\n%s", path, want, string(data))
	}
}
