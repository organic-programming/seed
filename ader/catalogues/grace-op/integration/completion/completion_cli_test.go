package completion_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestCompletion_CLI_GeneratesScripts(t *testing.T) {
	sb := integration.NewSandbox(t)
	cases := []struct {
		shell string
		want  []string
	}{
		{shell: "bash", want: []string{"__op_is_literal_json_completion()", "__op_escape_completion_candidates()"}},
		{shell: "zsh", want: []string{"__op_is_literal_json_completion()", "literalCompletions"}},
		{shell: "fish", want: []string{"complete", "op"}},
		{shell: "powershell", want: []string{"Register-ArgumentCompleter", "op"}},
	}

	for _, tc := range cases {
		t.Run(tc.shell, func(t *testing.T) {
			result := sb.RunOP(t, "completion", tc.shell)
			integration.RequireSuccess(t, result)
			for _, want := range tc.want {
				integration.RequireContains(t, result.Stdout, want)
			}
		})
	}
}

func TestCompletion_CLI_InstallZshIsIdempotent(t *testing.T) {
	sb := integration.NewSandbox(t)
	home := t.TempDir()

	first := sb.RunOPWithOptions(t, integration.RunOptions{
		Env: []string{"HOME=" + home, "SHELL=/bin/zsh"},
	}, "completion", "install", "zsh")
	integration.RequireSuccess(t, first)
	profilePath := filepath.Join(home, ".zshrc")
	data, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("read .zshrc: %v", err)
	}
	if !strings.Contains(string(data), `eval "$(op completion zsh)"`) {
		t.Fatalf(".zshrc missing completion line: %q", string(data))
	}
	integration.RequireContains(t, first.Stdout, "installed zsh completion")

	second := sb.RunOPWithOptions(t, integration.RunOptions{
		Env: []string{"HOME=" + home, "SHELL=/bin/zsh"},
	}, "completion", "install")
	integration.RequireSuccess(t, second)
	data, err = os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("read .zshrc after second install: %v", err)
	}
	if got := strings.Count(string(data), `eval "$(op completion zsh)"`); got != 1 {
		t.Fatalf("completion line count = %d, want 1", got)
	}
	integration.RequireContains(t, second.Stdout, "already configured zsh completion")
}

func TestCompletion_CLI_InstallBashWritesBashrc(t *testing.T) {
	sb := integration.NewSandbox(t)
	home := t.TempDir()

	result := sb.RunOPWithOptions(t, integration.RunOptions{
		Env: []string{"HOME=" + home, "SHELL=/bin/bash"},
	}, "completion", "install", "bash")
	integration.RequireSuccess(t, result)

	profilePath := filepath.Join(home, ".bashrc")
	data, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("read .bashrc: %v", err)
	}
	if !strings.Contains(string(data), `source <(op completion bash)`) {
		t.Fatalf(".bashrc missing completion line: %q", string(data))
	}
	integration.RequireContains(t, result.Stdout, "installed bash completion")
}
