//go:build darwin

package holons

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/organic-programming/grace-op/internal/identity"
	"github.com/organic-programming/grace-op/internal/testutil"
)

func TestActiveArtifactsBlockBeforeAnyCompositeMutation(t *testing.T) {
	for _, location := range []string{"parent", "child", "duplicate-uuid"} {
		t.Run(location, func(t *testing.T) {
			root := t.TempDir()
			parent := filepath.Join(root, "parent")
			child := writeProtoGoHolonFixture(t, root, "child")
			sibling := writeProtoGoHolonFixture(t, root, "sibling")
			if location == "duplicate-uuid" {
				path := filepath.Join(child, "api", "v1", "holon.proto")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(data), "child-uuid", "sibling-uuid")), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := testutil.WriteManifestFile(filepath.Join(parent, identity.ManifestFileName), `schema: holon/v0
kind: composite
build:
  runner: recipe
  members:
    - id: sibling
      path: ../sibling
      type: holon
    - id: child
      path: ../child
      type: holon
  targets:
    macos:
      steps:
        - build_member: sibling
        - build_member: child
artifacts:
  primary: .op/build/demo.holon
`); err != nil {
				t.Fatal(err)
			}
			var markers []string
			for _, dir := range []string{parent, sibling, child} {
				marker := filepath.Join(dir, ".op", "preserve.txt")
				if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(marker, []byte("original"), 0o644); err != nil {
					t.Fatal(err)
				}
				markers = append(markers, marker)
			}
			processDir := parent
			if location != "parent" {
				processDir = child
			}
			_, process := startDarwinArtifactTestProcess(t, filepath.Join(processDir, ".op", "build", "demo.holon", "App.app", "Contents", "Frameworks", "worker"), false)
			manifest, err := LoadManifest(parent)
			if err != nil {
				t.Fatal(err)
			}
			if err := writeBuildState(manifest, BuildContext{Target: "macos", Hardened: true}); err != nil {
				t.Fatal(err)
			}
			for _, op := range []Operation{OperationClean, OperationBuild} {
				report, err := ExecuteLifecycle(op, parent, BuildOptions{Target: "macos", Mode: "debug"})
				var conflict *ActiveArtifactConflict
				if !errors.As(err, &conflict) || len(conflict.Processes) != 1 || conflict.Processes[0].PID != process.PID {
					t.Fatalf("%s: expected worker conflict; got %v", op, err)
				}
				if len(report.Commands) != 0 || len(report.Children) != 0 {
					t.Fatalf("mutation started: %+v", report)
				}
				for _, marker := range markers {
					data, err := os.ReadFile(marker)
					if err != nil || string(data) != "original" {
						t.Fatalf("modified %s: %q, %v", marker, data, err)
					}
				}
				state, err := readBuildState(manifest)
				if err != nil || state == nil || !state.Hardened {
					t.Fatalf("build state changed: %+v, %v", state, err)
				}
			}
			for _, op := range []Operation{OperationBuild, OperationCheck, OperationTest} {
				if err := CheckActiveArtifacts(manifest, BuildContext{DryRun: true}, op); err != nil {
					t.Fatalf("read-only %s refused: %v", op, err)
				}
			}
		})
	}
}

func TestActiveArtifactCanonicalPathUsesFilesystemCase(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "CaseSensitiveSpelling")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(filepath.Dir(dir), "casesensitivespelling")
	if _, err := os.Stat(alias); os.IsNotExist(err) {
		t.Skip("test requires a case-insensitive filesystem")
	}
	want, err := canonicalArtifactPath(filepath.Join(dir, "future", "app"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := canonicalArtifactPath(filepath.Join(alias, "future", "app"))
	if err != nil || want != got {
		t.Fatalf("filesystem alias = %q, %v; want %q", got, err, want)
	}
}
