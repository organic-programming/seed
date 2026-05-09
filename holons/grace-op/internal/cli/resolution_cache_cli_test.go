package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootHelpIncludesResolutionCacheFlags(t *testing.T) {
	output := captureStdout(t, func() {
		code := Run([]string{"help"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("help returned %d, want 0", code)
		}
	})

	for _, want := range []string{"--no-cache", "--purge-cache"} {
		if !strings.Contains(output, want) {
			t.Fatalf("help output missing %q:\n%s", want, output)
		}
	}
}

func TestBarePurgeCachePurgesAndExitsZero(t *testing.T) {
	root := t.TempDir()
	runtimeHome := filepath.Join(root, ".runtime")
	t.Setenv("OPPATH", runtimeHome)
	t.Setenv("OPBIN", filepath.Join(runtimeHome, "bin"))

	resolutionDir := filepath.Join(runtimeHome, "resolutions")
	if err := os.MkdirAll(resolutionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(resolutionDir, ".dirty"),
		filepath.Join(resolutionDir, "abcdef0123456789.json"),
	} {
		if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	output := captureStdout(t, func() {
		code := Run([]string{"--purge-cache"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("op --purge-cache returned %d, want 0", code)
		}
	})
	if strings.TrimSpace(output) != "" {
		t.Fatalf("op --purge-cache output = %q, want empty", output)
	}
	if _, err := os.Stat(resolutionDir); !os.IsNotExist(err) {
		t.Fatalf("resolution cache dir still exists after purge: %v", err)
	}
}
