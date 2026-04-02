# Clem Ader

`clem-ader` is the local verification holon of the seed.

The public command is `ader`.

The active verification root is [`verification/`](../../verification/README.md).

## Scope

`ader` does four things:

1. freeze the repo into a deterministic snapshot
2. execute a selected suite profile from a catalogue
3. keep reports and optional archives as evidence
4. promote or downgrade suite-local checks between `progression` and `regression`

It also orchestrates several catalogue runs through bouquets.

## Model

`ader` now works with four explicit levels:

- `check`: reusable execution atom
- `suite`: precise scenario
- `catalogue`: isolated verification root
- `bouquet`: orchestration across several catalogues

### Check

Checks live in `<catalogue>/checks.yaml`.

They define only reusable execution facts:

```yaml
checks:
  holons-grace-op-unit-internal-cli:
    workdir: holons/grace-op
    prereqs: [go]
    command: go test -v -count=1 -timeout 5m ./internal/cli
    description: grace-op cli package tests
```

### Suite

Suites live in `<catalogue>/suites/*.yaml`.

A suite owns:

- suite-local steps
- suite-local lanes
- profiles
- per-profile archive policy

Example:

```yaml
description: op proxy scenario

defaults:
  profile: smoke

steps:
  op-build:
    check: holons-grace-op-unit-internal-cli
    lane: regression

  proxy-smoke:
    check: integration-dispatch-say-hello-across-transports-go-auto
    lane: progression

profiles:
  smoke:
    description: canonical path
    archive: never
    steps: [op-build, proxy-smoke]

  full:
    description: broader proof
    archive: auto
    steps: [op-build, proxy-smoke]
```

Promotion state is **suite-local**. The same underlying check may be `progression` in one suite and `regression` in another.

### Catalogue

A catalogue is an isolated config root:

```text
verification/catalogues/op/
  ader.yaml
  checks.yaml
  suites/
  reports/
  archives/
  .artifacts/
  .t
```

`ader.yaml` inside a catalogue keeps only:

- storage paths
- default `source`
- default `lane`

There is no catalogue-level default suite. `--suite` is required for `test`, `promote`, and `downgrade`.

### Bouquet

Bouquets live in [`verification/bouquets/`](../../verification/bouquets).

They orchestrate several suite runs:

```yaml
description: local dev bouquet

defaults:
  source: workspace
  lane: progression
  archive: never

entries:
  - catalogue: op
    suite: op-proxy
    profile: smoke

  - catalogue: ader
    suite: ader-self
    profile: smoke
```

Run them with:

```bash
ader test-bouquet verification --name local-dev
```

## Commands

Catalogue commands:

```bash
ader test verification/catalogues/op --suite op-proxy --profile smoke
ader test verification/catalogues/ader --suite ader-self --profile smoke --lane progression --source workspace

ader promote verification/catalogues/ader --suite ader-self --step holons-clem-ader-unit-root
ader downgrade verification/catalogues/op --suite op-proxy --all

ader history verification/catalogues/op
ader show verification/catalogues/op --id <history-id>
ader archive verification/catalogues/op --latest
ader cleanup verification/catalogues/op
```

Bouquet commands:

```bash
ader test-bouquet verification --name local-dev
ader history-bouquet verification
ader show-bouquet verification --id <bouquet-history-id>
ader archive-bouquet verification --latest
```

## Snapshot and Execution

`ader` never runs against the live working tree directly.

It first creates a frozen snapshot from:

- `committed`: `git archive HEAD`
- `workspace`: a copy of the current working tree

Then it executes the selected suite steps inside that snapshot.

By default `ader test` prints:

- phase banners
- per-step `RUN` / `CMD` / `PASS` / `FAIL`
- wait heartbeats for long silent work
- raw subprocess output

Use `--silent` to suppress live output and keep only the final summary.

## Promotion and Downgrade

`ader test` never mutates suite YAML.

When a `progression` run passes, `ader` writes:

- `promotion.json`
- `promotion.md`

These propose an explicit `ader promote ...` command.

Only:

- `ader promote`
- `ader downgrade`

rewrite `steps.<id>.lane` in the selected suite file.

## Locking

Each catalogue owns an exclusive lock:

```text
<catalogue>/.artifacts/ader.lock
```

Commands that take the lock:

- `test`
- `promote`
- `downgrade`
- `archive`
- `cleanup`

Commands that stay read-only:

- `history`
- `show`

Bouquet workers use the same catalogue lock. This allows parallel execution across different catalogues while serializing work inside one catalogue.

## Reports

Child suite runs write reports under:

```text
verification/catalogues/<catalogue>/reports/<history-id>/
```

Each report includes:

- `manifest.json`
- `step-results.json`
- `summary.md`
- `summary.tsv`
- `suite-snapshot.yaml`
- `logs/`
- `promotion.json` and `promotion.md` when applicable

History ids use:

```text
<suite>_<source>_<profile>-YYYYMMDD_HH_MM_SS_NNNN
```

Bouquet reports live under:

```text
verification/reports/bouquets/<bouquet-history-id>/
verification/archives/bouquets/<bouquet-history-id>.tar.gz
```

## References

- [`../../verification/README.md`](../../verification/README.md)
- [`../../TDD.md`](../../TDD.md)
- [`api/v1/holon.proto`](api/v1/holon.proto)
