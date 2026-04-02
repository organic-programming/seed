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
	configDir := filepath.Join(root, "verification", "catalogues", "fixture")
	suitePath := filepath.Join(configDir, "suites", "fixture.yaml")
	if err := os.WriteFile(suitePath, []byte(`description: fixture suite
defaults:
  profile: my-custom-profile
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
    archive: never
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
	configDir := filepath.Join(root, "verification", "catalogues", "fixture")
	cfg, err := loadRunConfig(configDir, "fixture")
	if err != nil {
		t.Fatalf("loadRunConfig() error = %v", err)
	}
	if _, err := resolveProfileLaneSteps(cfg, "missing", "regression", filepath.Join(root, "snapshot")); err == nil || !strings.Contains(err.Error(), `does not define profile "missing"`) {
		t.Fatalf("resolveProfileLaneSteps() error = %v, want unknown-profile failure", err)
	}
}

func TestResolveProfileNameUsesSuiteDefault(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "verification", "catalogues", "fixture")
	suitePath := filepath.Join(configDir, "suites", "fixture.yaml")
	if err := os.WriteFile(suitePath, []byte(`description: fixture suite
defaults:
  profile: integration
steps:
  integration-short:
    check: integration-smoke
    lane: progression
profiles:
  integration:
    description: Deterministic black-box integration suite only
    archive: never
    steps: [integration-short]
`), 0o644); err != nil {
		t.Fatalf("rewrite suite: %v", err)
	}

	withWorkingDir(t, root, func() {
		result, err := Run(context.Background(), RunOptions{
			ConfigDir:     configDir,
			Suite:         "fixture",
			Lane:          "progression",
			Source:        "workspace",
			ArchivePolicy: "never",
		})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if result.Manifest.Profile != "integration" {
			t.Fatalf("profile = %q, want integration", result.Manifest.Profile)
		}
	})
}

func TestResolveProfileNameFallsBackToFirstProfileWhenSuiteDefaultMissing(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "verification", "catalogues", "fixture")
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
    archive: never
    steps: [fixture-script]
  omega:
    archive: never
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

func TestLoadRepoConfigDefaultsSourceAndLaneOnly(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "verification", "catalogues", "fixture")
	aderPath := filepath.Join(configDir, "ader.yaml")
	if err := os.WriteFile(aderPath, []byte(`storage:
  reports: reports
  archives: archives
  artifacts: .artifacts
  temp_alias: .t
defaults:
  source: committed
  lane: regression
`), 0o644); err != nil {
		t.Fatalf("rewrite ader.yaml: %v", err)
	}

	cfg, err := loadRepoConfig(configDir)
	if err != nil {
		t.Fatalf("loadRepoConfig() error = %v", err)
	}
	if cfg.Root.Defaults.Source != "committed" {
		t.Fatalf("default source = %q, want committed", cfg.Root.Defaults.Source)
	}
	if cfg.Root.Defaults.Lane != "regression" {
		t.Fatalf("default lane = %q, want regression", cfg.Root.Defaults.Lane)
	}
}
