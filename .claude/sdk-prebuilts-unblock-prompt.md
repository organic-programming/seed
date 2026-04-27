# Claude prompt — unblock the SDK prebuilts release pipeline

> Hand this prompt to a fresh Claude Code session. Self-contained: the
> session has filesystem access to the `seed` repo and `gh` CLI. Do not
> assume any prior conversation context.

---

## Mission (one sentence)

Make `op sdk install zig` (and `c`, `cpp`, `ruby`) succeed end-to-end on
a fresh checkout, so every example in `examples/hello-world/` builds and
runs without manual prerequisites.

That requires the GitHub Releases for the four SDK prebuilts to actually
exist. Today they do not — every run of `.github/workflows/sdk-prebuilts.yml`
fails before any job starts ("workflow file issue"), so the `promote`
job never publishes anything, and `op sdk install zig` reports:

```
op sdk install: no available sdk prebuilt release for zig aarch64-apple-darwin
```

---

## State at session start

- All prebuilts chantier PRs (#41–#45) are **merged** on `master`.
- All `sdk-prebuilts.yml` runs since commit `9b5d54f` are failing **at
  workflow load time** — `gh run view <id>` shows zero jobs and the message
  "This run likely failed because of a workflow file issue."
- No GitHub Releases exist (`gh release list` returns nothing).
- The hotfix in PR #54 (Rust gen + describe template) is merged. Rust
  hello-world builds. The Zig / C / C++ / Ruby hello-worlds remain
  blocked by the missing prebuilt releases.
- `op run gabriel-greeting-app-swiftui --clean` fails on the
  `gabriel-greeting-zig` member with the message above.

## Suspected root cause (verify before fixing)

Commit `9b5d54f` ("ci(sdk-prebuilts): restore full matrix with
pause-on-block mechanism") added two things to every language job in
`.github/workflows/sdk-prebuilts.yml`:

1. Five Linux/Windows matrix entries with a custom `continue_on_error: true`
   field on the matrix entry.
2. A job-level expression: `continue-on-error: ${{ matrix.continue_on_error || false }}`.

Diff:

```yaml
          - target: x86_64-unknown-linux-gnu
            runs_on: '["ubuntu-latest"]'
            continue_on_error: true  # paused: target needs validation
          # …
    continue-on-error: ${{ matrix.continue_on_error || false }}
    uses: ./.github/workflows/_sdk-prebuilt-target.yml
```

Hypotheses (rank by likelihood, verify the top one first):

1. **GitHub Actions does not allow `continue-on-error` at the job level
   when calling a reusable workflow with `uses: ./.github/workflows/...`**
   — only the standard job keys (`needs`, `if`, `permissions`, `strategy`,
   `secrets`, `with`) are allowed there. If true, the YAML is rejected
   at load.
2. The expression `${{ matrix.continue_on_error || false }}` evaluates
   to a string `"false"` or to the literal text on entries where
   `matrix.continue_on_error` is undefined, which fails the boolean
   schema validation.
3. `'["ubuntu-24.04-arm"]'` is not an available runner label in this
   org. (Less likely — would surface as a per-job failure, not a
   load-time failure.)

The "load-time failure" symptom (zero jobs, no logs accessible) most
strongly points to hypothesis 1.

## Required reading

1. [`.github/workflows/sdk-prebuilts.yml`](../.github/workflows/sdk-prebuilts.yml) — the broken workflow.
2. [`.github/workflows/_sdk-prebuilt-target.yml`](../.github/workflows/_sdk-prebuilt-target.yml) — the reusable workflow it calls.
3. Commit `9b5d54f`: `git show 9b5d54f -- .github/workflows/sdk-prebuilts.yml`.
4. [`docs/specs/sdk-prebuilts.md`](../docs/specs/sdk-prebuilts.md) — chantier spec, especially §promote logic and §matrix coverage.
5. [`holons/grace-op/internal/sdkprebuilts/sdkprebuilts.go`](../holons/grace-op/internal/sdkprebuilts/sdkprebuilts.go) — the `op sdk install` resolver. It looks up GitHub Releases by tag pattern `<lang>-holons-v<version>`.
6. [`.github/scripts/promote-sdk-prebuilts.sh`](../.github/scripts/promote-sdk-prebuilts.sh) — what the promote job runs.
7. [`WORKFLOW.md`](../WORKFLOW.md) — branch policy: `master` is trunk, all PRs target `master`.
8. [`CLAUDE.md`](../CLAUDE.md) — repo invariants, "doubt is the method".

## Implementation plan

### Step 1 — Confirm the root cause

Open the latest failed run in the browser and read the validation message:

```bash
gh run list --workflow sdk-prebuilts.yml --limit 5 --json databaseId,headBranch,conclusion
gh run view <latest_master_run_id>
# follow the printed URL → look for the YAML validation error in the GitHub UI
```

If hypothesis 1 is confirmed (`continue-on-error` not allowed at job
level for `uses:` jobs), proceed to Step 2. If a different cause shows
up, halt and report.

### Step 2 — Fix the workflow

The "pause-on-block" intent of commit `9b5d54f` is sound: non-mac
targets that fail should not block the overall workflow. Achieve it
without `continue-on-error` at the job level.

Two acceptable patterns. Pick one, justify in the PR description.

**Option A — push the `continue-on-error` into the reusable workflow.**
Add a `continue-on-error` input to `_sdk-prebuilt-target.yml` defaulting
to `false`. Plumb it down to the relevant build step(s). Pass `true`
from the matrix entries that need to be paused.

**Option B — keep all jobs blocking but mark the non-mac targets as
build-only paused via the matrix `if:` filter.** That excludes them
from CI for now (status quo before commit `9b5d54f`) and explicitly
tracks reintroduction in `docs/specs/sdk-prebuilts.md` and a follow-up
issue.

Option A is closer to the original "restore full matrix" intent. Prefer
A unless you discover a blocker.

Test by triggering `workflow_dispatch` on the branch:

```bash
gh workflow run sdk-prebuilts.yml --ref bpds/<your-branch>
gh run watch <run_id>
```

The macOS jobs must succeed. Non-mac jobs may fail but must not block
the run from reaching `conclusion: success`.

### Step 3 — Trigger a successful master run

After the fix lands on `master` (PR opened, composer admin-merges):

```bash
gh workflow run sdk-prebuilts.yml --ref master
gh run watch <run_id>
```

This run does **not** publish releases on its own — the `promote` job
only runs on `pull_request: closed && merged`. Do not expect releases
yet at this point.

Why run it on `master` first? It validates that the fix took, and it
also gates the next PR's CI (which goes through `pull_request`).

### Step 4 — Open a stub PR to trigger promotion

The promote job is gated on `pull_request.closed && merged`. To
publish the four releases for the first time, push a tiny no-op PR
that touches one of the trigger paths in `sdk-prebuilts.yml`:

```yaml
paths:
  - ".github/workflows/sdk-prebuilts.yml"
  - ".github/workflows/_sdk-prebuilt-target.yml"
  - ".github/scripts/build-prebuilt-zig.sh"
  - ".github/scripts/build-prebuilt-cpp.sh"
  - ".github/scripts/build-prebuilt-c.sh"
  - ".github/scripts/build-prebuilt-ruby.sh"
  - ".github/scripts/promote-sdk-prebuilts.sh"
  - "sdk/zig-holons/**"
  - "sdk/cpp-holons/**"
  - "sdk/c-holons/**"
  - "sdk/ruby-holons/**"
  - "holons/grace-op/internal/sdkprebuilts/**"
```

The workflow-fix PR from Step 2 is already enough — it touches
`sdk-prebuilts.yml`. So:

1. Wait for the workflow-fix PR's PR-level `sdk-prebuilts.yml` run to
   complete successfully (artifacts uploaded for mac targets at least).
2. Composer admin-merges the PR.
3. The `promote` job triggers on the close-merged event, downloads the
   PR's artifacts, and publishes:
   - `zig-holons-v0.1.0`
   - `cpp-holons-v1.80.0`
   - `c-holons-v1.80.0`
   - `ruby-holons-v1.58.3`

Watch the promote job:

```bash
gh run list --workflow sdk-prebuilts.yml --limit 5
gh run view <promote_run_id>
gh release list
```

If the promote job fails (e.g., no artifacts found because PR-CI ran
the broken workflow before Step 2's fix was committed), follow Step 5.

### Step 5 — Recovery if promote fails

If the artifacts from the merged PR are missing or stale:

1. Trigger `workflow_dispatch` on `master` post-merge — that uploads
   artifacts but does not publish releases.
2. Run `.github/scripts/promote-sdk-prebuilts.sh` locally with
   `RUNNER_TEMP=/tmp/promote-test` after manually downloading the
   artifacts via `gh run download`. This validates the script works
   off-line.
3. As a last resort, publish the four releases manually from local
   builds. The build scripts (`build-prebuilt-{zig,cpp,c,ruby}.sh`)
   produce artifacts that the promote script can consume. Document this
   manual recovery in the PR description so it's not repeated.

### Step 6 — Verify all hello-worlds build

From a clean checkout (or after `op clean ...`):

```bash
op sdk install zig
op sdk install c
op sdk install cpp
op sdk install ruby

op build gabriel-greeting-zig
op build gabriel-greeting-c
op build gabriel-greeting-cpp
op build gabriel-greeting-ruby

# The composite the user actually cares about:
op run gabriel-greeting-app-swiftui --clean
```

Each install must take ~1–2 seconds (download + extract; not source
compile). Each build must succeed. The composite must come up with all
13 member holons.

### Step 7 — Documentation & follow-ups

- Update `docs/specs/sdk-prebuilts.md` if the workflow shape changed
  (e.g., Option A added an input to `_sdk-prebuilt-target.yml`).
- Close issue #34 (Windows MSVC gap) only if the fix actually addresses
  it; otherwise note the residual scope on the issue.
- Confirm `OP_SDK.md` and `OP_BUILD.md` still reflect reality. The user
  reads these — don't let them drift.

## PR conventions

- Branch name: `bpds/fix-sdk-prebuilts-workflow` or similar (composer is
  the actor).
- Base: `master`.
- Title: `fix(ci): restore sdk-prebuilts workflow load`.
- One commit per logical step (Step 2 = the fix; Step 7 = doc updates).
- PR body must include:
  - Confirmed root cause (the validation error verbatim from the GitHub UI).
  - Which option (A or B) was chosen and why.
  - Evidence the workflow now loads (`gh run view <id>` showing jobs).
  - Evidence releases were published (`gh release list`).
  - Evidence at least one previously-blocked example builds
    (`op build gabriel-greeting-zig` output).

## Operating mode

- **Halt at any real doubt.** Examples: hypothesis 1 turns out wrong
  and the actual root cause is unclear; the promote script behaves
  unexpectedly when fed real artifacts; the user's `OP_SDK_*` env
  divergence affects results.
- **Do not retry destructive git operations** (force-push, branch
  delete) without composer confirmation.
- **Do not invent versions.** The four pinned versions
  (`0.1.0` / `1.80.0` / `1.80.0` / `1.58.3`) come from the spec;
  changing them is a separate decision.
- **Do not publish releases manually unless Step 5 explicitly invokes
  it.** Releases on this repo are explicit events, not casual.

## Definition of done

- [ ] `gh run list --workflow sdk-prebuilts.yml` shows recent runs with
      `conclusion: success` (mac targets all green; non-mac targets
      either green or paused-and-skipped per the chosen option).
- [ ] `gh release list` shows the four expected tags
      (`zig-holons-v0.1.0`, `cpp-holons-v1.80.0`, `c-holons-v1.80.0`,
      `ruby-holons-v1.58.3`).
- [ ] `op sdk install zig && op build gabriel-greeting-zig` succeeds
      on a fresh checkout (no manual prerequisites).
- [ ] `op run gabriel-greeting-app-swiftui --clean` succeeds with all
      13 members built.
- [ ] PR opened against `master`, composer admin-merges, chantier
      closes.

Go.
