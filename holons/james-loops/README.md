# James Loops

`james-loops` is a queue runner for overnight AI CLI work. Programs still pair briefs with `ader` acceptance gates, but the executing model is now selected through YAML profiles, and a dialectic mode can route each passing attempt through a separate evaluator before the commit is kept.

## Operating Model

The day shift prepares or instantiates programs under `ader/loops/queue/`. The night shift runs `james-loops run`, which promotes the lowest numbered slot into `live/`, creates a dedicated Git branch, executes each step in order, retries when needed, and moves the program to `done/` or `deferred/`. The morning shift reads `ader/loops/morning-report.md`, inspects patches or evaluator feedback, and decides what to re-enqueue.

## Concepts

- Program: a directory with `program.yaml` and any referenced brief files.
- Brief: a Markdown instruction file sent to the selected AI CLI.
- Gate: a shell command, usually `ader test ...`, evaluated against `expect: PASS` or `expect: FAIL`.
- Profile: a YAML definition that selects the driver (`codex`, `gemini`, `ollama`), model, CLI args, and quota probes.
- Solo mode: a single `coder` profile handles the step and the gate decides keep or reject.
- Dialectic mode: a `coder` produces the change, the gate validates it, then an `evaluator` may score or comment before the commit is kept.

## Profiles

Bundled profiles live in `holons/james-loops/.op/profiles/`. Local overrides live in `ader/loops/profiles/`. User-global profiles live in `~/.james-loops/profiles/`. Resolution order is:

1. `ader/loops/profiles`
2. bundled `.op/profiles`
3. `~/.james-loops/profiles`

Useful commands:

```bash
james-loops profile list
james-loops profile show codex-default
james-loops profile validate gemini-flash
```

Create a local profile by writing a YAML file under `ader/loops/profiles/<name>.yaml` using the same schema as the bundled examples.

## Dialectic Mode

In `mode: dialectic`, the program declares both `profiles.coder` and `profiles.evaluator`. A step may add:

```yaml
evaluate:
  brief: briefs/evaluate.md
  threshold: 6.0
  output_field: score
```

Flow:

1. The coder runs on the step brief.
2. The gate stays the binary authority and must pass first.
3. The evaluator receives the brief, a diff summary, gate metadata, and the gate report.
4. If a numeric score is extracted and is below `threshold`, the change is reset and the evaluator feedback is injected into the next coder retry.
5. If `threshold` is `0`, evaluation is feedback-only.

## Directory Layout

```text
ader/loops/
  cookbook/
  queue/
  live/
  deferred/
  done/
  morning-report.md
  run-log.tsv
```

- `queue/NNN/`: pending programs.
- `live/`: the active transient slot, ignored by Git.
- `deferred/NNN/`: blocked programs waiting for human action.
- `done/NNN/`: completed programs with passing steps.

## Commands

```bash
james-loops run
james-loops run --dry-run
james-loops run --coder-profile codex-default
james-loops run --evaluator-profile gemini-flash
james-loops run --root ader/loops --max-retries 5

james-loops enqueue ./path/to/program
james-loops enqueue --from-cookbook simplify

james-loops list
james-loops status
james-loops log refactor

james-loops drop 003
james-loops drop 002 --deferred
james-loops resume
james-loops skip
james-loops abort
james-loops re-enqueue 004

james-loops profile list
james-loops profile show codex-default
james-loops profile validate ollama-llama
```

## Git Strategy

Each program gets a branch named `james-loops/<slot>-<description-slug>`. Linear steps commit once per accepted step using messages like `james-loops: <step-id> PASS (attempt <N>)`. Iteration mode commits only kept iterations using `james-loops: <step-id> iteration <I>/<N> PASS`. Gate failures save a patch then reset to the step start. Evaluator rejections reset without keeping a transient commit.

## Iterative Mode

Set `iterations: N` on a step to switch into autoresearch mode. Each iteration makes one focused change, reruns the same gate, and is either kept or discarded. `max_consecutive_failures` can lock the step early if too many iterations fail in a row. In dialectic mode, the evaluator score governs `Kept` for gate-passing iterations.

## Reports

`ader/loops/morning-report.md` summarizes live, deferred, and done slots, including coder and evaluator profiles plus evaluator score/output when present. `ader/loops/run-log.tsv` is a flat ledger with one line per attempt, including gate result, patch/report paths, and evaluator fields.

## Cookbook

Bundled examples under `ader/loops/cookbook/`:

- `simplify`: solo autoresearch refactor loop using `codex-default`.
- `dialectic-refactor`: coder/evaluator flow using `codex-default` plus `gemini-flash`.
