package integration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareWorkspaceMirrorRemovesStaleFiles(t *testing.T) {
	seedRoot := t.TempDir()
	artifactsRoot := t.TempDir()

	sourceFile := filepath.Join(seedRoot, "examples", "hello.txt")
	if err := os.MkdirAll(filepath.Dir(sourceFile), 0o755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.WriteFile(sourceFile, []byte("fresh"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	staleFile := filepath.Join(artifactsRoot, "workspace", "examples", "stale.txt")
	if err := os.MkdirAll(filepath.Dir(staleFile), 0o755); err != nil {
		t.Fatalf("mkdir stale: %v", err)
	}
	if err := os.WriteFile(staleFile, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale: %v", err)
	}

	rt := &runtimeState{
		seedRoot:      seedRoot,
		artifactsRoot: artifactsRoot,
	}
	mirrorRoot, err := prepareWorkspaceMirror(rt)
	if err != nil {
		t.Fatalf("prepareWorkspaceMirror: %v", err)
	}

	if _, err := os.Stat(staleFile); !os.IsNotExist(err) {
		t.Fatalf("stale file still exists after mirror preparation: %v", err)
	}
	if _, err := os.Stat(filepath.Join(mirrorRoot, "examples", "hello.txt")); err != nil {
		t.Fatalf("fresh source file was not mirrored: %v", err)
	}
}
