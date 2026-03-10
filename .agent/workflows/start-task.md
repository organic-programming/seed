---
description: Begin work on a task — mark it in-progress in _TASKS.md
---

# Start Task

When starting work on a task, signal it clearly so other agents
and humans know what is being worked on.

## Steps

1. **Reset version folder if re-running** — if the folder has an emoji
   prefix from a previous run, strip it:
   ```bash
   mv "design/grace-op/✅ v0.3" "design/grace-op/v0.3"
   ```
2. **Reset previous run artifacts if re-running**:
   - Clear all Status column values in `_TASKS.md` back to `—`
   - Remove `## Status` blocks from task files
   - Delete `.failure.md` reports
3. **Update `_TASKS.md` Status column** — set the task's status to `💭`:
   ```md
   | 01 | [TASK01](./op_v0.3_TASK01_install_no_build.md) | `op install --no-build` flag | — | 💭 |
   ```
4. Commit and push the marking before starting implementation.

## Rules

- Only one agent should work on a given task at a time.
- Starting a task does not require modifying the task file itself,
  only the `_TASKS.md` index.
- The version folder is **never renamed during execution** — it stays
  as `v0.X` throughout. Emoji prefixes are applied only on completion.
- Task files are **never renamed** — status is tracked in the Status
  column and in the task file's `## Status` block.
