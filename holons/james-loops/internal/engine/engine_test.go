package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/organic-programming/james-loops/internal/profile"
)

func TestScanNumberedDirsSorted(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"010", "002", "001", "alpha"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
	}

	got, err := scanNumberedDirs(root)
	if err != nil {
		t.Fatalf("scanNumberedDirs() error = %v", err)
	}
	want := []string{"001", "002", "010"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scanNumberedDirs() = %v, want %v", got, want)
	}
}

func TestParseProgram(t *testing.T) {
	dir := t.TempDir()
	writeProgramFixture(t, dir, "Fixture program")

	program, err := parseProgram(dir)
	if err != nil {
		t.Fatalf("parseProgram() error = %v", err)
	}
	if program.Description != "Fixture program" {
		t.Fatalf("program description = %q", program.Description)
	}
	if len(program.Steps) != 2 {
		t.Fatalf("steps len = %d, want 2", len(program.Steps))
	}
	if program.MaxRetries != 2 {
		t.Fatalf("max retries = %d, want 2", program.MaxRetries)
	}
}

func TestStatusRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := &Status{
		State:       "running",
		ProgramDesc: "Round trip",
		CurrentStep: "step-1",
		Branch:      "james-loops/001-round-trip",
		StartedAt:   "2026-04-02T20:00:00Z",
		FinishedAt:  "",
		Steps: map[string]StepStatus{
			"step-1": {
				State: "running",
				Attempts: []Attempt{{
					StartedAt:     "2026-04-02T20:00:00Z",
					CodexExitCode: 0,
				}},
			},
		},
	}
	if err := WriteStatus(dir, want); err != nil {
		t.Fatalf("WriteStatus() error = %v", err)
	}

	got, err := ReadStatus(dir)
	if err != nil {
		t.Fatalf("ReadStatus() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("status round trip = %#v, want %#v", got, want)
	}
}

func TestRunGateExpectations(t *testing.T) {
	root := t.TempDir()
	t.Run("expect pass", func(t *testing.T) {
		passed, reportPath, err := runGate(context.Background(), root, Gate{
			Command: `printf 'report: reports/pass.md\n'; exit 0`,
			Expect:  "PASS",
		})
		if err != nil {
			t.Fatalf("runGate() error = %v", err)
		}
		if !passed {
			t.Fatal("expected PASS gate to pass")
		}
		if reportPath != "reports/pass.md" {
			t.Fatalf("report path = %q", reportPath)
		}
	})

	t.Run("expect fail", func(t *testing.T) {
		passed, reportPath, err := runGate(context.Background(), root, Gate{
			Command: `printf 'report: reports/fail.md\n'; exit 1`,
			Expect:  "FAIL",
		})
		if err != nil {
			t.Fatalf("runGate() error = %v", err)
		}
		if !passed {
			t.Fatal("expected FAIL gate to pass on non-zero exit")
		}
		if reportPath != "reports/fail.md" {
			t.Fatalf("report path = %q", reportPath)
		}
	})
}

func TestGenerateMorningReport(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"done/001", "deferred/002"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := WriteStatus(filepath.Join(root, "done", "001"), &Status{
		State:       "done",
		ProgramDesc: "Done program",
		Branch:      "james-loops/001-done-program",
		Steps: map[string]StepStatus{
			"lint": {State: "passed", Attempts: []Attempt{{GateResult: "PASS", GateReport: "reports/lint.md"}}},
		},
	}); err != nil {
		t.Fatalf("write done status: %v", err)
	}
	if err := WriteStatus(filepath.Join(root, "deferred", "002"), &Status{
		State:       "deferred",
		ProgramDesc: "Deferred program",
		Branch:      "james-loops/002-deferred-program",
		Steps: map[string]StepStatus{
			"test": {State: "failed", Attempts: []Attempt{{GateResult: "FAIL", GateReport: "reports/test.md"}}},
		},
	}); err != nil {
		t.Fatalf("write deferred status: %v", err)
	}

	reportPath, err := GenerateMorningReport(root)
	if err != nil {
		t.Fatalf("GenerateMorningReport() error = %v", err)
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "# James Loops Morning Report") {
		t.Fatalf("report missing heading: %q", text)
	}
	if !strings.Contains(text, "| lint | passed | 1 | 1 | reports/lint.md |") {
		t.Fatalf("report missing done row: %q", text)
	}
	if !strings.Contains(text, "Last failure: step `test` report `reports/test.md`") {
		t.Fatalf("report missing deferred failure detail: %q", text)
	}

	runLogData, err := os.ReadFile(filepath.Join(root, runLogFile))
	if err != nil {
		t.Fatalf("read run log: %v", err)
	}
	runLog := string(runLogData)
	if !strings.Contains(runLog, "slot\tstep_id\tattempt\titeration\tkept\tgate_result") {
		t.Fatalf("run log missing header: %q", runLog)
	}
	if !strings.Contains(runLog, "001\tlint\t1\t0\tfalse\tPASS") {
		t.Fatalf("run log missing done attempt row: %q", runLog)
	}
}

func TestSlotManagement(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"queue", "deferred", "cookbook"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	programDir := filepath.Join(t.TempDir(), "program")
	writeProgramFixture(t, programDir, "Queue me")

	first, err := Enqueue(context.Background(), EnqueueOptions{AderRoot: root, ProgramDir: programDir})
	if err != nil {
		t.Fatalf("Enqueue() first error = %v", err)
	}
	second, err := Enqueue(context.Background(), EnqueueOptions{AderRoot: root, ProgramDir: programDir})
	if err != nil {
		t.Fatalf("Enqueue() second error = %v", err)
	}
	if first.Slot != "001" || second.Slot != "002" {
		t.Fatalf("enqueue slots = %q, %q; want 001, 002", first.Slot, second.Slot)
	}

	if _, err := Drop(context.Background(), root, "001", false); err != nil {
		t.Fatalf("Drop() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "queue", "001")); !os.IsNotExist(err) {
		t.Fatalf("queue/001 still exists, stat err = %v", err)
	}

	deferredDir := filepath.Join(root, "deferred", "009")
	if err := copyDir(programDir, deferredDir); err != nil {
		t.Fatalf("copyDir deferred: %v", err)
	}
	if err := WriteStatus(deferredDir, &Status{
		State:       "deferred",
		ProgramDesc: "Queue me",
		Steps: map[string]StepStatus{
			"step-one": {State: "failed", Attempts: []Attempt{{GateResult: "FAIL"}}},
			"step-two": {State: "pending"},
		},
	}); err != nil {
		t.Fatalf("write deferred status: %v", err)
	}

	summary, _, err := ReEnqueue(context.Background(), root, "009")
	if err != nil {
		t.Fatalf("ReEnqueue() error = %v", err)
	}
	if summary.Slot != "003" {
		t.Fatalf("re-enqueue slot = %q, want 003", summary.Slot)
	}
	status, err := ReadStatus(filepath.Join(root, "queue", "003"))
	if err != nil {
		t.Fatalf("ReadStatus(re-enqueued) error = %v", err)
	}
	if status.State != "queued" {
		t.Fatalf("re-enqueued state = %q, want queued", status.State)
	}
	if attempts := len(status.Steps["step-one"].Attempts); attempts != 0 {
		t.Fatalf("re-enqueued attempts = %d, want 0", attempts)
	}
}

func TestExecuteProgramUsesInjectedCodexAndGit(t *testing.T) {
	repoRoot := t.TempDir()
	liveDir := filepath.Join(t.TempDir(), "live")
	if err := os.MkdirAll(filepath.Join(liveDir, "briefs"), 0o755); err != nil {
		t.Fatalf("mkdir briefs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(liveDir, "briefs", "step-one.md"), []byte("do the thing"), 0o644); err != nil {
		t.Fatalf("write brief: %v", err)
	}
	program := &Program{
		Description: "Injected",
		MaxRetries:  1,
		Steps: []ProgramStep{{
			ID:    "step-one",
			Brief: "briefs/step-one.md",
			Gate: Gate{
				Command: `printf 'report: reports/step-one.md\n'; exit 0`,
				Expect:  "PASS",
			},
		}},
	}
	status := newStatus(program, "running")
	git := &mockGitOps{repoRoot: repoRoot, currentCommit: "abc123"}
	codex := &mockCodexRunner{}

	r := newRunner(git)
	r.coder = codex
	allPassed, err := r.executeProgram(context.Background(), repoRoot, liveDir, program, status, 1, false)
	if err != nil {
		t.Fatalf("executeProgram() error = %v", err)
	}
	if !allPassed {
		t.Fatal("expected program to pass")
	}
	if len(codex.prompts) != 1 || codex.prompts[0] != "do the thing" {
		t.Fatalf("codex prompts = %v, want single prompt", codex.prompts)
	}
	if len(git.commits) != 1 || git.commits[0] != "james-loops: step-one PASS (attempt 1)" {
		t.Fatalf("git commits = %v", git.commits)
	}
}

func TestExecuteProgramWaitsForQuotaWithoutBurningRetries(t *testing.T) {
	repoRoot := t.TempDir()
	liveDir := filepath.Join(t.TempDir(), "live")
	if err := os.MkdirAll(filepath.Join(liveDir, "briefs"), 0o755); err != nil {
		t.Fatalf("mkdir briefs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(liveDir, "briefs", "step-one.md"), []byte("do the thing"), 0o644); err != nil {
		t.Fatalf("write brief: %v", err)
	}
	program := &Program{
		Description: "Quota wait",
		MaxRetries:  1,
		Steps: []ProgramStep{{
			ID:    "step-one",
			Brief: "briefs/step-one.md",
			Gate: Gate{
				Command: `printf 'report: reports/step-one.md\n'; exit 0`,
				Expect:  "PASS",
			},
		}},
	}
	status := newStatus(program, "running")
	git := &mockGitOps{repoRoot: repoRoot, currentCommit: "abc123"}
	codex := &scriptedCodexRunner{
		results: []scriptedCodexResult{
			{exitCode: 1, stderr: "rate limit exceeded"},
			{exitCode: 0, stdout: "n"},
			{exitCode: 0, stdout: "y"},
			{exitCode: 0, stdout: "ok"},
		},
	}

	r := newRunner(git)
	r.coder = codex
	r.quotaProbeInterval = 0
	r.sleep = func(ctx context.Context, delay time.Duration) error { return nil }

	allPassed, err := r.executeProgram(context.Background(), repoRoot, liveDir, program, status, 1, false)
	if err != nil {
		t.Fatalf("executeProgram() error = %v", err)
	}
	if !allPassed {
		t.Fatal("expected program to pass after quota wait")
	}
	wantPrompts := []string{"do the thing", "Are you ready? Respond y / n only.", "Are you ready? Respond y / n only.", "do the thing"}
	if !reflect.DeepEqual(codex.prompts, wantPrompts) {
		t.Fatalf("codex prompts = %v, want %v", codex.prompts, wantPrompts)
	}
	if len(git.commits) != 1 || git.commits[0] != "james-loops: step-one PASS (attempt 1)" {
		t.Fatalf("git commits = %v", git.commits)
	}
	if status.State != "running" {
		t.Fatalf("status state = %q, want running", status.State)
	}
}

func TestExecuteIterationStep(t *testing.T) {
	repoRoot := t.TempDir()
	liveDir := filepath.Join(t.TempDir(), "live")
	if err := os.MkdirAll(filepath.Join(liveDir, "briefs"), 0o755); err != nil {
		t.Fatalf("mkdir briefs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(liveDir, "briefs", "simplify.md"), []byte("simplify the code"), 0o644); err != nil {
		t.Fatalf("write brief: %v", err)
	}

	program := &Program{
		Description: "Iterate",
		MaxRetries:  1,
		Steps: []ProgramStep{{
			ID:                     "simplify",
			Brief:                  "briefs/simplify.md",
			Iterations:             5,
			MaxConsecutiveFailures: 0,
			Gate: Gate{
				Command: "exit 0",
				Expect:  "PASS",
			},
		}},
	}
	status := newStatus(program, "running")

	git := &mockGitOps{repoRoot: repoRoot, currentCommit: "abc123"}
	codex := &mockCodexRunner{}
	gate := &scriptedGateRunner{
		results: []scriptedGateResult{
			{passed: true, reportPath: "reports/1.md"},
			{passed: false, reportPath: "reports/2.md"},
			{passed: true, reportPath: "reports/3.md"},
			{passed: false, reportPath: "reports/4.md"},
			{passed: true, reportPath: "reports/5.md"},
		},
	}

	r := newRunner(git)
	r.coder = codex
	r.gate = gate
	allPassed, err := r.executeProgram(context.Background(), repoRoot, liveDir, program, status, 1, false)
	if err != nil {
		t.Fatalf("executeProgram() error = %v", err)
	}
	if !allPassed {
		t.Fatal("expected iteration step to pass")
	}

	stepStatus := status.Steps["simplify"]
	if stepStatus.State != "passed" {
		t.Fatalf("step state = %q, want passed", stepStatus.State)
	}
	if stepStatus.IterationsCompleted != 3 {
		t.Fatalf("iterations completed = %d, want 3", stepStatus.IterationsCompleted)
	}
	if len(stepStatus.Attempts) != 5 {
		t.Fatalf("attempts = %d, want 5", len(stepStatus.Attempts))
	}
	wantKept := []bool{true, false, true, false, true}
	for i, attempt := range stepStatus.Attempts {
		if attempt.Iteration != i+1 {
			t.Fatalf("attempt[%d].Iteration = %d, want %d", i, attempt.Iteration, i+1)
		}
		if attempt.Kept != wantKept[i] {
			t.Fatalf("attempt[%d].Kept = %t, want %t", i, attempt.Kept, wantKept[i])
		}
	}

	// Verify commit messages
	if len(git.commits) != 3 {
		t.Fatalf("git commits = %d, want 3", len(git.commits))
	}
	wantCommits := []string{
		"james-loops: simplify iteration 1/5 PASS",
		"james-loops: simplify iteration 3/5 PASS",
		"james-loops: simplify iteration 5/5 PASS",
	}
	if !reflect.DeepEqual(git.commits, wantCommits) {
		t.Fatalf("git commits = %v, want %v", git.commits, wantCommits)
	}
	if len(git.resets) != 2 {
		t.Fatalf("git resets = %d, want 2", len(git.resets))
	}
	if len(git.savedPatches) != 2 {
		t.Fatalf("saved patches = %d, want 2", len(git.savedPatches))
	}
}

func TestExecuteIterationStepLocksAfterConsecutiveFailures(t *testing.T) {
	repoRoot := t.TempDir()
	liveDir := filepath.Join(t.TempDir(), "live")
	if err := os.MkdirAll(filepath.Join(liveDir, "briefs"), 0o755); err != nil {
		t.Fatalf("mkdir briefs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(liveDir, "briefs", "simplify.md"), []byte("simplify the code"), 0o644); err != nil {
		t.Fatalf("write brief: %v", err)
	}

	program := &Program{
		Description: "Iterate lock",
		MaxRetries:  1,
		Steps: []ProgramStep{{
			ID:                     "simplify",
			Brief:                  "briefs/simplify.md",
			Iterations:             5,
			MaxConsecutiveFailures: 2,
			Gate: Gate{
				Command: "exit 0",
				Expect:  "PASS",
			},
		}},
	}
	status := newStatus(program, "running")

	git := &mockGitOps{repoRoot: repoRoot, currentCommit: "abc123"}
	codex := &mockCodexRunner{}
	gate := &scriptedGateRunner{
		results: []scriptedGateResult{
			{passed: false, reportPath: "reports/1.md"},
			{passed: false, reportPath: "reports/2.md"},
			{passed: true, reportPath: "reports/3.md"},
		},
	}

	r := newRunner(git)
	r.coder = codex
	r.gate = gate
	allPassed, err := r.executeProgram(context.Background(), repoRoot, liveDir, program, status, 1, false)
	if err != nil {
		t.Fatalf("executeProgram() error = %v", err)
	}
	if allPassed {
		t.Fatal("expected locked iteration step to stop the program")
	}
	stepStatus := status.Steps["simplify"]
	if stepStatus.State != "locked" {
		t.Fatalf("step state = %q, want locked", stepStatus.State)
	}
	if stepStatus.IterationsCompleted != 0 {
		t.Fatalf("iterations completed = %d, want 0", stepStatus.IterationsCompleted)
	}
	if len(stepStatus.Attempts) != 2 {
		t.Fatalf("attempts = %d, want 2", len(stepStatus.Attempts))
	}
}

func TestLinearStep_Dialectic(t *testing.T) {
	repoRoot := t.TempDir()
	liveDir := filepath.Join(t.TempDir(), "live")
	writeDialecticFixtures(t, repoRoot, liveDir)
	program := &Program{
		Description: "Dialectic",
		Mode:        "dialectic",
		Profiles: ProgramProfiles{
			Coder:     "codex-default",
			Evaluator: "gemini-flash",
		},
		MaxRetries: 1,
		Steps: []ProgramStep{{
			ID:    "step-one",
			Brief: "briefs/step-one.md",
			Gate: Gate{
				Command: `printf 'report: reports/step-one.md\n'; exit 0`,
				Expect:  "PASS",
			},
			Evaluate: &EvaluateConfig{
				Brief:       "briefs/evaluate.md",
				Threshold:   6.0,
				OutputField: "score",
			},
		}},
	}
	status := newStatus(program, "running")
	git := &mockGitOps{repoRoot: repoRoot, currentCommit: "abc123", diffStat: " 1 file changed, 2 insertions(+)"}
	coder := &mockCodexRunner{}
	evaluator := &mockCodexRunner{
		runFn: func(ctx context.Context, repoRoot, prompt string) (int, []byte, []byte, error) {
			return 0, []byte(`{"score":7.5,"feedback":"Good change."}`), nil, nil
		},
	}

	r := newRunner(git)
	r.coder = coder
	r.evaluator = evaluator
	r.evaluatorProfile = &profile.Profile{
		Name:      "gemini-flash",
		Driver:    profile.DriverGemini,
		ExtraArgs: []string{"--output-format", "json"},
	}

	allPassed, err := r.executeProgram(context.Background(), repoRoot, liveDir, program, status, 1, false)
	if err != nil {
		t.Fatalf("executeProgram() error = %v", err)
	}
	if !allPassed {
		t.Fatal("expected dialectic step to pass")
	}
	attempts := status.Steps["step-one"].Attempts
	if len(attempts) != 1 {
		t.Fatalf("attempts = %d, want 1", len(attempts))
	}
	if attempts[0].EvaluatorScore != 7.5 {
		t.Fatalf("evaluator score = %v, want 7.5", attempts[0].EvaluatorScore)
	}
	if !strings.Contains(attempts[0].EvaluatorOutput, `"score":7.5`) {
		t.Fatalf("evaluator output = %q", attempts[0].EvaluatorOutput)
	}
	if len(git.commits) != 1 {
		t.Fatalf("commits = %v, want 1 commit", git.commits)
	}
}

func TestLinearStep_Dialectic_ScoreBelowThreshold(t *testing.T) {
	repoRoot := t.TempDir()
	liveDir := filepath.Join(t.TempDir(), "live")
	writeDialecticFixtures(t, repoRoot, liveDir)
	program := &Program{
		Description: "Dialectic retry",
		Mode:        "dialectic",
		Profiles: ProgramProfiles{
			Coder:     "codex-default",
			Evaluator: "gemini-flash",
		},
		MaxRetries: 2,
		Steps: []ProgramStep{{
			ID:    "step-one",
			Brief: "briefs/step-one.md",
			Gate: Gate{
				Command: `printf 'report: reports/step-one.md\n'; exit 0`,
				Expect:  "PASS",
			},
			Evaluate: &EvaluateConfig{
				Brief:       "briefs/evaluate.md",
				Threshold:   6.0,
				OutputField: "score",
			},
		}},
	}
	status := newStatus(program, "running")
	git := &mockGitOps{repoRoot: repoRoot, currentCommit: "abc123", diffStat: " 1 file changed"}
	coder := &mockCodexRunner{}
	evaluator := &scriptedCodexRunner{
		results: []scriptedCodexResult{
			{exitCode: 0, stdout: `{"score":5.0,"feedback":"Needs clearer naming."}`},
			{exitCode: 0, stdout: `{"score":8.0,"feedback":"Looks good."}`},
		},
	}

	r := newRunner(git)
	r.coder = coder
	r.evaluator = evaluator
	r.evaluatorProfile = &profile.Profile{
		Name:      "gemini-flash",
		Driver:    profile.DriverGemini,
		ExtraArgs: []string{"--output-format", "json"},
	}

	allPassed, err := r.executeProgram(context.Background(), repoRoot, liveDir, program, status, 2, false)
	if err != nil {
		t.Fatalf("executeProgram() error = %v", err)
	}
	if !allPassed {
		t.Fatal("expected second dialectic attempt to pass")
	}
	if len(coder.prompts) != 2 {
		t.Fatalf("coder prompts = %d, want 2", len(coder.prompts))
	}
	if !strings.Contains(coder.prompts[1], "Evaluator rejected the previous attempt.") {
		t.Fatalf("retry prompt = %q", coder.prompts[1])
	}
	if len(git.resets) != 1 {
		t.Fatalf("git resets = %v, want 1 reset after rejection", git.resets)
	}
	if len(git.commits) != 1 {
		t.Fatalf("git commits = %v, want final commit only", git.commits)
	}
}

func TestLinearStep_Dialectic_GateFail_NoEvaluator(t *testing.T) {
	repoRoot := t.TempDir()
	liveDir := filepath.Join(t.TempDir(), "live")
	writeDialecticFixtures(t, repoRoot, liveDir)
	program := &Program{
		Description: "Dialectic gate fail",
		Mode:        "dialectic",
		Profiles: ProgramProfiles{
			Coder:     "codex-default",
			Evaluator: "gemini-flash",
		},
		MaxRetries: 1,
		Steps: []ProgramStep{{
			ID:    "step-one",
			Brief: "briefs/step-one.md",
			Gate: Gate{
				Command: `printf 'report: reports/step-one.md\n'; exit 1`,
				Expect:  "PASS",
			},
			Evaluate: &EvaluateConfig{
				Brief:       "briefs/evaluate.md",
				Threshold:   6.0,
				OutputField: "score",
			},
		}},
	}
	status := newStatus(program, "running")
	git := &mockGitOps{repoRoot: repoRoot, currentCommit: "abc123"}
	coder := &mockCodexRunner{}
	evaluator := &mockCodexRunner{}
	gate := &scriptedGateRunner{results: []scriptedGateResult{{passed: false, reportPath: "reports/step-one.md"}}}

	r := newRunner(git)
	r.coder = coder
	r.evaluator = evaluator
	r.gate = gate

	allPassed, err := r.executeProgram(context.Background(), repoRoot, liveDir, program, status, 1, false)
	if err != nil {
		t.Fatalf("executeProgram() error = %v", err)
	}
	if allPassed {
		t.Fatal("expected gate failure to stop the step")
	}
	if len(evaluator.prompts) != 0 {
		t.Fatalf("evaluator prompts = %v, want none", evaluator.prompts)
	}
}

func TestIterationStep_Dialectic(t *testing.T) {
	repoRoot := t.TempDir()
	liveDir := filepath.Join(t.TempDir(), "live")
	writeDialecticFixtures(t, repoRoot, liveDir)
	program := &Program{
		Description: "Dialectic iteration",
		Mode:        "dialectic",
		Profiles: ProgramProfiles{
			Coder:     "codex-default",
			Evaluator: "gemini-flash",
		},
		MaxRetries: 1,
		Steps: []ProgramStep{{
			ID:         "step-one",
			Brief:      "briefs/step-one.md",
			Iterations: 2,
			Gate: Gate{
				Command: "exit 0",
				Expect:  "PASS",
			},
			Evaluate: &EvaluateConfig{
				Brief:       "briefs/evaluate.md",
				Threshold:   6.0,
				OutputField: "score",
			},
		}},
	}
	status := newStatus(program, "running")
	git := &mockGitOps{repoRoot: repoRoot, currentCommit: "abc123", diffStat: " 1 file changed"}
	coder := &mockCodexRunner{}
	evaluator := &scriptedCodexRunner{
		results: []scriptedCodexResult{
			{exitCode: 0, stdout: `{"score":5.0,"feedback":"Not enough."}`},
			{exitCode: 0, stdout: `{"score":8.0,"feedback":"Keep it."}`},
		},
	}
	gate := &scriptedGateRunner{
		results: []scriptedGateResult{
			{passed: true, reportPath: "reports/step-one.md"},
			{passed: true, reportPath: "reports/step-one.md"},
		},
	}

	r := newRunner(git)
	r.coder = coder
	r.evaluator = evaluator
	r.evaluatorProfile = &profile.Profile{
		Name:      "gemini-flash",
		Driver:    profile.DriverGemini,
		ExtraArgs: []string{"--output-format", "json"},
	}
	r.gate = gate

	allPassed, err := r.executeProgram(context.Background(), repoRoot, liveDir, program, status, 1, false)
	if err != nil {
		t.Fatalf("executeProgram() error = %v", err)
	}
	if !allPassed {
		t.Fatal("expected dialectic iteration loop to complete")
	}
	stepStatus := status.Steps["step-one"]
	if stepStatus.IterationsCompleted != 1 {
		t.Fatalf("iterations completed = %d, want 1", stepStatus.IterationsCompleted)
	}
	if len(stepStatus.Attempts) != 2 {
		t.Fatalf("attempts = %d, want 2", len(stepStatus.Attempts))
	}
	if stepStatus.Attempts[0].Kept {
		t.Fatal("first iteration should have been discarded by evaluator")
	}
	if !stepStatus.Attempts[1].Kept {
		t.Fatal("second iteration should have been kept")
	}
}

func writeProgramFixture(t *testing.T, dir string, description string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "briefs"), 0o755); err != nil {
		t.Fatalf("mkdir briefs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "briefs", "step-one.md"), []byte("step one"), 0o644); err != nil {
		t.Fatalf("write step-one: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "briefs", "step-two.md"), []byte("step two"), 0o644); err != nil {
		t.Fatalf("write step-two: %v", err)
	}
	data := strings.Join([]string{
		"description: " + description,
		"max_retries: 2",
		"steps:",
		"  - id: step-one",
		"    brief: briefs/step-one.md",
		"    gate:",
		"      command: \"printf 'report: reports/step-one.md\\n'; exit 0\"",
		"      expect: PASS",
		"  - id: step-two",
		"    brief: briefs/step-two.md",
		"    gate:",
		"      command: \"printf 'report: reports/step-two.md\\n'; exit 0\"",
		"      expect: PASS",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(dir, programFile), []byte(data), 0o644); err != nil {
		t.Fatalf("write program: %v", err)
	}
}

type mockCodexRunner struct {
	prompts     []string
	runFn       func(ctx context.Context, repoRoot, prompt string) (int, []byte, []byte, error)
	quotaFn     func(exitCode int, stdout, stderr []byte) bool
	probeFn     func(exitCode int, stdout, stderr []byte) bool
	quotaPrompt string
}

func (m *mockCodexRunner) Run(ctx context.Context, repoRoot, prompt string) (int, []byte, []byte, error) {
	m.prompts = append(m.prompts, prompt)
	if m.runFn != nil {
		return m.runFn(ctx, repoRoot, prompt)
	}
	return 0, []byte("ok"), nil, nil
}

func (m *mockCodexRunner) IsQuotaIssue(exitCode int, stdout, stderr []byte) bool {
	if m.quotaFn != nil {
		return m.quotaFn(exitCode, stdout, stderr)
	}
	return false
}

func (m *mockCodexRunner) ProbeSaysReady(exitCode int, stdout, stderr []byte) bool {
	if m.probeFn != nil {
		return m.probeFn(exitCode, stdout, stderr)
	}
	return true
}

func (m *mockCodexRunner) QuotaProbePrompt() string {
	if m.quotaPrompt != "" {
		return m.quotaPrompt
	}
	return "Are you ready? Respond y / n only."
}

type scriptedCodexResult struct {
	exitCode int
	stdout   string
	stderr   string
	err      error
}

type scriptedCodexRunner struct {
	prompts []string
	results []scriptedCodexResult
}

func (m *scriptedCodexRunner) Run(ctx context.Context, repoRoot, prompt string) (int, []byte, []byte, error) {
	m.prompts = append(m.prompts, prompt)
	if len(m.results) == 0 {
		return 0, []byte("y"), nil, nil
	}
	result := m.results[0]
	m.results = m.results[1:]
	return result.exitCode, []byte(result.stdout), []byte(result.stderr), result.err
}

func (m *scriptedCodexRunner) IsQuotaIssue(exitCode int, stdout, stderr []byte) bool {
	text := strings.ToLower(string(stdout) + "\n" + string(stderr))
	return exitCode != 0 && strings.Contains(text, "rate limit")
}

func (m *scriptedCodexRunner) ProbeSaysReady(exitCode int, stdout, stderr []byte) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(string(stdout))), "y")
}

func (m *scriptedCodexRunner) QuotaProbePrompt() string {
	return "Are you ready? Respond y / n only."
}

type scriptedGateResult struct {
	passed     bool
	reportPath string
	err        error
}

type scriptedGateRunner struct {
	results []scriptedGateResult
}

func (m *scriptedGateRunner) Run(ctx context.Context, repoRoot string, gate Gate) (bool, string, error) {
	if len(m.results) == 0 {
		return true, "reports/default.md", nil
	}
	result := m.results[0]
	m.results = m.results[1:]
	return result.passed, result.reportPath, result.err
}

type mockGitOps struct {
	repoRoot      string
	currentCommit string
	commits       []string
	resets        []string
	savedPatches  []string
	commitSeq     int
	diffStat      string
}

func (m *mockGitOps) RepoRoot() (string, error) {
	return m.repoRoot, nil
}

func (m *mockGitOps) CurrentCommit() (string, error) {
	return m.currentCommit, nil
}

func (m *mockGitOps) CheckoutNewBranch(name string) error {
	return nil
}

func (m *mockGitOps) ResetHard(commit string) error {
	m.resets = append(m.resets, commit)
	m.currentCommit = commit
	return nil
}

func (m *mockGitOps) CommitAll(message string) error {
	m.commits = append(m.commits, message)
	m.commitSeq++
	m.currentCommit = fmt.Sprintf("commit-%d", m.commitSeq)
	return nil
}

func (m *mockGitOps) SavePatch(path string) error {
	m.savedPatches = append(m.savedPatches, path)
	return os.WriteFile(path, []byte("patch"), 0o644)
}

func (m *mockGitOps) DiffStat() (string, error) {
	if m.diffStat == "" {
		return " 1 file changed", nil
	}
	return m.diffStat, nil
}

func writeDialecticFixtures(t *testing.T, repoRoot string, liveDir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(liveDir, "briefs"), 0o755); err != nil {
		t.Fatalf("mkdir briefs: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "reports"), 0o755); err != nil {
		t.Fatalf("mkdir reports: %v", err)
	}
	if err := os.WriteFile(filepath.Join(liveDir, "briefs", "step-one.md"), []byte("do the thing"), 0o644); err != nil {
		t.Fatalf("write step brief: %v", err)
	}
	if err := os.WriteFile(filepath.Join(liveDir, "briefs", "evaluate.md"), []byte("evaluate the change"), 0o644); err != nil {
		t.Fatalf("write evaluate brief: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "reports", "step-one.md"), []byte("gate passed"), 0o644); err != nil {
		t.Fatalf("write gate report: %v", err)
	}
}
