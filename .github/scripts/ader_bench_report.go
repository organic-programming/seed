//go:build ignore

package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type cliArgs struct {
	repoRoot         string
	reportDir        string
	bouquet          string
	runID            string
	ref              string
	sha              string
	runnerName       string
	runnerOS         string
	runnerArch       string
	cacheNote        string
	runNote          string
	githubCreatedAt  string
	runnerAcquiredAt string
	reportStartedAt  string
}

func parseTime(raw string) (time.Time, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, false
	}
	if strings.HasSuffix(value, "Z") {
		value = strings.TrimSuffix(value, "Z") + "+00:00"
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}

func isoNow() string {
	return time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
}

func secondsBetween(start, end string) any {
	left, ok := parseTime(start)
	if !ok {
		return nil
	}
	right, ok := parseTime(end)
	if !ok {
		return nil
	}
	seconds := int(right.Sub(left).Seconds())
	if seconds < 0 {
		return 0
	}
	return seconds
}

func humanSeconds(value any) string {
	if value == nil {
		return "unknown"
	}
	seconds, ok := asInt(value)
	if !ok {
		return "unknown"
	}
	hours := seconds / 3600
	rem := seconds % 3600
	minutes := rem / 60
	secs := rem % 60
	if hours > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", hours, minutes, secs)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm%02ds", minutes, secs)
	}
	return fmt.Sprintf("%ds", secs)
}

func asInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case string:
		if strings.TrimSpace(typed) == "" {
			return 0, false
		}
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		return parsed, err == nil
	default:
		return 0, false
	}
}

func cleanCell(value any) string {
	return strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(text(value))
}

func text(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func readTSV(path string) ([]map[string]string, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return []map[string]string{}, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return []map[string]string{}, nil
	}
	fields := records[0]
	rows := make([]map[string]string, 0, len(records)-1)
	for _, record := range records[1:] {
		row := map[string]string{}
		for i, field := range fields {
			if i < len(record) {
				row[field] = record[i]
			} else {
				row[field] = ""
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func writeTSV(path string, fields []string, rows []map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	writer.Comma = '\t'
	writer.UseCRLF = false
	if err := writer.Write(fields); err != nil {
		return err
	}
	for _, row := range rows {
		record := make([]string, len(fields))
		for i, field := range fields {
			record[i] = cleanCell(row[field])
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func readJSON(path string, dest any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func statusIsFail(value any) bool {
	return strings.ToUpper(strings.TrimSpace(text(value))) == "FAIL"
}

func statusIsPass(value any) bool {
	return strings.ToUpper(strings.TrimSpace(text(value))) == "PASS"
}

func normalizeBlock(row map[string]string) map[string]any {
	var seconds any
	if parsed, err := strconv.Atoi(strings.TrimSpace(row["seconds"])); err == nil {
		seconds = parsed
	} else {
		seconds = nil
	}
	return map[string]any{
		"block":       row["block"],
		"status":      row["status"],
		"exit_code":   row["exit_code"],
		"started_at":  row["started_at"],
		"finished_at": row["finished_at"],
		"seconds":     seconds,
		"reason":      row["reason"],
	}
}

func latestBouquetReport(repoRoot, bouquet string, notBefore time.Time, hasNotBefore bool) string {
	reportsRoot := filepath.Join(repoRoot, "ader", "reports", "bouquets")
	matches, _ := filepath.Glob(filepath.Join(reportsRoot, "*", "manifest.json"))
	type candidate struct {
		started time.Time
		dir     string
	}
	candidates := []candidate{}
	for _, manifestPath := range matches {
		manifest := map[string]any{}
		if err := readJSON(manifestPath, &manifest); err != nil {
			continue
		}
		if text(manifest["bouquet"]) != bouquet {
			continue
		}
		started, ok := parseTime(text(manifest["started_at"]))
		if !ok {
			continue
		}
		if hasNotBefore && started.Before(notBefore) {
			continue
		}
		candidates = append(candidates, candidate{started: started, dir: filepath.Dir(manifestPath)})
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].started.Before(candidates[j].started) })
	return candidates[len(candidates)-1].dir
}

func resolveExistingPath(repoRoot string, raw any) string {
	value := strings.TrimSpace(text(raw))
	if value == "" {
		return ""
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(repoRoot, value)
	}
	if _, err := os.Stat(value); err != nil {
		return ""
	}
	return value
}

func entrySeconds(entry map[string]any) any {
	return secondsBetween(text(entry["started_at"]), text(entry["finished_at"]))
}

func collectEntries(repoRoot, bouquetReport string) []map[string]any {
	if bouquetReport == "" {
		return []map[string]any{}
	}
	entriesPath := filepath.Join(bouquetReport, "bouquet-entry-results.json")
	if _, err := os.Stat(entriesPath); err != nil {
		return []map[string]any{}
	}
	rawEntries := []map[string]any{}
	if err := readJSON(entriesPath, &rawEntries); err != nil {
		return []map[string]any{}
	}
	entries := make([]map[string]any, 0, len(rawEntries))
	for _, entry := range rawEntries {
		reason := text(entry["reason"])
		if strings.TrimSpace(reason) == "" && statusIsFail(entry["final_status"]) && strings.TrimSpace(text(entry["child_report_dir"])) != "" {
			reason = "see child_report_dir"
		}
		entries = append(entries, map[string]any{
			"catalogue":        text(entry["catalogue"]),
			"suite":            text(entry["suite"]),
			"profile":          text(entry["profile"]),
			"lane":             text(entry["lane"]),
			"source":           text(entry["source"]),
			"status":           text(entry["final_status"]),
			"seconds":          entrySeconds(entry),
			"started_at":       text(entry["started_at"]),
			"finished_at":      text(entry["finished_at"]),
			"child_report_dir": text(entry["child_report_dir"]),
			"reason":           reason,
			"child_history_id": text(entry["child_history_id"]),
		})
	}
	return entries
}

func collectFailedSteps(repoRoot string, entries []map[string]any) []map[string]any {
	failed := []map[string]any{}
	for _, entry := range entries {
		childDir := resolveExistingPath(repoRoot, entry["child_report_dir"])
		if childDir == "" {
			continue
		}
		stepsPath := filepath.Join(childDir, "step-results.json")
		if _, err := os.Stat(stepsPath); err != nil {
			continue
		}
		steps := []map[string]any{}
		if err := readJSON(stepsPath, &steps); err != nil {
			continue
		}
		for _, step := range steps {
			if !statusIsFail(step["status"]) {
				continue
			}
			reason := text(step["reason"])
			if strings.TrimSpace(reason) == "" && strings.TrimSpace(text(step["log_path"])) != "" {
				reason = "see log_path"
			}
			failed = append(failed, map[string]any{
				"catalogue":        entry["catalogue"],
				"suite":            entry["suite"],
				"child_history_id": entry["child_history_id"],
				"step_id":          text(step["step_id"]),
				"status":           text(step["status"]),
				"duration_seconds": step["duration_seconds"],
				"reason":           reason,
				"log_path":         text(step["log_path"]),
			})
		}
	}
	return failed
}

func firstFailure(blocks, entries, failedSteps []map[string]any) string {
	for _, block := range blocks {
		if statusIsFail(block["status"]) {
			reason := text(block["reason"])
			if strings.TrimSpace(reason) == "" {
				reason = "exit code " + text(block["exit_code"])
			}
			return fmt.Sprintf("block %s failed: %s", block["block"], reason)
		}
	}
	for _, entry := range entries {
		if statusIsFail(entry["status"]) {
			reason := text(entry["reason"])
			if strings.TrimSpace(reason) == "" {
				reason = "suite failed"
			}
			return fmt.Sprintf("%s/%s failed: %s", entry["catalogue"], entry["suite"], reason)
		}
	}
	if len(failedSteps) > 0 {
		step := failedSteps[0]
		reason := text(step["reason"])
		if strings.TrimSpace(reason) == "" {
			reason = "step failed"
		}
		return fmt.Sprintf("%s/%s step %s failed: %s", step["catalogue"], step["suite"], step["step_id"], reason)
	}
	return ""
}

func md(value any) string {
	return strings.ReplaceAll(cleanCell(value), "|", "\\|")
}

func mdTable(fields []string, rows []map[string]any) string {
	lines := []string{
		"| " + strings.Join(fields, " | ") + " |",
		"| " + strings.Join(repeat("---", len(fields)), " | ") + " |",
	}
	for _, row := range rows {
		values := make([]string, len(fields))
		for i, field := range fields {
			values[i] = md(row[field])
		}
		lines = append(lines, "| "+strings.Join(values, " | ")+" |")
	}
	return strings.Join(lines, "\n")
}

func repeat(value string, count int) []string {
	out := make([]string, count)
	for i := range out {
		out[i] = value
	}
	return out
}

func buildSummaryMarkdown(report map[string]any) string {
	identity := report["identity"].(map[string]any)
	wall := report["wall_clock"].(map[string]any)
	outcome := report["outcome"].(map[string]any)
	blocks := report["blocks"].([]map[string]any)
	entries := report["bouquet_entries"].([]map[string]any)
	failedSteps := report["failed_steps"].([]map[string]any)
	lines := []string{
		"# ader Benchmark Report",
		"",
		"## Run Identity",
		"",
		fmt.Sprintf("- Run ID: `%s`", identity["run_id"]),
		fmt.Sprintf("- Ref: `%s`", identity["ref"]),
		fmt.Sprintf("- SHA: `%s`", identity["sha"]),
		fmt.Sprintf("- Bouquet: `%s`", identity["bouquet"]),
		fmt.Sprintf("- Runner: `%s` (`%s` / `%s`)", identity["runner_name"], identity["runner_os"], identity["runner_arch"]),
		fmt.Sprintf("- Cache note: `%s`", identity["cache_note"]),
		fmt.Sprintf("- Run note: `%s`", identity["run_note"]),
		"",
		"## Wall Clock",
		"",
		mdTable([]string{"metric", "value"}, []map[string]any{
			{"metric": "GitHub created_at", "value": defaultUnknown(wall["github_created_at"])},
			{"metric": "Runner acquired_at", "value": defaultUnknown(wall["runner_acquired_at"])},
			{"metric": "Finished_at", "value": defaultUnknown(wall["finished_at"])},
			{"metric": "Runner wait", "value": humanSeconds(wall["runner_wait_seconds"])},
			{"metric": "Executed wall-clock", "value": humanSeconds(wall["executed_seconds"])},
		}),
		"",
		"## Outcome",
		"",
		fmt.Sprintf("- Functional status: `%s`", outcome["functional_status"]),
		fmt.Sprintf("- Failed block: `%s`", defaultNone(outcome["failed_block"])),
		fmt.Sprintf("- Failed catalogue/suite count: `%s`", outcome["failed_entry_count"]),
		fmt.Sprintf("- Skipped catalogue/suite count: `%s`", outcome["skipped_entry_count"]),
		fmt.Sprintf("- Failed internal step count: `%s`", outcome["failed_step_count"]),
		fmt.Sprintf("- First failure: `%s`", defaultNone(outcome["first_failure"])),
		fmt.Sprintf("- Bouquet report: `%s`", defaultNone(outcome["bouquet_report_dir"])),
		"",
		"## Blocks",
		"",
		mdTable([]string{"block", "status", "exit_code", "seconds", "reason"}, blocks),
		"",
		"## Bouquet Entries",
		"",
	}
	if len(entries) > 0 {
		lines = append(lines, mdTable([]string{"catalogue", "suite", "profile", "lane", "source", "status", "seconds", "child_report_dir", "reason"}, entries))
	} else {
		lines = append(lines, "No bouquet entry report was found.")
	}
	lines = append(lines, "", "## Failures", "")
	failedEntryCount, _ := asInt(outcome["failed_entry_count"])
	failedBlock := strings.TrimSpace(text(outcome["failed_block"]))
	if len(failedSteps) == 0 && failedEntryCount == 0 && failedBlock == "" {
		lines = append(lines, "No functional failures found.")
	} else {
		failedEntries := []map[string]any{}
		for _, entry := range entries {
			if statusIsFail(entry["status"]) {
				failedEntries = append(failedEntries, entry)
			}
		}
		if len(failedEntries) > 0 {
			lines = append(lines, "### Failed Catalogue/Suite Entries", "")
			lines = append(lines, mdTable([]string{"catalogue", "suite", "status", "seconds", "reason", "child_report_dir"}, failedEntries), "")
		}
		if len(failedSteps) > 0 {
			lines = append(lines, "### Failed Internal Steps", "")
			lines = append(lines, mdTable([]string{"catalogue", "suite", "step_id", "status", "duration_seconds", "reason", "log_path"}, failedSteps))
		} else if len(failedEntries) > 0 {
			lines = append(lines, "No failed internal steps were found in child step reports.")
		}
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func defaultUnknown(value any) string {
	if strings.TrimSpace(text(value)) == "" {
		return "unknown"
	}
	return text(value)
}

func defaultNone(value any) string {
	if strings.TrimSpace(text(value)) == "" {
		return "none"
	}
	return text(value)
}

func buildReport(args cliArgs) (map[string]any, error) {
	repoRoot, _ := filepath.Abs(args.repoRoot)
	reportDir, _ := filepath.Abs(args.reportDir)
	rawRows, err := readTSV(filepath.Join(reportDir, "blocks.tsv"))
	if err != nil {
		return nil, err
	}
	blocks := make([]map[string]any, 0, len(rawRows)+1)
	for _, row := range rawRows {
		blocks = append(blocks, normalizeBlock(row))
	}
	runnerAcquired, hasRunnerAcquired := parseTime(args.runnerAcquiredAt)
	bouquetReport := latestBouquetReport(repoRoot, args.bouquet, runnerAcquired, hasRunnerAcquired)
	bouquetManifest := map[string]any{}
	if bouquetReport != "" {
		_ = readJSON(filepath.Join(bouquetReport, "manifest.json"), &bouquetManifest)
	}
	entries := collectEntries(repoRoot, bouquetReport)
	failedSteps := collectFailedSteps(repoRoot, entries)
	reportFinishedAt := isoNow()
	blocks = append(blocks, map[string]any{
		"block":       "report-generation",
		"status":      "PASS",
		"exit_code":   "0",
		"started_at":  args.reportStartedAt,
		"finished_at": reportFinishedAt,
		"seconds":     secondsBetween(args.reportStartedAt, reportFinishedAt),
		"reason":      "",
	})
	if err := writeTSV(filepath.Join(reportDir, "blocks.tsv"), []string{"block", "status", "exit_code", "started_at", "finished_at", "seconds", "reason"}, blocks); err != nil {
		return nil, err
	}
	failedBlock := ""
	for _, block := range blocks {
		if statusIsFail(block["status"]) {
			failedBlock = text(block["block"])
			break
		}
	}
	failedEntries := []map[string]any{}
	skippedEntries := []map[string]any{}
	for _, entry := range entries {
		if statusIsFail(entry["status"]) {
			failedEntries = append(failedEntries, entry)
		}
		if strings.ToUpper(strings.TrimSpace(text(entry["status"]))) == "SKIP" {
			skippedEntries = append(skippedEntries, entry)
		}
	}
	bouquetStatus := strings.ToUpper(strings.TrimSpace(text(bouquetManifest["final_status"])))
	functionalStatus := "PASS"
	if failedBlock != "" || len(failedEntries) > 0 || len(failedSteps) > 0 || (bouquetStatus != "" && bouquetStatus != "PASS") {
		functionalStatus = "FAIL"
	}
	return map[string]any{
		"identity": map[string]any{
			"run_id":      args.runID,
			"ref":         args.ref,
			"sha":         args.sha,
			"bouquet":     args.bouquet,
			"runner_name": args.runnerName,
			"runner_os":   args.runnerOS,
			"runner_arch": args.runnerArch,
			"cache_note":  args.cacheNote,
			"run_note":    args.runNote,
		},
		"wall_clock": map[string]any{
			"github_created_at":   args.githubCreatedAt,
			"runner_acquired_at":  args.runnerAcquiredAt,
			"finished_at":         reportFinishedAt,
			"runner_wait_seconds": secondsBetween(args.githubCreatedAt, args.runnerAcquiredAt),
			"executed_seconds":    secondsBetween(args.runnerAcquiredAt, reportFinishedAt),
		},
		"outcome": map[string]any{
			"functional_status":   functionalStatus,
			"failed_block":        failedBlock,
			"failed_entry_count":  len(failedEntries),
			"skipped_entry_count": len(skippedEntries),
			"failed_step_count":   len(failedSteps),
			"first_failure":       firstFailure(blocks, entries, failedSteps),
			"bouquet_report_dir":  bouquetReport,
		},
		"blocks":          blocks,
		"bouquet_entries": entries,
		"failed_steps":    failedSteps,
	}, nil
}

func writeReport(reportDir string, report map[string]any) error {
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		return err
	}
	if err := writeTSV(filepath.Join(reportDir, "bouquet-entries.tsv"), []string{"catalogue", "suite", "profile", "lane", "source", "status", "seconds", "started_at", "finished_at", "child_report_dir", "reason"}, report["bouquet_entries"].([]map[string]any)); err != nil {
		return err
	}
	if err := writeTSV(filepath.Join(reportDir, "failed-steps.tsv"), []string{"catalogue", "suite", "child_history_id", "step_id", "status", "duration_seconds", "reason", "log_path"}, report["failed_steps"].([]map[string]any)); err != nil {
		return err
	}
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(reportDir, "summary.json"), append(jsonData, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(reportDir, "summary.md"), []byte(buildSummaryMarkdown(report)), 0o644)
}

func parseArgs() cliArgs {
	var args cliArgs
	flag.StringVar(&args.repoRoot, "repo-root", "", "")
	flag.StringVar(&args.reportDir, "report-dir", "", "")
	flag.StringVar(&args.bouquet, "bouquet", "", "")
	flag.StringVar(&args.runID, "run-id", "", "")
	flag.StringVar(&args.ref, "ref", "", "")
	flag.StringVar(&args.sha, "sha", "", "")
	flag.StringVar(&args.runnerName, "runner-name", "", "")
	flag.StringVar(&args.runnerOS, "runner-os", "", "")
	flag.StringVar(&args.runnerArch, "runner-arch", "", "")
	flag.StringVar(&args.cacheNote, "cache-note", "", "")
	flag.StringVar(&args.runNote, "run-note", "", "")
	flag.StringVar(&args.githubCreatedAt, "github-created-at", "", "")
	flag.StringVar(&args.runnerAcquiredAt, "runner-acquired-at", "", "")
	flag.StringVar(&args.reportStartedAt, "report-started-at", "", "")
	flag.Parse()
	return args
}

func main() {
	args := parseArgs()
	report, err := buildReport(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	reportDir, _ := filepath.Abs(args.reportDir)
	if err := writeReport(reportDir, report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
