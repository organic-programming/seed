package testrepo

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Create builds a tiny synthetic verification repository suitable for fast tests.
func Create(t testing.TB) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "README.md"), "# fixture\n")
	writeFile(t, filepath.Join(root, "state.txt"), "committed\n")
	writeFile(t, filepath.Join(root, "verification", "README.md"), "# verification\n")
	writeFile(t, filepath.Join(root, "verification", ".gitignore"), ".artifacts/\n.t\nreports/\narchives/\n")

	writeFile(t, filepath.Join(root, "verification", "bouquets", "local-dev.yaml"), `description: local dev bouquet
defaults:
  source: workspace
  lane: progression
  archive: never
entries:
  - catalogue: fixture
    suite: fixture
    profile: quick
  - catalogue: aux
    suite: smoke
    profile: smoke
`)

	writeFile(t, filepath.Join(root, "verification", "catalogues", "fixture", "ader.yaml"), `storage:
  reports: reports
  archives: archives
  artifacts: .artifacts
  temp_alias: .t
defaults:
  source: committed
  lane: regression
`)
	writeFile(t, filepath.Join(root, "verification", "catalogues", "fixture", "checks.yaml"), `checks:
  holons-clem-ader-unit-root:
    workdir: holons/clem-ader
    prereqs: [go]
    command: go test -v -count=1 -timeout 5m .
    description: clem-ader root package tests
  holons-grace-op-unit-root:
    workdir: holons/grace-op
    prereqs: [go]
    command: go test -v -count=1 -timeout 5m .
    description: grace-op root package tests
  sdk-go-holons-unit-root:
    workdir: sdk/go-holons
    prereqs: [go]
    command: go test -v -count=1 -timeout 5m .
    description: go-holons root package tests
  examples-hello-world-gabriel-greeting-go-unit-root:
    workdir: examples/hello-world/gabriel-greeting-go
    prereqs: [go]
    command: go test -v -count=1 -timeout 5m .
    description: gabriel greeting go root package tests
  fixture-script-check:
    workdir: holons/clem-ader
    script: scripts/fixture-step.sh
    args: [alpha, beta]
    description: fixture script execution
  quiet-script-check:
    workdir: holons/clem-ader
    script: scripts/quiet-step.sh
    description: fixture script with delayed first output
  integration-smoke:
    workdir: integration/tests
    prereqs: [go]
    command: go test -v -count=1 -timeout 5m -run '^TestSmoke$' ./...
    description: integration smoke proof
`)
	writeFile(t, filepath.Join(root, "verification", "catalogues", "fixture", "suites", "fixture.yaml"), `description: fixture suite
defaults:
  profile: quick
steps:
  holons-clem-ader-unit-root:
    check: holons-clem-ader-unit-root
    lane: regression
  holons-grace-op-unit-root:
    check: holons-grace-op-unit-root
    lane: regression
  sdk-go-holons-unit-root:
    check: sdk-go-holons-unit-root
    lane: progression
  examples-hello-world-gabriel-greeting-go-unit-root:
    check: examples-hello-world-gabriel-greeting-go-unit-root
    lane: progression
  fixture-script:
    check: fixture-script-check
    lane: progression
  quiet-script:
    check: quiet-script-check
    lane: progression
  integration-short:
    check: integration-smoke
    lane: progression
  integration-deterministic:
    check: integration-smoke
    lane: regression
profiles:
  quick:
    description: Fast proof for the canonical path and short black-box coverage
    archive: never
    steps: [fixture-script, quiet-script]
  unit:
    description: Native unit suites across grace-op, SDKs, examples, and ader itself
    archive: never
    steps:
      - holons-clem-ader-unit-root
      - holons-grace-op-unit-root
      - sdk-go-holons-unit-root
      - examples-hello-world-gabriel-greeting-go-unit-root
      - fixture-script
      - quiet-script
  integration:
    description: Deterministic black-box integration suite only
    archive: never
    steps: [integration-deterministic, integration-short]
  full:
    description: Unit suites plus deterministic integration suite
    archive: auto
    steps:
      - holons-clem-ader-unit-root
      - holons-grace-op-unit-root
      - sdk-go-holons-unit-root
      - examples-hello-world-gabriel-greeting-go-unit-root
      - fixture-script
      - quiet-script
      - integration-deterministic
      - integration-short
  stress:
    description: Opt-in black-box fuzz and stress only
    archive: never
    steps: []
`)
	writeFile(t, filepath.Join(root, "verification", "catalogues", "fixture", "suites", "reuse.yaml"), `description: reuse suite
defaults:
  profile: smoke
steps:
  fixture-script:
    check: fixture-script-check
    lane: progression
profiles:
  smoke:
    description: reused check in another scenario
    archive: never
    steps: [fixture-script]
`)

	writeFile(t, filepath.Join(root, "verification", "catalogues", "aux", "ader.yaml"), `storage:
  reports: reports
  archives: archives
  artifacts: .artifacts
  temp_alias: .t
defaults:
  source: committed
  lane: regression
`)
	writeFile(t, filepath.Join(root, "verification", "catalogues", "aux", "checks.yaml"), `checks:
  aux-script:
    workdir: holons/clem-ader
    script: scripts/fixture-step.sh
    args: [gamma, delta]
    description: aux script execution
`)
	writeFile(t, filepath.Join(root, "verification", "catalogues", "aux", "suites", "smoke.yaml"), `description: aux smoke suite
defaults:
  profile: smoke
steps:
  aux-script:
    check: aux-script
    lane: progression
profiles:
  smoke:
    description: aux smoke
    archive: never
    steps: [aux-script]
`)

	writeFile(t, filepath.Join(root, "integration", "tests", "go.mod"), "module example.com/fixture/integration\n\ngo 1.25.1\n")
	writeFile(t, filepath.Join(root, "integration", "tests", "smoke_test.go"), "package integration\n\nimport \"testing\"\n\nfunc TestSmoke(t *testing.T) {}\n")

	writeFile(t, filepath.Join(root, "holons", "clem-ader", "go.mod"), "module example.com/fixture/ader\n\ngo 1.25.1\n")
	writeFile(t, filepath.Join(root, "holons", "clem-ader", "smoke_test.go"), "package ader\n\nimport \"testing\"\n\nfunc TestSmoke(t *testing.T) {}\n")
	writeFileMode(t, filepath.Join(root, "holons", "clem-ader", "scripts", "fixture-step.sh"), "#!/usr/bin/env bash\nset -euo pipefail\necho fixture-script:$1:$2\n", 0o755)
	writeFileMode(t, filepath.Join(root, "holons", "clem-ader", "scripts", "quiet-step.sh"), "#!/usr/bin/env bash\nset -euo pipefail\nsleep \"${ADER_TEST_SILENT_STEP_SLEEP:-6}\"\necho quiet-step:done\n", 0o755)

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
