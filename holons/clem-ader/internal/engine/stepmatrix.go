package engine

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var profileDescriptions = map[string]string{
	"quick":       "Fast proof for the canonical path and short black-box coverage",
	"unit":        "Native unit suites across grace-op, SDKs, examples, and ader itself",
	"integration": "Deterministic black-box integration suite only",
	"full":        "Unit suites plus deterministic integration suite",
	"stress":      "Opt-in black-box fuzz and stress only",
}

func SupportedProfiles() []string {
	out := make([]string, 0, len(profileDescriptions))
	for profile := range profileDescriptions {
		out = append(out, profile)
	}
	sort.Strings(out)
	return out
}

func ProfileDescription(profile string) string {
	return profileDescriptions[profile]
}

func normalizeProfile(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "quick"
	}
	if value == "global" {
		return "full"
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
	profile := normalizeProfile(opts.Profile)
	if _, ok := profileDescriptions[profile]; !ok {
		return fmt.Errorf("unsupported profile %q", opts.Profile)
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

func resolveProfileLaneSteps(cfg *runtimeConfig, profile string, lane string, snapshotRoot string) ([]StepSpec, error) {
	profile = normalizeProfile(profile)
	lane = normalizeLane(lane)
	profileEntry, ok := cfg.Suite.Profiles[profile]
	if !ok {
		return nil, fmt.Errorf("suite %q does not define profile %q", cfg.SuiteName, profile)
	}

	seen := map[string]struct{}{}
	appendSteps := func(ids []string, stepLane string, out []StepSpec) ([]StepSpec, error) {
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, dup := seen[id]; dup {
				continue
			}
			stepEntry, ok := cfg.Suite.Steps[id]
			if !ok {
				return nil, fmt.Errorf("suite %q references unknown step %q in profile %q", cfg.SuiteName, id, profile)
			}
			workdir := stepEntry.Workdir
			if !filepath.IsAbs(workdir) {
				workdir = filepath.Join(snapshotRoot, filepath.FromSlash(workdir))
			}
			out = append(out, StepSpec{
				ID:          id,
				Lane:        stepLane,
				Workdir:     workdir,
				Prereqs:     append([]string(nil), stepEntry.Prereqs...),
				Command:     stepEntry.Command,
				Description: stepEntry.Description,
			})
			seen[id] = struct{}{}
		}
		return out, nil
	}

	var out []StepSpec
	var err error
	switch lane {
	case "progression":
		out, err = appendSteps(profileEntry.Progression, "progression", out)
	case "regression":
		out, err = appendSteps(profileEntry.Regression, "regression", out)
	case "both":
		out, err = appendSteps(profileEntry.Progression, "progression", out)
		if err == nil {
			out, err = appendSteps(profileEntry.Regression, "regression", out)
		}
	default:
		err = fmt.Errorf("unsupported lane %q", lane)
	}
	if err != nil {
		return nil, err
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
