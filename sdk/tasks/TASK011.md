# TASK011 — Update TODO.md to Reflect Current State

## Context

Depends on: all previous tasks (TASK001–TASK010), or can be done incrementally
after each batch.

`sdk/TODO.md` is **stale** — its "Current state per SDK" table still shows
`discover` and `connect` as ❌ across all SDKs, even though `discover` is
100% complete and `connect` is done in 7 of 14 SDKs.

## What to do

### 1. Update the table in `sdk/TODO.md`

Update the "Current state per SDK" table (line 29–44) to reflect the actual
state of each SDK. Use the following status markers:

- ✅ — module is implemented and tested
- ❓ — partial implementation, needs verification
- ❌ — not implemented

After all connect tasks are done, the table should show ✅ for `discover`
and `connect` in every row.

### 2. Update `sdk/TODO_STATUS_REPORT.md`

Replace the existing report with a current snapshot:
- Mark `discover` section as "Complete — all SDKs".
- Update `connect` section with current completions.
- Update recipe migration section.
- Update hello-world section.
- Update the practical summary at the bottom.

### 3. Mark completed TODO files

If all items in a TODO file are done, add a `## Status: Complete` header
at the top:
- `TODO_DISCOVER.md` — should already be marked complete.
- `TODO_CONNECT.md` — mark complete when all 14 SDKs have connect.
- `TODO_MIGRATE_RECIPES.md` — mark complete when all recipes are migrated.

## Rules

- Be factual — only mark items as ✅ if the file actually exists and tests pass.
- Verify each claim by checking for the file on disk.
- Do not modify SDK source code in this task — documentation only.
