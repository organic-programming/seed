package engine

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type CompletionItem struct {
	Value       string
	Description string
}

func DiscoverConfigDirs(start string) ([]CompletionItem, error) {
	if strings.TrimSpace(start) == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		start = cwd
	}
	absStart, err := filepath.Abs(start)
	if err != nil {
		return nil, err
	}
	repoRoot, err := detectRepoRootFrom(absStart)
	if err != nil {
		return nil, nil
	}
	var items []CompletionItem
	_ = filepath.WalkDir(repoRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() && shouldSkipCompletionDir(entry.Name()) {
			return filepath.SkipDir
		}
		if entry.IsDir() || entry.Name() != "ader.yaml" {
			return nil
		}
		dir := filepath.Dir(path)
		rel, err := filepath.Rel(absStart, dir)
		if err != nil {
			rel = dir
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			rel = dir
		}
		items = append(items, CompletionItem{Value: rel})
		return nil
	})
	sort.Slice(items, func(i, j int) bool { return items[i].Value < items[j].Value })
	return items, nil
}

func ListSuites(configDir string) ([]CompletionItem, error) {
	cfg, err := loadRepoConfig(configDir)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(cfg.ConfigDir, "suites"))
	if err != nil {
		return nil, err
	}
	var items []CompletionItem
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".yaml")
		suite, err := readSuiteConfig(filepath.Join(cfg.ConfigDir, "suites", entry.Name()))
		if err != nil {
			continue
		}
		items = append(items, CompletionItem{Value: name, Description: suite.Description})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Value < items[j].Value })
	return items, nil
}

func ListProfiles(configDir string, suite string) ([]CompletionItem, error) {
	cfg, err := loadRunConfig(configDir, suite)
	if err != nil {
		return nil, err
	}
	items := make([]CompletionItem, 0, len(cfg.Suite.Profiles))
	for _, profile := range SupportedProfiles() {
		if _, ok := cfg.Suite.Profiles[profile]; !ok {
			continue
		}
		items = append(items, CompletionItem{Value: profile, Description: ProfileDescription(profile)})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Value < items[j].Value })
	return items, nil
}

func ListRegressionSteps(configDir string, suite string, profile string) ([]CompletionItem, error) {
	cfg, err := loadRunConfig(configDir, suite)
	if err != nil {
		return nil, err
	}
	stepProfiles := map[string][]string{}
	if value := strings.TrimSpace(profile); value != "" {
		value = normalizeProfile(value)
		lanes, ok := cfg.Suite.Profiles[value]
		if !ok {
			return nil, fmt.Errorf("suite %q does not define profile %q", cfg.SuiteName, value)
		}
		for _, stepID := range lanes.Regression {
			stepProfiles[stepID] = append(stepProfiles[stepID], value)
		}
	} else {
		for _, item := range orderedSuiteProfiles(cfg.Suite.Profiles) {
			for _, stepID := range cfg.Suite.Profiles[item].Regression {
				stepProfiles[stepID] = append(stepProfiles[stepID], item)
			}
		}
	}
	items := make([]CompletionItem, 0, len(stepProfiles))
	for stepID, profiles := range stepProfiles {
		description := strings.TrimSpace(cfg.Suite.Steps[stepID].Description)
		if len(profiles) > 0 {
			description = strings.TrimSpace(strings.Join([]string{description, "regression in " + strings.Join(profiles, ", ")}, " | "))
		}
		items = append(items, CompletionItem{Value: stepID, Description: description})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Value < items[j].Value })
	return items, nil
}

func DefaultSuite(configDir string) (string, error) {
	cfg, err := loadRepoConfig(configDir)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(cfg.Root.Defaults.Suite), nil
}

func shouldSkipCompletionDir(name string) bool {
	switch name {
	case ".git", ".artifacts", "reports", "archives", ".t", "node_modules", "target", "build", ".build", "__pycache__", ".gradle", ".kotlin", "obj":
		return true
	default:
		return false
	}
}
