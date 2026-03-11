package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/organic-programming/codex-orchestrator/internal/cli"
	"github.com/organic-programming/codex-orchestrator/internal/codex"
	"github.com/organic-programming/codex-orchestrator/internal/git"
	"github.com/organic-programming/codex-orchestrator/internal/lifecycle"
	"github.com/organic-programming/codex-orchestrator/internal/preflight"
	"github.com/organic-programming/codex-orchestrator/internal/prompt"
	"github.com/organic-programming/codex-orchestrator/internal/state"
	"github.com/organic-programming/codex-orchestrator/internal/summary"
	"github.com/organic-programming/codex-orchestrator/internal/tasks"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if wantsHelp(args) {
		printUsage()
		return 0
	}

	cfg, err := cli.ParseArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage()
			return 0
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	lock, err := state.Acquire(cfg.Root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lock: %v\n", err)
		return 1
	}
	defer lock.Release()

	if err := preflight.Run(*cfg); err != nil {
		fmt.Fprintf(os.Stderr, "pre-flight: %v\n", err)
		return 1
	}

	st := state.Load(cfg.StateFile)
	interrupted := setupSignalHandler(st, lock)

	startTime := time.Now()
	var setResults []summary.SetResult
	stopAll := false

	for _, setName := range cfg.Sets {
		if interrupted() {
			break
		}

		setDir, project, err := tasks.FindSetDir(cfg.Root, setName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "set %s: %v\n", setName, err)
			return 1
		}

		gitOps := &git.Ops{Root: cfg.Root}
		if filepath.Base(setDir) != rawSetName(filepath.Base(setDir)) {
			if err := lifecycle.Reset(setDir, st, gitOps); err != nil {
				fmt.Fprintf(os.Stderr, "reset %s: %v\n", setName, err)
				return 1
			}
			setDir = filepath.Join(filepath.Dir(setDir), rawSetName(filepath.Base(setDir)))
		}

		if err := git.EnsureConsistency(cfg.Root, project, setName); err != nil {
			fmt.Fprintf(os.Stderr, "git consistency: %v\n", err)
			return 1
		}

		entries, err := tasks.Parse(filepath.Join(setDir, "_TASKS.md"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse tasks: %v\n", err)
			return 1
		}
		ordered, err := tasks.Sort(entries)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sort tasks: %v\n", err)
			return 1
		}

		submodules, err := git.ListSubmodulePaths(cfg.Root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "submodules: %v\n", err)
			return 1
		}

		setFailed := false
		for _, task := range ordered {
			if interrupted() {
				break
			}
			if st.IsCompleted(task.FilePath) {
				continue
			}

			if err := lifecycle.StartTask(task, setDir, gitOps); err != nil {
				fmt.Fprintf(os.Stderr, "start task %s: %v\n", task.Number, err)
				return 1
			}

			priorResults := st.CompletedResults(setDir)
			taskPrompt, err := prompt.Build(*cfg, setDir, task.FilePath, priorResults)
			if err != nil {
				fmt.Fprintf(os.Stderr, "build prompt for %s: %v\n", task.Number, err)
				return 1
			}

			taskContent, err := os.ReadFile(task.FilePath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "read task %s: %v\n", task.Number, err)
				return 1
			}
			addDirs := git.DetectRefs(string(taskContent), submodules)

			result := codex.ExecuteLoop(*cfg, task, taskPrompt, addDirs, st)
			if err := lifecycle.CompleteTask(task, result, setDir, gitOps); err != nil {
				fmt.Fprintf(os.Stderr, "complete task %s: %v\n", task.Number, err)
				return 1
			}
			if err := lifecycle.UpdateVersionStatus(setDir, ordered, gitOps); err != nil {
				fmt.Fprintf(os.Stderr, "update set status %s: %v\n", setName, err)
				return 1
			}
			if err := st.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "save state: %v\n", err)
				return 1
			}

			switch result.Outcome {
			case state.OutcomeFailed:
				setFailed = true
			case state.OutcomeDeferred:
				setFailed = true
				stopAll = true
			}
			if setFailed {
				break
			}
		}

		setResults = append(setResults, summary.BuildSetResult(setName, ordered, st))
		if !setFailed && allPassed(ordered, st) {
			if err := lifecycle.Release(setDir, setName, gitOps); err != nil {
				fmt.Fprintf(os.Stderr, "release %s: %v\n", setName, err)
				return 1
			}
		}

		if stopAll || interrupted() {
			break
		}
	}

	summary.Print(st, setResults, time.Since(startTime))

	if interrupted() {
		return 130
	}
	if stopAll {
		return 0
	}
	return 0
}

func wantsHelp(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}

func printUsage() {
	fmt.Fprintln(os.Stdout, "Usage: orchestrator --set <version> [--set <version> ...] [--model <model>] [--root <repo>]")
}

func allPassed(entries []tasks.Entry, st *state.State) bool {
	for _, entry := range entries {
		if !st.IsCompleted(entry.FilePath) {
			return false
		}
	}
	return true
}

func setupSignalHandler(st *state.State, lock *state.Lock) func() bool {
	var interrupted atomic.Bool
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	go func() {
		var firstSignal time.Time
		for sig := range signals {
			if interrupted.Load() && time.Since(firstSignal) <= 3*time.Second {
				_ = st.Save()
				_ = lock.Release()
				os.Exit(1)
			}

			firstSignal = time.Now()
			interrupted.Store(true)
			fmt.Fprintf(os.Stderr, "Received %s, shutting down gracefully...\n", sig)

			if cmd := codex.CurrentCmd(); cmd != nil && cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			} else {
				_ = st.Save()
				_ = lock.Release()
				os.Exit(130)
			}

			_ = st.Save()
			_ = lock.Release()
		}
	}()

	return interrupted.Load
}

func rawSetName(name string) string {
	for _, prefix := range []string{"✅ ", "⚠️ ", "⚠ ", "💭 "} {
		if strings.HasPrefix(name, prefix) {
			return strings.TrimPrefix(name, prefix)
		}
	}
	return name
}
