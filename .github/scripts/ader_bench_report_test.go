package scripts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAderBenchReportCommands(t *testing.T) {
	root := t.TempDir()
	reportDir := filepath.Join(root, "ader", "reports", "bench", "100")
	writeBlocks(t, reportDir, "bootstrap\tPASS\t0\t2026-05-03T00:01:00Z\t2026-05-03T00:01:03Z\t3\t\n")
	child := filepath.Join(root, "ader", "catalogues", "grace-op", "reports", "child-pass")
	writeChild(t, child, "PASS", []map[string]any{})
	writeBouquet(t, root, "local-dev", "PASS", []map[string]any{entry("grace-op", "op_version", "PASS", child, "")}, "2026-05-03T00:01:00Z")

	runScript(t, "ader_bench_report.go",
		"--repo-root", root,
		"--report-dir", reportDir,
		"--bouquet", "local-dev",
		"--run-id", "100",
		"--ref", "test-ref",
		"--sha", "abc123",
		"--runner-name", "self-hosted-macos",
		"--runner-os", "macOS",
		"--runner-arch", "ARM64",
		"--cache-note", "fixture",
		"--run-note", "",
		"--github-created-at", "2026-05-03T00:00:00Z",
		"--runner-acquired-at", "2026-05-03T00:01:00Z",
		"--report-started-at", "2026-05-03T00:02:00Z",
	)
	var summary map[string]any
	data, err := os.ReadFile(filepath.Join(reportDir, "summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatal(err)
	}
	outcome := summary["outcome"].(map[string]any)
	if outcome["functional_status"] != "PASS" {
		t.Fatalf("functional_status = %v", outcome["functional_status"])
	}
	md, err := os.ReadFile(filepath.Join(reportDir, "summary.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(md), "Functional status: `PASS`") {
		t.Fatalf("summary.md missing pass status:\n%s", string(md))
	}
	failedSteps, err := os.ReadFile(filepath.Join(reportDir, "failed-steps.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(failedSteps)) != "catalogue\tsuite\tchild_history_id\tstep_id\tstatus\tduration_seconds\treason\tlog_path" {
		t.Fatalf("failed-steps.tsv = %q", string(failedSteps))
	}
}

func writeBlocks(t *testing.T, reportDir string, row string) {
	t.Helper()
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "block\tstatus\texit_code\tstarted_at\tfinished_at\tseconds\treason\n" + row
	if err := os.WriteFile(filepath.Join(reportDir, "blocks.tsv"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeBouquet(t *testing.T, root, bouquet, status string, entries []map[string]any, startedAt string) {
	t.Helper()
	reportDir := filepath.Join(root, "ader", "reports", "bouquets", bouquet+"-20260503")
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(reportDir, "manifest.json"), map[string]any{
		"bouquet":      bouquet,
		"history_id":   filepath.Base(reportDir),
		"report_dir":   reportDir,
		"started_at":   startedAt,
		"finished_at":  "2026-05-03T00:02:00Z",
		"final_status": status,
	})
	writeJSON(t, filepath.Join(reportDir, "bouquet-entry-results.json"), entries)
}

func writeChild(t *testing.T, child, status string, steps []map[string]any) {
	t.Helper()
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(child, "manifest.json"), map[string]any{
		"history_id":   filepath.Base(child),
		"report_dir":   child,
		"started_at":   "2026-05-03T00:01:00Z",
		"finished_at":  "2026-05-03T00:02:00Z",
		"final_status": status,
	})
	writeJSON(t, filepath.Join(child, "step-results.json"), steps)
}

func entry(catalogue, suite, status, child, reason string) map[string]any {
	return map[string]any{
		"catalogue":        catalogue,
		"suite":            suite,
		"profile":          "smoke",
		"lane":             "both",
		"source":           "workspace",
		"archive_policy":   "never",
		"final_status":     status,
		"reason":           reason,
		"child_history_id": filepath.Base(child),
		"child_report_dir": child,
		"started_at":       "2026-05-03T00:01:00Z",
		"finished_at":      "2026-05-03T00:02:00Z",
	}
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
