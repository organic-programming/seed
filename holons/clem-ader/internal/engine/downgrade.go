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
	unlock, err := acquireCatalogueLock(context.Background(), cfg.Paths.ArtifactsDir)
	if err != nil {
		return nil, err
	}
	defer unlock()

	editor, err := loadSuiteEditor(cfg.SuitePath)
	if err != nil {
		return nil, err
	}

	targetStepIDs, err := targetedStepIDs(cfg, opts.StepIDs, opts.All, "regression")
	if err != nil {
		return nil, err
	}

	result := &DowngradeResult{
		Suite:     cfg.SuiteName,
		SuiteFile: cfg.SuitePath,
	}
	changed := false

	for _, stepID := range targetStepIDs {
		step, ok := cfg.Suite.Steps[stepID]
		if !ok {
			return nil, fmt.Errorf("suite %q does not define step %q", cfg.SuiteName, stepID)
		}
		if normalizeStepLane(step.Lane) != "regression" {
			result.IgnoredSteps = append(result.IgnoredSteps, stepID)
			continue
		}
		if err := editor.setStepLane(stepID, "progression"); err != nil {
			return nil, err
		}
		changed = true
		result.DowngradedSteps = append(result.DowngradedSteps, stepID)
	}

	if changed {
		if err := editor.write(); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func validateDowngradeOptions(opts DowngradeOptions) error {
	if strings.TrimSpace(opts.ConfigDir) == "" {
		return fmt.Errorf("config dir is required")
	}
	if strings.TrimSpace(opts.Suite) == "" {
		return fmt.Errorf("suite is required")
	}
	if opts.All == (len(orderedUniqueStrings(opts.StepIDs)) > 0) {
		return fmt.Errorf("downgrade requires exactly one of --all or --step")
	}
	return nil
}

func targetedStepIDs(cfg *runtimeConfig, rawStepIDs []string, all bool, currentLane string) ([]string, error) {
	if !all {
		return orderedUniqueStrings(rawStepIDs), nil
	}
	out := make([]string, 0, len(cfg.Suite.Steps))
	for _, stepID := range orderedStepIDs(cfg.Suite.Steps) {
		if normalizeStepLane(cfg.Suite.Steps[stepID].Lane) == currentLane {
			out = append(out, stepID)
		}
	}
	return out, nil
}

func orderedStepIDs(steps map[string]suiteStepConfig) []string {
	out := make([]string, 0, len(steps))
	for stepID := range steps {
		out = append(out, stepID)
	}
	sort.Strings(out)
	return out
}
