package engine

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/organic-programming/clem-ader/internal/testrepo"
)

func TestRunCommittedSnapshotUsesHEAD(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "verification", "catalogues", "fixture")
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

func TestNewHistoryIDReservesDirectory(t *testing.T) {
	reportsDir := t.TempDir()
	now := time.Date(2026, time.April, 1, 23, 30, 45, 0, time.Local)

	first, err := newHistoryID(reportsDir, "seed", "committed", "quick", now)
	if err != nil {
		t.Fatalf("newHistoryID() first error = %v", err)
	}
	second, err := newHistoryID(reportsDir, "seed", "committed", "quick", now)
	if err != nil {
		t.Fatalf("newHistoryID() second error = %v", err)
	}

	if first != "seed_committed_quick-20260401_23_30_45_0001" {
		t.Fatalf("first history id = %q", first)
	}
	if second != "seed_committed_quick-20260401_23_30_45_0002" {
		t.Fatalf("second history id = %q", second)
	}
	if !dirExists(filepath.Join(reportsDir, first)) {
		t.Fatalf("missing reserved report dir for %s", first)
	}
	if !dirExists(filepath.Join(reportsDir, second)) {
		t.Fatalf("missing reserved report dir for %s", second)
	}
}

func TestRunWorkspaceSnapshotUsesWorkingTree(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "verification", "catalogues", "fixture")
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

func TestCopyWorkspaceTreeSkipsSiblingCatalogueRuntimeState(t *testing.T) {
	srcRoot := t.TempDir()
	dstRoot := t.TempDir()

	keepPath := filepath.Join(srcRoot, "verification", "catalogues", "op", "integration", "discover_test.go")
	skipPath := filepath.Join(srcRoot, "verification", "catalogues", "ader", ".artifacts", "tool-cache", "go-mod", "cache.txt")

	if err := os.MkdirAll(filepath.Dir(keepPath), 0o755); err != nil {
		t.Fatalf("mkdir keep path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(skipPath), 0o755); err != nil {
		t.Fatalf("mkdir skip path: %v", err)
	}
	if err := os.WriteFile(keepPath, []byte("package integration\n"), 0o644); err != nil {
		t.Fatalf("write keep path: %v", err)
	}
	if err := os.WriteFile(skipPath, []byte("cache\n"), 0o644); err != nil {
		t.Fatalf("write skip path: %v", err)
	}

	if err := copyWorkspaceTree(srcRoot, dstRoot, filepath.Join("verification", "catalogues", "op")); err != nil {
		t.Fatalf("copyWorkspaceTree() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(dstRoot, "verification", "catalogues", "op", "integration", "discover_test.go")); err != nil {
		t.Fatalf("expected copied integration file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dstRoot, "verification", "catalogues", "ader", ".artifacts", "tool-cache", "go-mod", "cache.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected sibling catalogue artifact cache to be skipped, stat err = %v", err)
	}
}

func TestRunFullArchivesAndHistoryShow(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "verification", "catalogues", "fixture")
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

func TestRunWritesSuiteSnapshotAndStructuredHistoryID(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "verification", "catalogues", "fixture")

	withWorkingDir(t, root, func() {
		result, err := Run(context.Background(), RunOptions{
			ConfigDir:     configDir,
			Suite:         "fixture",
			Profile:       "unit",
			Lane:          "regression",
			Source:        "committed",
			ArchivePolicy: "never",
		})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if !strings.HasPrefix(result.Manifest.HistoryID, "fixture_committed_unit-") {
			t.Fatalf("history id = %q, want structured fixture_committed_unit-*", result.Manifest.HistoryID)
		}
		snapshotPath := filepath.Join(result.Manifest.ReportDir, reportSuiteSnapshot)
		data, err := os.ReadFile(snapshotPath)
		if err != nil {
			t.Fatalf("read suite snapshot: %v", err)
		}
		text := string(data)
		if !strings.Contains(text, "holons-clem-ader-unit-root:") {
			t.Fatalf("suite snapshot missing explicit step: %q", text)
		}
		if !strings.Contains(text, "profiles:") {
			t.Fatalf("suite snapshot missing profiles section: %q", text)
		}
		if strings.Contains(text, "generated_lanes:") || strings.Contains(text, "generation:") || strings.Contains(text, "generated_groups:") {
			t.Fatalf("suite snapshot should be a plain explicit suite, got obsolete sidecar syntax: %q", text)
		}
	})
}

func TestArchiveLatestAndCleanup(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "verification", "catalogues", "fixture")
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

		removedDir := filepath.Join(configDir, ".artifacts", "local-suite", result.Manifest.HistoryID)
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
	configDir := filepath.Join(root, "verification", "catalogues", "fixture")
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

func TestLoadRunConfigRequiresSuiteAndUsesCatalogueDefaults(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "verification", "catalogues", "fixture")

	cfg, err := loadRunConfig(configDir, "")
	if err == nil || !strings.Contains(err.Error(), "suite is required") {
		t.Fatalf("loadRunConfig() error = %v, want suite required", err)
	}
	cfg, err = loadRunConfig(configDir, "fixture")
	if err != nil {
		t.Fatalf("loadRunConfig() explicit suite error = %v", err)
	}
	if cfg.SuiteName != "fixture" {
		t.Fatalf("suite = %q, want fixture", cfg.SuiteName)
	}
	if cfg.Root.Defaults.Lane != "regression" {
		t.Fatalf("default lane = %q, want regression", cfg.Root.Defaults.Lane)
	}
	if cfg.Root.Defaults.Source != "committed" {
		t.Fatalf("default source = %q, want committed", cfg.Root.Defaults.Source)
	}
	if cfg.Suite.Defaults.Profile != "quick" {
		t.Fatalf("suite default profile = %q, want quick", cfg.Suite.Defaults.Profile)
	}
	if cfg.Suite.Profiles["full"].Archive != "auto" {
		t.Fatalf("full archive policy = %q, want auto", cfg.Suite.Profiles["full"].Archive)
	}
}

func TestResolveProfileLaneSteps(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "verification", "catalogues", "fixture")
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
	want := []string{"fixture-script", "quiet-script"}
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
	if _, err := readSuiteConfig(suitePath, checksConfig{}); err == nil || !strings.Contains(err.Error(), "exactly one of command or script") {
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
	if _, err := readSuiteConfig(suitePath, checksConfig{}); err == nil || !strings.Contains(err.Error(), "cannot define both command and script") {
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
	if _, err := readSuiteConfig(suitePath, checksConfig{}); err == nil || !strings.Contains(err.Error(), "cannot define args without script") {
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
	if _, err := readSuiteConfig(suitePath, checksConfig{}); err == nil || !strings.Contains(err.Error(), "old regression/progression format") {
		t.Fatalf("readSuiteConfig() error = %v, want migration failure", err)
	}
}

func TestReadSuiteConfigRejectsObsoleteSidecarSyntax(t *testing.T) {
	root := t.TempDir()
	suitePath := filepath.Join(root, "fixture.yaml")
	if err := os.WriteFile(suitePath, []byte(`generation:
  generated_file: fixture_generated.yaml
steps:
  ok:
    workdir: .
    command: echo ok
profiles:
  quick:
    steps: [ok]
`), 0o644); err != nil {
		t.Fatalf("write suite: %v", err)
	}
	if _, err := readSuiteConfig(suitePath, checksConfig{}); err == nil || !strings.Contains(err.Error(), `obsolete syntax "generation"`) {
		t.Fatalf("readSuiteConfig() error = %v, want obsolete sidecar syntax failure", err)
	}

	if err := os.WriteFile(suitePath, []byte(`generated_lanes:
  ok: regression
steps:
  ok:
    workdir: .
    command: echo ok
profiles:
  quick:
    steps: [ok]
`), 0o644); err != nil {
		t.Fatalf("rewrite suite with generated_lanes: %v", err)
	}
	if _, err := readSuiteConfig(suitePath, checksConfig{}); err == nil || !strings.Contains(err.Error(), `obsolete syntax "generated_lanes"`) {
		t.Fatalf("readSuiteConfig() error = %v, want generated_lanes rejection", err)
	}

	if err := os.WriteFile(suitePath, []byte(`steps:
  ok:
    workdir: .
    command: echo ok
profiles:
  quick:
    generated_groups: [generated_go_unit_steps]
`), 0o644); err != nil {
		t.Fatalf("rewrite suite with generated_groups: %v", err)
	}
	if _, err := readSuiteConfig(suitePath, checksConfig{}); err == nil || !strings.Contains(err.Error(), `obsolete syntax "generated_groups"`) {
		t.Fatalf("readSuiteConfig() error = %v, want generated_groups rejection", err)
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
	cfg, err := readSuiteConfig(suitePath, checksConfig{})
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
	if _, err := readSuiteConfig(suitePath, checksConfig{}); err == nil || !strings.Contains(err.Error(), "invalid lane") {
		t.Fatalf("readSuiteConfig() error = %v, want invalid-lane failure", err)
	}
}

func TestRunScriptStepWithArgs(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "verification", "catalogues", "fixture")
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

func TestCatalogueLockWaitsForRelease(t *testing.T) {
	artifactsDir := t.TempDir()
	unlock, err := acquireCatalogueLock(context.Background(), artifactsDir)
	if err != nil {
		t.Fatalf("acquireCatalogueLock() initial error = %v", err)
	}
	defer unlock()

	acquired := make(chan struct{})
	go func() {
		secondUnlock, err := acquireCatalogueLock(context.Background(), artifactsDir)
		if err == nil {
			secondUnlock()
		}
		close(acquired)
	}()

	select {
	case <-acquired:
		t.Fatal("second lock acquired before first release")
	case <-time.After(300 * time.Millisecond):
	}

	unlock()

	select {
	case <-acquired:
	case <-time.After(2 * time.Second):
		t.Fatal("second lock did not acquire after release")
	}
}

func TestRunBouquetHistoryShowAndArchive(t *testing.T) {
	root := testrepo.Create(t)
	verificationRoot := filepath.Join(root, "verification")

	withWorkingDir(t, root, func() {
		result, err := RunBouquet(context.Background(), BouquetOptions{
			VerificationRoot: verificationRoot,
			Name:             "local-dev",
		})
		if err != nil {
			t.Fatalf("RunBouquet() error = %v", err)
		}
		if len(result.Entries) != 2 {
			t.Fatalf("bouquet entries = %d, want 2", len(result.Entries))
		}
		if _, err := os.Stat(filepath.Join(result.Manifest.ReportDir, reportSummaryMD)); err != nil {
			t.Fatalf("bouquet summary missing: %v", err)
		}
		history, err := BouquetHistory(context.Background(), verificationRoot)
		if err != nil {
			t.Fatalf("BouquetHistory() error = %v", err)
		}
		if len(history) != 1 {
			t.Fatalf("BouquetHistory() count = %d, want 1", len(history))
		}
		shown, err := ShowBouquetHistory(context.Background(), verificationRoot, result.Manifest.HistoryID)
		if err != nil {
			t.Fatalf("ShowBouquetHistory() error = %v", err)
		}
		if shown.Manifest.HistoryID != result.Manifest.HistoryID {
			t.Fatalf("ShowBouquetHistory() history id = %q, want %q", shown.Manifest.HistoryID, result.Manifest.HistoryID)
		}
		archived, err := ArchiveBouquet(context.Background(), BouquetArchiveOptions{
			VerificationRoot: verificationRoot,
			Latest:           true,
		})
		if err != nil {
			t.Fatalf("ArchiveBouquet() error = %v", err)
		}
		if archived.Manifest.ArchivePath == "" {
			t.Fatal("expected bouquet archive path")
		}
		if _, err := os.Stat(archived.Manifest.ArchivePath); err != nil {
			t.Fatalf("bouquet archive missing: %v", err)
		}
	})
}

func TestProgressReporterPhaseHeartbeat(t *testing.T) {
	oldInterval := progressHeartbeatInterval
	oldPoll := progressHeartbeatPollInterval
	progressHeartbeatInterval = 20 * time.Millisecond
	progressHeartbeatPollInterval = 5 * time.Millisecond
	t.Cleanup(func() {
		progressHeartbeatInterval = oldInterval
		progressHeartbeatPollInterval = oldPoll
	})

	var buf bytes.Buffer
	reporter := newProgressReporter(&buf)
	if err := reporter.withPhase("snapshot workspace", "snapshot ready", func() error {
		time.Sleep(70 * time.Millisecond)
		return nil
	}); err != nil {
		t.Fatalf("withPhase() error = %v", err)
	}
	text := buf.String()
	if !strings.Contains(text, "[phase] snapshot workspace") {
		t.Fatalf("progress missing phase start: %q", text)
	}
	if !strings.Contains(text, "[wait] snapshot workspace still running") {
		t.Fatalf("progress missing heartbeat: %q", text)
	}
	if !strings.Contains(text, "[phase] snapshot ready (") {
		t.Fatalf("progress missing phase completion: %q", text)
	}
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
