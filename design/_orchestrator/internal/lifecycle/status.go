package lifecycle

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/organic-programming/codex-orchestrator/internal/git"
	"github.com/organic-programming/codex-orchestrator/internal/state"
	"github.com/organic-programming/codex-orchestrator/internal/tasks"
)

func UpdateVersionStatus(setDir string, entries []tasks.Entry, gitOps *git.Ops) error {
	tasksFile := filepath.Join(setDir, "_TASKS.md")
	statuses, err := readTaskStatuses(tasksFile)
	if err != nil {
		return err
	}
	if len(statuses) == 0 {
		return nil
	}

	allPassed := true
	terminalWarning := false
	for _, status := range statuses {
		switch status {
		case "✅":
		case "❌", "⚠️", "⚠":
			allPassed = false
			terminalWarning = true
		default:
			allPassed = false
		}
	}
	if !allPassed && !terminalWarning {
		return nil
	}

	currentBase := filepath.Base(setDir)
	rawName := rawSetName(currentBase)
	targetBase := rawName
	if allPassed {
		targetBase = "✅ " + rawName
	} else if terminalWarning {
		targetBase = "⚠️ " + rawName
	}
	if currentBase == targetBase {
		return nil
	}

	targetDir := filepath.Join(filepath.Dir(setDir), targetBase)
	if err := gitOps.Rename(setDir, targetDir); err != nil {
		return err
	}
	return gitOps.AddCommitPush(fmt.Sprintf("chore: update set status %s", rawName))
}

func updateTaskStatus(tasksFile string, task tasks.Entry, status string) error {
	content, err := os.ReadFile(tasksFile)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	rowUpdated := false
	taskBase := filepath.Base(task.FilePath)

	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") || !strings.HasSuffix(trimmed, "|") {
			continue
		}
		cells := parseRow(trimmed)
		if len(cells) != 5 || cells[0] == "#" || isSeparatorLine(cells) {
			continue
		}
		if normalizeTaskNumber(cells[0]) != normalizeTaskNumber(task.Number) {
			continue
		}
		if !strings.Contains(cells[1], taskBase) {
			continue
		}
		cells[4] = status
		lines[index] = formatRow(cells)
		rowUpdated = true
		break
	}

	if !rowUpdated {
		return fmt.Errorf("task %s not found in %s", task.Number, tasksFile)
	}

	return os.WriteFile(tasksFile, []byte(strings.Join(lines, "\n")), 0o644)
}

func readTaskStatuses(tasksFile string) ([]string, error) {
	content, err := os.ReadFile(tasksFile)
	if err != nil {
		return nil, err
	}

	var statuses []string
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") || !strings.HasSuffix(trimmed, "|") {
			continue
		}
		cells := parseRow(trimmed)
		if len(cells) != 5 || cells[0] == "#" || isSeparatorLine(cells) {
			continue
		}
		statuses = append(statuses, strings.TrimSpace(cells[4]))
	}

	return statuses, nil
}

func parseRow(line string) []string {
	parts := strings.Split(line[1:len(line)-1], "|")
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(part))
	}
	return cells
}

func formatRow(cells []string) string {
	return "| " + strings.Join(cells, " | ") + " |"
}

func isSeparatorLine(cells []string) bool {
	for _, cell := range cells {
		for _, r := range cell {
			if r != '-' && r != ':' {
				return false
			}
		}
	}
	return true
}

func normalizeTaskNumber(value string) string {
	value = strings.TrimSpace(value)
	number, err := strconv.Atoi(value)
	if err != nil {
		return value
	}
	return fmt.Sprintf("%02d", number)
}

func replaceStatusBlock(content, block string) string {
	re := regexp.MustCompile(`(?s)\n## Status\n.*$`)
	content = strings.TrimRight(content, "\n")
	content = re.ReplaceAllString(content, "")
	return content + "\n\n" + block
}

func writeStatusBlock(taskFile, block string) error {
	content, err := os.ReadFile(taskFile)
	if err != nil {
		return err
	}
	return os.WriteFile(taskFile, []byte(replaceStatusBlock(string(content), block)), 0o644)
}

func removeStatusBlock(taskFile string) error {
	content, err := os.ReadFile(taskFile)
	if err != nil {
		return err
	}
	re := regexp.MustCompile(`(?s)\n## Status\n.*$`)
	cleaned := strings.TrimRight(re.ReplaceAllString(string(content), ""), "\n") + "\n"
	return os.WriteFile(taskFile, []byte(cleaned), 0o644)
}

func taskStatusForOutcome(outcome state.Outcome) string {
	switch outcome {
	case state.OutcomeSuccess:
		return "✅"
	case state.OutcomeDeferred:
		return "⚠️"
	default:
		return "❌"
	}
}

func taskLabel(task tasks.Entry) string {
	return "TASK" + normalizeTaskNumber(task.Number)
}

func rawSetName(name string) string {
	for _, prefix := range []string{"✅ ", "⚠️ ", "⚠ ", "💭 "} {
		if strings.HasPrefix(name, prefix) {
			return strings.TrimPrefix(name, prefix)
		}
	}
	return name
}

func failureReportPath(taskFile string) string {
	ext := filepath.Ext(taskFile)
	if ext == "" {
		return taskFile + ".failure.md"
	}
	return strings.TrimSuffix(taskFile, ext) + ".failure.md"
}
