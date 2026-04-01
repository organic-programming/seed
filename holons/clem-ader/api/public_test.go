// These tests verify the public Code API operations of clem-ader against a tiny synthetic Git repository and config-dir suite.
package api

import (
	"os"
	"path/filepath"
	"testing"

	aderv1 "github.com/organic-programming/clem-ader/gen/go/v1"
	"github.com/organic-programming/clem-ader/internal/testrepo"
)

func TestPublicTestOperation(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")
	withWorkingDir(t, root, func() {
		response, err := Test(&aderv1.TestRequest{
			ConfigDir:     configDir,
			Suite:         "fixture",
			Profile:       "integration",
			Source:        "workspace",
			ArchivePolicy: "never",
		})
		if err != nil {
			t.Fatalf("Test() error = %v", err)
		}
		if response.GetManifest().GetFinalStatus() != "PASS" {
			t.Fatalf("final status = %q, want PASS", response.GetManifest().GetFinalStatus())
		}
		if response.GetManifest().GetHistoryId() == "" {
			t.Fatal("expected history id")
		}
		if response.GetManifest().GetSuite() != "fixture" {
			t.Fatalf("suite = %q, want fixture", response.GetManifest().GetSuite())
		}
	})
}

func TestPublicArchiveOperation(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")
	withWorkingDir(t, root, func() {
		if _, err := Test(&aderv1.TestRequest{
			ConfigDir:     configDir,
			Suite:         "fixture",
			Profile:       "integration",
			Source:        "workspace",
			ArchivePolicy: "never",
			KeepSnapshot:  true,
		}); err != nil {
			t.Fatalf("Test() setup error = %v", err)
		}
		response, err := Archive(&aderv1.ArchiveRequest{
			ConfigDir: configDir,
			Latest:    true,
		})
		if err != nil {
			t.Fatalf("Archive() error = %v", err)
		}
		if response.GetArchivePath() == "" {
			t.Fatal("expected archive path")
		}
		if _, err := os.Stat(response.GetArchivePath()); err != nil {
			t.Fatalf("archive stat: %v", err)
		}
	})
}

func TestPublicCleanupOperation(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")
	withWorkingDir(t, root, func() {
		stale := filepath.Join(root, "integration", ".artifacts", "local-suite", "stale")
		if err := os.MkdirAll(stale, 0o755); err != nil {
			t.Fatalf("mkdir stale: %v", err)
		}
		response, err := Cleanup(&aderv1.CleanupRequest{ConfigDir: configDir})
		if err != nil {
			t.Fatalf("Cleanup() error = %v", err)
		}
		if response.GetRemovedLocalSuiteDirs() == 0 {
			t.Fatal("expected cleanup to remove at least one local-suite dir")
		}
	})
}

func TestPublicHistoryAndShowHistoryOperations(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")
	withWorkingDir(t, root, func() {
		testResponse, err := Test(&aderv1.TestRequest{
			ConfigDir:     configDir,
			Suite:         "fixture",
			Profile:       "integration",
			Source:        "workspace",
			ArchivePolicy: "always",
		})
		if err != nil {
			t.Fatalf("Test() error = %v", err)
		}
		listResponse, err := History(&aderv1.HistoryRequest{ConfigDir: configDir})
		if err != nil {
			t.Fatalf("History() error = %v", err)
		}
		if len(listResponse.GetEntries()) != 1 {
			t.Fatalf("History() count = %d, want 1", len(listResponse.GetEntries()))
		}
		showResponse, err := ShowHistory(&aderv1.ShowHistoryRequest{
			ConfigDir: configDir,
			HistoryId: testResponse.GetManifest().GetHistoryId(),
		})
		if err != nil {
			t.Fatalf("ShowHistory() error = %v", err)
		}
		if showResponse.GetManifest().GetHistoryId() != testResponse.GetManifest().GetHistoryId() {
			t.Fatalf("ShowHistory() history id = %q, want %q", showResponse.GetManifest().GetHistoryId(), testResponse.GetManifest().GetHistoryId())
		}
		if showResponse.GetSummaryMarkdown() == "" {
			t.Fatal("expected summary markdown")
		}
	})
}

func withWorkingDir(t *testing.T, dir string, fn func()) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	defer func() {
		_ = os.Chdir(cwd)
	}()
	fn()
}
