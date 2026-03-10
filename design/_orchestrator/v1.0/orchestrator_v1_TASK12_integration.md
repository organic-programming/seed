# TASK12 — Integration + End-to-End Wiring

## Context

TASK01–11 each implement a distinct package. This task wires them into
`cmd/orchestrator/main.go` and validates with a smoke test.

## Objective

Create the entry point following §4.3 and verify the full flow.

## Changes

### [MODIFY] `cmd/orchestrator/main.go`

Wire all packages into the main execution flow:

```go
func main() {
    cfg, err := cli.Parse()
    if err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }

    lock, err := state.Acquire(cfg.Root)
    if err != nil {
        fmt.Fprintf(os.Stderr, "lock: %v\n", err)
        os.Exit(1)
    }
    defer lock.Release()

    if err := preflight.Run(cfg); err != nil {
        fmt.Fprintf(os.Stderr, "pre-flight: %v\n", err)
        os.Exit(1)
    }

    st := state.Load(cfg.StateFile)
    setupSignalHandler(st, lock)

    startTime := time.Now()
    var setResults []summary.SetResult

    for _, setName := range cfg.Sets {
        setDir, project, err := tasks.FindSetDir(cfg.Root, setName)
        if err != nil { ... }
        git.EnsureConsistency(cfg.Root, project, setName)

        entries, err := tasks.Parse(filepath.Join(setDir, "_TASKS.md"))
        if err != nil { ... }
        ordered, err := tasks.Sort(entries)
        if err != nil { ... }

        submodules, _ := git.ListSubmodulePaths(cfg.Root)

        for _, task := range ordered {
            if st.IsCompleted(task.FilePath) {
                continue
            }

            gitOps := &git.Ops{Root: cfg.Root}
            lifecycle.StartTask(task, setDir, gitOps)

            priorResults := st.CompletedResults(setDir)
            p, _ := prompt.Build(cfg, setDir, task.FilePath, priorResults)

            taskContent, _ := os.ReadFile(task.FilePath)
            addDirs := git.DetectRefs(string(taskContent), submodules)

            result := codex.ExecuteLoop(cfg, task, p, addDirs, st)

            lifecycle.CompleteTask(task, result, setDir, gitOps)
            lifecycle.UpdateVersionStatus(setDir, ordered, gitOps)
            st.Save()
        }

        setResults = append(setResults, summary.BuildSetResult(setName, ordered, st))

        if allPassed(ordered, st) {
            lifecycle.Release(setDir, setName, &git.Ops{Root: cfg.Root})
        }
    }

    summary.Print(st, setResults, time.Since(startTime))
}

// allPassed returns true if every task in the set completed successfully.
func allPassed(entries []tasks.Entry, st *state.State) bool {
    for _, e := range entries {
        if !st.IsCompleted(e.FilePath) {
            return false
        }
    }
    return true
}
```

### Smoke Test

Run against a synthetic 2-task set to verify the full flow compiles and
produces the expected log files and state.

## Acceptance Criteria

- [ ] All packages wired into `main.go`
- [ ] `go build ./cmd/orchestrator` produces a binary
- [ ] `go vet ./...` — zero warnings
- [ ] Binary prints usage on `--help`

## Dependencies

TASK02, TASK03, TASK04, TASK05, TASK06, TASK07, TASK08, TASK09, TASK10, TASK11.
