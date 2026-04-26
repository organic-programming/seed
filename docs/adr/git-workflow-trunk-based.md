# ADR: trunk-based development on `master`

Status: proposed  
Date: 2026-04-26  
Decision: drop the `dev` integration branch; feature branches target `master` directly.

---

## Context

Until 2026-04-26, the seed repo used a two-branch workflow:

```
feature branch (e.g. codex/sdk-prebuilts-phase3)
    ↓ PR
dev (rolling integration branch)
    ↓ periodic dev → master PR
master (release branch)
```

This was inherited from earlier in the project's life when:
- The chantiers were less mature and `dev` provided a buffer.
- Release events were rare and warranted explicit promotion.
- CI workflows were untested and needed a "trial" branch.

Today's reality:
- Chantiers are well-structured; phase PRs land regularly.
- The composer admin-merges per phase within minutes; the `dev` buffer adds latency without filtering anything.
- CI workflows (especially the new prebuilts pipeline) trigger on `pull_request: branches: [master]`, which means they only run on the rare `dev → master` event. Pre-merge validation of CI changes is therefore artificially delayed.
- The two-branch model adds a coordination tax: every prompt, every doc, every Codex session embeds the assumption "PRs target dev, not master".

The cost has outgrown the benefit. Trunk-based development on `master` aligns with current operational reality.

## Decision

1. **All feature branches target `master` directly.** The `gh pr create --base master` becomes the default.
2. **`dev` is retired.** After a final consolidation merge (`dev → master`), the branch is left as a frozen bookmark or deleted. New PRs do not target it.
3. **Releases are tagged on `master`**, not differentiated by branch position.
4. **Branch protection on `master`** is tightened: required PR review (admin can bypass for solo work), required CI checks, no direct pushes outside protected admin accounts.

## Consequences

### Positive

- **CI prebuilts workflow runs on every PR**, not just on rare release events. Phase 3's doubt about deferred validation goes away naturally.
- **Composer cognitive load reduced**: one branch to track instead of two.
- **Codex prompts simplified**: every reference to `dev` becomes `master`.
- **Faster feedback loop**: a feature merges to master without an additional gate.
- **Releases become explicit tag events** rather than implicit branch position.

### Negative

- **Master is no longer a "stable" branch in the traditional sense.** Anything merged is "live" — it's the latest, but it's not necessarily a release.
- **External consumers** (if any) can no longer assume "master = released stable"; they must consult tags. This is the trunk-based development cultural shift; standard in modern OSS.
- **One-time migration cost**: update prompts, docs, memory, retarget any open PRs.

### Neutral

- No CI YAML changes required: existing workflows already target `master` via `pull_request: branches: [master]`. They will simply trigger more often.

## Migration plan

Sequenced to avoid disrupting active chantiers:

1. **Wait for prebuilts Phase 3 (PR #36) to merge on `dev`.** This is in flight; aborting it to retarget would lose work.
2. **Open a `dev → master` consolidation PR.** Merges all current `dev` state onto `master`. This becomes the last `dev → master` PR.
3. **Update prompts**:
   - `.codex/sdk-prebuilts-prompt.md` — replace `dev` with `master` in §kickoff and constraints.
   - `.codex/ader-acceleration-prompt.md` — same.
4. **Update memory note**: `PRs target master, not dev` (was `PRs target dev, not master`).
5. **Update referenced docs**:
   - `docs/specs/sdk-prebuilts.md` §11.x mentions of `dev`.
   - `docs/st_emilion/04_closeout.md` references to `dev → master` transition (no longer applies).
   - Any `OP_BUILD.md` or `CLAUDE.md` mentions of `dev`.
6. **Update branch protection on `master`** in GitHub Settings.
7. **Optionally**: delete or freeze `dev`. Recommend frozen bookmark for one release cycle, then delete.

For active Codex sessions:
- The prebuilts session: tell it to target `master` from Phase 4 onwards.
- Future sessions (e.g., ader-acceleration): the prompt already updated will use `master` from the start.

## Acceptance

The migration is complete when:

- [ ] Final `dev → master` consolidation PR merged.
- [ ] All Codex prompts reference `master`.
- [ ] User memory note updated.
- [ ] Branch protection rules on `master` enforce PR + CI checks.
- [ ] Next phase PR (Phase 4 prebuilts cpp) targets `master` and merges cleanly.
- [ ] One CI cycle observed where a feature PR triggers the prebuilts workflow on real GitHub-hosted runners.

After acceptance, the `dev` branch can be retired permanently.
