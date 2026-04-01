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
ader completion install zsh
ader downgrade integration --profile unit --all
ader downgrade integration --step sdk-go-unit
ader history integration
ader show integration --id <history-id>
ader archive integration --latest
ader cleanup integration
```

Default behavior is live progress on `stderr` plus a final summary on `stdout`. Add `--silent` if you only want the final summary.

## Install

Typical local setup:

```bash
go install github.com/organic-programming/grace-op/cmd/op@latest
op env --init
op install ./holons/clem-ader
eval "$(op env --shell)"
ader completion install zsh
exec zsh
```

If `op` is already installed:

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

Then the useful checks are:

```bash
ader test <TAB>
ader test integration --suite <TAB>
ader test integration --profile <TAB>
ader show integration --id <TAB>
```

## Files Produced

Reports:

```text
integration/reports/<history-id>/
```

Archives:

```text
integration/archives/<commit-hash>/<history-id>-<profile>.tar.gz
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

They also expose `cross_tier_suggestions` for the next profile in the ladder `quick -> unit -> integration -> full`.

Read that as:

- `progression` = TDD lane
- `regression` = test lane

`ader` does not perform that change automatically.

To reset checks back into TDD, use `ader downgrade ...`. It rewrites the suite YAML immediately and never auto-commits.

## Step Kinds

The suite YAML supports two step kinds:

- `command`: one-line shell command
- `script`: executable file resolved from the step `workdir`

Example from the seed suite:

```yaml
ader-unit:
  workdir: holons/clem-ader
  script: scripts/test-unit.sh
  description: clem-ader self test
```
