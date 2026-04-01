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

	targetStepIDs, err := targetedStepIDs(cfg, editor, opts.StepIDs, opts.All, "regression")
	if err != nil {
		return nil, err
	}

	result := &DowngradeResult{
		Suite:     cfg.SuiteName,
		SuiteFile: cfg.SuitePath,
	}
	changed := false

	for _, stepID := range targetStepIDs {
		lane, _, err := editor.stepLane(stepID)
		if err != nil {
			return nil, err
		}
		if lane != "regression" {
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
	if opts.All == (len(orderedUniqueStrings(opts.StepIDs)) > 0) {
		return fmt.Errorf("downgrade requires exactly one of --all or --step")
	}
	return nil
}

func targetedStepIDs(cfg *runtimeConfig, editor *suiteEditor, rawStepIDs []string, all bool, currentLane string) ([]string, error) {
	if !all {
		return orderedUniqueStrings(rawStepIDs), nil
	}
	out := make([]string, 0, len(cfg.Suite.Steps))
	for _, stepID := range orderedStepIDs(cfg.Suite.Steps) {
		lane, _, err := editor.stepLane(stepID)
		if err != nil {
			return nil, err
		}
		if lane == currentLane {
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
