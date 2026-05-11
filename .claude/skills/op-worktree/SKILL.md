---
name: op-worktree
description: Create an isolated git worktree for parallel core-dev work on op, ader, or an SDK. Bootstraps <worktree>/.op/ with freshly built op + ader and writes project-local env config for codex, Claude Code, Gemini CLI, and Antigravity so any agent attached to the worktree inherits OPPATH/OPBIN/PATH automatically. Use when the user asks to create a worktree, branch off for parallel toolchain work, or isolate a rebuild of op/ader/SDK.
user-invocable: true
allowed-tools:
  - Bash
  - Read
---

# OP Worktree

Use this skill for `/op-worktree <branch> [isolated|plain]`.

## Contract

1. Verify `op` is available on `PATH`. If it is absent, refuse clearly and do
   not attempt a manual fallback.
2. Parse `$ARGUMENTS` as a required branch name plus optional mode:
   `isolated` or `plain`.
3. If mode is absent, ask the user to choose `isolated` or `plain`. Do not
   default silently and do not run Bash before the user answers.
4. Invoke exactly:

```sh
op worktree create <branch> --<mode> --json
```

5. Parse the JSON output and report:
   - absolute worktree path
   - mode
   - `built` vs `reused`
   - generated agent config files for isolated worktrees
   - next steps: open the generated worktree as the Claude Code project, or use
     `op worktree launch <branch> -- codex` for Codex

Surface command errors verbatim. Do not retry, do not write to `~/.op`, and do
not add cache or idempotence logic in the skill.
