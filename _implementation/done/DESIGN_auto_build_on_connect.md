# Elapsed Progress and Auto-Build on Connect

Two features, one shared primitive.

## 1. Elapsed Progress (shared primitive)

All long-running `op` commands display elapsed-time progress on stderr. The same writer is used by `op build`, `op install --build`, and auto-build-on-connect.

### Behavior

| Context | On success | On failure |
|---------|-----------|-----------|
| `op build` | Progress stays (user asked to build) | Progress stays + error |
| `op install --build` | Progress stays | Progress stays + error |
| Auto-build on connect | Progress **erased**, only call result on stdout | Progress stays + error |

### Output format

A single line, overwritten in place each second and on every phase change:

```
00:00:01 building gabriel-greeting-swift…
00:00:02 building gabriel-greeting-swift… swift build       ← replaces previous
00:00:08 building gabriel-greeting-swift… linking           ← replaces previous
00:00:09 building gabriel-greeting-swift… packaging .holon  ← replaces previous
```

- Elapsed `%02d:%02d:%02d` ticks every second on the **same line** (carriage return `\r` + clear-to-EOL).
- A new phase message replaces the current line immediately.
- Non-TTY: prints each phase as a new line without ANSI codes; auto-build suppresses entirely.

### API

```go
// internal/progress/progress.go
type Writer struct {
    w         io.Writer
    isTTY     bool
    start     time.Time
    lastLines int
}

func New(w io.Writer) *Writer
func (pw *Writer) Print(msg string)   // "00:00:04 msg\n", tracks line count
func (pw *Writer) Clear()             // erases all printed lines (TTY only)
```

### Usage across commands

**`op build`** — progress stays visible:
```go
pw := progress.New(os.Stderr)
err := build(dir, func(line string) {
    pw.Print(fmt.Sprintf("building %s… %s", slug, line))
})
// no pw.Clear() — user explicitly asked to build
```

**`op install --build`** — same, progress stays:
```go
pw := progress.New(os.Stderr)
pw.Print(fmt.Sprintf("building %s…", slug))
buildErr := build(dir, func(line string) { pw.Print(...) })
if buildErr != nil { return buildErr }
pw.Print(fmt.Sprintf("installing %s…", slug))
installErr := install(dir)
```

**Auto-build on connect** — progress erased on success:
```go
pw := progress.New(os.Stderr)
pw.Print(fmt.Sprintf("building %s…", slug))
buildErr := build(entry.Dir, func(line string) { pw.Print(...) })
if buildErr != nil {
    pw.Print(fmt.Sprintf("build failed: %v", buildErr))
    os.Exit(1)
}
pw.Clear() // erase — the user only cares about the call result
conn, err = connect.Connect(slug)
```

---

## 2. Auto-Build on Connect

When `op <holon> <Method> <args>` finds no built binary, build it transparently before calling.

### Trigger

In `grace-op`'s connect dispatch[^1], after discovery succeeds but binary resolution fails:

```
discover.FindBySlug("gabriel-greeting-swift")  → entry found
resolveSourceLaunchTarget(entry)               → ErrBinaryNotFound
                                               ↓
                              auto-build (with progress)
                                               ↓
op build <entry.Dir>                           → .op/build/<slug>.holon/bin/<arch>/<binary>
                                               ↓
                              progress.Clear()
                                               ↓
resolveSourceLaunchTarget(entry)               → binary found, proceed
```

[^1]: `holons/grace-op/internal/cli/connect_dispatch.go`

### Error detection

Replace the string match with a sentinel:

```go
// sdk/go-holons/pkg/connect
var ErrBinaryNotFound = errors.New("built binary not found")
```

---

## Scope

| Item | Where |
|------|-------|
| `progress.Writer` | `holons/grace-op/internal/progress/` |
| Wire into `op build` | `holons/grace-op/internal/cli/build.go` |
| Wire into `op install` | `holons/grace-op/internal/cli/install.go` |
| Auto-build trigger | `holons/grace-op/internal/cli/connect_dispatch.go` |
| `ErrBinaryNotFound` | `sdk/go-holons/pkg/connect/connect.go` |

## Interpreted languages

JS, Ruby, and Python don't need `op build` to run. Like Go's `go run` trick, `resolveSourceLaunchTarget` should launch them directly from source via their interpreter when no binary exists:

| Runner | Launch command | Needs `op build`? |
|--------|---------------|:-:|
| `go`, `go-module` | `go run <main>` | No (already works) |
| `node`, `typescript` | `node <main>` | No |
| `python` | `python3 <main>` | No |
| `ruby` | `ruby <main>` | No |
| `swift-package` | — | **Yes** (compiled) |
| `gradle`, `maven` | — | **Yes** (compiled) |
| `cargo` | — | **Yes** (compiled) |
| `cmake`, `make` | — | **Yes** (compiled) |
| `dotnet` | — | **Yes** (compiled) |
| `dart` | `dart run <main>` | No |

Auto-build only triggers for compiled runners where no interpreter shortcut exists. `resolveSourceLaunchTarget` already has `interpreterForRunner()` for the package path — extend it to the source path too.[^2]

[^2]: `op build` is still useful for interpreted languages: it generates Incode Description and packages the `.holon` artifact. But it's not required to *run*.

## Documentation

On completion, append this section to `holons/grace-op/OP.md` under the `op <slug> <Method>` command:

````markdown
### Auto-build

When the target holon has no built binary, `op` builds it before calling.
Build progress displays on stderr with elapsed time:

```
00:00:01 building gabriel-greeting-swift…
00:00:08 building gabriel-greeting-swift… linking
```

On success, progress is erased and only the call result appears on stdout.
On failure, progress stays and the build error is shown.

Interpreted holons (Node, Python, Ruby, Dart) run from source without
building — like Go's `go run`, the interpreter is launched directly.

| Runner | Behavior |
|--------|----------|
| `go` | `go run <main>` — no build needed |
| `node`, `typescript` | `node <main>` — no build needed |
| `python` | `python3 <main>` — no build needed |
| `ruby` | `ruby <main>` — no build needed |
| `dart` | `dart run <main>` — no build needed |
| `swift-package`, `gradle`, `cargo`, `cmake`, `dotnet` | Auto-builds via `op build` |

To skip auto-build and fail immediately, use `--no-build`.
````

## Composite Build

From the user's perspective, `op build <slug>` behaves identically whether `<slug>` is a leaf holon or a composite[^3]. For a composite, `op build` resolves the dependency graph and builds each dependency in topological order, showing per-holon progress:

Each dependency occupies **one line** while building (same overwrite behavior as §1). Once done, its line is finalized with `✓` and the next dependency starts on a **new line**:

```
00:00:09 building gabriel-greeting-swift… ✓              ← line frozen
00:00:11 building gabriel-greeting-dart… ✓               ← line frozen
00:00:15 building gabriel-greeting-app-swiftui… linking   ← live, ticking
```

### Resolution

1. Parse the target holon's manifest for its dependency list.
2. Recursively discover each dependency via `discover.FindBySlug`.
3. Topological-sort the graph — fail on cycles.
4. Build each node in order. Skip nodes whose binary is fresh (mtime of binary ≥ mtime of source tree).

### Error handling

If any dependency fails, stop immediately and leave progress visible:

```
00:00:01 building gabriel-greeting-swift…
00:00:06 building gabriel-greeting-swift… error: missing module 'Foo'
```

The root holon is **not** attempted — the user sees exactly which dependency failed.

### Auto-build on connect

The same composite resolution runs during auto-build-on-connect. The only difference is the progress erasure rule from §1: on success, all lines are erased; on failure, they stay.

[^3]: A **composite holon** is one that depends on other holons at runtime (declared in its manifest).

## Not in scope

- File watcher / auto-rebuild on source change.
- Parallel builds across independent branches of the dependency graph (future optimization).
