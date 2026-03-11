package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/organic-programming/codex-orchestrator/internal/cli"
)

const (
	defaultModelMaxContext = 128000
	promptBudgetRatio      = 0.40
)

func Build(cfg cli.Config, setDir, taskFile string, priorResults []string) (string, error) {
	systemSections, err := loadSystemSections(cfg.Root)
	if err != nil {
		return "", err
	}

	versionSections, err := loadVersionSections(setDir)
	if err != nil {
		return "", err
	}

	taskContent, err := os.ReadFile(taskFile)
	if err != nil {
		return "", fmt.Errorf("read task file: %w", err)
	}

	historySections, err := loadHistorySections(priorResults)
	if err != nil {
		return "", err
	}

	historyText := strings.Join(historySections, "\n\n")
	prompt := renderPrompt(filepath.Base(filepath.Dir(setDir)), filepath.Base(setDir), systemSections, versionSections, historyText, string(taskContent))
	budget := int(float64(defaultModelMaxContext) * promptBudgetRatio)
	if EstimateTokens(prompt) <= budget {
		return prompt, nil
	}

	compressedHistory, err := CompressHistory(priorResults, setDir)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(compressedHistory) == "" {
		compressedHistory = "(No prior completed task reports.)"
	}

	prompt = renderPrompt(filepath.Base(filepath.Dir(setDir)), filepath.Base(setDir), systemSections, versionSections, compressedHistory, string(taskContent))
	if EstimateTokens(prompt) <= budget {
		return prompt, nil
	}

	truncatedHistory := truncateHistory(historySections, budget/3)
	prompt = renderPrompt(filepath.Base(filepath.Dir(setDir)), filepath.Base(setDir), systemSections, versionSections, truncatedHistory, string(taskContent))
	return prompt, nil
}

func loadSystemSections(root string) ([]string, error) {
	var sections []string
	for _, name := range []string{"CONVENTIONS.md", "AGENTS.md", "AGENT.md"} {
		content, ok, err := readOptionalFile(filepath.Join(root, name))
		if err != nil {
			return nil, err
		}
		if ok {
			sections = append(sections, fmt.Sprintf("--- %s ---\n%s", name, strings.TrimSpace(content)))
		}
	}
	if len(sections) == 0 {
		sections = append(sections, "--- SYSTEM ---\n(No repository-wide conventions files were found.)")
	}
	return sections, nil
}

func loadVersionSections(setDir string) ([]string, error) {
	patterns := []string{
		filepath.Join(setDir, "DESIGN.md"),
		filepath.Join(setDir, "DESIGN_*.md"),
	}

	seen := make(map[string]struct{})
	var paths []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			match = filepath.Clean(match)
			if _, ok := seen[match]; ok {
				continue
			}
			seen[match] = struct{}{}
			paths = append(paths, match)
		}
	}
	sort.Strings(paths)

	sections := make([]string, 0, len(paths))
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read design file %s: %w", path, err)
		}
		sections = append(sections, fmt.Sprintf("--- %s ---\n%s", filepath.Base(path), strings.TrimSpace(string(content))))
	}
	if len(sections) == 0 {
		sections = append(sections, "--- DESIGN ---\n(No version design files were found.)")
	}
	return sections, nil
}

func loadHistorySections(priorResults []string) ([]string, error) {
	sections := make([]string, 0, len(priorResults))
	for i := len(priorResults) - 1; i >= 0; i-- {
		path := priorResults[i]
		content, ok, err := readOptionalFile(path)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		sections = append(sections, fmt.Sprintf("--- %s ---\n%s", filepath.Base(path), strings.TrimSpace(content)))
	}
	return sections, nil
}

func renderPrompt(project, setName string, systemSections, versionSections []string, history, task string) string {
	if strings.TrimSpace(history) == "" {
		history = "(No prior completed task reports.)"
	}

	var parts []string
	parts = append(parts,
		fmt.Sprintf("You are implementing tasks for the %s project, version %s.", project, setName),
		"Follow the conventions and design documents below.",
		strings.Join(systemSections, "\n\n"),
		strings.Join(versionSections, "\n\n"),
		"--- COMPLETED TASKS ---\n"+strings.TrimSpace(history),
		"--- CURRENT TASK ---\n"+strings.TrimSpace(task),
		`Implement this task. Do not modify task files, _TASKS.md, ROADMAP.md, or INDEX.md.
Do not create or switch git branches.
Do not install system-level dependencies without documenting them in the final output.
Focus exclusively on the code changes described in the current task.`,
	)

	return strings.Join(parts, "\n\n")
}

func truncateHistory(sections []string, maxTokens int) string {
	if len(sections) == 0 {
		return "(No prior completed task reports.)"
	}

	var parts []string
	used := 0
	for _, section := range sections {
		cost := EstimateTokens(section)
		if len(parts) > 0 && used+cost > maxTokens {
			break
		}
		parts = append(parts, section)
		used += cost
	}
	if len(parts) == 0 {
		parts = append(parts, sections[0])
	}
	if len(parts) < len(sections) {
		parts = append([]string{"(Earlier tasks omitted — see _HISTORY_SUMMARY.md for details.)"}, parts...)
	}
	return strings.Join(parts, "\n\n")
}

func readOptionalFile(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return string(data), true, nil
}
