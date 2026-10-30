package holons

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestActiveArtifactConflictUsesCanonicalBoundaries(t *testing.T) {
	dir := t.TempDir()
	root, err := canonicalArtifactPath(filepath.Join(dir, ".op"))
	if err != nil {
		t.Fatal(err)
	}
	processes := []ActiveArtifactProcess{
		{PID: 8, Path: filepath.Join(dir, ".op", "demo.app", "Contents", "MacOS", "demo"), Started: "app"},
		{PID: 3, Path: filepath.Join(dir, ".op", "demo.app", "Contents", "Frameworks", "worker"), Started: "orphan"},
		{PID: 9, Path: filepath.Join(dir, ".op-other", "demo"), Started: "other"},
		{PID: 10, Path: filepath.Join(dir, "other-checkout", ".op", "demo"), Started: "other"},
	}
	err = activeArtifactConflict("demo", OperationClean, []activeArtifactRoot{{path: root, directory: true}}, processes)
	var conflict *ActiveArtifactConflict
	if !errors.As(err, &conflict) || len(conflict.Processes) != 2 {
		t.Fatalf("conflict = %#v, error = %v", conflict, err)
	}
	if conflict.Processes[0].PID != 3 || conflict.Processes[1].PID != 8 {
		t.Fatalf("unordered or unrelated processes: %#v", conflict.Processes)
	}
	for _, text := range []string{"cannot clean", "PID 3", "PID 8", "worker", "Close", "retry"} {
		if !strings.Contains(err.Error(), text) {
			t.Fatalf("diagnostic lacks %q: %v", text, err)
		}
	}
	file, err := canonicalArtifactPath(processes[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	err = activeArtifactConflict("demo", OperationBuild, []activeArtifactRoot{{path: file}}, processes)
	if !errors.As(err, &conflict) || len(conflict.Processes) != 1 || conflict.Processes[0].PID != 8 {
		t.Fatalf("exact artifact file matched siblings: %v", err)
	}
}

func TestActiveArtifactCanonicalSymlinkAncestor(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(dir, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	want, err := canonicalArtifactPath(filepath.Join(dir, "not-built", "demo"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := canonicalArtifactPath(filepath.Join(link, "not-built", "demo"))
	if err != nil || got != want {
		t.Fatalf("alias = %q, %v; want %q", got, err, want)
	}
}

func TestActiveArtifactStopRequiresCompleteIdentity(t *testing.T) {
	for _, process := range []ActiveArtifactProcess{{}, {PID: -1, Path: "/fake", Started: "fake"}, {PID: 1, Path: "/fake"}, {PID: 1, Started: "fake"}} {
		for _, force := range []bool{false, true} {
			if err := RequestActiveArtifactStop(context.Background(), process, force); !errors.Is(err, ErrActiveArtifactProcessChanged) {
				t.Fatalf("incomplete identity accepted: %#v: %v", process, err)
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := RequestActiveArtifactStop(ctx, ActiveArtifactProcess{}, false); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled request = %v", err)
	}
}
