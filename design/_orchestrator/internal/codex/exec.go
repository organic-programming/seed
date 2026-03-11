package codex

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/organic-programming/codex-orchestrator/internal/cli"
	"github.com/organic-programming/codex-orchestrator/internal/logging"
	"github.com/organic-programming/codex-orchestrator/internal/state"
	"github.com/organic-programming/codex-orchestrator/internal/tasks"
	"github.com/organic-programming/codex-orchestrator/internal/verify"
)

const (
	MaxFixAttempts = 3
	VerifyTimeout  = 5 * time.Minute
)

type Attempt struct {
	Phase        string
	Output       string
	Error        string
	Category     ErrorCategory
	Verification []verify.Result
}

type Result struct {
	Success  bool
	Outcome  state.Outcome
	ThreadID string
	Output   string
	Attempts int
	Tokens   state.TokenUsage
	History  []Attempt
	Error    string
}

type TaskLogPaths struct {
	JSONL  string
	Stderr string
	Result string
}

type TaskLogs struct {
	Stdout *logging.TeeWriter
	Stderr *logging.TeeWriter
	Paths  TaskLogPaths

	jsonlFile  *os.File
	stderrFile *os.File
}

type invocationMode string

const (
	modeCreate invocationMode = "CREATE"
	modeFix    invocationMode = "FIX"
)

type invocationResult struct {
	Result
	Mode      invocationMode
	RawStderr string
	ExitCode  int
	Err       error
	Events    []Event
}

var (
	currentCmdMu sync.Mutex
	currentCmd   *exec.Cmd
)

func SetCurrentCmd(cmd *exec.Cmd) {
	currentCmdMu.Lock()
	defer currentCmdMu.Unlock()

	currentCmd = cmd
}

func CurrentCmd() *exec.Cmd {
	currentCmdMu.Lock()
	defer currentCmdMu.Unlock()

	return currentCmd
}

func LogPaths(taskFile string) TaskLogPaths {
	return TaskLogPaths{
		JSONL:  taskFile + ".jsonl",
		Stderr: taskFile + ".stderr.log",
		Result: taskFile + ".result.md",
	}
}

func OpenTaskLogs(taskFile string, stdout, stderr io.Writer) (*TaskLogs, error) {
	paths := LogPaths(taskFile)

	jsonlFile, err := os.OpenFile(paths.JSONL, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open JSONL log: %w", err)
	}

	stderrFile, err := os.OpenFile(paths.Stderr, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		_ = jsonlFile.Close()
		return nil, fmt.Errorf("open stderr log: %w", err)
	}

	return &TaskLogs{
		Stdout: &logging.TeeWriter{
			Terminal: stdout,
			LogFile:  jsonlFile,
		},
		Stderr: &logging.TeeWriter{
			Terminal: stderr,
			LogFile:  stderrFile,
		},
		Paths:      paths,
		jsonlFile:  jsonlFile,
		stderrFile: stderrFile,
	}, nil
}

func (tl *TaskLogs) Close() error {
	if tl == nil {
		return nil
	}

	return errors.Join(closeFile(tl.jsonlFile), closeFile(tl.stderrFile))
}

func ExecuteOnce(cfg cli.Config, prompt, taskFile string, addDirs []string) (Result, error) {
	return executeWithWriters(cfg, prompt, taskFile, addDirs, os.Stdout, os.Stderr)
}

func executeWithWriters(cfg cli.Config, prompt, taskFile string, addDirs []string, stdout, stderr io.Writer) (Result, error) {
	invocation := runInvocation(cfg, taskFile, modeCreate, "", prompt, addDirs, stdout, stderr)
	return invocation.Result, invocation.Err
}

func ExecuteLoop(cfg cli.Config, task tasks.Entry, prompt string, addDirs []string, st *state.State) Result {
	taskState := st.Task(task.FilePath)
	if taskState.Attempts == 0 && taskState.Phase == "" {
		clearTaskArtifacts(task.FilePath)
	}

	commands, _ := verify.ExtractCommands(task.FilePath)
	result := Result{
		ThreadID: taskState.ThreadID,
		Tokens:   taskState.Tokens,
		Outcome:  state.OutcomePending,
	}

	phase := strings.ToLower(taskState.Phase)
	switch phase {
	case "fix":
		if taskState.ThreadID == "" || strings.TrimSpace(taskState.PendingPrompt) == "" {
			phase = "create"
		}
	case "verify":
	default:
		phase = "create"
	}

	for {
		switch phase {
		case "create":
			taskState = st.UpdateTask(task.FilePath, func(current *state.TaskState) {
				current.Phase = "create"
				current.PendingPrompt = ""
			})
			_ = st.Save()

			invocation := invokeWithRetry(cfg, task.FilePath, modeCreate, "", prompt, addDirs)
			result = mergeInvocationResult(result, invocation)
			if invocation.ThreadID != "" {
				result.ThreadID = invocation.ThreadID
			}
			taskState = st.UpdateTask(task.FilePath, func(current *state.TaskState) {
				current.ThreadID = result.ThreadID
				current.Tokens = result.Tokens
				current.Attempts = result.Attempts
			})
			_ = st.Save()

			if invocation.Err != nil {
				if finalResult, nextPhase, nextPrompt, done := handleInvocationFailure(st, task.FilePath, result, invocation); done {
					return finalResult
				} else {
					phase = nextPhase
					taskState = st.UpdateTask(task.FilePath, func(current *state.TaskState) {
						current.Phase = nextPhase
						current.PendingPrompt = nextPrompt
						current.ThreadID = result.ThreadID
						current.Attempts = result.Attempts
						current.Tokens = result.Tokens
					})
					_ = st.Save()
					continue
				}
			}

			if len(commands) == 0 {
				return finishSuccess(st, task.FilePath, result)
			}
			phase = "verify"
		case "verify":
			taskState = st.UpdateTask(task.FilePath, func(current *state.TaskState) {
				current.Phase = "verify"
				current.ThreadID = result.ThreadID
				current.Tokens = result.Tokens
				current.Attempts = result.Attempts
			})
			_ = st.Save()

			verifyResults := verify.Run(commands, cfg.Root, VerifyTimeout)
			result.History = append(result.History, Attempt{
				Phase:        "VERIFY",
				Verification: verifyResults,
				Output:       verificationOutput(verifyResults),
			})

			if allVerificationPassed(verifyResults) {
				return finishSuccess(st, task.FilePath, result)
			}

			if result.Attempts >= MaxFixAttempts {
				return finishFailure(st, task.FilePath, result, verificationOutput(verifyResults), state.OutcomeFailed)
			}

			fixPrompt := buildFixPrompt(verifyResults)
			taskState = st.UpdateTask(task.FilePath, func(current *state.TaskState) {
				current.Phase = "fix"
				current.PendingPrompt = fixPrompt
				current.ThreadID = result.ThreadID
				current.Attempts = result.Attempts
				current.Tokens = result.Tokens
			})
			_ = st.Save()
			phase = "fix"
		case "fix":
			taskState = st.UpdateTask(task.FilePath, func(current *state.TaskState) {
				current.Phase = "fix"
				current.ThreadID = result.ThreadID
				current.Attempts = result.Attempts
				current.Tokens = result.Tokens
			})
			_ = st.Save()

			fixPrompt := taskState.PendingPrompt
			if strings.TrimSpace(fixPrompt) == "" {
				return finishFailure(st, task.FilePath, result, "missing fix prompt for resume", state.OutcomeFailed)
			}

			invocation := invokeWithRetry(cfg, task.FilePath, modeFix, result.ThreadID, fixPrompt, addDirs)
			if invocation.Err != nil && resumeFlagsRejected(invocation.RawStderr) {
				fallbackPrompt := "Resume was not supported by the Codex CLI. Continue from this task context and fix the outstanding issue.\n\n" + fixPrompt
				invocation = invokeWithRetry(cfg, task.FilePath, modeCreate, "", fallbackPrompt, addDirs)
			}

			result = mergeInvocationResult(result, invocation)
			if invocation.ThreadID != "" {
				result.ThreadID = invocation.ThreadID
			}
			taskState = st.UpdateTask(task.FilePath, func(current *state.TaskState) {
				current.ThreadID = result.ThreadID
				current.Tokens = result.Tokens
				current.Attempts = result.Attempts
			})
			_ = st.Save()

			if invocation.Err != nil {
				if finalResult, nextPhase, nextPrompt, done := handleInvocationFailure(st, task.FilePath, result, invocation); done {
					return finalResult
				} else {
					phase = nextPhase
					taskState = st.UpdateTask(task.FilePath, func(current *state.TaskState) {
						current.Phase = nextPhase
						current.PendingPrompt = nextPrompt
						current.ThreadID = result.ThreadID
						current.Attempts = result.Attempts
						current.Tokens = result.Tokens
					})
					_ = st.Save()
					continue
				}
			}

			phase = "verify"
		default:
			return finishFailure(st, task.FilePath, result, "unknown execution phase", state.OutcomeFailed)
		}
	}
}

func handleInvocationFailure(st *state.State, taskFile string, result Result, invocation invocationResult) (Result, string, string, bool) {
	category := classifyInvocation(invocation)
	switch category {
	case ErrQuota:
		return finishFailure(st, taskFile, result, invocationErrorText(invocation), state.OutcomeDeferred), "", "", true
	case ErrSandboxViolation:
		return finishFailure(st, taskFile, result, invocationErrorText(invocation), state.OutcomeFailed), "", "", true
	case ErrTaskFailure:
		if result.Attempts >= MaxFixAttempts || result.ThreadID == "" {
			return finishFailure(st, taskFile, result, invocationErrorText(invocation), state.OutcomeFailed), "", "", true
		}
		return result, "fix", buildFailurePrompt(invocation), false
	default:
		return finishFailure(st, taskFile, result, invocationErrorText(invocation), state.OutcomeFailed), "", "", true
	}
}

func invokeWithRetry(cfg cli.Config, taskFile string, mode invocationMode, threadID, prompt string, addDirs []string) invocationResult {
	invocation := runInvocation(cfg, taskFile, mode, threadID, prompt, addDirs, os.Stdout, os.Stderr)
	if invocation.Err == nil {
		return invocation
	}

	category := classifyInvocation(invocation)
	if category != ErrNetwork && category != ErrQuota {
		return invocation
	}

	logger, closeLogger := openRetryLogger(taskFile)
	defer closeLogger()

	last := invocation
	err := RetryWithBackoff(category, func() error {
		last = runInvocation(cfg, taskFile, mode, threadID, prompt, addDirs, os.Stdout, os.Stderr)
		if last.Err == nil {
			return nil
		}
		nextCategory := classifyInvocation(last)
		if nextCategory != category {
			return &retryAbortError{err: last.Err}
		}
		return last.Err
	}, logger)
	if err != nil {
		last.Err = err
	}
	return last
}

func runInvocation(cfg cli.Config, taskFile string, mode invocationMode, threadID, prompt string, addDirs []string, stdout, stderr io.Writer) invocationResult {
	logs, err := OpenTaskLogs(taskFile, stdout, stderr)
	if err != nil {
		return invocationResult{
			Result: Result{Outcome: state.OutcomeFailed},
			Err:    err,
		}
	}

	args := buildExecArgs(cfg, mode, threadID, prompt, logs.Paths.Result, addDirs)
	cmd := exec.Command("codex", args...)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		_ = logs.Close()
		return invocationResult{Result: Result{Outcome: state.OutcomeFailed}, Err: fmt.Errorf("stdout pipe: %w", err)}
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		_ = logs.Close()
		return invocationResult{Result: Result{Outcome: state.OutcomeFailed}, Err: fmt.Errorf("stderr pipe: %w", err)}
	}

	if err := cmd.Start(); err != nil {
		_ = logs.Close()
		return invocationResult{Result: Result{Outcome: state.OutcomeFailed}, Err: fmt.Errorf("start codex: %w", err)}
	}

	SetCurrentCmd(cmd)
	defer SetCurrentCmd(nil)

	var (
		wg        sync.WaitGroup
		stdErrBuf strings.Builder
	)
	streamErrs := make(chan error, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		streamErrs <- streamPipe(stdoutPipe, logs.Stdout, nil)
	}()
	go func() {
		defer wg.Done()
		streamErrs <- streamPipe(stderrPipe, logs.Stderr, &stdErrBuf)
	}()

	waitErr := cmd.Wait()
	wg.Wait()
	close(streamErrs)

	var streamErr error
	for err := range streamErrs {
		if err != nil {
			streamErr = errors.Join(streamErr, err)
		}
	}

	closeErr := logs.Close()

	result := Result{
		Attempts: 1,
		Outcome:  state.OutcomeFailed,
	}

	if output, err := os.ReadFile(logs.Paths.Result); err == nil {
		result.Output = string(output)
	}

	events, eventsErr := ReadEvents(logs.Paths.JSONL)
	if eventsErr == nil {
		result.ThreadID = ExtractThreadID(events)
		result.Tokens = ExtractTokenUsage(events)
		result.Success = waitErr == nil && streamErr == nil && hasCompletedTurn(events) && !containsErrorEvents(events)
	}
	if result.Success {
		result.Outcome = state.OutcomeSuccess
	}

	invocation := invocationResult{
		Result:    result,
		Mode:      mode,
		RawStderr: strings.TrimSpace(stdErrBuf.String()),
		ExitCode:  exitCode(waitErr),
		Events:    events,
	}

	switch {
	case waitErr != nil:
		invocation.Err = waitErr
	case streamErr != nil:
		invocation.Err = streamErr
	case closeErr != nil:
		invocation.Err = closeErr
	case eventsErr != nil:
		invocation.Err = eventsErr
	}
	if invocation.Err == nil && !result.Success {
		invocation.Err = fmt.Errorf("codex did not complete successfully")
	}

	return invocation
}

func buildExecArgs(cfg cli.Config, mode invocationMode, threadID, prompt, resultFile string, addDirs []string) []string {
	args := []string{"exec"}
	if mode == modeFix && strings.TrimSpace(threadID) != "" {
		args = append(args, "resume", threadID)
	}
	args = append(args,
		"--full-auto",
		"--json",
		"--skip-git-repo-check",
		"-C", cfg.Root,
		"-s", "workspace-write",
		"-m", cfg.Model,
		"-o", resultFile,
	)

	for _, addDir := range addDirs {
		addDir = strings.TrimSpace(addDir)
		if addDir == "" {
			continue
		}
		args = append(args, "--add-dir", addDir)
	}

	args = append(args, prompt)
	return args
}

func streamPipe(reader io.Reader, tee *logging.TeeWriter, capture *strings.Builder) error {
	buffered := bufio.NewReader(reader)
	for {
		line, err := buffered.ReadString('\n')
		if len(line) > 0 {
			if capture != nil {
				capture.WriteString(line)
			}
			tee.WriteLine(line)
		}

		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) || strings.Contains(err.Error(), "file already closed") {
			return nil
		}

		return err
	}
}

func closeFile(file *os.File) error {
	if file == nil {
		return nil
	}
	return file.Close()
}

func clearTaskArtifacts(taskFile string) {
	paths := LogPaths(taskFile)
	_ = os.Remove(paths.JSONL)
	_ = os.Remove(paths.Stderr)
	_ = os.Remove(paths.Result)
}

func classifyInvocation(invocation invocationResult) ErrorCategory {
	if invocation.Err == nil {
		return ErrTaskFailure
	}
	if invocation.ExitCode != 0 && len(invocation.Events) == 0 && strings.TrimSpace(invocation.RawStderr) == "" {
		return ErrNetwork
	}
	return Classify(invocation.ExitCode, invocation.RawStderr)
}

func mergeInvocationResult(current Result, invocation invocationResult) Result {
	current.ThreadID = firstNonEmpty(invocation.ThreadID, current.ThreadID)
	current.Output = firstNonEmpty(invocation.Output, current.Output)
	current.Attempts += invocation.Attempts
	current.Tokens.Add(invocation.Tokens)
	current.Error = invocationErrorText(invocation)
	current.History = append(current.History, Attempt{
		Phase:    string(invocation.Mode),
		Output:   strings.TrimSpace(invocation.Output),
		Error:    invocationErrorText(invocation),
		Category: classifyInvocation(invocation),
	})
	return current
}

func finishSuccess(st *state.State, taskFile string, result Result) Result {
	result.Success = true
	result.Outcome = state.OutcomeSuccess
	result.Error = ""
	st.UpdateTask(taskFile, func(current *state.TaskState) {
		current.Completed = true
		current.Outcome = state.OutcomeSuccess
		current.ThreadID = result.ThreadID
		current.Tokens = result.Tokens
		current.Phase = ""
		current.Attempts = result.Attempts
		current.PendingPrompt = ""
	})
	_ = st.Save()
	return result
}

func finishFailure(st *state.State, taskFile string, result Result, message string, outcome state.Outcome) Result {
	result.Success = false
	result.Outcome = outcome
	result.Error = strings.TrimSpace(message)
	st.UpdateTask(taskFile, func(current *state.TaskState) {
		current.Completed = false
		current.Outcome = outcome
		current.ThreadID = result.ThreadID
		current.Tokens = result.Tokens
		current.Phase = ""
		current.Attempts = result.Attempts
		current.PendingPrompt = ""
	})
	_ = st.Save()
	return result
}

func verificationOutput(results []verify.Result) string {
	var sections []string
	for _, result := range results {
		status := "PASS"
		if !result.Passed {
			status = "FAIL"
		}
		sections = append(sections, fmt.Sprintf("[%s] %s\n%s", status, result.Command, strings.TrimSpace(result.Output)))
	}
	return strings.TrimSpace(strings.Join(sections, "\n\n"))
}

func buildFixPrompt(results []verify.Result) string {
	var builder strings.Builder
	builder.WriteString("The following verification commands failed after your implementation:\n\n")
	for _, result := range results {
		if result.Passed {
			continue
		}
		builder.WriteString("--- COMMAND ---\n")
		builder.WriteString(result.Command)
		builder.WriteString("\n\n--- OUTPUT ---\n")
		builder.WriteString(strings.TrimSpace(result.Output))
		builder.WriteString("\n\n")
	}
	builder.WriteString("Fix the issues and ensure all verification commands pass.")
	return builder.String()
}

func buildFailurePrompt(invocation invocationResult) string {
	var builder strings.Builder
	builder.WriteString("The previous Codex attempt failed.\n\n")
	builder.WriteString("--- STDERR ---\n")
	builder.WriteString(strings.TrimSpace(invocation.RawStderr))
	if strings.TrimSpace(invocation.Output) != "" {
		builder.WriteString("\n\n--- LAST OUTPUT ---\n")
		builder.WriteString(strings.TrimSpace(invocation.Output))
	}
	builder.WriteString("\n\nFix the issue and complete the task.")
	return builder.String()
}

func allVerificationPassed(results []verify.Result) bool {
	if len(results) == 0 {
		return true
	}
	for _, result := range results {
		if !result.Passed {
			return false
		}
	}
	return true
}

func hasCompletedTurn(events []Event) bool {
	if len(events) == 0 {
		return false
	}
	last := events[len(events)-1]
	return eventType(last.Data) == "turn.completed"
}

func containsErrorEvents(events []Event) bool {
	for _, event := range events {
		eventType := eventType(event.Data)
		if strings.Contains(eventType, "error") {
			return true
		}
		if _, ok := event.Data["error"]; ok {
			return true
		}
	}
	return false
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}

func invocationErrorText(invocation invocationResult) string {
	if strings.TrimSpace(invocation.RawStderr) != "" {
		return strings.TrimSpace(invocation.RawStderr)
	}
	if invocation.Err != nil {
		return invocation.Err.Error()
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func resumeFlagsRejected(stderr string) bool {
	text := strings.ToLower(stderr)
	return strings.Contains(text, "unknown flag") ||
		strings.Contains(text, "flag provided but not defined") ||
		strings.Contains(text, "accepts") ||
		strings.Contains(text, "resume")
}

func openRetryLogger(taskFile string) (*logging.TeeWriter, func()) {
	file, err := os.OpenFile(LogPaths(taskFile).Stderr, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return &logging.TeeWriter{Terminal: os.Stderr}, func() {}
	}
	return &logging.TeeWriter{Terminal: os.Stderr, LogFile: file}, func() { _ = file.Close() }
}
