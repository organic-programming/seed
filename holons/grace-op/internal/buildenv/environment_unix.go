//go:build !windows

package buildenv

import (
	"os"
	"os/exec"
)

// Prepare intentionally leaves the existing non-Windows environment unchanged.
func Prepare(string) ([]string, error)               { return nil, nil }
func Lookup(_ []string, name string) (string, error) { return exec.LookPath(name) }
func Value(_ []string, name string) string           { return os.Getenv(name) }
func Merge(base, extra []string) []string            { return append(append([]string{}, base...), extra...) }
func Command(dir string, args, extra []string) (*exec.Cmd, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	if len(extra) != 0 {
		cmd.Env = append(os.Environ(), extra...)
	}
	return cmd, nil
}
