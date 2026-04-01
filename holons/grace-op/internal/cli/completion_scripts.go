package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func patchCompletionCommands(root *cobra.Command) {
	completionCmd := findChildCommand(root, "completion")
	if completionCmd == nil {
		return
	}

	patchCompletionShell(root, completionCmd, "bash", generatePatchedBashCompletion)
	patchCompletionShell(root, completionCmd, "zsh", generatePatchedZshCompletion)
	completionCmd.AddCommand(newCompletionInstallCommand(root))
}

func patchCompletionShell(
	root *cobra.Command,
	completionCmd *cobra.Command,
	shell string,
	generate func(root *cobra.Command, noDesc bool) (string, error),
) {
	shellCmd := findChildCommand(completionCmd, shell)
	if shellCmd == nil {
		return
	}

	shellCmd.RunE = func(cmd *cobra.Command, _ []string) error {
		script, err := generate(root, completionNoDescriptions(root, cmd))
		if err != nil {
			return err
		}
		_, err = io.WriteString(cmd.OutOrStdout(), script)
		return err
	}
}

func completionNoDescriptions(root *cobra.Command, cmd *cobra.Command) bool {
	if root.CompletionOptions.DisableDescriptions {
		return true
	}

	flag := cmd.Flags().Lookup("no-descriptions")
	if flag == nil {
		return false
	}

	noDesc, err := cmd.Flags().GetBool("no-descriptions")
	return err == nil && noDesc
}

func newCompletionInstallCommand(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install [shell]",
		Short: "Install shell completion into the active shell profile",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			overrideProfile, _ := cmd.Flags().GetString("profile")
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			shell, profile, line, err := completionInstallTarget(args, os.Getenv("SHELL"), home, overrideProfile)
			if err != nil {
				return err
			}
			status, err := ensureProfileSnippet(profile, "# op CLI autocompletion", line)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s %s completion in %s\n", status, shell, profile)
			return err
		},
	}
	cmd.Flags().String("profile", "", "explicit shell profile path")
	return cmd
}

func completionInstallTarget(args []string, shellPath string, home string, overrideProfile string) (shell string, profile string, line string, err error) {
	if len(args) > 0 {
		shell = strings.ToLower(strings.TrimSpace(args[0]))
	} else {
		shell = strings.ToLower(strings.TrimSpace(filepath.Base(shellPath)))
	}
	if shell == "" {
		return "", "", "", fmt.Errorf("completion install requires a shell name or SHELL to be set")
	}

	switch shell {
	case "zsh":
		line = `eval "$(op completion zsh)"`
		if strings.TrimSpace(overrideProfile) != "" {
			profile = overrideProfile
		} else {
			profile = filepath.Join(home, ".zshrc")
		}
	case "bash":
		line = `source <(op completion bash)`
		if strings.TrimSpace(overrideProfile) != "" {
			profile = overrideProfile
		} else {
			profile = filepath.Join(home, ".bashrc")
		}
	default:
		return "", "", "", fmt.Errorf("unsupported shell %q; supported shells: zsh, bash", shell)
	}
	return shell, profile, line, nil
}

func ensureProfileSnippet(profile string, comment string, line string) (string, error) {
	content := ""
	if data, err := os.ReadFile(profile); err == nil {
		content = string(data)
	} else if !os.IsNotExist(err) {
		return "", err
	}

	if strings.Contains(content, line) {
		return "already configured", nil
	}

	var b strings.Builder
	if strings.TrimSpace(content) != "" {
		b.WriteString(strings.TrimRight(content, "\n"))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(comment)
	b.WriteString("\n")
	b.WriteString(line)
	b.WriteString("\n")
	if err := os.WriteFile(profile, []byte(b.String()), 0o644); err != nil {
		return "", err
	}
	return "installed", nil
}

func findChildCommand(parent *cobra.Command, name string) *cobra.Command {
	if parent == nil {
		return nil
	}
	for _, child := range parent.Commands() {
		if child.Name() == name {
			return child
		}
	}
	return nil
}

func generatePatchedBashCompletion(root *cobra.Command, noDesc bool) (string, error) {
	var buf bytes.Buffer
	if err := root.GenBashCompletionV2(&buf, !noDesc); err != nil {
		return "", err
	}
	return patchBashCompletionScript(buf.String(), root.Name())
}

func patchBashCompletionScript(script string, name string) (string, error) {
	helperAnchor := fmt.Sprintf("__%s_handle_completion_types() {", name)
	helperBlock := fmt.Sprintf(`
__%[1]s_is_literal_json_completion() {
    local comp="$1"
    [[ $comp == "'{"*"}'" || $comp == "'["*"]'" ]]
}

__%[1]s_escape_completion_candidates() {
    local comp
    for comp in "$@"; do
        if __%[1]s_is_literal_json_completion "$comp"; then
            printf "%%s\n" "$comp"
        else
            printf "%%q\n" "$comp"
        fi
    done
}

`, name)
	if !strings.Contains(script, helperAnchor) {
		return "", fmt.Errorf("bash completion patch anchor %q not found", helperAnchor)
	}
	script = strings.Replace(script, helperAnchor, helperBlock+helperAnchor, 1)

	replacements := []struct {
		old string
		new string
	}{
		{
			old: `printf "%q\n" "${completions[@]%%$tab*}"`,
			new: fmt.Sprintf(`__%s_escape_completion_candidates "${completions[@]%%%%$tab*}"`, name),
		},
		{
			old: `printf "%q\n" "${completions[@]}"`,
			new: fmt.Sprintf(`__%s_escape_completion_candidates "${completions[@]}"`, name),
		},
		{
			old: `printf "%q\n" "${COMPREPLY[@]}"`,
			new: fmt.Sprintf(`__%s_escape_completion_candidates "${COMPREPLY[@]}"`, name),
		},
		{
			old: `printf -v comp "%q" "${compline%%$tab*}" &>/dev/null || comp=$(printf "%q" "${compline%%$tab*}")`,
			new: fmt.Sprintf(`comp=$(__%s_escape_completion_candidates "${compline%%%%$tab*}")`, name),
		},
		{
			old: `COMPREPLY[0]=$(printf "%q" "${COMPREPLY[0]}")`,
			new: fmt.Sprintf(`COMPREPLY[0]=$(__%s_escape_completion_candidates "${COMPREPLY[0]}")`, name),
		},
		{
			old: `COMPREPLY[0]=$(printf "%q" "${COMPREPLY[0]%%$tab*}")`,
			new: fmt.Sprintf(`COMPREPLY[0]=$(__%s_escape_completion_candidates "${COMPREPLY[0]%%%%$tab*}")`, name),
		},
	}

	for _, replacement := range replacements {
		if !strings.Contains(script, replacement.old) {
			return "", fmt.Errorf("bash completion patch snippet not found: %q", replacement.old)
		}
		script = strings.ReplaceAll(script, replacement.old, replacement.new)
	}

	return script, nil
}

func generatePatchedZshCompletion(root *cobra.Command, noDesc bool) (string, error) {
	var buf bytes.Buffer
	if noDesc {
		if err := root.GenZshCompletionNoDesc(&buf); err != nil {
			return "", err
		}
	} else {
		if err := root.GenZshCompletion(&buf); err != nil {
			return "", err
		}
	}
	return patchZshCompletionScript(buf.String(), root.Name())
}

func patchZshCompletionScript(script string, name string) (string, error) {
	helperAnchor := fmt.Sprintf("_%s()", name)
	helperBlock := fmt.Sprintf(`
__%[1]s_is_literal_json_completion()
{
    local comp="$1"
    [[ "$comp" == "'{"*"}'" || "$comp" == "'["*"]'" ]]
}

`, name)
	if !strings.Contains(script, helperAnchor) {
		return "", fmt.Errorf("zsh completion patch anchor %q not found", helperAnchor)
	}
	script = strings.Replace(script, helperAnchor, helperBlock+helperAnchor, 1)

	localArrayOld := "    local -a completions\n"
	localArrayNew := "    local -a completions literalCompletions\n"
	if !strings.Contains(script, localArrayOld) {
		return "", fmt.Errorf("zsh completion patch snippet not found: %q", strings.TrimSpace(localArrayOld))
	}
	script = strings.Replace(script, localArrayOld, localArrayNew, 1)

	addCompletionOld := fmt.Sprintf(`        if [ -n "$comp" ]; then
            # If requested, completions are returned with a description.
            # The description is preceded by a TAB character.
            # For zsh's _describe, we need to use a : instead of a TAB.
            # We first need to escape any : as part of the completion itself.
            comp=${comp//:/\\:}

            local tab="$(printf '\t')"
            comp=${comp//$tab/:}

            __%[1]s_debug "Adding completion: ${comp}"
            completions+=${comp}
            lastComp=$comp
        fi`, name)
	addCompletionNew := fmt.Sprintf(`        if [ -n "$comp" ]; then
            if __%[1]s_is_literal_json_completion "$comp"; then
                __%[1]s_debug "Adding literal completion: ${comp}"
                literalCompletions+=${comp}
                lastComp=$comp
                continue
            fi

            # If requested, completions are returned with a description.
            # The description is preceded by a TAB character.
            # For zsh's _describe, we need to use a : instead of a TAB.
            # We first need to escape any : as part of the completion itself.
            comp=${comp//:/\\:}

            local tab="$(printf '\t')"
            comp=${comp//$tab/:}

            __%[1]s_debug "Adding completion: ${comp}"
            completions+=${comp}
            lastComp=$comp
        fi`, name)
	if !strings.Contains(script, addCompletionOld) {
		return "", fmt.Errorf("zsh completion add-completion block not found")
	}
	script = strings.Replace(script, addCompletionOld, addCompletionNew, 1)

	describeOld := fmt.Sprintf(`    else
        __%[1]s_debug "Calling _describe"
        if eval _describe $keepOrder "completions" completions $flagPrefix $noSpace; then
            __%[1]s_debug "_describe found some completions"

            # Return the success of having called _describe
            return 0
        else
            __%[1]s_debug "_describe did not find completions."
            __%[1]s_debug "Checking if we should do file completion."
            if [ $((directive & shellCompDirectiveNoFileComp)) -ne 0 ]; then
                __%[1]s_debug "deactivating file completion"

                # We must return an error code here to let zsh know that there were no
                # completions found by _describe; this is what will trigger other
                # matching algorithms to attempt to find completions.
                # For example zsh can match letters in the middle of words.
                return 1
            else
                # Perform file completion
                __%[1]s_debug "Activating file completion"

                # We must return the result of this command, so it must be the
                # last command, or else we must store its result to return it.
                _arguments '*:filename:_files'" ${flagPrefix}"
            fi
        fi
    fi`, name)
	describeNew := fmt.Sprintf(`    else
        local literalResult=1
        local describeResult=1

        if [ ${#literalCompletions} -ne 0 ]; then
            __%[1]s_debug "Calling compadd -Q for literal completions"
            compadd -Q -- "${literalCompletions[@]}"
            literalResult=$?
        fi

        if [ ${#completions} -ne 0 ]; then
            __%[1]s_debug "Calling _describe"
            if eval _describe $keepOrder "completions" completions $flagPrefix $noSpace; then
                __%[1]s_debug "_describe found some completions"
                describeResult=0
            else
                __%[1]s_debug "_describe did not find completions."
            fi
        fi

        if [ $literalResult -eq 0 ] || [ $describeResult -eq 0 ]; then
            __%[1]s_debug "Completion choices added."
            return 0
        fi

        __%[1]s_debug "Checking if we should do file completion."
        if [ $((directive & shellCompDirectiveNoFileComp)) -ne 0 ]; then
            __%[1]s_debug "deactivating file completion"

            # We must return an error code here to let zsh know that there were no
            # completions found; this is what will trigger other
            # matching algorithms to attempt to find completions.
            # For example zsh can match letters in the middle of words.
            return 1
        else
            # Perform file completion
            __%[1]s_debug "Activating file completion"

            # We must return the result of this command, so it must be the
            # last command, or else we must store its result to return it.
            _arguments '*:filename:_files'" ${flagPrefix}"
        fi
    fi`, name)
	if !strings.Contains(script, describeOld) {
		return "", fmt.Errorf("zsh completion describe block not found")
	}
	script = strings.Replace(script, describeOld, describeNew, 1)

	return script, nil
}
