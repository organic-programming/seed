---
description: Mark a completed task with ✅ or ❌, update _TASKS.md Status column, inject ## Status in the task file
---

# Complete Task

When a task is finished (success or failure), mark it in two
places: the task file (`## Status` block) and the `_TASKS.md`
Status column. Task files are **never renamed**.

## Status Emojis

| Emoji | Meaning | When to use |
|---|---|---|
| 💭 | In progress | Task is being worked on |
| ✅ | Success | Task fully completed and verified |
| ❌ | Failure | Task failed, blocked, or abandoned |
| ⚠️ | Attention | Completed but with warnings or caveats |

## Steps — Success (✅)

1. Finish the implementation and tests.
2. Commit and push all changes.
3. **Add a `## Status` block** in the task file:
   ```md
   ## Status

   ✅ Complete

   - Commit: `abc1234`
   - Verify: https://github.com/<owner>/<repo>/commit/abc1234
   ```
4. **Update `_TASKS.md` Status column** to `✅`:
   ```md
   | 01 | [TASK01](./op_v0.3_TASK01_install_no_build.md) | `op install --no-build` flag | — | ✅ |
   ```
5. If the task has a checklist, convert completed items to `[x]`.
6. Commit the marking changes and push.

## Steps — Failure (❌)

1. **Add a `## Status` block** in the task file:
   ```md
   ## Status

   ❌ Failed — see [failure report](./op_v0.4_TASK03_assembly_manifests.failure.md)
   ```
2. **Create a failure report** — same basename with `.failure.md`:
   ```
   op_v0.4_TASK03_assembly_manifests.failure.md
   ```
   The report must contain:
   - **Summary** — one-line description of what failed
   - **Error** — the actual error message or log output
   - **Context** — what was attempted, commands run, environment
   - **Root cause** — if known
   - **Next steps** — what to try next, or why the task is abandoned
3. **Update `_TASKS.md` Status column** to `❌`:
   ```md
   | 03 | [TASK03](./op_v0.4_TASK03_assembly_manifests.md) | Create 48 assembly manifests | TASK01, TASK02 | ❌ |
   ```
4. Commit all changes and push.

## Multiple commits

When the work spans multiple repos or submodules, list every
commit in the status block:

```md
## Status

✅ Complete

- `seed`: `f4974af` | https://github.com/organic-programming/seed/commit/f4974af
- `cpp-holons`: `96d6470` | https://github.com/organic-programming/cpp-holons/commit/96d6470
```

## URL format

Derive the verification URL from the repo remote:

- `git@github.com:organic-programming/seed.git` →
  `https://github.com/organic-programming/seed/commit/<sha>`

## Folder status update

After the **last task** in a version completes, update the version
folder using `/update-version-status`.

## Done criteria

A task is only done when:

- The task file contains a `## Status` block with commit SHA(s)
- The `_TASKS.md` Status column is updated with the emoji
- (❌ only) a `.failure.md` report exists alongside the task
- (last task) the version folder status is up to date
