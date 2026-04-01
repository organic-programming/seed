# Integration

`integration/` is the seed configuration root for `ader`.

It is not just a folder of tests. It defines the local verification system.

## What Lives Here

- [`ader.yaml`](./ader.yaml)
  Global defaults and storage paths.
- [`suites/seed.yaml`](./suites/seed.yaml)
  The seed verification suite.
- [`tests/`](./tests/)
  The black-box `op` integration suite.

## Scope

This config drives two loops:

- `progression`
  workspace proof, used during TDD before a check is promoted
- `regression`
  committed proof, archiveable, used once the check has become a project test

The suite does not replace native tests. It orchestrates them.

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

Default behavior is live progress on `stderr` plus a final summary on `stdout`. Add `--silent` if you only want the final summary.

## Files Produced

Reports:

```text
integration/reports/<run-id>/
```

Archives:

```text
integration/archives/<commit-hash>/<run-id>-<profile>.tar.gz
```

Deterministic local residue:

```text
integration/.artifacts/
integration/.t
```

## Promotion

When a `progression` run passes completely, `ader` may write:

- `promotion.json`
- `promotion.md`

These files propose how to move step references from the `progression` lane to the `regression` lane in the suite YAML.

Read that as:

- `progression` = TDD lane
- `regression` = test lane

`ader` does not perform that change automatically.
