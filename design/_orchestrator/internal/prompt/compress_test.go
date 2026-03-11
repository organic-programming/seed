package prompt

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompressHistoryWritesAndReusesCache(t *testing.T) {
	originalExecCommand := execCommand
	execCommand = helperExecCommand
	defer func() { execCommand = originalExecCommand }()

	root := t.TempDir()
	setDir := filepath.Join(root, "v1.0")
	if err := os.MkdirAll(setDir, 0o755); err != nil {
		t.Fatalf("mkdir set dir: %v", err)
	}

	resultFile := filepath.Join(setDir, "task01.result.md")
	if err := os.WriteFile(resultFile, []byte("implementation summary"), 0o644); err != nil {
		t.Fatalf("write result file: %v", err)
	}

	t.Setenv("PROMPT_TEST_OUTPUT", "summary one")
	summary, err := CompressHistory([]string{resultFile}, setDir)
	if err != nil {
		t.Fatalf("CompressHistory returned error: %v", err)
	}
	if strings.TrimSpace(summary) != "summary one" {
		t.Fatalf("unexpected summary: %q", summary)
	}

	t.Setenv("PROMPT_TEST_OUTPUT", "summary two")
	summary, err = CompressHistory([]string{resultFile}, setDir)
	if err != nil {
		t.Fatalf("CompressHistory returned error on cache reuse: %v", err)
	}
	if strings.TrimSpace(summary) != "summary one" {
		t.Fatalf("expected cached summary, got %q", summary)
	}
}

func helperExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestPromptHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = append(os.Environ(), "GO_WANT_PROMPT_HELPER=1")
	return cmd
}

func TestPromptHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_PROMPT_HELPER") != "1" {
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

	var outputFile string
	for i := 0; i < len(args); i++ {
		if args[i] == "-o" && i+1 < len(args) {
			outputFile = args[i+1]
		}
	}
	if outputFile == "" {
		fmt.Fprintln(os.Stderr, "missing -o")
		os.Exit(1)
	}
	if err := os.WriteFile(outputFile, []byte(os.Getenv("PROMPT_TEST_OUTPUT")+"\n"), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}
