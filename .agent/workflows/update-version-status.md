---
description: Update a version folder's status emoji (💭 running, ✅ done, ⚠️ needs attention)
---

# Update Version Status

Version folders use an emoji prefix to signal their current state
at a glance.

## Status Emojis

| Emoji | Prefix | Meaning |
|---|---|---|
| (none) | `v0.X` | Not started — no work has been done yet |
| 💭 | `💭 v0.X` | Running — at least one task is in progress |
| ✅ | `✅ v0.X` | Done — all tasks completed successfully |
| ⚠️ | `⚠️ v0.X` | Attention — human review needed (blocked or has failures) |

## When to Update

| Event | New folder status |
|---|---|
| First task in a version starts | (none) → `💭 v0.X` |
| A task fails (❌) | any → `⚠️ v0.X` |
| Work is blocked, waiting for human decision | any → `⚠️ v0.X` |
| Human resolves the issue, work resumes | `⚠️ v0.X` → `💭 v0.X` |
| Last task in a version completes (all ✅) | `💭 v0.X` → `✅ v0.X` |

## Steps

1. Determine the new status from the table above.
2. **Rename the folder** with `git mv`:
   ```bash
   # Starting work
   git mv design/grace-op/v0.3 "design/grace-op/💭 v0.3"

   # All tasks done
   git mv "design/grace-op/💭 v0.3" "design/grace-op/✅ v0.3"

   # Failure or blocked
   git mv "design/grace-op/💭 v0.4" "design/grace-op/⚠️ v0.4"
   ```
3. **Update references** — after renaming a folder, update:
   - `ROADMAP.md` — the `**Tasks:**` link
   - `INDEX.md` — the version folder link
   - Any cross-version `../vX.Y/` references in other folders
4. Commit and push.

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
- A version stays `💭` as long as work is progressing normally.
- Never skip from (none) directly to ✅ — always go through 💭.
- On ✅, always bump `holon.yaml` and tag before moving on.
