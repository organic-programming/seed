//go:build windows

package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsPowerShellStepPreservesLiteralArguments(t *testing.T) {
	script := filepath.Join(t.TempDir(), "native step.ps1")
	if err := os.WriteFile(script, []byte("[Console]::Write($args[0])"), 0o600); err != nil {
		t.Fatal(err)
	}
	argument := "path with spaces & literal $variable ; `text`"
	cmd, err := buildStepCommand(StepSpec{Script: script, Args: []string{argument}})
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("PowerShell step: %v\n%s", err, output)
	}
	if string(output) != argument {
		t.Fatalf("argument = %q, want %q", output, argument)
	}
	for _, arg := range cmd.Args {
		if strings.Contains(strings.ToLower(arg), "executionpolicy") {
			t.Fatal("must respect host script execution policy")
		}
	}
}

func TestWindowsNativeStepRequiresPEImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fake.exe")
	if err := os.WriteFile(path, []byte("not a portable executable"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := buildStepCommand(StepSpec{Script: path}); err == nil {
		t.Fatal("accepted text as a Windows executable")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buildStepCommand(StepSpec{Script: executable}); err != nil {
		t.Fatalf("rejected test PE image: %v", err)
	}
}
