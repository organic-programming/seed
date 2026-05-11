package seedtoolchain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSeedReleaseReadsCanonicalToolchainPin(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, FileName), []byte(`seed_release: "2.3.4"`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	seed, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := SeedRelease(seed); got != "2.3.4" {
		t.Fatalf("SeedRelease = %q", got)
	}
}

func TestManifestJSONUsesYamlV3ForRealSeedToolchain(t *testing.T) {
	root := repoRootFromPackage(t)
	seed, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	data, err := ManifestJSON(seed, "java", "aarch64-apple-darwin")
	if err != nil {
		t.Fatal(err)
	}
	var entries []ToolchainEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("manifest JSON did not parse: %v\n%s", err, string(data))
	}
	if len(entries) == 0 || entries[0].Name != "protoc" || entries[0].Version != "32.0" {
		t.Fatalf("entries = %#v, want protoc 32.0 first", entries)
	}
	if tag := CPPProtobufTag(seed); strings.TrimSpace(tag) == "" {
		t.Fatal("CPP protobuf tag is empty")
	}
}

func repoRootFromPackage(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(wd, "..", "..", ".."))
}
