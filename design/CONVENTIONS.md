# Design Conventions

Rules and naming patterns for the `design/<subject or holon>/` task and version
folder system.

---

## Version Folders

Each version has its own folder: `v0.3/`, `v0.4/`, ..., `v0.11/`.

A folder contains:
- `_TASKS.md` — task index with status
- `DESIGN_*.md` — design documents for this version
- `<project>_vX.Y_TASK*.md` — task files

### Folder Status Emojis

The folder is **never renamed during execution** — it stays as `v0.X`
while work is in progress. An emoji prefix is applied **only on
completion**:

| Prefix | Meaning |
|---|---|
| `v0.X` | Not started, or in progress |
| `✅ v0.X` | Done — all tasks completed successfully |
| `⚠️ v0.X` | Attention — has failures or needs review |

On re-run, the emoji prefix is stripped back to `v0.X`.

Executed via `/update-version-status`.

---

## Task Files

### Naming

```
grace-op_v<version>_TASK<NN>_<slug>.md
```

Examples:
- `grace-op_v0.3_TASK01_install_no_build.md`
- `grace-op_v0.4_TASK06_composition_recipes.md`

The version is embedded in the filename for automation — any tool
can determine the target version from the filename alone.

Task files are **never renamed**. Status is tracked in the `_TASKS.md`
Status column and in the task file's `## Status` block.

### Task Lifecycle

```
(not started) → 💭 in-progress → ✅ success
                                → ❌ failure + .failure.md
```

| Stage | `_TASKS.md` Status column | Task file | Workflow |
|---|---|---|---|
| Start work | 💭 | unchanged | `/start-task` |
| Success | ✅ | `## Status` block added | `/complete-task` |
| Failure | ❌ | `## Status` block + `.failure.md` | `/complete-task` |

On failure, a companion report is created:
```
grace-op_v0.4_TASK03_assembly_manifests.md           ← task file (unchanged name)
grace-op_v0.4_TASK03_assembly_manifests.failure.md   ← failure report
```

---

## Design Documents

Design documents use the `DESIGN_` prefix and live in their
version folder:

```
v0.4/DESIGN_recipe_ecosystem.md
v0.8/DESIGN_mesh.md
```

They define architecture and rationale. Tasks reference them
but never duplicate their content.

---

## `_TASKS.md` Format

Each version folder has a `_TASKS.md` index with a 5-column table:

```md
| # | File | Summary | Depends on | Status |
|---|---|---|---|---|
| 01 | [TASK01](./grace-op_v0.3_TASK01_foo.md) | `op install` flag | — | — |
| 02 | [TASK02](./grace-op_v0.3_TASK02_bar.md) | composite kind | TASK01 | 💭 |
| 03 | [TASK03](./grace-op_v0.3_TASK03_baz.md) | tier1 runners | — | ✅ |
```

Status values: `—` (not started), `💭` (in progress), `✅`, `❌`, `⚠️`.

File links are **stable** — they never change because task files
are never renamed.

---

## Task Output Staging

Tasks that produce deliverables (spec docs, config files, etc.)
write them to an `output/` folder inside the version folder:

```
v0.3/output/
├── 📝 OP_mesh_section.md
├── 📝 MESH_YAML.md
└── 📝 PROTOCOL_transport_security.md
```

- Output files use a **`📝 ` prefix** in their filename to signal
  they need human review.
- A human must review before moving output to the repo root.
- The task checklist includes `📝 Human review` and
  `Move reviewed files` as final steps.
- The `output/` folder is deleted once all files are merged.

---

## Draft Files

`.DRAFT.md` files store intermediary notes, analysis, and
reasoning produced while elaborating specs and design documents.

```
design/.DRAFT.md                    ← cross-cutting draft
design/grace-op/v0.3/.DRAFT.md     ← version-specific draft
```

- Drafts are working documents — not deliverables.
- They capture the *thinking process*: comparisons, options
  considered, decisions made, and context gathered.
- **Gitignored** — `.DRAFT*` is in `.gitignore`, drafts never
  enter version control.
- Drafts may be deleted once the resulting DESIGN or TASK
  documents are finalized.
- Unlike `📝 ` output files, drafts do not require formal
  review — they are reference material for the author.

---

## Cross-Version References

- Use `../vX.Y/filename.md` for links across versions.
- Prefer version-qualified names in prose: "v0.5 TASK01"
  rather than bare "TASK11".
- Each version folder should be as self-contained as possible.

---

## Workflows

These conventions are executed via agent slash commands:

| Command | Purpose |
|---|---|
| `/start-task` | Set Status column to 💭 in `_TASKS.md` |
| `/complete-task` | Set Status to ✅/❌, inject `## Status` block |
| `/update-version-status` | Set folder emoji on completion (✅/⚠️) |
