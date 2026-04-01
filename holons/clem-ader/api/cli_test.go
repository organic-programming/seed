// These tests verify the CLI facet of clem-ader from config-dir parsing down to history, show, and completion output.
package api

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	configDir := filepath.Join(root, "integration")
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
		if !strings.Contains(stdout.String(), "# Verification Report") {
			t.Fatalf("show stdout missing report heading: %q", stdout.String())
		}
	})
}

func TestCLICompletionSuggestsSuitesProfilesAndHistoryIDs(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")
	withWorkingDir(t, root, func() {
		testCmd := findCommand(newRootCommand(io.Discard, io.Discard), "test")
		if testCmd == nil {
			t.Fatal("test command missing")
		}

		args, directive := testCmd.ValidArgsFunction(testCmd, nil, "inte")
		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Fatalf("config-dir completion directive = %v, want %v", directive, cobra.ShellCompDirectiveNoFileComp)
		}
		if len(args) == 0 || !strings.Contains(string(args[0]), "integration") {
			t.Fatalf("config-dir completion = %v, want integration entry", args)
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

func TestRunCLITestSilentSuppressesLiveProgress(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")
	withWorkingDir(t, root, func() {
		var stdout bytes.Buffer
		var stderr bytes.Buffer

		if code := RunCLI([]string{"test", configDir, "--suite", "fixture", "--profile", "integration", "--source", "workspace", "--archive", "never", "--silent"}, &stdout, &stderr); code != 0 {
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

func TestCLIConfigEnvAndFlagPrecedence(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")
	withWorkingDir(t, root, func() {
		t.Setenv("ADER_TEST_SOURCE", "workspace")
		t.Setenv("ADER_TEST_SUITE", "fixture")

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		if code := RunCLI([]string{"test", configDir, "--profile", "integration", "--archive", "never"}, &stdout, &stderr); code != 0 {
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
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() > latest {
			latest = entry.Name()
		}
	}
	if latest == "" {
		t.Fatal("expected at least one report dir")
	}
	return latest
}
