// These tests verify the CLI facet of clem-ader from config-dir parsing down to history and show output.
package api

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/organic-programming/clem-ader/internal/testrepo"
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
			t.Fatalf("history stdout = %q, want exactly one run line", stdout.String())
		}
		runID := strings.Split(lines[0], "\t")[0]

		stdout.Reset()
		stderr.Reset()
		if code := RunCLI([]string{"show", configDir, "--run", runID}, &stdout, &stderr); code != 0 {
			t.Fatalf("RunCLI(show) = %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "# Verification Report") {
			t.Fatalf("show stdout missing report heading: %q", stdout.String())
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
