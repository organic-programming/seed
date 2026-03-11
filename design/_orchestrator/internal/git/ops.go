package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type Ops struct {
	Root string
}

func (o *Ops) Rename(from, to string) error {
	fromRel, err := relToRoot(o.Root, from)
	if err != nil {
		return err
	}
	toRel, err := relToRoot(o.Root, to)
	if err != nil {
		return err
	}
	if fromRel == toRel {
		return nil
	}
	_, err = o.run("git", "-C", o.Root, "mv", fromRel, toRel)
	return err
}

func (o *Ops) AddCommitPush(msg string, files ...string) error {
	args := []string{"-C", o.Root, "add"}
	if len(files) == 0 {
		args = append(args, "-A")
	} else {
		for _, file := range files {
			rel, err := relToRoot(o.Root, file)
			if err != nil {
				return err
			}
			args = append(args, rel)
		}
	}
	if _, err := o.run("git", args...); err != nil {
		return err
	}

	diff, err := o.run("git", "-C", o.Root, "diff", "--cached", "--name-only")
	if err != nil {
		return err
	}
	if strings.TrimSpace(diff) == "" {
		return nil
	}

	if _, err := o.run("git", "-C", o.Root, "commit", "-m", msg); err != nil {
		return err
	}

	if err := o.pushCurrentBranch(); err != nil {
		return err
	}
	return nil
}

func (o *Ops) Tag(name, msg string) error {
	if _, err := o.run("git", "-C", o.Root, "tag", "-a", name, "-m", msg); err != nil {
		return err
	}
	if hasOriginRemote(o.Root) {
		if _, err := o.run("git", "-C", o.Root, "push", "origin", name); err != nil {
			return err
		}
	}
	return nil
}

func EnsureConsistency(root, project, setName string) error {
	targetBranch := fmt.Sprintf("%s-%s-dev", project, setName)
	repos := []string{root}
	submodules, err := ListSubmodulePaths(root)
	if err != nil {
		return err
	}
	repos = append(repos, submodules...)

	for _, repo := range repos {
		current, err := currentBranch(repo)
		if err != nil {
			return err
		}
		if !strings.HasSuffix(current, "-dev") {
			return fmt.Errorf("repository %s is not on a -dev branch (%s)", repo, current)
		}
		if current == targetBranch {
			continue
		}
		if localBranchExists(repo, targetBranch) {
			if _, err := runGit(repo, "checkout", targetBranch); err != nil {
				return err
			}
		} else {
			if _, err := runGit(repo, "checkout", "-b", targetBranch); err != nil {
				return err
			}
		}
		if hasOriginRemote(repo) {
			_, _ = runGit(repo, "push", "-u", "origin", targetBranch)
		}
	}
	return nil
}

func (o *Ops) run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(output.String()))
	}
	return output.String(), nil
}

func (o *Ops) pushCurrentBranch() error {
	if !hasOriginRemote(o.Root) {
		return nil
	}

	if branch, err := currentBranch(o.Root); err == nil {
		if _, err := o.run("git", "-C", o.Root, "push", "-u", "origin", branch); err == nil {
			return nil
		}
	}
	_, err := o.run("git", "-C", o.Root, "push")
	return err
}

func currentBranch(repo string) (string, error) {
	output, err := runGit(repo, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func localBranchExists(repo, branch string) bool {
	_, err := runGit(repo, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

func hasOriginRemote(repo string) bool {
	cmd := exec.Command("git", "-C", repo, "remote", "get-url", "origin")
	return cmd.Run() == nil
}

func runGit(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(output.String()))
	}
	return output.String(), nil
}

func relToRoot(root, path string) (string, error) {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return path, nil
	}
	root = filepath.Clean(root)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	return rel, nil
}
