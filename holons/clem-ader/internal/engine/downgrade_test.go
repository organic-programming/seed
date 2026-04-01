package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/organic-programming/clem-ader/internal/testrepo"
)

func TestDowngradeSpecificStepInSpecificProfile(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")

	result, err := Downgrade(context.Background(), DowngradeOptions{
		ConfigDir: configDir,
		Suite:     "fixture",
		Profile:   "unit",
		StepIDs:   []string{"sdk-go-unit"},
	})
	if err != nil {
		t.Fatalf("Downgrade() error = %v", err)
	}
	if len(result.ProfileChanges) != 1 {
		t.Fatalf("profile changes = %d, want 1", len(result.ProfileChanges))
	}
	change := result.ProfileChanges[0]
	if change.Profile != "unit" {
		t.Fatalf("profile = %q, want unit", change.Profile)
	}
	if strings.Join(change.MovedSteps, ",") != "sdk-go-unit" {
		t.Fatalf("moved steps = %v, want [sdk-go-unit]", change.MovedSteps)
	}

	cfg, err := loadRunConfig(configDir, "fixture")
	if err != nil {
		t.Fatalf("loadRunConfig() error = %v", err)
	}
	unit := cfg.Suite.Profiles["unit"]
	if containsString(unit.Regression, "sdk-go-unit") {
		t.Fatal("sdk-go-unit still present in unit regression")
	}
	if got := unit.Progression[len(unit.Progression)-1]; got != "sdk-go-unit" {
		t.Fatalf("unit progression last = %q, want sdk-go-unit", got)
	}
}

func TestDowngradeAllInSingleProfile(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")

	_, err := Downgrade(context.Background(), DowngradeOptions{
		ConfigDir: configDir,
		Suite:     "fixture",
		Profile:   "integration",
		All:       true,
	})
	if err != nil {
		t.Fatalf("Downgrade() error = %v", err)
	}

	cfg, err := loadRunConfig(configDir, "fixture")
	if err != nil {
		t.Fatalf("loadRunConfig() error = %v", err)
	}
	integration := cfg.Suite.Profiles["integration"]
	if len(integration.Regression) != 0 {
		t.Fatalf("integration regression = %v, want empty", integration.Regression)
	}
	want := []string{"integration-short", "integration-deterministic"}
	if strings.Join(integration.Progression, ",") != strings.Join(want, ",") {
		t.Fatalf("integration progression = %v, want %v", integration.Progression, want)
	}
}

func TestDowngradeAllAcrossAllProfiles(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")

	result, err := Downgrade(context.Background(), DowngradeOptions{
		ConfigDir: configDir,
		Suite:     "fixture",
		All:       true,
	})
	if err != nil {
		t.Fatalf("Downgrade() error = %v", err)
	}
	if len(result.ProfileChanges) != 4 {
		t.Fatalf("profile changes = %d, want 4", len(result.ProfileChanges))
	}

	cfg, err := loadRunConfig(configDir, "fixture")
	if err != nil {
		t.Fatalf("loadRunConfig() error = %v", err)
	}
	for _, profile := range []string{"quick", "unit", "integration", "full"} {
		if len(cfg.Suite.Profiles[profile].Regression) != 0 {
			t.Fatalf("%s regression = %v, want empty", profile, cfg.Suite.Profiles[profile].Regression)
		}
	}
}

func TestDowngradeDoesNotDuplicateExistingProgressionStep(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")

	_, err := Downgrade(context.Background(), DowngradeOptions{
		ConfigDir: configDir,
		Suite:     "fixture",
		Profile:   "quick",
		StepIDs:   []string{"ader-unit"},
	})
	if err != nil {
		t.Fatalf("Downgrade() error = %v", err)
	}

	cfg, err := loadRunConfig(configDir, "fixture")
	if err != nil {
		t.Fatalf("loadRunConfig() error = %v", err)
	}
	quick := cfg.Suite.Profiles["quick"]
	if containsString(quick.Regression, "ader-unit") {
		t.Fatal("ader-unit still present in quick regression")
	}
	count := 0
	for _, stepID := range quick.Progression {
		if stepID == "ader-unit" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("ader-unit progression count = %d, want 1", count)
	}
}

func TestDowngradeValidation(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")

	if _, err := Downgrade(context.Background(), DowngradeOptions{
		ConfigDir: configDir,
		Suite:     "fixture",
	}); err == nil || !strings.Contains(err.Error(), "exactly one of --all or --step") {
		t.Fatalf("Downgrade() error = %v, want exact flag validation", err)
	}

	if _, err := Downgrade(context.Background(), DowngradeOptions{
		ConfigDir: configDir,
		Suite:     "fixture",
		All:       true,
		StepIDs:   []string{"sdk-go-unit"},
	}); err == nil || !strings.Contains(err.Error(), "exactly one of --all or --step") {
		t.Fatalf("Downgrade() error = %v, want mutually exclusive validation", err)
	}
}

func TestDowngradePreservesYAMLStructureOutsideEditedLanes(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")
	suitePath := filepath.Join(configDir, "suites", "fixture.yaml")
	if err := os.WriteFile(suitePath, []byte(`description: fixture suite
# catalog comment
steps:
  sdk-go-unit:
    workdir: sdk/go-holons
    prereqs: [go]
    command: go test ./...
    description: go sdk unit tests
profiles:
  # quick profile comment
  quick:
    regression: [sdk-go-unit]
    progression: []
`), 0o644); err != nil {
		t.Fatalf("rewrite suite: %v", err)
	}

	if _, err := Downgrade(context.Background(), DowngradeOptions{
		ConfigDir: configDir,
		Suite:     "fixture",
		Profile:   "quick",
		All:       true,
	}); err != nil {
		t.Fatalf("Downgrade() error = %v", err)
	}

	data, err := os.ReadFile(suitePath)
	if err != nil {
		t.Fatalf("read suite after downgrade: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "# catalog comment") {
		t.Fatalf("suite lost catalog comment: %s", text)
	}
	if !strings.Contains(text, "# quick profile comment") {
		t.Fatalf("suite lost profile comment: %s", text)
	}
	if strings.Index(text, "steps:") > strings.Index(text, "profiles:") {
		t.Fatalf("suite reordered top-level keys unexpectedly: %s", text)
	}
}

func TestBuildPromotionProposalAddsCrossTierSuggestions(t *testing.T) {
	cfg := &runtimeConfig{
		SuiteName: "fixture",
		SuitePath: "/tmp/fixture.yaml",
		Suite: suiteConfig{
			Profiles: map[string]suiteProfileLanes{
				"unit": {
					Regression:  []string{},
					Progression: []string{"sdk-go-unit"},
				},
				"integration": {
					Regression:  []string{},
					Progression: []string{},
				},
			},
		},
	}
	result := &RunResult{
		Manifest: HistoryRecord{
			Suite:   "fixture",
			Profile: "unit",
			Lane:    "progression",
		},
		Steps: []StepResult{
			{StepID: "sdk-go-unit", Lane: "progression", Status: "PASS"},
		},
	}

	proposal := buildPromotionProposal(cfg, result)
	if proposal == nil {
		t.Fatal("buildPromotionProposal() = nil, want proposal")
	}
	if len(proposal.CrossTierSuggestions) != 1 {
		t.Fatalf("cross-tier suggestions = %d, want 1", len(proposal.CrossTierSuggestions))
	}
	suggestion := proposal.CrossTierSuggestions[0]
	if suggestion.FromProfile != "unit" || suggestion.ToProfile != "integration" || suggestion.ToLane != "progression" {
		t.Fatalf("suggestion = %+v, want unit -> integration progression", suggestion)
	}
	if !strings.Contains(buildPromotionMarkdown(proposal), "## Cross-Profile Suggestions") {
		t.Fatal("promotion markdown missing cross-profile section")
	}
}

func TestBuildPromotionProposalSkipsExistingOrOutOfLadderSuggestions(t *testing.T) {
	testCases := []struct {
		name    string
		profile string
		next    suiteProfileLanes
		want    int
	}{
		{
			name:    "existing in next profile",
			profile: "quick",
			next: suiteProfileLanes{
				Regression: []string{"sdk-go-unit"},
			},
			want: 0,
		},
		{
			name:    "full has no next profile",
			profile: "full",
			next:    suiteProfileLanes{},
			want:    0,
		},
		{
			name:    "stress is outside ladder",
			profile: "stress",
			next:    suiteProfileLanes{},
			want:    0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			profiles := map[string]suiteProfileLanes{
				tc.profile: {
					Regression:  []string{},
					Progression: []string{"sdk-go-unit"},
				},
			}
			if nextProfile, ok := nextProfileInLadder(tc.profile); ok {
				profiles[nextProfile] = tc.next
			}
			cfg := &runtimeConfig{
				SuiteName: "fixture",
				SuitePath: "/tmp/fixture.yaml",
				Suite:     suiteConfig{Profiles: profiles},
			}
			result := &RunResult{
				Manifest: HistoryRecord{
					Suite:   "fixture",
					Profile: tc.profile,
					Lane:    "progression",
				},
				Steps: []StepResult{
					{StepID: "sdk-go-unit", Lane: "progression", Status: "PASS"},
				},
			}

			proposal := buildPromotionProposal(cfg, result)
			if proposal == nil {
				t.Fatal("buildPromotionProposal() = nil, want proposal")
			}
			if len(proposal.CrossTierSuggestions) != tc.want {
				t.Fatalf("cross-tier suggestions = %d, want %d", len(proposal.CrossTierSuggestions), tc.want)
			}
		})
	}
}
