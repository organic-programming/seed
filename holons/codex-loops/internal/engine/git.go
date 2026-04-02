package engine

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type GitOps interface {
	RepoRoot() (string, error)
	CurrentCommit() (string, error)
	CheckoutNewBranch(name string) error
	ResetHard(commit string) error
	CommitAll(message string) error
	SavePatch(path string) error
}

type shellGitOps struct {
	dir string
}

func newShellGitOps(dir string) *shellGitOps {
	return &shellGitOps{dir: dir}
}

func (g *shellGitOps) RepoRoot() (string, error) {
	root, err := runGitOutput(g.dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	root = strings.TrimSpace(root)
	g.dir = root
	return root, nil
}

func (g *shellGitOps) CurrentCommit() (string, error) {
	commit, err := runGitOutput(g.dir, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(commit), nil
}

func (g *shellGitOps) CheckoutNewBranch(name string) error {
	if _, err := runGitOutput(g.dir, "checkout", "-b", name); err != nil {
		return fmt.Errorf("git checkout -b %s: %w", name, err)
	}
	return nil
}

func (g *shellGitOps) ResetHard(commit string) error {
	if _, err := runGitOutput(g.dir, "reset", "--hard", commit); err != nil {
		return fmt.Errorf("git reset --hard %s: %w", commit, err)
	}
	return nil
}

func (g *shellGitOps) CommitAll(message string) error {
	if _, err := runGitOutput(g.dir, "add", "-A"); err != nil {
		return fmt.Errorf("git add -A: %w", err)
	}
	if _, err := runGitOutput(g.dir, "commit", "-m", message); err != nil {
		return fmt.Errorf("git commit -m %q: %w", message, err)
	}
	return nil
}

func (g *shellGitOps) SavePatch(path string) error {
	diff, err := runGitOutput(g.dir, "diff")
	if err != nil {
		return fmt.Errorf("git diff: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(diff), 0o644); err != nil {
		return fmt.Errorf("write patch %s: %w", path, err)
	}
	return nil
}

func gitRepoRoot() (string, error) {
	root, err := runGitOutput("", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	return strings.TrimSpace(root), nil
}

func gitCurrentCommit() (string, error) {
	commit, err := runGitOutput("", "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(commit), nil
}

func gitCheckoutNewBranch(name string) error {
	if _, err := runGitOutput("", "checkout", "-b", name); err != nil {
		return fmt.Errorf("git checkout -b %s: %w", name, err)
	}
	return nil
}

func gitCheckoutBranch(name string) error {
	if _, err := runGitOutput("", "checkout", name); err != nil {
		return fmt.Errorf("git checkout %s: %w", name, err)
	}
	return nil
}

func gitResetHard(commit string) error {
	if _, err := runGitOutput("", "reset", "--hard", commit); err != nil {
		return fmt.Errorf("git reset --hard %s: %w", commit, err)
	}
	return nil
}

func gitCommitAll(message string) error {
	if _, err := runGitOutput("", "add", "-A"); err != nil {
		return fmt.Errorf("git add -A: %w", err)
	}
	if _, err := runGitOutput("", "commit", "-m", message); err != nil {
		return fmt.Errorf("git commit -m %q: %w", message, err)
	}
	return nil
}

func gitDiff() (string, error) {
	diff, err := runGitOutput("", "diff")
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return diff, nil
}

func gitSavePatch(path string) error {
	diff, err := gitDiff()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(diff), 0o644); err != nil {
		return fmt.Errorf("write patch %s: %w", path, err)
	}
	return nil
}

func runGitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if strings.TrimSpace(dir) != "" {
		cmd.Dir = dir
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return "", err
	}
	return stdout.String(), nil
}
