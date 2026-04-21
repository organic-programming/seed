package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompletionCommandPatchesBashScriptForLiteralJSON(t *testing.T) {
	root := newRootCmd("0.1.0-test")
	completionCmd := mustFindSubcommand(t, root, "completion")
	bashCmd := mustFindSubcommand(t, completionCmd, "bash")

	var out bytes.Buffer
	bashCmd.SetOut(&out)
	if err := bashCmd.RunE(bashCmd, nil); err != nil {
		t.Fatalf("bash completion RunE returned error: %v", err)
	}

	script := out.String()
	for _, want := range []string{
		"__op_is_literal_json_completion()",
		"__op_escape_completion_candidates()",
		`COMPREPLY[0]=$(__op_escape_completion_candidates "${COMPREPLY[0]%%$tab*}")`,
		`comp=$(__op_escape_completion_candidates "${compline%%$tab*}")`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("bash completion script missing %q", want)
		}
	}
}

func TestCompletionCommandPatchesZshScriptForLiteralJSON(t *testing.T) {
	root := newRootCmd("0.1.0-test")
	completionCmd := mustFindSubcommand(t, root, "completion")
	zshCmd := mustFindSubcommand(t, completionCmd, "zsh")

	var out bytes.Buffer
	zshCmd.SetOut(&out)
	if err := zshCmd.RunE(zshCmd, nil); err != nil {
		t.Fatalf("zsh completion RunE returned error: %v", err)
	}

	script := out.String()
	for _, want := range []string{
		"__op_is_literal_json_completion()",
		"local -a completions literalCompletions",
		`compadd -Q -- "${literalCompletions[@]}"`,
		`literalCompletions+=${comp}`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("zsh completion script missing %q", want)
		}
	}
}

func TestCompletionInstallZshIsIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/zsh")

	root := newRootCmd("0.1.0-test")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"completion", "install", "zsh"})
	if err := root.Execute(); err != nil {
		t.Fatalf("completion install zsh returned error: %v", err)
	}

	profilePath := filepath.Join(home, ".zshrc")
	data, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("read .zshrc: %v", err)
	}
	if !strings.Contains(string(data), `eval "$(op completion zsh)"`) {
		t.Fatalf(".zshrc missing completion line: %q", string(data))
	}
	if !strings.Contains(out.String(), "installed zsh completion") {
		t.Fatalf("stdout = %q, want install confirmation", out.String())
	}

	out.Reset()
	root = newRootCmd("0.1.0-test")
	root.SetOut(&out)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"completion", "install"})
	if err := root.Execute(); err != nil {
		t.Fatalf("completion install autodetect returned error: %v", err)
	}
	data, err = os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("read .zshrc after second install: %v", err)
	}
	if got := strings.Count(string(data), `eval "$(op completion zsh)"`); got != 1 {
		t.Fatalf("completion line count = %d, want 1", got)
	}
	if !strings.Contains(out.String(), "already configured zsh completion") {
		t.Fatalf("stdout = %q, want idempotent confirmation", out.String())
	}
}

func TestCompletionInstallBashWritesBashrc(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/bash")

	root := newRootCmd("0.1.0-test")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"completion", "install", "bash"})
	if err := root.Execute(); err != nil {
		t.Fatalf("completion install bash returned error: %v", err)
	}

	profilePath := filepath.Join(home, ".bashrc")
	data, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("read .bashrc: %v", err)
	}
	if !strings.Contains(string(data), `source <(op completion bash)`) {
		t.Fatalf(".bashrc missing completion line: %q", string(data))
	}
	if !strings.Contains(out.String(), "installed bash completion") {
		t.Fatalf("stdout = %q, want install confirmation", out.String())
	}
}
