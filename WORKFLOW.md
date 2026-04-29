# WORKFLOW.md

How work moves through this repository today. Descriptive, not prescriptive — this reflects current practice while Organic Programming is still in exploratory elaboration. Expect this file to change as the paradigm stabilizes and as collaborators (human or agent) are added.

## Branches

- **`master`** — the trunk. All day-to-day evolution lands here through PRs. Push frequency is high (several times per day, sometimes per hour during active chantiers).
- **`<actor>/<short-desc>`** — ephemeral feature branches used to deliver work. Examples: `codex/sdk-prebuilts-phase4`, `bpds/post-saint-emilion-closeout`, `agent/<task>`. Cut from the latest `master`, merged back into `master` via PR, then deleted.

No other permanent branches exist. The previous `dev` integration branch was retired in 2026-04 (see [`docs/adr/git-workflow-trunk-based.md`](docs/adr/git-workflow-trunk-based.md)).

## Who merges what

The composer is the only merge captain. Concretely:

- Agents (Codex sessions, Claude Code, others) never merge into `master` themselves.
- They push their feature branch to origin, open a PR against `master`, and stop there.
- The composer reviews the PR (locally, on GitHub UI, or with another agent's help), then admin-merges via `gh pr merge --merge --delete-branch --admin`.
- Releases are explicit `git tag` events on `master`, not branch promotions.

## How agents work

Agents operate either in isolated git worktrees branched from the latest `master`, or via fresh shallow clones (depending on the agent's preference). The patterns observed in practice:

### Worktree pattern (long-running tasks)

```bash
git worktree add ../seed-<short-desc> -b <actor>/<short-desc> origin/master
cd ../seed-<short-desc>
# ... do the work, commit on the feature branch ...
git push -u origin <actor>/<short-desc>
gh pr create --base master --head <actor>/<short-desc>
```

The composer then admin-merges. After merge, the worktree can be removed:

```bash
git worktree remove ../seed-<short-desc>
git branch -d <actor>/<short-desc>     # after the remote branch is auto-deleted by gh pr merge --delete-branch
```

### Shallow-clone pattern (short docs / single-file tweaks)

```bash
git clone --depth 5 --branch master <repo-url> /tmp/seed-<short-desc>
cd /tmp/seed-<short-desc>
git checkout -b <actor>/<short-desc>
# ... do the work, commit ...
git push -u origin <actor>/<short-desc>
gh pr create --base master --head <actor>/<short-desc>
# ... composer admin-merges ...
rm -rf /tmp/seed-<short-desc>
```

Both patterns are valid. The worktree pattern is preferred for multi-day tasks where the agent revisits the same workspace; the shallow-clone pattern is preferred for one-shot deliverables.

Long-running worktrees should be rebased on the latest `origin/master` periodically; otherwise drift becomes painful when the PR is opened.

## CI on PRs

Workflows in `.github/workflows/` trigger on `pull_request: branches: [master]`. The main pre-merge path is `.github/workflows/pipeline.yml`: six sequential tool-gate jobs, conditional SDK prebuilt rebuilds, composite coverage, and per-SDK deep tests. Popok-bound jobs may queue when popok is busy with other chantiers; hosted jobs continue to run normally.

The composer can still admin-merge when operationally necessary, especially while popok is offline, but the expected signal is the ordered pipeline. This is a pragmatic choice while the project is in elaboration; it will tighten when external contributors are added.

## What is intentionally not formalized yet

The following are deferred on purpose. Adding them now would impose rigidity on a paradigm still in trial-and-error:

- **No mandatory CI gate on merge.** Heavy CI signals exist but do not block (the composer admin-merges through `BLOCKED` status when standard checks are green).
- **No enforced commit message convention.** Messages are expected to be informative; some PRs use conventional-commits prefixes (`feat(SDK-zig):`, `chore(examples):`) by habit, none enforce it.
- **No formal attribution scheme for agent commits.** Codex commits use the agent's configured author identity; Claude Code commits use the composer's identity (since the composer drives them). Traceability is handled ad hoc.
- **Limited branch protection rules on `master`.** A PR is required for merges (no direct push), but admin override is permitted. Required reviewers are not enforced (the composer is the sole captain).
- **No release automation.** Tags are created manually when a release event warrants it.

These will be revisited when any of the following happens:

- External contributors are invited.
- The public history is rewritten to its canonical form.
- The downstream flows (private applications consuming `seed`, and the bridge between them and `seed`) are designed.

## Relationship to other repositories

`seed` is the public paradigm repository on GitHub. It is consumed by private application repositories hosted elsewhere. The workflow governing how work moves between `seed` and its consumers is a separate concern and is not documented here yet.
