//go:build windows

package discover

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWindowsPackageProbeIgnoresAdjacentDLLs(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin", runtime.GOOS+"_"+runtime.GOARCH)
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a-library.dll", "z-worker.EXE"} {
		if err := os.WriteFile(filepath.Join(bin, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := packageBinaryPath(root)
	if want := filepath.Join(bin, "z-worker.EXE"); err != nil || got != want {
		t.Fatalf("package binary = %q, %v; want %q", got, err, want)
	}
}

func TestWindowsBinaryRefPointsToImage(t *testing.T) {
	ref := holonRefFromEntry(HolonEntry{Dir: `C:\Tools`, SourceKind: "binary", Entrypoint: `C:\Tools\op.exe`})
	if ref.URL != "file:///C:/Tools/op.exe" {
		t.Fatalf("binary URL = %q", ref.URL)
	}
}
