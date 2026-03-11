package summary

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/organic-programming/codex-orchestrator/internal/state"
	"github.com/organic-programming/codex-orchestrator/internal/tasks"
)

func TestBuildSetResultAndPrint(t *testing.T) {
	root := t.TempDir()
	stateFile := filepath.Join(root, ".codex_orchestrator_state.json")
	st := state.Load(stateFile)

	taskOne := filepath.Join(root, "v1.0", "task01.md")
	taskTwo := filepath.Join(root, "v1.0", "task02.md")
	st.SetTask(taskOne, state.TaskState{Outcome: state.OutcomeSuccess, Tokens: state.TokenUsage{InputTokens: 3, OutputTokens: 2}})
	st.SetTask(taskTwo, state.TaskState{Outcome: state.OutcomeFailed, Tokens: state.TokenUsage{InputTokens: 5, OutputTokens: 1}})
	if err := st.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	setResult := BuildSetResult("v1.0", []tasks.Entry{{FilePath: taskOne}, {FilePath: taskTwo}}, st)
	if setResult.Passed != 1 || setResult.Failed != 1 || setResult.Status != "⚠️" {
		t.Fatalf("unexpected set result: %+v", setResult)
	}

	Print(st, []SetResult{setResult}, 42*time.Second)

	reportPath := filepath.Join(root, ".codex_orchestrator_summary.md")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read summary report: %v", err)
	}
	if !strings.Contains(string(data), "v1.0") {
		t.Fatalf("summary report missing set name:\n%s", string(data))
	}
}
