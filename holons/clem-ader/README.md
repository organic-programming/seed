# Clem Ader

`clem-ader` is the local verification holon.

- command: `ader`
- seed config dir: [`integration/`](../../integration/)
- seed suite: [`integration/suites/seed.yaml`](../../integration/suites/seed.yaml)

## Scope

`ader` does four things:

1. freeze a snapshot of a repo state
2. execute a configured verification suite from that snapshot
3. store reports and optional archives
4. propose promotion from `progression` (TDD) to `regression` (tests)

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
- `ListRuns`
- `ShowRun`

CLI:

- `ader test <config-dir>`
- `ader archive <config-dir>`
- `ader cleanup <config-dir>`
- `ader history <config-dir>`
- `ader show <config-dir> --run <id>`

`history` is the CLI view of `ListRuns`.

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
ader history integration
ader show integration --run <run-id>
ader archive integration --latest
ader cleanup integration
```

By default, `ader test` streams live step progress and subprocess output on `stderr`, then prints the final summary on `stdout`. Use `--silent` to keep only the final summary.

## Outputs

Reports:

```text
integration/reports/<run-id>/
```

Archives:

```text
integration/archives/<commit-hash>/<run-id>-<profile>.tar.gz
```

Promotion proposals, when applicable:

- `promotion.json`
- `promotion.md`

## Why It Exists

The seed is too large and too polyglot to trust short-lived context or cheap CI as the primary verification memory.

`ader` gives the project:

- a frozen local proof
- an execution history
- archived evidence tied to commits
- a clean path from TDD checks to regression tests
