# OP Design Conventions

Rules and naming patterns for the `design/grace-op/` task and version
folder system.

---

## Version Folders

Each version has its own folder: `v0.3/`, `v0.4/`, ..., `v0.10/`.

A folder contains:
- `_TASKS.md` — task index with status
- `DESIGN_*.md` — design documents for this version
- `op_vX.Y_TASK*.md` — task files

### Folder Status Emojis

The folder name carries a status emoji prefix:

| Prefix | Meaning |
|---|---|
| `v0.X` | Not started |
| `💭 v0.X` | Running — work in progress |
| `✅ v0.X` | Done — all tasks completed successfully |
| `⚠️ v0.X` | Attention — blocked or has failures |

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

### Task Status Emojis

On completion, the filename gains an emoji before `.md`:

| Suffix | Meaning |
|---|---|
| `.md` | Not started or in progress |
| `.✅.md` | Completed successfully |
| `.❌.md` | Failed |

On failure, a companion report is created:
```
grace-op_v0.4_TASK03_assembly_manifests.❌.md       ← task file
grace-op_v0.4_TASK03_assembly_manifests.failure.md  ← failure report
```

### Task Lifecycle

```
(not started) → 🔨 in-progress → ✅ success
                               → ❌ failure + .failure.md
```

| Stage | `_TASKS.md` marker | Filename | Workflow |
|---|---|---|---|
| Start work | 🔨 in summary | unchanged | `/start-task` |
| Success | ✅ in summary | `.✅.md` | `/complete-task` |
| Failure | ❌ in summary | `.❌.md` | `/complete-task` |

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

Each version folder has a `_TASKS.md` index:

```md
| # | File | Summary | Depends on |
|---|---|---|---|
| 01 | [TASK01](./grace-op_v0.3_TASK01_foo.md) | `op install` flag | — |
| 02 | [TASK02](./grace-op_v0.3_TASK02_bar.md) | 🔨 composite kind | TASK01 |
| 03 | [TASK03](./grace-op_v0.3_TASK03_baz.✅.md) | ✅ tier1 runners | — |
```

Status emojis are prepended to the summary column.

---

## Task Output Staging

Tasks that produce deliverables (spec docs, config files, etc.)
write them to an `output/` folder inside the version folder:

```
v0.3/output/
├── OP_mesh_section.md         ← draft for OP.md §11
├── MESH_YAML.md               ← new spec doc
└── PROTOCOL_transport_security.md
```

- Output files are **📝 staging only** — never the final location.
- A human must review before moving output to the repo root.
- The task checklist includes `📝 Human review` and
  `Move reviewed files` as final steps.
- The `output/` folder is deleted once all files are merged.

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
| `/start-task` | Mark a task 🔨 in `_TASKS.md`, set folder to 💭 |
| `/complete-task` | Mark ✅/❌, rename file, update folder status |
| `/update-version-status` | Set folder emoji (💭/✅/⚠️) |
