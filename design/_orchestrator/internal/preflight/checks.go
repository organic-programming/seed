package preflight

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/organic-programming/codex-orchestrator/internal/cli"
	"github.com/organic-programming/codex-orchestrator/internal/tasks"
)

var execLookPath = exec.LookPath
var execCommand = exec.Command

func Run(cfg cli.Config) error {
	if _, err := execLookPath("codex"); err != nil {
		return fmt.Errorf("codex not found on PATH")
	}

	if output, err := runCommand(cfg.Root, "codex", "login", "status"); err != nil {
		return fmt.Errorf("codex not logged in — run codex login: %s", trimOutput(output))
	}

	if output, err := runCommand(
		cfg.Root,
		"codex",
		"exec",
		"--ephemeral",
		"--skip-git-repo-check",
		"-C", cfg.Root,
		"-s", "workspace-write",
		"-m", cfg.Model,
		"Reply OK",
	); err != nil {
		return fmt.Errorf("model %s not available: %s", cfg.Model, trimOutput(output))
	}

	if output, err := runCommand(cfg.Root, "git", "status", "--porcelain"); err != nil {
		return fmt.Errorf("git status failed: %s", trimOutput(output))
	} else if hasMeaningfulGitChanges(string(output)) {
		return fmt.Errorf("uncommitted changes — commit or stash first")
	}

	if output, err := runCommand(cfg.Root, "git", "submodule", "status", "--recursive"); err != nil {
		return fmt.Errorf("git submodule status failed: %s", trimOutput(output))
	} else {
		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "-") {
				fields := strings.Fields(line)
				name := "submodule"
				if len(fields) >= 2 {
					name = fields[1]
				}
				return fmt.Errorf("submodule %s not initialized", name)
			}
		}
	}

	for _, setName := range cfg.Sets {
		setDir, _, err := tasks.FindSetDir(cfg.Root, setName)
		if err != nil {
			return err
		}
		if _, err := os.Stat(setDir); err != nil {
			return fmt.Errorf("set directory %s not found", setName)
		}
		if _, err := os.Stat(filepath.Join(setDir, "_TASKS.md")); err != nil {
			return fmt.Errorf("_TASKS.md missing in %s", setDir)
		}
	}

	return nil
}

func runCommand(dir string, name string, args ...string) ([]byte, error) {
	cmd := execCommand(name, args...)
	cmd.Dir = dir

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	err := cmd.Run()
	return output.Bytes(), err
}

func trimOutput(output []byte) string {
	text := strings.TrimSpace(string(output))
	if text == "" {
		return "(no output)"
	}
	return text
}

func hasMeaningfulGitChanges(statusOutput string) bool {
	for _, line := range strings.Split(strings.TrimSpace(statusOutput), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		path := line
		if len(line) > 3 {
			path = strings.TrimSpace(line[3:])
		}
		if isRuntimeArtifact(path) {
			continue
		}
		return true
	}
	return false
}

func isRuntimeArtifact(path string) bool {
	path = filepath.Base(strings.TrimSpace(path))
	switch path {
	case ".codex_orchestrator.lock", ".codex_orchestrator_state.json", ".codex_orchestrator_summary.md":
		return true
	}
	return strings.HasSuffix(path, ".jsonl") ||
		strings.HasSuffix(path, ".stderr.log") ||
		strings.HasSuffix(path, ".result.md") ||
		strings.HasSuffix(path, ".failure.md")
}
