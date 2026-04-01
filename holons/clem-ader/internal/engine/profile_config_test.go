package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/organic-programming/clem-ader/internal/testrepo"
)

func TestRunAcceptsCustomProfileDefinedInSuite(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")
	suitePath := filepath.Join(configDir, "suites", "fixture.yaml")
	if err := os.WriteFile(suitePath, []byte(`description: fixture suite
steps:
  fixture-script:
    workdir: holons/clem-ader
    script: scripts/fixture-step.sh
    args: [alpha, beta]
    description: fixture script execution
    lane: progression
profiles:
  my-custom-profile:
    description: Custom profile loaded from YAML
    steps: [fixture-script]
`), 0o644); err != nil {
		t.Fatalf("rewrite suite: %v", err)
	}

	withWorkingDir(t, root, func() {
		result, err := Run(context.Background(), RunOptions{
			ConfigDir:     configDir,
			Suite:         "fixture",
			Profile:       "my-custom-profile",
			Lane:          "progression",
			Source:        "workspace",
			ArchivePolicy: "never",
		})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if result.Manifest.Profile != "my-custom-profile" {
			t.Fatalf("profile = %q, want my-custom-profile", result.Manifest.Profile)
		}
	})
}

func TestResolveProfileLaneStepsUnknownProfileStillFails(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")
	cfg, err := loadRunConfig(configDir, "fixture")
	if err != nil {
		t.Fatalf("loadRunConfig() error = %v", err)
	}
	if _, err := resolveProfileLaneSteps(cfg, "missing", "regression", filepath.Join(root, "snapshot")); err == nil || !strings.Contains(err.Error(), `does not define profile "missing"`) {
		t.Fatalf("resolveProfileLaneSteps() error = %v, want unknown-profile failure", err)
	}
}

func TestResolveProfileNameUsesConfiguredDefault(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")
	aderPath := filepath.Join(configDir, "ader.yaml")
	if err := os.WriteFile(aderPath, []byte(`storage:
  reports: reports
  archives: archives
  artifacts: .artifacts
  temp_alias: .t
defaults:
  suite: fixture
  source: committed
  lane: regression
  profile: unit
  ladder: [quick, unit, integration, full]
  archive_policy:
    quick: never
    unit: never
    integration: never
    full: auto
    stress: never
`), 0o644); err != nil {
		t.Fatalf("rewrite ader.yaml: %v", err)
	}

	withWorkingDir(t, root, func() {
		result, err := Run(context.Background(), RunOptions{
			ConfigDir:     configDir,
			Suite:         "fixture",
			Lane:          "progression",
			StepFilter:    "^fixture-script$",
			Source:        "workspace",
			ArchivePolicy: "never",
		})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if result.Manifest.Profile != "unit" {
			t.Fatalf("profile = %q, want unit", result.Manifest.Profile)
		}
	})
}

func TestResolveProfileNameFallsBackToFirstProfileWhenDefaultMissing(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")
	suitePath := filepath.Join(configDir, "suites", "fixture.yaml")
	if err := os.WriteFile(suitePath, []byte(`description: fallback suite
steps:
  fixture-script:
    workdir: holons/clem-ader
    script: scripts/fixture-step.sh
    args: [alpha, beta]
    description: fixture script execution
    lane: progression
profiles:
  alpha:
    steps: [fixture-script]
  omega:
    steps: []
`), 0o644); err != nil {
		t.Fatalf("rewrite suite: %v", err)
	}

	cfg, err := loadRunConfig(configDir, "fixture")
	if err != nil {
		t.Fatalf("loadRunConfig() error = %v", err)
	}
	if got := resolveProfileName(cfg, ""); got != "alpha" {
		t.Fatalf("resolveProfileName() = %q, want alpha", got)
	}
}

func TestLoadRunConfigBackwardsCompatibleWithoutProfileOrLadder(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")
	aderPath := filepath.Join(configDir, "ader.yaml")
	if err := os.WriteFile(aderPath, []byte(`storage:
  reports: reports
  archives: archives
  artifacts: .artifacts
  temp_alias: .t
defaults:
  suite: fixture
  source: committed
  lane: regression
`), 0o644); err != nil {
		t.Fatalf("rewrite ader.yaml: %v", err)
	}

	cfg, err := loadRunConfig(configDir, "fixture")
	if err != nil {
		t.Fatalf("loadRunConfig() error = %v", err)
	}
	if cfg.Root.Defaults.Profile != "quick" {
		t.Fatalf("default profile = %q, want quick compatibility fallback", cfg.Root.Defaults.Profile)
	}
	if len(cfg.Root.Defaults.Ladder) != 0 {
		t.Fatalf("default ladder = %v, want empty compatibility fallback", cfg.Root.Defaults.Ladder)
	}
}
