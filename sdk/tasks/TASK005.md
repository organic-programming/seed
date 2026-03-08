# TASK005 — Implement `connect` in `c-holons`

## Context

The Organic Programming SDK fleet requires a `connect` module in every SDK.
`c-holons` uses a single-file architecture (`src/holons.c` + `include/holons/holons.h`).
The `connect` functions must be added to these existing files.

The **reference implementation** is `go-holons/pkg/connect/connect.go` — study
it before starting.

## Workspace

- SDK root: `sdk/c-holons/`
- Existing files: `include/holons/holons.h` (header), `src/holons.c` (implementation)
- Reference: `sdk/go-holons/pkg/connect/connect.go`
- Spec: `sdk/TODO_CONNECT.md` § `c-holons`

## What to implement

Add functions to `include/holons/holons.h` and `src/holons.c`.

### Public API

```c
grpc_channel *holons_connect(const char *target);
grpc_channel *holons_connect_with_opts(const char *target, holons_connect_options opts);
void holons_disconnect(grpc_channel *channel);

typedef struct {
    int timeout_ms;        // default 5000
    const char *transport; // "stdio" (default), "tcp" (explicit override)
    int start;             // 1 = start if not running (default 1)
    const char *port_file; // NULL = use default
} holons_connect_options;
```

### Resolution logic

Same 3-step algorithm:
1. `target` contains `:` → direct dial via `grpc_insecure_channel_create`.
2. Else → slug → use existing `holons_discover_by_slug` → port file → start → dial.

### Process management

- Use `fork()`/`exec()` to launch the binary.
- Track child PIDs in a static array or linked list.
- `holons_disconnect()`: close channel, if ephemeral → `kill(pid, SIGTERM)`,
  `waitpid` with 2s timeout, then `kill(pid, SIGKILL)`.
- Parse port from child's stdout/stderr using `pipe()` + `read()`.

### Port file convention

Path: `$CWD/.op/run/<slug>.port`
Content: `tcp://127.0.0.1:<port>\n`

## Testing

Add tests in `test/` following existing patterns.

1. Direct dial test
2. Slug resolution test
3. Port file reuse test
4. Stale port file cleanup test

## Rules

- Follow existing code style in `holons.c` — look at the discover section for patterns.
- Use POSIX-only APIs (`fork`, `exec`, `pipe`, `kill`, `waitpid`).
- Run existing test suite — all must still pass.
- Run with `-Wall -Wextra` — no new warnings.
