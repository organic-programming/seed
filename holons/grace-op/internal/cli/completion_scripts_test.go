package cli

import (
	"bytes"
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
