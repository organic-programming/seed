package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/organic-programming/clem-ader/internal/testrepo"
)

func TestPromoteSpecificStep(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")

	result, err := Promote(context.Background(), PromoteOptions{
		ConfigDir: configDir,
		Suite:     "fixture",
		StepIDs:   []string{"fixture-script"},
	})
	if err != nil {
		t.Fatalf("Promote() error = %v", err)
	}
	if strings.Join(result.PromotedSteps, ",") != "fixture-script" {
		t.Fatalf("promoted steps = %v, want [fixture-script]", result.PromotedSteps)
	}

	cfg, err := loadRunConfig(configDir, "fixture")
	if err != nil {
		t.Fatalf("loadRunConfig() error = %v", err)
	}
	if got := normalizeStepLane(cfg.Suite.Steps["fixture-script"].Lane); got != "regression" {
		t.Fatalf("fixture-script lane = %q, want regression", got)
	}
}

func TestPromoteAllProgressionSteps(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")

	result, err := Promote(context.Background(), PromoteOptions{
		ConfigDir: configDir,
		Suite:     "fixture",
		All:       true,
	})
	if err != nil {
		t.Fatalf("Promote() error = %v", err)
	}
	want := []string{"example-go-unit", "fixture-script", "integration-short"}
	if strings.Join(result.PromotedSteps, ",") != strings.Join(want, ",") {
		t.Fatalf("promoted steps = %v, want %v", result.PromotedSteps, want)
	}
}

func TestPromoteValidation(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")

	if _, err := Promote(context.Background(), PromoteOptions{
		ConfigDir: configDir,
		Suite:     "fixture",
	}); err == nil || !strings.Contains(err.Error(), "exactly one of --all or --step") {
		t.Fatalf("Promote() error = %v, want exact flag validation", err)
	}

	if _, err := Promote(context.Background(), PromoteOptions{
		ConfigDir: configDir,
		Suite:     "fixture",
		All:       true,
		StepIDs:   []string{"fixture-script"},
	}); err == nil || !strings.Contains(err.Error(), "exactly one of --all or --step") {
		t.Fatalf("Promote() error = %v, want mutually exclusive validation", err)
	}
}

func TestPromotePreservesYAMLStructureOutsideEditedSteps(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")
	suitePath := filepath.Join(configDir, "suites", "fixture.yaml")
	if err := os.WriteFile(suitePath, []byte(`description: fixture suite
# catalog comment
steps:
  fixture-script:
    workdir: holons/clem-ader
    script: scripts/fixture-step.sh
    args: [alpha, beta]
    description: fixture script execution
    lane: progression
profiles:
  # quick profile comment
  quick:
    steps: [fixture-script]
`), 0o644); err != nil {
		t.Fatalf("rewrite suite: %v", err)
	}

	if _, err := Promote(context.Background(), PromoteOptions{
		ConfigDir: configDir,
		Suite:     "fixture",
		StepIDs:   []string{"fixture-script"},
	}); err != nil {
		t.Fatalf("Promote() error = %v", err)
	}

	data, err := os.ReadFile(suitePath)
	if err != nil {
		t.Fatalf("read suite after promote: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "# catalog comment") {
		t.Fatalf("suite lost catalog comment: %s", text)
	}
	if !strings.Contains(text, "# quick profile comment") {
		t.Fatalf("suite lost profile comment: %s", text)
	}
	if !strings.Contains(text, "lane: regression") {
		t.Fatalf("suite missing updated lane: %s", text)
	}
}

func TestDowngradeSpecificStep(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")

	result, err := Downgrade(context.Background(), DowngradeOptions{
		ConfigDir: configDir,
		Suite:     "fixture",
		StepIDs:   []string{"grace-op-unit"},
	})
	if err != nil {
		t.Fatalf("Downgrade() error = %v", err)
	}
	if strings.Join(result.DowngradedSteps, ",") != "grace-op-unit" {
		t.Fatalf("downgraded steps = %v, want [grace-op-unit]", result.DowngradedSteps)
	}

	cfg, err := loadRunConfig(configDir, "fixture")
	if err != nil {
		t.Fatalf("loadRunConfig() error = %v", err)
	}
	if got := normalizeStepLane(cfg.Suite.Steps["grace-op-unit"].Lane); got != "progression" {
		t.Fatalf("grace-op-unit lane = %q, want progression", got)
	}
}

func TestDowngradeAllRegressionSteps(t *testing.T) {
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
	want := []string{"ader-unit", "grace-op-unit", "integration-deterministic", "sdk-go-unit"}
	if strings.Join(result.DowngradedSteps, ",") != strings.Join(want, ",") {
		t.Fatalf("downgraded steps = %v, want %v", result.DowngradedSteps, want)
	}
}

func TestDowngradeIgnoresAlreadyProgressionStep(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")

	result, err := Downgrade(context.Background(), DowngradeOptions{
		ConfigDir: configDir,
		Suite:     "fixture",
		StepIDs:   []string{"fixture-script"},
	})
	if err != nil {
		t.Fatalf("Downgrade() error = %v", err)
	}
	if len(result.DowngradedSteps) != 0 {
		t.Fatalf("downgraded steps = %v, want none", result.DowngradedSteps)
	}
	if strings.Join(result.IgnoredSteps, ",") != "fixture-script" {
		t.Fatalf("ignored steps = %v, want [fixture-script]", result.IgnoredSteps)
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
		StepIDs:   []string{"grace-op-unit"},
	}); err == nil || !strings.Contains(err.Error(), "exactly one of --all or --step") {
		t.Fatalf("Downgrade() error = %v, want mutually exclusive validation", err)
	}
}

func TestDowngradePreservesYAMLStructureOutsideEditedSteps(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")
	suitePath := filepath.Join(configDir, "suites", "fixture.yaml")
	if err := os.WriteFile(suitePath, []byte(`description: fixture suite
# catalog comment
steps:
  grace-op-unit:
    workdir: holons/grace-op
    prereqs: [go]
    command: go test ./...
    description: grace-op unit tests
    lane: regression
profiles:
  # quick profile comment
  quick:
    steps: [grace-op-unit]
`), 0o644); err != nil {
		t.Fatalf("rewrite suite: %v", err)
	}

	if _, err := Downgrade(context.Background(), DowngradeOptions{
		ConfigDir: configDir,
		Suite:     "fixture",
		StepIDs:   []string{"grace-op-unit"},
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
	if !strings.Contains(text, "lane: progression") {
		t.Fatalf("suite missing updated lane: %s", text)
	}
}

func TestBuildPromotionProposalIncludesPromoteCommand(t *testing.T) {
	cfg := &runtimeConfig{
		ConfigDir:    "/tmp/repo/integration",
		ConfigRelDir: "integration",
		RepoRoot:     "/tmp/repo",
		SuiteName:    "fixture",
		SuitePath:    "/tmp/repo/integration/suites/fixture.yaml",
		Suite: suiteConfig{
			Steps: map[string]suiteStepConfig{
				"sdk-go-unit": {Lane: "progression"},
			},
			Profiles: map[string]suiteProfileConfig{
				"unit": {Steps: []string{"sdk-go-unit"}},
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
	if got := proposal.SuggestedCommand; got != "ader promote integration --step sdk-go-unit" {
		t.Fatalf("suggested command = %q, want promote command", got)
	}
	markdown := buildPromotionMarkdown(proposal)
	if !strings.Contains(markdown, "## Apply") || !strings.Contains(markdown, "ader promote integration --step sdk-go-unit") {
		t.Fatalf("promotion markdown missing promote command: %s", markdown)
	}
}

func TestBuildPromotionProposalAddsCrossTierSuggestions(t *testing.T) {
	cfg := &runtimeConfig{
		SuiteName: "fixture",
		SuitePath: "/tmp/fixture.yaml",
		Root: rootConfig{
			Defaults: defaultsConfig{
				Ladder: []string{"quick", "unit", "integration", "full"},
			},
		},
		Suite: suiteConfig{
			Steps: map[string]suiteStepConfig{
				"sdk-go-unit": {Lane: "progression"},
			},
			Profiles: map[string]suiteProfileConfig{
				"unit":        {Steps: []string{"sdk-go-unit"}},
				"integration": {Steps: []string{}},
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
}

func TestBuildPromotionProposalHandlesLadderGracefully(t *testing.T) {
	testCases := []struct {
		name   string
		ladder []string
		want   int
	}{
		{name: "empty ladder", ladder: nil, want: 0},
		{name: "invalid entry skipped", ladder: []string{"quick", "missing", "integration"}, want: 1},
		{name: "full has no next profile", ladder: []string{"quick", "unit", "integration", "full"}, want: 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			profile := "quick"
			if tc.name == "full has no next profile" {
				profile = "full"
			}
			cfg := &runtimeConfig{
				SuiteName: "fixture",
				SuitePath: "/tmp/fixture.yaml",
				Root: rootConfig{
					Defaults: defaultsConfig{
						Ladder: tc.ladder,
					},
				},
				Suite: suiteConfig{
					Steps: map[string]suiteStepConfig{
						"sdk-go-unit": {Lane: "progression"},
					},
					Profiles: map[string]suiteProfileConfig{
						"quick":       {Steps: []string{"sdk-go-unit"}},
						"integration": {Steps: []string{}},
						"full":        {Steps: []string{"sdk-go-unit"}},
					},
				},
			}
			result := &RunResult{
				Manifest: HistoryRecord{
					Suite:   "fixture",
					Profile: profile,
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
