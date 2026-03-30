package mod

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	openv "github.com/organic-programming/grace-op/internal/env"
	"github.com/organic-programming/grace-op/internal/identity"
	"github.com/organic-programming/grace-op/internal/testutil"
)

func TestInitInfersHolonPathFromIdentity(t *testing.T) {
	dir := t.TempDir()

	id := identity.New()
	id.GivenName = "Alpha"
	id.FamilyName = "Builder"
	id.Motto = "Builds holons."
	id.Composer = "test"
	id.Clade = "deterministic/pure"
	id.Status = "draft"
	id.Lang = "go"
	if err := testutil.WriteIdentityFile(id, filepath.Join(dir, identity.ManifestFileName)); err != nil {
		t.Fatal(err)
	}

	result, err := Init(dir, "")
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	if result.HolonPath != "alpha-builder" {
		t.Fatalf("HolonPath = %q, want %q", result.HolonPath, "alpha-builder")
	}
}

func TestInitUsesExplicitHolonPath(t *testing.T) {
	dir := t.TempDir()

	result, err := Init(dir, "github.com/example/custom")
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	if result.HolonPath != "github.com/example/custom" {
		t.Fatalf("HolonPath = %q, want %q", result.HolonPath, "github.com/example/custom")
	}
}

func TestAddResolvesLatestTagWhenVersionMissing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "holon.mod"), []byte("holon alpha-builder\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	restore := SetRemoteTagsForTesting(func(depPath string) ([]string, error) {
		if depPath != "github.com/example/dep" {
			t.Fatalf("unexpected depPath %q", depPath)
		}
		return []string{"v1.0.0", "v1.4.0", "v1.2.0"}, nil
	})
	t.Cleanup(restore)

	result, err := Add(dir, "github.com/example/dep", "")
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if result.Dependency.Version != "v1.4.0" {
		t.Fatalf("version = %q, want %q", result.Dependency.Version, "v1.4.0")
	}
}

func TestUpdateSpecificModule(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "holon.mod"), []byte("holon alpha-builder\n\nrequire (\n    github.com/example/dep v1.0.0\n    github.com/example/keep v2.0.0\n)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	orig := listRemoteTags
	listRemoteTags = func(depPath string) ([]string, error) {
		switch depPath {
		case "github.com/example/dep":
			return []string{"v1.0.0", "v1.3.0", "v2.0.0"}, nil
		case "github.com/example/keep":
			return []string{"v2.0.0", "v2.1.0"}, nil
		default:
			return nil, nil
		}
	}
	t.Cleanup(func() { listRemoteTags = orig })

	result, err := Update(dir, "github.com/example/dep")
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if len(result.Updated) != 1 {
		t.Fatalf("updated count = %d, want 1", len(result.Updated))
	}
	if result.Updated[0].NewVersion != "v1.3.0" {
		t.Fatalf("updated version = %q, want %q", result.Updated[0].NewVersion, "v1.3.0")
	}

	data, err := os.ReadFile(filepath.Join(dir, "holon.mod"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "github.com/example/dep v1.3.0") {
		t.Fatalf("holon.mod missing updated dependency: %s", content)
	}
	if !strings.Contains(content, "github.com/example/keep v2.0.0") {
		t.Fatalf("holon.mod unexpectedly changed untouched dependency: %s", content)
	}
}

func TestPullUsesOPPATHCache(t *testing.T) {
	dir := t.TempDir()
	runtimeHome := filepath.Join(dir, ".runtime")
	t.Setenv("OPPATH", runtimeHome)
	if err := os.WriteFile(filepath.Join(dir, "holon.mod"), []byte("holon alpha-builder\n\nrequire (\n    github.com/example/dep v1.0.0\n)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cacheDir := filepath.Join(openv.CacheDir(), "github.com/example/dep@v1.0.0")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	id := identity.New()
	id.GivenName = "Cached"
	id.FamilyName = "Dep"
	id.Motto = "Cached dependency."
	id.Composer = "test"
	id.Clade = "deterministic/pure"
	id.Status = "draft"
	id.Lang = "go"
	if err := testutil.WriteIdentityFile(id, filepath.Join(cacheDir, identity.ManifestFileName)); err != nil {
		t.Fatal(err)
	}

	result, err := Pull(dir)
	if err != nil {
		t.Fatalf("Pull returned error: %v", err)
	}
	if len(result.Fetched) != 1 {
		t.Fatalf("fetched count = %d, want 1", len(result.Fetched))
	}
	if result.Fetched[0].CachePath != cacheDir {
		t.Fatalf("cache path = %q, want %q", result.Fetched[0].CachePath, cacheDir)
	}
}

func TestTidyCanonicalizesHolonModAndLeavesLanguageFilesUntouched(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "holon.mod"), []byte("holon alpha-builder\n\nrequire (\n    github.com/example/dep v1.0.0\n    github.com/example/dep v1.2.0\n    github.com/example/zed v0.1.0\n)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "holon.sum"), []byte("github.com/example/dep v1.2.0 h1:keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	goModPath := filepath.Join(dir, "go.mod")
	packageJSONPath := filepath.Join(dir, "package.json")
	cargoPath := filepath.Join(dir, "Cargo.toml")
	if err := os.WriteFile(goModPath, []byte("module example.com/demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packageJSONPath, []byte("{\"name\":\"demo\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cargoPath, []byte("[package]\nname = \"demo\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Tidy(dir)
	if err != nil {
		t.Fatalf("Tidy returned error: %v", err)
	}
	if len(result.Current) != 2 {
		t.Fatalf("current dependency count = %d, want 2", len(result.Current))
	}

	data, err := os.ReadFile(filepath.Join(dir, "holon.mod"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Count(content, "github.com/example/dep") != 1 {
		t.Fatalf("holon.mod still contains duplicate dep entries: %s", content)
	}
	if !strings.Contains(content, "github.com/example/dep v1.2.0") {
		t.Fatalf("holon.mod missing highest dep version: %s", content)
	}

	for path, want := range map[string]string{
		goModPath:       "module example.com/demo\n",
		packageJSONPath: "{\"name\":\"demo\"}\n",
		cargoPath:       "[package]\nname = \"demo\"\n",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Fatalf("%s changed: got %q want %q", path, string(data), want)
		}
	}
}
