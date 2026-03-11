package lifecycle

import (
	"fmt"
	"path/filepath"

	"github.com/organic-programming/codex-orchestrator/internal/git"
	"github.com/organic-programming/codex-orchestrator/internal/tasks"
)

func StartTask(task tasks.Entry, setDir string, gitOps *git.Ops) error {
	tasksFile := filepath.Join(setDir, "_TASKS.md")
	if err := updateTaskStatus(tasksFile, task, "💭"); err != nil {
		return err
	}
	return gitOps.AddCommitPush(fmt.Sprintf("chore: start %s", taskLabel(task)), tasksFile)
}
