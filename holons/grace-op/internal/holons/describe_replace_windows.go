//go:build windows && go1.25

package holons

import (
	"os"
	"path/filepath"
)

func replaceDescribeOutput(staged, destination string) error {
	root, err := os.OpenRoot(filepath.Dir(destination))
	if err != nil {
		return err
	}
	defer root.Close()
	// Root.Rename uses native FILE_RENAME_POSIX_SEMANTICS on Windows. Unlike
	// os.Rename/MoveFileEx, it keeps shared-delete mapped readers on their old
	// complete version while new opens see the new complete source.
	return root.Rename(filepath.Base(staged), filepath.Base(destination))
}
