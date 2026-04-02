package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const morningReportFile = "morning-report.md"

func GenerateMorningReport(aderRoot string) (string, error) {
	absRoot, err := filepath.Abs(aderRoot)
	if err != nil {
		return "", fmt.Errorf("resolve ader root %s: %w", aderRoot, err)
	}

	sections := []struct {
		title string
		slots []reportProgram
	}{
		{title: "Live", slots: collectReportPrograms(filepath.Join(absRoot, "live"), true)},
		{title: "Deferred", slots: collectReportPrograms(filepath.Join(absRoot, "deferred"), false)},
		{title: "Done", slots: collectReportPrograms(filepath.Join(absRoot, "done"), false)},
	}

	var b strings.Builder
	b.WriteString("# Codex Loops Morning Report\n\n")
	b.WriteString("Date: ")
	b.WriteString(time.Now().Format(time.RFC3339))
	b.WriteString("\n")

	for _, section := range sections {
		if len(section.slots) == 0 {
			continue
		}
		b.WriteString("\n## ")
		b.WriteString(section.title)
		b.WriteString("\n")
		for _, item := range section.slots {
			writeReportProgram(&b, item)
		}
	}

	path := filepath.Join(absRoot, morningReportFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}

type reportProgram struct {
	slot   string
	dir    string
	status *Status
}

func collectReportPrograms(dir string, single bool) []reportProgram {
	if single {
		status, err := ReadStatus(dir)
		if err != nil {
			return nil
		}
		return []reportProgram{{
			slot:   inferLiveSlot(status),
			dir:    dir,
			status: status,
		}}
	}
	slots, err := scanNumberedDirs(dir)
	if err != nil {
		return nil
	}
	items := make([]reportProgram, 0, len(slots))
	for _, slot := range slots {
		slotDir := filepath.Join(dir, slot)
		status, err := ReadStatus(slotDir)
		if err != nil {
			continue
		}
		items = append(items, reportProgram{
			slot:   slot,
			dir:    slotDir,
			status: status,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].slot < items[j].slot })
	return items
}

func writeReportProgram(b *strings.Builder, item reportProgram) {
	status := item.status
	b.WriteString("\n### ")
	b.WriteString(item.slot)
	if strings.TrimSpace(status.ProgramDesc) != "" {
		b.WriteString(" | ")
		b.WriteString(status.ProgramDesc)
	}
	b.WriteString("\n\n")
	if strings.TrimSpace(status.Branch) != "" {
		b.WriteString("Branch: ")
		b.WriteString(status.Branch)
		b.WriteString("\n\n")
	}
	b.WriteString("| step | result | attempts | gate report path |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	stepIDs := sortedStepIDs(status)
	for _, stepID := range stepIDs {
		step := status.Steps[stepID]
		lastReport := ""
		if attempt := lastAttempt(step); attempt != nil {
			lastReport = attempt.GateReport
		}
		fmt.Fprintf(b, "| %s | %s | %d | %s |\n", stepID, step.State, len(step.Attempts), lastReport)
	}
	if status.State == "deferred" {
		if stepID, attempt := lastFailedAttempt(status); attempt != nil {
			b.WriteString("\nLast failure: step `")
			b.WriteString(stepID)
			b.WriteString("`")
			if strings.TrimSpace(attempt.GateReport) != "" {
				b.WriteString(" report `")
				b.WriteString(attempt.GateReport)
				b.WriteString("`")
			}
			b.WriteString("\n")
		}
	}
}

func lastAttempt(step StepStatus) *Attempt {
	if len(step.Attempts) == 0 {
		return nil
	}
	return &step.Attempts[len(step.Attempts)-1]
}

func lastFailedAttempt(status *Status) (string, *Attempt) {
	var (
		bestStep string
		best     *Attempt
	)
	for _, stepID := range sortedStepIDs(status) {
		attempt := lastAttempt(status.Steps[stepID])
		if attempt == nil || attempt.GateResult != "FAIL" {
			continue
		}
		copyAttempt := *attempt
		bestStep = stepID
		best = &copyAttempt
	}
	return bestStep, best
}

func sortedStepIDs(status *Status) []string {
	ids := make([]string, 0, len(status.Steps))
	for stepID := range status.Steps {
		ids = append(ids, stepID)
	}
	sort.Strings(ids)
	return ids
}
