//go:build !windows

package holons

import "os"

// Keep the established Unix generation and restoration semantics unchanged.
func writeDescribeOutput(path string, data []byte, mode os.FileMode) error {
	return os.WriteFile(path, data, mode)
}
