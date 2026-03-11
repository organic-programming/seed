package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/organic-programming/codex-orchestrator/internal/state"
)

func TestRunSmoke(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test User")
	runGit(t, repo, "checkout", "-b", "base-dev")

	binDir := filepath.Join(repo, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	codexPath := filepath.Join(binDir, "codex")
	script := `#!/bin/sh
if [ "$1" = "login" ] && [ "$2" = "status" ]; then
	exit 0
fi
if [ "$1" != "exec" ]; then
	echo "unexpected command" >&2
	exit 1
fi
root=""
out=""
last=""
while [ "$#" -gt 0 ]; do
	case "$1" in
		-C)
			shift
			root="$1"
			;;
		-o)
			shift
			out="$1"
			;;
		*)
			last="$1"
			;;
	esac
	shift
done
if printf '%s' "$last" | grep -q "Reply OK"; then
	echo "OK"
	exit 0
fi
if printf '%s' "$last" | grep -q "TASK01"; then
	touch "$root/task01.done"
fi
if printf '%s' "$last" | grep -q "TASK02"; then
	touch "$root/task02.done"
fi
printf '%s\n' '{"type":"thread.started","thread_id":"thread-123"}'
printf '%s\n' '{"type":"turn.completed","usage":{"input_tokens":5,"cached_input_tokens":1,"output_tokens":2}}'
printf 'done\n' > "$out"
`
	if err := os.WriteFile(codexPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module example.com/smoke\n\ngo 1.25.1\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "holon.yaml"), []byte("version: v0.0\n"), 0o644); err != nil {
		t.Fatalf("write holon.yaml: %v", err)
	}

	writeFixtureTest(t, filepath.Join(repo, "task1fixture", "fixture_test.go"), "task01.done")
	writeFixtureTest(t, filepath.Join(repo, "task2fixture", "fixture_test.go"), "task02.done")

	setDir := filepath.Join(repo, "v1.0")
	if err := os.MkdirAll(setDir, 0o755); err != nil {
		t.Fatalf("mkdir set dir: %v", err)
	}
	taskOne := filepath.Join(setDir, "task01.md")
	taskTwo := filepath.Join(setDir, "task02.md")
	if err := os.WriteFile(taskOne, []byte("# TASK01\n\n## Acceptance Criteria\n\n- [ ] `go test ./task1fixture`\n"), 0o644); err != nil {
		t.Fatalf("write task one: %v", err)
	}
	if err := os.WriteFile(taskTwo, []byte("# TASK02\n\n## Acceptance Criteria\n\n- [ ] `go test ./task2fixture`\n"), 0o644); err != nil {
		t.Fatalf("write task two: %v", err)
	}
	tasksFile := filepath.Join(setDir, "_TASKS.md")
	if err := os.WriteFile(tasksFile, []byte("| # | File | Summary | Depends on | Status |\n|---|---|---|---|---|\n| 01 | [TASK01](./task01.md) | First | — | — |\n| 02 | [TASK02](./task02.md) | Second | TASK01 | — |\n"), 0o644); err != nil {
		t.Fatalf("write _TASKS.md: %v", err)
	}

	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "initial")

	if code := run([]string{"--root", repo, "--set", "v1.0"}); code != 0 {
		t.Fatalf("run returned %d", code)
	}

	if _, err := os.Stat(filepath.Join(repo, "✅ v1.0")); err != nil {
		t.Fatalf("expected completed set dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".codex_orchestrator_summary.md")); err != nil {
		t.Fatalf("expected summary report: %v", err)
	}

	st := state.Load(filepath.Join(repo, ".codex_orchestrator_state.json"))
	if !st.IsCompleted(taskOne) || !st.IsCompleted(taskTwo) {
		t.Fatalf("expected both tasks completed")
	}

	tagOutput := runGitOutput(t, repo, "tag", "--list")
	if !strings.Contains(tagOutput, "v1.0.0") {
		t.Fatalf("expected release tag, got %q", tagOutput)
	}
}

func TestRunHelp(t *testing.T) {
	if code := run([]string{"--help"}); code != 0 {
		t.Fatalf("run(--help) = %d", code)
	}
}

func writeFixtureTest(t *testing.T, path, expectedFile string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir fixture dir: %v", err)
	}
	content := `package fixture
import "os"
import "testing"
func TestDone(t *testing.T) {
	if _, err := os.Stat("../` + expectedFile + `"); err != nil {
		t.Fatal(err)
	}
}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture test: %v", err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}

func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
	return string(output)
}
