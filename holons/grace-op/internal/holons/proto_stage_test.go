package holons

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/organic-programming/grace-op/internal/identity"
	"github.com/organic-programming/grace-op/internal/progress"
	"github.com/organic-programming/grace-op/internal/testutil"
)

func TestProtoStageWritesDescriptor(t *testing.T) {
	if _, err := execLookPath("go"); err != nil {
		t.Skip("go command not available")
	}

	root := t.TempDir()
	chdirForHolonTest(t, root)
	dir := writeProtoGoHolonFixture(t, root, "proto-desc-test")

	_, err := ExecuteLifecycle(OperationBuild, dir)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	descriptorPath := filepath.Join(dir, ".op", "pb", "descriptors.binpb")
	data, readErr := os.ReadFile(descriptorPath)
	if readErr != nil {
		t.Fatalf("descriptor not written: %v", readErr)
	}
	if len(data) == 0 {
		t.Fatal("descriptor file is empty")
	}
}

func TestProtoStageWritesReferenceDoc(t *testing.T) {
	if _, err := execLookPath("go"); err != nil {
		t.Skip("go command not available")
	}

	root := t.TempDir()
	chdirForHolonTest(t, root)
	dir := writeProtoGoHolonFixture(t, root, "proto-doc-test")

	ExecuteLifecycle(OperationBuild, dir)

	refPath := filepath.Join(dir, ".op", "doc", "REFERENCE.md")
	data, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("REFERENCE.md not written: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Reference") {
		t.Fatal("REFERENCE.md missing title")
	}
	if !strings.Contains(content, "op build") {
		t.Fatal("REFERENCE.md missing commands section")
	}
}

func TestProtoStageFailsOnBrokenProto(t *testing.T) {
	root := t.TempDir()

	// Create a minimal LoadedManifest pointing at root.
	manifest := &LoadedManifest{
		Dir: root,
		Manifest: Manifest{
			Kind:  KindNative,
			Build: BuildConfig{Runner: RunnerGoModule},
		},
	}

	// Write a broken proto that the proto stage will discover and fail on.
	if err := os.MkdirAll(filepath.Join(root, "api", "v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	brokenProto := `syntax = "proto3";
package test.v1;
message Broken {
  string name = 1;
`
	if err := os.WriteFile(filepath.Join(root, "api", "v1", "broken.proto"), []byte(brokenProto), 0644); err != nil {
		t.Fatal(err)
	}

	err := protoStage(manifest, progress.Silence())
	if err == nil {
		t.Fatal("expected proto stage failure for broken proto")
	}
	if !strings.Contains(err.Error(), "proto stage") {
		t.Fatalf("error should mention proto stage, got: %v", err)
	}
}

func TestProtoStageSkipsWhenNoProtos(t *testing.T) {
	root := t.TempDir()
	chdirForHolonTest(t, root)

	dir := filepath.Join(root, "yaml-only")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := testutil.WriteManifestFile(filepath.Join(dir, identity.ManifestFileName), `schema: holon/v0
kind: native
build:
  runner: go-module
artifacts:
  binary: test-holon
`); err != nil {
		t.Fatal(err)
	}

	_, err := ExecuteLifecycle(OperationBuild, dir)
	// Build may fail (no Go code), but proto stage must NOT fail.
	if err != nil && strings.Contains(err.Error(), "proto stage") {
		t.Fatalf("proto stage should be a no-op for YAML holons: %v", err)
	}

	// Descriptor should NOT exist.
	descriptorPath := filepath.Join(dir, ".op", "pb", "descriptors.binpb")
	if _, statErr := os.Stat(descriptorPath); statErr == nil {
		t.Fatal("descriptor should not exist for YAML-only holon")
	}
}
