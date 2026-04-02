# TDD

The seed now uses `ader` as a local proof runner built around four levels:

- `check`: reusable execution atom
- `suite`: precise scenario with suite-local promotion state
- `catalogue`: isolated Ader root containing checks, suites, reports, archives, caches, and lock
- `bouquet`: orchestration across several catalogues

The active root is [`ader/`](ader/README.md).

The current shared black-box scenario source for `op`, `swiftui`, and `flutter` lives under:

- [`ader/catalogues/grace-op/integration/`](ader/catalogues/grace-op/integration)

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
ader/
  bouquets/
    local-dev.yaml
    cross-platform.yaml

  catalogues/
    grace-op/
      ader.yaml
      checks.yaml
      suites/
        op-instances.yaml
        op-proxy.yaml
        op-examples.yaml
        op-composites.yaml

    clem-ader/
      ader.yaml
      checks.yaml
      suites/
        ader-core.yaml
        ader-self.yaml
```

[`ader/catalogues/*/ader.yaml`](ader/) contains only catalogue-local runtime defaults:

- storage paths
- default `source`
- default `lane`

There is no default suite at catalogue level. `--suite` is required for `test`, `promote`, and `downgrade`.

[`ader/catalogues/*/checks.yaml`](ader/) defines reusable execution facts:

- `workdir`
- `prereqs`
- `command` xor `script`
- `args`
- `description`

Suite files under [`ader/catalogues/*/suites/`](ader/) define:

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
ader test ader/catalogues/grace-op \
  --suite op-proxy \
  --profile smoke \
  --lane progression \
  --source workspace

ader promote ader/catalogues/grace-op \
  --suite op-proxy \
  --step integration-dispatch-say-hello-across-transports-go-auto

ader test ader/catalogues/grace-op \
  --suite op-proxy \
  --profile smoke \
  --lane regression \
  --source committed
```

To reset a scenario back into TDD:

```bash
ader downgrade ader/catalogues/grace-op --suite op-proxy --all
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
  - catalogue: grace-op
    suite: op-proxy
    profile: smoke

  - catalogue: clem-ader
    suite: ader-self
    profile: smoke
```

Run it with:

```bash
ader test-bouquet ader --name local-dev
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
ader/catalogues/<name>/reports/<history-id>/
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
ader/reports/bouquets/<bouquet-history-id>/
ader/archives/bouquets/<bouquet-history-id>.tar.gz
```

## Commands

Catalogue commands:

```bash
ader test ader/catalogues/clem-ader --suite ader-self --profile smoke
ader promote ader/catalogues/clem-ader --suite ader-self --step holons-clem-ader-unit-root
ader downgrade ader/catalogues/clem-ader --suite ader-self --all
ader history ader/catalogues/clem-ader
ader show ader/catalogues/clem-ader --id <history-id>
ader archive ader/catalogues/clem-ader --latest
ader cleanup ader/catalogues/clem-ader
```

Bouquet commands:

```bash
ader test-bouquet ader --name local-dev
ader history-bouquet ader
ader show-bouquet ader --id <bouquet-history-id>
ader archive-bouquet ader --latest
```

## References

- [`ader/README.md`](ader/README.md)
- [`holons/clem-ader/README.md`](holons/clem-ader/README.md)
- [`holons/clem-ader/api/v1/holon.proto`](holons/clem-ader/api/v1/holon.proto)
