---
description: Mark a completed task with ✅, exact commit SHA(s), and direct verification URL(s)
---

# Complete Task

When a task is finished, update the task document so completion is
visible and directly verifiable from the file itself.

## Rules

- Always mark the completed task with `✅`.
- Always record the exact commit SHA after the work is committed.
- Always include a direct commit URL for verification.
- If the work spans multiple repos or a submodule plus parent repo,
  list every relevant commit.
- Do not mark the task complete until the commit exists and has been pushed.

## Required status block

Add a `## Status` section near the top of the task file.

### Single commit

```md
## Status

Complete ✅

- Commit: `abc1234`
- Verify: https://github.com/<owner>/<repo>/commit/abc1234
```

### Multiple commits

Use this when the work spans multiple repositories, submodules, or a
set of commits that are all required to verify the task.

```md
## Status

Complete ✅

- `seed`: `f4974af` | https://github.com/organic-programming/seed/commit/f4974af
- `cpp-holons`: `96d6470` | https://github.com/organic-programming/cpp-holons/commit/96d6470
```

## Steps

1. Finish the implementation and tests.
2. Commit the changes in every affected repo.
3. Push the commit or commits.
4. Update the task file:
   - add `✅`
   - add the commit SHA or SHAs
   - add the direct verification URL or URLs
5. If the task has a checklist, convert completed items to `[x]`.
6. If a submodule changed, include both:
   - the submodule commit
   - the parent repo commit that updates the submodule pointer

## URL format

Derive the verification URL from the repo remote:

- `git@github.com:organic-programming/seed.git` ->
  `https://github.com/organic-programming/seed/commit/<sha>`
- `git@github.com:organic-programming/cpp-holons.git` ->
  `https://github.com/organic-programming/cpp-holons/commit/<sha>`

## Done criteria

A task is only done when the task file contains:

- `✅`
- the exact commit SHA or SHAs
- a direct verification URL for each commit
