# Codex Loops

`codex-loops` is a queue runner for overnight Codex work. It takes prepared programs made of briefs and acceptance gates, executes them one step at a time with `codex exec`, validates each step with `ader`, rewinds failed attempts, defers blocked programs, and writes a morning report the human can review when the night run ends.

## Operating model

The day shift writes or instantiates programs, edits briefs, and queues work under `ader/codex-loops/queue/`. The night shift runs `codex-loops run`, which promotes the lowest numbered program into `live/`, creates a dedicated Git branch, executes each step in sequence, retries failed gates, and either moves the program into `done/` or defers it. The morning shift reads `ader/codex-loops/morning-report.md`, inspects any deferred reports or patches, and decides what to re-enqueue.

## Core concepts

- Program: a directory containing `program.yaml` plus any referenced brief files. The program declares a description, ordered steps, and retry policy.
- Brief: a Markdown instruction file passed to `codex exec --full-auto -a never`.
- Gate: a shell command, usually an `ader test ...` invocation, whose exit status is evaluated against `expect: PASS` or `expect: FAIL`.
- Cookbook: a reusable template stored under `ader/codex-loops/cookbook/` that can be instantiated into the queue with `codex-loops enqueue --from-cookbook <name>`.

## Directory layout

```text
ader/codex-loops/
  cookbook/
  queue/
  live/
  deferred/
  done/
  morning-report.md
```

- `queue/NNN/`: pending programs waiting to be run.
- `live/`: the transient active program. This directory is never committed.
- `deferred/NNN/`: programs blocked after all retries or explicitly aborted.
- `done/NNN/`: completed programs with passing steps.

## Commands

```bash
codex-loops run
codex-loops run --dry-run
codex-loops run --root ader/codex-loops --max-retries 5

codex-loops enqueue ./path/to/program
codex-loops enqueue --from-cookbook fix-unit-bug

codex-loops list
codex-loops status

codex-loops drop 003
codex-loops drop 002 --deferred

codex-loops resume
codex-loops skip
codex-loops abort

codex-loops re-enqueue 004
codex-loops log lint-step
```

## Git strategy

Each program gets its own branch named `codex-loops/<slot>-<description-slug>`. Every passing step is committed immediately with a message shaped like `codex-loops: <step-id> PASS (attempt <N>)`. If a gate fails, the runner saves a patch for the failed attempt, then resets the worktree back to the step-start commit before retrying the same brief.

## Morning report

The morning report is Markdown written to `ader/codex-loops/morning-report.md`. It includes a date header, a section for live, deferred, and done programs, and a per-program table of `step | result | attempts | gate report path`. Deferred programs also include the last failing gate report so the human can decide whether to edit the brief, fix the environment, or re-enqueue the work later.

## Codex invocation

Each attempt uses:

```bash
codex exec --full-auto -a never -C <repo-root> "<brief>"
```

On retries, the previous gate report is appended under a `--- PREVIOUS ATTEMPT FAILED ---` separator so Codex sees the failure context before producing the next attempt.
