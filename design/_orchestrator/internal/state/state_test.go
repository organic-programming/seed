package state

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStateSaveLoadAndCompletedResults(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	setDir := filepath.Join(root, "v1.0")
	stateFile := filepath.Join(root, ".codex_orchestrator_state.json")

	st := Load(stateFile)
	taskOne := filepath.Join(setDir, "task01.md")
	taskTwo := filepath.Join(setDir, "task02.md")
	otherTask := filepath.Join(root, "v2.0", "task01.md")

	st.SetTask(taskOne, TaskState{Outcome: OutcomeSuccess, Tokens: TokenUsage{InputTokens: 10}})
	st.SetTask(taskTwo, TaskState{Outcome: OutcomeFailed, Attempts: 2})
	st.SetTask(otherTask, TaskState{Outcome: OutcomeSuccess})

	if err := st.Save(); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	loaded := Load(stateFile)
	if !loaded.IsCompleted(taskOne) {
		t.Fatalf("expected %s to be completed", taskOne)
	}
	if loaded.IsCompleted(taskTwo) {
		t.Fatalf("expected %s to be incomplete", taskTwo)
	}

	results := loaded.CompletedResults(setDir)
	if len(results) != 1 || results[0] != taskOne+".result.md" {
		t.Fatalf("CompletedResults = %v", results)
	}

	loaded.RemoveSet(setDir)
	if loaded.IsCompleted(taskOne) {
		t.Fatalf("expected %s to be removed", taskOne)
	}
	if !loaded.IsCompleted(otherTask) {
		t.Fatalf("expected other set task to remain")
	}
}

func TestLoadMalformedStateRenamesBadFileAndWarns(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), ".codex_orchestrator_state.json")
	if err := os.WriteFile(stateFile, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("write state file: %v", err)
	}

	originalStderr := os.Stderr
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	os.Stderr = writePipe

	st := Load(stateFile)

	_ = writePipe.Close()
	os.Stderr = originalStderr

	if got := len(st.TasksSnapshot()); got != 0 {
		t.Fatalf("expected empty state, got %d tasks", got)
	}

	warning, err := io.ReadAll(readPipe)
	if err != nil {
		t.Fatalf("read stderr warning: %v", err)
	}
	_ = readPipe.Close()

	if !strings.Contains(string(warning), "warning: state file") || !strings.Contains(string(warning), ".bad") {
		t.Fatalf("unexpected warning: %q", string(warning))
	}

	if _, err := os.Stat(stateFile); !os.IsNotExist(err) {
		t.Fatalf("expected original corrupt state file to be renamed, stat err=%v", err)
	}

	badPath := stateFile + ".bad"
	data, err := os.ReadFile(badPath)
	if err != nil {
		t.Fatalf("read bad state file: %v", err)
	}
	if string(data) != "{not-json" {
		t.Fatalf("unexpected bad state contents: %q", string(data))
	}
}
