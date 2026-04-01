package testrepo

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Create builds a tiny synthetic seed-like Git repository suitable for fast tests.
func Create(t testing.TB) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "README.md"), "# fixture\n")
	writeFile(t, filepath.Join(root, "state.txt"), "committed\n")
	writeFile(t, filepath.Join(root, "integration", "README.md"), "# integration\n")
	writeFile(t, filepath.Join(root, "integration", ".gitignore"), ".artifacts/\n.t\nreports/\narchives/\n")
	writeFile(t, filepath.Join(root, "integration", "ader.yaml"), `storage:
  reports: reports
  archives: archives
  artifacts: .artifacts
  temp_alias: .t
defaults:
  suite: fixture
  source: committed
  lane: regression
  profile: quick
  ladder: [quick, unit, integration, full]
  archive_policy:
    quick: never
    unit: never
    integration: never
    full: auto
    stress: never
`)
	writeFile(t, filepath.Join(root, "integration", "suites", "fixture.yaml"), `description: fixture suite
steps:
  ader-unit:
    workdir: holons/clem-ader
    prereqs: [go]
    command: go test ./...
    description: ader self test
    lane: regression
  grace-op-unit:
    workdir: holons/grace-op
    prereqs: [go]
    command: go test ./...
    description: grace-op unit tests
    lane: regression
  sdk-go-unit:
    workdir: sdk/go-holons
    prereqs: [go]
    command: go test ./...
    description: go sdk unit tests
    lane: regression
  fixture-script:
    workdir: holons/clem-ader
    script: scripts/fixture-step.sh
    args: [alpha, beta]
    description: fixture script execution
    lane: progression
  example-go-unit:
    workdir: examples/hello-world/gabriel-greeting-go
    prereqs: [go]
    command: go test ./...
    description: canonical go example unit tests
    lane: progression
  integration-short:
    workdir: integration/tests
    prereqs: [go]
    command: go test -short -count=1 ./...
    description: short black-box integration suite
    lane: progression
  integration-deterministic:
    workdir: integration/tests
    prereqs: [go]
    command: go test -count=1 ./...
    description: deterministic integration suite
    lane: regression
profiles:
  quick:
    description: Fast proof for the canonical path and short black-box coverage
    steps: [ader-unit, sdk-go-unit, example-go-unit, integration-short]
  unit:
    description: Native unit suites across grace-op, SDKs, examples, and ader itself
    steps: [ader-unit, grace-op-unit, sdk-go-unit, example-go-unit, fixture-script]
  integration:
    description: Deterministic black-box integration suite only
    steps: [integration-deterministic, integration-short]
  full:
    description: Unit suites plus deterministic integration suite
    steps: [ader-unit, grace-op-unit, sdk-go-unit, example-go-unit, integration-deterministic]
  stress:
    description: Opt-in black-box fuzz and stress only
    steps: []
`)

	writeFile(t, filepath.Join(root, "integration", "tests", "go.mod"), "module example.com/fixture/integration\n\ngo 1.25.1\n")
	writeFile(t, filepath.Join(root, "integration", "tests", "smoke_test.go"), "package integration\n\nimport \"testing\"\n\nfunc TestSmoke(t *testing.T) {}\n")

	writeFile(t, filepath.Join(root, "holons", "clem-ader", "go.mod"), "module example.com/fixture/ader\n\ngo 1.25.1\n")
	writeFile(t, filepath.Join(root, "holons", "clem-ader", "smoke_test.go"), "package ader\n\nimport \"testing\"\n\nfunc TestSmoke(t *testing.T) {}\n")
	writeFileMode(t, filepath.Join(root, "holons", "clem-ader", "scripts", "fixture-step.sh"), "#!/usr/bin/env bash\nset -euo pipefail\necho fixture-script:$1:$2\n", 0o755)

	writeFile(t, filepath.Join(root, "holons", "grace-op", "go.mod"), "module example.com/fixture/grace-op\n\ngo 1.25.1\n")
	writeFile(t, filepath.Join(root, "holons", "grace-op", "smoke_test.go"), "package graceop\n\nimport \"testing\"\n\nfunc TestSmoke(t *testing.T) {}\n")

	writeFile(t, filepath.Join(root, "sdk", "go-holons", "go.mod"), "module example.com/fixture/sdk-go\n\ngo 1.25.1\n")
	writeFile(t, filepath.Join(root, "sdk", "go-holons", "smoke_test.go"), "package goholons\n\nimport \"testing\"\n\nfunc TestSmoke(t *testing.T) {}\n")

	writeFile(t, filepath.Join(root, "examples", "hello-world", "gabriel-greeting-go", "go.mod"), "module example.com/fixture/example-go\n\ngo 1.25.1\n")
	writeFile(t, filepath.Join(root, "examples", "hello-world", "gabriel-greeting-go", "smoke_test.go"), "package greetinggo\n\nimport \"testing\"\n\nfunc TestSmoke(t *testing.T) {}\n")

	runGit(t, root, "init")
	runGit(t, root, "config", "user.name", "Codex")
	runGit(t, root, "config", "user.email", "codex@example.invalid")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "fixture")
	return root
}

func writeFile(t testing.TB, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeFileMode(t testing.TB, path string, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func runGit(t testing.TB, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, string(output))
	}
}
