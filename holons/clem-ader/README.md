# Clem Ader

`clem-ader` is the local flight tester holon.

`ader`[^1] takes your code to the runway, checks whether it flies, and keeps the evidence.

- command: `ader`
- seed config dir: [`integration/`](../../integration/)
- seed suite: [`integration/suites/seed.yaml`](../../integration/suites/seed.yaml)

## Scope

`ader` does five things:

1. copy the codebase into an isolated sandbox so tests can't affect your working directory
2. run a configured set of test commands inside that sandbox
3. keep the results as reports and optional archives
4. when all tests pass, propose which ones are ready to become permanent safety checks
5. apply promotions or downgrades to the test configuration when you decide

`ader` does **not**:

- replace native unit or integration tests
- invent product tests by itself
- mutate the repo automatically after a successful run
- act as CI

It is a local verification engine and evidence keeper.

## Concepts

### Steps

A **step** is one test command. It has a name, a working directory, the command to execute, and a **lane** indicating whether it's active TDD or established safety net.

```yaml
steps:
  sdk-go-unit:
    workdir: sdk/go-holons
    prereqs: [go]
    command: go test ./...
    description: Go SDK unit tests
    lane: progression               # TDD — not yet promoted
```

| Field | Description |
|-------|-------------|
| `workdir` | Directory where the command runs (relative to repo root) |
| `command` | Shell one-liner via `bash -lc` |
| `script` | Alternative to command: executable file relative to `workdir` |
| `args` | Arguments passed to `script` |
| `prereqs` | Required tools; step is skipped if missing |
| `description` | Human-readable label |
| `lane` | `progression` (default) or `regression` |

### Lanes

A step is in one of two lanes:

- **`progression`** — active TDD. The step is being developed or fixed. Failure is expected.
- **`regression`** — safety net. The step protects the project. Failure means something broke.

`ader promote` moves a step from `progression` to `regression`.
`ader downgrade` moves it back.

### Profiles

A **profile** is a named group of steps. Different profiles select different subsets for different purposes.

```yaml
profiles:
  quick:
    description: Fast proof for the canonical path
    steps: [ader-unit, sdk-go-unit, example-go-unit, integration-short]
  unit:
    description: All native unit suites
    steps: [ader-unit, sdk-go-unit, sdk-c-unit, ...]
  full:
    description: Unit suites plus integration
    steps: [ader-unit, ..., integration-deterministic]
```

A step can appear in several profiles. Its lane is always the same everywhere (it's on the step, not the profile).

### Suites

A **suite** is a YAML file in `<config-dir>/suites/` that contains all the steps and all the profiles. The seed has one suite: `seed.yaml`.

## Configuration

Two files, both in the config directory (`integration/` for the seed):

### `ader.yaml` — Defaults

```yaml
defaults:
  suite: seed              # which suite to load
  profile: quick           # which profile to run
  lane: regression         # which lane to filter
  source: committed        # where to take the snapshot from
  ladder: [quick, unit, integration, full]
```

When you type `ader test integration` with no flags, these defaults apply.

### `suites/seed.yaml` — Suite Definition

Contains the `steps:` and `profiles:` described above. This is your source of truth for what gets tested.

## The Command

```
ader test integration --suite seed --profile quick --lane regression --source committed
│    │    │              │            │               │                │
│    │    │              │            │               │                └─ what to snapshot
│    │    │              │            │               └─ which lane to run
│    │    │              │            └─ which group of steps
│    │    │              └─ which suite file
│    │    └─ config directory (contains ader.yaml)
│    └─ subcommand
└─ binary
```

| Part | Values | Where defined | Default | Where default set |
|------|--------|---------------|---------|-------------------|
| `<config-dir>` | any path | filesystem | required | — |
| `--suite` | any `<name>` matching `suites/<name>.yaml` | `suites/` directory | `seed` | `ader.yaml` → `defaults.suite` |
| `--profile` | `quick`, `unit`, `integration`, `full`, `stress` | keys under `profiles:` in suite YAML | `quick` | `ader.yaml` → `defaults.profile` |
| `--lane` | `regression`, `progression`, `both` | engine (fixed) | `regression` | `ader.yaml` → `defaults.lane` |
| `--source` | `committed`, `workspace` | engine (fixed) | `committed` | `ader.yaml` → `defaults.source` |

### Other Flags

| Flag | Role | Default |
|------|------|---------|
| `--step-filter` | Regex to match step IDs | all steps |
| `--silent` | Suppress live output | off |
| `--full` | Shorthand for `--profile full` | off |
| `--archive` | Archive policy: `auto`, `always`, `never` | per-profile in `ader.yaml` |
| `--keep-report` | Keep report after archiving | off |
| `--keep-snapshot` | Preserve the snapshot directory | off |

## What Happens When You Run

### 1. Snapshot

Tests never run against your live working directory. Ader first creates a **frozen copy** of the codebase:

```
--source committed                    --source workspace
         │                                     │
         ▼                                     ▼
  git archive HEAD                    copy of the working tree
  (last commit only)                  (includes uncommitted changes)
         │                                     │
         ▼                                     ▼
         ┌────────────────────────────────────┐
         │  /tmp/ader-int-store-<runID>/      │
         │    run/                            │
         │      snapshot/      ← tests run    │
         │        sdk/go-holons/  against     │
         │        holons/grace-op/ this copy  │
         │        ...                         │
         └────────────────────────────────────┘
         deleted after the run (unless --keep-snapshot)
```

- `committed` = reproducible proof. Same commit = same snapshot on any machine. Used for `regression`.
- `workspace` = fast iteration. Includes uncommitted changes. Used for `progression` (TDD).

### 2. Execute

Each step is executed sequentially inside the snapshot. Ader provides environment variables:

| Variable | Value |
|----------|-------|
| `ADER_REPO_ROOT` | snapshot root |
| `ADER_LIVE_REPO_ROOT` | actual repo root |
| `ADER_RUN_ARTIFACTS` | per-run temp directory |
| `ADER_TOOL_CACHE` | shared cache (Go modules, npm, gradle, etc.) |

Each step produces a log file and a result: `PASS`, `FAIL`, or `SKIP` (missing prereqs or workdir).

By default, `ader test` streams live step progress and subprocess output on `stderr`, then prints the final summary on `stdout`. Use `--silent` to keep only the final summary.

### 3. Report

After all steps complete, ader writes a report:

```
reports/<runID>/
├── manifest.json        ← run metadata (suite, profile, lane, commit, pass/fail counts)
├── step-results.json    ← per-step results
├── summary.md           ← human-readable report
├── summary.tsv          ← machine-parseable
├── tool-versions.txt    ← versions of all tools used
├── logs/
│   ├── ader-unit.log
│   └── ...
├── promotion.json       ← if progression clean pass
└── promotion.md         ← if progression clean pass
```

### 4. Promote (optional)

If the run was `--lane progression` and all steps passed, ader generates a promotion proposal. `promotion.md` tells you exactly what to run:

```
ader promote integration --step ader-unit --step sdk-go-unit
```

`promotion.json` / `promotion.md` also include:

- `suggested_command`, which is an `ader promote ...` command
- `cross_tier_suggestions` for the next profile in the ladder `quick -> unit -> integration -> full`

See [TDD.md](../../TDD.md) for the full promotion lifecycle.

### 5. Archive (optional)

Reports can be archived as tarballs tied to a commit hash:

```
archives/<commit-hash>/<runID>-<profile>.tar.gz
```

## Public Surface

Code API and RPC:

- `Test`
- `Archive`
- `Cleanup`
- `Promote`
- `Downgrade`
- `History`
- `ShowHistory`

CLI:

- `ader test <config-dir>`
- `ader archive <config-dir>`
- `ader cleanup <config-dir>`
- `ader promote <config-dir>`
- `ader downgrade <config-dir>`
- `ader history <config-dir>`
- `ader show <config-dir> --id <history-id>`

`history` is the CLI view of `History`.

## All Commands

```bash
# Run tests
ader test integration --suite seed --profile quick
ader test integration --suite seed --profile full
ader test integration --suite seed --profile unit --lane progression --source workspace
ader test integration --suite seed --profile quick --silent

# Promote / downgrade step lanes
ader promote integration --step sdk-go-unit --step example-go-unit
ader promote integration --all
ader downgrade integration --all
ader downgrade integration --step sdk-go-unit --step example-go-unit

# History and reports
ader history integration
ader show integration --id <history-id>

# Archive and cleanup
ader archive integration --latest
ader cleanup integration

# Shell completion
ader completion install zsh
```

## Typical Workflows

### TDD Iteration

```bash
ader test integration --suite seed --profile unit --lane progression --source workspace
# fix failures, re-run, until clean pass
# promotion.md says: ader promote integration --step X
ader promote integration --step X
git commit -am "Promote X to regression"
```

### Full Regression Proof

```bash
ader test integration --suite seed --profile full
ader archive integration --latest
```

### Reset Everything to Progression

```bash
ader downgrade integration --all
```

## Install

The canonical path is: install `op`, install the `clem-ader` holon into `OPBIN`, then enable shell completion.

From the seed:

```bash
go install github.com/organic-programming/grace-op/cmd/op@latest
op env --init
op install ./holons/clem-ader
eval "$(op env --shell)"
ader completion install zsh
exec zsh
```

If `op` is already installed and on `PATH`, the minimum is:

```bash
op env --init
op install ./holons/clem-ader
eval "$(op env --shell)"
ader completion install zsh
exec zsh
```

## Zsh Completion

Install it once:

```bash
ader completion install zsh
exec zsh
```

Quick check:

```bash
ader test <TAB>
ader test integration --suite <TAB>
ader test integration --profile <TAB>
ader show integration --id <TAB>
```

The installed line is:

```zsh
eval "$(ader completion zsh)"
```

`op env --shell` only exposes `OPPATH`, `OPBIN`, and `PATH`. It does not install `ader` completion by itself.

## Why It Exists

The seed is too large and too polyglot to trust short-lived context or cheap CI as the primary verification memory.

`ader` gives the project:

- a frozen local proof
- an execution history
- archived evidence tied to commits
- a clean path from TDD checks to regression tests

## References

- [ADER.md](../../ADER.md) — root-level reference document
- [TDD.md](../../TDD.md) — promotion pipeline methodology
- [integration/](../../integration/) — seed config directory
- [Suite definition](../../integration/suites/seed.yaml) — seed suite

[^1]: Named after Clément Ader (1841–1925), the French engineer who achieved powered flight before the Wright brothers. https://en.wikipedia.org/wiki/Clément_Ader
