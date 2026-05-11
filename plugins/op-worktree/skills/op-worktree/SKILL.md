---
name: op-worktree
description: Use when a user asks Codex to create a git worktree in the seed repository, isolate parallel op/ader/SDK development, or prepare a worktree-local OPPATH runtime.
---

# OP Worktree

Use the repository command `op worktree`; do not reimplement worktree
creation or bootstrap logic.

## Workflow

1. Confirm the request is in the seed repository and `op` is available on
   `PATH`.
2. Determine the requested branch name.
3. Ask the user to choose `isolated`, `plain`, or `cancel` before mutation if
   the mode is not explicit.
4. Run exactly:

```sh
op worktree create <branch> --<mode> --json
```

5. Report the absolute worktree path, mode, `built` vs `reused`, and generated
   agent config files when isolated.

Never silently default the mode. Surface command errors directly.
