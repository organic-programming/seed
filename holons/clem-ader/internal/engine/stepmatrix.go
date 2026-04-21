package engine

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func normalizeProfile(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "global" {
		return "full"
	}
	return value
}

func normalizeStepLane(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "progression"
	}
	return value
}

func normalizeLane(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "regression"
	}
	return value
}

func normalizeSource(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "committed"
	}
	return value
}

func normalizeArchivePolicy(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "auto"
	}
	return value
}

func validateRunOptions(opts RunOptions) error {
	if strings.TrimSpace(opts.ConfigDir) == "" {
		return fmt.Errorf("config dir is required")
	}
	if strings.TrimSpace(opts.Suite) == "" {
		return fmt.Errorf("suite is required")
	}
	switch normalizeLane(opts.Lane) {
	case "regression", "progression", "both":
	default:
		return fmt.Errorf("unsupported lane %q", opts.Lane)
	}
	switch normalizeSource(opts.Source) {
	case "committed", "workspace":
	default:
		return fmt.Errorf("unsupported source %q", opts.Source)
	}
	switch normalizeArchivePolicy(opts.ArchivePolicy) {
	case "auto", "always", "never":
	default:
		return fmt.Errorf("unsupported archive policy %q", opts.ArchivePolicy)
	}
	if strings.TrimSpace(opts.StepFilter) != "" {
		if _, err := regexp.Compile(opts.StepFilter); err != nil {
			return fmt.Errorf("invalid step filter %q: %w", opts.StepFilter, err)
		}
	}
	return nil
}

func resolveProfileName(cfg *runtimeConfig, raw string) string {
	if profile := normalizeProfile(raw); profile != "" {
		return profile
	}
	if profile := normalizeProfile(cfg.Suite.Defaults.Profile); profile != "" {
		if _, ok := cfg.Suite.Profiles[profile]; ok {
			return profile
		}
	}
	return firstProfileName(cfg.Suite.Profiles)
}

func firstProfileName(profiles map[string]suiteProfileConfig) string {
	if len(profiles) == 0 {
		return ""
	}
	names := orderedProfileNames(profiles)
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

func orderedProfileNames(profiles map[string]suiteProfileConfig) []string {
	names := make([]string, 0, len(profiles))
	for profile := range profiles {
		names = append(names, profile)
	}
	sort.Strings(names)
	return names
}

func resolveProfileLaneSteps(cfg *runtimeConfig, profile string, lane string, snapshotRoot string) ([]StepSpec, error) {
	profile = resolveProfileName(cfg, profile)
	lane = normalizeLane(lane)
	profileEntry, ok := cfg.Suite.Profiles[profile]
	if !ok {
		return nil, fmt.Errorf("suite %q does not define profile %q", cfg.SuiteName, profile)
	}

	seen := map[string]struct{}{}
	appendStep := func(id string, out []StepSpec) ([]StepSpec, error) {
		id = strings.TrimSpace(id)
		if id == "" {
			return out, nil
		}
		if _, dup := seen[id]; dup {
			return out, nil
		}
		stepEntry, ok := cfg.Suite.Steps[id]
		if !ok {
			return nil, fmt.Errorf("suite %q references unknown step %q in profile %q", cfg.SuiteName, id, profile)
		}
		stepLane := normalizeStepLane(stepEntry.Lane)
		switch lane {
		case "progression", "regression":
			if stepLane != lane {
				return out, nil
			}
		case "both":
		default:
			return nil, fmt.Errorf("unsupported lane %q", lane)
		}
		workdir := stepEntry.Workdir
		if !filepath.IsAbs(workdir) {
			workdir = filepath.Join(snapshotRoot, filepath.FromSlash(workdir))
		}
		script := strings.TrimSpace(stepEntry.Script)
		if script != "" && !filepath.IsAbs(script) {
			script = filepath.Join(workdir, filepath.FromSlash(script))
		}
		out = append(out, StepSpec{
			ID:          id,
			Lane:        stepLane,
			Workdir:     workdir,
			Prereqs:     append([]string(nil), stepEntry.Prereqs...),
			Command:     stepEntry.Command,
			Script:      script,
			Args:        append([]string(nil), stepEntry.Args...),
			Description: stepEntry.Description,
		})
		seen[id] = struct{}{}
		return out, nil
	}

	var out []StepSpec
	for _, id := range profileEntry.Steps {
		var err error
		out, err = appendStep(id, out)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func filterSteps(steps []StepSpec, pattern string) ([]StepSpec, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return steps, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	out := make([]StepSpec, 0, len(steps))
	for _, step := range steps {
		if re.MatchString(step.ID) {
			out = append(out, step)
		}
	}
	return out, nil
}
