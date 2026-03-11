package summary

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/organic-programming/codex-orchestrator/internal/state"
	"github.com/organic-programming/codex-orchestrator/internal/tasks"
)

type SetResult struct {
	Name     string
	Tasks    int
	Passed   int
	Failed   int
	Tokens   state.TokenUsage
	Status   string
	Failures []string
}

func Print(st *state.State, sets []SetResult, elapsed time.Duration) {
	report := buildReport(sets, elapsed)
	fmt.Print(report)

	root := filepath.Dir(st.Path())
	if root == "." || root == "" {
		return
	}
	_ = os.WriteFile(filepath.Join(root, ".codex_orchestrator_summary.md"), []byte(report), 0o644)
}

func BuildSetResult(setName string, entries []tasks.Entry, st *state.State) SetResult {
	result := SetResult{
		Name:  setName,
		Tasks: len(entries),
	}

	allPassed := len(entries) > 0
	for _, entry := range entries {
		taskState := st.Task(entry.FilePath)
		result.Tokens.Add(taskState.Tokens)
		switch taskState.Outcome {
		case state.OutcomeSuccess:
			result.Passed++
		case state.OutcomeFailed, state.OutcomeDeferred:
			result.Failed++
			result.Failures = append(result.Failures, failureReportPath(entry.FilePath))
			allPassed = false
		default:
			allPassed = false
		}
	}

	switch {
	case allPassed && result.Tasks > 0:
		result.Status = "✅"
	case result.Failed > 0:
		result.Status = "⚠️"
	default:
		result.Status = "💭"
	}

	return result
}

func buildReport(sets []SetResult, elapsed time.Duration) string {
	var totalTasks, totalPassed, totalFailed int
	var totalTokens state.TokenUsage

	var builder strings.Builder
	builder.WriteString("Codex Orchestrator — Run Summary\n\n")
	builder.WriteString(fmt.Sprintf("Elapsed: %s\n\n", elapsed.Round(time.Second)))
	builder.WriteString("| Set | Tasks | Passed | Failed | Tokens | Status |\n")
	builder.WriteString("|---|---:|---:|---:|---:|---|\n")
	for _, setResult := range sets {
		totalTasks += setResult.Tasks
		totalPassed += setResult.Passed
		totalFailed += setResult.Failed
		totalTokens.Add(setResult.Tokens)
		builder.WriteString(fmt.Sprintf(
			"| %s | %d | %d | %d | %d | %s |\n",
			setResult.Name,
			setResult.Tasks,
			setResult.Passed,
			setResult.Failed,
			setResult.Tokens.InputTokens+setResult.Tokens.OutputTokens,
			setResult.Status,
		))
	}
	builder.WriteString(fmt.Sprintf("| Total | %d | %d | %d | %d | |\n", totalTasks, totalPassed, totalFailed, totalTokens.InputTokens+totalTokens.OutputTokens))

	var failures []string
	for _, setResult := range sets {
		failures = append(failures, setResult.Failures...)
	}
	if len(failures) > 0 {
		builder.WriteString("\nFailed tasks:\n")
		for _, failure := range failures {
			builder.WriteString("- " + failure + "\n")
		}
	}

	return builder.String()
}

func failureReportPath(taskFile string) string {
	ext := filepath.Ext(taskFile)
	if ext == "" {
		return taskFile + ".failure.md"
	}
	return strings.TrimSuffix(taskFile, ext) + ".failure.md"
}
