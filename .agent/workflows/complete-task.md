---
description: Mark a completed task with ✅ or ❌, rename file, update _TASKS.md, and create failure report if needed
---

# Complete Task

When a task is finished (success or failure), mark it in three
places: the task file, the filename, and the `_TASKS.md` index.

## Status Emojis

| Emoji | Meaning | When to use |
|---|---|---|
| ✅ | Success | Task fully completed and verified |
| ❌ | Failure | Task failed, blocked, or abandoned |

## Steps — Success (✅)

1. Finish the implementation and tests.
2. Commit and push all changes.
3. **Rename the task file** — append ✅ before the extension:
   ```
   op_v0.3_TASK01_install_no_build.md
   → op_v0.3_TASK01_install_no_build.✅.md
   ```
4. **Add a `## Status` block** near the top of the task file:
   ```md
   ## Status

   Complete ✅

   - Commit: `abc1234`
   - Verify: https://github.com/<owner>/<repo>/commit/abc1234
   ```
5. **Update `_TASKS.md`** — add ✅ to the summary column:
   ```md
   | 01 | [TASK01](./op_v0.3_TASK01_install_no_build.✅.md) | ✅ `op install --no-build` flag | — |
   ```
6. If the task has a checklist, convert completed items to `[x]`.
7. Commit the marking changes and push.

## Steps — Failure (❌)

1. **Rename the task file** — append ❌ before the extension:
   ```
   op_v0.4_TASK03_assembly_manifests.md
   → op_v0.4_TASK03_assembly_manifests.❌.md
   ```
2. **Add a `## Status` block** near the top of the task file:
   ```md
   ## Status

   Failed ❌ — see [failure report](./op_v0.4_TASK03_assembly_manifests.failure.md)
   ```
3. **Create a failure report** — same basename with `.failure.md`:
   ```
   op_v0.4_TASK03_assembly_manifests.failure.md
   ```
   The report must contain:
   - **Summary** — one-line description of what failed
   - **Error** — the actual error message or log output
   - **Context** — what was attempted, commands run, environment
   - **Root cause** — if known
   - **Next steps** — what to try next, or why the task is abandoned
4. **Update `_TASKS.md`** — add ❌ to the summary column:
   ```md
   | 03 | [TASK03](./op_v0.4_TASK03_assembly_manifests.❌.md) | ❌ Create 48 assembly manifests | TASK01, TASK02 |
   ```
5. Commit all changes and push.

## Multiple commits

When the work spans multiple repos or submodules, list every
commit in the status block:

```md
## Status

Complete ✅

- `seed`: `f4974af` | https://github.com/organic-programming/seed/commit/f4974af
- `cpp-holons`: `96d6470` | https://github.com/organic-programming/cpp-holons/commit/96d6470
```

## URL format

Derive the verification URL from the repo remote:

- `git@github.com:organic-programming/seed.git` →
  `https://github.com/organic-programming/seed/commit/<sha>`

## Folder status update

After the **last task** in a version completes, update the version
folder using `/update-version-status`. The folder stays as `v0.X`
throughout execution — it is only renamed once, at the end.

## Done criteria

A task is only done when:

- The filename contains ✅ or ❌
- The task file contains a `## Status` block with commit SHA(s)
- The `_TASKS.md` row is updated with the emoji
- (❌ only) a `.failure.md` report exists alongside the task
- (last task) the version folder status is up to date
