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
	if err := validateRunOptions(opts); err != nil {
		return nil, err
	}
	runtimeCfg, err := loadRunConfig(opts.ConfigDir, opts.Suite)
	if err != nil {
		return nil, err
	}
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

	now := time.Now().UTC()
	runID := fmt.Sprintf("%s-%d", now.Format("20060102T150405.000000000Z"), os.Getpid())
	tmpStore := filepath.Join(os.TempDir(), tempStorePrefix+runID)
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

	commitHash, branch, dirty := gitMetadata(repoRoot)
	profile := normalizeProfile(opts.Profile)
	source := normalizeSource(firstNonEmpty(opts.Source, runtimeCfg.Root.Defaults.Source))
	lane := normalizeLane(firstNonEmpty(opts.Lane, runtimeCfg.Root.Defaults.Lane))
	archivePolicy := normalizeArchivePolicy(firstNonEmpty(opts.ArchivePolicy, runtimeCfg.Root.Defaults.ArchivePolicy[profile]))

	switch source {
	case "committed":
		if err := snapshotCommitted(ctx, repoRoot, snapshotRoot); err != nil {
			return nil, err
		}
	case "workspace":
		if err := snapshotWorkspace(repoRoot, snapshotRoot, paths, runtimeCfg.ConfigRelDir); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported source %q", source)
	}
	if err := normalizeSnapshot(snapshotRoot); err != nil {
		return nil, err
	}
	if err := replaceShortTempAlias(paths.ShortTempAlias, tmpStore); err != nil {
		return nil, err
	}

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

	manifest := RunManifest{
		ConfigDir:     runtimeCfg.ConfigDir,
		Suite:         runtimeCfg.SuiteName,
		RunID:         runID,
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
		StartedAt:     now.Format(time.RFC3339),
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
			Command:     step.Command,
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
			printProgress(progress, "[%02d/%02d] SKIP %s (workdir missing from snapshot)\n", index+1, len(steps), step.ID)
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
			printProgress(progress, "[%02d/%02d] SKIP %s (%s)\n", index+1, len(steps), step.ID, result.Reason)
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
			printProgress(progress, "[%02d/%02d] SKIP %s (%s)\n", index+1, len(steps), step.ID, reason)
			continue
		}

		printProgress(progress, "[%02d/%02d] RUN  %s\n", index+1, len(steps), step.ID)
		start := time.Now().UTC()
		result.StartedAt = start.Format(time.RFC3339)
		code, err := runStepCommand(ctx, step, env, logPath, progress)
		end := time.Now().UTC()
		result.FinishedAt = end.Format(time.RFC3339)
		result.DurationSeconds = int64(end.Sub(start).Seconds())
		if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
			result.Reason = err.Error()
		}
		if code == 0 {
			result.Status = "PASS"
			manifest.PassCount++
			printProgress(progress, "[%02d/%02d] PASS %s (%ds)\n", index+1, len(steps), step.ID, result.DurationSeconds)
		} else {
			result.Status = "FAIL"
			manifest.FailCount++
			printProgress(progress, "[%02d/%02d] FAIL %s (%ds)\n", index+1, len(steps), step.ID, result.DurationSeconds)
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
	}
	if proposal := buildPromotionProposal(runtimeCfg, result); proposal != nil {
		result.Promotion = proposal
	}
	if err := writeReport(result, reportDir, toolVersions); err != nil {
		return nil, err
	}

	if shouldArchive(profile, archivePolicy) {
		archivePath, err := archiveReportDir(paths, manifest, reportDir)
		if err != nil {
			return result, err
		}
		result.Manifest.ArchivePath = archivePath
		if err := writeReport(result, reportDir, toolVersions); err != nil {
			return nil, err
		}
		if !opts.KeepReport {
			_ = os.RemoveAll(reportDir)
		}
	}

	if opts.KeepSnapshot {
		_ = os.RemoveAll(preservedRunDir)
		if err := os.MkdirAll(filepath.Dir(preservedRunDir), 0o755); err != nil {
			return nil, err
		}
		if err := copyTree(runArtifactsDir, preservedRunDir); err != nil {
			return nil, err
		}
		_ = os.Remove(filepath.Join(preservedRunDir, runPIDFile))
		result.Manifest.SnapshotRoot = filepath.Join(preservedRunDir, "snapshot")
		if err := writeReport(result, reportDir, toolVersions); err != nil {
			return nil, err
		}
	}
	_ = os.RemoveAll(tmpStore)

	return result, nil
}

func Archive(ctx context.Context, opts ArchiveOptions) (*RunResult, error) {
	runtimeCfg, err := loadRepoConfig(opts.ConfigDir)
	if err != nil {
		return nil, err
	}
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

func ListRuns(_ context.Context, configDir string) ([]RunSummary, error) {
	runtimeCfg, err := loadRepoConfig(configDir)
	if err != nil {
		return nil, err
	}
	paths := runtimeCfg.Paths
	items := map[string]RunSummary{}
	reportEntries, _ := os.ReadDir(paths.ReportsDir)
	for _, entry := range reportEntries {
		if !entry.IsDir() {
			continue
		}
		manifest, err := readManifestFile(filepath.Join(paths.ReportsDir, entry.Name(), reportManifestName))
		if err != nil {
			continue
		}
		items[manifest.RunID] = summaryFromManifest(manifest)
	}
	archivePaths, _ := findArchivePaths(paths.ArchivesDir)
	for _, archivePath := range archivePaths {
		manifest, _, _, err := readArchivePayload(archivePath)
		if err != nil {
			continue
		}
		summary := summaryFromManifest(manifest)
		summary.ArchivePath = archivePath
		if existing, ok := items[summary.RunID]; ok {
			if existing.ReportDir != "" {
				existing.ArchivePath = archivePath
				items[summary.RunID] = existing
				continue
			}
		}
		items[summary.RunID] = summary
	}
	out := make([]RunSummary, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].RunID > out[j].RunID
	})
	return out, nil
}

func ShowRun(_ context.Context, configDir string, runID string) (*RunResult, error) {
	runtimeCfg, err := loadRepoConfig(configDir)
	if err != nil {
		return nil, err
	}
	paths := runtimeCfg.Paths
	reportDir := filepath.Join(paths.ReportsDir, runID)
	if dirExists(reportDir) {
		return loadRunFromReportDir(reportDir)
	}
	archivePaths, _ := findArchivePaths(paths.ArchivesDir)
	for _, archivePath := range archivePaths {
		manifest, steps, summaryMD, err := readArchivePayload(archivePath)
		if err != nil || manifest.RunID != runID {
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
	return nil, fmt.Errorf("run %q not found", runID)
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

func runStepCommand(ctx context.Context, step StepSpec, env []string, logPath string, progress io.Writer) (int, error) {
	logFile, err := os.Create(logPath)
	if err != nil {
		return 1, err
	}
	defer logFile.Close()
	writer := io.Writer(logFile)
	if progress != nil {
		writer = io.MultiWriter(logFile, progress)
	}
	cmd := exec.CommandContext(ctx, "bash", "-lc", step.Command)
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

func buildSummaryMarkdown(manifest RunManifest, steps []StepResult) string {
	var b strings.Builder
	fmt.Fprintln(&b, "# Verification Report")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "- Config Dir: `%s`\n", manifest.ConfigDir)
	fmt.Fprintf(&b, "- Suite: `%s`\n", manifest.Suite)
	fmt.Fprintf(&b, "- Run ID: `%s`\n", manifest.RunID)
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

func archiveReportDir(paths repoPaths, manifest RunManifest, reportDir string) (string, error) {
	commitDir := filepath.Join(paths.ArchivesDir, manifest.CommitHash)
	if err := os.MkdirAll(commitDir, 0o755); err != nil {
		return "", err
	}
	suffix := ""
	if manifest.Source == "workspace" && manifest.Dirty {
		suffix = "-dirty"
	}
	archivePath := filepath.Join(commitDir, fmt.Sprintf("%s-%s%s.tar.gz", manifest.RunID, manifest.Profile, suffix))
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
	runID := strings.TrimSpace(opts.RunID)
	if runID == "" {
		return nil, "", fmt.Errorf("archive requires --run or --latest")
	}
	reportDir := filepath.Join(paths.ReportsDir, runID)
	if !dirExists(reportDir) {
		if archivePath, ok := findArchiveByRunID(paths.ArchivesDir, runID); ok {
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
		return nil, "", fmt.Errorf("run %q not found", runID)
	}
	result, err := loadRunFromReportDir(reportDir)
	return result, reportDir, err
}

func latestReportDir(reportsDir string) (string, error) {
	entries, err := os.ReadDir(reportsDir)
	if err != nil {
		return "", err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	if len(names) == 0 {
		return "", fmt.Errorf("no extracted runs found under %s", reportsDir)
	}
	sort.Strings(names)
	return filepath.Join(reportsDir, names[len(names)-1]), nil
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

func readManifestFile(path string) (RunManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RunManifest{}, err
	}
	var manifest RunManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return RunManifest{}, err
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

func findArchiveByRunID(archivesDir string, runID string) (string, bool) {
	paths, err := findArchivePaths(archivesDir)
	if err != nil {
		return "", false
	}
	for _, path := range paths {
		if strings.Contains(filepath.Base(path), runID+"-") {
			return path, true
		}
	}
	return "", false
}

func readArchivePayload(path string) (RunManifest, []StepResult, string, error) {
	manifestData, err := readArchiveFile(path, reportManifestName)
	if err != nil {
		return RunManifest{}, nil, "", err
	}
	stepsData, err := readArchiveFile(path, reportStepsName)
	if err != nil {
		return RunManifest{}, nil, "", err
	}
	summaryMD, err := readArchiveFile(path, reportSummaryMD)
	if err != nil {
		return RunManifest{}, nil, "", err
	}
	var manifest RunManifest
	if err := json.Unmarshal([]byte(manifestData), &manifest); err != nil {
		return RunManifest{}, nil, "", err
	}
	var steps []StepResult
	if err := json.Unmarshal([]byte(stepsData), &steps); err != nil {
		return RunManifest{}, nil, "", err
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

func summaryFromManifest(manifest RunManifest) RunSummary {
	return RunSummary{
		RunID:       manifest.RunID,
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
	progression := make([]string, 0)
	resultByID := make(map[string]StepResult, len(result.Steps))
	for _, step := range result.Steps {
		resultByID[step.StepID] = step
		if step.Lane == "progression" {
			progression = append(progression, step.StepID)
		}
	}
	if len(progression) == 0 {
		return nil
	}
	for _, id := range progression {
		if resultByID[id].Status != "PASS" {
			return nil
		}
	}

	profileEntry, ok := cfg.Suite.Profiles[result.Manifest.Profile]
	if !ok {
		return nil
	}
	regressionIDs := append([]string(nil), profileEntry.Regression...)
	for _, id := range progression {
		if !containsString(regressionIDs, id) {
			regressionIDs = append(regressionIDs, id)
		}
	}
	remainingProgression := make([]string, 0, len(profileEntry.Progression))
	for _, id := range profileEntry.Progression {
		if !containsString(progression, id) {
			remainingProgression = append(remainingProgression, id)
		}
	}

	suggestedPatch := strings.TrimSpace(fmt.Sprintf(
		"profiles:\n  %s:\n    regression:\n%s\n    progression:\n%s\n",
		result.Manifest.Profile,
		yamlList(regressionIDs, 6),
		yamlList(remainingProgression, 6),
	))

	return &PromotionProposal{
		Suite:           result.Manifest.Suite,
		Profile:         result.Manifest.Profile,
		Lane:            result.Manifest.Lane,
		DestinationLane: "regression",
		SuiteFile:       cfg.SuitePath,
		EligibleSteps:   progression,
		SuggestedPatch:  suggestedPatch,
		SuggestedGitCommands: []string{
			fmt.Sprintf("git add %s", cfg.SuitePath),
			fmt.Sprintf("git commit -m %q", fmt.Sprintf("Promote %s %s progression checks", result.Manifest.Suite, result.Manifest.Profile)),
		},
		SuggestedCommitMessage: fmt.Sprintf("Promote %s %s progression checks", result.Manifest.Suite, result.Manifest.Profile),
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
	fmt.Fprintln(&b, "## Suggested Patch")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "```yaml")
	fmt.Fprintln(&b, proposal.SuggestedPatch)
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
	normalized := filepath.ToSlash(rel)
	switch normalized {
	case ".git":
		return true
	}
	baseConfigDir := strings.TrimPrefix(filepath.ToSlash(configRelDir), "./")
	for _, suffix := range []string{".artifacts", "reports", "archives", ".t"} {
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
