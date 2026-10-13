package holons

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// installWindowsSelf is called only for OP's Windows self-install. OPBIN remains
// the canonical package/launcher location, but an OP invoked from another
// directory must also be updated there: changing a child process's PATH cannot
// change the command that its parent shell will invoke next.
func installWindowsSelf(artifact, installed, binaryName string) ([]string, error) {
	if info, err := os.Stat(PackageBinaryPath(artifact, binaryName)); err != nil {
		return nil, fmt.Errorf("self-install executable: %w", err)
	} else if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("self-install executable is not a regular file")
	}
	running, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locate invoked executable: %w", err)
	}
	launcher := filepath.Join(filepath.Dir(installed), hostExecutableName(binaryName))
	packageBinary := PackageBinaryPath(installed, binaryName)
	paths := []string{launcher}
	// The Install API also runs inside tests or another embedding executable.
	// Only the current OP image is a self-update destination. Never scan PATH
	// and overwrite unrelated executables merely because they have this name.
	if strings.EqualFold(filepath.Base(running), hostExecutableName(binaryName)) &&
		!sameWindowsInstallPath(running, launcher) && !sameWindowsInstallPath(running, packageBinary) {
		paths = append(paths, running)
	}
	if err := installWindowsSelfPackage(artifact, installed); err != nil {
		return nil, err
	}
	notes := []string{"installed into " + installed}
	for _, path := range paths {
		if err := installExecutableCopy(packageBinary, path); err != nil {
			return notes, fmt.Errorf("install executable %s: %w", path, err)
		}
		notes = append(notes, "installed executable "+path)
	}
	return notes, nil
}

func sameWindowsInstallPath(left, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

// Stage the complete package before replacing it. In particular, do not
// RemoveAll the installed package when OP itself is running from its bin
// directory. Windows permits renaming the mapped image but keeps it alive
// until the process exits. A later self-install reclaims any retired package.
// This handles process lifetime; it does not promise power-loss durability.
func installWindowsSelfPackage(src, dst string) error {
	parent, name := filepath.Dir(dst), filepath.Base(dst)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	removeRetiredSelfPackages(dst)
	staged, err := os.MkdirTemp(parent, "."+name+".new-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staged)
	if err := copyDir(src, staged); err != nil {
		return fmt.Errorf("stage self-install package: %w", err)
	}
	previous := ""
	if info, err := os.Lstat(dst); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("self-install package is not a regular directory: %s", dst)
		}
		previous, err = os.MkdirTemp(parent, "."+name+".previous-")
		if err != nil {
			return err
		}
		if err := os.Remove(previous); err != nil {
			return err
		}
		if err := os.Rename(dst, previous); err != nil {
			return fmt.Errorf("retire self-install package: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(staged, dst); err != nil {
		if previous != "" {
			if restoreErr := os.Rename(previous, dst); restoreErr != nil {
				return errors.Join(err, fmt.Errorf("restore self-install package %s: %w", previous, restoreErr))
			}
		}
		return err
	}
	if previous != "" {
		_ = os.RemoveAll(previous) // The running image may remain until exit.
	}
	return nil
}

func removeRetiredSelfPackages(installed string) {
	parent := filepath.Dir(installed)
	entries, err := os.ReadDir(parent)
	if err != nil {
		return
	}
	prefix := "." + filepath.Base(installed) + ".previous-"
	for _, entry := range entries {
		if entry.IsDir() && entry.Type()&os.ModeSymlink == 0 && strings.HasPrefix(entry.Name(), prefix) {
			_ = os.RemoveAll(filepath.Join(parent, entry.Name()))
		}
	}
}
