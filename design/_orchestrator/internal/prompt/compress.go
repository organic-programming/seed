package prompt

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const historySummaryFile = "_HISTORY_SUMMARY.md"

var execCommand = exec.Command

func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return (len(text) + 3) / 4
}

func CompressHistory(results []string, setDir string) (string, error) {
	if len(results) == 0 {
		return "", nil
	}

	summaryPath := filepath.Join(setDir, historySummaryFile)
	reuse, err := canReuseSummary(summaryPath, results)
	if err != nil {
		return "", err
	}
	if reuse {
		content, err := os.ReadFile(summaryPath)
		if err != nil {
			return "", err
		}
		return string(content), nil
	}

	var builder strings.Builder
	builder.WriteString("Summarize these task completion reports into a single concise briefing.\n")
	builder.WriteString("Preserve: what was implemented, which files were changed, and any decisions or caveats.\n")
	builder.WriteString("Remove verbosity.\n\n")
	for _, path := range results {
		content, ok, err := readOptionalFile(path)
		if err != nil {
			return "", err
		}
		if !ok {
			continue
		}
		builder.WriteString("--- ")
		builder.WriteString(filepath.Base(path))
		builder.WriteString(" ---\n")
		builder.WriteString(strings.TrimSpace(content))
		builder.WriteString("\n\n")
	}

	tempDir := filepath.Dir(summaryPath)
	tempOutput, err := os.CreateTemp(tempDir, ".history-summary-*.md")
	if err != nil {
		return "", err
	}
	tempOutputPath := tempOutput.Name()
	if err := tempOutput.Close(); err != nil {
		_ = os.Remove(tempOutputPath)
		return "", err
	}
	defer os.Remove(tempOutputPath)

	cmd := execCommand(
		"codex",
		"exec",
		"--ephemeral",
		"-C", setDir,
		"-s", "workspace-write",
		"-m", "gpt-5.1-codex-mini",
		"-o", tempOutputPath,
		builder.String(),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("compress history: %w: %s", err, strings.TrimSpace(string(output)))
	}

	summary, err := os.ReadFile(tempOutputPath)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(summaryPath, summary, 0o644); err != nil {
		return "", err
	}
	return string(summary), nil
}

func canReuseSummary(summaryPath string, results []string) (bool, error) {
	summaryInfo, err := os.Stat(summaryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	for _, path := range results {
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return false, err
		}
		if info.ModTime().After(summaryInfo.ModTime()) {
			return false, nil
		}
	}

	return true, nil
}
