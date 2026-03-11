package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAcquirePreventsConcurrentRuns(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	lock, err := Acquire(root)
	if err != nil {
		t.Fatalf("Acquire returned error: %v", err)
	}
	defer lock.Release()

	_, err = Acquire(root)
	if err == nil {
		t.Fatal("expected concurrent acquire error, got nil")
	}
	if !strings.Contains(err.Error(), "another orchestrator is running") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAcquireReclaimsStaleLock(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	lockPath := filepath.Join(root, lockFileName)
	if err := os.WriteFile(lockPath, []byte("999999\n"), 0o644); err != nil {
		t.Fatalf("write stale lock: %v", err)
	}

	lock, err := Acquire(root)
	if err != nil {
		t.Fatalf("Acquire returned error: %v", err)
	}
	defer lock.Release()
}
