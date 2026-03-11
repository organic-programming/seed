package preflight

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/organic-programming/codex-orchestrator/internal/cli"
)

func TestRunPassesForHealthyEnvironment(t *testing.T) {
	originalLookPath := execLookPath
	originalCommand := execCommand
	execLookPath = func(file string) (string, error) { return "/usr/bin/" + file, nil }
	execCommand = helperExecCommand
	defer func() {
		execLookPath = originalLookPath
		execCommand = originalCommand
	}()

	root := t.TempDir()
	setDir := filepath.Join(root, "✅ v1.0")
	if err := os.MkdirAll(setDir, 0o755); err != nil {
		t.Fatalf("mkdir set dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(setDir, "_TASKS.md"), []byte("| # | File | Summary | Depends on | Status |\n|---|---|---|---|---|\n| 01 | [TASK01](./task01.md) | x | — | — |\n"), 0o644); err != nil {
		t.Fatalf("write _TASKS.md: %v", err)
	}

	if err := Run(cli.Config{Root: root, Model: "gpt-5.4", Sets: []string{"v1.0"}}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}

func TestRunFailsForDirtyGitRepo(t *testing.T) {
	originalLookPath := execLookPath
	originalCommand := execCommand
	execLookPath = func(file string) (string, error) { return "/usr/bin/" + file, nil }
	execCommand = helperExecCommand
	defer func() {
		execLookPath = originalLookPath
		execCommand = originalCommand
	}()

	root := t.TempDir()
	setDir := filepath.Join(root, "v1.0")
	if err := os.MkdirAll(setDir, 0o755); err != nil {
		t.Fatalf("mkdir set dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(setDir, "_TASKS.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write _TASKS.md: %v", err)
	}

	t.Setenv("PREFLIGHT_HELPER_GIT_STATUS", " M main.go\n")
	err := Run(cli.Config{Root: root, Model: "gpt-5.4", Sets: []string{"v1.0"}})
	if err == nil {
		t.Fatal("expected dirty repo error, got nil")
	}
	if !strings.Contains(err.Error(), "uncommitted changes") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func helperExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestPreflightHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = append(os.Environ(), "GO_WANT_PREFLIGHT_HELPER=1")
	return cmd
}

func TestPreflightHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_PREFLIGHT_HELPER") != "1" {
		return
	}

	args := os.Args
	separator := 0
	for i, arg := range args {
		if arg == "--" {
			separator = i
			break
		}
	}
	args = args[separator+1:]
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "missing command")
		os.Exit(1)
	}

	command := args[0]
	switch command {
	case "codex":
		if len(args) >= 3 && args[1] == "login" && args[2] == "status" {
			os.Exit(0)
		}
		if len(args) >= 2 && args[1] == "exec" {
			fmt.Fprintln(os.Stdout, "OK")
			os.Exit(0)
		}
	case "git":
		if len(args) >= 3 && args[1] == "status" && args[2] == "--porcelain" {
			fmt.Fprint(os.Stdout, os.Getenv("PREFLIGHT_HELPER_GIT_STATUS"))
			os.Exit(0)
		}
		if len(args) >= 4 && args[1] == "submodule" && args[2] == "status" && args[3] == "--recursive" {
			fmt.Fprint(os.Stdout, os.Getenv("PREFLIGHT_HELPER_SUBMODULE_STATUS"))
			os.Exit(0)
		}
	}

	fmt.Fprintf(os.Stderr, "unexpected helper invocation: %v", args)
	os.Exit(1)
}
