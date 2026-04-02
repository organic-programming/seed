package engine

import (
	"context"
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

	"gopkg.in/yaml.v3"
)

const (
	defaultMaxRetries = 3
	programFile       = "program.yaml"
	queueDirName      = "queue"
	liveDirName       = "live"
	deferredDirName   = "deferred"
	doneDirName       = "done"
	cookbookDirName   = "cookbook"
)

type runner struct {
	codex CodexRunner
	git   GitOps
}

func newRunner(codex CodexRunner, git GitOps) runner {
	return runner{codex: codex, git: git}
}

func Run(ctx context.Context, opts RunOptions) error {
	return newRunner(shellCodexRunner{}, newShellGitOps("")).Run(ctx, opts)
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
	return newRunner(shellCodexRunner{}, newShellGitOps("")).Resume(ctx, aderRoot, maxRetries)
}

func Skip(ctx context.Context, aderRoot string, maxRetries int) (string, string, error) {
	return newRunner(shellCodexRunner{}, newShellGitOps("")).Skip(ctx, aderRoot, maxRetries)
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
		if strings.TrimSpace(status.Branch) == "" {
			branch := fmt.Sprintf("codex-loops/%s-%s", slot, slugify(program.Description))
			if err := r.git.CheckoutNewBranch(branch); err != nil {
				return err
			}
			status.Branch = branch
		}
		if strings.TrimSpace(status.StartedAt) == "" {
			status.StartedAt = nowRFC3339()
		}
		status.State = "running"
		status.ProgramDesc = program.Description
		if err := WriteStatus(liveDir, status); err != nil {
			return err
		}
		allPassed, err := r.executeProgram(ctx, repoRoot, liveDir, program, status, resolveMaxRetries(opts.MaxRetries, program.MaxRetries), false)
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
	if status.State != "deferred" {
		return fmt.Errorf("live program is not deferred")
	}
	status.State = "running"
	status.FinishedAt = ""
	if err := WriteStatus(liveDir, status); err != nil {
		return err
	}
	allPassed, err := r.executeProgram(ctx, repoRoot, liveDir, program, status, resolveMaxRetries(maxRetries, program.MaxRetries), false)
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
	if err := WriteStatus(liveDir, status); err != nil {
		return "", "", err
	}
	allPassed, err := r.executeProgram(ctx, repoRoot, liveDir, program, status, resolveMaxRetries(maxRetries, program.MaxRetries), false)
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
		startCommit, err := r.git.CurrentCommit()
		if err != nil {
			return false, err
		}
		status.State = "running"
		status.CurrentStep = step.ID
		stepStatus.State = "running"
		status.Steps[step.ID] = stepStatus
		if err := WriteStatus(liveDir, status); err != nil {
			return false, err
		}

		retryContext := ""
		passed := false
		for attemptNo := 1; attemptNo <= maxRetries; attemptNo++ {
			attempt := Attempt{StartedAt: nowRFC3339()}
			prompt, err := loadPrompt(filepath.Join(liveDir, filepath.Clean(step.Brief)), retryContext)
			if err != nil {
				return false, err
			}
			exitCode, _, _, err := r.codex.Run(ctx, repoRoot, prompt)
			if err != nil {
				return false, err
			}
			attempt.CodexExitCode = exitCode

			gatePassed, reportPath, err := runGate(ctx, repoRoot, step.Gate)
			if err != nil {
				return false, err
			}
			attempt.FinishedAt = nowRFC3339()
			attempt.GateReport = reportPath
			if gatePassed {
				attempt.GateResult = "PASS"
				stepStatus.State = "passed"
				stepStatus.Attempts = append(stepStatus.Attempts, attempt)
				status.Steps[step.ID] = stepStatus
				if err := r.git.CommitAll(fmt.Sprintf("codex-loops: %s PASS (attempt %d)", step.ID, attemptNo)); err != nil {
					return false, err
				}
				if err := WriteStatus(liveDir, status); err != nil {
					return false, err
				}
				passed = true
				break
			}

			attempt.GateResult = "FAIL"
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
			status.Steps[step.ID] = stepStatus
			if err := WriteStatus(liveDir, status); err != nil {
				return false, err
			}
			retryContext = readRetryContext(repoRoot, reportPath)
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
	return filepath.Join(root, "ader", "codex-loops"), nil
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
	return &program, nil
}

func newStatus(program *Program, state string) *Status {
	status := &Status{
		State:       state,
		ProgramDesc: program.Description,
		Steps:       make(map[string]StepStatus, len(program.Steps)),
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
		Slot:        slot,
		Description: status.ProgramDesc,
		State:       status.State,
		Branch:      status.Branch,
		CurrentStep: status.CurrentStep,
		StepsTotal:  len(status.Steps),
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
	matches := regexp.MustCompile(`codex-loops/(\d+)-`).FindStringSubmatch(status.Branch)
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
