---
description: Create a plain or isolated OP git worktree.
---

# Create OP Worktree

Create a worktree through the canonical repository command.

Ask for the branch name if it was not provided. Ask for one of:

- `isolated` for core-dev work on `op`, `ader`, or SDKs.
- `plain` for a normal git worktree that keeps using `~/.op`.
- `cancel` to stop.

Then run:

```sh
op worktree create <branch> --<mode> --json
```

Report the worktree path and, for isolated mode, the local `OPPATH`, `OPBIN`,
whether bootstrap returned `built` or `reused`, and the generated agent config
files.
