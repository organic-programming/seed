package engine

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	reportManifestName  = "manifest.json"
	reportStepsName     = "step-results.json"
	reportSummaryMD     = "summary.md"
	reportSummaryTSV    = "summary.tsv"
	reportToolVersions  = "tool-versions.txt"
	reportSuiteSnapshot = "suite-snapshot.yaml"
	reportPromotionJSON = "promotion.json"
	reportPromotionMD   = "promotion.md"
	reportLogsDir       = "logs"
	runPIDFile          = ".ader-run.pid"
	tempStorePrefix     = "ader-int-store-"
)

type repoPaths struct {
	RepoRoot       string
	ConfigDir      string
	ArtifactsDir   string
	LocalSuiteDir  string
	ToolCacheDir   string
	ReportsDir     string
	ArchivesDir    string
	ShortTempAlias string
}

func Run(ctx context.Context, opts RunOptions) (*RunResult, error) {
	return run(ctx, opts, nil)
}

func RunWithProgress(ctx context.Context, opts RunOptions, progress io.Writer) (*RunResult, error) {
	return run(ctx, opts, progress)
}

func run(ctx context.Context, opts RunOptions, progress io.Writer) (*RunResult, error) {
	reporter := newProgressReporter(progress)
	if err := validateRunOptions(opts); err != nil {
		return nil, err
	}
	runtimeCfg, err := loadRunConfig(opts.ConfigDir, opts.Suite)
	if err != nil {
		return nil, err
	}
	unlock, err := acquireCatalogueLock(ctx, runtimeCfg.Paths.ArtifactsDir)
	if err != nil {
		return nil, err
	}
	defer unlock()
	reporter.phase("config loaded")
	paths := runtimeCfg.Paths
	repoRoot := runtimeCfg.RepoRoot
	if err := os.MkdirAll(paths.LocalSuiteDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(paths.ToolCacheDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(paths.ReportsDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(paths.ArchivesDir, 0o755); err != nil {
		return nil, err
	}

	commitHash, branch, dirty := gitMetadata(repoRoot)
	profile := resolveProfileName(runtimeCfg, opts.Profile)
	source := normalizeSource(firstNonEmpty(opts.Source, runtimeCfg.Root.Defaults.Source))
	lane := normalizeLane(firstNonEmpty(opts.Lane, runtimeCfg.Root.Defaults.Lane))
	archivePolicy := normalizeArchivePolicy(firstNonEmpty(opts.ArchivePolicy, runtimeCfg.Suite.Profiles[profile].Archive))
	started := time.Now()
	runID, err := newHistoryID(paths.ReportsDir, runtimeCfg.SuiteName, source, profile, started)
	if err != nil {
		return nil, err
	}
	tmpStore, err := os.MkdirTemp(os.TempDir(), tempStorePrefix+sanitizeHistoryToken(runtimeCfg.CatalogueName)+"-"+runID+"-")
	if err != nil {
		return nil, err
	}
	runArtifactsDir := filepath.Join(tmpStore, "run")
	snapshotRoot := filepath.Join(runArtifactsDir, "snapshot")
	preservedRunDir := filepath.Join(paths.LocalSuiteDir, runID)
	reportDir := filepath.Join(paths.ReportsDir, runID)
	logDir := filepath.Join(reportDir, reportLogsDir)

	for _, dir := range []string{runArtifactsDir, snapshotRoot, reportDir, logDir, tmpStore, filepath.Join(paths.ToolCacheDir, "go-build"), filepath.Join(paths.ToolCacheDir, "go-mod"), filepath.Join(paths.ToolCacheDir, "gradle"), filepath.Join(paths.ToolCacheDir, "npm"), filepath.Join(paths.ToolCacheDir, "bundle"), filepath.Join(paths.ToolCacheDir, "dart-pub"), filepath.Join(paths.ToolCacheDir, "nuget"), filepath.Join(paths.ToolCacheDir, "dotnet-home")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	if err := writePIDMarker(filepath.Join(runArtifactsDir, runPIDFile)); err != nil {
		return nil, err
	}
	if err := writePIDMarker(filepath.Join(tmpStore, runPIDFile)); err != nil {
		return nil, err
	}
	defer func() {
		_ = os.Remove(filepath.Join(runArtifactsDir, runPIDFile))
		_ = os.Remove(filepath.Join(tmpStore, runPIDFile))
		_ = os.Remove(paths.ShortTempAlias)
	}()

	reporter.phase(fmt.Sprintf("source %s", source))

	if err := reporter.withPhase("snapshot "+source, "snapshot ready", func() error {
		switch source {
		case "committed":
			return snapshotCommitted(ctx, repoRoot, snapshotRoot)
		case "workspace":
			return snapshotWorkspace(repoRoot, snapshotRoot, paths, runtimeCfg.ConfigRelDir)
		default:
			return fmt.Errorf("unsupported source %q", source)
		}
	}); err != nil {
		return nil, err
	}
	if err := reporter.withPhase("snapshot normalization", "snapshot normalized", func() error {
		return normalizeSnapshot(snapshotRoot)
	}); err != nil {
		return nil, err
	}
	if err := replaceShortTempAlias(paths.ShortTempAlias, tmpStore); err != nil {
		return nil, err
	}
	reporter.phase("environment ready")

	steps, err := resolveProfileLaneSteps(runtimeCfg, profile, lane, snapshotRoot)
	if err != nil {
		return nil, err
	}
	steps, err = filterSteps(steps, opts.StepFilter)
	if err != nil {
		return nil, err
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("no steps matched profile %q and filter %q", profile, opts.StepFilter)
	}
	reporter.phase(fmt.Sprintf("selected %d steps for profile=%s lane=%s", len(steps), profile, lane))

	manifest := HistoryRecord{
		ConfigDir:     runtimeCfg.ConfigDir,
		Suite:         runtimeCfg.SuiteName,
		HistoryID:     runID,
		Profile:       profile,
		Lane:          lane,
		Source:        source,
		ArchivePolicy: archivePolicy,
		StepFilter:    strings.TrimSpace(opts.StepFilter),
		RepoRoot:      repoRoot,
		SnapshotRoot:  snapshotRoot,
		ReportDir:     reportDir,
		CommitHash:    commitHash,
		Branch:        branch,
		Dirty:         dirty,
		StartedAt:     started.UTC().Format(time.RFC3339),
	}

	toolVersions, _ := collectToolVersions()
	env := runEnvironment(paths, repoRoot, snapshotRoot, runArtifactsDir, tmpStore)

	results := make([]StepResult, 0, len(steps))
	for index, step := range steps {
		logPath := filepath.Join(logDir, step.ID+".log")
		result := StepResult{
			StepID:      step.ID,
			Lane:        step.Lane,
			Description: step.Description,
			Workdir:     step.Workdir,
			Command:     displayStepCommand(step),
			LogPath:     logPath,
		}

		if !dirExists(step.Workdir) {
			result.Status = "SKIP"
			result.Reason = "workdir missing from snapshot"
			if err := os.WriteFile(logPath, []byte("status: SKIP\nreason: workdir missing from snapshot\n"), 0o644); err != nil {
				return nil, err
			}
			results = append(results, result)
			manifest.SkipCount++
			printProgress(reporter, "[%02d/%02d] SKIP %s (workdir missing from snapshot)\n", index+1, len(steps), step.ID)
			continue
		}

		if missing := missingPrereqs(step.Prereqs); len(missing) > 0 {
			result.Status = "SKIP"
			result.Reason = "missing prerequisites: " + strings.Join(missing, ", ")
			if err := os.WriteFile(logPath, []byte("status: SKIP\nreason: "+result.Reason+"\n"), 0o644); err != nil {
				return nil, err
			}
			results = append(results, result)
			manifest.SkipCount++
			printProgress(reporter, "[%02d/%02d] SKIP %s (%s)\n", index+1, len(steps), step.ID, result.Reason)
			continue
		}

		if reason := setupSkipReason(step.ID, step.Workdir); reason != "" {
			result.Status = "SKIP"
			result.Reason = reason
			if err := os.WriteFile(logPath, []byte("status: SKIP\nreason: "+reason+"\n"), 0o644); err != nil {
				return nil, err
			}
			results = append(results, result)
			manifest.SkipCount++
			printProgress(reporter, "[%02d/%02d] SKIP %s (%s)\n", index+1, len(steps), step.ID, reason)
			continue
		}

		printProgress(reporter, "[%02d/%02d] RUN  %s\n", index+1, len(steps), step.ID)
		printProgress(reporter, "[%02d/%02d] CMD %s :: %s\n", index+1, len(steps), displayStepWorkdir(snapshotRoot, step.Workdir), displayStepCommand(step))
		start := time.Now().UTC()
		result.StartedAt = start.Format(time.RFC3339)
		code, err := runStepCommand(ctx, step, env, logPath, reporter)
		end := time.Now().UTC()
		result.FinishedAt = end.Format(time.RFC3339)
		result.DurationSeconds = int64(end.Sub(start).Seconds())
		if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
			result.Reason = err.Error()
		}
		if code == 0 {
			result.Status = "PASS"
			manifest.PassCount++
			printProgress(reporter, "[%02d/%02d] PASS %s (%ds)\n", index+1, len(steps), step.ID, result.DurationSeconds)
		} else {
			result.Status = "FAIL"
			manifest.FailCount++
			printProgress(reporter, "[%02d/%02d] FAIL %s (%ds)\n", index+1, len(steps), step.ID, result.DurationSeconds)
		}
		results = append(results, result)
	}

	manifest.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	if manifest.FailCount == 0 {
		manifest.FinalStatus = "PASS"
	} else {
		manifest.FinalStatus = "FAIL"
	}

	summaryMD := buildSummaryMarkdown(manifest, results)
	summaryTSV := buildSummaryTSV(results)
	result := &RunResult{
		Manifest:        manifest,
		Steps:           results,
		SummaryMarkdown: summaryMD,
		SummaryTSV:      summaryTSV,
		SuiteSnapshot:   runtimeCfg.EffectiveSuiteYAML,
	}
	if proposal := buildPromotionProposal(runtimeCfg, result); proposal != nil {
		result.Promotion = proposal
	}
	if err := reporter.withPhase("writing report", "report ready", func() error {
		return writeReport(result, reportDir, toolVersions)
	}); err != nil {
		return nil, err
	}

	if shouldArchive(profile, archivePolicy) {
		var archivePath string
		if err := reporter.withPhase("archiving report", "archive ready", func() error {
			var err error
			archivePath, err = archiveReportDir(paths, manifest, reportDir)
			return err
		}); err != nil {
			return result, err
		}
		result.Manifest.ArchivePath = archivePath
		if err := reporter.withPhase("writing archived report", "archived report ready", func() error {
			return writeReport(result, reportDir, toolVersions)
		}); err != nil {
			return nil, err
		}
		if !opts.KeepReport {
			_ = os.RemoveAll(reportDir)
		}
	}

	if opts.KeepSnapshot {
		if err := reporter.withPhase("preserving snapshot", "snapshot preserved", func() error {
			_ = os.RemoveAll(preservedRunDir)
			if err := os.MkdirAll(filepath.Dir(preservedRunDir), 0o755); err != nil {
				return err
			}
			return copyTree(runArtifactsDir, preservedRunDir)
		}); err != nil {
			return nil, err
		}
		_ = os.Remove(filepath.Join(preservedRunDir, runPIDFile))
		result.Manifest.SnapshotRoot = filepath.Join(preservedRunDir, "snapshot")
		if err := reporter.withPhase("writing preserved report", "preserved report ready", func() error {
			return writeReport(result, reportDir, toolVersions)
		}); err != nil {
			return nil, err
		}
	}
	_ = reporter.withPhase("cleanup temp store", "cleanup complete", func() error {
		return os.RemoveAll(tmpStore)
	})

	return result, nil
}

func Archive(ctx context.Context, opts ArchiveOptions) (*RunResult, error) {
	runtimeCfg, err := loadRepoConfig(opts.ConfigDir)
	if err != nil {
		return nil, err
	}
	unlock, err := acquireCatalogueLock(ctx, runtimeCfg.Paths.ArtifactsDir)
	if err != nil {
		return nil, err
	}
	defer unlock()
	paths := runtimeCfg.Paths
	result, reportDir, err := loadRunForArchive(paths, opts)
	if err != nil {
		return nil, err
	}
	archivePath, err := archiveReportDir(paths, result.Manifest, reportDir)
	if err != nil {
		return nil, err
	}
	result.Manifest.ArchivePath = archivePath
	toolVersions, _ := os.ReadFile(filepath.Join(reportDir, reportToolVersions))
	if err := writeReport(result, reportDir, string(toolVersions)); err != nil {
		return nil, err
	}
	return result, nil
}

func Cleanup(_ context.Context, configDir string) (*CleanupResult, error) {
	runtimeCfg, err := loadRepoConfig(configDir)
	if err != nil {
		return nil, err
	}
	unlock, err := acquireCatalogueLock(context.Background(), runtimeCfg.Paths.ArtifactsDir)
	if err != nil {
		return nil, err
	}
	defer unlock()
	paths := runtimeCfg.Paths
	result := &CleanupResult{}
	if target, ok := symlinkTarget(paths.ShortTempAlias); ok {
		if err := os.Remove(paths.ShortTempAlias); err == nil {
			result.RemovedTempAliases++
			result.RemovedPaths = append(result.RemovedPaths, paths.ShortTempAlias)
		}
		if strings.Contains(filepath.Base(target), tempStorePrefix) && removableRunDir(target) {
			if err := os.RemoveAll(target); err == nil {
				result.RemovedTempStores++
				result.RemovedPaths = append(result.RemovedPaths, target)
			}
		}
	}
	entries, _ := os.ReadDir(paths.LocalSuiteDir)
	for _, entry := range entries {
		fullPath := filepath.Join(paths.LocalSuiteDir, entry.Name())
		if !entry.IsDir() {
			continue
		}
		if removableRunDir(fullPath) {
			if err := os.RemoveAll(fullPath); err == nil {
				result.RemovedLocalSuiteDirs++
				result.RemovedPaths = append(result.RemovedPaths, fullPath)
			}
		}
	}
	tempEntries, _ := os.ReadDir(os.TempDir())
	for _, entry := range tempEntries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), tempStorePrefix) {
			continue
		}
		fullPath := filepath.Join(os.TempDir(), entry.Name())
		if removableRunDir(fullPath) {
			if err := os.RemoveAll(fullPath); err == nil {
				result.RemovedTempStores++
				result.RemovedPaths = append(result.RemovedPaths, fullPath)
			}
		}
	}
	sort.Strings(result.RemovedPaths)
	return result, nil
}

func History(_ context.Context, configDir string) ([]HistoryEntry, error) {
	runtimeCfg, err := loadRepoConfig(configDir)
	if err != nil {
		return nil, err
	}
	paths := runtimeCfg.Paths
	items := map[string]HistoryEntry{}
	reportEntries, _ := os.ReadDir(paths.ReportsDir)
	for _, entry := range reportEntries {
		if !entry.IsDir() {
			continue
		}
		manifest, err := readManifestFile(filepath.Join(paths.ReportsDir, entry.Name(), reportManifestName))
		if err != nil {
			continue
		}
		items[manifest.HistoryID] = summaryFromManifest(manifest)
	}
	archivePaths, _ := findArchivePaths(paths.ArchivesDir)
	for _, archivePath := range archivePaths {
		manifest, _, _, err := readArchivePayload(archivePath)
		if err != nil {
			continue
		}
		summary := summaryFromManifest(manifest)
		summary.ArchivePath = archivePath
		if existing, ok := items[summary.HistoryID]; ok {
			if existing.ReportDir != "" {
				existing.ArchivePath = archivePath
				items[summary.HistoryID] = existing
				continue
			}
		}
		items[summary.HistoryID] = summary
	}
	out := make([]HistoryEntry, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		left := historyTimestamp(out[i])
		right := historyTimestamp(out[j])
		if !left.Equal(right) {
			return left.After(right)
		}
		return out[i].HistoryID > out[j].HistoryID
	})
	return out, nil
}

func ShowHistory(_ context.Context, configDir string, historyID string) (*RunResult, error) {
	runtimeCfg, err := loadRepoConfig(configDir)
	if err != nil {
		return nil, err
	}
	paths := runtimeCfg.Paths
	reportDir := filepath.Join(paths.ReportsDir, historyID)
	if dirExists(reportDir) {
		return loadRunFromReportDir(reportDir)
	}
	archivePaths, _ := findArchivePaths(paths.ArchivesDir)
	for _, archivePath := range archivePaths {
		manifest, steps, summaryMD, err := readArchivePayload(archivePath)
		if err != nil || manifest.HistoryID != historyID {
			continue
		}
		summaryTSV, _ := readArchiveFile(archivePath, reportSummaryTSV)
		manifest.ArchivePath = archivePath
		return &RunResult{
			Manifest:        manifest,
			Steps:           steps,
			SummaryMarkdown: summaryMD,
			SummaryTSV:      summaryTSV,
		}, nil
	}
	return nil, fmt.Errorf("history %q not found", historyID)
}

func gitMetadata(repoRoot string) (commitHash string, branch string, dirty bool) {
	commitHash = strings.TrimSpace(runGit(repoRoot, "rev-parse", "HEAD"))
	if commitHash == "" {
		commitHash = "unknown"
	}
	branch = strings.TrimSpace(runGit(repoRoot, "rev-parse", "--abbrev-ref", "HEAD"))
	if branch == "" {
		branch = "unknown"
	}
	dirty = strings.TrimSpace(runGit(repoRoot, "status", "--porcelain")) != ""
	return commitHash, branch, dirty
}

func runGit(repoRoot string, args ...string) string {
	cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(output)
}

func snapshotCommitted(ctx context.Context, repoRoot string, snapshotRoot string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "archive", "--format=tar", "HEAD")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := extractTar(snapshotRoot, stdout); err != nil {
		_ = cmd.Wait()
		return err
	}
	return cmd.Wait()
}

func snapshotWorkspace(repoRoot string, snapshotRoot string, paths repoPaths, configRelDir string) error {
	return copyWorkspaceTree(repoRoot, snapshotRoot, configRelDir, paths.ArtifactsDir, paths.ReportsDir, paths.ArchivesDir, paths.ShortTempAlias)
}

func normalizeSnapshot(snapshotRoot string) error {
	rootProtos := filepath.Join(snapshotRoot, "_protos")
	if dirExists(rootProtos) {
		return nil
	}
	exampleProtos := filepath.Join(snapshotRoot, "examples", "_protos")
	if !dirExists(exampleProtos) {
		return nil
	}
	return copyTree(exampleProtos, rootProtos)
}

func shouldArchive(profile string, policy string) bool {
	switch policy {
	case "always":
		return true
	case "never":
		return false
	default:
		return profile == "full"
	}
}

func runEnvironment(paths repoPaths, repoRoot string, snapshotRoot string, runArtifactsDir string, tmpStore string) []string {
	env := append([]string(nil), os.Environ()...)
	setEnv := func(key, value string) {
		for i := range env {
			if strings.HasPrefix(env[i], key+"=") {
				env[i] = key + "=" + value
				return
			}
		}
		env = append(env, key+"="+value)
	}
	setEnv("ADER_LIVE_REPO_ROOT", repoRoot)
	setEnv("ADER_REPO_ROOT", snapshotRoot)
	setEnv("ADER_CONFIG_DIR", paths.ConfigDir)
	setEnv("ADER_RUN_ARTIFACTS", runArtifactsDir)
	setEnv("ADER_TOOL_CACHE", paths.ToolCacheDir)
	setEnv("TMPDIR", tmpStore)
	setEnv("TMP", tmpStore)
	setEnv("TEMP", tmpStore)
	setEnv("GOCACHE", filepath.Join(paths.ToolCacheDir, "go-build"))
	setEnv("GOMODCACHE", filepath.Join(paths.ToolCacheDir, "go-mod"))
	setEnv("GRADLE_USER_HOME", filepath.Join(paths.ToolCacheDir, "gradle"))
	setEnv("npm_config_cache", filepath.Join(paths.ToolCacheDir, "npm"))
	setEnv("BUNDLE_PATH", filepath.Join(paths.ToolCacheDir, "bundle"))
	setEnv("PUB_CACHE", filepath.Join(paths.ToolCacheDir, "dart-pub"))
	setEnv("DOTNET_CLI_HOME", filepath.Join(paths.ToolCacheDir, "dotnet-home"))
	setEnv("NUGET_PACKAGES", filepath.Join(paths.ToolCacheDir, "nuget"))
	setEnv("CARGO_TARGET_DIR", filepath.Join(runArtifactsDir, "cargo", "default"))
	setEnv("PYTHONDONTWRITEBYTECODE", "1")
	return env
}

func runStepCommand(ctx context.Context, step StepSpec, env []string, logPath string, progress *progressReporter) (int, error) {
	logFile, err := os.Create(logPath)
	if err != nil {
		return 1, err
	}
	defer logFile.Close()
	writer := io.Writer(logFile)
	var monitor *heartbeatMonitor
	if progress != nil && progress.enabled() {
		monitor = progress.startMonitor(step.ID, "no output yet")
		defer monitor.stop()
		writer = io.MultiWriter(logFile, monitor.writer())
	}
	cmd, err := buildStepCommand(ctx, step)
	if err != nil {
		return 1, err
	}
	cmd.Dir = step.Workdir
	cmd.Env = env
	cmd.Stdout = writer
	cmd.Stderr = writer
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				return status.ExitStatus(), nil
			}
			return exitErr.ExitCode(), nil
		}
		return 1, err
	}
	return 0, nil
}

func buildStepCommand(ctx context.Context, step StepSpec) (*exec.Cmd, error) {
	if strings.TrimSpace(step.Script) != "" {
		info, err := os.Stat(step.Script)
		if err != nil {
			return nil, fmt.Errorf("script %s: %w", step.Script, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("script %s is a directory", step.Script)
		}
		if info.Mode()&0o111 == 0 {
			return nil, fmt.Errorf("script %s is not executable", step.Script)
		}
		return exec.CommandContext(ctx, step.Script, step.Args...), nil
	}
	return exec.CommandContext(ctx, "bash", "-lc", step.Command), nil
}

func displayStepCommand(step StepSpec) string {
	if strings.TrimSpace(step.Script) != "" {
		script := filepath.ToSlash(step.Script)
		if rel, err := filepath.Rel(step.Workdir, step.Script); err == nil && strings.TrimSpace(rel) != "" && rel != "." && !strings.HasPrefix(rel, "..") {
			script = filepath.ToSlash(rel)
		}
		parts := append([]string{script}, step.Args...)
		return strings.Join(parts, " ")
	}
	return step.Command
}

func displayStepWorkdir(snapshotRoot string, workdir string) string {
	if rel, err := filepath.Rel(snapshotRoot, workdir); err == nil && strings.TrimSpace(rel) != "" && rel != "." && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(workdir)
}

func missingPrereqs(prereqs []string) []string {
	var missing []string
	for _, prereq := range prereqs {
		if strings.TrimSpace(prereq) == "" {
			continue
		}
		if _, err := exec.LookPath(prereq); err != nil {
			missing = append(missing, prereq)
		}
	}
	return missing
}

func setupSkipReason(stepID string, workdir string) string {
	switch stepID {
	case "sdk-js-unit", "example-node-unit", "sdk-js-web-unit":
		if !dirExists(filepath.Join(workdir, "node_modules")) {
			return "dependencies not restored: missing node_modules"
		}
	case "sdk-ruby-unit", "example-ruby-unit":
		cmd := exec.Command("bash", "-lc", "bundle exec ruby -e 'exit 0'")
		cmd.Dir = workdir
		if err := cmd.Run(); err != nil {
			return "gems not installed or bundler/runtime mismatch"
		}
	}
	return ""
}

func buildSummaryMarkdown(manifest HistoryRecord, steps []StepResult) string {
	var b strings.Builder
	fmt.Fprintln(&b, "# Verification Report")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "- Config Dir: `%s`\n", manifest.ConfigDir)
	fmt.Fprintf(&b, "- Suite: `%s`\n", manifest.Suite)
	fmt.Fprintf(&b, "- History ID: `%s`\n", manifest.HistoryID)
	fmt.Fprintf(&b, "- Profile: `%s`\n", manifest.Profile)
	fmt.Fprintf(&b, "- Lane: `%s`\n", manifest.Lane)
	fmt.Fprintf(&b, "- Source: `%s`\n", manifest.Source)
	fmt.Fprintf(&b, "- Step Filter: `%s`\n", emptyAsNone(manifest.StepFilter))
	fmt.Fprintf(&b, "- Started: `%s`\n", manifest.StartedAt)
	fmt.Fprintf(&b, "- Finished: `%s`\n", manifest.FinishedAt)
	fmt.Fprintf(&b, "- Commit: `%s`\n", manifest.CommitHash)
	fmt.Fprintf(&b, "- Branch: `%s`\n", manifest.Branch)
	fmt.Fprintf(&b, "- Dirty: `%t`\n", manifest.Dirty)
	fmt.Fprintf(&b, "- Snapshot Root: `%s`\n", manifest.SnapshotRoot)
	fmt.Fprintf(&b, "- Report Dir: `%s`\n", manifest.ReportDir)
	if strings.TrimSpace(manifest.ArchivePath) != "" {
		fmt.Fprintf(&b, "- Archive: `%s`\n", manifest.ArchivePath)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| Status | Lane | Duration | Step | Description | Workdir | Command | Log |")
	fmt.Fprintln(&b, "| --- | --- | --- | --- | --- | --- | --- | --- |")
	for _, step := range steps {
		relLog := filepath.Join(reportLogsDir, filepath.Base(step.LogPath))
		fmt.Fprintf(&b, "| %s | %s | %ds | `%s` | %s | `%s` | `%s` | [%s](%s) |\n",
			step.Status,
			emptyAsNone(step.Lane),
			step.DurationSeconds,
			step.StepID,
			step.Description,
			step.Workdir,
			step.Command,
			relLog,
			relLog,
		)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Totals")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "- Pass: %d\n", manifest.PassCount)
	fmt.Fprintf(&b, "- Fail: %d\n", manifest.FailCount)
	fmt.Fprintf(&b, "- Skip: %d\n", manifest.SkipCount)
	return b.String()
}

func buildSummaryTSV(steps []StepResult) string {
	var b strings.Builder
	fmt.Fprintln(&b, "status\tlane\tduration_sec\tstep\tdescription\tworkdir\tcommand\tlog")
	for _, step := range steps {
		fmt.Fprintf(&b, "%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\n",
			step.Status,
			step.Lane,
			step.DurationSeconds,
			step.StepID,
			step.Description,
			step.Workdir,
			step.Command,
			step.LogPath,
		)
	}
	return b.String()
}

func writeReport(result *RunResult, reportDir string, toolVersions string) error {
	if err := os.MkdirAll(filepath.Join(reportDir, reportLogsDir), 0o755); err != nil {
		return err
	}
	manifestJSON, err := json.MarshalIndent(result.Manifest, "", "  ")
	if err != nil {
		return err
	}
	stepsJSON, err := json.MarshalIndent(result.Steps, "", "  ")
	if err != nil {
		return err
	}
	files := map[string][]byte{
		filepath.Join(reportDir, reportManifestName): []byte(string(manifestJSON) + "\n"),
		filepath.Join(reportDir, reportStepsName):    []byte(string(stepsJSON) + "\n"),
		filepath.Join(reportDir, reportSummaryMD):    []byte(result.SummaryMarkdown),
		filepath.Join(reportDir, reportSummaryTSV):   []byte(result.SummaryTSV),
		filepath.Join(reportDir, reportToolVersions): []byte(toolVersions),
	}
	if strings.TrimSpace(result.SuiteSnapshot) != "" {
		files[filepath.Join(reportDir, reportSuiteSnapshot)] = []byte(result.SuiteSnapshot)
	}
	if result.Promotion != nil {
		promotionJSON, err := json.MarshalIndent(result.Promotion, "", "  ")
		if err != nil {
			return err
		}
		files[filepath.Join(reportDir, reportPromotionJSON)] = []byte(string(promotionJSON) + "\n")
		files[filepath.Join(reportDir, reportPromotionMD)] = []byte(buildPromotionMarkdown(result.Promotion))
	}
	for path, content := range files {
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func archiveReportDir(paths repoPaths, manifest HistoryRecord, reportDir string) (string, error) {
	commitDir := filepath.Join(paths.ArchivesDir, manifest.CommitHash)
	if err := os.MkdirAll(commitDir, 0o755); err != nil {
		return "", err
	}
	suffix := ""
	if manifest.Source == "workspace" && manifest.Dirty {
		suffix = "-dirty"
	}
	archivePath := filepath.Join(commitDir, fmt.Sprintf("%s%s.tar.gz", manifest.HistoryID, suffix))
	file, err := os.Create(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	gz := gzip.NewWriter(file)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	if err := filepath.Walk(reportDir, func(path string, info fs.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(reportDir, path)
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		source, err := os.Open(path)
		if err != nil {
			return err
		}
		defer source.Close()
		_, err = io.Copy(tw, source)
		return err
	}); err != nil {
		return "", err
	}
	return archivePath, nil
}

func loadRunForArchive(paths repoPaths, opts ArchiveOptions) (*RunResult, string, error) {
	if opts.Latest {
		reportDir, err := latestReportDir(paths.ReportsDir)
		if err != nil {
			return nil, "", err
		}
		result, err := loadRunFromReportDir(reportDir)
		return result, reportDir, err
	}
	historyID := strings.TrimSpace(opts.HistoryID)
	if historyID == "" {
		return nil, "", fmt.Errorf("archive requires --id or --latest")
	}
	reportDir := filepath.Join(paths.ReportsDir, historyID)
	if !dirExists(reportDir) {
		if archivePath, ok := findArchiveByHistoryID(paths.ArchivesDir, historyID); ok {
			manifest, steps, summaryMD, err := readArchivePayload(archivePath)
			if err != nil {
				return nil, "", err
			}
			summaryTSV, _ := readArchiveFile(archivePath, reportSummaryTSV)
			manifest.ArchivePath = archivePath
			return &RunResult{
				Manifest:        manifest,
				Steps:           steps,
				SummaryMarkdown: summaryMD,
				SummaryTSV:      summaryTSV,
			}, "", nil
		}
		return nil, "", fmt.Errorf("history %q not found", historyID)
	}
	result, err := loadRunFromReportDir(reportDir)
	return result, reportDir, err
}

func latestReportDir(reportsDir string) (string, error) {
	entries, err := os.ReadDir(reportsDir)
	if err != nil {
		return "", err
	}
	type candidate struct {
		path      string
		startedAt time.Time
		historyID string
	}
	var candidates []candidate
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		fullPath := filepath.Join(reportsDir, entry.Name())
		manifest, err := readManifestFile(filepath.Join(fullPath, reportManifestName))
		if err == nil {
			candidates = append(candidates, candidate{
				path:      fullPath,
				startedAt: historyTimestamp(summaryFromManifest(manifest)),
				historyID: manifest.HistoryID,
			})
			continue
		}
		candidates = append(candidates, candidate{path: fullPath, historyID: entry.Name()})
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no extracted runs found under %s", reportsDir)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if !candidates[i].startedAt.Equal(candidates[j].startedAt) {
			return candidates[i].startedAt.After(candidates[j].startedAt)
		}
		return candidates[i].historyID > candidates[j].historyID
	})
	return candidates[0].path, nil
}

func loadRunFromReportDir(reportDir string) (*RunResult, error) {
	manifest, err := readManifestFile(filepath.Join(reportDir, reportManifestName))
	if err != nil {
		return nil, err
	}
	steps, err := readStepsFile(filepath.Join(reportDir, reportStepsName))
	if err != nil {
		return nil, err
	}
	summaryMD, _ := os.ReadFile(filepath.Join(reportDir, reportSummaryMD))
	summaryTSV, _ := os.ReadFile(filepath.Join(reportDir, reportSummaryTSV))
	return &RunResult{
		Manifest:        manifest,
		Steps:           steps,
		SummaryMarkdown: string(summaryMD),
		SummaryTSV:      string(summaryTSV),
	}, nil
}

func readManifestFile(path string) (HistoryRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return HistoryRecord{}, err
	}
	var manifest HistoryRecord
	if err := json.Unmarshal(data, &manifest); err != nil {
		return HistoryRecord{}, err
	}
	return manifest, nil
}

func readStepsFile(path string) ([]StepResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var steps []StepResult
	if err := json.Unmarshal(data, &steps); err != nil {
		return nil, err
	}
	return steps, nil
}

func findArchivePaths(archivesDir string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(archivesDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".tar.gz") {
			paths = append(paths, path)
		}
		return nil
	})
	sort.Strings(paths)
	return paths, err
}

func findArchiveByHistoryID(archivesDir string, historyID string) (string, bool) {
	paths, err := findArchivePaths(archivesDir)
	if err != nil {
		return "", false
	}
	for _, path := range paths {
		if strings.Contains(filepath.Base(path), historyID+"-") {
			return path, true
		}
	}
	return "", false
}

func readArchivePayload(path string) (HistoryRecord, []StepResult, string, error) {
	manifestData, err := readArchiveFile(path, reportManifestName)
	if err != nil {
		return HistoryRecord{}, nil, "", err
	}
	stepsData, err := readArchiveFile(path, reportStepsName)
	if err != nil {
		return HistoryRecord{}, nil, "", err
	}
	summaryMD, err := readArchiveFile(path, reportSummaryMD)
	if err != nil {
		return HistoryRecord{}, nil, "", err
	}
	var manifest HistoryRecord
	if err := json.Unmarshal([]byte(manifestData), &manifest); err != nil {
		return HistoryRecord{}, nil, "", err
	}
	var steps []StepResult
	if err := json.Unmarshal([]byte(stepsData), &steps); err != nil {
		return HistoryRecord{}, nil, "", err
	}
	return manifest, steps, summaryMD, nil
}

func readArchiveFile(path string, name string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		if filepath.ToSlash(name) == header.Name {
			data, err := io.ReadAll(tr)
			if err != nil {
				return "", err
			}
			return string(data), nil
		}
	}
	return "", fmt.Errorf("%s not found in %s", name, path)
}

func summaryFromManifest(manifest HistoryRecord) HistoryEntry {
	return HistoryEntry{
		HistoryID:   manifest.HistoryID,
		Suite:       manifest.Suite,
		Profile:     manifest.Profile,
		Lane:        manifest.Lane,
		Source:      manifest.Source,
		FinalStatus: manifest.FinalStatus,
		CommitHash:  manifest.CommitHash,
		Dirty:       manifest.Dirty,
		StartedAt:   manifest.StartedAt,
		FinishedAt:  manifest.FinishedAt,
		ReportDir:   manifest.ReportDir,
		ArchivePath: manifest.ArchivePath,
	}
}

func buildPromotionProposal(cfg *runtimeConfig, result *RunResult) *PromotionProposal {
	if result.Manifest.Lane == "regression" {
		return nil
	}
	eligible := make([]string, 0)
	for _, step := range result.Steps {
		if step.Lane == "progression" && step.Status == "PASS" {
			eligible = append(eligible, step.StepID)
		}
	}
	if len(eligible) == 0 {
		return nil
	}
	sort.Strings(eligible)
	configDir := cfg.ConfigRelDir
	if strings.TrimSpace(configDir) == "" {
		configDir = cfg.ConfigDir
	}
	suiteFile := cfg.SuitePath
	if rel, err := filepath.Rel(cfg.RepoRoot, cfg.SuitePath); err == nil {
		suiteFile = filepath.ToSlash(rel)
	}
	promoteArgs := make([]string, 0, len(eligible)+6)
	promoteArgs = append(promoteArgs, "ader", "promote", configDir, "--suite", result.Manifest.Suite)
	for _, id := range eligible {
		promoteArgs = append(promoteArgs, "--step", id)
	}
	suggestedCommand := strings.Join(promoteArgs, " ")

	return &PromotionProposal{
		Suite:            result.Manifest.Suite,
		Profile:          result.Manifest.Profile,
		Lane:             result.Manifest.Lane,
		DestinationLane:  "regression",
		SuiteFile:        suiteFile,
		EligibleSteps:    eligible,
		SuggestedCommand: suggestedCommand,
		SuggestedGitCommands: []string{
			fmt.Sprintf("git add %s", suiteFile),
			fmt.Sprintf("git commit -m %q", fmt.Sprintf("Promote %s to regression", strings.Join(eligible, ", "))),
		},
		SuggestedCommitMessage: fmt.Sprintf("Promote %s to regression", strings.Join(eligible, ", ")),
	}
}

func buildPromotionMarkdown(proposal *PromotionProposal) string {
	var b strings.Builder
	fmt.Fprintln(&b, "# Promotion Proposal")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "- Suite: `%s`\n", proposal.Suite)
	fmt.Fprintf(&b, "- Profile: `%s`\n", proposal.Profile)
	fmt.Fprintf(&b, "- Executed Lane: `%s`\n", proposal.Lane)
	fmt.Fprintf(&b, "- Destination Lane: `%s`\n", proposal.DestinationLane)
	fmt.Fprintf(&b, "- Suite File: `%s`\n", proposal.SuiteFile)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Eligible Steps")
	fmt.Fprintln(&b)
	for _, step := range proposal.EligibleSteps {
		fmt.Fprintf(&b, "- `%s`\n", step)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Apply")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "```bash")
	fmt.Fprintln(&b, proposal.SuggestedCommand)
	fmt.Fprintln(&b, "```")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Suggested Git Commands")
	fmt.Fprintln(&b)
	for _, command := range proposal.SuggestedGitCommands {
		fmt.Fprintf(&b, "- `%s`\n", command)
	}
	return b.String()
}

func writePIDMarker(path string) error {
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644)
}

func removableRunDir(path string) bool {
	pidPath := filepath.Join(path, runPIDFile)
	data, err := os.ReadFile(pidPath)
	if err == nil {
		pid, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
		if parseErr == nil && processRunning(pid) {
			return false
		}
	}
	return true
}

func processRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}

func symlinkTarget(path string) (string, bool) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return "", false
	}
	target, err := os.Readlink(path)
	if err != nil {
		return "", false
	}
	return target, true
}

func replaceShortTempAlias(alias string, target string) error {
	if err := os.Remove(alias); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Symlink(target, alias)
}

func collectToolVersions() (string, error) {
	commands := [][]string{
		{"git", "--version"},
		{"go", "version"},
		{"protoc", "--version"},
		{"cargo", "--version"},
		{"rustc", "--version"},
		{"swift", "--version"},
		{"dotnet", "--version"},
		{"node", "--version"},
		{"npm", "--version"},
		{"ruby", "--version"},
		{"bundle", "--version"},
		{"python3", "--version"},
		{"java", "-version"},
		{"gradle", "--version"},
		{"dart", "--version"},
		{"cmake", "--version"},
		{"make", "--version"},
	}
	var b strings.Builder
	fmt.Fprintf(&b, "os=%s\narch=%s\n", runtime.GOOS, runtime.GOARCH)
	for _, command := range commands {
		if _, err := exec.LookPath(command[0]); err != nil {
			continue
		}
		cmd := exec.Command(command[0], command[1:]...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Fprintf(&b, "%s: %s", strings.Join(command, " "), strings.TrimSpace(string(output)))
			if !strings.HasSuffix(b.String(), "\n") {
				b.WriteString("\n")
			}
			continue
		}
		fmt.Fprintf(&b, "%s: %s\n", command[0], strings.TrimSpace(firstLine(string(output))))
	}
	return b.String(), nil
}

func extractTar(dst string, source io.Reader) error {
	tr := tar.NewReader(source)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dst, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, fs.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(header.Linkname, target); err != nil && !os.IsExist(err) {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fs.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(file, tr); err != nil {
				file.Close()
				return err
			}
			if err := file.Close(); err != nil {
				return err
			}
		}
	}
}

func copyWorkspaceTree(srcRoot string, dstRoot string, configRelDir string, excludeRoots ...string) error {
	return filepath.WalkDir(srcRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == srcRoot {
			return nil
		}
		for _, excluded := range excludeRoots {
			if excluded == "" {
				continue
			}
			if path == excluded || strings.HasPrefix(path, excluded+string(os.PathSeparator)) {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		if shouldSkipWorkspacePath(rel, entry, configRelDir) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dstRoot, rel)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			return os.Symlink(linkTarget, target)
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target, info.Mode())
	})
}

func shouldSkipWorkspacePath(rel string, entry fs.DirEntry, configRelDir string) bool {
	normalized := strings.TrimPrefix(filepath.ToSlash(rel), "./")
	switch normalized {
	case ".git":
		return true
	}
	baseConfigDir := strings.TrimPrefix(filepath.ToSlash(configRelDir), "./")
	for _, suffix := range []string{".artifacts", "reports", "archives", ".t"} {
		if pathHasSegment(normalized, suffix) {
			return true
		}
		target := suffix
		if baseConfigDir != "" && baseConfigDir != "." {
			target = baseConfigDir + "/" + suffix
		}
		if normalized == target {
			return true
		}
	}
	base := entry.Name()
	switch base {
	case ".git", ".gradle", ".kotlin", ".build", "build", "target", "obj", "__pycache__", "node_modules":
		return true
	}
	return false
}

func pathHasSegment(path string, segment string) bool {
	if path == segment {
		return true
	}
	for _, part := range strings.Split(path, "/") {
		if part == segment {
			return true
		}
	}
	return false
}

func copyTree(srcRoot string, dstRoot string) error {
	return filepath.WalkDir(srcRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dstRoot, 0o755)
		}
		target := filepath.Join(dstRoot, rel)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			return os.Symlink(linkTarget, target)
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src string, dst string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func firstLine(input string) string {
	parts := strings.Split(strings.TrimSpace(input), "\n")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func emptyAsNone(value string) string {
	if strings.TrimSpace(value) == "" {
		return "<none>"
	}
	return value
}

func printProgress(progress io.Writer, format string, args ...any) {
	if progress == nil {
		return
	}
	fmt.Fprintf(progress, format, args...)
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func newHistoryID(reportsDir string, suite string, source string, profile string, now time.Time) (string, error) {
	base := fmt.Sprintf("%s_%s_%s-%s",
		sanitizeHistoryToken(suite),
		sanitizeHistoryToken(source),
		sanitizeHistoryToken(profile),
		now.Format("20060102_15_04_05"),
	)
	for index := 1; index <= 9999; index++ {
		candidate := fmt.Sprintf("%s_%04d", base, index)
		candidateDir := filepath.Join(reportsDir, candidate)
		if err := os.Mkdir(candidateDir, 0o755); err == nil {
			return candidate, nil
		} else if os.IsExist(err) {
			continue
		} else {
			return "", err
		}
	}
	return "", fmt.Errorf("could not allocate history id for %s", base)
}

func sanitizeHistoryToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "unknown"
	}
	var b strings.Builder
	lastHyphen := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastHyphen = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastHyphen = false
		case r == '-' || r == '_':
			b.WriteRune(r)
			lastHyphen = false
		default:
			if !lastHyphen {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	out := strings.Trim(b.String(), "-_")
	if out == "" {
		return "unknown"
	}
	return out
}

func historyTimestamp(item HistoryEntry) time.Time {
	if ts, err := time.Parse(time.RFC3339, strings.TrimSpace(item.StartedAt)); err == nil {
		return ts
	}
	return time.Time{}
}

func yamlList(values []string, indent int) string {
	if len(values) == 0 {
		return strings.Repeat(" ", indent) + "[]"
	}
	prefix := strings.Repeat(" ", indent)
	lines := make([]string, 0, len(values))
	for _, value := range values {
		lines = append(lines, prefix+"- "+value)
	}
	return strings.Join(lines, "\n")
}
