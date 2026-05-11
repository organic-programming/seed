# OP Worktree Isolation

`op worktree` creates git worktrees for parallel core-dev work on
`op`, `ader`, and SDKs without sharing the user-global `~/.op` runtime.
`scripts/op-worktree` remains as a compatibility wrapper that delegates to
`op worktree`.

Use it when two branches may rebuild the toolchain at the same time. Plain
git worktrees already isolate source files; `op-worktree` isolates the runtime
home used by `op`, `ader`, SDK caches, and installed binaries.

## Modes

Use an isolated worktree for core-dev work that may run:

```sh
op build op --install
op build clem-ader --install
op sdk build ...
```

Create one with:

```sh
op worktree create feature/X --isolated
```

Use a plain worktree when you only need a normal git worktree and want `op` to
keep using the normal `~/.op` runtime:

```sh
op worktree create feature/X --plain
```

No mode is chosen silently in non-interactive use. Pass `--isolated` or
`--plain`.

## Branch Base Resolution

When you run `op worktree create <new-branch>` and `<new-branch>` does not
yet exist, neither locally nor as `origin/<new-branch>`, the new branch is
created from the **current HEAD of the directory where you invoked the
command**:

| Where you run from | Base of new branch |
|---|---|
| `seed/` (main) on `master` | tip of `master` |
| Worktree on `feature/X` | tip of `feature/X` |
| Detached HEAD | exact commit you're sitting on |

This is git's native `HEAD` semantics. It enables **chaining worktrees**:
start `feature/Y` as a continuation of `feature/X` without merging the
intermediate branch back to `master`.

If `<new-branch>` already exists locally or as `origin/<new-branch>`, it is
checked out as-is in the new worktree and the cwd's HEAD is ignored.

## Codex

For the strongest Codex isolation, launch Codex through the worktree launcher:

```sh
op worktree launch feature/X -- codex
```

The launcher creates or reuses the worktree, bootstraps `<worktree>/.op`, then
starts Codex with:

```sh
OPPATH=<worktree>/.op
OPBIN=<worktree>/.op/bin
PATH=<worktree>/.op/bin:$PATH
```

Inside that Codex session, normal commands such as `op ...` and `ader ...`
resolve against the worktree-local runtime.

## Claude Code

From Claude Code in the main checkout, use the project skill:

```text
/op-worktree feature/X isolated
```

Then open the generated worktree as the Claude Code project. Bootstrap writes
`<worktree>/.claude/settings.local.json`, so new Claude Code sessions attached
to that worktree inherit the local `OPPATH`, `OPBIN`, and `PATH`.

If a Claude Code session was already open before bootstrap, reload or reopen it
on the generated worktree.

## Gemini CLI And Antigravity

Bootstrap writes project-local activation files for additional agents:

- Codex project config: `<worktree>/.codex/config.toml`
- Claude Code project config: `<worktree>/.claude/settings.local.json`
- Gemini CLI: `<worktree>/.gemini/.env`
- Antigravity / VS Code terminals: `<worktree>/.vscode/settings.json`

Open or restart sessions from the generated worktree after bootstrap so the
tool can read its project-local configuration.

## Programmatic Use

`op worktree` is also exposed as the public `Worktree` API/RPC. Programmatic
callers can request `create`, `bootstrap`, or `doctor` and receive the same
activation data and generated config paths as JSON/protobuf fields.

## Idempotence

Bootstrap records `<worktree>/.op/.bootstrap.json`. If the branch, `HEAD`,
local binaries, and managed config files still match, rerunning bootstrap
returns `reused` and does not rebuild `op` or `ader`.

```sh
op worktree bootstrap feature/X --json
```

## Promoting A Plain Worktree

A plain worktree can become isolated later:

```sh
op worktree create feature/X --plain
op worktree bootstrap feature/X --json
```

The existing worktree is reused; only the local `.op` runtime and agent config
files are added.

## Troubleshooting

Check whether the current shell/session is isolated:

```sh
op worktree doctor --json
```

`doctor` succeeds only when `OPPATH` and `OPBIN` point at the current
worktree's `.op` directory and `op` / `ader` resolve from that local `bin`.

Outside an isolated worktree session, `op` keeps its normal behavior and uses
the default `~/.op` runtime.
