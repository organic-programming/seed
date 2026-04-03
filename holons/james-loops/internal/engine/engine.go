package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/organic-programming/james-loops/internal/profile"
	runnerpkg "github.com/organic-programming/james-loops/internal/runner"
	"gopkg.in/yaml.v3"
)

const (
	defaultMaxRetries      = 3
	defaultQuotaProbeDelay = time.Minute
	programFile            = "program.yaml"
	queueDirName           = "queue"
	liveDirName            = "live"
	deferredDirName        = "deferred"
	doneDirName            = "done"
	cookbookDirName        = "cookbook"
)

type runner struct {
	coder              runnerpkg.AIRunner
	evaluator          runnerpkg.AIRunner
	coderProfile       profile.Profile
	evaluatorProfile   *profile.Profile
	gate               GateRunner
	git                GitOps
	quotaProbeInterval time.Duration
	sleep              func(context.Context, time.Duration) error
}

func newRunner(git GitOps) runner {
	return runner{
		gate:               shellGateRunner{},
		git:                git,
		quotaProbeInterval: defaultQuotaProbeDelay,
		sleep:              sleepContext,
	}
}

func newRunnerFromProfiles(base runner, coderProfile profile.Profile, evaluatorProfile *profile.Profile) (runner, error) {
	coder, err := runnerpkg.New(coderProfile)
	if err != nil {
		return runner{}, err
	}
	base.coder = coder
	base.coderProfile = coderProfile
	base.evaluator = nil
	base.evaluatorProfile = nil
	if evaluatorProfile == nil {
		return base, nil
	}
	evaluator, err := runnerpkg.New(*evaluatorProfile)
	if err != nil {
		return runner{}, err
	}
	base.evaluator = evaluator
	copyProfile := *evaluatorProfile
	base.evaluatorProfile = &copyProfile
	return base, nil
}

func Run(ctx context.Context, opts RunOptions) error {
	return newRunner(newShellGitOps("")).Run(ctx, opts)
}

func Enqueue(ctx context.Context, opts EnqueueOptions) (SlotSummary, error) {
	_ = ctx
	aderRoot, err := resolveAderRoot(opts.AderRoot, "")
	if err != nil {
		return SlotSummary{}, err
	}
	sourceDir, err := resolveProgramSource(aderRoot, opts)
	if err != nil {
		return SlotSummary{}, err
	}
	program, err := parseProgram(sourceDir)
	if err != nil {
		return SlotSummary{}, err
	}
	slot, err := nextSlotNumber(filepath.Join(aderRoot, queueDirName))
	if err != nil {
		return SlotSummary{}, err
	}
	targetDir := filepath.Join(aderRoot, queueDirName, slot)
	if err := copyDir(sourceDir, targetDir); err != nil {
		return SlotSummary{}, err
	}
	status := newStatus(program, "queued")
	if err := WriteStatus(targetDir, status); err != nil {
		return SlotSummary{}, err
	}
	return summarizeSlot(slot, targetDir)
}

func List(ctx context.Context, aderRoot string) (*StatusResult, error) {
	_ = ctx
	root, err := resolveAderRoot(aderRoot, "")
	if err != nil {
		return nil, err
	}
	return collectStatusResult(root)
}

func ReadMorningReport(ctx context.Context, aderRoot string) (string, string, error) {
	_ = ctx
	root, err := resolveAderRoot(aderRoot, "")
	if err != nil {
		return "", "", err
	}
	path, err := GenerateMorningReport(root)
	if err != nil {
		return "", "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("read %s: %w", path, err)
	}
	return path, string(data), nil
}

func Drop(ctx context.Context, aderRoot string, slot string, deferred bool) (string, error) {
	_ = ctx
	root, err := resolveAderRoot(aderRoot, "")
	if err != nil {
		return "", err
	}
	dirName := queueDirName
	if deferred {
		dirName = deferredDirName
	}
	path := filepath.Join(root, dirName, slot)
	if err := os.RemoveAll(path); err != nil {
		return "", fmt.Errorf("remove %s: %w", path, err)
	}
	return path, nil
}

func Resume(ctx context.Context, aderRoot string, maxRetries int) error {
	return newRunner(newShellGitOps("")).Resume(ctx, aderRoot, maxRetries)
}

func Skip(ctx context.Context, aderRoot string, maxRetries int) (string, string, error) {
	return newRunner(newShellGitOps("")).Skip(ctx, aderRoot, maxRetries)
}

func Abort(ctx context.Context, aderRoot string) (string, error) {
	_ = ctx
	root, err := resolveAderRoot(aderRoot, "")
	if err != nil {
		return "", err
	}
	liveDir := filepath.Join(root, liveDirName)
	status, err := ReadStatus(liveDir)
	if err != nil {
		return "", err
	}
	status.State = "deferred"
	status.FinishedAt = nowRFC3339()
	if err := WriteStatus(liveDir, status); err != nil {
		return "", err
	}
	slot, err := nextSlotNumber(filepath.Join(root, deferredDirName))
	if err != nil {
		return "", err
	}
	target := filepath.Join(root, deferredDirName, slot)
	if err := moveDir(liveDir, target); err != nil {
		return "", err
	}
	return slot, nil
}

func ReEnqueue(ctx context.Context, aderRoot string, slot string) (SlotSummary, string, error) {
	_ = ctx
	root, err := resolveAderRoot(aderRoot, "")
	if err != nil {
		return SlotSummary{}, "", err
	}
	sourceDir := filepath.Join(root, deferredDirName, slot)
	program, err := parseProgram(sourceDir)
	if err != nil {
		return SlotSummary{}, "", err
	}
	nextSlot, err := nextSlotNumber(filepath.Join(root, queueDirName))
	if err != nil {
		return SlotSummary{}, "", err
	}
	targetDir := filepath.Join(root, queueDirName, nextSlot)
	if err := moveDir(sourceDir, targetDir); err != nil {
		return SlotSummary{}, "", err
	}
	if err := WriteStatus(targetDir, newStatus(program, "queued")); err != nil {
		return SlotSummary{}, "", err
	}
	summary, err := summarizeSlot(nextSlot, targetDir)
	if err != nil {
		return SlotSummary{}, "", err
	}
	return summary, targetDir, nil
}

func Log(ctx context.Context, aderRoot string, stepID string) (*LogResult, error) {
	_ = ctx
	root, err := resolveAderRoot(aderRoot, "")
	if err != nil {
		return nil, err
	}
	type candidate struct {
		slot string
		dir  string
	}
	var candidates []candidate
	liveDir := filepath.Join(root, liveDirName)
	if _, err := os.Stat(filepath.Join(liveDir, statusFile)); err == nil {
		candidates = append(candidates, candidate{slot: inferLiveSlotFromDir(liveDir), dir: liveDir})
	}
	for _, stateDir := range []string{doneDirName, deferredDirName, queueDirName} {
		slots, err := scanNumberedDirs(filepath.Join(root, stateDir))
		if err != nil {
			continue
		}
		sort.Sort(sort.Reverse(sort.StringSlice(slots)))
		for _, slot := range slots {
			candidates = append(candidates, candidate{
				slot: slot,
				dir:  filepath.Join(root, stateDir, slot),
			})
		}
	}
	for _, item := range candidates {
		status, err := ReadStatus(item.dir)
		if err != nil {
			continue
		}
		step, ok := status.Steps[stepID]
		if !ok {
			continue
		}
		return &LogResult{
			Slot:     item.slot,
			StepID:   stepID,
			Attempts: append([]Attempt(nil), step.Attempts...),
		}, nil
	}
	return nil, fmt.Errorf("step %q not found", stepID)
}

func ListCookbookTemplates(aderRoot string) ([]CompletionItem, error) {
	root, err := resolveAderRoot(aderRoot, "")
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(root, cookbookDirName))
	if err != nil {
		return nil, err
	}
	items := make([]CompletionItem, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		items = append(items, CompletionItem{Value: entry.Name()})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Value < items[j].Value })
	return items, nil
}

func ListSlotsForState(aderRoot string, dirName string) ([]CompletionItem, error) {
	root, err := resolveAderRoot(aderRoot, "")
	if err != nil {
		return nil, err
	}
	slots, err := scanNumberedDirs(filepath.Join(root, dirName))
	if err != nil {
		return nil, err
	}
	items := make([]CompletionItem, 0, len(slots))
	for _, slot := range slots {
		items = append(items, CompletionItem{Value: slot})
	}
	return items, nil
}

func ListLogStepIDs(aderRoot string) ([]CompletionItem, error) {
	root, err := resolveAderRoot(aderRoot, "")
	if err != nil {
		return nil, err
	}
	programPaths := []string{filepath.Join(root, liveDirName, programFile)}
	if latestDone, ok := latestSlotPath(filepath.Join(root, doneDirName)); ok {
		programPaths = append(programPaths, filepath.Join(latestDone, programFile))
	}
	for _, programPath := range programPaths {
		data, err := os.ReadFile(programPath)
		if err != nil {
			continue
		}
		var program Program
		if err := yaml.Unmarshal(data, &program); err != nil {
			continue
		}
		items := make([]CompletionItem, 0, len(program.Steps))
		for _, step := range program.Steps {
			items = append(items, CompletionItem{Value: step.ID})
		}
		return items, nil
	}
	return nil, nil
}

func DryRunPlan(ctx context.Context, aderRoot string) ([]SlotSummary, error) {
	_ = ctx
	root, err := resolveAderRoot(aderRoot, "")
	if err != nil {
		return nil, err
	}
	return collectSlotSummaries(filepath.Join(root, queueDirName))
}

func (r runner) Run(ctx context.Context, opts RunOptions) error {
	repoRoot, err := r.git.RepoRoot()
	if err != nil {
		return err
	}
	aderRoot, err := resolveAderRoot(opts.AderRoot, repoRoot)
	if err != nil {
		return err
	}
	if opts.DryRun {
		_, err := DryRunPlan(ctx, aderRoot)
		return err
	}
	liveDir := filepath.Join(aderRoot, liveDirName)
	if liveBusy(liveDir) {
		return fmt.Errorf("live program already present in %s", liveDir)
	}

	queueDir := filepath.Join(aderRoot, queueDirName)
	for {
		slots, err := scanNumberedDirs(queueDir)
		if err != nil {
			return err
		}
		if len(slots) == 0 {
			break
		}
		slot := slots[0]
		if err := activateSlot(aderRoot, slot); err != nil {
			return err
		}
		program, status, err := loadLiveProgram(liveDir)
		if err != nil {
			return err
		}
		coderProfile, err := resolveCoderProfile(program, status, opts.CoderProfile)
		if err != nil {
			return err
		}
		var evaluatorProfile *profile.Profile
		if programMode(program) == "dialectic" {
			resolved, err := resolveEvaluatorProfile(program, status, opts.EvaluatorProfile)
			if err != nil {
				return err
			}
			evaluatorProfile = &resolved
		}
		activeRunner, err := newRunnerFromProfiles(r, coderProfile, evaluatorProfile)
		if err != nil {
			return err
		}
		if strings.TrimSpace(status.Branch) == "" {
			branch := fmt.Sprintf("james-loops/%s-%s", slot, slugify(program.Description))
			if err := activeRunner.git.CheckoutNewBranch(branch); err != nil {
				return err
			}
			status.Branch = branch
		}
		if strings.TrimSpace(status.StartedAt) == "" {
			status.StartedAt = nowRFC3339()
		}
		status.State = "running"
		status.ProgramDesc = program.Description
		status.CoderProfile = coderProfile.Name
		if evaluatorProfile != nil {
			status.EvaluatorProfile = evaluatorProfile.Name
		} else {
			status.EvaluatorProfile = ""
		}
		if err := WriteStatus(liveDir, status); err != nil {
			return err
		}
		allPassed, err := activeRunner.executeProgram(ctx, repoRoot, liveDir, program, status, resolveMaxRetries(opts.MaxRetries, program.MaxRetries), false)
		if err != nil {
			return err
		}
		if allPassed {
			status.State = "done"
			status.CurrentStep = ""
			status.FinishedAt = nowRFC3339()
			if err := WriteStatus(liveDir, status); err != nil {
				return err
			}
			if err := moveDir(liveDir, filepath.Join(aderRoot, doneDirName, slot)); err != nil {
				return err
			}
			continue
		}
		status.State = "deferred"
		status.FinishedAt = nowRFC3339()
		if err := WriteStatus(liveDir, status); err != nil {
			return err
		}
		deferredSlot, err := nextSlotNumber(filepath.Join(aderRoot, deferredDirName))
		if err != nil {
			return err
		}
		if err := moveDir(liveDir, filepath.Join(aderRoot, deferredDirName, deferredSlot)); err != nil {
			return err
		}
	}
	_, err = GenerateMorningReport(aderRoot)
	return err
}

func (r runner) Resume(ctx context.Context, aderRoot string, maxRetries int) error {
	repoRoot, err := r.git.RepoRoot()
	if err != nil {
		return err
	}
	root, err := resolveAderRoot(aderRoot, repoRoot)
	if err != nil {
		return err
	}
	liveDir := filepath.Join(root, liveDirName)
	program, status, err := loadLiveProgram(liveDir)
	if err != nil {
		return err
	}
	coderProfile, err := resolveCoderProfile(program, status, "")
	if err != nil {
		return err
	}
	var evaluatorProfile *profile.Profile
	if programMode(program) == "dialectic" {
		resolved, err := resolveEvaluatorProfile(program, status, "")
		if err != nil {
			return err
		}
		evaluatorProfile = &resolved
	}
	activeRunner, err := newRunnerFromProfiles(r, coderProfile, evaluatorProfile)
	if err != nil {
		return err
	}
	if status.State != "deferred" {
		return fmt.Errorf("live program is not deferred")
	}
	status.State = "running"
	status.FinishedAt = ""
	status.CoderProfile = coderProfile.Name
	if evaluatorProfile != nil {
		status.EvaluatorProfile = evaluatorProfile.Name
	}
	if err := WriteStatus(liveDir, status); err != nil {
		return err
	}
	allPassed, err := activeRunner.executeProgram(ctx, repoRoot, liveDir, program, status, resolveMaxRetries(maxRetries, program.MaxRetries), false)
	if err != nil {
		return err
	}
	if allPassed {
		status.State = "done"
		status.CurrentStep = ""
		status.FinishedAt = nowRFC3339()
		if err := WriteStatus(liveDir, status); err != nil {
			return err
		}
		doneSlot := inferLiveSlot(status)
		if !slotPattern.MatchString(doneSlot) {
			doneSlot, err = nextSlotNumber(filepath.Join(root, doneDirName))
			if err != nil {
				return err
			}
		}
		if err := moveDir(liveDir, filepath.Join(root, doneDirName, doneSlot)); err != nil {
			return err
		}
	} else {
		status.State = "deferred"
		status.FinishedAt = nowRFC3339()
		if err := WriteStatus(liveDir, status); err != nil {
			return err
		}
	}
	_, err = GenerateMorningReport(root)
	return err
}

func (r runner) Skip(ctx context.Context, aderRoot string, maxRetries int) (string, string, error) {
	repoRoot, err := r.git.RepoRoot()
	if err != nil {
		return "", "", err
	}
	root, err := resolveAderRoot(aderRoot, repoRoot)
	if err != nil {
		return "", "", err
	}
	liveDir := filepath.Join(root, liveDirName)
	program, status, err := loadLiveProgram(liveDir)
	if err != nil {
		return "", "", err
	}
	coderProfile, err := resolveCoderProfile(program, status, "")
	if err != nil {
		return "", "", err
	}
	var evaluatorProfile *profile.Profile
	if programMode(program) == "dialectic" {
		resolved, err := resolveEvaluatorProfile(program, status, "")
		if err != nil {
			return "", "", err
		}
		evaluatorProfile = &resolved
	}
	activeRunner, err := newRunnerFromProfiles(r, coderProfile, evaluatorProfile)
	if err != nil {
		return "", "", err
	}
	currentIndex := stepIndex(program, status.CurrentStep)
	if currentIndex < 0 {
		currentIndex = firstPendingStep(program, status)
	}
	if currentIndex < 0 {
		return "", "", fmt.Errorf("no current step to skip")
	}
	skippedStep := program.Steps[currentIndex].ID
	stepStatus := ensureStepStatus(status, skippedStep)
	stepStatus.State = "skipped"
	status.Steps[skippedStep] = stepStatus
	status.State = "running"
	nextStep := ""
	if currentIndex+1 < len(program.Steps) {
		nextStep = program.Steps[currentIndex+1].ID
	}
	status.CurrentStep = nextStep
	status.CoderProfile = coderProfile.Name
	if evaluatorProfile != nil {
		status.EvaluatorProfile = evaluatorProfile.Name
	}
	if err := WriteStatus(liveDir, status); err != nil {
		return "", "", err
	}
	allPassed, err := activeRunner.executeProgram(ctx, repoRoot, liveDir, program, status, resolveMaxRetries(maxRetries, program.MaxRetries), false)
	if err != nil {
		return skippedStep, nextStep, err
	}
	if allPassed {
		status.State = "done"
		status.CurrentStep = ""
		status.FinishedAt = nowRFC3339()
		if err := WriteStatus(liveDir, status); err != nil {
			return skippedStep, nextStep, err
		}
		doneSlot := inferLiveSlot(status)
		if !slotPattern.MatchString(doneSlot) {
			doneSlot, err = nextSlotNumber(filepath.Join(root, doneDirName))
			if err != nil {
				return skippedStep, nextStep, err
			}
		}
		if err := moveDir(liveDir, filepath.Join(root, doneDirName, doneSlot)); err != nil {
			return skippedStep, nextStep, err
		}
	} else {
		status.State = "deferred"
		status.FinishedAt = nowRFC3339()
		if err := WriteStatus(liveDir, status); err != nil {
			return skippedStep, nextStep, err
		}
	}
	if _, err := GenerateMorningReport(root); err != nil {
		return skippedStep, nextStep, err
	}
	return skippedStep, nextStep, nil
}

func (r runner) executeProgram(ctx context.Context, repoRoot string, liveDir string, program *Program, status *Status, maxRetries int, skipCurrent bool) (bool, error) {
	startIndex := firstPendingStep(program, status)
	if strings.TrimSpace(status.CurrentStep) != "" {
		if idx := stepIndex(program, status.CurrentStep); idx >= 0 {
			startIndex = idx
		}
	}
	if startIndex < 0 {
		return true, nil
	}
	if skipCurrent {
		startIndex++
	}
	for i := startIndex; i < len(program.Steps); i++ {
		step := program.Steps[i]
		stepStatus := ensureStepStatus(status, step.ID)
		if stepStatus.State == "passed" || stepStatus.State == "skipped" {
			continue
		}

		status.State = "running"
		status.CurrentStep = step.ID
		stepStatus.State = "running"
		status.Steps[step.ID] = stepStatus
		if err := WriteStatus(liveDir, status); err != nil {
			return false, err
		}

		var passed bool
		var err error
		if step.Iterations > 1 {
			passed, err = r.executeIterationStep(ctx, repoRoot, liveDir, step, status, &stepStatus)
		} else {
			passed, err = r.executeLinearStep(ctx, repoRoot, liveDir, step, status, &stepStatus, maxRetries)
		}
		if err != nil {
			return false, err
		}
		status.Steps[step.ID] = stepStatus
		if err := WriteStatus(liveDir, status); err != nil {
			return false, err
		}
		if !passed {
			status.State = "deferred"
			status.CurrentStep = step.ID
			if err := WriteStatus(liveDir, status); err != nil {
				return false, err
			}
			return false, nil
		}
	}
	status.CurrentStep = ""
	if err := WriteStatus(liveDir, status); err != nil {
		return false, err
	}
	return true, nil
}

// executeLinearStep is the original retry-on-failure loop: try up to maxRetries,
// stop on first PASS, defer on exhaustion.
func (r runner) executeLinearStep(ctx context.Context, repoRoot, liveDir string, step ProgramStep, status *Status, stepStatus *StepStatus, maxRetries int) (bool, error) {
	startCommit, err := r.git.CurrentCommit()
	if err != nil {
		return false, err
	}
	retryContext := ""
	for attemptNo := 1; attemptNo <= maxRetries; {
		attempt := Attempt{StartedAt: nowRFC3339()}
		briefPath := filepath.Join(liveDir, filepath.Clean(step.Brief))
		prompt, err := loadPrompt(briefPath, retryContext)
		if err != nil {
			return false, err
		}
		exitCode, stdout, stderr, err := r.coder.Run(ctx, repoRoot, prompt)
		if err != nil {
			return false, err
		}
		if r.coder.IsQuotaIssue(exitCode, stdout, stderr) {
			if err := r.waitForQuotaRecovery(ctx, repoRoot, liveDir, status, step.ID); err != nil {
				return false, err
			}
			continue
		}
		attempt.CodexExitCode = exitCode

		gatePassed, reportPath, err := r.gate.Run(ctx, repoRoot, step.Gate)
		if err != nil {
			return false, err
		}
		attempt.GateReport = reportPath
		if gatePassed {
			attempt.GateResult = "PASS"
			if r.evaluator != nil && step.Evaluate != nil {
				briefContent, err := os.ReadFile(briefPath)
				if err != nil {
					return false, fmt.Errorf("read brief %s: %w", briefPath, err)
				}
				keepCommit, feedback, score, err := r.executeEvaluation(
					ctx,
					repoRoot,
					liveDir,
					step,
					strings.TrimSpace(string(briefContent)),
					reportPath,
					&attempt,
				)
				if err != nil {
					return false, err
				}
				attempt.EvaluatorScore = score
				attempt.FinishedAt = nowRFC3339()
				if keepCommit {
					stepStatus.State = "passed"
					stepStatus.Attempts = append(stepStatus.Attempts, attempt)
					status.Steps[step.ID] = *stepStatus
					if err := r.git.CommitAll(fmt.Sprintf("james-loops: %s PASS (attempt %d)", step.ID, attemptNo)); err != nil {
						return false, err
					}
					if err := WriteStatus(liveDir, status); err != nil {
						return false, err
					}
					return true, nil
				}
				if err := r.git.ResetHard(startCommit); err != nil {
					return false, err
				}
				stepStatus.State = "failed"
				stepStatus.Attempts = append(stepStatus.Attempts, attempt)
				status.Steps[step.ID] = *stepStatus
				if err := WriteStatus(liveDir, status); err != nil {
					return false, err
				}
				retryContext = buildEvaluationRetryContext(feedback, attempt.EvaluatorOutput, score, step.Evaluate.Threshold)
				attemptNo++
				continue
			}
			attempt.FinishedAt = nowRFC3339()
			stepStatus.State = "passed"
			stepStatus.Attempts = append(stepStatus.Attempts, attempt)
			status.Steps[step.ID] = *stepStatus
			if err := r.git.CommitAll(fmt.Sprintf("james-loops: %s PASS (attempt %d)", step.ID, attemptNo)); err != nil {
				return false, err
			}
			if err := WriteStatus(liveDir, status); err != nil {
				return false, err
			}
			return true, nil
		}

		attempt.GateResult = "FAIL"
		attempt.FinishedAt = nowRFC3339()
		patchPath := filepath.Join(liveDir, fmt.Sprintf("%s-attempt-%d.patch", step.ID, attemptNo))
		if err := r.git.SavePatch(patchPath); err != nil {
			return false, err
		}
		attempt.DiffPatch = patchPath
		if err := r.git.ResetHard(startCommit); err != nil {
			return false, err
		}
		stepStatus.State = "failed"
		stepStatus.Attempts = append(stepStatus.Attempts, attempt)
		status.Steps[step.ID] = *stepStatus
		if err := WriteStatus(liveDir, status); err != nil {
			return false, err
		}
		retryContext = readRetryContext(repoRoot, reportPath)
		attemptNo++
	}
	return false, nil
}

// executeIterationStep is the autoresearch loop: run the step N times,
// keep gate-passing commits, discard regressions. Lock after max consecutive failures.
func (r runner) executeIterationStep(ctx context.Context, repoRoot, liveDir string, step ProgramStep, status *Status, stepStatus *StepStatus) (bool, error) {
	iterations := step.Iterations
	maxConsecFail := step.MaxConsecutiveFailures
	keptCount := 0
	consecutiveFailures := 0
	lastChange := "baseline"

	for iter := 1; iter <= iterations; iter++ {
		iterCommit, err := r.git.CurrentCommit()
		if err != nil {
			return false, err
		}
		attempt := Attempt{StartedAt: nowRFC3339(), Iteration: iter}
		briefPath := filepath.Join(liveDir, filepath.Clean(step.Brief))

		prompt, err := loadPrompt(
			briefPath,
			iterationContext(iter, iterations, keptCount, lastChange),
		)
		if err != nil {
			return false, err
		}
		exitCode, stdout, stderr, err := r.coder.Run(ctx, repoRoot, prompt)
		if err != nil {
			return false, err
		}
		if r.coder.IsQuotaIssue(exitCode, stdout, stderr) {
			if err := r.waitForQuotaRecovery(ctx, repoRoot, liveDir, status, step.ID); err != nil {
				return false, err
			}
			iter-- // retry same iteration after quota recovery
			continue
		}
		attempt.CodexExitCode = exitCode

		gatePassed, reportPath, err := r.gate.Run(ctx, repoRoot, step.Gate)
		if err != nil {
			return false, err
		}
		attempt.GateReport = reportPath

		if gatePassed {
			attempt.GateResult = "PASS"
			if r.evaluator != nil && step.Evaluate != nil {
				briefContent, err := os.ReadFile(briefPath)
				if err != nil {
					return false, fmt.Errorf("read brief %s: %w", briefPath, err)
				}
				keepCommit, _, score, err := r.executeEvaluation(
					ctx,
					repoRoot,
					liveDir,
					step,
					strings.TrimSpace(string(briefContent)),
					reportPath,
					&attempt,
				)
				if err != nil {
					return false, err
				}
				attempt.EvaluatorScore = score
				attempt.FinishedAt = nowRFC3339()
				if keepCommit {
					attempt.Kept = true
					commitMsg := fmt.Sprintf("james-loops: %s iteration %d/%d PASS", step.ID, iter, iterations)
					if err := r.git.CommitAll(commitMsg); err != nil {
						return false, err
					}
					keptCount++
					consecutiveFailures = 0
					lastChange = commitMsg
				} else {
					attempt.Kept = false
					if err := r.git.ResetHard(iterCommit); err != nil {
						return false, err
					}
					consecutiveFailures++
					lastChange = fmt.Sprintf("evaluator rejected iteration %d", iter)
				}
			} else {
				attempt.FinishedAt = nowRFC3339()
				attempt.Kept = true
				commitMsg := fmt.Sprintf("james-loops: %s iteration %d/%d PASS", step.ID, iter, iterations)
				if err := r.git.CommitAll(commitMsg); err != nil {
					return false, err
				}
				keptCount++
				consecutiveFailures = 0
				lastChange = commitMsg
			}
		} else {
			attempt.GateResult = "FAIL"
			attempt.FinishedAt = nowRFC3339()
			attempt.Kept = false
			patchPath := filepath.Join(liveDir, fmt.Sprintf("%s-iter-%d.patch", step.ID, iter))
			if err := r.git.SavePatch(patchPath); err != nil {
				return false, err
			}
			attempt.DiffPatch = patchPath
			if err := r.git.ResetHard(iterCommit); err != nil {
				return false, err
			}
			consecutiveFailures++
		}

		stepStatus.Attempts = append(stepStatus.Attempts, attempt)
		stepStatus.IterationsCompleted = keptCount
		status.Steps[step.ID] = *stepStatus
		if err := WriteStatus(liveDir, status); err != nil {
			return false, err
		}

		// Lock: bail if max consecutive failures reached
		if maxConsecFail > 0 && consecutiveFailures >= maxConsecFail {
			stepStatus.State = "locked"
			status.Steps[step.ID] = *stepStatus
			if err := WriteStatus(liveDir, status); err != nil {
				return false, err
			}
			return false, nil
		}
	}

	// Iteration step always passes (the gate is the invariant, not the count)
	stepStatus.State = "passed"
	stepStatus.IterationsCompleted = keptCount
	return true, nil
}

// iterationContext builds the context block injected into the prompt for iteration > 1.
func iterationContext(iteration, total, keptCount int, lastChange string) string {
	if iteration <= 1 {
		return ""
	}
	return fmt.Sprintf(
		"\n\n--- ITERATION %d/%d ---\n"+
			"Commits kept so far: %d\n"+
			"Last change: %s\n\n"+
			"Continue improving. Do not repeat previous changes. "+
			"Make one focused change and stop. "+
			"Do not ask questions.",
		iteration, total, keptCount, lastChange,
	)
}

func (r runner) waitForQuotaRecovery(ctx context.Context, repoRoot string, liveDir string, status *Status, stepID string) error {
	status.State = "waiting-budget"
	status.CurrentStep = stepID
	if err := WriteStatus(liveDir, status); err != nil {
		return err
	}

	for {
		if err := r.sleep(ctx, r.quotaProbeInterval); err != nil {
			return err
		}
		exitCode, stdout, stderr, err := r.coder.Run(ctx, repoRoot, r.coder.QuotaProbePrompt())
		if err != nil {
			return err
		}
		if r.coder.IsQuotaIssue(exitCode, stdout, stderr) {
			continue
		}
		if r.coder.ProbeSaysReady(exitCode, stdout, stderr) {
			status.State = "running"
			status.CurrentStep = stepID
			return WriteStatus(liveDir, status)
		}
	}
}

// executeEvaluation calls the evaluator with the full context and returns
// (keepCommit bool, feedback string, score float64, err error).
// keepCommit=false means: revert the commit and feed feedback to next coder attempt.
func (r runner) executeEvaluation(
	ctx context.Context,
	repoRoot, liveDir string,
	step ProgramStep,
	briefContent string,
	gateReport string,
	attempt *Attempt,
) (keepCommit bool, feedback string, score float64, err error) {
	if step.Evaluate == nil || r.evaluator == nil {
		return true, "", 0, nil
	}
	diffStat, diffErr := r.git.DiffStat()
	if diffErr != nil || strings.TrimSpace(diffStat) == "" {
		diffStat = "(diff stat unavailable)"
	}
	evaluateBriefPath := filepath.Join(liveDir, filepath.Clean(step.Evaluate.Brief))
	evaluateBrief, err := os.ReadFile(evaluateBriefPath)
	if err != nil {
		return false, "", 0, fmt.Errorf("read evaluate brief %s: %w", evaluateBriefPath, err)
	}
	aderReport := truncateForEvaluation(readRetryContext(repoRoot, gateReport), 4000)
	prompt := strings.TrimSpace(fmt.Sprintf(
		"[BRIEF]\n%s\n\n[CODER ACTION]\nLe coder a traite le brief et produit les modifications suivantes :\n%s\n\n[GATE RESULT]\nGate: PASS\nCommande : %s\n\n[ADER REPORT]\n%s\n\n[EVALUATE]\n%s\n",
		briefContent,
		diffStat,
		step.Gate.Command,
		aderReport,
		strings.TrimSpace(string(evaluateBrief)),
	))
	evaluator := r.evaluator
	needsJSONScore := false
	if step.Evaluate.OutputField != "" && r.evaluatorProfile != nil && r.evaluatorProfile.Driver == profile.DriverGemini {
		needsJSONScore = true
		if !hasGeminiOutputFormat(r.evaluatorProfile.ExtraArgs) {
			evalProfile := runnerpkg.WithGeminiJSONOutput(*r.evaluatorProfile)
			evaluator, err = runnerpkg.New(evalProfile)
			if err != nil {
				return false, "", 0, err
			}
		}
	}
	exitCode, stdout, stderr, err := evaluator.Run(ctx, repoRoot, prompt)
	if err != nil {
		return false, "", 0, err
	}
	if evaluator.IsQuotaIssue(exitCode, stdout, stderr) {
		return false, "", 0, fmt.Errorf("evaluator profile %q hit a quota issue", r.evaluatorProfile.Name)
	}
	output := strings.TrimSpace(string(stdout))
	if output == "" {
		output = strings.TrimSpace(string(stderr))
	}
	attempt.EvaluatorOutput = output
	if !needsJSONScore {
		return true, output, 0, nil
	}
	score, feedback, err = extractEvaluationScore(output, step.Evaluate.OutputField)
	if err != nil {
		return false, "", 0, err
	}
	if step.Evaluate.Threshold == 0 || score >= step.Evaluate.Threshold {
		return true, feedback, score, nil
	}
	return false, feedback, score, nil
}

func resolveProgramSource(aderRoot string, opts EnqueueOptions) (string, error) {
	if strings.TrimSpace(opts.FromCookbook) != "" && strings.TrimSpace(opts.ProgramDir) != "" {
		return "", fmt.Errorf("program directory and cookbook are mutually exclusive")
	}
	switch {
	case strings.TrimSpace(opts.FromCookbook) != "":
		return filepath.Join(aderRoot, cookbookDirName, opts.FromCookbook), nil
	case strings.TrimSpace(opts.ProgramDir) != "":
		return filepath.Abs(opts.ProgramDir)
	default:
		return "", fmt.Errorf("enqueue requires a program directory or cookbook name")
	}
}

func resolveAderRoot(aderRoot string, repoRoot string) (string, error) {
	if strings.TrimSpace(aderRoot) != "" {
		return filepath.Abs(aderRoot)
	}
	root := repoRoot
	if strings.TrimSpace(root) == "" {
		var err error
		root, err = gitRepoRoot()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(root, "ader", "loops"), nil
}

func parseProgram(dir string) (*Program, error) {
	data, err := os.ReadFile(filepath.Join(dir, programFile))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Join(dir, programFile), err)
	}
	var program Program
	if err := yaml.Unmarshal(data, &program); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", filepath.Join(dir, programFile), err)
	}
	if program.MaxRetries <= 0 {
		program.MaxRetries = defaultMaxRetries
	}
	if strings.TrimSpace(program.Mode) == "" {
		program.Mode = "solo"
	}
	return &program, nil
}

func newStatus(program *Program, state string) *Status {
	status := &Status{
		State:            state,
		ProgramDesc:      program.Description,
		CoderProfile:     program.Profiles.Coder,
		EvaluatorProfile: program.Profiles.Evaluator,
		Steps:            make(map[string]StepStatus, len(program.Steps)),
	}
	for _, step := range program.Steps {
		status.Steps[step.ID] = StepStatus{State: "pending"}
	}
	return status
}

func loadLiveProgram(liveDir string) (*Program, *Status, error) {
	program, err := parseProgram(liveDir)
	if err != nil {
		return nil, nil, err
	}
	status, err := ReadStatus(liveDir)
	if err != nil {
		if !os.IsNotExist(err) {
			// ReadStatus wraps the path, so keep a fallback existence check.
			if _, statErr := os.Stat(filepath.Join(liveDir, statusFile)); statErr == nil {
				return nil, nil, err
			}
		}
		status = newStatus(program, "queued")
	}
	for _, step := range program.Steps {
		if _, ok := status.Steps[step.ID]; !ok {
			status.Steps[step.ID] = StepStatus{State: "pending"}
		}
	}
	if strings.TrimSpace(status.ProgramDesc) == "" {
		status.ProgramDesc = program.Description
	}
	if strings.TrimSpace(status.CoderProfile) == "" {
		status.CoderProfile = program.Profiles.Coder
	}
	if strings.TrimSpace(status.EvaluatorProfile) == "" {
		status.EvaluatorProfile = program.Profiles.Evaluator
	}
	return program, status, nil
}

func collectStatusResult(aderRoot string) (*StatusResult, error) {
	queue, err := collectSlotSummaries(filepath.Join(aderRoot, queueDirName))
	if err != nil {
		return nil, err
	}
	deferred, err := collectSlotSummaries(filepath.Join(aderRoot, deferredDirName))
	if err != nil {
		return nil, err
	}
	done, err := collectSlotSummaries(filepath.Join(aderRoot, doneDirName))
	if err != nil {
		return nil, err
	}
	result := &StatusResult{
		QueueSlots:    queue,
		DeferredSlots: deferred,
		DoneSlots:     done,
	}
	liveDir := filepath.Join(aderRoot, liveDirName)
	if _, err := os.Stat(filepath.Join(liveDir, statusFile)); err == nil {
		summary, err := summarizeSlot(inferLiveSlotFromDir(liveDir), liveDir)
		if err == nil {
			result.LiveSlot = &summary
		}
	}
	return result, nil
}

func collectSlotSummaries(dir string) ([]SlotSummary, error) {
	slots, err := scanNumberedDirs(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	items := make([]SlotSummary, 0, len(slots))
	for _, slot := range slots {
		summary, err := summarizeSlot(slot, filepath.Join(dir, slot))
		if err != nil {
			return nil, err
		}
		items = append(items, summary)
	}
	return items, nil
}

func summarizeSlot(slot string, dir string) (SlotSummary, error) {
	status, err := ReadStatus(dir)
	if err != nil {
		return SlotSummary{}, err
	}
	summary := SlotSummary{
		Slot:             slot,
		Description:      status.ProgramDesc,
		State:            status.State,
		Branch:           status.Branch,
		CurrentStep:      status.CurrentStep,
		StepsTotal:       len(status.Steps),
		CoderProfile:     status.CoderProfile,
		EvaluatorProfile: status.EvaluatorProfile,
	}
	for _, step := range status.Steps {
		if step.State == "passed" {
			summary.StepsPassed++
		}
	}
	if summary.State == "" {
		summary.State = "queued"
	}
	return summary, nil
}

var slotPattern = regexp.MustCompile(`^\d+$`)

func scanNumberedDirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	slots := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !slotPattern.MatchString(entry.Name()) {
			continue
		}
		slots = append(slots, entry.Name())
	}
	sort.Slice(slots, func(i, j int) bool {
		return slots[i] < slots[j]
	})
	return slots, nil
}

func nextSlotNumber(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}
	slots, err := scanNumberedDirs(dir)
	if err != nil {
		return "", err
	}
	last := 0
	for _, slot := range slots {
		value, err := strconv.Atoi(slot)
		if err != nil {
			continue
		}
		if value > last {
			last = value
		}
	}
	return fmt.Sprintf("%03d", last+1), nil
}

func copyDir(src string, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src string, dst string, mode fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return os.Chmod(dst, mode)
}

func moveDir(src string, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
	}
	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("remove %s: %w", dst, err)
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", src, dst, err)
	}
	return nil
}

func activateSlot(aderRoot string, slot string) error {
	liveDir := filepath.Join(aderRoot, liveDirName)
	if err := os.RemoveAll(liveDir); err != nil {
		return fmt.Errorf("remove %s: %w", liveDir, err)
	}
	return moveDir(filepath.Join(aderRoot, queueDirName, slot), liveDir)
}

func liveBusy(liveDir string) bool {
	entries, err := os.ReadDir(liveDir)
	if err != nil {
		return false
	}
	return len(entries) > 0
}

func stepIndex(program *Program, stepID string) int {
	for i, step := range program.Steps {
		if step.ID == stepID {
			return i
		}
	}
	return -1
}

func firstPendingStep(program *Program, status *Status) int {
	for i, step := range program.Steps {
		current := status.Steps[step.ID]
		if current.State != "passed" && current.State != "skipped" {
			return i
		}
	}
	return -1
}

func ensureStepStatus(status *Status, stepID string) StepStatus {
	current, ok := status.Steps[stepID]
	if !ok {
		return StepStatus{State: "pending"}
	}
	return current
}

func resolveMaxRetries(override int, configured int) int {
	if override > 0 {
		return override
	}
	if configured > 0 {
		return configured
	}
	return defaultMaxRetries
}

func loadPrompt(briefPath string, retryContext string) (string, error) {
	brief, err := os.ReadFile(briefPath)
	if err != nil {
		return "", fmt.Errorf("read brief %s: %w", briefPath, err)
	}
	prompt := strings.TrimSpace(string(brief))
	if strings.TrimSpace(retryContext) != "" {
		prompt += "\n\n--- PREVIOUS ATTEMPT FAILED ---\n" + strings.TrimSpace(retryContext)
	}
	return prompt, nil
}

func readRetryContext(repoRoot string, reportPath string) string {
	reportPath = strings.TrimSpace(reportPath)
	if reportPath == "" {
		return ""
	}
	path := reportPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(repoRoot, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("gate report: %s", reportPath)
	}
	return string(data)
}

func latestSlotPath(dir string) (string, bool) {
	slots, err := scanNumberedDirs(dir)
	if err != nil || len(slots) == 0 {
		return "", false
	}
	return filepath.Join(dir, slots[len(slots)-1]), true
}

func inferLiveSlot(status *Status) string {
	if status == nil {
		return "live"
	}
	matches := regexp.MustCompile(`james-loops/(\d+)-`).FindStringSubmatch(status.Branch)
	if len(matches) == 2 {
		return matches[1]
	}
	return "live"
}

func inferLiveSlotFromDir(liveDir string) string {
	status, err := ReadStatus(liveDir)
	if err != nil {
		return "live"
	}
	return inferLiveSlot(status)
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "program"
	}
	return value
}

func nowRFC3339() string {
	return time.Now().Format(time.RFC3339)
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func resolveCoderProfile(program *Program, status *Status, override string) (profile.Profile, error) {
	name := strings.TrimSpace(override)
	if name == "" && status != nil {
		name = strings.TrimSpace(status.CoderProfile)
	}
	if name == "" && program != nil {
		name = strings.TrimSpace(program.Profiles.Coder)
	}
	if name == "" {
		return profile.Profile{}, fmt.Errorf(
			"no coder profile specified; set profiles.coder in program.yaml or use --coder-profile\n(run `james-loops profile list` to see available profiles)",
		)
	}
	return profile.Load(name)
}

func resolveEvaluatorProfile(program *Program, status *Status, override string) (profile.Profile, error) {
	name := strings.TrimSpace(override)
	if name == "" && status != nil {
		name = strings.TrimSpace(status.EvaluatorProfile)
	}
	if name == "" && program != nil {
		name = strings.TrimSpace(program.Profiles.Evaluator)
	}
	if name == "" {
		return profile.Profile{}, fmt.Errorf(
			"no evaluator profile specified; set profiles.evaluator in program.yaml or use --evaluator-profile\n(run `james-loops profile list` to see available profiles)",
		)
	}
	return profile.Load(name)
}

func programMode(program *Program) string {
	if program == nil || strings.TrimSpace(program.Mode) == "" {
		return "solo"
	}
	return strings.TrimSpace(program.Mode)
}

func buildEvaluationRetryContext(feedback string, output string, score float64, threshold float64) string {
	var b strings.Builder
	b.WriteString("Evaluator rejected the previous attempt.")
	if threshold > 0 {
		fmt.Fprintf(&b, "\nScore: %.2f (threshold %.2f)", score, threshold)
	}
	if strings.TrimSpace(feedback) != "" {
		b.WriteString("\nFeedback:\n")
		b.WriteString(strings.TrimSpace(feedback))
	} else if strings.TrimSpace(output) != "" {
		b.WriteString("\nOutput:\n")
		b.WriteString(strings.TrimSpace(output))
	}
	return b.String()
}

func truncateForEvaluation(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}

func hasGeminiOutputFormat(args []string) bool {
	for i := 0; i < len(args); i++ {
		if args[i] == "--output-format" {
			return true
		}
		if strings.HasPrefix(args[i], "--output-format=") {
			return true
		}
	}
	return false
}

func extractEvaluationScore(output string, field string) (float64, string, error) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		return 0, "", fmt.Errorf("parse evaluator output as JSON: %w", err)
	}
	rawScore, ok := payload[field]
	if !ok {
		return 0, "", fmt.Errorf("evaluator output missing %q", field)
	}
	score, ok := numericValue(rawScore)
	if !ok {
		return 0, "", fmt.Errorf("evaluator field %q is not numeric", field)
	}
	feedback, _ := payload["feedback"].(string)
	return score, feedback, nil
}

func numericValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	default:
		return 0, false
	}
}
