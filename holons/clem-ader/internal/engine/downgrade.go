package engine

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

func Downgrade(_ context.Context, opts DowngradeOptions) (*DowngradeResult, error) {
	if err := validateDowngradeOptions(opts); err != nil {
		return nil, err
	}
	cfg, err := loadRunConfig(opts.ConfigDir, opts.Suite)
	if err != nil {
		return nil, err
	}

	editor, err := loadSuiteEditor(cfg.SuitePath)
	if err != nil {
		return nil, err
	}

	profiles, err := targetedDowngradeProfiles(cfg, opts)
	if err != nil {
		return nil, err
	}
	targetStepIDs := orderedUniqueStrings(opts.StepIDs)

	result := &DowngradeResult{
		Suite:     cfg.SuiteName,
		SuiteFile: cfg.SuitePath,
	}
	ignoredSet := map[string]struct{}{}
	changed := false

	for _, profile := range profiles {
		moved, ignored, err := editor.downgradeProfile(profile, targetStepIDs, opts.All)
		if err != nil {
			return nil, err
		}
		if len(moved) > 0 {
			changed = true
			result.ProfileChanges = append(result.ProfileChanges, DowngradeProfileChange{
				Profile:    profile,
				MovedSteps: moved,
			})
		}
		for _, stepID := range ignored {
			ignoredSet[stepID] = struct{}{}
		}
	}

	if changed {
		if err := editor.write(); err != nil {
			return nil, err
		}
	}

	if !opts.All {
		for _, stepID := range targetStepIDs {
			if _, ok := ignoredSet[stepID]; ok {
				result.IgnoredSteps = append(result.IgnoredSteps, stepID)
			}
		}
	}
	return result, nil
}

func validateDowngradeOptions(opts DowngradeOptions) error {
	if strings.TrimSpace(opts.ConfigDir) == "" {
		return fmt.Errorf("config dir is required")
	}
	if opts.All == (len(orderedUniqueStrings(opts.StepIDs)) > 0) {
		return fmt.Errorf("downgrade requires exactly one of --all or --step")
	}
	if profile := strings.TrimSpace(opts.Profile); profile != "" {
		normalized := normalizeProfile(profile)
		if _, ok := profileDescriptions[normalized]; !ok {
			return fmt.Errorf("unsupported profile %q", opts.Profile)
		}
	}
	return nil
}

func targetedDowngradeProfiles(cfg *runtimeConfig, opts DowngradeOptions) ([]string, error) {
	if profile := strings.TrimSpace(opts.Profile); profile != "" {
		normalized := normalizeProfile(profile)
		if _, ok := cfg.Suite.Profiles[normalized]; !ok {
			return nil, fmt.Errorf("suite %q does not define profile %q", cfg.SuiteName, normalized)
		}
		return []string{normalized}, nil
	}
	if opts.All {
		return orderedSuiteProfiles(cfg.Suite.Profiles), nil
	}
	stepIDs := orderedUniqueStrings(opts.StepIDs)
	seen := make(map[string]struct{})
	var profiles []string
	for _, profile := range orderedSuiteProfiles(cfg.Suite.Profiles) {
		lanes := cfg.Suite.Profiles[profile]
		for _, stepID := range stepIDs {
			if !containsString(lanes.Regression, stepID) {
				continue
			}
			if _, ok := seen[profile]; ok {
				break
			}
			seen[profile] = struct{}{}
			profiles = append(profiles, profile)
			break
		}
	}
	if len(profiles) == 0 {
		return orderedSuiteProfiles(cfg.Suite.Profiles), nil
	}
	return profiles, nil
}

func orderedSuiteProfiles(profiles map[string]suiteProfileLanes) []string {
	out := make([]string, 0, len(profiles))
	seen := make(map[string]struct{}, len(profiles))
	for _, profile := range append(append([]string(nil), profileLadder...), "stress") {
		if _, ok := profiles[profile]; ok {
			out = append(out, profile)
			seen[profile] = struct{}{}
		}
	}
	var extras []string
	for profile := range profiles {
		if _, ok := seen[profile]; ok {
			continue
		}
		extras = append(extras, profile)
	}
	sort.Strings(extras)
	return append(out, extras...)
}
