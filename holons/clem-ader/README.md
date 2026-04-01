# Clem Ader

`clem-ader` is the local verification holon.

- command: `ader`
- seed config dir: [`integration/`](../../integration/)
- seed suite: [`integration/suites/seed.yaml`](../../integration/suites/seed.yaml)

## Scope

`ader` does five things:

1. freeze a snapshot of a repo state
2. execute a configured verification suite from that snapshot
3. store reports and optional archives
4. propose promotion from `progression` (TDD) to `regression` (tests)
5. downgrade `regression` steps back to `progression` when a profile must be reset

`ader` does **not**:

- replace native unit or integration tests
- invent product tests by itself
- mutate the repo automatically after a successful run
- act as CI

It is a local verification engine and evidence keeper.

## Public Surface

Code API and RPC:

- `Test`
- `Archive`
- `Cleanup`
- `Downgrade`
- `History`
- `ShowHistory`

CLI:

- `ader test <config-dir>`
- `ader archive <config-dir>`
- `ader cleanup <config-dir>`
- `ader downgrade <config-dir>`
- `ader history <config-dir>`
- `ader show <config-dir> --id <history-id>`

`history` is the CLI view of `History`.

## Core Model

`ader` is config-driven.

It loads:

- `ader.yaml`
- `suites/<name>.yaml`

The suite defines:

- named steps
- profiles such as `quick`, `unit`, `integration`, `full`, `stress`
- two lanes:
  - `progression`: TDD checks not promoted yet
  - `regression`: tests that now protect the project

The seed uses repo-local config files under [`integration/`](../../integration/), but the same tool can be used in another project with another config dir.

## Commands

```bash
ader test integration --suite seed --profile quick
ader test integration --suite seed --profile full
ader test integration --suite seed --profile quick --lane progression --source workspace
ader test integration --suite seed --profile quick --silent
ader completion install zsh
ader downgrade integration --profile unit --all
ader downgrade integration --step sdk-go-unit --step example-go-unit
ader history integration
ader show integration --id <history-id>
ader archive integration --latest
ader cleanup integration
```

By default, `ader test` streams live step progress and subprocess output on `stderr`, then prints the final summary on `stdout`. Use `--silent` to keep only the final summary.

## Flag Reference

```
ader test integration --suite seed --profile quick --lane regression --source committed
         │              │             │              │                │
         config-dir     suite         profile        lane             source
```

| Flag | Role | Values | Default |
|------|------|--------|---------|
| `<config-dir>` | Directory containing `ader.yaml` and `suites/` | any path | required |
| `--suite` | Which suite file to load from `suites/<name>.yaml` | any suite in `suites/` | from `ader.yaml` (`seed`) |
| `--profile` | Which group of steps to run | `quick`, `unit`, `integration`, `full`, `stress` | `quick` |
| `--lane` | Which subset of the profile to run | `regression`, `progression`, `both` | `regression` |
| `--source` | Where the snapshot comes from | `committed`, `workspace` | `committed` |
| `--step-filter` | Regex to filter step IDs | any regex | none |
| `--silent` | Suppress live progress | flag | off |
| `--archive` | Archive policy | `auto`, `always`, `never` | per-profile in `ader.yaml` |
| `--keep-report` | Keep report dir after archiving | flag | off |
| `--keep-snapshot` | Keep the frozen snapshot after the run | flag | off |
| `--full` | Shorthand for `--profile full` | flag | off |

## Snapshot Sources

Tests never run against the live working directory. Every run creates a frozen snapshot in a temporary directory. `--source` controls what gets copied:

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
         │
         ▼
       deleted after the run (unless --keep-snapshot)
```

- `committed` = reproducible proof. Two machines with the same commit get the same snapshot. Used for `regression`.
- `workspace` = fast iteration. Test uncommitted changes immediately. Used for `progression` (TDD).


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

## Step Types

Each suite step defines exactly one execution mode:

- `command`: shell one-liner executed with `bash -lc`
- `script`: executable file resolved relative to the step `workdir`

Example:

```yaml
steps:
  ader-unit:
    workdir: holons/clem-ader
    script: scripts/test-unit.sh
    description: clem-ader self test
```

## Outputs

Reports:

```text
integration/reports/<history-id>/
```

Archives:

```text
integration/archives/<commit-hash>/<history-id>-<profile>.tar.gz
```

Promotion proposals, when applicable:

- `promotion.json`
- `promotion.md`

`promotion.json` / `promotion.md` also include `cross_tier_suggestions` for the next profile in the ladder `quick -> unit -> integration -> full`.

## Why It Exists

The seed is too large and too polyglot to trust short-lived context or cheap CI as the primary verification memory.

`ader` gives the project:

- a frozen local proof
- an execution history
- archived evidence tied to commits
- a clean path from TDD checks to regression tests
