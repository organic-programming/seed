package holons

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestHolonPackagePathsForNativeBuild(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "demo-holon")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeRunnerManifest(t, dir, `schema: holon/v0
uuid: 12345678-1234-1234-1234-1234567890ab
given_name: Demo
family_name: Holon
motto: A packaged test holon.
status: draft
lang: go
kind: native
build:
  runner: go-module
artifacts:
  binary: demo-holon
`)

	manifest, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	wantPkgDir := filepath.Join(dir, ".op", "build", "demo-holon.holon")
	if got := manifest.HolonPackageDir(); got != wantPkgDir {
		t.Fatalf("HolonPackageDir() = %q, want %q", got, wantPkgDir)
	}

	wantBinaryPath := filepath.Join(wantPkgDir, "bin", runtimeArchitecture(), "demo-holon")
	if got := manifest.BinaryPath(); got != wantBinaryPath {
		t.Fatalf("BinaryPath() = %q, want %q", got, wantBinaryPath)
	}

	if got := manifest.ArtifactPath(BuildContext{Target: canonicalRuntimeTarget()}); got != wantPkgDir {
		t.Fatalf("ArtifactPath() = %q, want %q", got, wantPkgDir)
	}
}

func TestWriteHolonJSONWritesPackageMetadata(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "demo-holon")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeRunnerManifest(t, dir, `schema: holon/v0
uuid: 12345678-1234-1234-1234-1234567890ab
given_name: Demo
family_name: Holon
motto: A packaged test holon.
status: stable
lang: go
transport: stdio
kind: native
build:
  runner: go-module
artifacts:
  binary: demo-holon
`)

	manifest, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(manifest.BinaryPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest.BinaryPath(), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(manifest.HolonPackageDir(), "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(manifest.HolonPackageDir(), "git"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := writeHolonJSON(manifest); err != nil {
		t.Fatalf("writeHolonJSON failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(manifest.HolonPackageDir(), ".holon.json"))
	if err != nil {
		t.Fatal(err)
	}

	var payload HolonPackageJSON
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if payload.Schema != "holon-package/v1" {
		t.Fatalf("Schema = %q, want holon-package/v1", payload.Schema)
	}
	if payload.Slug != "demo-holon" {
		t.Fatalf("Slug = %q, want demo-holon", payload.Slug)
	}
	if payload.UUID != "12345678-1234-1234-1234-1234567890ab" {
		t.Fatalf("UUID = %q", payload.UUID)
	}
	if payload.Identity.GivenName != "Demo" || payload.Identity.FamilyName != "Holon" || payload.Identity.Motto != "A packaged test holon." {
		t.Fatalf("Identity = %#v", payload.Identity)
	}
	if payload.Entrypoint != "demo-holon" {
		t.Fatalf("Entrypoint = %q, want demo-holon", payload.Entrypoint)
	}
	if !payload.HasDist || !payload.HasSource {
		t.Fatalf("HasDist/HasSource = %v/%v, want true/true", payload.HasDist, payload.HasSource)
	}
	if got, want := payload.Architectures, []string{runtimeArchitecture()}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Architectures = %v, want %v", got, want)
	}
}
