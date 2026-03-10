---
description: Begin work on a task — mark it in-progress in _TASKS.md
---

# Start Task

When starting work on a task, signal it clearly so other agents
and humans know what is being worked on.

## Steps

1. **Update `_TASKS.md`** — add 🔨 to the summary column:
   ```md
   | 01 | [TASK01](./op_v0.3_TASK01_install_no_build.md) | 🔨 `op install --no-build` flag | — |
   ```
2. **If this is the first task in the version**, update the folder
   status using the `/update-version-status` workflow:
   ```bash
   git mv design/op/v0.3 "design/op/💭 v0.3"
   ```
3. Commit and push the marking before starting implementation.

## Rules

- Only one agent should work on a given task at a time.
- Starting a task does not require modifying the task file itself,
  only the `_TASKS.md` index.
