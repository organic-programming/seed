# TDD

The seed now uses `ader` as a local verification runner built around four levels:

- `check`: reusable execution atom
- `suite`: precise scenario with suite-local promotion state
- `catalogue`: isolated verification root containing checks, suites, reports, archives, caches, and lock
- `bouquet`: orchestration across several catalogues

The active root is [`verification/`](verification/README.md).

The current shared black-box scenario source for `op`, `swiftui`, and `flutter` lives under:

- [`verification/catalogues/op/integration/`](verification/catalogues/op/integration)

## Core Rule

A suite is a scenario, not a global inventory.

That has one consequence for promotion:

- promotion and downgrade are **suite-local**
- the same underlying check may be `progression` in one suite and `regression` in another

This keeps `ader` useful for composed TDD:

- develop one scenario in `workspace`
- promote only the checks proven in that scenario
- keep an anti-regression baseline for that scenario
- aggregate several scenarios later with a bouquet

## Layout

```text
verification/
  bouquets/
    local-dev.yaml
    cross-platform.yaml

  catalogues/
    op/
      ader.yaml
      checks.yaml
      suites/
        op-instances.yaml
        op-proxy.yaml
        op-examples.yaml
        op-composites.yaml

    ader/
      ader.yaml
      checks.yaml
      suites/
        ader-core.yaml
        ader-self.yaml
```

[`verification/catalogues/*/ader.yaml`](verification/) contains only catalogue-local runtime defaults:

- storage paths
- default `source`
- default `lane`

There is no default suite at catalogue level. `--suite` is required for `test`, `promote`, and `downgrade`.

[`verification/catalogues/*/checks.yaml`](verification/) defines reusable execution facts:

- `workdir`
- `prereqs`
- `command` xor `script`
- `args`
- `description`

Suite files under [`verification/catalogues/*/suites/`](verification/) define:

- suite-local steps
- `lane` on each suite step
- profiles
- suite-local `archive` policy per profile

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

## TDD Loop

Suite-local lanes still follow the same lifecycle:

```text
progression -> promote -> regression
regression -> downgrade -> progression
```

Typical loop on one scenario:

```bash
ader test verification/catalogues/op \
  --suite op-proxy \
  --profile smoke \
  --lane progression \
  --source workspace

ader promote verification/catalogues/op \
  --suite op-proxy \
  --step integration-dispatch-say-hello-across-transports-go-auto

ader test verification/catalogues/op \
  --suite op-proxy \
  --profile smoke \
  --lane regression \
  --source committed
```

To reset a scenario back into TDD:

```bash
ader downgrade verification/catalogues/op --suite op-proxy --all
```

`ader test` never mutates the suite. Only `promote` and `downgrade` rewrite `steps.<id>.lane` in the selected suite file.

## Bouquets

A bouquet orchestrates several catalogue runs:

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

Run it with:

```bash
ader test-bouquet verification --name local-dev
```

Execution policy:

- entries for the same catalogue run sequentially
- different catalogues may run in parallel
- if one entry fails inside a catalogue, later entries in that same catalogue are marked `SKIP`
- other catalogues continue

## Locking

Each catalogue owns a lock file:

```text
<catalogue>/.artifacts/ader.lock
```

Commands that take the lock:

- `test`
- `promote`
- `downgrade`
- `archive`
- `cleanup`

Read-only commands do not lock:

- `history`
- `show`

This allows safe parallel execution across different catalogues while preventing concurrent mutation inside one catalogue.

## Reports

Child suite runs still write normal catalogue-local reports:

```text
verification/catalogues/<name>/reports/<history-id>/
```

Every child report contains:

- `manifest.json`
- `step-results.json`
- `summary.md`
- `summary.tsv`
- `suite-snapshot.yaml`
- `logs/`
- `promotion.json` and `promotion.md` when a progression run finishes cleanly

History ids use:

```text
<suite>_<source>_<profile>-YYYYMMDD_HH_MM_SS_NNNN
```

Bouquet runs write aggregate reports at:

```text
verification/reports/bouquets/<bouquet-history-id>/
verification/archives/bouquets/<bouquet-history-id>.tar.gz
```

## Commands

Catalogue commands:

```bash
ader test verification/catalogues/ader --suite ader-self --profile smoke
ader promote verification/catalogues/ader --suite ader-self --step holons-clem-ader-unit-root
ader downgrade verification/catalogues/ader --suite ader-self --all
ader history verification/catalogues/ader
ader show verification/catalogues/ader --id <history-id>
ader archive verification/catalogues/ader --latest
ader cleanup verification/catalogues/ader
```

Bouquet commands:

```bash
ader test-bouquet verification --name local-dev
ader history-bouquet verification
ader show-bouquet verification --id <bouquet-history-id>
ader archive-bouquet verification --latest
```

## References

- [`verification/README.md`](verification/README.md)
- [`holons/clem-ader/README.md`](holons/clem-ader/README.md)
- [`holons/clem-ader/api/v1/holon.proto`](holons/clem-ader/api/v1/holon.proto)
