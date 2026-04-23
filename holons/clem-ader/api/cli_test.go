// These tests verify the CLI facet of clem-ader from config-dir parsing down to history, show, and completion output.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/organic-programming/clem-ader/internal/testrepo"
	"github.com/spf13/cobra"
)

func TestRunCLIVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := RunCLI([]string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("RunCLI(version) = %d, want 0", code)
	}
	if got := strings.TrimSpace(stdout.String()); !strings.HasPrefix(got, "ader ") {
		t.Fatalf("version output = %q, want prefix %q", got, "ader ")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunCLITestArchiveHistoryAndShow(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "ader", "catalogues", "fixture")
	withWorkingDir(t, root, func() {
		var stdout bytes.Buffer
		var stderr bytes.Buffer

		if code := RunCLI([]string{"test", configDir, "--suite", "fixture", "--profile", "integration", "--source", "workspace", "--archive", "never"}, &stdout, &stderr); code != 0 {
			t.Fatalf("RunCLI(test) = %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "summary: pass=") {
			t.Fatalf("test stdout missing summary: %q", stdout.String())
		}
		if !strings.Contains(stderr.String(), "RUN  integration-deterministic") {
			t.Fatalf("test stderr missing live RUN progress: %q", stderr.String())
		}
		if !strings.Contains(stderr.String(), "PASS integration-deterministic") {
			t.Fatalf("test stderr missing live PASS progress: %q", stderr.String())
		}

		stdout.Reset()
		stderr.Reset()
		if code := RunCLI([]string{"archive", configDir, "--latest"}, &stdout, &stderr); code != 0 {
			t.Fatalf("RunCLI(archive) = %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "archived ") {
			t.Fatalf("archive stdout missing archived marker: %q", stdout.String())
		}

		stdout.Reset()
		stderr.Reset()
		if code := RunCLI([]string{"history", configDir}, &stdout, &stderr); code != 0 {
			t.Fatalf("RunCLI(history) = %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
		}
		lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
		if len(lines) != 1 || lines[0] == "" {
			t.Fatalf("history stdout = %q, want exactly one history line", stdout.String())
		}
		historyID := strings.Split(lines[0], "\t")[0]

		stdout.Reset()
		stderr.Reset()
		if code := RunCLI([]string{"show", configDir, "--id", historyID}, &stdout, &stderr); code != 0 {
			t.Fatalf("RunCLI(show) = %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "# Ader Report") {
			t.Fatalf("show stdout missing report heading: %q", stdout.String())
		}
	})
}

func TestRunCLITestShowsPhaseFeedbackAndHeartbeat(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "ader", "catalogues", "fixture")
	withWorkingDir(t, root, func() {
		t.Setenv("ADER_TEST_SILENT_STEP_SLEEP", "6")
		var stdout bytes.Buffer
		var stderr bytes.Buffer

		if code := RunCLI([]string{"test", configDir, "--suite", "fixture", "--profile", "unit", "--lane", "progression", "--step-filter", "^quiet-script$", "--source", "workspace", "--archive", "never"}, &stdout, &stderr); code != 0 {
			t.Fatalf("RunCLI(test quiet-script) = %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "summary: pass=1 fail=0 skip=0") {
			t.Fatalf("stdout missing final summary: %q", stdout.String())
		}
		text := stderr.String()
		if !strings.Contains(text, "[phase] config loaded") {
			t.Fatalf("stderr missing config phase: %q", text)
		}
		if !strings.Contains(text, "[phase] snapshot workspace") {
			t.Fatalf("stderr missing snapshot phase: %q", text)
		}
		if !strings.Contains(text, "[phase] selected 1 steps for profile=unit lane=progression") {
			t.Fatalf("stderr missing selected-steps phase: %q", text)
		}
		if !strings.Contains(text, "[01/01] RUN  quiet-script") {
			t.Fatalf("stderr missing RUN line: %q", text)
		}
		if !strings.Contains(text, "[01/01] CMD holons/clem-ader :: scripts/quiet-step.sh") {
			t.Fatalf("stderr missing CMD line: %q", text)
		}
		if !strings.Contains(text, "[wait] quiet-script still running") || !strings.Contains(text, "no output yet") {
			t.Fatalf("stderr missing heartbeat: %q", text)
		}
		if !strings.Contains(text, "quiet-step:done") {
			t.Fatalf("stderr missing raw subprocess output: %q", text)
		}
		if strings.Index(text, "[phase] snapshot workspace") > strings.Index(text, "[01/01] RUN  quiet-script") {
			t.Fatalf("snapshot phase should appear before first step: %q", text)
		}
	})
}

func TestRunCLIPromoteAndDowngrade(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "ader", "catalogues", "fixture")
	withWorkingDir(t, root, func() {
		var stdout bytes.Buffer
		var stderr bytes.Buffer

		if code := RunCLI([]string{"promote", configDir, "--suite", "fixture", "--step", "fixture-script"}, &stdout, &stderr); code != 0 {
			t.Fatalf("RunCLI(promote) = %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "promoted: fixture-script") {
			t.Fatalf("promote stdout = %q, want promoted step summary", stdout.String())
		}

		stdout.Reset()
		stderr.Reset()
		if code := RunCLI([]string{"downgrade", configDir, "--suite", "fixture", "--step", "holons-grace-op-unit-root"}, &stdout, &stderr); code != 0 {
			t.Fatalf("RunCLI(downgrade) = %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "downgraded: holons-grace-op-unit-root") {
			t.Fatalf("downgrade stdout = %q, want downgraded step summary", stdout.String())
		}
		if stderr.Len() != 0 {
			t.Fatalf("stderr = %q, want empty", stderr.String())
		}
	})
}

func TestCLICompletionSuggestsSuitesProfilesAndHistoryIDs(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "ader", "catalogues", "fixture")
	withWorkingDir(t, root, func() {
		testCmd := findCommand(newRootCommand(io.Discard, io.Discard), "test")
		if testCmd == nil {
			t.Fatal("test command missing")
		}

		args, directive := testCmd.ValidArgsFunction(testCmd, nil, "ad")
		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Fatalf("config-dir completion directive = %v, want %v", directive, cobra.ShellCompDirectiveNoFileComp)
		}
		if !strings.Contains(strings.Join(completionsToStrings(args), "\n"), "ader/catalogues/fixture") {
			t.Fatalf("config-dir completion = %v, want ader/catalogues/fixture entry", args)
		}

		suiteCompletion, ok := testCmd.GetFlagCompletionFunc("suite")
		if !ok {
			t.Fatal("suite completion lookup failed")
		}
		if suiteCompletion == nil {
			t.Fatal("suite completion missing")
		}
		suites, _ := suiteCompletion(testCmd, []string{configDir}, "fi")
		if len(suites) == 0 || !strings.Contains(string(suites[0]), "fixture") {
			t.Fatalf("suite completion = %v, want fixture", suites)
		}

		profileCompletion, ok := testCmd.GetFlagCompletionFunc("profile")
		if !ok {
			t.Fatal("profile completion lookup failed")
		}
		if profileCompletion == nil {
			t.Fatal("profile completion missing")
		}
		if err := testCmd.Flags().Set("suite", "fixture"); err != nil {
			t.Fatalf("set suite flag: %v", err)
		}
		profiles, _ := profileCompletion(testCmd, []string{configDir}, "in")
		if len(profiles) == 0 || !strings.Contains(string(profiles[0]), "integration") {
			t.Fatalf("profile completion = %v, want integration", profiles)
		}
		if !strings.Contains(string(profiles[0]), "Deterministic black-box integration suite only") {
			t.Fatalf("profile completion = %v, want YAML description", profiles)
		}

		downgradeCmd := findCommand(newRootCommand(io.Discard, io.Discard), "downgrade")
		if downgradeCmd == nil {
			t.Fatal("downgrade command missing")
		}
		if err := downgradeCmd.Flags().Set("suite", "fixture"); err != nil {
			t.Fatalf("set downgrade suite flag: %v", err)
		}
		stepCompletion, ok := downgradeCmd.GetFlagCompletionFunc("step")
		if !ok {
			t.Fatal("downgrade --step completion lookup failed")
		}
		steps, _ := stepCompletion(downgradeCmd, []string{configDir}, "holons")
		if len(steps) == 0 || !strings.Contains(string(steps[0]), "holons-clem-ader-unit-root") {
			t.Fatalf("downgrade step completion = %v, want holons-clem-ader-unit-root", steps)
		}

		promoteCmd := findCommand(newRootCommand(io.Discard, io.Discard), "promote")
		if promoteCmd == nil {
			t.Fatal("promote command missing")
		}
		if err := promoteCmd.Flags().Set("suite", "fixture"); err != nil {
			t.Fatalf("set promote suite flag: %v", err)
		}
		promoteCompletion, ok := promoteCmd.GetFlagCompletionFunc("step")
		if !ok {
			t.Fatal("promote --step completion lookup failed")
		}
		promoteSteps, _ := promoteCompletion(promoteCmd, []string{configDir}, "fixture")
		if len(promoteSteps) == 0 || !strings.Contains(string(promoteSteps[0]), "fixture-script") {
			t.Fatalf("promote step completion = %v, want progression step", promoteSteps)
		}

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		if code := RunCLI([]string{"test", configDir, "--suite", "fixture", "--profile", "integration", "--source", "workspace", "--archive", "never"}, &stdout, &stderr); code != 0 {
			t.Fatalf("RunCLI(test for history completion) = %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
		}

		showCmd := findCommand(newRootCommand(io.Discard, io.Discard), "show")
		if showCmd == nil {
			t.Fatal("show command missing")
		}
		idCompletion, ok := showCmd.GetFlagCompletionFunc("id")
		if !ok {
			t.Fatal("show --id completion lookup failed")
		}
		if idCompletion == nil {
			t.Fatal("show --id completion missing")
		}
		ids, _ := idCompletion(showCmd, []string{configDir}, "")
		if len(ids) == 0 || !strings.Contains(string(ids[0]), "fixture") {
			t.Fatalf("history id completion = %v, want fixture description", ids)
		}
	})
}

func TestRunCLIUsesConfiguredDefaultProfile(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "ader", "catalogues", "fixture")
	suitePath := filepath.Join(configDir, "suites", "fixture.yaml")
	withWorkingDir(t, root, func() {
		if err := os.WriteFile(suitePath, []byte(`description: fixture suite
defaults:
  profile: integration
steps:
  integration-short:
    check: integration-smoke
    lane: progression
profiles:
  integration:
    description: Deterministic black-box integration suite only
    archive: never
    steps: [integration-short]
`), 0o644); err != nil {
			t.Fatalf("rewrite suite: %v", err)
		}

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		if code := RunCLI([]string{"test", configDir, "--suite", "fixture", "--lane", "progression", "--source", "workspace", "--archive", "never"}, &stdout, &stderr); code != 0 {
			t.Fatalf("RunCLI(test default profile) = %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stderr.String(), "integration-short") {
			t.Fatalf("stderr = %q, want integration profile execution", stderr.String())
		}
	})
}

func TestRunCLITestSilentSuppressesLiveProgress(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "ader", "catalogues", "fixture")
	withWorkingDir(t, root, func() {
		t.Setenv("ADER_TEST_SILENT_STEP_SLEEP", "6")
		var stdout bytes.Buffer
		var stderr bytes.Buffer

		if code := RunCLI([]string{"test", configDir, "--suite", "fixture", "--profile", "unit", "--lane", "progression", "--step-filter", "^quiet-script$", "--source", "workspace", "--archive", "never", "--silent"}, &stdout, &stderr); code != 0 {
			t.Fatalf("RunCLI(test --silent) = %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "summary: pass=") {
			t.Fatalf("silent stdout missing summary: %q", stdout.String())
		}
		if stderr.Len() != 0 {
			t.Fatalf("silent stderr = %q, want empty", stderr.String())
		}
	})
}

func TestRunCLIContextCancellationStopsRun(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "ader", "catalogues", "fixture")
	withWorkingDir(t, root, func() {
		t.Setenv("ADER_TEST_SILENT_STEP_SLEEP", "30")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		done := make(chan int, 1)
		go func() {
			done <- runCLIContext(ctx, []string{
				"test",
				configDir,
				"--suite", "fixture",
				"--profile", "unit",
				"--lane", "progression",
				"--step-filter", "^quiet-script$",
				"--source", "workspace",
				"--archive", "never",
			}, &stdout, &stderr)
		}()

		time.Sleep(250 * time.Millisecond)
		cancel()

		select {
		case code := <-done:
			if code == 0 {
				t.Fatalf("runCLIContext() = %d, want non-zero on cancellation", code)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("CLI run did not stop after context cancellation")
		}

		if !strings.Contains(stderr.String(), "context canceled") {
			t.Fatalf("stderr = %q, want cancellation error", stderr.String())
		}
	})
}

func TestRunCLITestBouquetArchiveHistoryAndShow(t *testing.T) {
	root := testrepo.Create(t)
	aderRoot := filepath.Join(root, "ader")
	withWorkingDir(t, root, func() {
		var stdout bytes.Buffer
		var stderr bytes.Buffer

		if code := RunCLI([]string{"test-bouquet", aderRoot, "--name", "local-dev"}, &stdout, &stderr); code != 0 {
			t.Fatalf("RunCLI(test-bouquet) = %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "summary: pass=") {
			t.Fatalf("test-bouquet stdout missing summary: %q", stdout.String())
		}

		stdout.Reset()
		stderr.Reset()
		if code := RunCLI([]string{"history-bouquet", aderRoot}, &stdout, &stderr); code != 0 {
			t.Fatalf("RunCLI(history-bouquet) = %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
		}
		lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
		if len(lines) != 1 || lines[0] == "" {
			t.Fatalf("history-bouquet stdout = %q, want exactly one history line", stdout.String())
		}
		historyID := strings.Split(lines[0], "\t")[0]

		stdout.Reset()
		stderr.Reset()
		if code := RunCLI([]string{"show-bouquet", aderRoot, "--id", historyID}, &stdout, &stderr); code != 0 {
			t.Fatalf("RunCLI(show-bouquet) = %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "# Bouquet Report") {
			t.Fatalf("show-bouquet stdout missing report heading: %q", stdout.String())
		}

		stdout.Reset()
		stderr.Reset()
		if code := RunCLI([]string{"archive-bouquet", aderRoot, "--latest"}, &stdout, &stderr); code != 0 {
			t.Fatalf("RunCLI(archive-bouquet) = %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "archived bouquet ") {
			t.Fatalf("archive-bouquet stdout missing archived marker: %q", stdout.String())
		}
	})
}

func TestCLIConfigEnvAndFlagPrecedence(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "ader", "catalogues", "fixture")
	withWorkingDir(t, root, func() {
		t.Setenv("ADER_TEST_SOURCE", "workspace")

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		if code := RunCLI([]string{"test", configDir, "--suite", "fixture", "--profile", "integration", "--archive", "never"}, &stdout, &stderr); code != 0 {
			t.Fatalf("RunCLI(test env override) = %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
		}
		runID := latestRunID(t, configDir)
		manifestData, err := os.ReadFile(filepath.Join(configDir, "reports", runID, "manifest.json"))
		if err != nil {
			t.Fatalf("read manifest: %v", err)
		}
		if !strings.Contains(string(manifestData), `"source": "workspace"`) {
			t.Fatalf("manifest missing env-overridden source: %s", string(manifestData))
		}

		stdout.Reset()
		stderr.Reset()
		if code := RunCLI([]string{"test", configDir, "--suite", "fixture", "--profile", "integration", "--source", "committed", "--archive", "never"}, &stdout, &stderr); code != 0 {
			t.Fatalf("RunCLI(test flag override) = %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
		}
		runID = latestRunID(t, configDir)
		manifestData, err = os.ReadFile(filepath.Join(configDir, "reports", runID, "manifest.json"))
		if err != nil {
			t.Fatalf("read manifest after flag override: %v", err)
		}
		if !strings.Contains(string(manifestData), `"source": "committed"`) {
			t.Fatalf("manifest missing flag-overridden source: %s", string(manifestData))
		}
	})
}

func TestCLICompletionInstallZshIsIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/zsh")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := RunCLI([]string{"completion", "install", "zsh"}, &stdout, &stderr); code != 0 {
		t.Fatalf("RunCLI(completion install zsh) = %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}

	profilePath := filepath.Join(home, ".zshrc")
	data, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("read .zshrc: %v", err)
	}
	if !strings.Contains(string(data), `eval "$(ader completion zsh)"`) {
		t.Fatalf(".zshrc missing completion install line: %q", string(data))
	}
	if !strings.Contains(stdout.String(), "installed zsh completion") {
		t.Fatalf("stdout = %q, want install confirmation", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := RunCLI([]string{"completion", "install"}, &stdout, &stderr); code != 0 {
		t.Fatalf("RunCLI(completion install autodetect) = %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	data, err = os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("read .zshrc after second install: %v", err)
	}
	if got := strings.Count(string(data), `eval "$(ader completion zsh)"`); got != 1 {
		t.Fatalf("completion line count = %d, want 1", got)
	}
	if !strings.Contains(stdout.String(), "already configured zsh completion") {
		t.Fatalf("stdout = %q, want idempotent confirmation", stdout.String())
	}
}

func latestRunID(t *testing.T, configDir string) string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(configDir, "reports"))
	if err != nil {
		t.Fatalf("readdir reports: %v", err)
	}
	latest := ""
	latestStarted := time.Time{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(configDir, "reports", entry.Name(), "manifest.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		var payload struct {
			StartedAt string `json:"started_at"`
			HistoryID string `json:"history_id"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			continue
		}
		started, err := time.Parse(time.RFC3339, payload.StartedAt)
		if err != nil {
			started = time.Time{}
		}
		if started.After(latestStarted) || (started.Equal(latestStarted) && entry.Name() > latest) {
			latestStarted = started
			latest = entry.Name()
		}
	}
	if latest == "" {
		t.Fatal("expected at least one report dir")
	}
	return latest
}

func completionsToStrings(values []cobra.Completion) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, string(value))
	}
	return out
}

func TestRunCLITestReturnsNonZeroWhenStepsFail(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "ader", "catalogues", "fixture")
	withWorkingDir(t, root, func() {
		var stdout bytes.Buffer
		var stderr bytes.Buffer

		code := RunCLI([]string{"test", configDir, "--suite", "failing", "--profile", "fail", "--lane", "progression", "--source", "workspace", "--archive", "never"}, &stdout, &stderr)
		if code == 0 {
			t.Fatalf("RunCLI(test fail) = 0, want non-zero\nstdout=%s\nstderr=%s", stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "summary: pass=0 fail=1") {
			t.Fatalf("stdout missing failing summary: %q", stdout.String())
		}
		if !strings.Contains(stderr.String(), "suite failing failed") {
			t.Fatalf("stderr missing failure diagnostic: %q", stderr.String())
		}
	})
}

func TestRunCLITestBouquetReturnsNonZeroWhenEntriesFail(t *testing.T) {
	root := testrepo.Create(t)
	aderRoot := filepath.Join(root, "ader")
	withWorkingDir(t, root, func() {
		var stdout bytes.Buffer
		var stderr bytes.Buffer

		code := RunCLI([]string{"test-bouquet", aderRoot, "--name", "fail-bouquet"}, &stdout, &stderr)
		if code == 0 {
			t.Fatalf("RunCLI(test-bouquet fail) = 0, want non-zero\nstdout=%s\nstderr=%s", stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "summary: pass=0 fail=1") {
			t.Fatalf("stdout missing failing summary: %q", stdout.String())
		}
		if !strings.Contains(stderr.String(), "bouquet fail-bouquet failed") {
			t.Fatalf("stderr missing failure diagnostic: %q", stderr.String())
		}
	})
}
