---
description: Update a version folder's status emoji (✅ done, ⚠️ needs attention)
---

# Update Version Status

Version folders use an emoji prefix to signal their final state
at a glance. The folder is **never renamed during execution** —
it stays as `v0.X` while work is in progress.

## Status Emojis

| Emoji | Prefix | Meaning |
|---|---|---|
| (none) | `v0.X` | Not started, or in progress |
| ✅ | `✅ v0.X` | Done — all tasks completed successfully |
| ⚠️ | `⚠️ v0.X` | Attention — has failures or needs human review |

## When to Update

| Event | Folder status |
|---|---|
| All tasks in the version are ✅ | `v0.X` → `✅ v0.X` |
| Any task is ❌ or blocked | `v0.X` → `⚠️ v0.X` |
| Re-running a version (new attempt) | `✅ v0.X` or `⚠️ v0.X` → `v0.X` |

## Steps — On Completion

1. Determine the status from the table above.
2. **Rename the folder**:
   ```bash
   # All tasks passed
   mv "design/grace-op/v0.3" "design/grace-op/✅ v0.3"

   # Any task failed
   mv "design/grace-op/v0.4" "design/grace-op/⚠️ v0.4"
   ```
3. Commit and push.

## Steps — On Re-Run

1. **Strip the emoji prefix** to restore stable paths:
   ```bash
   mv "design/grace-op/✅ v0.3" "design/grace-op/v0.3"
   mv "design/grace-op/⚠️ v0.4" "design/grace-op/v0.4"
   ```
2. Strip task file status suffixes (`.✅.md` → `.md`, `.❌.md` → `.md`).
3. Remove `.failure.md` reports from the previous run.
4. Commit and push, then proceed with `/start-task`.

## On ✅ Completion — Release

When a version reaches ✅ (all tasks done), perform two
additional release steps:

5. **Bump `holon.yaml` version** — update the `version:` field
   in the holon's manifest to match the completed version:
   ```yaml
   # before
   version: 0.2.0

   # after
   version: 0.3.0
   ```
6. **Tag the repository** — create an annotated git tag on the
   holon's repository (if it has its own repo):
   ```bash
   git tag -a v0.3.0 -m "grace-op v0.3 — Core Maturity"
   git push origin v0.3.0
   ```
   The tag message should include the version subtitle from the
   roadmap.

> [!IMPORTANT]
> Only tag after the `holon.yaml` bump is committed and pushed.
> The tagged commit must contain the correct version number.

## Rules

- A version is `⚠️` if **any** task in it is ❌ or blocked.
- A version is `✅` only when **every** task in it is ✅.
- A version stays as `v0.X` (no prefix) throughout execution.
- On re-run, always strip the prefix before starting work.
- On ✅, always bump `holon.yaml` and tag before moving on.
