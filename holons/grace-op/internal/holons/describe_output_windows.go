package holons

import (
	"bytes"
	"os"
	"path/filepath"
)

// A reader may map generated sources (for example, while inspecting a diff).
// Windows refuses to truncate a mapped file. Leave identical content alone and
// publish changed content using a complete sibling file, so existing mappings
// keep reading their original bytes. A handle denying delete sharing still
// causes an error; do not truncate, remove the destination or bypass that lock.
// This is a source-generation replacement, not a power-loss durability promise.
func writeDescribeOutput(path string, data []byte, mode os.FileMode) error {
	previous, err := os.ReadFile(path)
	if err == nil && bytes.Equal(previous, data) {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	staged, err := os.CreateTemp(filepath.Dir(path), ".op-describe-*")
	if err != nil {
		return err
	}
	defer os.Remove(staged.Name())
	if _, err := staged.Write(data); err != nil {
		_ = staged.Close()
		return err
	}
	if err := staged.Chmod(mode); err != nil {
		_ = staged.Close()
		return err
	}
	if err := staged.Close(); err != nil {
		return err
	}
	return replaceDescribeOutput(staged.Name(), path)
}
