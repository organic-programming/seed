//go:build windows

package observability

import (
	"os"
	"testing"
)

func TestDefaultSlugWindowsExecutablePath(t *testing.T) {
	previous := os.Args
	t.Cleanup(func() { os.Args = previous })
	for _, executable := range []string{
		`C:\Applications\projet français 日本語\mody-media.exe`,
		`C:/Applications/project with spaces/mody-media.exe`,
		`\\server\share\mody-media.EXE`,
		`mody-media.exe`,
		`mody-media`,
	} {
		os.Args = []string{executable}
		if got := defaultSlug(); got != "mody-media" {
			t.Errorf("defaultSlug(%q) = %q", executable, got)
		}
	}
}
