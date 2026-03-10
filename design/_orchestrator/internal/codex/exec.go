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

	"github.com/organic-programming/codex-orchestrator/internal/cli"
	"github.com/organic-programming/codex-orchestrator/internal/logging"
	"github.com/organic-programming/codex-orchestrator/internal/state"
)

type Result struct {
	Success  bool
	ThreadID string
	Output   string
	Attempts int
	Tokens   state.TokenUsage
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

	jsonlFile, err := os.Create(paths.JSONL)
	if err != nil {
		return nil, fmt.Errorf("create JSONL log: %w", err)
	}

	stderrFile, err := os.Create(paths.Stderr)
	if err != nil {
		_ = jsonlFile.Close()
		return nil, fmt.Errorf("create stderr log: %w", err)
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
	logs, err := OpenTaskLogs(taskFile, stdout, stderr)
	if err != nil {
		return Result{}, err
	}

	args := buildExecArgs(cfg, prompt, logs.Paths.Result, addDirs)
	cmd := exec.Command("codex", args...)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		_ = logs.Close()
		return Result{}, fmt.Errorf("stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		_ = logs.Close()
		return Result{}, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = logs.Close()
		return Result{}, fmt.Errorf("start codex: %w", err)
	}

	SetCurrentCmd(cmd)
	defer SetCurrentCmd(nil)

	var wg sync.WaitGroup
	streamErrs := make(chan error, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		streamErrs <- streamPipe(stdoutPipe, logs.Stdout)
	}()
	go func() {
		defer wg.Done()
		streamErrs <- streamPipe(stderrPipe, logs.Stderr)
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
		Success: waitErr == nil,
	}

	if output, err := os.ReadFile(logs.Paths.Result); err == nil {
		result.Output = string(output)
	}

	events, eventsErr := ReadEvents(logs.Paths.JSONL)
	if eventsErr == nil {
		result.ThreadID = ExtractThreadID(events)
		result.Tokens = ExtractTokenUsage(events)
	}

	if waitErr != nil {
		return result, waitErr
	}
	if streamErr != nil {
		return result, streamErr
	}
	if closeErr != nil {
		return result, closeErr
	}
	if eventsErr != nil {
		return result, eventsErr
	}

	return result, nil
}

func buildExecArgs(cfg cli.Config, prompt, resultFile string, addDirs []string) []string {
	args := []string{
		"exec",
		"--full-auto",
		"--json",
		"--skip-git-repo-check",
		"-C", cfg.Root,
		"-s", "workspace-write",
		"-m", cfg.Model,
		"-o", resultFile,
	}

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

func streamPipe(reader io.Reader, tee *logging.TeeWriter) error {
	buffered := bufio.NewReader(reader)

	for {
		line, err := buffered.ReadString('\n')
		if len(line) > 0 {
			tee.WriteLine(line)
		}

		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
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
