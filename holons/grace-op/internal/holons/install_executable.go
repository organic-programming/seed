package holons

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// installExecutableCopy publishes a standalone Windows launcher without a
// privileged symlink. Renaming the previous image allows an already running
// launcher to finish; Windows may retain that image until its process exits.
func installExecutableCopy(src, dst string) error {
	removeRetiredExecutableCopies(dst)
	staged, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".new-*")
	if err != nil {
		return err
	}
	stagedPath := staged.Name()
	if err := staged.Close(); err != nil {
		_ = os.Remove(stagedPath)
		return err
	}
	defer os.Remove(stagedPath)
	if err := copyFile(src, stagedPath); err != nil {
		return err
	}

	previous, err := retireInstalledExecutable(dst)
	if err != nil {
		return err
	}
	if err := os.Rename(stagedPath, dst); err != nil {
		if previous != "" {
			if rollbackErr := os.Rename(previous, dst); rollbackErr != nil {
				return errors.Join(err, fmt.Errorf("restore previous executable %s: %w", previous, rollbackErr))
			}
		}
		return err
	}
	if previous != "" {
		_ = os.Remove(previous) // A running image is released after its process exits.
	}
	return nil
}

// Earlier updates may have left an image in use. Remove only our regular
// backup files; a still-running image remains protected by Windows.
func removeRetiredExecutableCopies(path string) {
	dir := filepath.Dir(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	prefix := "." + filepath.Base(path) + ".previous-"
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.HasPrefix(entry.Name(), prefix) {
			_ = os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
}

func removeInstalledExecutable(path string) error {
	previous, err := retireInstalledExecutable(path)
	if err != nil {
		return err
	}
	if previous != "" {
		_ = os.Remove(previous)
	}
	return nil
}

func retireInstalledExecutable(path string) (string, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("launcher path is a directory: %s", path)
	}
	previous, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".previous-*")
	if err != nil {
		return "", err
	}
	previousPath := previous.Name()
	if err := previous.Close(); err != nil {
		_ = os.Remove(previousPath)
		return "", err
	}
	if err := os.Remove(previousPath); err != nil {
		return "", err
	}
	if err := os.Rename(path, previousPath); err != nil {
		return "", err
	}
	return previousPath, nil
}
