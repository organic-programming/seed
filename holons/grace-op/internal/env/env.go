package env

import (
	"os"
	"path/filepath"
	"strings"
)

// OPPATH returns the user-local runtime home used by op.
func OPPATH() string {
	if runtimeHome := strings.TrimSpace(os.Getenv("OPPATH")); runtimeHome != "" {
		return cleanOrFallback(runtimeHome)
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return cleanOrFallback(".op")
	}
	return cleanOrFallback(filepath.Join(home, ".op"))
}

// OPBIN returns the canonical install directory for holon binaries.
func OPBIN() string {
	if binaryHome := strings.TrimSpace(os.Getenv("OPBIN")); binaryHome != "" {
		return cleanOrFallback(binaryHome)
	}
	return cleanOrFallback(filepath.Join(OPPATH(), "bin"))
}

// CacheDir returns the dependency cache used by op.
func CacheDir() string {
	return cleanOrFallback(filepath.Join(OPPATH(), "cache"))
}

// Root returns the current effective root for commands run from cwd.
// Setting OPROOT overrides cwd (used by --root).
func Root() string {
	if override := strings.TrimSpace(os.Getenv("OPROOT")); override != "" {
		return cleanOrFallback(override)
	}
	cwd, err := os.Getwd()
	if err != nil || strings.TrimSpace(cwd) == "" {
		return "."
	}
	return cleanOrFallback(cwd)
}

// Init creates the runtime home and binary directory if they do not exist.
func Init() error {
	for _, dir := range []string{OPPATH(), OPBIN(), CacheDir()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// ShellSnippet returns a shell fragment suitable for zsh/bash startup files.
func ShellSnippet() string {
	return strings.Join([]string{
		`export OPPATH="${OPPATH:-$HOME/.op}"`,
		`export OPBIN="${OPBIN:-$OPPATH/bin}"`,
		`mkdir -p "$OPBIN"`,
		`export PATH="$OPBIN:$PATH"`,
	}, "\n")
}

func cleanOrFallback(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return filepath.Clean(path)
}
