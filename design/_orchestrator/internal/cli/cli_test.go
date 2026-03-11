package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseArgsValidFlags(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg, err := ParseArgs([]string{"--set", "v1.0", "--set", "v1.1", "--model", "gpt-5.4", "--root", root})
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}

	if got, want := len(cfg.Sets), 2; got != want {
		t.Fatalf("len(cfg.Sets) = %d, want %d", got, want)
	}
	if cfg.Sets[0] != "v1.0" || cfg.Sets[1] != "v1.1" {
		t.Fatalf("unexpected sets: %v", cfg.Sets)
	}
	if cfg.Model != "gpt-5.4" {
		t.Fatalf("cfg.Model = %q", cfg.Model)
	}
	if cfg.Root != root {
		t.Fatalf("cfg.Root = %q, want %q", cfg.Root, root)
	}
	if cfg.StateFile != filepath.Join(root, stateFileName) {
		t.Fatalf("cfg.StateFile = %q", cfg.StateFile)
	}
}

func TestParseArgsMissingSet(t *testing.T) {
	t.Parallel()

	_, err := ParseArgs([]string{"--model", "gpt-5.4"})
	if err == nil {
		t.Fatal("expected missing --set error, got nil")
	}
	if !strings.Contains(err.Error(), "at least one --set is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseArgsEmptyModel(t *testing.T) {
	t.Parallel()

	_, err := ParseArgs([]string{"--set", "v1.0", "--model", ""})
	if err == nil {
		t.Fatal("expected empty model error, got nil")
	}
	if !strings.Contains(err.Error(), "--model cannot be empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}
