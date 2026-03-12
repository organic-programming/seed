package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type TargetKind string

const (
	assemblyTarget    TargetKind = "assembly"
	compositionTarget TargetKind = "composition"
)

type ResultStatus string

const (
	statusDryRun      ResultStatus = "dry-run"
	statusSkipped     ResultStatus = "skipped"
	statusPassed      ResultStatus = "passed"
	statusSmokePassed ResultStatus = "smoke-passed"
	statusBuildFailed ResultStatus = "build-failed"
	statusRunFailed   ResultStatus = "run-failed"
	statusTimedOut    ResultStatus = "timed-out"
)

type MatrixConfig struct {
	RepoRoot   string
	OpPath     string
	FilterExpr string
	Filter     *regexp.Regexp
	SkipExpr   string
	Skip       *regexp.Regexp
	Timeout    time.Duration
	Format     string
	DryRun     bool
}

type RuntimeEnv struct {
	GOOS     string
	LookPath func(string) (string, error)
}

type manifestLite struct {
	FamilyName string   `yaml:"family_name,omitempty"`
	Kind       string   `yaml:"kind"`
	Transport  string   `yaml:"transport,omitempty"`
	Platforms  []string `yaml:"platforms,omitempty"`
	Requires   struct {
		Commands []string `yaml:"commands,omitempty"`
	} `yaml:"requires,omitempty"`
}

type Target struct {
	Name             string     `json:"name"`
	Path             string     `json:"path"`
	ManifestPath     string     `json:"manifest_path"`
	FamilyName       string     `json:"family_name,omitempty"`
	DisplayFamily    string     `json:"display_family,omitempty"`
	DisplayName      string     `json:"display_name,omitempty"`
	Kind             TargetKind `json:"kind"`
	Transport        string     `json:"transport,omitempty"`
	Platforms        []string   `json:"platforms,omitempty"`
	RequiresCommands []string   `json:"requires_commands,omitempty"`
}

type MatrixReport struct {
	GeneratedAt string         `json:"generated_at"`
	RepoRoot    string         `json:"repo_root"`
	Filter      string         `json:"filter,omitempty"`
	Skip        string         `json:"skip,omitempty"`
	Timeout     string         `json:"timeout"`
	DryRun      bool           `json:"dry_run"`
	Summary     MatrixSummary  `json:"summary"`
	Results     []TargetResult `json:"results"`
}

type MatrixSummary struct {
	Discovered   int `json:"discovered"`
	Selected     int `json:"selected"`
	Assemblies   int `json:"assemblies"`
	Compositions int `json:"compositions"`
	DryRun       int `json:"dry_run"`
	Skipped      int `json:"skipped"`
	Passed       int `json:"passed"`
	SmokePassed  int `json:"smoke_passed"`
	BuildFailed  int `json:"build_failed"`
	RunFailed    int `json:"run_failed"`
	TimedOut     int `json:"timed_out"`
}

type TargetResult struct {
	Name          string       `json:"name"`
	Path          string       `json:"path"`
	FamilyName    string       `json:"family_name,omitempty"`
	DisplayFamily string       `json:"display_family,omitempty"`
	DisplayName   string       `json:"display_name,omitempty"`
	Kind          TargetKind   `json:"kind"`
	Status        ResultStatus `json:"status"`
	Validation    string       `json:"validation"`
	Reason        string       `json:"reason,omitempty"`
	Transport     string       `json:"transport,omitempty"`
	Platforms     []string     `json:"platforms,omitempty"`
	Requires      []string     `json:"requires,omitempty"`
	BuildMS       int64        `json:"build_ms,omitempty"`
	RunMS         int64        `json:"run_ms,omitempty"`
	BuildCode     *int         `json:"build_code,omitempty"`
	RunCode       *int         `json:"run_code,omitempty"`
	ExpectedJSON  string       `json:"expected_json,omitempty"`
	ActualJSON    string       `json:"actual_json,omitempty"`
	BuildOutput   string       `json:"build_output,omitempty"`
	RunOutput     string       `json:"run_output,omitempty"`
}

func (r MatrixReport) HasFailures() bool {
	return r.Summary.BuildFailed > 0 || r.Summary.RunFailed > 0 || r.Summary.TimedOut > 0
}

type commandRunner interface {
	Run(ctx context.Context, dir string, name string, args ...string) commandResult
}

type commandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
	Elapsed  time.Duration
	TimedOut bool
}

type shellRunner struct{}

func (shellRunner) Run(ctx context.Context, dir string, name string, args ...string) commandResult {
	cmd := exec.CommandContext(ctx, shellPath(), "-lc", shellCommandInDir(dir, name, args...))
	cmd.Env = append(os.Environ(), "PWD="+dir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start)

	exitCode := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exitCode = exitErr.ExitCode()
	}

	timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)
	if timedOut && exitCode == 0 {
		exitCode = -1
	}

	return commandResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Err:      err,
		Elapsed:  elapsed,
		TimedOut: timedOut,
	}
}

func shellCommandInDir(dir, name string, args ...string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, shellQuote(name))
	for _, arg := range args {
		parts = append(parts, shellQuote(arg))
	}
	return "cd " + shellQuote(dir) + " && exec " + strings.Join(parts, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func shellPath() string {
	if shell := strings.TrimSpace(os.Getenv("SHELL")); shell != "" {
		return shell
	}
	return "/bin/sh"
}

func defaultRuntimeEnv() RuntimeEnv {
	return RuntimeEnv{
		GOOS:     runtime.GOOS,
		LookPath: exec.LookPath,
	}
}

func runMatrix(ctx context.Context, cfg MatrixConfig, runner commandRunner, env RuntimeEnv) (MatrixReport, error) {
	targets, err := discoverTargets(cfg.RepoRoot)
	if err != nil {
		return MatrixReport{}, err
	}
	return executeTargets(ctx, cfg, targets, runner, env), nil
}

func executeTargets(ctx context.Context, cfg MatrixConfig, targets []Target, runner commandRunner, env RuntimeEnv) MatrixReport {
	report := MatrixReport{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		RepoRoot:    cfg.RepoRoot,
		Filter:      cfg.FilterExpr,
		Skip:        cfg.SkipExpr,
		Timeout:     cfg.Timeout.String(),
		DryRun:      cfg.DryRun,
		Summary: MatrixSummary{
			Discovered: len(targets),
		},
	}

	for _, target := range targets {
		if !matchesPattern(cfg.Filter, target) {
			continue
		}
		report.Summary.Selected++
		if target.Kind == assemblyTarget {
			report.Summary.Assemblies++
		} else {
			report.Summary.Compositions++
		}

		displayFamily := target.DisplayFamily
		if strings.TrimSpace(displayFamily) == "" {
			displayFamily = targetDisplayFamily(target.Kind, target.FamilyName, target.Name)
		}
		displayName := target.DisplayName
		if strings.TrimSpace(displayName) == "" {
			displayName = targetDisplayName(target.Kind, displayFamily)
		}

		result := TargetResult{
			Name:          target.Name,
			Path:          target.Path,
			FamilyName:    target.FamilyName,
			DisplayFamily: displayFamily,
			DisplayName:   displayName,
			Kind:          target.Kind,
			Transport:     target.Transport,
			Platforms:     append([]string(nil), target.Platforms...),
			Requires:      append([]string(nil), target.RequiresCommands...),
			Validation:    validationMode(target.Kind),
		}

		switch {
		case cfg.Skip != nil && matchesPattern(cfg.Skip, target):
			result.Status = statusSkipped
			result.Reason = "matched --skip"
		case skipReason(target, env) != "":
			result.Status = statusSkipped
			result.Reason = skipReason(target, env)
		case cfg.DryRun:
			result.Status = statusDryRun
		default:
			result = executeTarget(ctx, cfg, target, runner, result)
		}

		report.Results = append(report.Results, result)
	}

	report.Summary = summarize(report.Results, report.Summary)
	return report
}

func discoverTargets(repoRoot string) ([]Target, error) {
	var targets []Target

	assemblyPaths, err := filepath.Glob(filepath.Join(repoRoot, "recipes", "assemblies", "*", "holon.yaml"))
	if err != nil {
		return nil, err
	}
	for _, manifestPath := range assemblyPaths {
		target, err := loadTarget(repoRoot, manifestPath, assemblyTarget)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}

	compositionPaths, err := filepath.Glob(filepath.Join(repoRoot, "recipes", "composition", "*", "*", "holon.yaml"))
	if err != nil {
		return nil, err
	}
	for _, manifestPath := range compositionPaths {
		rel, err := filepath.Rel(repoRoot, manifestPath)
		if err != nil {
			return nil, err
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) != 5 || parts[2] == "workers" {
			continue
		}
		target, err := loadTarget(repoRoot, manifestPath, compositionTarget)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}

	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Kind != targets[j].Kind {
			return targets[i].Kind < targets[j].Kind
		}
		return targets[i].Path < targets[j].Path
	})
	return targets, nil
}

func loadTarget(repoRoot, manifestPath string, kind TargetKind) (Target, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return Target{}, fmt.Errorf("read %s: %w", manifestPath, err)
	}

	var manifest manifestLite
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return Target{}, fmt.Errorf("parse %s: %w", manifestPath, err)
	}

	dir := filepath.Dir(manifestPath)
	relDir, err := filepath.Rel(repoRoot, dir)
	if err != nil {
		return Target{}, err
	}
	relManifest, err := filepath.Rel(repoRoot, manifestPath)
	if err != nil {
		return Target{}, err
	}

	displayFamily := targetDisplayFamily(kind, manifest.FamilyName, filepath.Base(dir))

	return Target{
		Name:             filepath.Base(dir),
		Path:             filepath.ToSlash(relDir),
		ManifestPath:     filepath.ToSlash(relManifest),
		FamilyName:       manifest.FamilyName,
		DisplayFamily:    displayFamily,
		DisplayName:      targetDisplayName(kind, displayFamily),
		Kind:             kind,
		Transport:        manifest.Transport,
		Platforms:        append([]string(nil), manifest.Platforms...),
		RequiresCommands: append([]string(nil), manifest.Requires.Commands...),
	}, nil
}

func locateRepoRootFrom(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}

	for {
		candidate := filepath.Join(dir, "design", "grace-op", "v0.4", "recipes.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find repository root from %s", start)
		}
		dir = parent
	}
}

func matchesPattern(pattern *regexp.Regexp, target Target) bool {
	if pattern == nil {
		return true
	}
	displayFamily := target.DisplayFamily
	if strings.TrimSpace(displayFamily) == "" {
		displayFamily = targetDisplayFamily(target.Kind, target.FamilyName, target.Name)
	}
	displayName := target.DisplayName
	if strings.TrimSpace(displayName) == "" {
		displayName = targetDisplayName(target.Kind, displayFamily)
	}
	haystack := strings.Join([]string{
		target.Name,
		target.Path,
		target.ManifestPath,
		target.FamilyName,
		displayFamily,
		displayName,
		target.Transport,
	}, " ")
	return pattern.MatchString(haystack)
}

func skipReason(target Target, env RuntimeEnv) string {
	if !supportsPlatform(target.Platforms, env.GOOS) {
		return fmt.Sprintf("unsupported platform: %s", canonicalPlatform(env.GOOS))
	}

	var missing []string
	for _, command := range target.RequiresCommands {
		if command == "" {
			continue
		}
		if _, err := env.LookPath(command); err != nil {
			missing = append(missing, command)
		}
	}
	if len(missing) > 0 {
		return "missing commands: " + strings.Join(missing, ", ")
	}
	return ""
}

func supportsPlatform(platforms []string, goos string) bool {
	if len(platforms) == 0 {
		return true
	}
	current := canonicalPlatform(goos)
	for _, platform := range platforms {
		if strings.EqualFold(platform, current) {
			return true
		}
	}
	return false
}

func canonicalPlatform(goos string) string {
	switch strings.ToLower(strings.TrimSpace(goos)) {
	case "darwin":
		return "macos"
	default:
		return strings.ToLower(strings.TrimSpace(goos))
	}
}

func executeTarget(ctx context.Context, cfg MatrixConfig, target Target, runner commandRunner, result TargetResult) TargetResult {
	opPath := cfg.OpPath
	if strings.TrimSpace(opPath) == "" {
		opPath = "op"
	}
	targetArg := filepath.Join(cfg.RepoRoot, filepath.FromSlash(target.Path))

	buildResult := runner.Run(ctx, cfg.RepoRoot, opPath, "-f", "json", "build", targetArg)
	result.BuildMS = buildResult.Elapsed.Milliseconds()
	result.BuildCode = intPtr(buildResult.ExitCode)
	if buildResult.Err != nil {
		result.Status = statusBuildFailed
		result.Reason = buildFailureReason(buildResult)
		result.BuildOutput = collapseOutput(buildResult.Stdout, buildResult.Stderr)
		return result
	}

	runCtx := ctx
	cancel := func() {}
	if cfg.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, cfg.Timeout)
	}
	defer cancel()

	runResult := runner.Run(runCtx, cfg.RepoRoot, opPath, "-f", "json", "run", "--no-build", targetArg)
	result.RunMS = runResult.Elapsed.Milliseconds()
	result.RunCode = intPtr(runResult.ExitCode)

	if target.Kind == assemblyTarget {
		result = finalizeAssemblyRun(result, runResult, cfg.Timeout)
	} else {
		result = finalizeCompositionRun(result, target, runResult, cfg.Timeout)
	}
	return result
}

func finalizeAssemblyRun(result TargetResult, runResult commandResult, timeout time.Duration) TargetResult {
	if runResult.TimedOut {
		result.Status = statusSmokePassed
		result.Reason = fmt.Sprintf("launch exceeded %s and was treated as a successful smoke test", timeout)
		return result
	}
	if runResult.Err != nil {
		result.Status = statusRunFailed
		result.Reason = buildFailureReason(runResult)
		result.RunOutput = collapseOutput(runResult.Stdout, runResult.Stderr)
		return result
	}
	result.Status = statusSmokePassed
	result.Reason = "launch smoke exited cleanly"
	return result
}

func finalizeCompositionRun(result TargetResult, target Target, runResult commandResult, timeout time.Duration) TargetResult {
	if runResult.TimedOut {
		result.Status = statusTimedOut
		result.Reason = fmt.Sprintf("run exceeded %s", timeout)
		result.RunOutput = collapseOutput(runResult.Stdout, runResult.Stderr)
		return result
	}
	if runResult.Err != nil {
		result.Status = statusRunFailed
		result.Reason = buildFailureReason(runResult)
		result.RunOutput = collapseOutput(runResult.Stdout, runResult.Stderr)
		return result
	}

	expected, ok := expectedCompositionJSON(target.Name)
	if !ok {
		result.Status = statusRunFailed
		result.Reason = "no expected JSON registered for composition"
		result.RunOutput = collapseOutput(runResult.Stdout, runResult.Stderr)
		return result
	}

	actual := strings.TrimSpace(runResult.Stdout)
	result.ExpectedJSON = expected
	result.ActualJSON = actual
	if !jsonEquivalent(expected, actual) {
		result.Status = statusRunFailed
		result.Reason = "unexpected JSON output"
		result.RunOutput = collapseOutput(runResult.Stdout, runResult.Stderr)
		return result
	}

	if stderr := strings.TrimSpace(runResult.Stderr); stderr != "" {
		result.RunOutput = clipTail(stderr)
	}
	result.Status = statusPassed
	return result
}

func expectedCompositionJSON(name string) (string, bool) {
	switch {
	case strings.Contains(name, "charon-direct-"):
		return `{"pattern":"direct-call","input":42,"result":1764}`, true
	case strings.Contains(name, "charon-pipeline-"):
		return `{"pattern":"pipeline","input":5,"computed":25,"transformed":"52"}`, true
	case strings.Contains(name, "charon-fanout-"):
		return `{"pattern":"fan-out","compute_input":5,"compute_result":25,"transform_input":"hello","transform_result":"olleh"}`, true
	default:
		return "", false
	}
}

func jsonEquivalent(expected, actual string) bool {
	var expectedValue any
	if err := json.Unmarshal([]byte(expected), &expectedValue); err != nil {
		return false
	}
	var actualValue any
	if err := json.Unmarshal([]byte(actual), &actualValue); err != nil {
		return false
	}
	return reflect.DeepEqual(expectedValue, actualValue)
}

func summarize(results []TargetResult, summary MatrixSummary) MatrixSummary {
	for _, result := range results {
		switch result.Status {
		case statusDryRun:
			summary.DryRun++
		case statusSkipped:
			summary.Skipped++
		case statusPassed:
			summary.Passed++
		case statusSmokePassed:
			summary.SmokePassed++
		case statusBuildFailed:
			summary.BuildFailed++
		case statusRunFailed:
			summary.RunFailed++
		case statusTimedOut:
			summary.TimedOut++
		}
	}
	return summary
}

func renderReport(report MatrixReport, format string) string {
	if format == "json" {
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return "{}\n"
		}
		return string(out) + "\n"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Matrix summary: %d selected from %d discovered\n", report.Summary.Selected, report.Summary.Discovered)
	fmt.Fprintf(&b, "passed=%d smoke-passed=%d skipped=%d dry-run=%d build-failed=%d run-failed=%d timed-out=%d\n",
		report.Summary.Passed,
		report.Summary.SmokePassed,
		report.Summary.Skipped,
		report.Summary.DryRun,
		report.Summary.BuildFailed,
		report.Summary.RunFailed,
		report.Summary.TimedOut,
	)
	fmt.Fprintf(&b, "%-13s %-46s %-11s %-9s %s\n", "STATUS", "DISPLAY NAME", "KIND", "TRANSPORT", "TARGET")
	for _, result := range report.Results {
		fmt.Fprintf(
			&b,
			"%-13s %-46s %-11s %-9s %s",
			result.Status,
			displayNameForResult(result),
			result.Kind,
			displayTransport(result.Transport),
			result.Path,
		)
		if result.Reason != "" {
			fmt.Fprintf(&b, "  %s", result.Reason)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func validationMode(kind TargetKind) string {
	if kind == assemblyTarget {
		return "launch-smoke"
	}
	return "json-output"
}

func buildFailureReason(result commandResult) string {
	if result.TimedOut {
		return "command timed out"
	}
	if result.Err == nil {
		return ""
	}
	if result.ExitCode != 0 {
		return fmt.Sprintf("exit code %d", result.ExitCode)
	}
	return result.Err.Error()
}

func collapseOutput(stdout, stderr string) string {
	var parts []string
	if trimmed := strings.TrimSpace(stdout); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if trimmed := strings.TrimSpace(stderr); trimmed != "" {
		parts = append(parts, trimmed)
	}
	return clipTail(strings.Join(parts, "\n"))
}

func clipTail(text string) string {
	const maxChars = 4000
	trimmed := strings.TrimSpace(text)
	if len(trimmed) <= maxChars {
		return trimmed
	}
	return "..." + trimmed[len(trimmed)-maxChars:]
}

func intPtr(value int) *int {
	return &value
}

func targetDisplayFamily(kind TargetKind, familyName, fallbackName string) string {
	family := strings.TrimSpace(familyName)
	if family == "" {
		family = strings.TrimSpace(fallbackName)
	}
	if family == "" || kind != assemblyTarget {
		return family
	}

	label := assemblyHostUILabel(family)
	if label == "" || strings.Contains(family, "("+label+")") {
		return family
	}
	return family + " (" + label + ")"
}

func targetDisplayName(kind TargetKind, displayFamily string) string {
	trimmed := strings.TrimSpace(displayFamily)
	if trimmed == "" {
		return ""
	}
	if kind != assemblyTarget || strings.HasPrefix(trimmed, "Gudule ") {
		return trimmed
	}
	return "Gudule " + trimmed
}

func assemblyHostUILabel(family string) string {
	parts := strings.Split(strings.TrimSpace(family), "-")
	if len(parts) < 2 {
		return ""
	}

	hostUI := ""
	if strings.EqualFold(parts[len(parts)-1], "web") {
		hostUI = "web"
	} else if len(parts) >= 3 {
		hostUI = strings.ToLower(parts[1])
	}

	switch hostUI {
	case "compose":
		return "Kotlin UI"
	case "flutter":
		return "Flutter UI"
	case "swiftui":
		return "SwiftUI"
	case "dotnet":
		return ".NET UI"
	case "qt":
		return "Qt UI"
	case "web":
		return "Web UI"
	default:
		return ""
	}
}

func displayNameForResult(result TargetResult) string {
	if trimmed := strings.TrimSpace(result.DisplayName); trimmed != "" {
		return trimmed
	}
	if trimmed := strings.TrimSpace(result.DisplayFamily); trimmed != "" {
		return trimmed
	}
	if trimmed := strings.TrimSpace(result.FamilyName); trimmed != "" {
		return trimmed
	}
	return result.Name
}

func displayTransport(transport string) string {
	if trimmed := strings.TrimSpace(transport); trimmed != "" {
		return trimmed
	}
	return "-"
}
