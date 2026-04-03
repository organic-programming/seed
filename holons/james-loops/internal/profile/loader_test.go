package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_BundledProfile(t *testing.T) {
	item, err := Load("codex-default")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if item.Name != "codex-default" {
		t.Fatalf("profile name = %q, want codex-default", item.Name)
	}
	if item.Driver != DriverCodex {
		t.Fatalf("profile driver = %q, want %q", item.Driver, DriverCodex)
	}
}

func TestLoad_UnknownProfile(t *testing.T) {
	_, err := Load("missing-profile")
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "james-loops profile list") {
		t.Fatalf("error = %q, want profile list hint", err)
	}
}

func TestLoadAll_Dedup(t *testing.T) {
	repoRoot := t.TempDir()
	homeDir := t.TempDir()
	localDir := filepath.Join(repoRoot, "ader", "loops", "profiles")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("mkdir local profiles: %v", err)
	}
	localProfile := strings.Join([]string{
		"name: codex-default",
		"driver: codex",
		"model: local-model",
		"extra_args: []",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(localDir, "codex-default.yaml"), []byte(localProfile), 0o644); err != nil {
		t.Fatalf("write local profile: %v", err)
	}

	oldRepoRootResolver := repoRootResolver
	oldUserHomeResolver := userHomeResolver
	repoRootResolver = func() (string, error) { return repoRoot, nil }
	userHomeResolver = func() (string, error) { return homeDir, nil }
	defer func() {
		repoRootResolver = oldRepoRootResolver
		userHomeResolver = oldUserHomeResolver
	}()

	items, err := LoadAll()
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}
	for _, item := range items {
		if item.Name == "codex-default" {
			if item.Model != "local-model" {
				t.Fatalf("dedup model = %q, want local-model", item.Model)
			}
			return
		}
	}
	t.Fatal("codex-default missing from LoadAll()")
}
