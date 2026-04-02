package engine

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	bouquetEntriesName = "bouquet-entry-results.json"
)

type bouquetConfig struct {
	Description string               `yaml:"description"`
	Defaults    bouquetDefaults      `yaml:"defaults"`
	Entries     []bouquetEntryConfig `yaml:"entries"`
}

type bouquetDefaults struct {
	Source  string `yaml:"source"`
	Lane    string `yaml:"lane"`
	Archive string `yaml:"archive"`
}

type bouquetEntryConfig struct {
	Catalogue string `yaml:"catalogue"`
	Suite     string `yaml:"suite"`
	Profile   string `yaml:"profile"`
	Source    string `yaml:"source"`
	Lane      string `yaml:"lane"`
	Archive   string `yaml:"archive"`
}

func RunBouquet(ctx context.Context, opts BouquetOptions) (*BouquetRunResult, error) {
	return runBouquet(ctx, opts, nil)
}

func RunBouquetWithProgress(ctx context.Context, opts BouquetOptions, progress io.Writer) (*BouquetRunResult, error) {
	return runBouquet(ctx, opts, progress)
}

func BouquetHistory(_ context.Context, aderRoot string) ([]BouquetHistoryEntry, error) {
	root, err := resolveAderRoot(aderRoot)
	if err != nil {
		return nil, err
	}
	reportsDir := filepath.Join(root, "reports", "bouquets")
	archivesDir := filepath.Join(root, "archives", "bouquets")
	items := map[string]BouquetHistoryEntry{}
	reportEntries, _ := os.ReadDir(reportsDir)
	for _, entry := range reportEntries {
		if !entry.IsDir() {
			continue
		}
		manifest, err := readBouquetManifestFile(filepath.Join(reportsDir, entry.Name(), reportManifestName))
		if err != nil {
			continue
		}
		items[manifest.HistoryID] = bouquetHistoryFromManifest(manifest)
	}
	archiveEntries, _ := os.ReadDir(archivesDir)
	for _, entry := range archiveEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tar.gz") {
			continue
		}
		archivePath := filepath.Join(archivesDir, entry.Name())
		manifest, _, _, err := readBouquetArchivePayload(archivePath)
		if err != nil {
			continue
		}
		summary := bouquetHistoryFromManifest(manifest)
		summary.ArchivePath = archivePath
		if existing, ok := items[summary.HistoryID]; ok && existing.ReportDir != "" {
			existing.ArchivePath = archivePath
			items[summary.HistoryID] = existing
			continue
		}
		items[summary.HistoryID] = summary
	}
	out := make([]BouquetHistoryEntry, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		left := historyTime(out[i].StartedAt)
		right := historyTime(out[j].StartedAt)
		if !left.Equal(right) {
			return left.After(right)
		}
		return out[i].HistoryID > out[j].HistoryID
	})
	return out, nil
}

func ShowBouquetHistory(_ context.Context, aderRoot string, historyID string) (*BouquetRunResult, error) {
	root, err := resolveAderRoot(aderRoot)
	if err != nil {
		return nil, err
	}
	reportDir := filepath.Join(root, "reports", "bouquets", strings.TrimSpace(historyID))
	if dirExists(reportDir) {
		return loadBouquetRunFromReportDir(reportDir)
	}
	archivePath := filepath.Join(root, "archives", "bouquets", strings.TrimSpace(historyID)+".tar.gz")
	if fileExists(archivePath) {
		manifest, entries, summaryMD, err := readBouquetArchivePayload(archivePath)
		if err != nil {
			return nil, err
		}
		manifest.ArchivePath = archivePath
		return &BouquetRunResult{
			Manifest:        manifest,
			Entries:         entries,
			SummaryMarkdown: summaryMD,
		}, nil
	}
	return nil, fmt.Errorf("bouquet history %q not found", historyID)
}

func ArchiveBouquet(_ context.Context, opts BouquetArchiveOptions) (*BouquetRunResult, error) {
	root, err := resolveAderRoot(opts.AderRoot)
	if err != nil {
		return nil, err
	}
	reportsDir := filepath.Join(root, "reports", "bouquets")
	archivesDir := filepath.Join(root, "archives", "bouquets")
	if err := os.MkdirAll(archivesDir, 0o755); err != nil {
		return nil, err
	}
	var reportDir string
	if opts.Latest {
		reportDir, err = latestReportDir(reportsDir)
		if err != nil {
			return nil, err
		}
	} else {
		if strings.TrimSpace(opts.HistoryID) == "" {
			return nil, fmt.Errorf("archive-bouquet requires --id or --latest")
		}
		reportDir = filepath.Join(reportsDir, opts.HistoryID)
		if !dirExists(reportDir) {
			return nil, fmt.Errorf("bouquet history %q not found", opts.HistoryID)
		}
	}
	result, err := loadBouquetRunFromReportDir(reportDir)
	if err != nil {
		return nil, err
	}
	archivePath := filepath.Join(archivesDir, result.Manifest.HistoryID+".tar.gz")
	if err := archiveDirectory(reportDir, archivePath); err != nil {
		return nil, err
	}
	result.Manifest.ArchivePath = archivePath
	if err := writeBouquetReport(result, reportDir); err != nil {
		return nil, err
	}
	return result, nil
}

func runBouquet(ctx context.Context, opts BouquetOptions, progress io.Writer) (*BouquetRunResult, error) {
	if strings.TrimSpace(opts.AderRoot) == "" {
		return nil, fmt.Errorf("Ader root is required")
	}
	if strings.TrimSpace(opts.Name) == "" {
		return nil, fmt.Errorf("bouquet name is required")
	}
	root, err := resolveAderRoot(opts.AderRoot)
	if err != nil {
		return nil, err
	}
	reporter := newProgressReporter(progress)
	cfg, err := readBouquetConfig(root, opts.Name)
	if err != nil {
		return nil, err
	}
	reportsDir := filepath.Join(root, "reports", "bouquets")
	archivesDir := filepath.Join(root, "archives", "bouquets")
	for _, dir := range []string{reportsDir, archivesDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	started := time.Now()
	historyID, err := newBouquetHistoryID(reportsDir, opts.Name, started)
	if err != nil {
		return nil, err
	}
	reportDir := filepath.Join(reportsDir, historyID)
	reporter.phase(fmt.Sprintf("bouquet %s loaded", opts.Name))

	type indexedEntry struct {
		index int
		entry bouquetEntryConfig
	}
	groupOrder := make([]string, 0)
	grouped := map[string][]indexedEntry{}
	for index, entry := range cfg.Entries {
		catalogue := strings.TrimSpace(entry.Catalogue)
		if catalogue == "" {
			return nil, fmt.Errorf("bouquet %q entry %d is missing catalogue", opts.Name, index)
		}
		if _, ok := grouped[catalogue]; !ok {
			groupOrder = append(groupOrder, catalogue)
		}
		grouped[catalogue] = append(grouped[catalogue], indexedEntry{index: index, entry: entry})
	}

	results := make([]BouquetEntryResult, len(cfg.Entries))
	var wg sync.WaitGroup

	for _, catalogue := range groupOrder {
		items := grouped[catalogue]
		wg.Add(1)
		go func(catalogue string, items []indexedEntry) {
			defer wg.Done()
			blocked := false
			for _, indexed := range items {
				entry := indexed.entry
				entryResult := BouquetEntryResult{
					Catalogue: catalogue,
					ConfigDir: filepath.Join(root, "catalogues", catalogue),
					Suite:     strings.TrimSpace(entry.Suite),
				}
				if blocked {
					entryResult.FinalStatus = "SKIP"
					entryResult.Reason = "blocked by prior failure in catalogue"
					results[indexed.index] = entryResult
					continue
				}
				if entryResult.Suite == "" {
					entryResult.FinalStatus = "FAIL"
					entryResult.Reason = "suite is required"
					blocked = true
					results[indexed.index] = entryResult
					continue
				}
				runOpts := RunOptions{
					ConfigDir:     entryResult.ConfigDir,
					Suite:         entryResult.Suite,
					Profile:       strings.TrimSpace(entry.Profile),
					Lane:          firstNonEmpty(strings.TrimSpace(entry.Lane), strings.TrimSpace(cfg.Defaults.Lane)),
					Source:        firstNonEmpty(strings.TrimSpace(entry.Source), strings.TrimSpace(cfg.Defaults.Source)),
					ArchivePolicy: firstNonEmpty(strings.TrimSpace(entry.Archive), strings.TrimSpace(cfg.Defaults.Archive)),
				}
				reporter.phase(fmt.Sprintf("bouquet %s running %s/%s", opts.Name, catalogue, entryResult.Suite))
				child, runErr := Run(ctx, runOpts)
				now := time.Now().UTC().Format(time.RFC3339)
				if runErr != nil {
					entryResult.Profile = strings.TrimSpace(entry.Profile)
					entryResult.Lane = normalizeLane(runOpts.Lane)
					entryResult.Source = normalizeSource(runOpts.Source)
					entryResult.ArchivePolicy = normalizeArchivePolicy(runOpts.ArchivePolicy)
					entryResult.FinalStatus = "FAIL"
					entryResult.Reason = runErr.Error()
					entryResult.StartedAt = now
					entryResult.FinishedAt = now
					blocked = true
					results[indexed.index] = entryResult
					continue
				}
				entryResult.Profile = child.Manifest.Profile
				entryResult.Lane = child.Manifest.Lane
				entryResult.Source = child.Manifest.Source
				entryResult.ArchivePolicy = child.Manifest.ArchivePolicy
				entryResult.FinalStatus = child.Manifest.FinalStatus
				entryResult.ChildHistoryID = child.Manifest.HistoryID
				entryResult.ChildReportDir = child.Manifest.ReportDir
				entryResult.ChildArchive = child.Manifest.ArchivePath
				entryResult.StartedAt = child.Manifest.StartedAt
				entryResult.FinishedAt = child.Manifest.FinishedAt
				results[indexed.index] = entryResult
				if child.Manifest.FinalStatus != "PASS" {
					blocked = true
				}
			}
		}(catalogue, items)
	}
	wg.Wait()

	record := BouquetRecord{
		AderRoot:  root,
		Bouquet:   opts.Name,
		HistoryID: historyID,
		ReportDir: reportDir,
		StartedAt: started.UTC().Format(time.RFC3339),
	}
	for _, entry := range results {
		switch entry.FinalStatus {
		case "PASS":
			record.PassCount++
		case "SKIP":
			record.SkipCount++
		default:
			record.FailCount++
		}
	}
	record.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	if record.FailCount > 0 {
		record.FinalStatus = "FAIL"
	} else {
		record.FinalStatus = "PASS"
	}
	result := &BouquetRunResult{
		Manifest:        record,
		Entries:         results,
		SummaryMarkdown: buildBouquetSummaryMarkdown(record, results),
	}
	if err := writeBouquetReport(result, reportDir); err != nil {
		return nil, err
	}
	return result, nil
}

func resolveAderRoot(raw string) (string, error) {
	return resolveConfigDir(raw)
}

func readBouquetConfig(root string, name string) (bouquetConfig, error) {
	path := filepath.Join(root, "bouquets", strings.TrimSpace(name)+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return bouquetConfig{}, err
	}
	var cfg bouquetConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return bouquetConfig{}, err
	}
	if len(cfg.Entries) == 0 {
		return bouquetConfig{}, fmt.Errorf("bouquet %q does not define any entries", name)
	}
	return cfg, nil
}

func buildBouquetSummaryMarkdown(manifest BouquetRecord, entries []BouquetEntryResult) string {
	var b strings.Builder
	fmt.Fprintln(&b, "# Bouquet Report")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "- Bouquet: `%s`\n", manifest.Bouquet)
	fmt.Fprintf(&b, "- History ID: `%s`\n", manifest.HistoryID)
	fmt.Fprintf(&b, "- Started: `%s`\n", manifest.StartedAt)
	fmt.Fprintf(&b, "- Finished: `%s`\n", manifest.FinishedAt)
	fmt.Fprintf(&b, "- Report Dir: `%s`\n", manifest.ReportDir)
	if strings.TrimSpace(manifest.ArchivePath) != "" {
		fmt.Fprintf(&b, "- Archive: `%s`\n", manifest.ArchivePath)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| Status | Catalogue | Suite | Profile | Lane | Source | Child History | Child Report | Reason |")
	fmt.Fprintln(&b, "| --- | --- | --- | --- | --- | --- | --- | --- | --- |")
	for _, entry := range entries {
		fmt.Fprintf(&b, "| %s | `%s` | `%s` | `%s` | `%s` | `%s` | `%s` | `%s` | %s |\n",
			entry.FinalStatus,
			entry.Catalogue,
			entry.Suite,
			emptyAsNone(entry.Profile),
			emptyAsNone(entry.Lane),
			emptyAsNone(entry.Source),
			emptyAsNone(entry.ChildHistoryID),
			emptyAsNone(entry.ChildReportDir),
			emptyAsNone(entry.Reason),
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

func writeBouquetReport(result *BouquetRunResult, reportDir string) error {
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		return err
	}
	manifestJSON, err := json.MarshalIndent(result.Manifest, "", "  ")
	if err != nil {
		return err
	}
	entriesJSON, err := json.MarshalIndent(result.Entries, "", "  ")
	if err != nil {
		return err
	}
	files := map[string][]byte{
		filepath.Join(reportDir, reportManifestName): []byte(string(manifestJSON) + "\n"),
		filepath.Join(reportDir, bouquetEntriesName): []byte(string(entriesJSON) + "\n"),
		filepath.Join(reportDir, reportSummaryMD):    []byte(result.SummaryMarkdown),
	}
	for path, content := range files {
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func loadBouquetRunFromReportDir(reportDir string) (*BouquetRunResult, error) {
	manifest, err := readBouquetManifestFile(filepath.Join(reportDir, reportManifestName))
	if err != nil {
		return nil, err
	}
	entries, err := readBouquetEntriesFile(filepath.Join(reportDir, bouquetEntriesName))
	if err != nil {
		return nil, err
	}
	summaryMD, _ := os.ReadFile(filepath.Join(reportDir, reportSummaryMD))
	return &BouquetRunResult{
		Manifest:        manifest,
		Entries:         entries,
		SummaryMarkdown: string(summaryMD),
	}, nil
}

func readBouquetManifestFile(path string) (BouquetRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return BouquetRecord{}, err
	}
	var manifest BouquetRecord
	if err := json.Unmarshal(data, &manifest); err != nil {
		return BouquetRecord{}, err
	}
	return manifest, nil
}

func readBouquetEntriesFile(path string) ([]BouquetEntryResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []BouquetEntryResult
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func readBouquetArchivePayload(path string) (BouquetRecord, []BouquetEntryResult, string, error) {
	manifestData, err := readArchiveFile(path, reportManifestName)
	if err != nil {
		return BouquetRecord{}, nil, "", err
	}
	entriesData, err := readArchiveFile(path, bouquetEntriesName)
	if err != nil {
		return BouquetRecord{}, nil, "", err
	}
	summaryMD, err := readArchiveFile(path, reportSummaryMD)
	if err != nil {
		return BouquetRecord{}, nil, "", err
	}
	var manifest BouquetRecord
	if err := json.Unmarshal([]byte(manifestData), &manifest); err != nil {
		return BouquetRecord{}, nil, "", err
	}
	var entries []BouquetEntryResult
	if err := json.Unmarshal([]byte(entriesData), &entries); err != nil {
		return BouquetRecord{}, nil, "", err
	}
	return manifest, entries, summaryMD, nil
}

func bouquetHistoryFromManifest(manifest BouquetRecord) BouquetHistoryEntry {
	return BouquetHistoryEntry{
		HistoryID:   manifest.HistoryID,
		Bouquet:     manifest.Bouquet,
		FinalStatus: manifest.FinalStatus,
		StartedAt:   manifest.StartedAt,
		FinishedAt:  manifest.FinishedAt,
		ReportDir:   manifest.ReportDir,
		ArchivePath: manifest.ArchivePath,
	}
}

func historyTime(value string) time.Time {
	if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value)); err == nil {
		return parsed
	}
	return time.Time{}
}

func newBouquetHistoryID(reportsDir string, bouquet string, now time.Time) (string, error) {
	base := fmt.Sprintf("%s-%s", sanitizeHistoryToken(bouquet), now.Format("20060102_15_04_05"))
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
	return "", fmt.Errorf("could not allocate bouquet history id for %s", base)
}

func archiveDirectory(reportDir string, archivePath string) error {
	file, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gz := gzip.NewWriter(file)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	return filepath.Walk(reportDir, func(path string, info os.FileInfo, walkErr error) error {
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
	})
}
