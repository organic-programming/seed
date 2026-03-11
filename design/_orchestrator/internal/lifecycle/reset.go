package lifecycle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/organic-programming/codex-orchestrator/internal/git"
	"github.com/organic-programming/codex-orchestrator/internal/state"
	"github.com/organic-programming/codex-orchestrator/internal/tasks"
)

func Reset(setDir string, st *state.State, gitOps *git.Ops) error {
	originalSetDir := setDir
	rawName := rawSetName(filepath.Base(setDir))
	if filepath.Base(setDir) != rawName {
		targetDir := filepath.Join(filepath.Dir(setDir), rawName)
		if err := gitOps.Rename(setDir, targetDir); err != nil {
			return err
		}
		setDir = targetDir
	}

	tasksFile := filepath.Join(setDir, "_TASKS.md")
	entries, err := tasks.Parse(tasksFile)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if err := updateTaskStatus(tasksFile, entry, "—"); err != nil {
			return err
		}
		if err := removeStatusBlock(entry.FilePath); err != nil {
			return err
		}
		_ = os.Remove(failureReportPath(entry.FilePath))
	}

	st.RemoveSet(originalSetDir)
	st.RemoveSet(setDir)
	if err := st.Save(); err != nil {
		return err
	}

	return gitOps.AddCommitPush(
		fmt.Sprintf("chore: reset %s", rawName),
		append(resetFiles(setDir, entries), tasksFile)...,
	)
}

func resetFiles(setDir string, entries []tasks.Entry) []string {
	files := []string{setDir}
	for _, entry := range entries {
		files = append(files, entry.FilePath)
		reportPath := failureReportPath(entry.FilePath)
		if _, err := os.Stat(reportPath); err == nil {
			files = append(files, reportPath)
		}
	}
	return uniqueFiles(files)
}

func uniqueFiles(files []string) []string {
	seen := make(map[string]struct{}, len(files))
	var result []string
	for _, file := range files {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		if _, ok := seen[file]; ok {
			continue
		}
		seen[file] = struct{}{}
		result = append(result, file)
	}
	return result
}
