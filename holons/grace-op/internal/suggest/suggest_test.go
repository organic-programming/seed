package suggest

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/organic-programming/grace-op/internal/holons"
	"github.com/organic-programming/grace-op/internal/identity"
	"github.com/organic-programming/grace-op/internal/testutil"
)

func TestBuildSuggestionsIncludeTestInstallRunAndDirectLaunch(t *testing.T) {
	root := t.TempDir()
	manifest := writeSuggestManifest(t, root, holons.KindNative, "dummy-holon", map[string]any{"grpc": true})
	restore := currentGOOS
	currentGOOS = func() string { return "linux" }
	t.Cleanup(func() { currentGOOS = restore })
	expectedBinary := ".op/build/dummy-holon.holon"

	var buf bytes.Buffer
	Print(&buf, Context{
		Command:     "build",
		Holon:       "dummy-holon",
		Manifest:    manifest,
		BuildTarget: "linux",
		Artifact:    ".op/build/dummy-holon.holon",
	})

	out := buf.String()
	for _, expected := range []string{
		"Next steps:",
		"op test dummy-holon",
		"op install dummy-holon",
		"op run dummy-holon:9090",
		expectedBinary + " --help",
	} {
		if !strings.Contains(out, expected) {
			t.Fatalf("output missing %q: %q", expected, out)
		}
	}
	if strings.Contains(out, "op test dummy-holon  run tests") {
		t.Fatalf("output still renders command and description on one line: %q", out)
	}
	if !strings.Contains(out, "    - run tests\n      op test dummy-holon\n") {
		t.Fatalf("output missing separated command layout: %q", out)
	}
}

func TestBuildSuggestionsSkipInstallForCompositeAndNoEmptyBlock(t *testing.T) {
	root := t.TempDir()
	manifest := writeSuggestManifest(t, root, holons.KindComposite, "", nil)

	var buf bytes.Buffer
	Print(&buf, Context{
		Command:     "uninstall",
		Holon:       "gudule",
		Manifest:    manifest,
		BuildTarget: "linux",
	})
	if buf.Len() != 0 {
		t.Fatalf("unexpected suggestions for uninstall: %q", buf.String())
	}
}

func TestTestSuggestionsDependOnArtifactPresence(t *testing.T) {
	root := t.TempDir()
	manifest := writeSuggestManifest(t, root, holons.KindNative, "dummy-holon", nil)
	restore := currentGOOS
	currentGOOS = func() string { return "linux" }
	t.Cleanup(func() { currentGOOS = restore })

	var buf bytes.Buffer
	Print(&buf, Context{
		Command:     "test",
		Holon:       "dummy-holon",
		Manifest:    manifest,
		BuildTarget: "linux",
	})
	out := buf.String()
	if !strings.Contains(out, "op build dummy-holon") {
		t.Fatalf("test suggestions missing build step: %q", out)
	}
	if !strings.Contains(out, "op install dummy-holon") {
		t.Fatalf("test suggestions missing install step: %q", out)
	}

	if err := os.MkdirAll(filepath.Dir(manifest.BinaryPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest.BinaryPath(), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	buf.Reset()
	Print(&buf, Context{
		Command:     "test",
		Holon:       "dummy-holon",
		Manifest:    manifest,
		BuildTarget: "linux",
	})
	out = buf.String()
	if strings.Contains(out, "op build dummy-holon") {
		t.Fatalf("test suggestions still show build after artifact exists: %q", out)
	}
}

func TestInstallSuggestionsUseInstalledPath(t *testing.T) {
	root := t.TempDir()
	manifest := writeSuggestManifest(t, root, holons.KindWrapper, "dummy-holon", map[string]any{"grpc": true})
	restore := currentGOOS
	currentGOOS = func() string { return "linux" }
	t.Cleanup(func() { currentGOOS = restore })

	var buf bytes.Buffer
	Print(&buf, Context{
		Command:     "install",
		Holon:       "dummy-holon",
		Manifest:    manifest,
		BuildTarget: "linux",
		Installed:   filepath.Join(root, ".op", "bin", "dummy-holon.holon"),
	})
	out := buf.String()
	if !strings.Contains(out, "op run dummy-holon:9090") {
		t.Fatalf("install suggestions missing run command: %q", out)
	}
	expectedInstalledBinary := filepath.Join(root, ".op", "bin", "dummy-holon.holon") + " --help"
	if !strings.Contains(out, expectedInstalledBinary) {
		t.Fatalf("install suggestions missing direct run: %q", out)
	}
}

func TestModAndNewSuggestions(t *testing.T) {
	tests := []struct {
		command  string
		holon    string
		expected []string
	}{
		{"mod init", "", []string{"op mod add <module>"}},
		{"mod pull", "", []string{"op mod list", "op mod graph", "op build"}},
		{"mod add", "", []string{"op mod pull", "op build"}},
		{"mod tidy", "", []string{"op mod pull", "op build"}},
		{"new", "megg-prober", []string{"op check megg-prober", "op build megg-prober"}},
		{"clean", "dummy-holon", []string{"op build dummy-holon"}},
	}

	for _, tc := range tests {
		t.Run(tc.command, func(t *testing.T) {
			var buf bytes.Buffer
			Print(&buf, Context{Command: tc.command, Holon: tc.holon})
			out := buf.String()
			for _, expected := range tc.expected {
				if !strings.Contains(out, expected) {
					t.Fatalf("output missing %q: %q", expected, out)
				}
			}
		})
	}
}

func TestPlatformAwareNoteForMismatchedBuildTarget(t *testing.T) {
	root := t.TempDir()
	manifest := writeSuggestManifest(t, root, holons.KindNative, "dummy-holon", nil)

	restore := currentGOOS
	currentGOOS = func() string { return "linux" }
	t.Cleanup(func() { currentGOOS = restore })

	var buf bytes.Buffer
	Print(&buf, Context{
		Command:     "build",
		Holon:       "dummy-holon",
		Manifest:    manifest,
		BuildTarget: "macos",
		Artifact:    ".op/build/dummy-holon.holon",
	})
	out := buf.String()
	if !strings.Contains(out, "built for macos, current platform is linux") {
		t.Fatalf("output missing platform mismatch note: %q", out)
	}
	if strings.Contains(out, ".op/build/dummy-holon.holon/bin/") {
		t.Fatalf("output unexpectedly contains direct launch on mismatched platform: %q", out)
	}
}

func writeSuggestManifest(t *testing.T, dir, kind, binary string, contract any) *holons.LoadedManifest {
	t.Helper()

	artifactBlock := ""
	buildBlock := "build:\n  runner: go-module\n"
	if binary != "" {
		artifactBlock = "  binary: " + binary + "\n"
	} else {
		buildBlock = "build:\n  runner: recipe\n  members:\n    - id: app\n      path: app\n      type: component\n  targets:\n    linux:\n      steps:\n        - assert_file:\n            path: build/app\n"
		artifactBlock = "  primary: build/app\n"
	}
	contractBlock := ""
	if contract != nil {
		contractBlock = "contract:\n  grpc: true\n"
	}
	content := "schema: holon/v0\nkind: " + kind + "\n" + buildBlock + "artifacts:\n" + artifactBlock + contractBlock
	if err := testutil.WriteManifestFile(filepath.Join(dir, identity.ManifestFileName), content); err != nil {
		t.Fatal(err)
	}
	manifest, err := holons.LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}
	return manifest
}
