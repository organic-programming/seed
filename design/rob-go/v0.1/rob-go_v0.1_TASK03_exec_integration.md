# TASK03 — Wire Exec Mode to Embedded Toolchain

## Context

`gorunner.Run()` currently calls `exec.Command("go", ...)` with
`os.Environ()`. It must use the embedded toolchain binary and
hermetic environment instead.

## Objective

Refactor exec mode to use the provisioned toolchain.

## Changes

### `internal/gorunner/gorunner.go`

Replace the direct `"go"` command with a configurable binary path.
Replace `os.Environ()` with the hermetic env.

```go
// Runner executes Go toolchain commands using an embedded toolchain.
type Runner struct {
    GoBinary string   // absolute path to the embedded go binary
    BaseEnv  []string // hermetic environment
}

func (r *Runner) Run(subcommand string, args []string, workdir string, env []string, timeoutS int) Result
func (r *Runner) RunJSON(args []string, workdir string, env []string, timeoutS int) (Result, []TestEvent)
```

Key changes:
- `exec.CommandContext(ctx, r.GoBinary, argv...)`
- `cmd.Env = mergeEnv(r.BaseEnv, env)` — no `os.Environ()`

### `internal/service/service.go`

`GoServer` holds a `*Runner` initialized at startup with the
toolchain's binary path and hermetic env.

### `cmd/rob-go/main.go`

At startup:
1. Call `toolchain.Ensure(version)` to provision
2. Build `Runner` with toolchain paths
3. Pass `Runner` to `GoServer`

### `internal/gorunner/gorunner_test.go`

Update tests to construct a `Runner` with system go binary
(test mode uses whatever Go is available).

## Acceptance Criteria

- [ ] `gorunner.Runner.Run()` uses embedded go binary
- [ ] No `os.Environ()` in exec path
- [ ] Caller `env` overrides applied on top of hermetic env
- [ ] Existing tests pass with minimal changes
- [ ] `go test ./... -race` — zero failures

## Dependencies

TASK01 (toolchain provisioning), TASK02 (hermetic env).
