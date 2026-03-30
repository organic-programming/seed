package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/organic-programming/grace-op/internal/holons"
)

// cmdCompletion outputs a shell completion script.
func cmdCompletion(args []string) int {
	if len(args) == 0 || args[0] == "zsh" {
		fmt.Print(zshCompletion)
		return 0
	}
	if args[0] == "bash" {
		fmt.Print(bashCompletion)
		return 0
	}
	fmt.Fprintf(os.Stderr, "op completion: unsupported shell %q (use zsh or bash)\n", args[0])
	return 1
}

// cmdComplete is the hidden __complete handler called by shell completions.
// Usage: op __complete <verb> <prefix>
func cmdComplete(args []string) int {
	if len(args) < 1 {
		return 0
	}
	verb := args[0]
	prefix := ""
	if len(args) > 1 {
		prefix = strings.ToLower(args[1])
	}

	switch verb {
	case "build", "run", "install", "check", "test", "clean", "inspect", "show", "do":
		completeSlugs(prefix)
	case "uninstall":
		completeInstalled(prefix)
	default:
		// Complete verbs
		completeVerbs(prefix)
	}
	return 0
}

// completeSlugs lists all discoverable holon slugs matching the prefix.
func completeSlugs(prefix string) {
	local, _ := holons.DiscoverLocalHolons()
	cached, _ := holons.DiscoverCachedHolons()

	seen := make(map[string]struct{})
	for _, h := range append(local, cached...) {
		// Identity-derived slug
		slug := h.Identity.Slug()
		if slug == "" {
			slug = filepath.Base(h.Dir)
		}
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
		if strings.HasPrefix(slug, prefix) {
			fmt.Println(slug)
		}
	}

	// Also include installed binaries from OPBIN
	for _, entry := range holons.DiscoverInOPBIN() {
		// Format is "name -> path"
		name := strings.SplitN(entry, " -> ", 2)[0]
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		if strings.HasPrefix(name, prefix) {
			fmt.Println(name)
		}
	}
}

// completeInstalled lists installed holons in OPBIN matching the prefix.
func completeInstalled(prefix string) {
	for _, entry := range holons.DiscoverInOPBIN() {
		name := strings.SplitN(entry, " -> ", 2)[0]
		if strings.HasPrefix(name, prefix) {
			fmt.Println(name)
		}
	}
}

// completeVerbs lists op subcommands matching the prefix.
func completeVerbs(prefix string) {
	verbs := []string{
		"build", "check", "clean", "completion", "discover", "do",
		"env", "help", "inspect", "install", "list", "mcp",
		"mod", "new", "run", "serve", "show", "test", "tools",
		"uninstall", "version",
	}
	for _, v := range verbs {
		if strings.HasPrefix(v, prefix) {
			fmt.Println(v)
		}
	}
}

const zshCompletion = `#compdef op

_op() {
    local -a commands
    local curcontext="$curcontext" state line

    if (( CURRENT == 2 )); then
        commands=($(op __complete verb "${words[CURRENT]}"))
        _describe 'op commands' commands
        return
    fi

    case "${words[2]}" in
        build|run|install|check|test|clean|inspect|show|do)
            local -a slugs
            slugs=($(op __complete "${words[2]}" "${words[CURRENT]}"))
            _describe 'holons' slugs
            ;;
        uninstall)
            local -a installed
            installed=($(op __complete uninstall "${words[CURRENT]}"))
            _describe 'installed holons' installed
            ;;
    esac
}

compdef _op op
`

const bashCompletion = `_op() {
    local cur prev
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    if [[ ${COMP_CWORD} -eq 1 ]]; then
        COMPREPLY=($(compgen -W "$(op __complete verb "$cur")" -- "$cur"))
        return
    fi

    case "${COMP_WORDS[1]}" in
        build|run|install|check|test|clean|inspect|show|do)
            COMPREPLY=($(compgen -W "$(op __complete "${COMP_WORDS[1]}" "$cur")" -- "$cur"))
            ;;
        uninstall)
            COMPREPLY=($(compgen -W "$(op __complete uninstall "$cur")" -- "$cur"))
            ;;
    esac
}

complete -F _op op
`
