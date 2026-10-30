//go:build !windows

package holons

import (
	"errors"
	"os"
	"path/filepath"
)

// Resolve existing ancestors, too: the output itself may not exist yet, while
// its checkout can already be reached through a symlink (e.g. /var on macOS).
func canonicalArtifactPath(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var suffix []string
	for {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			resolved, err = canonicalExistingArtifactPath(resolved)
			if err != nil {
				return "", err
			}
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", err
		}
		suffix = append(suffix, filepath.Base(path))
		path = parent
	}
}
