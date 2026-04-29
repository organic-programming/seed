# Codex prompt — RFC codegen chantier finish (overnight, cold start)

> Fresh Codex session. Self-contained. Read `.bpds/op_build_proto_generation.md` (the RFC spec)
> and `CLAUDE.md` (repo invariants). Do not assume any prior session context.

---

## State of the world right now

**Master HEAD**: just received the consolidated stack PR #63 (PRs #59 + #60 + #62 + auto-install + ci hygiene). What that brings to master:

- `bufbuild/protocompile` replaces `jhump/protoreflect` in op (RFC §5).
- `build.codegen.languages` field added to manifest schema (RFC §7).
- Codegen driver in `holons/grace-op/internal/holons/codegen.go` with adapter plugin protocol (RFC §6).
- RFC §0.1 added (Flow A regen requirements when `manifest.proto` evolves).
- Auto-install resolver in `holons/grace-op/internal/holons/lifecycle.go` (`resolveRequiredSDKPrebuilt` decides install vs build vs skip via `decideSDKPrebuiltAutoResolution`).
- `OP_SDK_<LANG>_PATH` env-var override checked first in preflight (priority over auto-install).
- `--no-auto-install` flag on `op build`.
- `dev` branch refs removed from CI workflows; per-workflow header comments added.

**RFC chantier progress**:
- ✅ PR1 (lib swap to protocompile) — merged via #61.
- ✅ PR2 (codegen driver + manifest schema) — merged via stack #63.
- 🟡 PR3 (distributions × 14 langs + adapter plugin) — **prepared locally** on branch `codex/op-build-codegen-distributions`, not yet pushed. ~32 files, +1669 LOC. Includes RFC §6.4 amendment (adapter plugins for protoc built-in emitters).
- ⏳ PR4 (holon migration per RFC §11.2) — to do.
- ⏳ PR5 (cleanup per RFC §11.3) — to do.

**Composer is asleep** for the next ~10 hours. You run autonomously.

---

## Mission this session

Finish the RFC codegen chantier: PR3 → PR4 → PR5. Each PR targets `master` directly. Composer admin-merges in the morning; you do not wait for merge between PRs — push the next as soon as your own verification is green.

If, **and only if**, all 3 PRs are pushed, green on CI, and verified before sunrise, optionally proceed with the pipeline restructure plan at `/Users/bpds/.claude/plans/users-bpds-documents-entrepot-git-compi-vast-rivest.md`. Stop after one open PR for that — composer reviews in the morning.

---

## Context discipline — non-negotiable

Previous session OOM'd. Do not repeat:

- Read each file once with focused offset+limit. Never re-read what you already have in context.
- Do not paste large logs (build output, test verbose, full diffs) back. Extract the diagnostic line, discard the rest.
- Between PRs, drop everything you no longer need: file content, intermediate analysis, search results.
- Carry forward across PRs only: spec invariants + composer decisions + the next PR's exit criterion.
- Plan terse, code direct. No prose to yourself.

---

## Composer decisions (immutable across the chantier)

1. **Distribution layout for codegen plugins**: per RFC §6.2, `codegen.plugins[].{name, binary, out_subdir}` block in `manifest.json`. `binary` relative to distribution root. `out_subdir` under `gen/` at the holon.
2. **Adapter plugins** (RFC §6.4 — codex amended this RFC §): for languages whose emitter is built into `protoc` (cpp, java, python, ruby, csharp, kotlin, legacy js), distribution ships (a) `protoc` binary, (b) `protoc-gen-<lang>` wrapper that fork-execs protoc with `--descriptor_set_in` + `--<lang>_out`. Single Go binary `cmd/protoc-gen-op-adapter` is acceptable.
3. **Op self-build**: per RFC §12, op never sets `build.codegen` on its own manifest. Committed `gen/` is the source of truth for op bootstrap. PR4 holon migration deliberately skips op.
4. **Open question codex resolves in PR4 (or PR5)**: how `op` itself regenerates its stubs when `holon.proto` changes. Pick one — `make regen-stubs` target OR `holons/grace-op/scripts/regen-stubs.sh`. Justify in PR body.
5. **Migration zero-regression**: PR4 must produce a byte-identical `gen/` tree per migrated holon (modulo deterministic header noise). If a diff is non-trivial, halt that holon, do not advance.

---

## PR3 — distributions × 14 langs + adapter plugin

Branch already prepared: `codex/op-build-codegen-distributions`. Tip carries the RFC §6.4 amendment as the first commit, then the distribution wiring.

Steps:

1. `git fetch origin master`. `git checkout codex/op-build-codegen-distributions`.
2. `git rebase origin/master`. Resolve conflicts if any. Most likely areas: `holons/grace-op/internal/sdkprebuilts/{sdkprebuilts,build}.go`, `.github/scripts/build-prebuilt-*.sh`, `sdk/PREBUILTS.md`. Keep your branch's intent (per-lang scripts, registry expansion); drop conflicting fragments from master only when they break that intent.
3. Run locally: `go test ./holons/grace-op/... ./sdk/go-holons/...` (must be green). `bash -n .github/scripts/build-prebuilt-*.sh` (syntax check). `git diff --check`.
4. Push: `git push -u origin codex/op-build-codegen-distributions`.
5. Open PR against `master`. Title: `feat(op): codegen distributions for 14 languages + adapter plugin`. Body **must explicitly flag**: heavy SDK scripts (c/cpp/ruby/zig) were not smoke-built locally — CI is the first full validation. Cite the RFC §6.4 amendment as the first commit of the PR.
6. While CI runs (~3 h popok queue), start PR4 prep locally. Do not wait for PR3 merge.

**Verify before push**:
- `go test ./holons/grace-op/internal/sdkprebuilts/...` green.
- `holons/grace-op/cmd/protoc-gen-op-adapter/main.go` builds (`go build ./holons/grace-op/cmd/protoc-gen-op-adapter`).
- `sdk/PREBUILTS.md` and `holons/grace-op/OP_SDK.md` reflect the 14 distributions.

---

## PR4 — holon migration to `build.codegen`

While PR3 CI runs, build PR4 locally on a fresh branch off PR3's tip:

1. `git checkout -b codex/op-holon-migration codex/op-build-codegen-distributions`.
2. Audit: `grep -rln "before_commands" examples/ holons/` and `grep -rln "tools/generate" holons/`.
3. For each holon found (skip `grace-op` per composer decision #3):
   - Read `api/v1/holon.proto`. Identify which languages it generates stubs for via the existing `before_commands` shell script (look at `examples/<holon>/scripts/generate_proto.sh` or the `before_commands.argv`).
   - Add `build.codegen.languages: [<langs>]` to the manifest. Use the names declared in the per-lang distribution `manifest.json` `codegen.plugins[].name`.
   - Add the corresponding entries to `requires.sdk_prebuilts`.
   - Remove `before_commands` block.
   - Remove `requires.commands` entries that listed `protoc`, `protoc-gen-*`.
4. After modifying each holon:
   - `op build <holon>` (with the relevant `op sdk install <lang>` distributions installed).
   - `git diff <holon>/gen/` — must be byte-clean or only deterministic header noise.
   - If diff is non-trivial, halt that holon, isolate the cause, do not advance other holons.
5. Group migrations: gabriel-greeting examples (one batch), matt-calculator (one batch), other holons (case-by-case). Composer's call on whether to split into multiple PRs — default: one PR for everything that migrates byte-clean.
6. Resolve composer decision #4: pick `make regen-stubs` OR `scripts/regen-stubs.sh` for op self regen. Implement it. Justify the choice in PR body.
7. Once PR3 is mergeable (composer review pending), push PR4 against master. The PR can declare itself "blocked on PR3 merge"; that's fine — composer chains.

**Verify before push**:
- Every migrated holon: `op build <holon>` green, `git diff <holon>/gen/` byte-clean.
- `go test ./...` green at the workspace root.
- `before_commands` count in repo decreases by N (where N = migrated holons).

---

## PR5 — cleanup

After PR4 merges (or as a draft on top of PR4 if PR4 still in review):

1. `git checkout -b codex/op-codegen-cleanup origin/master` (after PR4 lands; else off PR4 tip).
2. Delete `holons/grace-op/tools/generate/` directory.
3. Delete `examples/hello-world/*/scripts/generate_proto.sh` files.
4. Delete `examples/calculator/*/scripts/generate_proto.sh` files (if any).
5. Update `holons/grace-op/OP_BUILD.md`:
   - Mark `before_commands` as legacy with a deprecation note pointing at `build.codegen`.
   - Reference the `regen-stubs` mechanism chosen in PR4.
6. Push and open PR. Title: `chore(op): remove legacy proto-gen scripts (RFC §11.3)`.

**Verify before push**:
- `git grep -n "before_commands\|tools/generate\|generate_proto.sh"` returns only doc/spec references and the OP_BUILD.md deprecation note.
- `go test ./...` green.

---

## Optional — pipeline restructure (only if PR5 lands before sunrise)

Plan file: `/Users/bpds/.claude/plans/users-bpds-documents-entrepot-git-compi-vast-rivest.md` (composer-approved, sections A, A.bis, A.ter, B, C cover the whole spec).

Briefly: create `.github/workflows/pipeline.yml` (6 mandatory tool-gate stages + tier 1 conditional SDK rebuild + tier 2 composite tests + tier 3 per-SDK deep), delete `ci.yml` `ader.yml` `zig-sdk.yml`, reduce `sdk-prebuilts.yml` to post-merge promote, add `.github/workflows/README.md`.

Branch: `codex/pipeline-restructure`. Stop at one open PR — do not merge yourself, composer reviews.

---

## Reporting cadence

After each PR is **pushed** (not merged): post a single message with:
- PR URL.
- Commit range: `git log --oneline <prev-tip>..HEAD`.
- `git diff --stat`.
- RFC section(s) addressed.
- DoD bullet from this prompt that the PR ticks.
- Open question this PR resolves (if any).
- Next PR you are starting NOW.

Then start the next PR immediately.

If you halt mid-chantier:
- Which PR, which step, which file.
- Iterations tried with measured outcome.
- Reproduction steps.
- Recommendation. Do not act on the recommendation; halt and wait for composer.

---

## Hard constraints

- PRs target `master` directly.
- Do not commit transient files in `.codex/` or `.bpds/`.
- Do not modify generated code under `gen/` by hand — only via codegen.
- Zero regression on master tests at every push.
- 10 iterations max on a single blocker before halting.
- Marathon mode. No pauses for "is this OK". Composer is asleep.

---

## Halt rules — strict

True blockers (halt + report):
- RFC ambiguity that materially changes implementation AND no defensible default.
- Test that was green on master is now red after your change AND root cause is unclear after 3 iterations.
- A holon migration produces non-trivial `gen/` diff that you cannot reduce to deterministic header noise after 3 iterations.
- Merge conflict on a rebase that requires non-trivial judgment.

NOT blockers (keep going):
- CI flake on a non-codegen test → `gh run rerun --failed`.
- Composer hasn't reviewed PR3 yet → push PR4 anyway, mark "blocked on PR3 merge".
- Two equally defensible implementations → pick the simpler, document in PR body.
- A holon you cannot migrate cleanly → exclude from PR4, document in PR body, advance other holons.

---

## Definition of done (chantier-level)

- 3 PRs (PR3, PR4, PR5) pushed against master with green CI.
- All 14 distributions ship a `codegen` block.
- All reference holons under `examples/` and `holons/` (except grace-op) use `build.codegen` or have no codegen needs.
- `holons/grace-op/tools/generate/` and per-example `scripts/generate_proto.sh` deleted.
- `op` self regen mechanism implemented and documented.
- `OP_BUILD.md` marks `before_commands` as legacy.
- `OP_SDK.md` documents the `codegen` block in distribution manifests.
- `op build` of a fresh checkout on a machine with **only Go installed** succeeds for grace-op (gen/ committed, RFC §12).

Go.
