//go:build windows && !go1.25

package holons

import "os"

// Preserve compilation with the module's Go 1.24 minimum. Identical sources
// still need no write; a changed mapped source may require Go 1.25 or later.
// Do not fall back to truncation when replacement is unavailable.
func replaceDescribeOutput(staged, destination string) error {
	return os.Rename(staged, destination)
}
