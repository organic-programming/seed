# Codex prompt — RFC codegen chantier overnight (resilient, cold start)

> Fresh Codex session. Self-contained. Autonomous through the night.
> Read `.bpds/op_build_proto_generation.md` (the RFC) and `CLAUDE.md` (repo invariants).
> Composer is asleep — no human review available until ~08h. Do not halt unless a true blocker.

---

## Self-discovery — first thing you do

Run these checks before any code. They tell you the actual world state, not assumptions.

```bash
git fetch origin --prune
gh pr view 63 --json state,mergeable,mergeStateStatus
gh pr view 63 --json statusCheckRollup --jq '[.statusCheckRollup[] | .conclusion] | group_by(.) | map({c: .[0], n: length})'
git log --oneline origin/master | head -3
git log --oneline origin/bpds/stack-2026-04-28 | head -5
git ls-remote origin codex/op-build-codegen-distributions | awk '{print $1}'
```

Then branch on what you find:

| Observed state | What it means | Action |
|---|---|---|
| PR #63 OPEN, all checks green | Composer hasn't merged yet (asleep). | Skip step "fix 3rd regression". Proceed to PR3 work, build PRs stacked on `origin/bpds/stack-2026-04-28`, target `master` with "depends on #63" note. |
| PR #63 OPEN, has FAILURE checks | Likely the 3rd regression (git ls-files) — see step 1. | Apply fix in step 1, push, wait CI. If still red after fix, halt. |
| PR #63 OPEN, checks IN_PROGRESS | CI mid-cycle. | Wait via polling until conclusion lands. Then re-evaluate. |
| PR #63 MERGED | Best case. | Skip step 1, proceed PR3 onto fresh `origin/master`, target `master` directly. |
| PR #63 CLOSED (not merged) | Composer rejected; chantier likely deferred. | Halt + report. |

**Polling helper for "wait until CI conclusion lands"** (do not chain shorter sleeps):

```bash
until gh pr view 63 --json statusCheckRollup --jq '[.statusCheckRollup[] | .status] | all(. == "COMPLETED")' | grep -q true; do
  sleep 300
done
```

(5-min poll, fine for self-hosted popok queues that take hours.)

---

## Step 1 — fix 3rd regression (only if PR #63 has FAILURE)

**Symptom**: tests fail with `hash local SDK source for c: list SDK source files with git: exit status 128`.

**Affected tests**: `TestInvoke_CLI_{Examples,Composite,OPService}AcrossTransports` — 9 sub-tests.

**Root cause**: `sdkprebuilts.LocalSourceTreeSHA256` (`holons/grace-op/internal/sdkprebuilts/build.go` near line 130) calls `git ls-files` to hash the SDK source tree. Test workspace mirrors are not git repos → exit 128 → propagates.

**Fix** (commit on `bpds/stack-2026-04-28`):

1. In `localSourceTreeSHA256`:
   - Probe via `git -C repoRoot rev-parse --git-dir` — if it errors, return `("", false, nil)`.
   - When `sourceTreeFiles` returns an error, return `("", false, nil)` instead of propagating.
   - When `sourceTreeFiles` returns 0 files, same: `("", false, nil)` (downgrade the current `fmt.Errorf("no files selected ...")` to a soft "no source").

2. Add a unit test in `holons/grace-op/internal/sdkprebuilts/`:
   - Create a tmp dir that's NOT a git repo, copy `sdk/c-holons` into it as `sdk/c-holons`, run `LocalSourceTreeSHA256("c")` — expect `("", false, nil)`, no error.

3. Verify locally:
   ```bash
   go test ./holons/grace-op/internal/sdkprebuilts/...
   ```

4. Commit:
   ```
   fix(sdk-prebuilts): tolerate non-git workspaces in source tree hash

   The auto-install resolver hashes local SDK sources via `git ls-files` to detect
   when a developer's tree diverges from the published release. Integration test
   workspace mirrors are not git repositories — git exits 128 and the error
   propagated through preflight, breaking 9 transport-level tests.

   Treat "not a git repo" or any git failure as "no local source detectable":
   the resolver falls back to the standard install path, matching the behaviour
   when sdk/<lang>-holons/ doesn't exist at all.
   ```

5. Push to `bpds/stack-2026-04-28`. Wait for CI to flip green via the polling helper above.

If CI is still red after this fix → halt + report (different bug, do not iterate blindly).

---

## State of the world (assume merged after step 1 / polling)

What `master` (or `bpds/stack-2026-04-28` tip pre-merge) carries:
- `bufbuild/protocompile` replacing `jhump/protoreflect` (RFC §5).
- `build.codegen.languages` field on manifest (RFC §7).
- Codegen driver in `holons/grace-op/internal/holons/codegen.go` with adapter plugin protocol (RFC §6).
- RFC §0.1 added (Flow A regen requirements).
- Auto-install resolver in `lifecycle.go` (`resolveRequiredSDKPrebuilt` decides install/build/skip).
- `OP_SDK_<LANG>_PATH` env-var override checked first in preflight.
- `--no-auto-install` flag.

RFC chantier progress:
- ✅ PR1 (lib swap) merged via #61.
- ✅ PR2 (codegen driver + manifest schema) merged via stack #63.
- 🟡 PR3 (distributions × 14 langs + adapter plugin) — prepared on `codex/op-build-codegen-distributions`.
- ⏳ PR4 (holon migration per RFC §11.2).
- ⏳ PR5 (cleanup per RFC §11.3).

---

## Branching strategy when PR #63 not merged

Composer is asleep — they will merge #63 in the morning. You **do not wait**. You build PRs stacked off `origin/bpds/stack-2026-04-28` (the post-#63 state), pushing each as its own branch, opening each PR against `master` with a body note `Depends on #63 — rebase trivial after merge.`

Composer's morning ritual: merge #63 → rebase + auto-retarget the others (gh handles this) → admin-merge in chain.

When PR #63 IS merged before you start: same logic but you base on `origin/master` directly and skip the "depends on #63" note.

---

## Context discipline — non-negotiable

Previous session OOM'd. You are running 10h+. Do not repeat:

- Read each file once with focused offset+limit. Never re-read what you have in context.
- Do not paste large logs back. Extract the diagnostic line, discard the rest.
- Between PRs, drop file content + intermediate analysis. Carry forward only spec invariants + composer decisions + next PR's exit criterion.
- Plan terse, code direct. No prose to yourself.
- Use grep first to locate, then read only the region needed.

---

## Composer decisions (immutable)

1. **Distribution layout for codegen plugins** (RFC §6.2): `codegen.plugins[].{name, binary, out_subdir}` block in `manifest.json`. `binary` relative to distribution root. `out_subdir` under `gen/` at the holon.
2. **Adapter plugins** (RFC §6.4 — your earlier work): for protoc built-in emitters (cpp, java, python, ruby, csharp, kotlin, legacy js), distribution ships (a) `protoc` binary, (b) `protoc-gen-<lang>` wrapper that fork-execs protoc with `--descriptor_set_in` + `--<lang>_out`. Single Go binary `cmd/protoc-gen-op-adapter` is acceptable (already in PR3 prep).
3. **Op self-build** (RFC §12): op never sets `build.codegen` on its own manifest. Committed `gen/` is op's source of truth. PR4 holon migration deliberately skips op.
4. **Open question codex resolves in PR4 or PR5**: how `op` regenerates its stubs when `holon.proto` changes. Pick `make regen-stubs` target OR `holons/grace-op/scripts/regen-stubs.sh`. Justify in PR body. Either is acceptable.
5. **Migration zero-regression** (PR4): each migrated holon must produce byte-identical `gen/` (modulo deterministic header noise). Non-trivial diff → halt that holon, do not advance other batches.

---

## PR3 — distributions × 14 langs + adapter plugin

Branch already prepared: `codex/op-build-codegen-distributions`. Tip carries the RFC §6.4 amendment and the distribution wiring.

```bash
git fetch origin
git checkout codex/op-build-codegen-distributions
git rebase origin/bpds/stack-2026-04-28   # if #63 not merged
# OR
git rebase origin/master                  # if #63 merged
```

Resolve conflicts (likely: `holons/grace-op/internal/sdkprebuilts/{sdkprebuilts,build}.go`, `.github/scripts/build-prebuilt-*.sh`, `sdk/PREBUILTS.md`). Keep your branch's intent (per-lang scripts, registry expansion); drop conflicting fragments from the base only when they break that intent.

Verify before push:
```bash
go test ./holons/grace-op/... ./sdk/go-holons/...
bash -n .github/scripts/build-prebuilt-*.sh
git diff --check
```

Push:
```bash
git push -u origin codex/op-build-codegen-distributions
gh pr create --base master --title "feat(op): codegen distributions for 14 languages + adapter plugin" --body "<see body template below>"
```

Body template:
```
## Summary
- Adds codegen plugin support to all 14 language distributions per RFC §11.0.
- Standalone-plugin distributions: go, c, rust, dart, swift, js-web, zig.
- Built-in protoc emitter distributions: cpp, java, python, ruby, csharp, kotlin, legacy js — wrapped via `protoc-gen-op-adapter` (RFC §6.4 amendment, first commit).
- Heavy SDK scripts (c, cpp, ruby, zig) were not smoke-built locally — CI is the first full validation.

Depends on #63 — rebase trivial after merge.

## Test plan
- `go test ./holons/grace-op/internal/sdkprebuilts/...` green.
- `holons/grace-op/cmd/protoc-gen-op-adapter` builds.
- 10/14 distributions smoke-built locally (go, python, java, csharp, kotlin, rust, dart, swift, js-web, js).

🤖 Generated with [Claude Code](https://claude.com/claude-code)
```

While CI runs (~3 h popok queue), start PR4 prep locally. Do not wait for PR3 merge.

---

## PR4 — holon migration to `build.codegen`

Branch off PR3's tip:
```bash
git checkout -b codex/op-holon-migration codex/op-build-codegen-distributions
```

Audit:
```bash
grep -rln "before_commands" examples/ holons/ | grep -v node_modules
grep -rln "tools/generate" holons/ examples/
```

For each holon found (skip `grace-op` per composer decision #3):
1. Read `<holon>/api/v1/holon.proto` near the top — identify which langs the existing generator emits (cf the `before_commands.argv` or `examples/<holon>/scripts/generate_proto.sh`).
2. In the manifest, add `build.codegen.languages: [<lang_names>]` matching the distribution's `codegen.plugins[].name`.
3. Add `requires.sdk_prebuilts: [<langs>]` if not already there.
4. Remove `before_commands` block.
5. Remove `requires.commands` entries listing `protoc`, `protoc-gen-*`.
6. Run `op build <holon>` (with the right distributions installed via `op sdk install <lang>`).
7. `git diff <holon>/gen/` — must be byte-clean or only deterministic header noise (e.g. timestamp, generator version).
8. If non-trivial diff: revert this holon's changes, document in PR4 body as deferred, advance to next holon.

Group: gabriel-greeting examples (one batch), matt-calculator (one batch), other holons. Default: one PR for everything that migrates byte-clean.

Resolve composer decision #4: pick `make regen-stubs` OR `scripts/regen-stubs.sh` for op self regen. Implement. Justify choice in PR4 body.

Push when PR3 reaches mergeable state (CI green; need not wait for human merge):
```bash
git push -u origin codex/op-holon-migration
gh pr create --base master --title "feat(op): migrate holons to build.codegen (RFC §11.2)" --body "Depends on #63 and PR3."
```

---

## PR5 — cleanup

Branch off PR4's tip:
```bash
git checkout -b codex/op-codegen-cleanup codex/op-holon-migration
```

1. Delete `holons/grace-op/tools/generate/` directory.
2. Delete `examples/hello-world/*/scripts/generate_proto.sh`.
3. Delete `examples/calculator/*/scripts/generate_proto.sh` (if any).
4. In `holons/grace-op/OP_BUILD.md`, mark `before_commands` as legacy with deprecation note pointing at `build.codegen`. Reference the regen-stubs mechanism chosen in PR4.

Verify:
```bash
git grep -n "before_commands\|tools/generate\|generate_proto.sh"
# Should only return doc/spec references and the OP_BUILD.md deprecation note.
go test ./...
```

Push:
```bash
git push -u origin codex/op-codegen-cleanup
gh pr create --base master --title "chore(op): remove legacy proto-gen scripts (RFC §11.3)" --body "Depends on PR3, PR4."
```

---

## Optional — pipeline restructure (only if PR5 pushed before sunrise)

Plan file: `/Users/bpds/.claude/plans/users-bpds-documents-entrepot-git-compi-vast-rivest.md`. Composer-approved Phase 2.

In short: create `.github/workflows/pipeline.yml` (6 mandatory tool-gate stages + tier 1 conditional SDK rebuild + tier 2 composite tests + tier 3 per-SDK deep), delete `ci.yml` `ader.yml` `zig-sdk.yml`, reduce `sdk-prebuilts.yml` to post-merge promote, add `.github/workflows/README.md`.

Branch: `codex/pipeline-restructure` off PR5's tip. Stop at one open PR — composer reviews in the morning.

---

## Reporting cadence

After each PR is **pushed** (do not wait for merge), post a single message:
- PR URL.
- Commit range: `git log --oneline <prev-tip>..HEAD`.
- `git diff --stat`.
- RFC section addressed.
- DoD bullet from this prompt that the PR ticks.
- Open question this PR resolves (if any).
- Next PR you are starting NOW.

Then start the next PR immediately.

If you halt mid-chantier:
- Which PR, which step, which file.
- Iterations tried with measured outcome (≤ 10).
- Reproduction steps.
- Recommendation. Do not act on it; halt and wait for composer.

---

## Hard constraints

- PRs target `master` directly. If PR #63 unmerged, body says "Depends on #63 — rebase trivial after merge."
- Do not commit transient files in `.codex/` or `.bpds/`.
- Do not modify generated code under `gen/` by hand — only via codegen.
- Zero regression on master tests at every push.
- 10 iterations max on a single blocker before halting.
- Marathon mode. No pauses for "is this OK". Composer is asleep until ~08h.

---

## Halt rules — strict

True blockers (halt + report):
- RFC ambiguity that materially changes implementation AND no defensible default.
- Test that was green is red after your change AND root cause unclear after 3 iterations.
- A holon migration produces non-trivial `gen/` diff that you cannot reduce after 3 iterations.
- Merge conflict requiring non-trivial judgment.
- After step 1 fix, CI is still red — different bug, escalate.

NOT blockers (keep going):
- CI flake on a non-codegen test → `gh run rerun --failed`.
- Composer hasn't merged PR3 yet → push PR4 anyway, body says "Depends on PR3".
- Two equally defensible implementations → pick simpler, document in PR body.
- A holon you cannot migrate cleanly → exclude from PR4, document in PR body, advance other holons.

---

## Definition of done

- 3 PRs (PR3, PR4, PR5) pushed against master. Each with green CI by morning OR clearly documented blocker.
- All 14 distributions ship a `codegen` block.
- All reference holons (except grace-op) use `build.codegen` or have no codegen needs.
- `holons/grace-op/tools/generate/` and per-example `scripts/generate_proto.sh` deleted.
- `op` self regen mechanism implemented and documented.
- `OP_BUILD.md` marks `before_commands` as legacy.
- `OP_SDK.md` documents the `codegen` block.
- Step 1 fix landed if it was needed.

Go.
