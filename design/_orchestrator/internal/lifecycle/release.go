package lifecycle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/organic-programming/codex-orchestrator/internal/git"
)

func Release(setDir, version string, gitOps *git.Ops) error {
	holonPath := filepath.Join(gitOps.Root, "holon.yaml")
	content, err := os.ReadFile(holonPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	updated := false
	for index, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "version:") {
			lines[index] = "version: " + version
			updated = true
			break
		}
	}
	if !updated {
		lines = append(lines, "version: "+version)
	}

	if err := os.WriteFile(holonPath, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return err
	}
	if err := gitOps.AddCommitPush(fmt.Sprintf("chore: release %s", version), holonPath); err != nil {
		return err
	}
	return gitOps.Tag(version+".0", fmt.Sprintf("release %s", version))
}
