package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/organic-programming/clem-ader/internal/testrepo"
)

func TestRunCommittedSnapshotUsesHEAD(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")
	if err := os.WriteFile(filepath.Join(root, "state.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("write dirty state: %v", err)
	}

	withWorkingDir(t, root, func() {
		result, err := Run(context.Background(), RunOptions{
			ConfigDir:     configDir,
			Suite:         "fixture",
			Profile:       "integration",
			Source:        "committed",
			ArchivePolicy: "never",
			KeepSnapshot:  true,
		})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if !result.Manifest.Dirty {
			t.Fatal("expected dirty workspace metadata")
		}
		data, err := os.ReadFile(filepath.Join(result.Manifest.SnapshotRoot, "state.txt"))
		if err != nil {
			t.Fatalf("read committed snapshot file: %v", err)
		}
		if got := string(data); got != "committed\n" {
			t.Fatalf("committed snapshot state = %q, want %q", got, "committed\n")
		}
	})
}

func TestRunWorkspaceSnapshotUsesWorkingTree(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")
	if err := os.WriteFile(filepath.Join(root, "state.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("write dirty state: %v", err)
	}

	withWorkingDir(t, root, func() {
		result, err := Run(context.Background(), RunOptions{
			ConfigDir:     configDir,
			Suite:         "fixture",
			Profile:       "integration",
			Source:        "workspace",
			ArchivePolicy: "never",
			KeepSnapshot:  true,
		})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		data, err := os.ReadFile(filepath.Join(result.Manifest.SnapshotRoot, "state.txt"))
		if err != nil {
			t.Fatalf("read workspace snapshot file: %v", err)
		}
		if got := string(data); got != "dirty\n" {
			t.Fatalf("workspace snapshot state = %q, want %q", got, "dirty\n")
		}
	})
}

func TestRunFullArchivesAndHistoryShow(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")
	withWorkingDir(t, root, func() {
		result, err := Run(context.Background(), RunOptions{
			ConfigDir:     configDir,
			Suite:         "fixture",
			Profile:       "full",
			Lane:          "regression",
			Source:        "workspace",
			ArchivePolicy: "always",
		})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if result.Manifest.ArchivePath == "" {
			t.Fatal("expected archive path")
		}
		if _, err := os.Stat(result.Manifest.ArchivePath); err != nil {
			t.Fatalf("archive missing: %v", err)
		}
		if _, err := os.Stat(result.Manifest.ReportDir); !os.IsNotExist(err) {
			t.Fatalf("expected extracted report to be removed, stat err = %v", err)
		}

		history, err := History(context.Background(), configDir)
		if err != nil {
			t.Fatalf("History() error = %v", err)
		}
		if len(history) != 1 {
			t.Fatalf("History() count = %d, want 1", len(history))
		}
		if history[0].ArchivePath == "" {
			t.Fatal("expected archived history entry")
		}
		if history[0].Suite != "fixture" {
			t.Fatalf("history suite = %q, want fixture", history[0].Suite)
		}

		shown, err := ShowHistory(context.Background(), configDir, result.Manifest.HistoryID)
		if err != nil {
			t.Fatalf("ShowHistory() error = %v", err)
		}
		if shown.Manifest.HistoryID != result.Manifest.HistoryID {
			t.Fatalf("ShowHistory() history id = %q, want %q", shown.Manifest.HistoryID, result.Manifest.HistoryID)
		}
		if !strings.Contains(shown.SummaryMarkdown, "Verification Report") {
			t.Fatalf("summary markdown missing heading: %q", shown.SummaryMarkdown)
		}
	})
}

func TestArchiveLatestAndCleanup(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")
	withWorkingDir(t, root, func() {
		result, err := Run(context.Background(), RunOptions{
			ConfigDir:     configDir,
			Suite:         "fixture",
			Profile:       "integration",
			Source:        "workspace",
			ArchivePolicy: "never",
			KeepSnapshot:  true,
		})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		archived, err := Archive(context.Background(), ArchiveOptions{
			ConfigDir: configDir,
			Latest:    true,
		})
		if err != nil {
			t.Fatalf("Archive() error = %v", err)
		}
		if archived.Manifest.ArchivePath == "" {
			t.Fatal("expected archive path after manual archive")
		}
		if _, err := os.Stat(archived.Manifest.ArchivePath); err != nil {
			t.Fatalf("manual archive missing: %v", err)
		}

		removedDir := filepath.Join(root, "integration", ".artifacts", "local-suite", result.Manifest.HistoryID)
		if !dirExists(removedDir) {
			t.Fatalf("expected local-suite dir to exist before cleanup: %s", removedDir)
		}
		cleanup, err := Cleanup(context.Background(), configDir)
		if err != nil {
			t.Fatalf("Cleanup() error = %v", err)
		}
		if cleanup.RemovedLocalSuiteDirs == 0 {
			t.Fatal("expected cleanup to remove at least one local-suite dir")
		}
		if dirExists(removedDir) {
			t.Fatalf("cleanup did not remove %s", removedDir)
		}
	})
}

func TestProgressionProposalDoesNotMutateSuite(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")
	suitePath := filepath.Join(configDir, "suites", "fixture.yaml")
	before, err := os.ReadFile(suitePath)
	if err != nil {
		t.Fatalf("read suite before run: %v", err)
	}

	withWorkingDir(t, root, func() {
		result, err := Run(context.Background(), RunOptions{
			ConfigDir:     configDir,
			Suite:         "fixture",
			Profile:       "quick",
			Lane:          "progression",
			Source:        "workspace",
			ArchivePolicy: "never",
			KeepSnapshot:  true,
			KeepReport:    true,
		})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if result.Promotion == nil {
			t.Fatal("expected promotion proposal for clean progression run")
		}
		if _, err := os.Stat(filepath.Join(result.Manifest.ReportDir, reportPromotionJSON)); err != nil {
			t.Fatalf("promotion.json missing: %v", err)
		}
		if _, err := os.Stat(filepath.Join(result.Manifest.ReportDir, reportPromotionMD)); err != nil {
			t.Fatalf("promotion.md missing: %v", err)
		}
	})

	after, err := os.ReadFile(suitePath)
	if err != nil {
		t.Fatalf("read suite after run: %v", err)
	}
	if string(after) != string(before) {
		t.Fatal("progression proposal mutated the suite file")
	}
}

func TestLoadRunConfigDefaults(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")

	cfg, err := loadRunConfig(configDir, "")
	if err != nil {
		t.Fatalf("loadRunConfig() error = %v", err)
	}
	if cfg.SuiteName != "fixture" {
		t.Fatalf("default suite = %q, want fixture", cfg.SuiteName)
	}
	if cfg.Root.Defaults.Lane != "regression" {
		t.Fatalf("default lane = %q, want regression", cfg.Root.Defaults.Lane)
	}
	if cfg.Root.Defaults.Profile != "quick" {
		t.Fatalf("default profile = %q, want quick", cfg.Root.Defaults.Profile)
	}
	if cfg.Root.Defaults.Source != "committed" {
		t.Fatalf("default source = %q, want committed", cfg.Root.Defaults.Source)
	}
	if strings.Join(cfg.Root.Defaults.Ladder, ",") != "quick,unit,integration,full" {
		t.Fatalf("default ladder = %v, want [quick unit integration full]", cfg.Root.Defaults.Ladder)
	}
	if cfg.Root.Defaults.ArchivePolicy["full"] != "auto" {
		t.Fatalf("full archive policy = %q, want auto", cfg.Root.Defaults.ArchivePolicy["full"])
	}
}

func TestResolveProfileLaneSteps(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")
	cfg, err := loadRunConfig(configDir, "fixture")
	if err != nil {
		t.Fatalf("loadRunConfig() error = %v", err)
	}

	got, err := resolveProfileLaneSteps(cfg, "quick", "both", filepath.Join(root, "snapshot"))
	if err != nil {
		t.Fatalf("resolveProfileLaneSteps() error = %v", err)
	}
	var ids []string
	for _, step := range got {
		ids = append(ids, step.ID)
	}
	want := []string{"ader-unit", "sdk-go-unit", "example-go-unit", "integration-short"}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("selected ids = %v, want %v", ids, want)
	}
}

func TestReadSuiteConfigRejectsMalformedScriptSteps(t *testing.T) {
	root := t.TempDir()
	suitePath := filepath.Join(root, "fixture.yaml")
	if err := os.WriteFile(suitePath, []byte(`steps:
  broken:
    workdir: .
profiles:
  quick:
    steps: [broken]
`), 0o644); err != nil {
		t.Fatalf("write suite: %v", err)
	}
	if _, err := readSuiteConfig(suitePath); err == nil || !strings.Contains(err.Error(), "exactly one of command or script") {
		t.Fatalf("readSuiteConfig() error = %v, want exact command/script validation", err)
	}

	if err := os.WriteFile(suitePath, []byte(`steps:
  broken:
    workdir: .
    command: echo hi
    script: scripts/test.sh
profiles:
  quick:
    steps: [broken]
`), 0o644); err != nil {
		t.Fatalf("rewrite suite: %v", err)
	}
	if _, err := readSuiteConfig(suitePath); err == nil || !strings.Contains(err.Error(), "cannot define both command and script") {
		t.Fatalf("readSuiteConfig() error = %v, want xor validation", err)
	}

	if err := os.WriteFile(suitePath, []byte(`steps:
  broken:
    workdir: .
    args: [alpha]
profiles:
  quick:
    steps: [broken]
`), 0o644); err != nil {
		t.Fatalf("rewrite suite with args only: %v", err)
	}
	if _, err := readSuiteConfig(suitePath); err == nil || !strings.Contains(err.Error(), "cannot define args without script") {
		t.Fatalf("readSuiteConfig() error = %v, want args-without-script validation", err)
	}
}

func TestReadSuiteConfigRejectsOldProfileLaneFormat(t *testing.T) {
	root := t.TempDir()
	suitePath := filepath.Join(root, "fixture.yaml")
	if err := os.WriteFile(suitePath, []byte(`steps:
  ok:
    workdir: .
    command: echo ok
profiles:
  quick:
    regression: [ok]
    progression: []
`), 0o644); err != nil {
		t.Fatalf("write suite: %v", err)
	}
	if _, err := readSuiteConfig(suitePath); err == nil || !strings.Contains(err.Error(), "old regression/progression format") {
		t.Fatalf("readSuiteConfig() error = %v, want migration failure", err)
	}
}

func TestReadSuiteConfigDefaultsMissingLaneToProgression(t *testing.T) {
	root := t.TempDir()
	suitePath := filepath.Join(root, "fixture.yaml")
	if err := os.WriteFile(suitePath, []byte(`steps:
  ok:
    workdir: .
    command: echo ok
profiles:
  quick:
    steps: [ok]
`), 0o644); err != nil {
		t.Fatalf("write suite: %v", err)
	}
	cfg, err := readSuiteConfig(suitePath)
	if err != nil {
		t.Fatalf("readSuiteConfig() error = %v", err)
	}
	if got := normalizeStepLane(cfg.Steps["ok"].Lane); got != "progression" {
		t.Fatalf("lane = %q, want progression", got)
	}
}

func TestReadSuiteConfigRejectsInvalidLane(t *testing.T) {
	root := t.TempDir()
	suitePath := filepath.Join(root, "fixture.yaml")
	if err := os.WriteFile(suitePath, []byte(`steps:
  broken:
    workdir: .
    command: echo ok
    lane: sideways
profiles:
  quick:
    steps: [broken]
`), 0o644); err != nil {
		t.Fatalf("write suite: %v", err)
	}
	if _, err := readSuiteConfig(suitePath); err == nil || !strings.Contains(err.Error(), "invalid lane") {
		t.Fatalf("readSuiteConfig() error = %v, want invalid-lane failure", err)
	}
}

func TestRunScriptStepWithArgs(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")
	withWorkingDir(t, root, func() {
		result, err := Run(context.Background(), RunOptions{
			ConfigDir:     configDir,
			Suite:         "fixture",
			Profile:       "unit",
			Lane:          "progression",
			StepFilter:    "^fixture-script$",
			Source:        "workspace",
			ArchivePolicy: "never",
			KeepReport:    true,
		})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if len(result.Steps) != 1 {
			t.Fatalf("steps count = %d, want 1", len(result.Steps))
		}
		step := result.Steps[0]
		if step.Status != "PASS" {
			t.Fatalf("script step status = %q, want PASS", step.Status)
		}
		if !strings.Contains(step.Command, "fixture-step.sh alpha beta") {
			t.Fatalf("script step command = %q, want script path with args", step.Command)
		}
		logData, err := os.ReadFile(step.LogPath)
		if err != nil {
			t.Fatalf("read script log: %v", err)
		}
		if !strings.Contains(string(logData), "fixture-script:alpha:beta") {
			t.Fatalf("script log = %q, want fixture output", string(logData))
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
