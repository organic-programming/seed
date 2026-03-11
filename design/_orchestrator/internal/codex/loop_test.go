package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/organic-programming/codex-orchestrator/internal/cli"
	"github.com/organic-programming/codex-orchestrator/internal/state"
	"github.com/organic-programming/codex-orchestrator/internal/tasks"
)

func TestExecuteLoopResumeAfterVerificationFailure(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}

	codexPath := filepath.Join(binDir, "codex")
	script := `#!/bin/sh
mode="create"
root=""
out=""
thread=""
while [ "$#" -gt 0 ]; do
	case "$1" in
		resume)
			mode="resume"
			shift
			thread="$1"
			;;
		-C)
			shift
			root="$1"
			;;
		-o)
			shift
			out="$1"
			;;
	esac
	shift
done
if [ "$mode" = "resume" ]; then
	touch "$root/fixed.txt"
	printf '%s\n' '{"type":"turn.completed","usage":{"input_tokens":2,"cached_input_tokens":0,"output_tokens":1}}'
	printf 'fixed\n' > "$out"
	exit 0
fi
printf '%s\n' '{"type":"thread.started","thread_id":"thread-123"}'
printf '%s\n' '{"type":"turn.completed","usage":{"input_tokens":3,"cached_input_tokens":1,"output_tokens":2}}'
printf 'created\n' > "$out"
`
	if err := os.WriteFile(codexPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/tmp\n\ngo 1.25.1\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	testFile := filepath.Join(root, "main_test.go")
	testContent := `package main
import "os"
import "testing"
func TestFixed(t *testing.T) {
	if _, err := os.Stat("fixed.txt"); err != nil {
		t.Fatal(err)
	}
}
`
	if err := os.WriteFile(testFile, []byte(testContent), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	taskFile := filepath.Join(root, "TASK01.md")
	taskContent := `# TASK01

## Acceptance Criteria

- [ ] ` + "`go test ./...`" + `
`
	if err := os.WriteFile(taskFile, []byte(taskContent), 0o644); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	st := state.Load(filepath.Join(root, ".codex_orchestrator_state.json"))
	result := ExecuteLoop(
		cli.Config{Root: root, Model: "gpt-5.4"},
		tasks.Entry{Number: "01", FilePath: taskFile},
		"implement task",
		nil,
		st,
	)
	if !result.Success || result.Outcome != state.OutcomeSuccess {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Attempts != 2 {
		t.Fatalf("expected 2 codex attempts, got %d", result.Attempts)
	}
	if !st.IsCompleted(taskFile) {
		t.Fatalf("expected task to be marked completed")
	}
	jsonl, err := os.ReadFile(taskFile + ".jsonl")
	if err != nil {
		t.Fatalf("read jsonl log: %v", err)
	}
	if !strings.Contains(string(jsonl), "thread-123") {
		t.Fatalf("expected thread id in jsonl log, got %q", string(jsonl))
	}
}
