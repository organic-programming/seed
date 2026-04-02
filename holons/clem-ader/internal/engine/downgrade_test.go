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
	configDir := filepath.Join(root, "ader", "catalogues", "fixture")

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
	configDir := filepath.Join(root, "ader", "catalogues", "fixture")

	result, err := Promote(context.Background(), PromoteOptions{
		ConfigDir: configDir,
		Suite:     "fixture",
		All:       true,
	})
	if err != nil {
		t.Fatalf("Promote() error = %v", err)
	}
	want := []string{"examples-hello-world-gabriel-greeting-go-unit-root", "fixture-script", "integration-short", "quiet-script", "sdk-go-holons-unit-root"}
	if strings.Join(result.PromotedSteps, ",") != strings.Join(want, ",") {
		t.Fatalf("promoted steps = %v, want %v", result.PromotedSteps, want)
	}
}

func TestPromoteValidation(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "ader", "catalogues", "fixture")

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
	configDir := filepath.Join(root, "ader", "catalogues", "fixture")
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
	configDir := filepath.Join(root, "ader", "catalogues", "fixture")

	result, err := Downgrade(context.Background(), DowngradeOptions{
		ConfigDir: configDir,
		Suite:     "fixture",
		StepIDs:   []string{"holons-grace-op-unit-root"},
	})
	if err != nil {
		t.Fatalf("Downgrade() error = %v", err)
	}
	if strings.Join(result.DowngradedSteps, ",") != "holons-grace-op-unit-root" {
		t.Fatalf("downgraded steps = %v, want [holons-grace-op-unit-root]", result.DowngradedSteps)
	}

	cfg, err := loadRunConfig(configDir, "fixture")
	if err != nil {
		t.Fatalf("loadRunConfig() error = %v", err)
	}
	if got := normalizeStepLane(cfg.Suite.Steps["holons-grace-op-unit-root"].Lane); got != "progression" {
		t.Fatalf("holons-grace-op-unit-root lane = %q, want progression", got)
	}
}

func TestDowngradeAllRegressionSteps(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "ader", "catalogues", "fixture")

	result, err := Downgrade(context.Background(), DowngradeOptions{
		ConfigDir: configDir,
		Suite:     "fixture",
		All:       true,
	})
	if err != nil {
		t.Fatalf("Downgrade() error = %v", err)
	}
	want := []string{"holons-clem-ader-unit-root", "holons-grace-op-unit-root", "integration-deterministic"}
	if strings.Join(result.DowngradedSteps, ",") != strings.Join(want, ",") {
		t.Fatalf("downgraded steps = %v, want %v", result.DowngradedSteps, want)
	}
}

func TestDowngradeIgnoresAlreadyProgressionStep(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "ader", "catalogues", "fixture")

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
	configDir := filepath.Join(root, "ader", "catalogues", "fixture")

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
		StepIDs:   []string{"holons-grace-op-unit-root"},
	}); err == nil || !strings.Contains(err.Error(), "exactly one of --all or --step") {
		t.Fatalf("Downgrade() error = %v, want mutually exclusive validation", err)
	}
}

func TestDowngradePreservesYAMLStructureOutsideEditedSteps(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "ader", "catalogues", "fixture")
	suitePath := filepath.Join(configDir, "suites", "fixture.yaml")
	if err := os.WriteFile(suitePath, []byte(`description: fixture suite
# catalog comment
steps:
  holons-grace-op-unit-root:
    workdir: holons/grace-op
    prereqs: [go]
    command: go test -v -count=1 -timeout 5m .
    description: grace-op root package tests
    lane: regression
profiles:
  # quick profile comment
  quick:
    steps: [holons-grace-op-unit-root]
`), 0o644); err != nil {
		t.Fatalf("rewrite suite: %v", err)
	}

	if _, err := Downgrade(context.Background(), DowngradeOptions{
		ConfigDir: configDir,
		Suite:     "fixture",
		StepIDs:   []string{"holons-grace-op-unit-root"},
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
		t.Fatalf("suite missing downgraded explicit lane: %s", text)
	}
}

func TestBuildPromotionProposalIncludesPromoteCommand(t *testing.T) {
	cfg := &runtimeConfig{
		ConfigDir:    "/tmp/repo/ader/catalogues/fixture",
		ConfigRelDir: "ader/catalogues/fixture",
		RepoRoot:     "/tmp/repo",
		SuiteName:    "fixture",
		SuitePath:    "/tmp/repo/ader/catalogues/fixture/suites/fixture.yaml",
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
	if got := proposal.SuggestedCommand; got != "ader promote ader/catalogues/fixture --suite fixture --step sdk-go-unit" {
		t.Fatalf("suggested command = %q, want promote command", got)
	}
	markdown := buildPromotionMarkdown(proposal)
	if !strings.Contains(markdown, "## Apply") || !strings.Contains(markdown, "ader promote ader/catalogues/fixture --suite fixture --step sdk-go-unit") {
		t.Fatalf("promotion markdown missing promote command: %s", markdown)
	}
}

func TestPromoteIsSuiteLocalForReusedCheck(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "ader", "catalogues", "fixture")

	if _, err := Promote(context.Background(), PromoteOptions{
		ConfigDir: configDir,
		Suite:     "fixture",
		StepIDs:   []string{"fixture-script"},
	}); err != nil {
		t.Fatalf("Promote() error = %v", err)
	}

	primaryCfg, err := loadRunConfig(configDir, "fixture")
	if err != nil {
		t.Fatalf("loadRunConfig(fixture) error = %v", err)
	}
	reuseCfg, err := loadRunConfig(configDir, "reuse")
	if err != nil {
		t.Fatalf("loadRunConfig(reuse) error = %v", err)
	}
	if got := normalizeStepLane(primaryCfg.Suite.Steps["fixture-script"].Lane); got != "regression" {
		t.Fatalf("fixture suite lane = %q, want regression", got)
	}
	if got := normalizeStepLane(reuseCfg.Suite.Steps["fixture-script"].Lane); got != "progression" {
		t.Fatalf("reuse suite lane = %q, want progression", got)
	}
}
