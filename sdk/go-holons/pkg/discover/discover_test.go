package discover

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/organic-programming/go-holons/pkg/identity"
)

func TestDiscoverRecursesSkipsAndDedups(t *testing.T) {
	root := t.TempDir()

	writeHolon(t, filepath.Join(root, "holons", "alpha"), holonSeed{
		uuid:       "uuid-alpha",
		givenName:  "Alpha",
		familyName: "Go",
		binary:     "alpha-go",
	})
	writeHolon(t, filepath.Join(root, "nested", "beta"), holonSeed{
		uuid:       "uuid-beta",
		givenName:  "Beta",
		familyName: "Rust",
		binary:     "beta-rust",
	})
	writeHolon(t, filepath.Join(root, "nested", "dup", "alpha"), holonSeed{
		uuid:       "uuid-alpha",
		givenName:  "Alpha",
		familyName: "Go",
		binary:     "alpha-go",
	})

	for _, skipped := range []string{
		filepath.Join(root, ".git", "hidden"),
		filepath.Join(root, ".op", "hidden"),
		filepath.Join(root, "node_modules", "hidden"),
		filepath.Join(root, "vendor", "hidden"),
		filepath.Join(root, "build", "hidden"),
		filepath.Join(root, "testdata", "hidden"),
		filepath.Join(root, ".cache", "hidden"),
	} {
		writeHolon(t, skipped, holonSeed{
			uuid:       filepath.Base(skipped) + "-uuid",
			givenName:  "Ignored",
			familyName: "Holon",
			binary:     "ignored-holon",
		})
	}

	entries, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("Discover returned %d entries, want 2", len(entries))
	}

	got := map[string]HolonEntry{}
	for _, entry := range entries {
		got[entry.UUID] = entry
	}

	alpha := got["uuid-alpha"]
	if alpha.Slug != "alpha-go" {
		t.Fatalf("alpha slug = %q, want %q", alpha.Slug, "alpha-go")
	}
	if alpha.RelativePath != "holons/alpha" {
		t.Fatalf("alpha relative path = %q, want %q", alpha.RelativePath, "holons/alpha")
	}
	if alpha.Manifest == nil || alpha.Manifest.Build.Runner != "go-module" {
		t.Fatalf("alpha manifest missing build runner: %#v", alpha.Manifest)
	}

	beta := got["uuid-beta"]
	if beta.RelativePath != "nested/beta" {
		t.Fatalf("beta relative path = %q, want %q", beta.RelativePath, "nested/beta")
	}
}

func TestDiscoverAllUsesStandardSearchOrder(t *testing.T) {
	root := t.TempDir()
	opHome := filepath.Join(root, "runtime")
	opBin := filepath.Join(opHome, "bin")
	cache := filepath.Join(opHome, "cache")
	buildRoot := filepath.Join(root, "local", ".op", "build")

	t.Setenv("OPPATH", opHome)
	t.Setenv("OPBIN", opBin)

	writeHolon(t, filepath.Join(root, "local", "rob-go"), holonSeed{
		uuid:       "same-uuid",
		givenName:  "Rob",
		familyName: "Go",
		binary:     "rob-go",
	})
	writePackageHolon(t, filepath.Join(buildRoot, "rob-go.holon"), packageSeed{
		uuid:          "same-uuid",
		givenName:     "Rob",
		familyName:    "Go",
		runner:        "go-module",
		entrypoint:    "rob-go",
		kind:          "native",
		architectures: []string{runtime.GOOS + "_" + runtime.GOARCH},
	})
	writePackageHolon(t, filepath.Join(opBin, "rob-go.holon"), packageSeed{
		uuid:       "same-uuid",
		givenName:  "Rob",
		familyName: "Go",
		runner:     "go-module",
		entrypoint: "rob-go",
		kind:       "native",
	})
	writePackageHolon(t, filepath.Join(cache, "deps", "rob-go.holon"), packageSeed{
		uuid:       "same-uuid",
		givenName:  "Rob",
		familyName: "Go",
		runner:     "go-module",
		entrypoint: "rob-go",
		kind:       "native",
	})

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(filepath.Join(root, "local")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	entries, err := DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll returned error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("DiscoverAll returned %d entries, want 1", len(entries))
	}
	if entries[0].Origin != "build" {
		t.Fatalf("entry origin = %q, want %q", entries[0].Origin, "build")
	}
	if entries[0].SourceKind != "package" {
		t.Fatalf("entry source kind = %q, want %q", entries[0].SourceKind, "package")
	}
}

func TestDiscoverAllPrefersBundlePackages(t *testing.T) {
	root := t.TempDir()
	appExecutable := filepath.Join(root, "MyApp.app", "Contents", "MacOS", "MyApp")
	bundleRoot := filepath.Join(root, "MyApp.app", "Contents", "Resources", "Holons")
	t.Setenv("OPPATH", filepath.Join(root, "runtime"))
	t.Setenv("OPBIN", filepath.Join(root, "runtime", "bin"))

	if err := os.MkdirAll(filepath.Dir(appExecutable), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(appExecutable, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	originalExecutablePath := executablePath
	executablePath = func() (string, error) { return appExecutable, nil }
	t.Cleanup(func() { executablePath = originalExecutablePath })

	writePackageHolon(t, filepath.Join(bundleRoot, "rob-go.holon"), packageSeed{
		uuid:          "bundle-uuid",
		givenName:     "Rob",
		familyName:    "Go",
		runner:        "go-module",
		entrypoint:    "rob-go",
		kind:          "native",
		architectures: []string{runtime.GOOS + "_" + runtime.GOARCH},
	})
	writeHolon(t, filepath.Join(root, "rob-go"), holonSeed{
		uuid:       "bundle-uuid",
		givenName:  "Rob",
		familyName: "Go",
		binary:     "rob-go",
	})

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalWD) })

	entries, err := DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll returned error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("DiscoverAll returned %d entries, want 1", len(entries))
	}
	if entries[0].Origin != "bundle" {
		t.Fatalf("entry origin = %q, want %q", entries[0].Origin, "bundle")
	}
}

func TestFindBySlugAndUUID(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OPPATH", filepath.Join(root, "runtime"))
	t.Setenv("OPBIN", filepath.Join(root, "runtime", "bin"))

	writeHolon(t, filepath.Join(root, "rob-go"), holonSeed{
		uuid:       "c7f3a1b2-1111-1111-1111-111111111111",
		givenName:  "Rob",
		familyName: "Go",
		binary:     "rob-go",
	})

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	entry, err := FindBySlug("rob-go")
	if err != nil {
		t.Fatalf("FindBySlug returned error: %v", err)
	}
	if entry == nil || entry.UUID != "c7f3a1b2-1111-1111-1111-111111111111" {
		t.Fatalf("FindBySlug returned %#v", entry)
	}

	entry, err = FindByUUID("c7f3a1b2")
	if err != nil {
		t.Fatalf("FindByUUID returned error: %v", err)
	}
	if entry == nil || entry.Slug != "rob-go" {
		t.Fatalf("FindByUUID returned %#v", entry)
	}

	missing, err := FindBySlug("missing")
	if err != nil {
		t.Fatalf("FindBySlug(missing) returned error: %v", err)
	}
	if missing != nil {
		t.Fatalf("FindBySlug(missing) = %#v, want nil", missing)
	}
}

func TestDiscoverProtoBackedHolonUsesManifestAndHolonRoot(t *testing.T) {
	root := t.TempDir()

	holonDir := filepath.Join(root, "proto-holon")
	writeSharedManifestProto(t, holonDir)
	if err := os.MkdirAll(filepath.Join(holonDir, "v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(holonDir, "cmd", "daemon"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(holonDir, "go.mod"), []byte("module example.com/protoholon\n\ngo 1.25.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(holonDir, "cmd", "daemon", "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	proto := `syntax = "proto3";

package proto.v1;

import "holons/v1/manifest.proto";

option (holons.v1.manifest) = {
  identity: {
    uuid: "proto-uuid"
    given_name: "Proto"
    family_name: "Holon"
    motto: "Proto-backed holon."
    composer: "test"
    clade: "deterministic/pure"
    status: "draft"
    born: "2026-03-15"
  }
  lineage: {
    reproduction: "manual"
    generated_by: "test"
  }
  kind: "native"
  lang: "go"
  build: {
    runner: "go-module"
    main: "./cmd/daemon"
  }
  requires: {
    files: ["go.mod"]
  }
  artifacts: {
    binary: "proto-holon"
  }
};
`
	if err := os.WriteFile(filepath.Join(holonDir, "v1", "holon.proto"), []byte(proto), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Discover returned %d entries, want 1", len(entries))
	}

	entry := entries[0]
	if entry.Slug != "proto-holon" {
		t.Fatalf("slug = %q, want %q", entry.Slug, "proto-holon")
	}
	if entry.RelativePath != "proto-holon" {
		t.Fatalf("relative path = %q, want %q", entry.RelativePath, "proto-holon")
	}
	if entry.Dir != holonDir {
		t.Fatalf("dir = %q, want %q", entry.Dir, holonDir)
	}
	if entry.Manifest == nil {
		t.Fatal("manifest should be resolved from holon.proto")
	}
	if entry.Manifest.Build.Runner != "go-module" {
		t.Fatalf("build runner = %q, want %q", entry.Manifest.Build.Runner, "go-module")
	}
	if entry.Manifest.Build.Main != "./cmd/daemon" {
		t.Fatalf("build main = %q, want %q", entry.Manifest.Build.Main, "./cmd/daemon")
	}
	if entry.Manifest.Artifacts.Binary != "proto-holon" {
		t.Fatalf("binary = %q, want %q", entry.Manifest.Artifacts.Binary, "proto-holon")
	}
}

type holonSeed struct {
	uuid       string
	givenName  string
	familyName string
	binary     string
}

type packageSeed struct {
	uuid          string
	givenName     string
	familyName    string
	runner        string
	entrypoint    string
	kind          string
	architectures []string
	hasDist       bool
	hasSource     bool
}

func writeHolon(t *testing.T, dir string, seed holonSeed) {
	t.Helper()

	writeSharedManifestProto(t, dir)

	if err := os.MkdirAll(filepath.Join(dir, "v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	data := fmt.Sprintf(`syntax = "proto3";

package discover.v1;

import "holons/v1/manifest.proto";

option (holons.v1.manifest) = {
  identity: {
    uuid: %q
    given_name: %q
    family_name: %q
    motto: "Test"
    composer: "test"
    clade: "deterministic/pure"
    status: "draft"
    born: "2026-03-07"
  }
  lineage: {
    generated_by: "test"
  }
  kind: "native"
  build: {
    runner: "go-module"
  }
  artifacts: {
    binary: %q
  }
};
`, seed.uuid, seed.givenName, seed.familyName, seed.binary)

	if err := os.WriteFile(filepath.Join(dir, "v1", "holon.proto"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writePackageHolon(t *testing.T, dir string, seed packageSeed) {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if seed.kind == "" {
		seed.kind = "native"
	}

	architectures := "[]"
	if len(seed.architectures) > 0 {
		quoted := make([]string, 0, len(seed.architectures))
		for _, arch := range seed.architectures {
			quoted = append(quoted, fmt.Sprintf("%q", arch))
		}
		architectures = "[" + strings.Join(quoted, ", ") + "]"
	}

	data := fmt.Sprintf(`{
  "schema": "holon-package/v1",
  "slug": %q,
  "uuid": %q,
  "identity": {
    "given_name": %q,
    "family_name": %q
  },
  "lang": "go",
  "runner": %q,
  "status": "draft",
  "kind": %q,
  "entrypoint": %q,
  "architectures": %s,
  "has_dist": %t,
  "has_source": %t
}
`, strings.ToLower(seed.givenName+"-"+seed.familyName), seed.uuid, seed.givenName, seed.familyName, seed.runner, seed.kind, seed.entrypoint, architectures, seed.hasDist, seed.hasSource)

	if err := os.WriteFile(filepath.Join(dir, ".holon.json"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeSharedManifestProto(t *testing.T, root string) {
	t.Helper()

	source := filepath.Join(identityTestdataRoot(t), "_protos", "holons", "v1", "manifest.proto")
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read %s: %v", source, err)
	}

	target := filepath.Join(root, "_protos", "holons", "v1")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", target, err)
	}
	if err := os.WriteFile(filepath.Join(target, "manifest.proto"), data, 0o644); err != nil {
		t.Fatalf("write manifest.proto: %v", err)
	}
}

func identityTestdataRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "identity", "testdata")
}

func TestDiscoverPackageFallbackProbe(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OPPATH", filepath.Join(root, "runtime"))
	t.Setenv("OPBIN", filepath.Join(root, "runtime", "bin"))

	// Create a .holon dir WITHOUT .holon.json — only the directory exists.
	pkgDir := filepath.Join(root, "alpha.holon")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Register a probe that returns a synthetic entry.
	SetProbe(func(packageDir string) (*HolonEntry, error) {
		if filepath.Base(packageDir) != "alpha.holon" {
			return nil, os.ErrNotExist
		}
		entry := &HolonEntry{
			Slug:       "alpha-go",
			UUID:       "probe-uuid-alpha",
			SourceKind: "package",
			Runner:     "go-module",
			Entrypoint: "alpha-go",
			Identity: identity.Identity{
				UUID:       "probe-uuid-alpha",
				GivenName:  "Alpha",
				FamilyName: "Go",
			},
			Manifest: &Manifest{
				Kind:  "native",
				Build: Build{Runner: "go-module"},
			},
		}
		// Write cache as side effect.
		if err := WritePackageJSON(packageDir, *entry); err != nil {
			return nil, err
		}
		return entry, nil
	})
	t.Cleanup(func() { SetProbe(nil) })

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalWD) })

	entries, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Discover returned %d entries, want 1", len(entries))
	}
	if entries[0].Slug != "alpha-go" {
		t.Fatalf("slug = %q, want %q", entries[0].Slug, "alpha-go")
	}
	if entries[0].UUID != "probe-uuid-alpha" {
		t.Fatalf("uuid = %q, want %q", entries[0].UUID, "probe-uuid-alpha")
	}

	// Verify .holon.json was written as a cache.
	jsonPath := filepath.Join(pkgDir, ".holon.json")
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatalf(".holon.json should have been written: %v", err)
	}
}

func TestWritePackageJSON(t *testing.T) {
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "test.holon")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}

	entry := HolonEntry{
		Slug:       "test-go",
		UUID:       "test-uuid",
		Entrypoint: "test-go",
		Identity: identity.Identity{
			UUID:       "test-uuid",
			GivenName:  "Test",
			FamilyName: "Go",
			Motto:      "A test holon.",
			Lang:       "go",
			Status:     "draft",
		},
		Manifest: &Manifest{
			Kind:  "native",
			Build: Build{Runner: "go-module"},
		},
	}

	if err := WritePackageJSON(pkgDir, entry); err != nil {
		t.Fatalf("WritePackageJSON: %v", err)
	}

	// Verify the file can be loaded back.
	loaded, err := loadPackageEntry(dir, pkgDir, "test")
	if err != nil {
		t.Fatalf("loadPackageEntry after write: %v", err)
	}
	if loaded.Slug != "test-go" {
		t.Fatalf("loaded slug = %q, want %q", loaded.Slug, "test-go")
	}
	if loaded.UUID != "test-uuid" {
		t.Fatalf("loaded uuid = %q, want %q", loaded.UUID, "test-uuid")
	}
	if loaded.Runner != "go-module" {
		t.Fatalf("loaded runner = %q, want %q", loaded.Runner, "go-module")
	}
}
