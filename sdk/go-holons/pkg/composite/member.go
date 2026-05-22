// Package composite provides helpers for composite holons.
package composite

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var executablePath = os.Executable

// Member resolves a declared member's binary relative to the calling
// composite's own executable. It returns the absolute path to the member's
// primary binary.
func Member(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("member id is required")
	}

	self, err := executablePath()
	if err != nil {
		return "", err
	}
	if self == "" {
		return "", fmt.Errorf("executable path is empty")
	}
	if !filepath.IsAbs(self) {
		self, err = filepath.Abs(self)
		if err != nil {
			return "", err
		}
	}

	memberDir := filepath.Join(filepath.Dir(self), "holons", id)
	entries, err := os.ReadDir(memberDir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if runtime.GOOS == "windows" && strings.HasSuffix(strings.ToLower(entry.Name()), ".exe") {
			return filepath.Join(memberDir, entry.Name()), nil
		}
		info, err := entry.Info()
		if err != nil {
			return "", err
		}
		if info.Mode().Perm()&0o111 != 0 {
			return filepath.Join(memberDir, entry.Name()), nil
		}
	}
	return "", fmt.Errorf("no executable found in %s", memberDir)
}
