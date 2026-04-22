# WORKFLOW.md

How work moves through this repository today. Descriptive, not prescriptive — this reflects current practice while Organic Programming is still in exploratory elaboration. Expect this file to change as the paradigm stabilizes and as collaborators (human or agent) are added.

## Branches

- **`master`** — stable snapshots. Updated rarely, only by promotion from `dev`. Treat as "the current reference state of the paradigm."
- **`dev`** — the working branch. All day-to-day evolution lands here. Push frequency is high (several times per day).
- **`agent/<short-desc>`** — ephemeral branches used by agents working in git worktrees. Cut from `dev`, merged back into `dev` by the composer, then removed.

No other permanent branches exist.

## Who merges what

The composer is the only merge captain. Concretely:

- Agents never merge into `dev` themselves. They commit on their `agent/*` branch inside their worktree and stop there.
- The composer reviews agent work locally, then performs the merge into `dev`.
- Promotion from `dev` to `master` is a deliberate, infrequent act by the composer.

## How agents work

Agents operate in isolated git worktrees created from `dev`:

```bash
git worktree add ../seed-<short-desc> -b agent/<short-desc> dev
```

Each worktree is scoped to one task. When the task is done, the composer reviews the diff, merges into `dev` in the main checkout, and removes the worktree:

```bash
git worktree remove ../seed-<short-desc>
git branch -d agent/<short-desc>   # after merge
```

Agent worktrees should be synced from `dev` periodically if the task spans multiple days; otherwise drift becomes painful.

## What is intentionally not formalized yet

The following are deferred on purpose. Adding them now would impose rigidity on a paradigm still in trial-and-error:

- **No merge request / pull request process.** The composer reviews locally.
- **No enforced commit message convention.** Messages are expected to be informative but not formatted.
- **No branch protection rules on GitHub.** Not needed while the composer is the sole merge captain.
- **No mandatory CI gate on merge.** CI signals exist but do not block.
- **No formal attribution scheme for agent commits.** Traceability is handled ad hoc for now.

These will be revisited when any of the following happens:

- External contributors are invited.
- The public history is rewritten to its canonical form.
- The downstream flows (private applications consuming `seed`, and the bridge between them and `seed`) are designed.

## Relationship to other repositories

`seed` is the public paradigm repository on GitHub. It is consumed by private application repositories hosted elsewhere. The workflow governing how work moves between `seed` and its consumers is a separate concern and is not documented here yet.
