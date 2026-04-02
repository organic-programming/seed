package engine

import (
	"context"
	"fmt"
	"strings"
)

func Promote(_ context.Context, opts PromoteOptions) (*PromoteResult, error) {
	if err := validatePromoteOptions(opts); err != nil {
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

	targetStepIDs, err := targetedStepIDs(cfg, opts.StepIDs, opts.All, "progression")
	if err != nil {
		return nil, err
	}

	result := &PromoteResult{
		Suite:     cfg.SuiteName,
		SuiteFile: cfg.SuitePath,
	}
	changed := false

	for _, stepID := range targetStepIDs {
		step, ok := cfg.Suite.Steps[stepID]
		if !ok {
			return nil, fmt.Errorf("suite %q does not define step %q", cfg.SuiteName, stepID)
		}
		if normalizeStepLane(step.Lane) != "progression" {
			result.IgnoredSteps = append(result.IgnoredSteps, stepID)
			continue
		}
		if err := editor.setStepLane(stepID, "regression"); err != nil {
			return nil, err
		}
		changed = true
		result.PromotedSteps = append(result.PromotedSteps, stepID)
	}

	if changed {
		if err := editor.write(); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func validatePromoteOptions(opts PromoteOptions) error {
	if strings.TrimSpace(opts.ConfigDir) == "" {
		return fmt.Errorf("config dir is required")
	}
	if strings.TrimSpace(opts.Suite) == "" {
		return fmt.Errorf("suite is required")
	}
	if opts.All == (len(orderedUniqueStrings(opts.StepIDs)) > 0) {
		return fmt.Errorf("promote requires exactly one of --all or --step")
	}
	return nil
}
